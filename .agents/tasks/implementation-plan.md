# Implementation Plan: Add Feature Capability

## Planning Scope and Constraints
- Source documents: `/workspace/.agents/tasks/prd.json`, `/workspace/.agents/tasks/tech-spec.md`.
- Goal: deliver an operator-facing stateful feature flow with validation, exactly-once execution, durable status, and retry.
- Constraint: planning only. No implementation in this work item.

## Phase 1: Discovery, Decisions, and Delivery Guardrails

### Task 1.1: Finalize unresolved product and policy decisions
#### Sub-task 1.1.1
Define exact RBAC mapping for `feature_capability:access`, `feature_capability:execute`, and `feature_capability:retry` by operator role.
#### Sub-task 1.1.2
Set target end-to-end completion SLO and per-step timeout budgets (API and worker).
#### Sub-task 1.1.3
Define audit granularity, retention period, and compliance controls for `feature_run_events`.
#### Sub-task 1.1.4
Document approved recoverable vs terminal failure taxonomy and error-code catalog.

### Task 1.2: Align rollout and release strategy
#### Sub-task 1.2.1
Define feature flag strategy for backend write paths and frontend entry-point visibility.
#### Sub-task 1.2.2
Select pilot operator cohort and success/failure rollback criteria.
#### Sub-task 1.2.3
Create release checklist covering migrations, API deployment, worker rollout, and flag flips.

### Task 1.3: Lock interface contracts for build teams
#### Sub-task 1.3.1
Publish canonical API contract examples for all endpoints and error states (`403`, `409`, `422`, `429`).
#### Sub-task 1.3.2
Publish validation violation schema contract (`field`, `code`, `message`, `blocking`) and message standards.
#### Sub-task 1.3.3
Publish UI state contract mapping run/attempt states to screens and actions.

## Phase 2: Data Model and Persistence Foundations

### Task 2.1: Implement schema for workflow runs, attempts, and events
#### Sub-task 2.1.1
Create `feature_runs` table with status enum, payload snapshots, retry counters, and optimistic `version` field.
#### Sub-task 2.1.2
Create `feature_attempts` table with unique idempotency key and `(run_id, attempt_number)` uniqueness.
#### Sub-task 2.1.3
Create `feature_run_events` immutable audit table linked to run and optional attempt.
#### Sub-task 2.1.4
Add required indexes for operator history and status polling performance.

### Task 2.2: Enforce integrity and state invariants at persistence layer
#### Sub-task 2.2.1
Add constraints to prevent invalid enum values and invalid retry counters.
#### Sub-task 2.2.2
Add transaction patterns guaranteeing single active attempt per run.
#### Sub-task 2.2.3
Define migration-safe defaults and backfill strategy for nullable fields.

### Task 2.3: Prepare migration and rollback procedures
#### Sub-task 2.3.1
Create forward migration scripts for enums, tables, indexes, constraints.
#### Sub-task 2.3.2
Create rollback scripts compatible with in-flight feature-flag disabled state.
#### Sub-task 2.3.3
Test migration timing and lock behavior in staging-sized dataset.

## Phase 3: Backend Workflow Domain and APIs

### Task 3.1: Build workflow state machine service
#### Sub-task 3.1.1
Implement transition rules for run states: `DRAFT -> READY -> EXECUTING -> terminal`.
#### Sub-task 3.1.2
Enforce retry eligibility: only `FAILED_RECOVERABLE`, under retry limit.
#### Sub-task 3.1.3
Implement optimistic concurrency checks on all mutating operations with `version` conflicts.
#### Sub-task 3.1.4
Emit immutable transition events for create, update, validate, execute, retry, completion.

### Task 3.2: Implement validation service and progression gating
#### Sub-task 3.2.1
Implement authoritative server-side validation for each step payload.
#### Sub-task 3.2.2
Normalize validation violations to canonical schema and blocking semantics.
#### Sub-task 3.2.3
Gate step progression and execution readiness on blocking violations.

### Task 3.3: Implement API controller endpoints
#### Sub-task 3.3.1
Implement `POST /runs` to create or initialize run context.
#### Sub-task 3.3.2
Implement `GET /runs/{run_id}` with persisted status, validation snapshot, retry summary.
#### Sub-task 3.3.3
Implement `PATCH /runs/{run_id}/steps/{step_number}` with schema checks and `409` conflict handling.
#### Sub-task 3.3.4
Implement `POST /runs/{run_id}/validate` for full-run validation.
#### Sub-task 3.3.5
Implement `POST /runs/{run_id}/execute` requiring idempotency key and valid state.
#### Sub-task 3.3.6
Implement `POST /runs/{run_id}/retry` requiring retryable state, key, and limit checks.
#### Sub-task 3.3.7
Implement `GET /runs/{run_id}/attempts` for operator traceability.

### Task 3.4: Enforce authn/authz and operational protections
#### Sub-task 3.4.1
Apply authenticated operator context to all endpoints.
#### Sub-task 3.4.2
Enforce per-action RBAC scopes and explicit `403` behavior.
#### Sub-task 3.4.3
Apply payload schema hardening (unknown-field rejection).
#### Sub-task 3.4.4
Add per-operator/session rate limits for mutation endpoints.

## Phase 4: Execution Pipeline and Reliability Controls

### Task 4.1: Implement dispatcher and queue integration
#### Sub-task 4.1.1
Publish execution jobs with `run_id`, `attempt_id`, and idempotency key.
#### Sub-task 4.1.2
Persist dispatch metadata to support replay and observability.
#### Sub-task 4.1.3
Prevent duplicate job submission under retries or client replays.

### Task 4.2: Implement idempotent worker execution
#### Sub-task 4.2.1
Load attempt context and enforce idempotent task execution.
#### Sub-task 4.2.2
Classify outcomes into `SUCCEEDED`, `FAILED_RECOVERABLE`, `FAILED_TERMINAL`.
#### Sub-task 4.2.3
Persist attempt terminal status, timestamps, and error details.
#### Sub-task 4.2.4
Update parent run terminal state in same consistency boundary.

### Task 4.3: Add resilience and failure handling
#### Sub-task 4.3.1
Implement execution timeout and cancellation policies.
#### Sub-task 4.3.2
Configure worker retry/backoff rules distinct from operator retry logic.
#### Sub-task 4.3.3
Route poisoned messages to dead-letter/error queue with replay tooling.

## Phase 5: Frontend Feature Flow Experience

### Task 5.1: Deliver entry point and access-denied states
#### Sub-task 5.1.1
Add visible feature entry action in primary operator interface under feature flag.
#### Sub-task 5.1.2
Handle unauthorized open with explicit access-denied screen and recovery navigation.

### Task 5.2: Build flow shell and run lifecycle handling
#### Sub-task 5.2.1
Implement create-or-resume behavior for `feature_run` and track `run_id`.
#### Sub-task 5.2.2
Render stepper with current/completed/error states.
#### Sub-task 5.2.3
Implement navigation controls (`Next`, `Back`, `Submit`, `Retry`) bound to server state.
#### Sub-task 5.2.4
Restore run state after refresh/reopen via `GET /runs/{run_id}`.

### Task 5.3: Build step forms with validation UX
#### Sub-task 5.3.1
Define required fields and local pre-validation rules per step.
#### Sub-task 5.3.2
Wire `PATCH` step updates and map server validation to field-level errors.
#### Sub-task 5.3.3
Block progression on blocking violations and preserve user-entered values.

### Task 5.4: Implement submission, execution status, and terminal states
#### Sub-task 5.4.1
Call execute endpoint with idempotency key and latest `version`.
#### Sub-task 5.4.2
Disable duplicate submits while `EXECUTING` and show deterministic progress UI.
#### Sub-task 5.4.3
Render definitive success confirmation for `SUCCEEDED`.
#### Sub-task 5.4.4
Render actionable error panels for recoverable vs terminal failure states.

### Task 5.5: Implement retry UX and history visibility
#### Sub-task 5.5.1
Show retry action only when backend reports retry available.
#### Sub-task 5.5.2
Preserve safe input snapshot across retries and avoid unsafe auto-overwrites.
#### Sub-task 5.5.3
Display attempt history/outcomes from `GET /attempts` for traceability.

## Phase 6: Observability, Auditability, and Operations

### Task 6.1: Instrument logs, metrics, and traces
#### Sub-task 6.1.1
Log key lifecycle events with `run_id`, `attempt_id`, `request_id` correlation IDs.
#### Sub-task 6.1.2
Add metrics: flow starts, validation failures, execute outcomes, retry outcomes, completion latency.
#### Sub-task 6.1.3
Add tracing around API-to-worker handoff and terminal updates.

### Task 6.2: Operational dashboards and alerting
#### Sub-task 6.2.1
Create dashboard for funnel conversion and failure breakdown by error code.
#### Sub-task 6.2.2
Create alerts for error-rate spikes, timeout saturation, and dead-letter growth.
#### Sub-task 6.2.3
Create runbook for stuck executions and manual recovery procedures.

### Task 6.3: Audit controls and data governance
#### Sub-task 6.3.1
Enforce write-once protections for audit event records.
#### Sub-task 6.3.2
Implement retention and archival policy per compliance decision.
#### Sub-task 6.3.3
Validate audit completeness for every required transition.

## Phase 7: Testing, Quality Gates, and Rollout

### Task 7.1: Backend test coverage
#### Sub-task 7.1.1
Add unit tests for state machine transitions, retry guardrails, and concurrency conflicts.
#### Sub-task 7.1.2
Add API integration tests for happy path and all documented error responses.
#### Sub-task 7.1.3
Add idempotency tests for execute/retry deduplication and exactly-once behavior.
#### Sub-task 7.1.4
Add worker tests for failure classification and timeout handling.

### Task 7.2: Frontend test coverage
#### Sub-task 7.2.1
Add component tests for step validation rendering and progression blocking.
#### Sub-task 7.2.2
Add flow integration tests for create/resume, submit, status refresh, and retry.
#### Sub-task 7.2.3
Add end-to-end tests for success path, recoverable failure/retry, and terminal failure.

### Task 7.3: Local CI and hook validation
#### Sub-task 7.3.1
Ensure local tooling prerequisites (`act`, Docker) are available in contributor docs and CI checks.
#### Sub-task 7.3.2
Run pre-commit `lint-and-check` job via `act`.
#### Sub-task 7.3.3
Run pre-push parallel tests via `act` (`go-unit-tests`, `node-unit-tests`, `playwright-tests`).
#### Sub-task 7.3.4
Run full local CI with `make ci-local-act` before rollout approval.

### Task 7.4: Controlled rollout and verification
#### Sub-task 7.4.1
Deploy with feature flag off and verify no impact on unrelated operator flows.
#### Sub-task 7.4.2
Enable for pilot roles, monitor success metrics, and validate retry/error behavior.
#### Sub-task 7.4.3
Promote to general availability after meeting thresholds and closing open questions.
#### Sub-task 7.4.4
Conduct post-launch review against PRD success metrics and produce follow-up backlog.

## PRD Story Traceability Matrix
- US-001 covered by Phase 5 Task 5.1 and backend auth paths in Phase 3 Task 3.4.
- US-002 covered by Phase 3 Task 3.2 and Phase 5 Task 5.3.
- US-003 covered by Phase 3 Task 3.1/3.3, Phase 4 Task 4.1/4.2, and Phase 7 Task 7.1.3.
- US-004 covered by Phase 3 Task 3.3.2 and Phase 5 Task 5.2/5.4.
- US-005 covered by Phase 3 Task 3.1.2 and 3.3.6, Phase 5 Task 5.5, and Phase 7 Task 7.2.3.

## Exit Criteria for Plan Completion
- Open questions from PRD have owners and decision deadlines.
- Every API and state transition in tech spec maps to at least one implementation task.
- Test plan includes unit, integration, E2E, idempotency, and failure-path coverage.
- Rollout includes pilot gating, observability thresholds, and rollback path.
