---
id: prd-ff-smith-provider-gemini-enabled
title: PRD for SMITH_PROVIDER_GEMINI_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_PROVIDER_GEMINI_ENABLED

## Overview

`SMITH_PROVIDER_GEMINI_ENABLED` gates Gemini support in `smith-api` provider catalog and provider profile validation. This keeps backend contracts safe until Gemini provider behavior is validated in controlled environments.

This feature is currently gated and under development.

## Goals

- Keep Gemini API provider type disabled by default.
- Allow intentional backend enablement when environment readiness is confirmed.
- Maintain strict rejection behavior when disabled.

## Non-Goals

- Controlling console provider visibility (handled by separate UI gate).
- Defining final provider fallback policy.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Gemini API provider type is environment-gated

As a platform owner, I want Gemini backend availability controlled by flag so unsupported types are not accepted by accident.

#### Acceptance Criteria

- With `SMITH_PROVIDER_GEMINI_ENABLED=false`, provider create/update rejects `gemini` type.
- With `SMITH_PROVIDER_GEMINI_ENABLED=true`, provider catalog and create/update allow `gemini` type.
- Disabled-state errors are explicit and actionable.
- Negative path: alias input (for example `google`) does not bypass disable checks.

### US-002: Document backend value and rollout process

As a release manager, I want documentation that explains why Gemini backend support is gated.

#### Depends On

- US-001

#### Acceptance Criteria

- Negative path: with `SMITH_PROVIDER_GEMINI_ENABLED=false`, `POST /v1/providers` using `provider_type=gemini` is rejected with an actionable error response.
- Docs pages that reference Gemini provider behavior include a visible "gated and under development" notice.
- Docs include exact controls (`api.featureFlags.providerGemini`, `SMITH_PROVIDER_GEMINI_ENABLED`) and a disabled-by-default example.
- Value statement is explicit: this flag prevents accidental Gemini enablement before backend readiness.
