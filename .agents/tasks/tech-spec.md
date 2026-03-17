# Technical Specification: Add Feature Capability

## 1. Scope and Assumptions

### 1.1 Scope
This specification defines a new operator-facing multi-step feature flow that:
- Provides a discoverable entry point.
- Collects and validates required inputs.
- Executes a backend task exactly once.
- Shows in-progress and terminal status.
- Supports safe retry for recoverable failures.

### 1.2 Assumptions
Because the PRD does not name a concrete domain entity, this spec uses neutral names:
- Feature flow key: `feature_capability`
- Execution entity: `feature_run`
- Required input payload: `input_payload` (JSON)
- Operator identity and RBAC infrastructure already exist.
- Existing frontend supports routed pages and authenticated API calls.
- Existing backend supports relational DB and async job execution.

### 1.3 Out of Scope
- Redesign of unrelated operator UX.
- Changes to global auth policy model.
- Fully automated decisioning where operator confirmation is required.

## 2. Architecture Overview

### 2.1 High-Level Design
The feature uses a stateful workflow model with clear separation of concerns:
- Frontend flow UI: step-based wizard, validation display, status and retry actions.
- API layer: authoritative validation, run lifecycle management, idempotent execution trigger.
- Workflow service: state transitions, recoverability classification, retry orchestration.
- Execution worker: performs the intended task asynchronously and emits status updates.
- Persistence layer: stores run metadata, inputs, step progress, attempts, and audit events.

### 2.2 Runtime Flow
1. Operator opens entry point in primary interface.
2. Frontend creates or resumes a draft `feature_run`.
3. Operator fills required fields; client-side and server-side validation gate progression.
4. On final submit, API performs idempotent transition `READY -> EXECUTING`.
5. Worker processes task and updates status (`SUCCEEDED`/`FAILED_RECOVERABLE`/`FAILED_TERMINAL`).
6. Frontend polls or subscribes for status; persists visibility after refresh by reloading run state.
7. If recoverable failure, operator retries; valid prior inputs are reused safely.

### 2.3 State Model
Run-level states:
- `DRAFT`: run initialized, inputs editable.
- `READY`: all required inputs valid, awaiting execute.
- `EXECUTING`: task dispatched/running.
- `SUCCEEDED`: terminal success.
- `FAILED_RECOVERABLE`: terminal for attempt, retry allowed.
- `FAILED_TERMINAL`: terminal, retry disallowed.
- `CANCELLED` (optional future extension, not required in V1).

Attempt-level states:
- `STARTED`, `SUCCEEDED`, `FAILED_RECOVERABLE`, `FAILED_TERMINAL`.

## 3. Detailed Component Design

### 3.1 Frontend Components

#### 3.1.1 Entry Point Integration
- Add a visible navigation/action item in primary operator interface.
- Visibility controlled by permission check (`feature_capability:access`).
- If unauthorized, route to access-denied view with explicit guidance.

#### 3.1.2 Feature Flow Container
Responsibilities:
- Load existing draft run or create new run.
- Render stepper with active/completed/error indicators.
- Persist local unsaved field edits (optional cache) and reconcile with server canonical state.
- Enforce blocking progression if current step validation fails.

#### 3.1.3 Step Forms
- Each step declares:
  - Required fields.
  - Client validation rules (format/presence/basic bounds).
  - Server validation endpoint integration for authoritative checks.
- Field-level errors map to specific controls and include actionable text.

#### 3.1.4 Submission and Status View
- Final action button triggers execute API with idempotency key.
- Show in-progress indicator while state is `EXECUTING`.
- Display terminal success/failure banner and detailed error panel.
- On refresh/reopen, fetch latest run and reconstruct UI from persisted `current_step` + `status`.

#### 3.1.5 Retry UX
- Retry action visible only on `FAILED_RECOVERABLE`.
- Retry confirmation modal summarizes what is reused and what may be revalidated.
- After retry start, UI transitions back to `EXECUTING` and appends attempt history.

### 3.2 Backend Services

#### 3.2.1 Feature Flow API Controller
- Authenticates operator.
- Authorizes access and mutating actions.
- Validates request schema.
- Delegates business logic to workflow service.

#### 3.2.2 Workflow Service
- Owns transition rules and invariants:
  - Only `READY` may execute.
  - Only one active execution per run.
  - Retry only from `FAILED_RECOVERABLE`.
- Applies optimistic concurrency (`version` column) to avoid double transitions.
- Writes audit events for create, step update, validation result, execute, retry, completion.

#### 3.2.3 Validation Service
- Runs domain-level validation of `input_payload`.
- Returns machine-readable violations (`field`, `code`, `message`, `blocking`).
- Distinguishes warnings vs blocking errors (warnings do not block).

#### 3.2.4 Execution Dispatcher + Worker
- Dispatcher enqueues execution job with `run_id`, `attempt_id`, and idempotency token.
- Worker performs exactly-once semantics per attempt token.
- Worker updates attempt and run statuses atomically.
- Worker records structured error classification (`recoverable` vs `terminal`).

### 3.3 Observability
- Structured logs with correlation IDs (`run_id`, `attempt_id`, `request_id`).
- Metrics:
  - `feature_flow_run_created_total`
  - `feature_flow_execute_total{outcome}`
  - `feature_flow_retry_total{outcome}`
  - `feature_flow_step_validation_fail_total{step,field}`
  - `feature_flow_duration_seconds` (from create to terminal)
- Alerting on high terminal failure ratio and execution timeout breaches.

## 4. API Definitions

Base path: `/api/v1/feature-capability`

### 4.1 Create Run
`POST /runs`

Request:
```json
{
  "flow_key": "feature_capability"
}
```

Response `201`:
```json
{
  "run_id": "fr_123",
  "status": "DRAFT",
  "current_step": 1,
  "version": 1,
  "created_at": "2026-03-17T00:00:00Z"
}
```

### 4.2 Get Run
`GET /runs/{run_id}`

Response `200`:
```json
{
  "run_id": "fr_123",
  "status": "EXECUTING",
  "current_step": 3,
  "input_payload": {"fieldA": "value"},
  "validation": {
    "is_valid": true,
    "violations": []
  },
  "attempt_summary": {
    "latest_attempt": 2,
    "max_retries": 3,
    "retry_available": false
  },
  "version": 6,
  "updated_at": "2026-03-17T00:02:00Z"
}
```

### 4.3 Update Step Inputs
`PATCH /runs/{run_id}/steps/{step_number}`

Request:
```json
{
  "fields": {
    "fieldA": "value",
    "fieldB": 10
  },
  "version": 3
}
```

Response `200`:
```json
{
  "run_id": "fr_123",
  "current_step": 2,
  "validation": {
    "is_valid": false,
    "violations": [
      {
        "field": "fieldB",
        "code": "OUT_OF_RANGE",
        "message": "Value must be <= 5",
        "blocking": true
      }
    ]
  },
  "version": 4
}
```

### 4.4 Validate Run (Optional explicit call)
`POST /runs/{run_id}/validate`

Response `200`:
```json
{
  "is_valid": true,
  "violations": []
}
```

### 4.5 Execute Run
`POST /runs/{run_id}/execute`
Headers:
- `Idempotency-Key: <uuid>` (required)

Request:
```json
{
  "version": 5
}
```

Response `202`:
```json
{
  "run_id": "fr_123",
  "status": "EXECUTING",
  "attempt_id": "fa_456",
  "version": 6
}
```

Error cases:
- `409` if run not in executable state or version conflict.
- `422` if validation fails.

### 4.6 Retry Run
`POST /runs/{run_id}/retry`
Headers:
- `Idempotency-Key: <uuid>` (required)

Request:
```json
{
  "reason": "operator_retry",
  "version": 8
}
```

Response `202`:
```json
{
  "run_id": "fr_123",
  "status": "EXECUTING",
  "attempt_id": "fa_789",
  "retry_count": 2,
  "version": 9
}
```

### 4.7 List Attempts / Audit
`GET /runs/{run_id}/attempts`

Response `200`:
```json
{
  "attempts": [
    {
      "attempt_id": "fa_456",
      "status": "FAILED_RECOVERABLE",
      "started_at": "2026-03-17T00:02:00Z",
      "ended_at": "2026-03-17T00:03:00Z",
      "error_code": "UPSTREAM_TIMEOUT"
    }
  ]
}
```

## 5. Data Model Changes

### 5.1 New Tables

#### 5.1.1 `feature_runs`
Columns:
- `id` (PK, string/uuid)
- `flow_key` (string)
- `operator_id` (string, indexed)
- `status` (enum)
- `current_step` (int)
- `input_payload` (jsonb)
- `validation_snapshot` (jsonb)
- `latest_attempt_id` (nullable FK)
- `retry_count` (int default 0)
- `max_retries` (int default 3)
- `version` (int, optimistic lock)
- `created_at`, `updated_at` (timestamps)

Indexes:
- `(operator_id, created_at desc)` for recent runs
- `(status, updated_at)` for operational monitoring

#### 5.1.2 `feature_attempts`
Columns:
- `id` (PK)
- `run_id` (FK -> feature_runs.id, indexed)
- `attempt_number` (int)
- `status` (enum)
- `idempotency_key` (string, unique)
- `error_code` (nullable string)
- `error_detail` (nullable jsonb)
- `started_at`, `ended_at` (timestamps)

Constraints:
- unique `(run_id, attempt_number)`

#### 5.1.3 `feature_run_events`
Columns:
- `id` (PK)
- `run_id` (FK, indexed)
- `attempt_id` (nullable FK)
- `event_type` (string)
- `actor_type` (enum: operator/system)
- `actor_id` (string)
- `event_payload` (jsonb)
- `created_at` (timestamp)

Purpose:
- Audit trail and debugging timeline.

### 5.2 Migration Strategy
1. Add new enums and tables behind feature flag.
2. Deploy API with write/read support for new schema.
3. Enable UI entry point for pilot roles.
4. Backfill not required (new feature only).

## 6. Security Considerations

### 6.1 Authentication and Authorization
- Require authenticated operator session/token on all endpoints.
- Enforce RBAC permission gates:
  - `feature_capability:access`
  - `feature_capability:execute`
  - `feature_capability:retry`
- Return `403` with explicit access-denied payload for unauthorized actions.

### 6.2 Input and Output Security
- Validate all request bodies against strict JSON schemas.
- Reject unknown fields to reduce injection surface.
- Sanitize user-provided strings before rendering back in UI.
- Never expose internal stack traces; return stable error codes.

### 6.3 Integrity and Idempotency
- Require idempotency keys for execute/retry endpoints.
- Enforce optimistic concurrency via `version` to prevent stale overwrites.
- Atomic transaction boundaries for run+attempt state updates.

### 6.4 Audit and Traceability
- Record all state transitions and retry outcomes in immutable event log.
- Include operator identity, timestamp, and transition reason.
- Ensure audit retention matches org compliance policy.

### 6.5 Abuse and Reliability Controls
- Apply rate limits on mutation endpoints per operator.
- Add execution timeout and dead-letter handling for worker jobs.
- Cap retry attempts (`max_retries`) with clear terminal messaging.

## 7. Acceptance Mapping

- US-001: Entry point + access denied in frontend routing and RBAC checks.
- US-002: Required fields + blocking inline validation via step update/validate APIs.
- US-003: Final execute endpoint with idempotency and single-transition guarantees.
- US-004: Persisted run state and status retrieval across refresh/reopen.
- US-005: Recoverable failure classification + retry endpoint + attempt history.

## 8. Open Questions to Resolve Before Build

- Exact role/permission mapping for access, execute, and retry.
- SLO target for end-to-end completion time and timeout thresholds.
- Required audit granularity and retention period.
- Whether status updates should be polling-only (V1) or SSE/WebSocket (V1/V2).
