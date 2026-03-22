# Technical Specification: In Focus Lane for Active Task Execution

## 1. Scope and Objectives

Implement an `In Focus` Kanban lane in the Tasks board so operators can immediately identify actively executing work.

This spec covers:
- deterministic status-to-lane assignment,
- Tasks board rendering updates,
- live lane membership updates within refresh cadence,
- non-regression for existing non-running task grouping.

Out of scope:
- changing task execution semantics,
- adding new task statuses,
- backend scheduler behavior changes.

## 2. Architecture Overview

### 2.1 Current System Context
- Frontend Tasks route: `frontend/src/routes/tasks/+page.svelte`.
- Task API contract: `frontend/src/lib/api.ts` (`TaskContract`).
- Task status source of truth: backend model in `internal/source/model/task_contract.go`.
- Existing status domain: `draft`, `validated`, `approved`, `running`, `completed`, `blocked`.

### 2.2 Proposed Architecture
- Keep backend APIs and persistence unchanged.
- Introduce frontend board-domain mapping layer that derives a lane from task status.
- Render board as explicit lanes, including new `In Focus` lane.
- Keep board state synchronized via existing refresh mechanism plus periodic polling (single refresh cycle target).

High-level flow:
1. `GET /tasks` (proxied to `/v1/tasks`) returns `TaskContract[]`.
2. Frontend maps each task status to a lane.
3. UI groups tasks by lane and renders lane columns.
4. On refresh cycle / action completion, tasks are re-fetched and cards move lanes deterministically.

## 3. Detailed Component Design

### 3.1 Lane Mapping Module

Create a dedicated mapping utility (frontend domain layer) to avoid inline lane logic in UI markup.

Proposed types:
- `TaskStatus = TaskContract['status']`
- `TaskLane = 'backlog' | 'in_focus' | 'blocked' | 'done'`

Deterministic mapping:
- `running -> in_focus`
- `draft | validated | approved -> backlog`
- `blocked -> blocked`
- `completed -> done`

Rules:
- Each status maps to exactly one lane (mutual exclusivity).
- Unknown/future statuses default to non-`in_focus` fallback lane (`backlog`) and emit a client warning log.
- `In Focus` contains active execution statuses only; for current domain this is `running` only.

### 3.2 Tasks Board State Controller

Refactor route-level state handling into board-oriented derived state:
- raw state: `tasks: TaskContract[]`
- derived state: `laneBuckets: Record<TaskLane, TaskCardView[]>`
- computed counters per lane for testability.

`TaskCardView` (frontend-only projection):
- `id`
- `status`
- `lane`
- `objective`
- `project_id`
- `provider_profile_id`
- `updated_at`
- `current_step` (derived from `metadata.current_step` when present)

### 3.3 Kanban UI Composition

Implement first-class lane rendering in `/tasks` page (or split into components):
- `In Focus` lane visually emphasized and positioned before non-active lanes on desktop.
- Lane order:
  1. `In Focus`
  2. `Backlog`
  3. `Blocked`
  4. `Done`

Lane behavior:
- Empty lane renders explicit empty-state placeholder.
- Cards never duplicated across lanes.
- Existing task actions (approve, validate, reopen, start loop, edit) remain available based on status.

### 3.4 In Focus Card Metadata Prioritization

Show execution metadata needed for quick inspection using existing payload fields only:
- primary: objective
- secondary: status + relative recency (from `updated_at`)
- execution detail: `metadata.current_step` if available
- fallback labels when data absent:
  - current step: `Step unavailable`
  - timestamp: `Update time unavailable`

No backend field additions are required.

### 3.5 Refresh and Lane Transition Behavior

To satisfy “within one board refresh cycle” and avoid manual reload:
- retain explicit Refresh button behavior,
- add lightweight polling (e.g., every 10s, paused on hidden tab) to call `loadTasks()`.

Transition expectations:
- non-active -> running => card appears in `In Focus` next cycle.
- running -> completed/blocked => card exits `In Focus` and appears in mapped lane next cycle.
- local actions continue to trigger immediate `loadTasks()` after successful mutation.

### 3.6 Feature Flag and Rollout

Because `/tasks` is already gated by `featureTasksEnabled`, two rollout options are supported:
- Option A (default): ship Kanban/In Focus under existing `featureTasksEnabled` gate.
- Option B (optional): add `featureTasksKanbanEnabled` runtime flag for phased rollout.

Recommendation: Option A unless staged deployment risk requires a second gate.

## 4. API Definitions

No new backend endpoints are required.

### 4.1 Read API (existing)
- `GET /v1/tasks` (frontend currently calls `/tasks` via API base/proxy).
- Response: array of `TaskContract`.
- Required fields for lane behavior: `id`, `status`.
- Required fields for In Focus metadata: `objective`, `updated_at`, `metadata` (optional).

### 4.2 Mutation APIs (existing, unchanged)
- `PATCH /v1/tasks/{id}` for status/objective updates.
- `POST /v1/tasks/{id}/approve`.
- `POST /v1/loops` with `task_contract_id` (drives `running` transition indirectly).

### 4.3 Error Handling Contract
- If tasks fetch fails, board preserves previous rendered state and shows non-blocking error banner/toast.
- Lane derivation must never throw on missing optional fields.

## 5. Data Model Changes

### 5.1 Backend Persistence
- No database schema changes.
- No new server-side task status values.
- No task contract API schema changes.

### 5.2 Frontend View Model
Add frontend-only derived model:
- `lane` (derived, not persisted).
- `TaskCardView` projection for rendering and metadata fallback handling.

If Option B flag is chosen:
- add runtime config key `featureTasksKanbanEnabled` (boolean-like parse behavior aligned with existing flags).

## 6. Security Considerations

- Authorization remains enforced by existing Tasks feature gate and server auth; no privilege model expansion.
- Do not trust `metadata` content as HTML; render as escaped text only.
- Avoid exposing hidden data in empty-state or error messages (no raw API payload dumps to UI).
- Polling interval must be bounded to prevent accidental client-side request amplification.
- Ensure board updates are read-only projections; no client-side status mutation outside existing authenticated APIs.

## 7. Testing and Quality Gates

### 7.1 Unit Tests
- Status-to-lane mapping exhaustiveness across all known statuses.
- Unknown status fallback behavior (non-`in_focus`).
- Card metadata fallback rendering.

### 7.2 UI/Component Tests
- `In Focus` lane presence and ordering.
- Card counts per lane for seeded fixtures.
- Empty-state rendering when no active tasks.
- No duplication across lanes.

### 7.3 Integration Tests
- Transition `queued-equivalent(non-active)` -> `running` reflected in `In Focus` next refresh cycle.
- Transition `running` -> `completed`/`blocked` exits `In Focus` and enters correct lane.
- Regression suite verifies unchanged non-active lane assignments.

### 7.4 Required Gates
- `npm --prefix frontend run check`
- `npm --prefix frontend run build`
- `go test ./...`

## 8. Open Questions and Default Decisions

1. Should paused tasks move to In Focus?
- Default: No. Current domain has no `paused`; only `running` is active.

2. In Focus ordering basis?
- Default: `updated_at` descending (most recently active first), as this optimizes active inspection.

These defaults can be changed later without backend API/schema changes.
