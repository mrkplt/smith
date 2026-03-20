---
id: prd-ff-smith-provider-claude-enabled
title: PRD for SMITH_PROVIDER_CLAUDE_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_PROVIDER_CLAUDE_ENABLED

## Overview

`SMITH_PROVIDER_CLAUDE_ENABLED` gates Claude support in `smith-api` provider catalog and provider profile validation. This prevents unsupported provider types from being accepted before backend support is operationally validated.

This feature is currently gated and under development.

## Goals

- Keep Claude API provider type disabled by default.
- Allow controlled enablement in environments prepared for Claude.
- Ensure backend rejects unsupported provider types when disabled.

## Non-Goals

- Managing console visibility directly (handled by separate UI flag).
- Implementing provider-specific usage/billing controls in this phase.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Claude API provider type is environment-gated

As a platform owner, I want backend provider enablement gated so unsupported provider types cannot be configured accidentally.

#### Acceptance Criteria

- With `SMITH_PROVIDER_CLAUDE_ENABLED=false`, provider create/update rejects `claude` type.
- With `SMITH_PROVIDER_CLAUDE_ENABLED=true`, provider catalog and create/update allow `claude` type.
- Error responses are actionable when the type is disabled.
- Negative path: alias input (for example `anthropic`) does not bypass disable checks.

### US-002: Document backend rollout value

As a release manager, I want API gating documented so provider rollout is staged safely.

#### Depends On

- US-001

#### Acceptance Criteria

- Negative path: with `SMITH_PROVIDER_CLAUDE_ENABLED=false`, `POST /v1/providers` using `provider_type=claude` is rejected with an actionable error response.
- Docs pages that reference Claude provider behavior include a visible "gated and under development" notice.
- Docs include exact controls (`api.featureFlags.providerClaude`, `SMITH_PROVIDER_CLAUDE_ENABLED`) and a disabled-by-default example.
- Value statement is explicit: this flag prevents accidental provider enablement in unready environments.
