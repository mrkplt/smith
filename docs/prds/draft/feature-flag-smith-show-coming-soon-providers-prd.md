---
id: prd-ff-smith-show-coming-soon-providers
title: PRD for SMITH_SHOW_COMING_SOON_PROVIDERS
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_SHOW_COMING_SOON_PROVIDERS

## Overview

`SMITH_SHOW_COMING_SOON_PROVIDERS` controls whether "coming soon" provider hints are displayed in console UX. This helps balance roadmap communication with operator clarity in production deployments.

This feature is currently gated and under development.

## Goals

- Keep roadmap teaser hints disabled by default.
- Allow opt-in visibility for internal demos or preview environments.
- Prevent confusion in environments where only fully supported providers should be shown.

## Non-Goals

- Enabling provider functionality by itself.
- Replacing provider readiness documentation.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Coming-soon provider hints are explicitly gated

As an operator, I want roadmap hints shown only when intended so production UX stays focused on supported capabilities.

#### Acceptance Criteria

- With `SMITH_SHOW_COMING_SOON_PROVIDERS=false`, coming-soon hints are hidden.
- With `SMITH_SHOW_COMING_SOON_PROVIDERS=true`, coming-soon hints are visible in configured UI surfaces.
- Supported providers remain unchanged in both states.
- Negative path: hint-only providers cannot be selected for active configuration.

### US-002: Document value and usage boundaries

As a product owner, I want this control documented so operators understand the communication-vs-stability tradeoff.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks this behavior as gated and under development.
- Documentation explains value: optional roadmap visibility without production noise.
- Documentation clarifies that hints do not imply provider availability.
