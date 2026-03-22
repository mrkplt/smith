---
id: prd-ff-smith-feature-tasks-kanban-enabled
title: PRD for SMITH_FEATURE_TASKS_KANBAN_ENABLED
status: approved
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_TASKS_KANBAN_ENABLED

## Overview

`SMITH_FEATURE_TASKS_KANBAN_ENABLED` gates a new Tasks Kanban dashboard in Smith Console. The dashboard organizes Smith task contracts into four operator-facing lanes:

- Scheduled
- In Focus
- Awaiting Approval
- Finished

This dashboard is backed by Smith task APIs and Smith task status values. It is inspired by local `td` workflow patterns, but it does not depend on `td` data or `td` runtime.

## Goals

- Provide a simple, high-signal Kanban view for Smith task contracts.
- Keep rollout safe by gating the surface behind a feature flag.
- Map existing Smith task statuses into clear lane semantics without changing the underlying status model.
- Preserve compatibility with the existing `/tasks` task-contract lifecycle.

## Non-Goals

- Replacing Smith task APIs with `td`.
- Changing canonical Smith task statuses (`draft`, `validated`, `approved`, `running`, `completed`, `blocked`).
- Introducing drag-and-drop workflow transitions in v1.
- Building a project-management suite beyond the task dashboard.

## Feature Flag

- Env var: `SMITH_FEATURE_TASKS_KANBAN_ENABLED`
- Runtime config key: `featureTasksKanbanEnabled`
- Helm value: `console.featureFlags.tasksKanban`
- Default: `false`

When disabled, the Kanban navigation entry and route are hidden, and direct route access is redirected to `/tasks`.

## Lane Mapping (Smith-Native)

Lane definitions in v1 are deterministic and derived from Smith task statuses:

- Scheduled: `draft`, `approved`
- In Focus: `running`
- Awaiting Approval: `validated`
- Finished: `completed`, `blocked`

`blocked` appears in Finished as a terminal outcome and must be visually distinguished from `completed` (for example with an error badge).

## Functional Requirements

- Render four Kanban columns with lane title and item count.
- Show task cards with: task id, objective, project id, provider profile id, status, and updated time.
- Highlight currently active work in In Focus without introducing a new backend status.
- Support manual refresh and periodic auto-refresh.
- Show lane-level empty states and failure states.
- Keep existing `/tasks` create/edit/approve/start loop flows available.

## UX Requirements

- Route: `/tasks/kanban`.
- Include an entry point from `/tasks` when feature is enabled.
- Use responsive layout for desktop and mobile.
- Ensure keyboard navigation across lanes and cards.
- Preserve existing Console visual language and feature-flag behavior.

## API and Data Contract

The view consumes Smith task contracts from existing endpoints:

- `GET /v1/tasks`
- `GET /v1/tasks/{id}`

Console performs lane grouping client-side in v1. A server-aggregated board endpoint is optional future work if pagination/performance requires it.

## User Stories

### US-001: Feature-gated Kanban visibility

As an operator, I want the Tasks Kanban view available only when enabled so unfinished workflow UX does not appear by default.

#### Acceptance Criteria

- When `SMITH_FEATURE_TASKS_KANBAN_ENABLED=false`, `/tasks/kanban` is hidden from navigation and redirects to `/tasks`.
- When `SMITH_FEATURE_TASKS_KANBAN_ENABLED=true`, navigation entry and route are available.
- Stale browser state does not keep Kanban visible after disabling the flag.
- Negative path: direct route access while disabled must not render board data and must return a clear redirect outcome to `/tasks`.

### US-002: Scheduled lane for queued work

As an operator, I want non-running queued tasks grouped in Scheduled so I can see what is ready to refine or launch.

#### Acceptance Criteria

- Scheduled contains all tasks in `draft` or `approved` status.
- Card metadata is visible and legible at dashboard density.
- Lane count matches rendered cards.
- Negative path: tasks with missing or invalid status values are excluded from Scheduled, counted as unmapped, and reported with a visible lane-level mapping warning.

### US-003: In Focus lane for active execution

As an operator, I want running tasks grouped separately so I can quickly inspect active execution.

#### Acceptance Criteria

- In Focus contains all tasks in `running` status.
- Running tasks are visually distinct from non-running lanes.
- Negative path: if there are no `running` tasks, In Focus renders an explicit empty state and must not show stale cards from prior refreshes.

### US-004: Awaiting Approval lane for review queue

As an operator, I want validated tasks grouped in one lane so I can approve the next work item efficiently.

#### Acceptance Criteria

- Awaiting Approval contains all tasks in `validated` status.
- From each card, operator can navigate to the existing approve action flow.
- Negative path: approve attempts on tasks no longer in `validated` status are rejected with a clear error and the board refreshes to current state.

### US-005: Finished lane for terminal outcomes

As an operator, I want completed and blocked tasks grouped in Finished so I can confirm closed outcomes and detect failures.

#### Acceptance Criteria

- Finished contains all tasks in `completed` or `blocked` status.
- `blocked` tasks are clearly labeled and distinguishable from `completed` tasks.
- Negative path: unknown or invalid terminal status values are not silently treated as `completed`; they are excluded from Finished and surfaced through a visible board error indicator.

## Quality Gates

- `go test ./...`
- `npm --prefix frontend run check`
- `npm --prefix frontend run test:unit`

## Rollout and Observability

- Roll out in non-production environments first.
- Track route usage, refresh failures, and lane rendering errors.
- Keep feature disabled by default until operator feedback confirms lane model clarity.
