# Technical Specification: Feature-gated Tasks Kanban Visibility

## 1. Overview

### 1.1 Objective
Implement a dedicated feature gate that controls visibility and access to the **Tasks Kanban** experience. The gate must default to disabled, hide all Kanban entry points, and block direct route access when disabled.

### 1.2 Scope
- Add a new **console runtime feature flag** for Tasks Kanban.
- Gate Kanban navigation/entry points and direct route access using one canonical evaluator.
- Surface clear non-sensitive diagnostics of enabled/disabled evaluation.
- Add automated coverage for both enabled and disabled states.

### 1.3 Out of Scope
- Redesigning Kanban UX or task workflows.
- API or persistence changes for task entities.
- User-targeted rollout/segmentation.

## 2. Architecture Overview

Kanban visibility will follow the existing console feature-flag flow:

1. Helm values (`console.featureFlags.tasksKanban`) provide desired state.
2. Console ConfigMap injects env var `SMITH_FEATURE_TASKS_KANBAN_ENABLED`.
3. Frontend runtime config template maps env var to `window.__SMITH_CONFIG__.featureTasksKanbanEnabled`.
4. Frontend canonical evaluator (`isTasksKanbanEnabled`) parses value and applies default `false`.
5. UI entry points and route guards consume the evaluator to enforce behavior.

```
Helm values.yaml
  -> console-config ConfigMap env
    -> runtime-config.js (__SMITH_CONFIG__)
      -> feature-flags.ts (canonical parser/evaluator)
        -> Sidebar/Tasks UI entry points + /tasks/kanban route guard
```

### 2.1 Design Principle: Single Evaluation Path
All Kanban checks must call one function (`isTasksKanbanEnabled`) to satisfy deterministic behavior and avoid split logic.

## 3. Detailed Component Design

## 3.1 Feature Flag Contract (US-001)

### New flag contract
- Runtime config key: `featureTasksKanbanEnabled`
- Env var: `SMITH_FEATURE_TASKS_KANBAN_ENABLED`
- Helm value: `console.featureFlags.tasksKanban`
- Default: `false`
- Allowed values at runtime: boolean or boolean-like strings (`true/false`, `1/0`, `yes/no`)
- Invalid/missing values: resolve to `false`

### Files impacted
- `frontend/src/lib/feature-flags.ts`
- `frontend/deploy/runtime-config.template.js`
- `helm/smith/templates/console-configmap.yaml`
- `helm/smith/values.yaml`
- `helm/smith/values.schema.json`
- `docs/feature-flags.md`
- `docs/environment-variables.md`

### Evaluator rules
- `isTasksKanbanEnabled(config)` returns parsed value with fallback `false`.
- Optional helper: `isTasksKanbanVisible(config) = isTasksEnabled(config) && isTasksKanbanEnabled(config)`
  - Prevents Kanban exposure if base Tasks surface is disabled.

## 3.2 UI Entry Point Gating (US-002)

### Entry points to gate
All links/buttons/tabs that navigate to Kanban (for example inside Tasks page header, sub-navigation, or CTA links) must be conditionally rendered only when `isTasksKanbanVisible()` is true.

### Behavior
- Disabled: no Kanban UI affordance rendered.
- Enabled: Kanban affordances rendered normally.
- Reload correctness: initial state is derived from runtime config on load; no persistent client cache should override runtime config.

### Component strategy
- Keep gate logic in container-level page/component state and pass booleans into presentational components.
- Avoid duplicating parsing logic in individual components.

## 3.3 Route Access Protection (US-003)

### Route target
Kanban route is assumed as `/tasks/kanban` (or equivalent dedicated Kanban route).

### Guard behavior
- On route entry, evaluate canonical gate before rendering Kanban content.
- Disabled: redirect to safe fallback `/tasks` and emit user-facing muted toast (or equivalent non-disruptive notice).
- Enabled: render normally.
- Browser back/forward: route-level guard executes on navigation, preserving block behavior.

### Guard placement
- Preferred: route-level load/guard in tasks-kanban route boundary.
- Alternative: parent tasks layout guard if Kanban is nested and easier to centralize.

## 3.4 Operational Diagnostics (US-004)

### Requirement
Expose flag state via existing diagnostics without sensitive leakage.

### Proposed diagnostics
- Emit a structured, non-sensitive console diagnostic at app startup or first gate evaluation:
  - `feature=tasks-kanban enabled=true|false source=runtime-config`
- Emit explicit diagnostic on blocked direct route access:
  - `feature=tasks-kanban action=route-block redirect=/tasks reason=disabled`

### Constraints
- Do not log tokens, permissions, or raw config payloads.
- Boolean state and route/action metadata only.

## 3.5 Automated Coverage (US-005)

### Unit tests
- `feature-flags` tests:
  - default false when missing
  - true/false parsing for boolean and string values
  - invalid value falls back false
- Any helper visibility function tests (`isTasksKanbanVisible`).

### Component/route tests
- Navigation/entry-point tests:
  - Kanban entry hidden when disabled
  - visible when enabled
- Route guard tests:
  - direct `/tasks/kanban` redirects to `/tasks` when disabled
  - renders when enabled
  - back/forward cannot bypass guard

### E2E (Playwright)
- Disabled profile: no Kanban affordance + direct route redirects.
- Enabled profile: affordance visible + route accessible.

### Regression focus
- Existing `/tasks` behavior remains unchanged.
- Existing `SMITH_FEATURE_TASKS_ENABLED` gate semantics remain unchanged.

## 4. API Definitions

No backend API contract changes are required.

### Runtime config contract update (frontend-facing)
`window.__SMITH_CONFIG__` gains:

- `featureTasksKanbanEnabled?: boolean | string`

This is configuration surface expansion, not a server API shape change.

## 5. Data Model Changes

No datastore schema or task contract model changes are required.

### Configuration model changes
- Helm values add `console.featureFlags.tasksKanban: false`.
- Helm JSON schema includes `tasksKanban` as a boolean property under `console.featureFlags`.

## 6. Security Considerations

- Feature flags are **visibility/UX gates**, not authorization boundaries.
- Existing task authorization and API protections remain authoritative.
- Route block prevents accidental exposure via deep links but does not replace backend access control.
- Diagnostics must remain non-sensitive:
  - no secrets
  - no operator token values
  - no full runtime config dumps
- Invalid flag values fail closed to disabled.

## 7. Rollout Plan

1. Ship flag plumbing with default disabled.
2. Validate disabled behavior in local/staging.
3. Enable in selected environment by setting `console.featureFlags.tasksKanban=true`.
4. Validate enabled behavior and monitor diagnostics.
5. Update docs and release notes with exact toggle path.

## 8. Acceptance Criteria Traceability

- US-001: Section 3.1
- US-002: Section 3.2
- US-003: Section 3.3
- US-004: Section 3.4
- US-005: Section 3.5
