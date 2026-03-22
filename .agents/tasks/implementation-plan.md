# Implementation Plan: In Focus Lane for Active Task Execution

## Phase 1: Discovery, Decisions, and Scope Lock

### Task 1.1: Confirm task status domain and active-state definition
- Sub-task 1.1.1: Verify the canonical task status set across frontend `TaskContract` and backend task contract model.
- Sub-task 1.1.2: Lock active execution states for lane grouping (default: `running` only).
- Sub-task 1.1.3: Document explicit exclusion of non-active statuses from `In Focus`.

### Task 1.2: Resolve open product decisions for first release
- Sub-task 1.2.1: Decide paused-task behavior for future compatibility (default now: no paused status support).
- Sub-task 1.2.2: Confirm `In Focus` card ordering strategy (default: `updated_at` descending).
- Sub-task 1.2.3: Confirm rollout gate strategy (Option A: existing `featureTasksEnabled`; Option B: add `featureTasksKanbanEnabled`).

### Task 1.3: Define acceptance matrix and traceability
- Sub-task 1.3.1: Map PRD stories US-001..US-005 to concrete engineering deliverables.
- Sub-task 1.3.2: Define acceptance scenarios: positive, negative, transition, and regression.
- Sub-task 1.3.3: Create a status-to-lane truth table used by implementation and tests.

## Phase 2: Lane Mapping Domain Layer

### Task 2.1: Create lane mapping types and contract
- Sub-task 2.1.1: Introduce frontend `TaskStatus` alias from `TaskContract['status']`.
- Sub-task 2.1.2: Introduce `TaskLane` union (`backlog | in_focus | blocked | done`).
- Sub-task 2.1.3: Define a single exported mapping API for status-to-lane resolution.

### Task 2.2: Implement deterministic status-to-lane mapping logic
- Sub-task 2.2.1: Map `running -> in_focus`.
- Sub-task 2.2.2: Map `draft | validated | approved -> backlog`.
- Sub-task 2.2.3: Map `blocked -> blocked` and `completed -> done`.
- Sub-task 2.2.4: Add unknown-status fallback to non-`in_focus` lane (`backlog`) with client warning log.
- Sub-task 2.2.5: Enforce mutual exclusivity so each status resolves to exactly one lane.

### Task 2.3: Add unit coverage for mapping exhaustiveness
- Sub-task 2.3.1: Add test cases for all known statuses and expected lane outputs.
- Sub-task 2.3.2: Add negative test proving non-active statuses never resolve to `in_focus`.
- Sub-task 2.3.3: Add unknown-status fallback test and warning-log assertion.

## Phase 3: Board State Derivation and Data Projection

### Task 3.1: Refactor tasks route state into raw and derived layers
- Sub-task 3.1.1: Preserve raw `tasks: TaskContract[]` fetch state.
- Sub-task 3.1.2: Add derived `laneBuckets: Record<TaskLane, TaskCardView[]>`.
- Sub-task 3.1.3: Add per-lane count derivation for deterministic UI and test assertions.

### Task 3.2: Define and build `TaskCardView` projection
- Sub-task 3.2.1: Project required fields (`id`, `status`, `lane`, `objective`, `project_id`, `provider_profile_id`, `updated_at`).
- Sub-task 3.2.2: Derive `current_step` from `metadata.current_step` when present.
- Sub-task 3.2.3: Guarantee projection safety when optional fields are absent.

### Task 3.3: Validate exclusivity and stability of grouping behavior
- Sub-task 3.3.1: Add invariant checks/tests for no card duplication across lanes.
- Sub-task 3.3.2: Add fixture-based assertions for unchanged non-active lane grouping.
- Sub-task 3.3.3: Verify deterministic output ordering within each lane.

## Phase 4: Kanban UI Composition and In Focus Presentation

### Task 4.1: Render explicit lane set including `In Focus`
- Sub-task 4.1.1: Add lane rendering model with order: `In Focus`, `Backlog`, `Blocked`, `Done`.
- Sub-task 4.1.2: Ensure `In Focus` is first-class and visible without horizontal scrolling on desktop where feasible.
- Sub-task 4.1.3: Preserve compatibility with existing task-card actions and status-based controls.

### Task 4.2: Implement empty-state and no-stale-card behavior
- Sub-task 4.2.1: Add explicit empty-state content for an empty `In Focus` lane.
- Sub-task 4.2.2: Ensure lane render source is always current derived state, not cached card fragments.
- Sub-task 4.2.3: Add tests verifying empty `In Focus` shows placeholder and zero cards.

### Task 4.3: Prioritize In Focus card inspection metadata
- Sub-task 4.3.1: Display objective as primary card content.
- Sub-task 4.3.2: Display status and recency indicator derived from `updated_at`.
- Sub-task 4.3.3: Display `current_step` when available.
- Sub-task 4.3.4: Add safe fallbacks: `Step unavailable`, `Update time unavailable`.
- Sub-task 4.3.5: Ensure metadata is rendered as escaped text only.

### Task 4.4: Add component/UI tests for lane and card rendering
- Sub-task 4.4.1: Verify `In Focus` lane presence and lane ordering.
- Sub-task 4.4.2: Verify per-lane card counts for seeded fixtures.
- Sub-task 4.4.3: Verify metadata and fallback rendering cases.
- Sub-task 4.4.4: Verify no duplication across lanes.

## Phase 5: Refresh Cycle and Transition Correctness

### Task 5.1: Preserve and normalize refresh triggers
- Sub-task 5.1.1: Keep manual refresh button behavior.
- Sub-task 5.1.2: Ensure successful task mutations continue to trigger immediate reload.
- Sub-task 5.1.3: Confirm fetch error path preserves previous board state and shows non-blocking error UI.

### Task 5.2: Add polling for automatic lane updates
- Sub-task 5.2.1: Add bounded polling interval (target: every 10 seconds).
- Sub-task 5.2.2: Pause polling when tab is hidden; resume on visibility.
- Sub-task 5.2.3: Ensure polling teardown on route/component unmount.

### Task 5.3: Verify lane transition behavior across refresh cycle
- Sub-task 5.3.1: Test transition non-active -> `running` enters `In Focus` next cycle.
- Sub-task 5.3.2: Test transition `running` -> `completed` exits `In Focus` and enters `Done`.
- Sub-task 5.3.3: Test transition `running` -> `blocked` exits `In Focus` and enters `Blocked`.

## Phase 6: Feature Gating, QA, and Release Readiness

### Task 6.1: Apply and verify feature-flag behavior
- Sub-task 6.1.1: Implement selected rollout option (A or B) consistently in runtime config and route logic.
- Sub-task 6.1.2: Verify no behavior change when tasks feature is disabled.
- Sub-task 6.1.3: Document rollback path via feature flag configuration.

### Task 6.2: Execute regression and quality gates
- Sub-task 6.2.1: Run frontend static/type checks: `npm --prefix frontend run check`.
- Sub-task 6.2.2: Run frontend build validation: `npm --prefix frontend run build`.
- Sub-task 6.2.3: Run backend and shared tests: `go test ./...`.
- Sub-task 6.2.4: Validate lane-count regression suite for non-active statuses.

### Task 6.3: Validate local CI hook workflow alignment
- Sub-task 6.3.1: Confirm Docker availability for local `act` execution.
- Sub-task 6.3.2: Confirm `act` availability for hook jobs (`lint-and-check`, parallel test jobs).
- Sub-task 6.3.3: Run full local CI flow (`make ci-local-act`) when preparing merge candidate.

### Task 6.4: Release notes and operational handoff
- Sub-task 6.4.1: Document new lane behavior and status mapping for operators.
- Sub-task 6.4.2: Document polling cadence and expected refresh behavior.
- Sub-task 6.4.3: Record known limits (only `running` is active in current status domain).
- Sub-task 6.4.4: Capture post-release verification checklist for production smoke test.

## Dependency Order (Execution)
1. Phase 1 -> Phase 2 -> Phase 3 -> Phase 4 -> Phase 5 -> Phase 6
2. US-001 primarily covered by Phases 1-2.
3. US-002 and US-004 primarily covered by Phases 3-4.
4. US-003 primarily covered by Phase 5.
5. US-005 and final acceptance covered by Phases 5-6.
