# Technical Specification: Add Feature Capability

## 1. Architecture Overview

### 1.1 Objective
Implement an operator-facing, stateful feature flow that supports:
- Discoverable entry from the primary UI.
- Step-based input collection with inline validation.
- Exactly-once task execution from final submission.
- Durable progress and terminal status visibility across refresh/reopen.
- Safe retry after recoverable failures with traceable outcomes.

### 1.2 System Context
The solution is built as a workflow slice across existing frontend and backend platforms:
- Frontend app renders the multi-step flow and status UI.
- Backend API controls workflow lifecycle, validation, execution triggers, and retries.
- Async worker executes the intended task and reports outcomes.
- Relational DB stores run state, attempts, and audit events.

### 1.3 High-Level Runtime Sequence
1. Operator opens the feature flow entry point.
2. UI creates or resumes a `feature_run` in `DRAFT` state.
3. Operator updates step inputs; API validates and gates progression.
4. Final submit calls execute endpoint with idempotency key.
5. API transitions run to `EXECUTING` and dispatches a worker attempt.
6. Worker completes with `SUCCEEDED`, `FAILED_RECOVERABLE`, or `FAILED_TERMINAL`.
7. UI reads status and attempt history; retry action appears only for recoverable failures.

### 1.4 Workflow State Model
Run states:
- `DRAFT`
- `READY`
- `EXECUTING`
- `SUCCEEDED`
- `FAILED_RECOVERABLE`
- `FAILED_TERMINAL`

Attempt states:
- `STARTED`
- `SUCCEEDED`
- `FAILED_RECOVERABLE`
- `FAILED_TERMINAL`

Key invariants:
- Only `READY` can transition to `EXECUTING`.
- Only one active attempt per run.
- Retry allowed only from `FAILED_RECOVERABLE` and within retry limit.

## 2. Detailed Component Design

### 2.1 Frontend Components

#### 2.1.1 Entry Point and Access Control
- Add a visible feature entry action in the operator primary interface.
- Visibility can be role-aware, but backend remains the source of truth for authorization.
- Unauthorized opens show explicit access-denied state.

#### 2.1.2 Flow Shell
- Responsibilities:
  - Initialize or resume run.
  - Render stepper (current/completed/error states).
  - Coordinate navigation (`Next`, `Back`, `Submit`, `Retry`).
  - Restore state after reload using server `run_id` fetch.

#### 2.1.3 Step Form Modules
- Each step defines:
  - Required fields.
  - Local validation rules (presence/format/range basics).
  - Server validation mapping to field-level messages.
- Progression is blocked on server-declared blocking violations.

#### 2.1.4 Submission and Status Module
- Final submit triggers execution call with idempotency key.
- While `EXECUTING`, show deterministic progress indicator and disable duplicate submits.
- Terminal state UI:
  - Success confirmation for `SUCCEEDED`.
  - Actionable error panel for `FAILED_RECOVERABLE`/`FAILED_TERMINAL`.

#### 2.1.5 Retry Module
- Enabled only for `FAILED_RECOVERABLE`.
- Reuses previously valid input snapshot by default.
- Creates a new attempt record while preserving run history.

### 2.2 Backend Components

#### 2.2.1 Feature Flow API Controller
- Handles authn/authz, request schema validation, and HTTP contract mapping.
- Delegates domain logic to Workflow Service.

#### 2.2.2 Workflow Service
- Central owner of state transitions and guardrails.
- Enforces optimistic concurrency via `version` field.
- Coordinates run updates, attempt creation, and event emission in transactions.

#### 2.2.3 Validation Service
- Performs authoritative validation over run inputs.
- Returns normalized violation payload:
  - `field`
  - `code`
  - `message`
  - `blocking`

#### 2.2.4 Execution Dispatcher and Worker
- Dispatcher enqueues attempt job with `run_id`, `attempt_id`, and idempotency key.
- Worker executes task idempotently and writes terminal outcome.
- Failure classifier sets recoverable vs terminal category.

#### 2.2.5 Audit/Event Logging
- Immutable transition events recorded for create/update/validate/execute/retry/complete.
- Event payload carries actor identity and relevant context.

### 2.3 Cross-Cutting Concerns
- Observability:
  - Logs with `run_id`, `attempt_id`, `request_id` correlation.
  - Metrics: flow starts, validation failures, execute outcomes, retry outcomes, completion latency.
- Resilience:
  - Execution timeout handling.
  - Dead-letter or error queue for worker failures.

## 3. API Definitions

Base path: `/api/v1/feature-capability`

### 3.1 Create Run
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

### 3.2 Get Run
`GET /runs/{run_id}`

Response `200`:
```json
{
  "run_id": "fr_123",
  "status": "DRAFT",
  "current_step": 2,
  "input_payload": {
    "fieldA": "value"
  },
  "validation": {
    "is_valid": false,
    "violations": [
      {
        "field": "fieldB",
        "code": "REQUIRED",
        "message": "fieldB is required",
        "blocking": true
      }
    ]
  },
  "attempt_summary": {
    "latest_attempt": null,
    "retry_count": 0,
    "max_retries": 3,
    "retry_available": false
  },
  "version": 3,
  "updated_at": "2026-03-17T00:01:00Z"
}
```

### 3.3 Update Step Inputs
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
    "is_valid": true,
    "violations": []
  },
  "version": 4
}
```

Errors:
- `409` version conflict.
- `422` validation payload malformed.

### 3.4 Validate Run
`POST /runs/{run_id}/validate`

Response `200`:
```json
{
  "is_valid": true,
  "violations": []
}
```

### 3.5 Execute Run
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

Errors:
- `403` access denied.
- `409` invalid state or stale version.
- `422` run not valid for execution.

### 3.6 Retry Run
`POST /runs/{run_id}/retry`

Headers:
- `Idempotency-Key: <uuid>` (required)

Request:
```json
{
  "version": 8,
  "reason": "operator_retry"
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

Errors:
- `403` access denied.
- `409` not retryable or stale version.
- `429` retry limit reached.

### 3.7 List Attempts
`GET /runs/{run_id}/attempts`

Response `200`:
```json
{
  "attempts": [
    {
      "attempt_id": "fa_456",
      "attempt_number": 1,
      "status": "FAILED_RECOVERABLE",
      "error_code": "UPSTREAM_TIMEOUT",
      "started_at": "2026-03-17T00:02:00Z",
      "ended_at": "2026-03-17T00:03:00Z"
    }
  ]
}
```

## 4. Data Model Changes

### 4.1 New Table: `feature_runs`
Fields:
- `id` (PK, UUID/text)
- `flow_key` (text)
- `operator_id` (text, indexed)
- `status` (enum)
- `current_step` (int)
- `input_payload` (jsonb)
- `validation_snapshot` (jsonb)
- `latest_attempt_id` (nullable FK)
- `retry_count` (int default 0)
- `max_retries` (int default 3)
- `version` (int default 1)
- `created_at` (timestamp)
- `updated_at` (timestamp)

Indexes:
- `(operator_id, created_at DESC)`
- `(status, updated_at)`

### 4.2 New Table: `feature_attempts`
Fields:
- `id` (PK)
- `run_id` (FK to `feature_runs.id`, indexed)
- `attempt_number` (int)
- `status` (enum)
- `idempotency_key` (text, unique)
- `error_code` (nullable text)
- `error_detail` (nullable jsonb)
- `started_at` (timestamp)
- `ended_at` (nullable timestamp)

Constraints:
- Unique `(run_id, attempt_number)`

### 4.3 New Table: `feature_run_events`
Fields:
- `id` (PK)
- `run_id` (FK, indexed)
- `attempt_id` (nullable FK)
- `event_type` (text)
- `actor_type` (enum: `operator`, `system`)
- `actor_id` (text)
- `event_payload` (jsonb)
- `created_at` (timestamp)

Purpose:
- Durable audit trail for step transitions, execution, and retries.

### 4.4 Migration and Rollout
1. Add enums/tables and API support behind feature flag.
2. Deploy backend read/write paths.
3. Enable frontend entry point for pilot roles.
4. Enable generally after success metrics are met.

## 5. Security Considerations

### 5.1 Authentication and Authorization
- Require authenticated operator identity for all endpoints.
- Enforce per-action RBAC scopes:
  - `feature_capability:access`
  - `feature_capability:execute`
  - `feature_capability:retry`
- Return explicit `403` errors for denied actions.

### 5.2 Input Validation and Injection Safety
- Validate all API payloads with strict schemas.
- Reject unknown fields.
- Apply server-side canonical validation regardless of client checks.
- Escape/sanitize rendered messages to avoid XSS in operator UI.

### 5.3 Integrity and Exactly-Once Execution
- Require idempotency keys for `execute` and `retry`.
- Enforce optimistic concurrency with `version` checks on mutating endpoints.
- Use transactional state transitions for run/attempt/event consistency.

### 5.4 Auditability and Compliance
- Record all state transitions with actor, timestamp, and reason/context.
- Protect audit events from mutation/deletion via write-once policy.
- Apply retention policy aligned with compliance requirements (pending open question resolution).

### 5.5 Operational Protections
- Rate-limit mutation endpoints per operator/session.
- Set worker execution timeout and retry/backoff policy.
- Limit retry attempts and expose clear terminal messaging when exhausted.

## 6. PRD Traceability
- US-001: Entry point + explicit access denied handling.
- US-002: Required fields, inline validation, blocked progression on errors.
- US-003: Final submit triggers execution once using idempotency + state guards.
- US-004: Persisted run state and status retrieval after refresh/reopen.
- US-005: Recoverable failure retry with preserved safe inputs and attempt trace history.

## 7. Open Questions and Decisions Required
- Exact permission mapping by operator role for access/execute/retry.
- Target SLO for end-to-end completion time and timeout budgets.
- Required audit granularity and retention duration for step transitions.
