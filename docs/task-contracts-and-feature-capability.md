# Task Contracts and Feature Capability Flow

This page explains two newer runtime surfaces that landed after the initial MVP docs set:

> **Notice (Gated / Under Development):** Task Contracts (`/tasks`) and Feature Capability (`/feature-capability`) are feature-flagged surfaces and are currently disabled by default.

- Task Contracts (`/tasks`)
- Feature Capability flow (`/feature-capability`)

## Task Contracts

Task Contracts are structured, auditable work items that gate loop execution.

## Why they exist

- establish explicit objective/validation contract before execution,
- allow operator review and approval before a loop starts,
- keep task status synchronized with loop outcomes for traceability.

## Status model

Task status values:

- `draft`
- `validated`
- `approved`
- `running`
- `completed`
- `blocked`

Task patch transitions are intentionally narrow:

- `draft -> validated`
- `validated -> draft`

`approved` is only reachable through the explicit approve endpoint.

## API surface

- `GET /v1/tasks`
- `POST /v1/tasks`
- `GET /v1/tasks/{id}`
- `PATCH /v1/tasks/{id}`
- `POST /v1/tasks/{id}/approve`

Create-loop integration:

- `POST /v1/loops` with `task_contract_id`

Loop create requires the task contract to be in `approved` state.

## Loop-to-task status synchronization

When a loop is bound to a task contract, Smith synchronizes task status on loop lifecycle transitions:

- loop `running` -> task `running`
- loop `synced` -> task `completed`
- loop `flatline` or `cancelled` -> task `blocked`

This synchronization is audit/journal visible and driven by loop state transitions.

## Console workflow

The `/tasks` route supports:

- create task contract,
- edit draft/validated task fields,
- mark validated,
- approve,
- start loop from approved task,
- navigate to resulting pod detail.

## Feature Capability flow

The `/feature-capability` route is a guided 3-step runtime flow:

1. Required inputs
2. Review + submit
3. Execution status and retry handling

The UI persists flow state in local storage so refreshes do not lose in-progress context.

## Runtime contract expected by the UI

The feature-capability UI calls:

- `POST /v1/feature-capability/execute`
- `GET /v1/feature-capability/runs/{run_id}`
- `POST /v1/feature-capability/runs/{run_id}/retry`

Current implementation note (chat-wip baseline):

- The Console route and client contract are implemented.
- `smith-api` does not currently register `/v1/feature-capability/*` handlers.
- In local deployments without an external backing service/proxy for these endpoints, Feature Capability submissions will fail with API errors.

Execution statuses handled by the UI:

- `EXECUTING`
- `SUCCEEDED`
- `FAILED_RECOVERABLE`
- `FAILED_TERMINAL`

Retry is only offered when status is `FAILED_RECOVERABLE` and `retry_available=true`.

## Access gating

Feature Capability is permission-gated in Console navigation.

Runtime config keys used:

- `featureCapabilityAccess` (boolean-like override)
- `operatorPermissions` / `permissions` (must include `feature_capability:access` when permissions are present)

When access is denied, the route redirects to `/feature-capability/access-denied`.

## Validation commands

Useful focused checks for these surfaces:

```bash
go test ./cmd/smith-api -run ExecutionFlow
npm --prefix frontend run test:unit -- src/lib/feature-capability/access.test.ts src/lib/feature-capability/execution.test.ts src/lib/feature-capability/validation.test.ts src/lib/navigation.test.ts
```
