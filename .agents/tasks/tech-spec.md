# Technical Specification: Awaiting Approval Kanban Lane for Task Review Queue

## 1. Overview

### 1.1 Goal
Add an `Awaiting Approval` lane to the Tasks board so operators can review all `validated` tasks from one queue, with deterministic priority ordering, when feature flag `smith.feature_tasks_kanban_enabled` is enabled.

### 1.2 Scope Alignment
In scope:
- Show/hide `Awaiting Approval` lane behind feature flag.
- Group all `validated` tasks into this lane.
- Enforce stable ordering: higher priority first, then oldest validation timestamp.
- Remove tasks from `Awaiting Approval` immediately after approval or rejection (rejection maps to existing `validated -> draft` transition).
- Show empty state when no validated tasks exist.

Out of scope:
- New task states or approval roles.
- Bulk actions.
- Cross-project aggregation.

## 2. Architecture Overview

### 2.1 Current Baseline
- UI currently renders a flat task list in [`frontend/src/routes/tasks/+page.svelte`](/workspace/frontend/src/routes/tasks/+page.svelte).
- Tasks data comes from `GET /api/tasks` (`/v1/tasks` equivalent) and includes current status values only.
- Existing status model: `draft`, `validated`, `approved`, `running`, `completed`, `blocked`.

### 2.2 Target Architecture
- Keep existing task APIs as source of truth.
- Add a Kanban presentation layer in frontend that builds lanes from task status.
- Add a dedicated runtime flag for Kanban behavior:
  - Runtime config key: `featureTasksKanbanEnabled`
  - Env var: `SMITH_FEATURE_TASKS_KANBAN_ENABLED`
  - Helm value: `console.featureFlags.tasksKanban`
- Add ordering fields to task contract so sorting is deterministic and server-defined:
  - `review_priority` (integer)
  - `validated_at` (RFC3339 timestamp)

### 2.3 Lane Semantics
When Kanban flag is enabled:
- `Awaiting Approval` lane contains only tasks with `status=validated`.
- Other lanes continue to show all non-validated statuses.
- A validated task must not appear in any other lane (single-card placement invariant).

When Kanban flag is disabled:
- Existing board/list behavior remains unchanged.

## 3. Detailed Component Design

### 3.1 Feature Flag Plumbing

Frontend changes:
- Extend [`frontend/src/lib/feature-flags.ts`](/workspace/frontend/src/lib/feature-flags.ts):
  - Add `featureTasksKanbanEnabled?: boolean | string` to runtime config type.
  - Add `isTasksKanbanEnabled()` helper using existing boolean parsing logic.
- Extend runtime injection:
  - [`frontend/deploy/runtime-config.template.js`](/workspace/frontend/deploy/runtime-config.template.js)
  - [`frontend/deploy/30-smith-env.sh`](/workspace/frontend/deploy/30-smith-env.sh)
- Helm propagation:
  - [`helm/smith/templates/console-configmap.yaml`](/workspace/helm/smith/templates/console-configmap.yaml)
  - [`helm/smith/templates/console-deployment.yaml`](/workspace/helm/smith/templates/console-deployment.yaml)
  - [`helm/smith/values.yaml`](/workspace/helm/smith/values.yaml)

Flag behavior:
- `featureTasksEnabled` continues to gate route visibility.
- Kanban UI activates only when both are true:
  - `featureTasksEnabled == true`
  - `featureTasksKanbanEnabled == true`

### 3.2 Tasks UI: Kanban Composition

Primary file:
- [`frontend/src/routes/tasks/+page.svelte`](/workspace/frontend/src/routes/tasks/+page.svelte)

Design:
- Introduce lane model in component state:
  - `draft`, `awaiting_approval`, `approved`, `running`, `completed`, `blocked`
- Derive lanes from fetched `tasks` collection.
- `awaiting_approval` lane filter: `task.status === 'validated'`.
- Do not duplicate cards across lanes.

Ordering for `Awaiting Approval`:
1. `review_priority` descending (higher first)
2. `validated_at` ascending (oldest first)
3. `id` ascending (deterministic tiebreaker)

Fallbacks for legacy data:
- If `review_priority` missing: treat as `0`.
- If `validated_at` missing for validated tasks: fallback to `updated_at`, then `created_at`.

Post-action updates:
- Existing `approve()` and `reopenDraft()` flows already call server then `loadTasks()`.
- Kanban lanes are recomputed immediately from refreshed task list.
- Lane counts are computed from rendered lane data source (not separate counter state) to avoid drift.

Empty state:
- When `Awaiting Approval` lane has zero cards, show non-blocking message:
  - Example: `No tasks awaiting approval.`
- Empty state appears only when lane is enabled.

### 3.3 Backend Task Contract Enrichment

Core files:
- [`internal/source/model/task_contract.go`](/workspace/internal/source/model/task_contract.go)
- [`pkg/api/v1/types.go`](/workspace/pkg/api/v1/types.go)
- [`cmd/smith-api/main.go`](/workspace/cmd/smith-api/main.go)
- [`internal/source/store/mem_store_tasks.go`](/workspace/internal/source/store/mem_store_tasks.go)
- [`internal/source/store/etcd_tasks.go`](/workspace/internal/source/store/etcd_tasks.go)

Add fields:
- `review_priority int` (default `0`)
- `validated_at time.Time` (omitempty semantics)

Status transition rules:
- On `draft -> validated` (PATCH): set `validated_at=now` if empty.
- On create with `status=validated`: set `validated_at=now` if empty.
- On `validated -> draft` rejection: keep `validated_at` for audit history.
- On approve endpoint (`validated -> approved`): preserve `validated_at`.

Validation rules:
- `review_priority` range: `0..100` (or agreed bounded range).
- Reject out-of-range with `400`.

Audit metadata additions:
- Include `review_priority` and `validated_at` in patch/approve audit metadata when changed or relevant.

## 4. API Definitions

## 4.1 Endpoints
No new endpoint is required.

Existing endpoints remain:
- `GET /v1/tasks`
- `POST /v1/tasks`
- `GET /v1/tasks/{id}`
- `PATCH /v1/tasks/{id}`
- `POST /v1/tasks/{id}/approve`

(` /api/*` aliases remain supported as today.)

### 4.2 Contract Additions

Task object additions:
- `review_priority: integer`
- `validated_at: string (RFC3339, optional)`

Create request addition (`POST /v1/tasks`):
- `review_priority?: integer`

Patch request addition (`PATCH /v1/tasks/{id}`):
- `review_priority?: integer`

Response behavior:
- Legacy tasks missing new fields remain valid responses.
- API emits defaults/omitempty consistently so older clients do not break.

### 4.3 Ordering Responsibility
- Canonical queue ordering logic is defined in this spec and implemented in frontend sorter.
- Backend provides ordering inputs (`review_priority`, `validated_at`) and can later expose server-side sorting without contract break.

## 5. Data Model Changes

### 5.1 Model Updates
`model.TaskContract` and `api.TaskContract` add:
- `ReviewPriority int`
- `ValidatedAt time.Time`

Request types add:
- `TaskContractCreateRequest.ReviewPriority`
- `TaskContractPatchRequest.ReviewPriority`

### 5.2 Persistence / Migration
- Store is schemaless JSON in etcd/memory; no blocking migration required.
- Existing rows/documents deserialize with zero values.
- Backfill strategy for ranking legacy validated tasks:
  - If `validated_at` is zero and status is `validated`, use `updated_at` fallback for display ordering.

### 5.3 Compatibility
- Backward compatible with existing clients and data.
- No status enum changes.
- No transition matrix changes.

## 6. Security Considerations

- Fail-closed flag policy: Kanban lane is hidden unless explicit enablement.
- No new roles/permissions: approvals/rejections continue existing operator path.
- Input validation:
  - Enforce integer bounds on `review_priority`.
  - Sanitize/normalize incoming payload as existing handlers do.
- Auditability:
  - Continue emitting `patch-task` / `approve-task` audit events.
  - Include relevant queue-order metadata to support incident review.
- Data exposure:
  - No new sensitive fields introduced.
  - `Awaiting Approval` lane reuses existing task data visibility.

## 7. Testing and Verification (Implementation Guidance)

Frontend:
- Unit tests for lane composition and ordering comparator.
- Tests for flag on/off behavior and empty-state rendering.
- Tests ensuring validated cards appear once only.

Backend:
- API tests for create/patch with `review_priority`.
- Transition tests asserting `validated_at` set semantics.
- Tests for out-of-range `review_priority` request rejection.

End-to-end:
- Validate approve/reject removes card from `Awaiting Approval` after refresh cycle.
- Validate deterministic order stability across refreshes.

## 8. Rollout Plan

1. Ship backend contract additions first (backward-compatible).
2. Ship frontend Kanban lane gated by `SMITH_FEATURE_TASKS_KANBAN_ENABLED=false` default.
3. Enable flag in a non-prod environment and verify metrics.
4. Gradual production enablement.

## 9. Open Decisions

- Confirm bounded range for `review_priority` (`0..100` proposed).
- Confirm lane order among non-approval lanes in Kanban view (current status-based order proposed).
- Confirm whether API should eventually add server-side sorted/filter view (not required for this increment).
