---
id: prd-ff-smith-feature-secrets-enabled
title: PRD for SMITH_FEATURE_SECRETS_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_SECRETS_ENABLED

## Overview

`SMITH_FEATURE_SECRETS_ENABLED` gates the Settings Secrets UI section while secret-backed provider configuration hardens. This protects operators from partially polished management surfaces while preserving backend secret-ref behavior.

This feature is currently gated and under development.

## Goals

- Keep Secrets UI disabled by default.
- Preserve backend secret workflows even when UI section is hidden.
- Provide explicit enablement for controlled testing.

## Non-Goals

- Changing backend secret storage contracts.
- Building full enterprise secret governance in this phase.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Secrets section visibility follows feature gate

As an operator, I want Secrets settings visible only when enabled so that unfinished secret-management UX is not exposed by default.

#### Acceptance Criteria

- With `SMITH_FEATURE_SECRETS_ENABLED=false`, Settings Secrets section is hidden.
- With `SMITH_FEATURE_SECRETS_ENABLED=true`, Settings Secrets section is visible and usable.
- Provider configuration continues to use `secret_ref` contracts regardless of UI visibility.
- Negative path: direct deep-link attempts to hidden section are rejected/redirected and cannot bypass the gate.

### US-002: Document value and operational expectations

As a platform owner, I want clear docs so teams understand why this gate exists.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks this surface as gated and under development.
- Documentation explains value: safer rollout for secret-management UX while backend remains stable.
- Documentation includes enablement controls and expected defaults.
- Negative path: docs warn that operators cannot manage secrets in Settings when this flag is disabled.
