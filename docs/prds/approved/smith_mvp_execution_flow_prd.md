---
id: smith-mvp-execution-flow-prd
title: Smith MVP Execution Flow PRD
status: approved
doc_type: prd
source_doc: docs/planning/approved/smith-mvp-execution-flow.md
target_branch: mvp-0
owner: lars
---

# Product Requirements Document (PRD)

# Smith MVP Execution Flow: Document -> Loop -> PR -> Steering

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

Automatic merge is not part of the MVP.

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

---

# Success Metrics

The MVP is successful if Smith can:

- ingest a document
- produce a normalized task contract
- run a loop autonomously
- create a Pull Request
- accept operator steering

---

# Summary

The MVP execution model for Smith is:

Document -> TaskContract -> Operator Review -> Loop Execution -> Validation -> Pull Request -> Steering

This architecture proves the core Smith concept while minimizing system complexity.
