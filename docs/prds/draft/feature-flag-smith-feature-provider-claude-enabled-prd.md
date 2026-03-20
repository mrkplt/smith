---
id: prd-ff-smith-feature-provider-claude-enabled
title: PRD for SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED

## Overview

`SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED` gates Claude visibility in console provider selection and configuration workflows. This allows incremental provider expansion without exposing unfinished multi-provider UX to all operators.

This feature is currently gated and under development.

## Goals

- Keep Claude hidden in console provider UI by default.
- Allow controlled UI rollout when Claude path is validated.
- Align provider visibility with operator expectations.

## Non-Goals

- Enabling Claude API support by itself (API has a separate flag).
- Defining final multi-provider ranking/routing policies.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Claude provider UI appears only when enabled

As an operator, I want Claude options to appear only when explicitly enabled so that provider choices match supported rollout state.

#### Acceptance Criteria

- With `SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED=false`, Claude is absent from provider pickers/editors.
- With `SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED=true`, Claude appears in provider pickers/editors.
- Existing Codex flows remain unaffected in either state.
- Negative path: stale cached catalog state does not reveal Claude when flag is turned off.

### US-002: Document value of provider-UI gating

As a platform owner, I want Claude UI gating documented so rollout sequencing is explicit.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks Claude provider UI as gated and under development.
- Documentation explains value: safer staged provider expansion with lower operator confusion.
- Documentation references companion API gate requirements.
- Negative path: docs state that enabling UI flag without API flag leaves Claude create/update requests rejected.
