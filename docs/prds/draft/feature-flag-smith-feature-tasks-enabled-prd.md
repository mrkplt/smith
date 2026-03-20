---
id: prd-ff-smith-feature-tasks-enabled
title: PRD for SMITH_FEATURE_TASKS_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_TASKS_ENABLED

## Overview

`SMITH_FEATURE_TASKS_ENABLED` gates the Tasks runtime surface (`/tasks`) so operators only see it when the task-contract workflow is ready in their environment. This adds rollout safety by preventing incomplete task UX from appearing in production by default.

This feature is currently gated and under development.

## Goals

- Keep Tasks hidden by default until workflows are stable.
- Provide a predictable enablement switch for controlled environment rollouts.
- Ensure route, navigation, and onboarding behavior stay consistent when disabled.

## Non-Goals

- Redesigning the Tasks UX itself.
- Replacing Task Contract API schema.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Gate Tasks navigation and route visibility

As an operator, I want Tasks only visible when explicitly enabled so that unfinished runtime workflows do not distract or confuse the team.

#### Acceptance Criteria

- When `SMITH_FEATURE_TASKS_ENABLED=false`, `/tasks` is hidden from navigation and direct access is redirected.
- When `SMITH_FEATURE_TASKS_ENABLED=true`, Tasks navigation and route are available.
- Settings/runtime config clearly reflect the enabled/disabled state.
- Negative path: stale browser state does not reveal Tasks after the flag is turned off.

### US-002: Document rollout value and operating guidance

As a platform owner, I want clear guidance for this feature flag so I can enable it safely and explain its value.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation identifies this feature as gated and under development.
- Documentation explains the value: safer rollout control and reduced operator confusion.
- Documentation includes explicit enable/disable configuration points.
- When the flag is disabled, docs explicitly state that direct `/tasks` access is rejected and does not render task data.
