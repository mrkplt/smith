---
id: prd-ff-smith-feature-provider-gemini-enabled
title: PRD for SMITH_FEATURE_PROVIDER_GEMINI_ENABLED
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_PROVIDER_GEMINI_ENABLED

## Overview

`SMITH_FEATURE_PROVIDER_GEMINI_ENABLED` gates Gemini visibility in console provider selection and configuration workflows. It lets operators roll out Gemini support incrementally without exposing unfinished provider UX broadly.

This feature is currently gated and under development.

## Goals

- Keep Gemini hidden in console UI by default.
- Allow explicit enablement in test environments.
- Maintain consistent provider UX expectations during rollout.

## Non-Goals

- Enabling Gemini API support by itself (API has a separate flag).
- Implementing advanced provider arbitration logic.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Gemini provider UI appears only when enabled

As an operator, I want Gemini options shown only when enabled so visible providers match supported runtime state.

#### Acceptance Criteria

- With `SMITH_FEATURE_PROVIDER_GEMINI_ENABLED=false`, Gemini is absent from provider pickers/editors.
- With `SMITH_FEATURE_PROVIDER_GEMINI_ENABLED=true`, Gemini appears in provider pickers/editors.
- Existing Codex flows remain stable in both states.
- Negative path: deep-linking into cached form state cannot force Gemini visibility when disabled.

### US-002: Document rollout value and dependencies

As a release manager, I want clear documentation for Gemini UI gating so deployment behavior is predictable.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks Gemini provider UI as gated and under development.
- Documentation explains value: controlled provider expansion with reduced UX risk.
- Documentation references companion API gating requirements.
- Negative path: docs state that enabling UI flag without API flag leaves Gemini create/update requests rejected.
