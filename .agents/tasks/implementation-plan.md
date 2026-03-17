# Implementation Plan: Add Feature Capability

## Phase 1: Discovery, Decisions, and Delivery Guardrails

### Task 1.1: Resolve open product and compliance decisions
- Sub-task 1.1.1: Confirm exact RBAC role-to-permission mapping for `feature_capability:access`, `feature_capability:execute`, and `feature_capability:retry`.
- Sub-task 1.1.2: Define end-to-end completion SLO target and execution timeout thresholds.
- Sub-task 1.1.3: Finalize audit event granularity and retention requirements.
- Sub-task 1.1.4: Decide V1 status transport mode (polling only vs SSE/WebSocket).
- Sub-task 1.1.5: Record all decisions in an architecture decision log and link to implementation tickets.

### Task 1.2: Plan feature rollout and gating
- Sub-task 1.2.1: Define feature flag strategy for backend schema/API enablement and frontend entry point exposure.
- Sub-task 1.2.2: Define pilot operator cohort and rollback criteria.
- Sub-task 1.2.3: Create release checklist for migration, enablement, and monitoring verification.

### Task 1.3: Prepare repo and CI prerequisites
- Sub-task 1.3.1: Verify `act` and Docker availability in developer and CI-like environments.
- Sub-task 1.3.2: Validate pre-commit and pre-push hooks run expected `act` jobs.
- Sub-task 1.3.3: Confirm frontend dependency/install/build/check commands are documented and reproducible.

## Phase 2: Data Model and Persistence Foundation

### Task 2.1: Add workflow persistence schema
- Sub-task 2.1.1: Create migration for `feature_runs` table with required columns and indexes.
- Sub-task 2.1.2: Create migration for `feature_attempts` table with required constraints and indexes.
- Sub-task 2.1.3: Create migration for `feature_run_events` table and indexes.
- Sub-task 2.1.4: Add enum definitions for run and attempt statuses.

### Task 2.2: Enforce integrity constraints
- Sub-task 2.2.1: Add uniqueness constraint for `(run_id, attempt_number)`.
- Sub-task 2.2.2: Add uniqueness constraint for attempt `idempotency_key`.
- Sub-task 2.2.3: Add foreign key constraints between runs, attempts, and events.
- Sub-task 2.2.4: Validate optimistic concurrency support via `version` column semantics.

### Task 2.3: Implement data access layer changes
- Sub-task 2.3.1: Add repository/model definitions for run lifecycle CRUD.
- Sub-task 2.3.2: Add repository/model definitions for attempts and attempt summaries.
- Sub-task 2.3.3: Add event/audit append APIs with immutable writes.
- Sub-task 2.3.4: Add query paths for run resume, latest status, and attempt history.

## Phase 3: Backend API and Workflow Orchestration

### Task 3.1: Implement API endpoints and schemas
- Sub-task 3.1.1: Implement `POST /api/v1/feature-capability/runs`.
- Sub-task 3.1.2: Implement `GET /api/v1/feature-capability/runs/{run_id}`.
- Sub-task 3.1.3: Implement `PATCH /api/v1/feature-capability/runs/{run_id}/steps/{step_number}`.
- Sub-task 3.1.4: Implement `POST /api/v1/feature-capability/runs/{run_id}/validate` (if enabled in V1).
- Sub-task 3.1.5: Implement `POST /api/v1/feature-capability/runs/{run_id}/execute` with required `Idempotency-Key`.
- Sub-task 3.1.6: Implement `POST /api/v1/feature-capability/runs/{run_id}/retry` with required `Idempotency-Key`.
- Sub-task 3.1.7: Implement `GET /api/v1/feature-capability/runs/{run_id}/attempts`.

### Task 3.2: Implement authorization and request hardening
- Sub-task 3.2.1: Enforce authentication checks on all feature-capability endpoints.
- Sub-task 3.2.2: Enforce per-action RBAC gates (`access`, `execute`, `retry`).
- Sub-task 3.2.3: Add strict JSON schema validation and reject unknown fields.
- Sub-task 3.2.4: Standardize `403`, `409`, and `422` error response contracts.

### Task 3.3: Build workflow service transition engine
- Sub-task 3.3.1: Encode run state transitions (`DRAFT`, `READY`, `EXECUTING`, `SUCCEEDED`, `FAILED_RECOVERABLE`, `FAILED_TERMINAL`).
- Sub-task 3.3.2: Enforce “only `READY` may execute” invariant.
- Sub-task 3.3.3: Enforce “single active execution per run” invariant.
- Sub-task 3.3.4: Enforce “retry only from `FAILED_RECOVERABLE` and within `max_retries`”.
- Sub-task 3.3.5: Apply optimistic concurrency checks on all mutating transitions.
- Sub-task 3.3.6: Emit audit events for create/update/validate/execute/retry/completion transitions.

### Task 3.4: Implement validation service
- Sub-task 3.4.1: Define domain validation contract returning `field`, `code`, `message`, and `blocking`.
- Sub-task 3.4.2: Implement required-field and domain-rule validation paths.
- Sub-task 3.4.3: Separate warnings from blocking violations and propagate to API responses.
- Sub-task 3.4.4: Persist validation snapshots for resume and troubleshooting flows.

## Phase 4: Execution Worker, Idempotency, and Reliability

### Task 4.1: Implement dispatcher and queue integration
- Sub-task 4.1.1: Add dispatch logic for execute and retry requests.
- Sub-task 4.1.2: Pass `run_id`, `attempt_id`, and idempotency token in queued job payload.
- Sub-task 4.1.3: Add safeguards to prevent duplicate job enqueue under concurrency.

### Task 4.2: Implement execution worker lifecycle
- Sub-task 4.2.1: Implement attempt startup transition to `STARTED`.
- Sub-task 4.2.2: Execute the intended task and capture structured result metadata.
- Sub-task 4.2.3: Classify failures as recoverable vs terminal.
- Sub-task 4.2.4: Atomically persist attempt terminal state and run terminal/current state.
- Sub-task 4.2.5: Emit completion audit events and correlation-friendly logs.

### Task 4.3: Add reliability controls
- Sub-task 4.3.1: Enforce exactly-once semantics per attempt idempotency key.
- Sub-task 4.3.2: Add execution timeout handling and dead-letter path.
- Sub-task 4.3.3: Add mutation endpoint rate limits by operator.
- Sub-task 4.3.4: Add retry cap enforcement with clear terminal messaging.

## Phase 5: Frontend Flow, UX States, and Resume Behavior

### Task 5.1: Add feature entry point and access-denied behavior
- Sub-task 5.1.1: Add visible operator entry point in primary interface/navigation.
- Sub-task 5.1.2: Gate entry visibility by `feature_capability:access` permission.
- Sub-task 5.1.3: Route unauthorized access attempts to explicit access-denied view.

### Task 5.2: Build multi-step flow container
- Sub-task 5.2.1: Implement create-or-resume run initialization behavior.
- Sub-task 5.2.2: Implement stepper UI with active/completed/error states.
- Sub-task 5.2.3: Implement progression gating based on validation status.
- Sub-task 5.2.4: Add optional local draft caching and server reconciliation logic.

### Task 5.3: Implement step forms and validation feedback
- Sub-task 5.3.1: Define per-step required fields and client-side validation rules.
- Sub-task 5.3.2: Integrate step update endpoint for authoritative server validation.
- Sub-task 5.3.3: Map violations to field-level errors with actionable messages.
- Sub-task 5.3.4: Ensure valid inputs allow step progression without page reload.

### Task 5.4: Implement execute, status, and completion views
- Sub-task 5.4.1: Add final submit action with generated idempotency key.
- Sub-task 5.4.2: Show in-progress state while run is `EXECUTING`.
- Sub-task 5.4.3: Show definitive terminal status banners (`SUCCEEDED`/failure states).
- Sub-task 5.4.4: Implement refresh/reopen resume by refetching canonical run state.
- Sub-task 5.4.5: Implement attempt history display for operator traceability.

### Task 5.5: Implement retry UX for recoverable failures
- Sub-task 5.5.1: Show retry action only when run is `FAILED_RECOVERABLE` and retries remain.
- Sub-task 5.5.2: Add retry confirmation modal describing reused/revalidated data.
- Sub-task 5.5.3: Re-enter execution state and update UI attempt timeline post-retry.

## Phase 6: Observability, Testing, and Release Validation

### Task 6.1: Instrument logs, metrics, and alerts
- Sub-task 6.1.1: Add structured logs with `run_id`, `attempt_id`, and `request_id` correlation.
- Sub-task 6.1.2: Add metrics for run creation, execute outcomes, retry outcomes, validation failures, and flow duration.
- Sub-task 6.1.3: Configure alerts for elevated terminal failure ratio and timeout breaches.

### Task 6.2: Implement comprehensive automated tests
- Sub-task 6.2.1: Add unit tests for transition invariants and validation logic.
- Sub-task 6.2.2: Add API integration tests for success and key error paths (`403`, `409`, `422`).
- Sub-task 6.2.3: Add concurrency/idempotency tests for execute and retry semantics.
- Sub-task 6.2.4: Add worker tests for recoverable vs terminal failure classification.
- Sub-task 6.2.5: Add frontend component and flow tests for validation, execution, resume, and retry.
- Sub-task 6.2.6: Add end-to-end tests validating full operator flow and post-refresh status visibility.

### Task 6.3: Run local CI and pre-release verification
- Sub-task 6.3.1: Run lint/check and test suites through local `act` workflows.
- Sub-task 6.3.2: Verify migration apply/rollback in staging-like environment.
- Sub-task 6.3.3: Execute pilot role UAT against acceptance criteria US-001 to US-005.
- Sub-task 6.3.4: Validate observability dashboards and alert routing before enablement.

### Task 6.4: Launch and post-launch controls
- Sub-task 6.4.1: Enable feature flags for pilot roles and monitor execution health.
- Sub-task 6.4.2: Review pilot feedback and failure analytics; prioritize corrective fixes.
- Sub-task 6.4.3: Gradually expand rollout to broader operator groups.
- Sub-task 6.4.4: Conduct post-launch review against PRD success metrics and unresolved risks.

## Acceptance Criteria Traceability
- US-001 is covered by Phase 5.1 and Phase 3.2 authorization behavior.
- US-002 is covered by Phase 5.3 and Phase 3.4 validation service implementation.
- US-003 is covered by Phase 3.3, Phase 4.1, and Phase 4.2 execution invariants.
- US-004 is covered by Phase 5.4 resume/status design and Phase 2 persisted state model.
- US-005 is covered by Phase 5.5 retry UX, Phase 3.3 retry policy, and Phase 4 attempt lifecycle.
