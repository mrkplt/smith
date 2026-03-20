---
id: prd-ff-smith-feature-capability-access
title: PRD for SMITH_FEATURE_CAPABILITY_ACCESS
status: draft
doc_type: prd
source_doc: docs/planning/proposed/feature-flags-roadmap.md
target_branch: document-drafting
owner: lars
---

# Feature Flag PRD: SMITH_FEATURE_CAPABILITY_ACCESS

## Overview

`SMITH_FEATURE_CAPABILITY_ACCESS` provides an explicit access override for Feature Capability. It adds a coarse-grained operational safety lever so platform owners can keep the route available only to trusted operators even when the feature is enabled.

This feature is currently gated and under development.

## Goals

- Preserve an explicit access override for controlled testing.
- Keep permission checks deterministic and auditable.
- Reduce accidental exposure during staged rollouts.

## Non-Goals

- Replacing long-term RBAC/permission architecture.
- Introducing tenant-level policy systems.

## Quality Gates

- go test ./...
- npm --prefix frontend run check
- helm lint helm/smith

## Stories

### US-001: Access override enforces allow/deny behavior

As a platform owner, I want a deterministic override so that only intended operators can access Feature Capability during rollout.

#### Acceptance Criteria

- When override is false and operator permission is missing, access is denied.
- When override is true, access check allows route entry for rollout testing.
- Denied users are redirected to access-denied UX.
- Negative path: malformed permissions input does not bypass deny behavior.

### US-002: Explain security and rollout value in docs

As a security reviewer, I want this control documented so risk posture is explicit.

#### Depends On

- US-001

#### Acceptance Criteria

- Documentation marks this control as gated and under development.
- Documentation explains value: temporary guardrail for controlled enablement.
- Documentation clarifies relationship between override and operator permission list.
- Negative path: docs include explicit denied-access behavior when override is false and required permission is missing.
