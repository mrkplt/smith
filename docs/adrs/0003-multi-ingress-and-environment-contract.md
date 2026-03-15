# ADR 0003: Multi-Ingress Loop Creation with Explicit Environment Contract

- Status: Accepted
- Date: 2026-03-07
- Related commits: `21e90c0`, `6b05181`

## Context

Smith needed one consistent way to ingest work from different operator entry points while keeping runtime setup deterministic and auditable.

## Decision

Adopt a unified loop creation contract that supports GitHub issue ingress, PRD ingress, and direct interactive creation, and persist normalized environment configuration on each loop.

- Ingress routes map source payloads into a common loop/anomaly shape.
- Loop environment supports one source mode at a time (`mise`, `container_image`, or `dockerfile`) plus preset/env overlays.
- Validation rejects ambiguous or unsafe environment combinations.

## Consequences

- Operators can create loops from multiple workflows without changing core execution semantics.
- Runtime behavior is reproducible because environment resolution is explicit and stored.
- API validation and error messaging become a critical compatibility surface.

## Related ADRs

- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
- [ADR 0010 - Helm Chart as the Deployment Contract](0010-helm-chart-as-deployment-contract.md)
- [ADR 0016 - Enforce PRD Readiness as an Ingress Gate](0016-prd-readiness-as-ingress-gate.md)
