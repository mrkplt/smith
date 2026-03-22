# Implementation Plan: Feature-gated Tasks Kanban Visibility

## Phase 1: Discovery and Alignment

### Task 1.1: Confirm implementation boundaries and invariants
#### Sub-task 1.1.1
Map PRD goals, rules, and non-goals to concrete engineering constraints for this change set.
#### Sub-task 1.1.2
Confirm the single evaluation path requirement (`isTasksKanbanEnabled`) and fail-closed default (`false`).
#### Sub-task 1.1.3
Confirm route fallback target (`/tasks`) and expected UX behavior for blocked direct access.

### Task 1.2: Inventory existing Tasks/Kanban touchpoints
#### Sub-task 1.2.1
Identify all frontend entry points that navigate to Kanban (sidebar, tabs, buttons, CTA links).
#### Sub-task 1.2.2
Identify the canonical Tasks/Kanban route definitions and route guard insertion points.
#### Sub-task 1.2.3
Identify existing feature-flag parsing patterns to reuse and avoid duplicate logic.

### Task 1.3: Define execution order and dependency map
#### Sub-task 1.3.1
Sequence work so flag contract/plumbing lands before UI gating and route protection.
#### Sub-task 1.3.2
Establish explicit dependency links: US-001 as prerequisite for US-002, US-003, and US-004.
#### Sub-task 1.3.3
Define test stages (unit -> component/route -> e2e -> regression checks).

## Phase 2: Feature Flag Contract and Configuration Plumbing (US-001)

### Task 2.1: Add Helm values and schema support for Tasks Kanban flag
#### Sub-task 2.1.1
Add `console.featureFlags.tasksKanban` with default `false` in `helm/smith/values.yaml`.
#### Sub-task 2.1.2
Add schema validation for `console.featureFlags.tasksKanban` as boolean in `helm/smith/values.schema.json`.
#### Sub-task 2.1.3
Validate that absence of value still resolves to disabled behavior by default.

### Task 2.2: Inject runtime env var from console ConfigMap
#### Sub-task 2.2.1
Map Helm value to `SMITH_FEATURE_TASKS_KANBAN_ENABLED` in `helm/smith/templates/console-configmap.yaml`.
#### Sub-task 2.2.2
Ensure consistent naming and no collisions with existing feature env vars.
#### Sub-task 2.2.3
Verify rendered manifests include the new env var in disabled and enabled value scenarios.

### Task 2.3: Expose runtime config key in frontend bootstrap
#### Sub-task 2.3.1
Add `featureTasksKanbanEnabled` mapping in `frontend/deploy/runtime-config.template.js`.
#### Sub-task 2.3.2
Ensure runtime key naming exactly matches technical spec contract.
#### Sub-task 2.3.3
Verify missing env var produces undefined input to evaluator (not implicit true-like value).

### Task 2.4: Implement canonical evaluator and parse rules
#### Sub-task 2.4.1
Add `isTasksKanbanEnabled(config)` in `frontend/src/lib/feature-flags.ts`.
#### Sub-task 2.4.2
Implement boolean + boolean-like string parsing (`true/false`, `1/0`, `yes/no`).
#### Sub-task 2.4.3
Ensure invalid/missing values fail closed to `false`.
#### Sub-task 2.4.4
Add/confirm optional composition helper `isTasksKanbanVisible(config) = isTasksEnabled(config) && isTasksKanbanEnabled(config)`.

## Phase 3: UI Entry-Point Gating (US-002)

### Task 3.1: Apply gate at Tasks navigation surfaces
#### Sub-task 3.1.1
Gate sidebar/global navigation Kanban links with `isTasksKanbanVisible`.
#### Sub-task 3.1.2
Gate Tasks page local navigation/tabs/buttons leading to Kanban.
#### Sub-task 3.1.3
Gate secondary CTA links to prevent indirect access affordances.

### Task 3.2: Centralize gate evaluation in container-level logic
#### Sub-task 3.2.1
Compute visibility once per relevant page/layout boundary.
#### Sub-task 3.2.2
Pass derived boolean flags into presentational components via props/state.
#### Sub-task 3.2.3
Remove any duplicated inline parsing or ad hoc flag reads in child components.

### Task 3.3: Ensure reload and state consistency
#### Sub-task 3.3.1
Confirm rendered entry-point state derives from runtime config on each app load.
#### Sub-task 3.3.2
Confirm no persisted UI cache/local storage restores hidden Kanban affordances.
#### Sub-task 3.3.3
Validate toggled environments reflect expected visibility after reload.

## Phase 4: Route Access Protection (US-003)

### Task 4.1: Add route guard for `/tasks/kanban`
#### Sub-task 4.1.1
Implement route-level guard at Kanban boundary (or parent tasks layout if architecture requires).
#### Sub-task 4.1.2
Call canonical evaluator only; avoid direct raw config checks.
#### Sub-task 4.1.3
Block render path when disabled before Kanban content mounts.

### Task 4.2: Implement disabled-state redirect and user notice
#### Sub-task 4.2.1
Redirect disabled direct access from `/tasks/kanban` to `/tasks`.
#### Sub-task 4.2.2
Emit muted/non-disruptive user notice for blocked access (per existing UX patterns).
#### Sub-task 4.2.3
Ensure enabled state loads Kanban route normally without extra redirects.

### Task 4.3: Validate navigation-path enforcement
#### Sub-task 4.3.1
Verify direct URL entry cannot render Kanban when disabled.
#### Sub-task 4.3.2
Verify browser back/forward cannot bypass guard.
#### Sub-task 4.3.3
Verify guard preserves existing authorization boundaries and does not alter auth logic.

## Phase 5: Diagnostics and Documentation (US-004)

### Task 5.1: Add non-sensitive diagnostic for feature evaluation state
#### Sub-task 5.1.1
Emit structured diagnostic on startup or first evaluation with `feature=tasks-kanban enabled=<bool> source=runtime-config`.
#### Sub-task 5.1.2
Ensure diagnostic includes no secrets, tokens, or full config dumps.
#### Sub-task 5.1.3
Ensure missing/invalid config still emits a clear disabled-state signal.

### Task 5.2: Add blocked-route diagnostic event
#### Sub-task 5.2.1
Emit structured event for route block with action, redirect target, and disabled reason.
#### Sub-task 5.2.2
Deduplicate noisy repeated logs where necessary using existing logging conventions.
#### Sub-task 5.2.3
Validate logs are actionable for operators troubleshooting visibility behavior.

### Task 5.3: Update feature-flag and env-var documentation
#### Sub-task 5.3.1
Document new flag in `docs/feature-flags.md` (key, default, valid values, behavior).
#### Sub-task 5.3.2
Document env var in `docs/environment-variables.md`.
#### Sub-task 5.3.3
Add rollout note describing enable path via `console.featureFlags.tasksKanban=true`.

## Phase 6: Automated Test Coverage and Quality Gates (US-005)

### Task 6.1: Add unit coverage for evaluator and helper logic
#### Sub-task 6.1.1
Test default false for missing value.
#### Sub-task 6.1.2
Test parsing for boolean and accepted string variants.
#### Sub-task 6.1.3
Test invalid values fail closed to false.
#### Sub-task 6.1.4
Test `isTasksKanbanVisible` composition with base Tasks feature gate.

### Task 6.2: Add component/route tests for gated visibility
#### Sub-task 6.2.1
Assert Kanban entry points are absent when flag disabled.
#### Sub-task 6.2.2
Assert Kanban entry points are present when flag enabled.
#### Sub-task 6.2.3
Assert direct `/tasks/kanban` redirects when disabled.
#### Sub-task 6.2.4
Assert direct `/tasks/kanban` renders when enabled.
#### Sub-task 6.2.5
Assert back/forward navigation cannot bypass disabled guard.

### Task 6.3: Add Playwright E2E scenarios for both flag profiles
#### Sub-task 6.3.1
Create disabled-profile scenario: no affordance + direct-route redirect.
#### Sub-task 6.3.2
Create enabled-profile scenario: affordance visible + route accessible.
#### Sub-task 6.3.3
Stabilize fixtures/configuration to avoid flaky flag-state assumptions.

### Task 6.4: Execute required quality gates and regression checks
#### Sub-task 6.4.1
Run `go test ./...`.
#### Sub-task 6.4.2
Run `npm --prefix frontend run check`.
#### Sub-task 6.4.3
Run `npm --prefix frontend run build`.
#### Sub-task 6.4.4
Confirm existing `/tasks` behavior and `SMITH_FEATURE_TASKS_ENABLED` semantics are unchanged.

## Phase 7: Release Readiness and Rollout

### Task 7.1: Pre-release verification checklist
#### Sub-task 7.1.1
Verify default deployment keeps Kanban fully hidden and inaccessible.
#### Sub-task 7.1.2
Verify explicit enablement exposes navigation and route as expected.
#### Sub-task 7.1.3
Verify diagnostics appear for both enabled and disabled scenarios.

### Task 7.2: Environment rollout steps
#### Sub-task 7.2.1
Deploy with default disabled to local/staging and validate behavior.
#### Sub-task 7.2.2
Enable `console.featureFlags.tasksKanban=true` in selected environment.
#### Sub-task 7.2.3
Re-validate UI, route protection, and diagnostics post-enable.

### Task 7.3: Operational handoff
#### Sub-task 7.3.1
Capture final acceptance evidence mapped to US-001 through US-005.
#### Sub-task 7.3.2
Record rollback approach (set flag false and redeploy config).
#### Sub-task 7.3.3
Publish release note with exact flag key/env var and expected default behavior.
