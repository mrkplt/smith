# ADR 0010: Helm Chart as the Deployment Contract

- Status: Accepted
- Date: 2026-03-07
- Related commits: `e95a4ea`, `2cdca01`

## Context

Smith needed a repeatable deployment method for control-plane services and replica runtime wiring across local and staged environments.

## Decision

Use `helm/smith` as the canonical deployment contract, including runtime job template wiring and environment-specific values overlays.

- Component deployments/services are defined in one chart.
- Replica job behavior is driven through chart values and templates.
- Environment differences are represented through values files, not divergent manifests.

## Consequences

- Deployments are more reproducible and easier to promote across environments.
- Operational changes can be managed with chart/value diffs and rollback semantics.
- Chart schema and values compatibility become a long-term maintenance surface.

## Related ADRs

- [ADR 0001 - etcd State Machine with Kubernetes Job Execution](0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0003 - Multi-Ingress Loop Creation with Explicit Environment Contract](0003-multi-ingress-and-environment-contract.md)
- [ADR 0011 - Mount PRD Input into Runtime via ConfigMap](0011-mount-prd-into-runtime-via-configmap.md)
