---
id: prd-ff-smith-feature-prd-diagnostic-resolve-enabled
title: PRD for SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED

## Overview

`SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED` gates the Documents PRD diagnostic `Resolve` action so AI-assisted remediation can be rolled out safely. This allows teams to keep core validation visible while limiting automation until quality is proven.

This feature is currently gated and under development.

## Goals

- Keep diagnostic resolve actions disabled by default.
- Enable targeted rollout for teams validating remediation quality.
- Preserve baseline diagnostics visibility even when resolve is disabled.

## Non-Goals

- Replacing underlying PRD validation rules.
- Shipping fully autonomous PRD rewriting in this phase.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: PRD diagnostic resolve action is feature-gated

As an operator, I want the `Resolve` action available only when enabled so remediation controls match deployment readiness.

#### Acceptance Criteria

- With `SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED=false`, `Resolve` action is not shown.
- With `SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED=true`, `Resolve` action is shown for eligible diagnostics.
- Validation notifications remain visible in both states.
- Negative path: disabled flag prevents resolve action even if stale UI state is present.

### US-002: Document value and rollout constraints

As a release manager, I want clear docs so rollout risk for AI-assisted remediation is controlled.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks this capability as gated and under development.
- Documentation explains value: controlled introduction of AI-assisted remediation while preserving human review.
- Documentation includes exact flag configuration path.
- When the flag is disabled, docs explicitly state that resolve attempts are blocked and return an unavailable/error outcome.
