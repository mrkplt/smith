---
id: smith-mvp-execution-flow-prd
title: Smith MVP Execution Flow PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/smith-mvp-execution-flow.md
target_branch: mvp-0
owner: lars
---

# Smith MVP Execution Flow PRD: Document -> Loop -> PR -> Steering

## Overview

This PRD defines the minimum viable feature set required to run Smith end-to-end. The goal is to prove that Smith can:

1. Ingest a task document
2. Normalize/validate the document into a runnable task contract
3. Execute a Smith loop using a configured project and provider
4. Produce code changes
5. Run validation
6. Submit a Pull Request
7. Allow operator steering before and during execution

The scope intentionally focuses on the smallest complete product slice that demonstrates Smith's core value.

---

# Goals

1. Support ingestion of a task document (Markdown).
2. Normalize and validate the document into a canonical Smith task contract.
3. Bind the task contract to a configured Project and Provider Profile.
4. Launch a Smith loop that performs repository work autonomously.
5. Run project validation commands.
6. Push a branch and submit a Pull Request.
7. Allow operator steering before execution and during execution.

---

# Non-Goals

The following capabilities are explicitly out of scope for the MVP:

- Multi-document task synthesis
- Collaborative multi-user editing
- Automatic response to PR comments
- Cross-project orchestration
- Complex skill marketplaces
- Multiple steering actors
- Autonomous merge strategies

These may be added in later phases.

---

# Core Concept: Task Contract

All incoming work is normalized into a TaskContract object.

The TaskContract is the bridge between:

- document ingestion
- loop execution

### TaskContract Schema

Example structure:

```yaml
kind: smith.task
id: task-123

project_id: smith
provider_profile_id: openai-work

source_document: docs/task.md

objective: Restore green CI on main

constraints:
  - do not change public API

acceptance_criteria:
  - unit tests pass
  - CI workflow passes locally

validation:
  - make test
  - act -j ci

status: draft
```

### TaskContract Status

Possible states:

- draft
- validated
- approved
- running
- completed
- blocked

`completed_pending_review` is a loop state (not a TaskContract state).

### Loop Status

Possible loop states for MVP:

- queued
- running
- paused
- cancelled
- failed
- completed_pending_review

---

# System Components Required for MVP

## Required

- Smith API
- Smith Core Loop Reconciler
- Kubernetes runtime (replica jobs)
- GitHub integration
- Provider adapter
- Document normalization service
- Event journal

## Optional (Not Required for MVP)

- multi-provider orchestration
- batch task ingestion
- rich chat UI
- skill marketplace

## MVP Implementation Checklist

### Backend API

- [ ] implement `POST /api/tasks` with schema validation and `draft` state initialization
- [ ] implement `GET /api/tasks/{id}` and `PATCH /api/tasks/{id}` with audit-safe field updates
- [ ] implement `POST /api/tasks/{id}/approve` to transition `validated -> approved`
- [ ] implement loop lifecycle APIs (`create`, `pause`, `resume`, `cancel`) with state guards

### Core Loop and Runtime

- [ ] persist loop records and lock acquisition before runtime job launch
- [ ] launch Kubernetes replica jobs with task and provider binding
- [ ] implement ordered runtime steps (clone -> plan -> edit -> validate -> commit -> push -> PR)
- [ ] map success and failure outcomes to terminal loop/task states

### Frontend and Operator Controls

- [ ] provide task ingestion UI for markdown upload/paste
- [ ] provide normalized task review/edit and explicit approve action
- [ ] provide live loop state view with journal timeline
- [ ] provide steering controls (pause/resume/cancel/intervention)

### Integrations and Platform

- [ ] configure provider adapter profile resolution at runtime
- [ ] configure GitHub branch push and PR creation integration
- [ ] ensure event journal persists all state transitions and interventions
- [ ] ensure required local CI dependencies (`act`, Docker) are available in operator workflows

### Validation and E2E

- [ ] execute task-defined validation commands in runtime with structured results
- [ ] verify happy-path E2E run from document ingestion to PR creation
- [ ] verify failed and cancelled runs produce terminal states with reason codes
- [ ] verify intervention events are replayable from journal history

---

# End-to-End Execution Flow

## Step 1: Document Ingestion

User submits a task document.

Supported inputs:

- Markdown upload
- Markdown paste

Future support (not required yet):

- GitHub Issue ingestion

Result:

A `TaskContract` object in `draft` state.

---

## Step 2: Document Normalization

The system validates or rewrites the document into the canonical Smith task format.

This step may involve:

- deterministic schema validation
- LLM-assisted rewriting
- optional skill execution

Possible outcomes:

- Document passes validation
- Document rewritten into valid structure
- Document rejected with errors

Result:

`TaskContract.status = validated`

---

## Step 3: Operator Review and Editing

Before execution, the operator can review the normalized task.

Allowed actions:

- edit task fields
- modify validation commands
- adjust acceptance criteria
- change provider profile

Result:

`TaskContract.status = approved`

---

## Step 4: Loop Creation

A Smith loop is created referencing the approved TaskContract.

Example API request:

```json
{
  "task_contract_id": "task-123"
}
```

Smith Core will:

1. Persist the loop record
2. Acquire a loop lock
3. Launch a Kubernetes replica job

### Idempotency and Retry Expectations (MVP)

To prevent duplicate execution and ambiguous outcomes, the MVP enforces minimal idempotency and retry rules:

- `POST /api/loops` supports an optional idempotency key; duplicate submissions with the same key return the existing loop
- `pause`, `resume`, and `cancel` are idempotent state transitions (repeating the same action is a no-op)
- `POST /api/loops/{id}/interventions` accepts a client-generated event id to prevent duplicate journal entries

Retry policy for runtime operations:

- transient provider/network errors may be retried with bounded exponential backoff
- git push and PR creation may be retried for transient transport failures
- validation command failures are not auto-retried (treated as deterministic task failure)

Out of scope for MVP:

- cross-run deduplication across different idempotency keys
- advanced retry orchestration with per-step policy customization

---

# Loop Execution Responsibilities

The runtime replica must perform the following steps:

1. Clone the repository
2. Read the TaskContract
3. Generate an implementation plan
4. Modify repository files
5. Run validation commands
6. Commit changes
7. Push branch
8. Open Pull Request

---

# Completion Policy

For the MVP, loop completion occurs when:

- validation commands pass
- branch is pushed
- pull request is created

Loop state becomes:

`completed_pending_review`

TaskContract state becomes:

`completed`

Automatic merge is not part of the MVP.

## Failure and Cancellation Policy

The loop must end in a terminal non-success state when any required step cannot be completed.

Terminal outcomes:

- `failed`: implementation, validation, push, or PR creation fails
- `cancelled`: operator cancels the loop

When terminal outcomes occur:

- TaskContract state becomes `blocked`
- failure reason is recorded in the event journal

---

# Skills Usage

Skills are intentionally limited in scope.

## Document Normalization Skills

Examples:

- task extraction
- acceptance criteria detection
- schema enforcement

## Validation Skills

Examples:

- test output parsing
- CI workflow validation
- repository convention enforcement

Skills should support the pipeline rather than orchestrate the entire loop.

---

# Operator Steering

Steering is introduced in two stages.

## Stage A: Pre-Execution Editing

Operators can modify the TaskContract before loop launch.

This allows corrections such as:

- clarifying scope
- adjusting constraints
- modifying validation steps

---

## Stage B: In-Execution Steering

During execution operators can intervene.

Interventions are recorded in the loop journal.

Supported actions:

- pause loop
- resume loop
- cancel loop
- add operator instruction
- request replan

Example intervention event:

```json
{
  "type": "operator_intervention",
  "instruction": "Avoid modifying authentication logic"
}
```

---

# Event Journal

All runtime activity is journaled.

Journal entries include:

- loop state transitions
- runtime events
- validation results
- operator interventions

This provides full traceability.

---

# Minimal API Surface

## Task Contracts

```
POST   /api/tasks
GET    /api/tasks/{id}
PATCH  /api/tasks/{id}
POST   /api/tasks/{id}/approve
```

## Loops

```
POST   /api/loops
GET    /api/loops/{id}
POST   /api/loops/{id}/pause
POST   /api/loops/{id}/resume
POST   /api/loops/{id}/cancel
```

## Journal

```
GET /api/loops/{id}/journal
POST /api/loops/{id}/interventions
```

## Draft OpenAPI Skeleton

```yaml
openapi: 3.1.0
info:
  title: Smith MVP API
  version: 0.1.0
servers:
  - url: https://smith.example.com
paths:
  /api/tasks:
    post:
      operationId: createTask
      summary: Ingest and create a draft task contract
      requestBody:
        required: true
      responses:
        "201":
          description: TaskContract created
  /api/tasks/{id}:
    get:
      operationId: getTask
      responses:
        "200":
          description: TaskContract
    patch:
      operationId: updateTask
      responses:
        "200":
          description: TaskContract updated
  /api/tasks/{id}/approve:
    post:
      operationId: approveTask
      responses:
        "200":
          description: TaskContract approved
  /api/loops:
    post:
      operationId: createLoop
      parameters:
        - name: Idempotency-Key
          in: header
          required: false
          schema:
            type: string
      responses:
        "201":
          description: Loop created or returned from idempotent request
  /api/loops/{id}:
    get:
      operationId: getLoop
      responses:
        "200":
          description: Loop status
  /api/loops/{id}/pause:
    post:
      operationId: pauseLoop
      responses:
        "200":
          description: Loop paused (idempotent)
  /api/loops/{id}/resume:
    post:
      operationId: resumeLoop
      responses:
        "200":
          description: Loop resumed (idempotent)
  /api/loops/{id}/cancel:
    post:
      operationId: cancelLoop
      responses:
        "200":
          description: Loop cancelled (idempotent)
  /api/loops/{id}/journal:
    get:
      operationId: getLoopJournal
      responses:
        "200":
          description: Journal entries
  /api/loops/{id}/interventions:
    post:
      operationId: createIntervention
      responses:
        "201":
          description: Intervention appended
components:
  schemas:
    TaskContract:
      type: object
    Loop:
      type: object
    JournalEntry:
      type: object
```

---

# Success Metrics

The MVP is successful if Smith can:

- ingest a document
- produce a normalized task contract
- run a loop autonomously
- create a Pull Request
- accept operator steering

Minimum acceptance targets:

- E2E happy path succeeds for at least one real repository using configured validation commands
- for failed or cancelled runs, terminal state and reason are visible in the loop journal
- operator pause/resume/cancel and intervention instructions are persisted and replayable from journal entries

---

# Summary

The MVP execution model for Smith is:

Document -> TaskContract -> Operator Review -> Loop Execution -> Validation -> Pull Request -> Steering

This architecture proves the core Smith concept while minimizing system complexity.
