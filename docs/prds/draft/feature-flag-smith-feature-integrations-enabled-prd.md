---
id: prd-ff-smith-feature-integrations-enabled
title: PRD for SMITH_FEATURE_INTEGRATIONS_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_INTEGRATIONS_ENABLED

## Overview

`SMITH_FEATURE_INTEGRATIONS_ENABLED` gates the Settings Integrations section so unfinished external-system workflows are hidden by default. This reduces operator confusion and supports safer phased rollout.

This feature is currently gated and under development.

## Goals

- Keep Integrations section hidden by default.
- Allow controlled visibility in test environments.
- Prevent preview-only integrations UI from appearing in production by accident.

## Non-Goals

- Implementing all integration backends in this phase.
- Replacing long-term integration lifecycle management.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Integrations section visibility follows runtime flag

As an operator, I want Integrations visible only when enabled so settings navigation matches deployment readiness.

#### Acceptance Criteria

- With `SMITH_FEATURE_INTEGRATIONS_ENABLED=false`, Integrations section is absent from Settings navigation.
- With `SMITH_FEATURE_INTEGRATIONS_ENABLED=true`, Integrations section appears in Settings navigation.
- Deep-linking to `?section=integrations` while disabled redirects to a safe section.
- Negative path: cached browser state cannot force Integrations rendering when the flag is disabled.

### US-002: Documentation explains rollout value and guardrails

As a platform owner, I want documentation that explains the purpose of the Integrations gate so teams can enable it safely.

#### Depends On

- US-001

#### Acceptance Criteria

- Docs pages that mention Integrations include a visible "gated and under development" notice.
- Docs include exact controls (`console.featureFlags.integrations`, `SMITH_FEATURE_INTEGRATIONS_ENABLED`) and disabled-by-default behavior.
- Value statement is explicit: this flag prevents exposing incomplete integrations workflows to operators.
- Negative path: docs state that Integrations is unavailable when the flag is disabled.
