# Implementation Plan: Finished Lane for Terminal Outcomes

## Phase 1: Discovery, Scope Lock, and Readiness

### Task 1.1: Confirm baseline architecture and impacted components
- Sub-task 1.1.1: Trace task status flow through backend sync logic (`syncTaskContractStatusForLoop`) and identify all status write paths.
- Sub-task 1.1.2: Inventory Task contract type definitions across domain model, API v1 types, and mapper layers.
- Sub-task 1.1.3: Inventory frontend Tasks board composition flow, lane derivation logic, and details panel rendering points.
- Sub-task 1.1.4: Capture current feature-flag runtime parsing behavior and config hydration path (`window.__SMITH_CONFIG__`).

### Task 1.2: Define rollout and non-goals boundaries
- Sub-task 1.2.1: Confirm default-off behavior for `SMITH_FEATURE_TASKS_KANBAN_ENABLED` in all environments.
- Sub-task 1.2.2: Explicitly mark out-of-scope items (server-side terminal filtering/pagination, new terminal states beyond completed/blocked).
- Sub-task 1.2.3: Document backward-compatibility expectation for additive contract fields.

### Task 1.3: Prepare test harness prerequisites
- Sub-task 1.3.1: Ensure frontend dependency setup path is documented for local checks (`npm --prefix frontend install`).
- Sub-task 1.3.2: Validate local CI tooling availability (`act`, Docker) for hook-equivalent validation.
- Sub-task 1.3.3: Confirm test locations for backend unit tests, frontend unit tests, and e2e flows.

## Phase 2: Feature Flag and Runtime Config Wiring

### Task 2.1: Introduce new runtime feature-flag surface
- Sub-task 2.1.1: Add runtime config key `featureTasksKanbanEnabled` to console config schema/loading path.
- Sub-task 2.1.2: Map env var `SMITH_FEATURE_TASKS_KANBAN_ENABLED` to runtime config.
- Sub-task 2.1.3: Wire Helm value `console.featureFlags.tasksKanban` to the env/runtime mapping.
- Sub-task 2.1.4: Set and verify default value as `false` for controlled rollout.

### Task 2.2: Add frontend flag accessor
- Sub-task 2.2.1: Implement `isTasksKanbanEnabled(config = getRuntimeConfig()): boolean` in feature flag utilities.
- Sub-task 2.2.2: Ensure parser handles boolean and string-like truthy/falsy inputs via existing boolean parsing conventions.
- Sub-task 2.2.3: Keep disabled path behavior unchanged (legacy/flat rendering untouched).

### Task 2.3: Validate flag behavior with tests
- Sub-task 2.3.1: Add unit tests for default-off behavior.
- Sub-task 2.3.2: Add unit tests for explicit enabled/disabled overrides from runtime config.
- Sub-task 2.3.3: Add test asserting disabled flag does not activate Finished lane grouping logic.

## Phase 3: Backend Contract and Terminal Metadata Enrichment

### Task 3.1: Extend task contract model types (additive)
- Sub-task 3.1.1: Add optional `terminal_outcome` field (`completed|blocked|''` representation per existing conventions).
- Sub-task 3.1.2: Add optional `terminal_reason` field.
- Sub-task 3.1.3: Add optional `terminal_at` timestamp field (RFC3339-compatible).
- Sub-task 3.1.4: Propagate fields through internal model, API public types, and serialization/mapping code.

### Task 3.2: Implement terminal field state invariants in transition logic
- Sub-task 3.2.1: On transition to `completed`, set status and terminal fields (`terminal_outcome=completed`, clear reason, set terminal_at=now).
- Sub-task 3.2.2: On transition to `blocked`, set status and terminal fields (`terminal_outcome=blocked`, set reason when provided, set terminal_at=now).
- Sub-task 3.2.3: On transition from terminal to active states, clear `terminal_outcome`, `terminal_reason`, and `terminal_at`.
- Sub-task 3.2.4: Apply same invariants for any manual patch/status update path that can alter terminal state.

### Task 3.3: Add input hygiene and audit metadata enrichment
- Sub-task 3.3.1: Trim and length-bound terminal reason data before persistence/logging.
- Sub-task 3.3.2: Extend sync audit metadata with `terminal_outcome`, `terminal_reason_present`, and `terminal_at`.
- Sub-task 3.3.3: Preserve correlation between loop transition events and task sync audit entries.

### Task 3.4: Validate backend behavior with tests
- Sub-task 3.4.1: Add unit test for completed transition terminal field population.
- Sub-task 3.4.2: Add unit test for blocked transition with reason population.
- Sub-task 3.4.3: Add unit test for clearing terminal fields on return-to-active transition.
- Sub-task 3.4.4: Add handler/API response tests confirming additive JSON fields and backward compatibility.

## Phase 4: Kanban Lane Composition and Finished Lane UX

### Task 4.1: Implement canonical lane assignment function
- Sub-task 4.1.1: Define lane mapping for non-terminal states: Draft, Validated, Approved, Running.
- Sub-task 4.1.2: Map `completed` and `blocked` exclusively to Finished when kanban flag is enabled.
- Sub-task 4.1.3: Ensure each task is assigned to exactly one lane (no duplication invariant).
- Sub-task 4.1.4: Add status-based fallback inference when `terminal_outcome` is absent on older records.

### Task 4.2: Implement Finished lane sorting and counting
- Sub-task 4.2.1: Sort Finished lane by `terminal_at desc`.
- Sub-task 4.2.2: Add sort fallback chain: `updated_at desc`, then stable `id`.
- Sub-task 4.2.3: Compute Finished header count directly from assigned Finished lane items.
- Sub-task 4.2.4: Ensure count updates correctly after refresh and status transitions.

### Task 4.3: Render terminal outcome context in board cards and details
- Sub-task 4.3.1: Show clear outcome indicator badge for each Finished item (`Completed` vs `Blocked`).
- Sub-task 4.3.2: Preserve blocked visual distinction in mixed terminal lists.
- Sub-task 4.3.3: In task details, display terminal outcome label for all Finished tasks.
- Sub-task 4.3.4: Display blocked reason when available; show deterministic fallback text when absent.

### Task 4.4: Preserve disabled-state behavior
- Sub-task 4.4.1: Gate all Finished grouping logic behind `isTasksKanbanEnabled`.
- Sub-task 4.4.2: Verify non-kanban rendering path remains functionally unchanged when flag is off.
- Sub-task 4.4.3: Ensure behavior switches correctly on board reload after flag changes.

### Task 4.5: Validate UI behavior with tests
- Sub-task 4.5.1: Add test that completed tasks appear in Finished and not in active lanes.
- Sub-task 4.5.2: Add test that blocked tasks appear in Finished with blocked marking preserved.
- Sub-task 4.5.3: Add test for mixed terminal outcomes with distinct indicators.
- Sub-task 4.5.4: Add test asserting Finished count equals completed + blocked.
- Sub-task 4.5.5: Add invariant test that sum of lane counts equals total tasks.
- Sub-task 4.5.6: Add disabled-flag regression tests confirming no Finished grouping activation.

## Phase 5: End-to-End Validation and Quality Gates

### Task 5.1: Add/extend e2e scenarios for terminal outcomes
- Sub-task 5.1.1: Scenario: task completes and appears in Finished as completed.
- Sub-task 5.1.2: Scenario: task blocks with reason and appears in Finished with reason visible.
- Sub-task 5.1.3: Scenario: terminal task reopens to active and is removed from Finished with decremented count.
- Sub-task 5.1.4: Scenario: mixed terminal set remains selectable/openable without duplication.

### Task 5.2: Validate local CI and hook-equivalent checks
- Sub-task 5.2.1: Run frontend check/build after dependency install path is satisfied.
- Sub-task 5.2.2: Run backend and frontend unit tests relevant to modified areas.
- Sub-task 5.2.3: Run pre-commit equivalent job via `act` (`lint-and-check`).
- Sub-task 5.2.4: Run pre-push equivalent parallel tests via `act` (`go-unit-tests`, `node-unit-tests`, `playwright-tests`).

### Task 5.3: Perform auditability and regression verification
- Sub-task 5.3.1: Verify no Finished omissions/duplications against raw task payload.
- Sub-task 5.3.2: Verify audit metadata entries are emitted for terminal set/clear transitions.
- Sub-task 5.3.3: Verify legacy behavior and existing task views remain unaffected with flag off.

## Phase 6: Progressive Rollout and Operationalization

### Task 6.1: Stage deployment and flag rollout sequence
- Sub-task 6.1.1: Deploy additive backend contract fields first with no UI behavior dependency.
- Sub-task 6.1.2: Deploy UI Finished grouping code with flag defaulted off.
- Sub-task 6.1.3: Enable flag in test/staging environment and validate acceptance criteria.
- Sub-task 6.1.4: Promote flag progressively across environments with monitoring checkpoints.

### Task 6.2: Operational checks and runbook updates
- Sub-task 6.2.1: Document expected lane/count behavior for operators and support.
- Sub-task 6.2.2: Add troubleshooting guidance for missing terminal metadata and stale board refresh states.
- Sub-task 6.2.3: Define rollback action as immediate flag disable if regressions are observed.

### Task 6.3: Final acceptance traceability
- Sub-task 6.3.1: Map each PRD acceptance criterion (US-1 to US-5) to concrete test cases.
- Sub-task 6.3.2: Capture evidence checklist for completed, blocked, count, metadata, and flag-gating behavior.
- Sub-task 6.3.3: Record unresolved open questions (reason normalization, future terminal states, scaling filters) for post-release backlog.
