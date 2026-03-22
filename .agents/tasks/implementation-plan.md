# Implementation Plan: Awaiting Approval Lane for Review Queue

## Phase 1: Confirm Scope, Dependencies, and Rollout Constraints

### Task 1.1: Validate feature boundaries against PRD and technical spec
- Sub-task 1.1.1: Confirm in-scope behavior includes lane visibility gating, validated-task grouping, deterministic ordering, immediate lane updates on decisions, and empty-state rendering.
- Sub-task 1.1.2: Confirm out-of-scope exclusions (no new roles, no bulk actions, no cross-project aggregation, no mobile redesign).
- Sub-task 1.1.3: Record implementation invariants: single-card placement, no task-state model changes, backward compatibility for legacy data.

### Task 1.2: Confirm prerequisite platform dependencies
- Sub-task 1.2.1: Verify runtime feature-flag path supports adding `featureTasksKanbanEnabled`.
- Sub-task 1.2.2: Verify API contract can safely add `review_priority` and `validated_at` without breaking old clients.
- Sub-task 1.2.3: Verify deployment surfaces support propagating `SMITH_FEATURE_TASKS_KANBAN_ENABLED` from Helm values to frontend runtime config.

### Task 1.3: Define release and rollback strategy
- Sub-task 1.3.1: Sequence deployment as backend-first, frontend-second, flag-off by default.
- Sub-task 1.3.2: Define rollback path: disable flag immediately, then revert frontend/backend if needed.
- Sub-task 1.3.3: Define non-prod validation checklist before production enablement.

## Phase 2: Add Feature-Flag Plumbing for Kanban Enablement

### Task 2.1: Extend frontend runtime config model
- Sub-task 2.1.1: Add `featureTasksKanbanEnabled` to frontend runtime config typing.
- Sub-task 2.1.2: Add helper `isTasksKanbanEnabled()` using existing boolean parsing conventions.
- Sub-task 2.1.3: Ensure route behavior requires both `featureTasksEnabled` and `featureTasksKanbanEnabled` for Kanban activation.

### Task 2.2: Propagate flag into runtime configuration artifacts
- Sub-task 2.2.1: Add config template placeholder in `frontend/deploy/runtime-config.template.js`.
- Sub-task 2.2.2: Export flag in `frontend/deploy/30-smith-env.sh`.
- Sub-task 2.2.3: Validate generated runtime config value shape for `true/false/string` compatibility.

### Task 2.3: Wire Helm values to deployment
- Sub-task 2.3.1: Add `console.featureFlags.tasksKanban` to Helm values with default disabled.
- Sub-task 2.3.2: Inject env/config mapping in console ConfigMap and Deployment templates.
- Sub-task 2.3.3: Confirm rendered manifests contain the new flag when set.

## Phase 3: Extend Task API Contract and Model Fields

### Task 3.1: Add review queue metadata to shared API/model types
- Sub-task 3.1.1: Add `ReviewPriority int` to task contract types.
- Sub-task 3.1.2: Add `ValidatedAt time.Time` with omitempty semantics.
- Sub-task 3.1.3: Add `ReviewPriority` to create and patch request types.

### Task 3.2: Implement validation and defaulting rules
- Sub-task 3.2.1: Enforce `review_priority` bounds (agreed range; spec proposes `0..100`) for create/patch.
- Sub-task 3.2.2: Return `400` on out-of-range values with consistent error payload.
- Sub-task 3.2.3: Default absent `review_priority` to `0`.

### Task 3.3: Implement validated timestamp transition semantics
- Sub-task 3.3.1: Set `validated_at=now` on `draft -> validated` transitions when empty.
- Sub-task 3.3.2: Set `validated_at=now` on create when `status=validated` and timestamp absent.
- Sub-task 3.3.3: Preserve `validated_at` on `validated -> draft` rejection and on `validated -> approved` approval.

### Task 3.4: Update persistence and audit surfaces
- Sub-task 3.4.1: Ensure memory and etcd stores serialize/deserialize new fields with legacy compatibility.
- Sub-task 3.4.2: Include queue metadata (`review_priority`, `validated_at`) in audit events where relevant.
- Sub-task 3.4.3: Confirm API responses remain backward-compatible for tasks lacking new fields.

## Phase 4: Build Kanban Lane Composition and Rendering

### Task 4.1: Introduce Kanban lane model in Tasks UI
- Sub-task 4.1.1: Add lane definitions for `draft`, `awaiting_approval`, `approved`, `running`, `completed`, `blocked`.
- Sub-task 4.1.2: Build lane derivation from fetched task list behind Kanban flag guard.
- Sub-task 4.1.3: Preserve existing board/list behavior when Kanban flag is disabled.

### Task 4.2: Implement Awaiting Approval membership rules
- Sub-task 4.2.1: Route all `status=validated` tasks exclusively to `awaiting_approval` lane.
- Sub-task 4.2.2: Exclude validated tasks from all other lanes to maintain single-card placement.
- Sub-task 4.2.3: Ensure non-validated tasks continue to appear in their respective lanes.

### Task 4.3: Implement deterministic ordering for review queue
- Sub-task 4.3.1: Sort `awaiting_approval` by `review_priority` descending.
- Sub-task 4.3.2: Apply secondary sort by `validated_at` ascending (oldest first).
- Sub-task 4.3.3: Apply tertiary tie-breaker by `id` ascending for stable deterministic order.
- Sub-task 4.3.4: Add legacy fallbacks: missing `review_priority => 0`; missing `validated_at => updated_at => created_at`.

### Task 4.4: Implement empty state and lane counts
- Sub-task 4.4.1: Render non-blocking empty-state message when `awaiting_approval` has zero cards and lane is enabled.
- Sub-task 4.4.2: Hide empty-state message when at least one validated task exists.
- Sub-task 4.4.3: Compute lane counts directly from rendered lane card arrays.

## Phase 5: Ensure Post-Decision Queue Freshness

### Task 5.1: Validate approve/reject flow behavior in Kanban mode
- Sub-task 5.1.1: Confirm approve action triggers refresh and removes task from `awaiting_approval`.
- Sub-task 5.1.2: Confirm rejection flow (`validated -> draft`) triggers refresh and removes task from `awaiting_approval`.
- Sub-task 5.1.3: Confirm card appears in correct destination lane after refresh.

### Task 5.2: Protect against UI state drift
- Sub-task 5.2.1: Ensure lane derivation always recomputes from canonical fetched task data.
- Sub-task 5.2.2: Remove/avoid independent counter state that can diverge from rendered cards.
- Sub-task 5.2.3: Verify refresh cycle keeps lane ordering stable for unchanged data.

## Phase 6: Test Coverage and Verification

### Task 6.1: Add frontend unit/component test coverage
- Sub-task 6.1.1: Test feature-flag on/off lane visibility behavior.
- Sub-task 6.1.2: Test validated-task exclusivity in `awaiting_approval`.
- Sub-task 6.1.3: Test deterministic ordering comparator and legacy field fallbacks.
- Sub-task 6.1.4: Test empty-state visibility and non-blocking interaction behavior.

### Task 6.2: Add backend API and transition test coverage
- Sub-task 6.2.1: Test create/patch acceptance of valid `review_priority` values.
- Sub-task 6.2.2: Test rejection of out-of-range `review_priority` with `400`.
- Sub-task 6.2.3: Test `validated_at` set/preserve semantics across create, patch, approve, and rejection transitions.

### Task 6.3: Add end-to-end behavior verification
- Sub-task 6.3.1: Verify validated tasks appear in `awaiting_approval` within one refresh cycle.
- Sub-task 6.3.2: Verify approve/reject actions remove cards from review queue immediately after refresh.
- Sub-task 6.3.3: Verify ordering remains stable across repeated reloads when data unchanged.

## Phase 7: Release Readiness, Observability, and Approval

### Task 7.1: Prepare operational rollout checklist
- Sub-task 7.1.1: Confirm default production value keeps Kanban flag disabled until approval.
- Sub-task 7.1.2: Define staged enablement environments and owners.
- Sub-task 7.1.3: Document smoke tests for flag toggle, lane visibility, ordering, and decision transitions.

### Task 7.2: Validate success metrics instrumentation and monitoring
- Sub-task 7.2.1: Identify telemetry source for median time from board load to first approval action.
- Sub-task 7.2.2: Define method to track percentage of validated tasks surfaced in `awaiting_approval`.
- Sub-task 7.2.3: Establish post-release operator feedback capture for queue clarity metric.

### Task 7.3: Final readiness review for approval
- Sub-task 7.3.1: Confirm acceptance criteria mapping from US-1 through US-5 to test evidence.
- Sub-task 7.3.2: Review known risks (state mapping, ordering disputes, flag misconfiguration) and mitigations.
- Sub-task 7.3.3: Package release note for "Awaiting Approval lane behind `smith.feature_tasks_kanban_enabled`".
