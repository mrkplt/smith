# ADR 0011: Mount PRD Input into Runtime via ConfigMap

- Status: Accepted
- Date: 2026-03-09
- Related commits: `1748ffc`

## Context

Replicas needed deterministic access to PRD/task input at execution time without relying on transient API memory.

## Decision

Provide workspace PRD/task input to runtime jobs through Kubernetes ConfigMap mounts.

- PRD context is materialized and mounted into replica runtime.
- Runtime reads mounted artifacts as authoritative planning input.
- Input delivery is tied to job spec generation for traceability.

## Consequences

- Replica planning context is reproducible per execution attempt.
- Input provenance is clearer in Kubernetes runtime artifacts.
- ConfigMap lifecycle and size constraints must be managed operationally.

## Related ADRs

- [ADR 0010 - Helm Chart as the Deployment Contract](0010-helm-chart-as-deployment-contract.md)
- [ADR 0016 - Enforce PRD Readiness as an Ingress Gate](0016-prd-readiness-as-ingress-gate.md)
- [ADR 0003 - Multi-Ingress Loop Creation with Explicit Environment Contract](0003-multi-ingress-and-environment-contract.md)
