# Technical Specification: Finished Lane for Terminal Outcomes (Tasks Kanban)

## Document Control
- Source PRD: `/workspace/.agents/tasks/prd.json`
- Feature: `[Feature Flag Smith Feature Tasks Kanban Enabled Prd] Finished lane for terminal outcomes`
- Scope: specification only (no implementation)

## Architecture Overview

### Objective
Add a **Finished** lane in the Tasks Kanban experience that groups all terminal task outcomes (`completed`, `blocked`), preserves outcome context (including blocked reason), and is controlled by a dedicated kanban feature flag.

### Current Baseline
- Task contracts already have lifecycle status values including `completed` and `blocked`.
- Task status is synchronized from loop lifecycle transitions via `syncTaskContractStatusForLoop`.
- Tasks UI currently renders a flat list and already uses runtime feature-flag plumbing via `window.__SMITH_CONFIG__`.

### Target Architecture
- **Control plane (runtime config + feature flags):** add a new console runtime flag for Kanban behavior.
- **API/domain layer:** enrich task contract terminal metadata when tasks enter terminal states.
- **UI presentation layer:** derive Kanban lanes from task list response and group terminal states into Finished when flag is enabled.
- **Auditability layer:** ensure Finished counts are deterministic from API payload and consistent with task status transitions.

### High-Level Flow
1. Loop/task transition sets task to `completed` or `blocked`.
2. API persists terminal metadata (`terminal_outcome`, optional `terminal_reason`, `terminal_at`).
3. Tasks page fetches `/tasks`.
4. If Kanban flag is enabled, UI maps tasks to lanes and places terminal statuses in **Finished**.
5. Finished header count = number of tasks in Finished (completed + blocked).

## Detailed Component Design

### 1) Feature-flag and config surface

#### New runtime config key
- `featureTasksKanbanEnabled` (boolean-like; parsed with existing `parseBoolean` behavior).

#### Environment and Helm mapping
- Env var: `SMITH_FEATURE_TASKS_KANBAN_ENABLED`
- Helm value: `console.featureFlags.tasksKanban`
- Default: `false` (controlled rollout)

#### Frontend utility addition
- Add `isTasksKanbanEnabled(config = getRuntimeConfig()): boolean` in `frontend/src/lib/feature-flags.ts`.
- Add tests in `frontend/src/lib/feature-flags.test.ts` for default-off and boolean/string override behavior.

#### Disabled-state behavior (required by PRD)
- When `featureTasksKanbanEnabled=false`, Finished-lane grouping is not active.
- Existing non-kanban/legacy tasks rendering remains unchanged.

### 2) Domain model updates for terminal outcomes

#### Task contract additions
Add optional fields to task contract model/API types:
- `terminal_outcome`: `'completed' | 'blocked' | ''` (optional on wire)
- `terminal_reason`: `string` (optional; typically populated for blocked outcomes)
- `terminal_at`: RFC3339 timestamp string/time (optional)

#### Why explicit fields (vs metadata-only)
- Prevents UI dependence on ad-hoc metadata keys.
- Improves contract clarity for audit and filtering.
- Keeps backward compatibility through optional fields.

#### Transition behavior
- On transition to `completed`:
  - `status=completed`
  - `terminal_outcome=completed`
  - `terminal_reason=''`
  - `terminal_at=now`
- On transition to `blocked`:
  - `status=blocked`
  - `terminal_outcome=blocked`
  - `terminal_reason=reason` (if provided)
  - `terminal_at=now`
- On transition from terminal back to active (`draft|validated|approved|running`):
  - clear `terminal_outcome`, `terminal_reason`, `terminal_at`

#### Ownership of updates
- Primary path: `syncTaskContractStatusForLoop(...)` in API server.
- Manual patch path (if future transitions allow terminal edits) must preserve same invariants.

### 3) Tasks Kanban lane composition

#### Lane model
Canonical lane mapping when Kanban enabled:
- `Draft`: `draft`
- `Validated`: `validated`
- `Approved`: `approved`
- `Running`: `running`
- `Finished`: `completed | blocked`

#### Finished lane rules
- A task in `completed` or `blocked` appears exactly once in Finished.
- Terminal tasks are excluded from non-terminal lanes.
- Mixed terminal outcomes are visually distinguished by outcome badges:
  - `Completed` badge (success style)
  - `Blocked` badge (warning/error style)
- If `terminal_outcome` missing for older records, UI falls back to `status` inference.

#### Sorting and selectability
- Default Finished sort: `terminal_at desc` (fallback `updated_at desc`, then `id`).
- All items remain selectable/openable with current task details affordances.

#### Count behavior
- Finished lane header count = number of tasks assigned to Finished lane.
- Must update after refresh and after status transitions.
- No duplication allowed across lanes (single source lane assignment function).

### 4) Task details panel behavior in Finished
- Always show terminal outcome label (`completed`/`blocked`).
- Show `terminal_reason` when outcome is blocked and value exists.
- If blocked with no reason, show deterministic fallback copy (e.g., `No block reason provided`).

### 5) Auditing and observability

#### Audit records
Extend existing task-status synchronization audit metadata to include:
- `terminal_outcome`
- `terminal_reason_present` (`true|false`)
- `terminal_at`

#### Debug/diagnostic checks
- Add invariant check in UI unit tests: sum of lane counts equals task list length.
- Add API test assertions for terminal field set/clear behavior on state transitions.

## API Definitions

## Existing endpoints reused
- `GET /api/tasks` and `GET /v1/tasks`
- `PATCH /api/tasks/{id}` and `PATCH /v1/tasks/{id}`
- Loop lifecycle endpoints that trigger task sync (existing)

## Contract changes

### TaskContract response object (additive)
```json
{
  "id": "task-123",
  "status": "blocked",
  "terminal_outcome": "blocked",
  "terminal_reason": "runtime failed health checks",
  "terminal_at": "2026-03-22T02:10:00Z"
}
```

### Compatibility
- Additive, optional fields only.
- Existing clients ignoring unknown fields continue to function.

## Data Model Changes

### Backend model structs
- Update task contract structs in:
  - internal domain model
  - API public type (`pkg/api/v1/types.go`)
  - API<->model mapping functions

### Persistence
- No storage engine migration required for etcd/memory stores.
- JSON-serialized task documents naturally accept optional new fields.

### Indexing/query changes
- None required for initial scope (list-all then client-side lane partitioning).
- Future optimization (out of scope): server-side filtering for terminal states if task volume becomes large.

## Security Considerations

### Authorization and exposure
- No new endpoint introduced; existing auth boundaries remain.
- New fields (`terminal_reason`) may contain operational error text. Treat as operator-visible only under existing task access policy.

### Input validation and output safety
- Trim and length-bound `terminal_reason` when sourced from loop reason to prevent oversized payload/log amplification.
- Ensure UI escapes reason text (default framework escaping) to avoid XSS when displaying blocked reason.

### Audit integrity
- Preserve audit trail when terminal fields are written/cleared.
- Ensure correlation IDs continue to tie loop transition -> task status sync -> UI-visible outcome.

### Feature-flag safety
- Default flag-off rollout prevents accidental UX change.
- Behavior switch occurs on board reload; no hidden partial state when disabled.

## Testing Strategy

### Backend
- Unit tests for `syncTaskContractStatusForLoop`:
  - sets terminal fields for completed
  - sets terminal fields for blocked with reason
  - clears terminal fields when returning to active state
- API handler tests for additive JSON fields in task responses.

### Frontend
- `feature-flags` tests for `isTasksKanbanEnabled`.
- Tasks page tests:
  - completed/blocked both appear in Finished
  - terminal tasks not duplicated in active lanes
  - mixed terminal badges render correctly
  - Finished count equals completed + blocked
  - flag disabled path does not activate Finished grouping

### End-to-end
- Scenario: run task to completion => appears in Finished as completed.
- Scenario: block task with reason => appears in Finished as blocked with reason visible.
- Scenario: transition terminal task back to active => removed from Finished and count decremented.

## Rollout Plan
- Phase 1: ship additive API/model fields behind no behavior change risk.
- Phase 2: ship Kanban Finished grouping behind `SMITH_FEATURE_TASKS_KANBAN_ENABLED=false` default.
- Phase 3: enable in test environment, validate counts/audit parity, then promote progressively.

## Open Questions
- Should terminal reason be copied verbatim from loop `reason` or normalized to a curated enum + free-text detail?
- Should Finished include additional terminal states in future (e.g., `cancelled`) if task status model expands?
- For very large task sets, should `/tasks` gain server-side pagination/filtering before broad rollout?
