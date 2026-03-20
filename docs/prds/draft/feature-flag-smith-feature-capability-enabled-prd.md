---
id: prd-ff-smith-feature-capability-enabled
title: PRD for SMITH_FEATURE_CAPABILITY_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_CAPABILITY_ENABLED

## Overview

`SMITH_FEATURE_CAPABILITY_ENABLED` gates the Feature Capability runtime surface (`/feature-capability`) while backend contracts and operational behavior continue to mature. The flag creates a controlled launch path that protects operators from partially available flows.

This feature is currently gated and under development.

## Goals

- Disable Feature Capability by default.
- Enable controlled rollout of the route in test environments.
- Prevent navigation exposure when backend endpoints are not ready.

## Non-Goals

- Finalizing all Feature Capability backend APIs in this PRD.
- Changing authorization semantics beyond existing behavior.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Feature Capability surface respects runtime gate

As an operator, I want the Feature Capability route to appear only when explicitly enabled so that I do not run into dead-end flows.

#### Acceptance Criteria

- When `SMITH_FEATURE_CAPABILITY_ENABLED=false`, nav item and `/feature-capability` route are hidden/redirected.
- When `SMITH_FEATURE_CAPABILITY_ENABLED=true`, nav item and route are available.
- State remains deterministic across refreshes and deep links.
- Negative path: disabled flag with cached route state returns access-denied/redirect behavior and blocks submission actions.

### US-002: Document operational value of the gate

As a release manager, I want explicit documentation for this gate so rollout decisions are predictable.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks this feature as gated and under development.
- Documentation explains value: safer phased rollout while dependencies mature.
- Documentation points to the exact runtime env/Helm controls.
- Negative path: docs explicitly state that enabling the UI flag without backend readiness causes API errors and is unsupported.
