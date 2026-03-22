# Implementation Plan: Scheduled Lane for Queued Work

## Phase 1: Alignment and Decision Lock-In

### Task 1.1: Confirm scope and acceptance traceability
- Sub-task 1.1.1: Map PRD stories US-001..US-005 to explicit implementation outcomes in this plan.
- Sub-task 1.1.2: Confirm non-goals are preserved (no scheduler/execution-order changes, no new lifecycle states).
- Sub-task 1.1.3: Define done criteria for each phase using measurable checks from success metrics.

### Task 1.2: Resolve/record open-question defaults for MVP
- Sub-task 1.2.1: Adopt default handling for blocked queued tasks (exclude unless canonical queued+non-running signal is present).
- Sub-task 1.2.2: Adopt default Scheduled sorting strategy: priority desc, planned launch asc, created_at asc, id asc.
- Sub-task 1.2.3: Adopt filter/search order: apply filters first, then lane grouping.
- Sub-task 1.2.4: Record defaults in implementation notes to avoid ambiguous behavior during development and QA.

## Phase 2: Feature Gating and Entry-Point Wiring

### Task 2.1: Add runtime feature flag support for Kanban mode
- Sub-task 2.1.1: Add `featureTasksKanbanEnabled` to runtime config parsing/types.
- Sub-task 2.1.2: Add helper/selector for `isTasksKanbanEnabled()` consistent with existing tasks flag patterns.
- Sub-task 2.1.3: Add unit tests for config parsing, defaults, and malformed values.

### Task 2.2: Gate tasks view rendering path
- Sub-task 2.2.1: Update tasks route/view selector to render Kanban only when Tasks and Kanban flags are both enabled.
- Sub-task 2.2.2: Preserve existing tasks list rendering when Tasks is enabled and Kanban is disabled.
- Sub-task 2.2.3: Add route/view tests for all flag combinations.

## Phase 3: Lane Domain Model and Deterministic Mapping

### Task 3.1: Define lane model and mapping module
- Sub-task 3.1.1: Add/extend `LaneID` to include `scheduled`.
- Sub-task 3.1.2: Create mapping module API: `getTaskLane(task)`, `groupTasksByLane(tasks)`, `sortLaneTasks(tasks, strategy)`.
- Sub-task 3.1.3: Keep mapper pure/stateless and derived solely from current task data.

### Task 3.2: Implement canonical Scheduled predicate and normalization path
- Sub-task 3.2.1: Implement predicate `status == "queued" && runtime_state != "running"`.
- Sub-task 3.2.2: Add adapter normalization for payloads missing explicit `runtime_state`.
- Sub-task 3.2.3: Enforce hard exclusions so running and completed tasks cannot map to Scheduled.
- Sub-task 3.2.4: Document inference precedence and fallback rules to keep behavior deterministic.

### Task 3.3: Implement deterministic sorting for Scheduled
- Sub-task 3.3.1: Build sort key composer supporting priority, planned launch date, created_at, id.
- Sub-task 3.3.2: Handle missing metadata with fallback ordering (created_at, then id).
- Sub-task 3.3.3: Guarantee stable sort results for equal-key tasks.

## Phase 4: Kanban Rendering and UX Integration

### Task 4.1: Add Scheduled lane UI in board layout
- Sub-task 4.1.1: Add `Scheduled` lane header and integrate into lane ordering strategy.
- Sub-task 4.1.2: Add lane task-count badge sourced from grouped lane data.
- Sub-task 4.1.3: Ensure lane naming/visual styling is consistent with existing board lanes.

### Task 4.2: Render Scheduled cards using existing task-card interactions
- Sub-task 4.2.1: Reuse existing card component/actions in Scheduled lane to preserve behavior.
- Sub-task 4.2.2: Ensure task-detail open flow works from Scheduled cards.
- Sub-task 4.2.3: Ensure existing planning/priority metadata appears where currently supported on cards.

### Task 4.3: Preserve interaction and viewport behavior
- Sub-task 4.3.1: Keep drag/drop affordances aligned with existing transition validation logic.
- Sub-task 4.3.2: Validate mobile horizontal lane scrolling and sticky headers with Scheduled present.
- Sub-task 4.3.3: Verify no layout regressions at standard operator viewport sizes.

## Phase 5: State Transition Handling and Data Reconciliation

### Task 5.1: Update local state flow for no-reload lane movement
- Sub-task 5.1.1: On local task mutations, update in-memory task collection immediately.
- Sub-task 5.1.2: Re-run grouping/sorting after each local state update.
- Sub-task 5.1.3: Ensure transitions queued(non-running)->running remove task from Scheduled instantly.
- Sub-task 5.1.4: Ensure transitions running->queued(non-running) add task to Scheduled instantly.

### Task 5.2: Add periodic refresh/revalidation behavior
- Sub-task 5.2.1: Poll tasks endpoint every 5s while page is visible.
- Sub-task 5.2.2: Pause polling when tab/window is hidden.
- Sub-task 5.2.3: Trigger revalidation when tab/window regains focus.
- Sub-task 5.2.4: Reconcile server data with local state and re-run deterministic grouping.

## Phase 6: Testing, CI Hooks, and Release Readiness

### Task 6.1: Unit test coverage for mapping and sorting
- Sub-task 6.1.1: Add predicate tests for queued non-running, queued running, completed, and edge payloads.
- Sub-task 6.1.2: Add grouping determinism tests across varied input orderings.
- Sub-task 6.1.3: Add sorting stability tests, including equal-key tie handling.

### Task 6.2: Component/integration test coverage for board behavior
- Sub-task 6.2.1: Verify Scheduled lane renders on initial load when flags are enabled.
- Sub-task 6.2.2: Verify lane header count equals actual Scheduled cards.
- Sub-task 6.2.3: Verify state transitions move cards into/out of Scheduled without reload.
- Sub-task 6.2.4: Verify refresh preserves placement and excludes running/completed from Scheduled.

### Task 6.3: E2E and local CI/hook validation
- Sub-task 6.3.1: Add/adjust acceptance test scenario for Scheduled workflow and refresh behavior.
- Sub-task 6.3.2: Run lint/check and test suites required by project workflow.
- Sub-task 6.3.3: Verify `act` + Docker environment can execute pre-commit and pre-push job sets.
- Sub-task 6.3.4: Execute local CI equivalent (`make ci-local-act`) before merge readiness sign-off.

### Task 6.4: Rollout and fallback validation
- Sub-task 6.4.1: Confirm Kanban-off path continues rendering legacy tasks list view.
- Sub-task 6.4.2: Validate feature-flag toggling behavior is safe and reversible.
- Sub-task 6.4.3: Capture release checklist items for monitoring lane-assignment regressions post-release.
