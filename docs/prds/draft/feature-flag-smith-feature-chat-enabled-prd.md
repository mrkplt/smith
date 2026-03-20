---
id: prd-ff-smith-feature-chat-enabled
title: PRD for SMITH_FEATURE_CHAT_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_CHAT_ENABLED

## Overview

`SMITH_FEATURE_CHAT_ENABLED` gates chat entry points (TopBar chat button, `/assistant`, and document chat affordances). This enables phased delivery while keeping core loop workflows stable when chat behavior is still evolving.

This feature is currently gated and under development.

## Goals

- Keep chat surfaces disabled by default.
- Provide a single switch to expose or hide all chat entry points.
- Avoid orphaned route links when chat service is not ready.

## Non-Goals

- Reworking chat model/provider internals.
- Defining long-term chat product policy.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Chat surfaces are consistently gated

As an operator, I want chat controls to appear only when enabled so that the UI reflects actual runtime capability.

#### Acceptance Criteria

- With `SMITH_FEATURE_CHAT_ENABLED=false`, chat button, `/assistant`, and document chat entry points are hidden.
- With `SMITH_FEATURE_CHAT_ENABLED=true`, those surfaces are visible and reachable.
- Settings Chat section follows the same flag behavior.
- Negative path: direct `/assistant` navigation while disabled does not expose chat UI.

### US-002: Document operator value and expected behavior

As a release manager, I want clear docs for chat gating so support teams can reason about availability.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation explicitly marks chat as gated and under development.
- Documentation explains value: controlled release of assistant functionality without destabilizing base workflows.
- Documentation includes exact flag and Helm mapping.
