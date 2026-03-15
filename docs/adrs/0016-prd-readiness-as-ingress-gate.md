# ADR 0016: Enforce PRD Readiness as an Ingress Gate

- Status: Accepted
- Date: 2026-03-14
- Related commits: `05b20a2`, `824fabc`, `fa57003`

## Context

Loop quality degraded when partially specified PRDs were ingested directly into execution, causing avoidable retries and noisy failure states.

## Decision

Require PRD readiness validation before PRD-derived ingress can create executable loops.

- Add canonical PRD diagnostics/readiness checks.
- Reject non-ready PRD ingress with actionable validation feedback.
- Keep validation policy explicit in docs and workflow tooling.

## Consequences

- Execution loops start from more complete and testable requirements.
- Operators get earlier feedback during authoring instead of runtime failure.
- PRD schema/readiness rules become a maintained contract surface.

## Related ADRs

- [ADR 0003 - Multi-Ingress Loop Creation with Explicit Environment Contract](0003-multi-ingress-and-environment-contract.md)
- [ADR 0011 - Mount PRD Input into Runtime via ConfigMap](0011-mount-prd-into-runtime-via-configmap.md)
- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
