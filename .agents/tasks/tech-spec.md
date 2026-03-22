# Technical Specification: Scheduled Lane for Queued Work

## Scope
Implement a `Scheduled` lane in the feature tasks Kanban experience so operators can see non-running queued tasks ready for refinement/launch planning.

This spec covers:
- Kanban lane architecture and deterministic lane assignment
- Frontend component/module changes
- API contract usage (and optional non-breaking additions)
- Data model/view-model changes
- Security and rollout considerations

This spec does **not** change execution order, scheduler behavior, or introduce new task lifecycle states.

## Architecture Overview

### High-level flow
1. Tasks are loaded from existing tasks API.
2. UI maps each task to a Kanban lane using a pure lane-assignment function.
3. `Scheduled` lane renders tasks where queue status is queued and execution is not running.
4. UI updates lane placement on:
- initial load
- periodic/background refresh
- local task transitions (approve/start/loop status updates)
5. Feature gate controls exposure: Tasks feature must be enabled, and Kanban mode must be enabled.

### Logical layers
- API layer: existing task fetch/mutation endpoints (`/v1/tasks`, `/v1/tasks/{id}`, `/v1/tasks/{id}/approve`, `/v1/loops`).
- Domain mapping layer (new frontend module): deterministic lane assignment and sorting.
- Presentation layer: Kanban board, lane headers (with counts), cards, and existing task detail interactions.

### Deployment/rollout
- Add a dedicated runtime feature flag for Kanban mode under Tasks.
- Keep legacy Tasks list view available as fallback until Kanban is validated.

## Detailed Component Design

### 1. Feature flag and route behavior
- Add `featureTasksKanbanEnabled` to runtime config parsing.
- Gate Kanban rendering behind:
- `isTasksEnabled() === true`
- `isTasksKanbanEnabled() === true`
- If tasks enabled but Kanban disabled, keep existing list UI.

### 2. Lane domain model
Create a frontend-only lane model:
- `Scheduled`
- existing non-scheduled lanes (current board/lifecycle lanes retained)

Add a pure mapper module, e.g. `frontend/src/lib/tasks/kanban-mapping.ts`:
- `getTaskLane(task): LaneID`
- `groupTasksByLane(tasks): Record<LaneID, TaskContract[]>`
- `sortLaneTasks(tasks, strategy)`

### 3. Deterministic Scheduled mapping rule
Primary rule:
- Task is in `Scheduled` when it is queued and not running.

Deterministic predicate (canonical):
- `isScheduled(task) = task.status == "queued" && task.runtime_state != "running"`

If runtime_state is not explicitly provided, normalize from existing status model before mapping (adapter function) and keep this mapping in one place.

Hard exclusions:
- Running tasks must never appear in `Scheduled`.
- Completed tasks must never appear in `Scheduled`.

### 4. Board rendering changes
- Add `Scheduled` lane header with task count badge.
- Render task cards using existing card interaction patterns.
- Preserve current drag/drop affordances; lane drop validation still enforced by existing task transition rules.
- Ensure mobile behavior supports horizontal lane scrolling with sticky lane headers.

### 5. State updates and transitions
To satisfy “no reload” behavior:
- On local actions that change state, update in-memory task collection immediately, then re-run grouping.
- Run background refresh (polling) to pick up external updates and reconcile.
- Keep grouping idempotent and stateless (derived from current tasks array only).

Recommended refresh cadence:
- Poll `/v1/tasks` every 5s while page visible.
- Pause polling when tab hidden; revalidate on focus.

### 6. Sorting and refinement usability
Default `Scheduled` sort strategy (deterministic, PRD-aligned):
1. priority metadata descending (if present)
2. planned launch date ascending (if present)
3. created time ascending
4. stable tiebreaker: task id ascending

If metadata fields are absent, sorting falls through to created time then id.

### 7. Test design
Unit tests (frontend):
- lane predicate covers queued/non-running, running, completed cases
- grouping is deterministic regardless of input order
- sorting stability for equal priorities/timestamps

Component tests:
- `Scheduled` lane renders on initial load
- lane header count matches grouped tasks
- transition running -> queued(non-running) moves into `Scheduled`
- transition queued(non-running) -> running removes from `Scheduled`

E2E/acceptance:
- board refresh preserves placement
- no running/completed tasks in `Scheduled`

## API Definitions

## Existing APIs used
- `GET /v1/tasks` (or `/api/tasks` via frontend API base)
- `PATCH /v1/tasks/{id}`
- `POST /v1/tasks/{id}/approve`
- `POST /v1/loops` (task-to-loop transition path)

No breaking API changes are required if queued/non-running signals are already derivable from task payload.

## Optional non-breaking API enhancement (recommended if ambiguity exists)
Extend task response with a computed board hint:
- `queue_state`: `queued | not_queued`
- `runtime_state`: `running | non_running | completed`

This keeps lane mapping explicit and avoids fragile inference in UI.

## Data Model Changes

### Persistent/backend model
- No required schema/storage changes for MVP.
- Optional: add computed, non-persisted response fields (`queue_state`, `runtime_state`) in API serializer only.

### Frontend model additions
- New `LaneID` enum/type including `scheduled`.
- New derived view model:
- `KanbanTaskView { task, lane, sortKey }`
- Runtime flag parsing support for `featureTasksKanbanEnabled`.

### Migration/backward compatibility
- Backward compatible: if Kanban flag is off, existing Tasks list path remains active.
- If optional API fields are absent, fallback adapter infers lane inputs from existing status model.

## Security Considerations
- Preserve existing authz boundaries for Tasks endpoints; Kanban is a presentation change, not a permission bypass.
- Do not trust client lane assignment for server decisions; lane is UI-only derived state.
- Keep server-side validation authoritative for any drag/drop-triggered mutations.
- Avoid exposing sensitive metadata unintentionally on Scheduled cards; only show fields already approved for task card display.
- Prevent stale-state mistakes by reconciling local optimistic updates with periodic server refresh.
- Auditability remains unchanged: task status transitions continue through existing audited endpoints.

## Open Questions and Proposed Defaults
- Blocked queued tasks: keep out of `Scheduled` unless explicitly marked queued+non-running by canonical API fields.
- Filter/search order: apply filters first, then lane grouping (improves operator mental model for narrowed board scope).
- Scheduled sorting: use priority -> planned launch date -> created_at -> id unless product overrides.

## Acceptance Traceability
- US-001: `Scheduled` lane rendered with header count.
- US-002: deterministic predicate excludes running/completed.
- US-003: local transition handling + polling reconciliation keeps lane current without reload.
- US-004: existing card interactions retained inside Scheduled.
- US-005: unit/component/e2e coverage for mapping and transitions.
