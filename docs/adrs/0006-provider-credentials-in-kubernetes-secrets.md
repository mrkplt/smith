# ADR 0006: Store Provider Credentials in Kubernetes Secrets

- Status: Accepted
- Date: 2026-03-09
- Related commits: `9df0b6d`, `d20d334`

## Context

Provider authentication was required for runtime calls, but storing tokens in loop records or etcd keys would violate security boundaries and increase disclosure risk.

## Decision

Persist provider credentials in Kubernetes Secrets (via token-store abstraction), not in etcd orchestration keys.

- Use secure backend storage for access/refresh tokens.
- Keep loop state and metadata token-free.
- Expose auth lifecycle endpoints for connect/status/disconnect flows.

## Consequences

- Secret material is isolated from orchestration state and journal records.
- Runtime can refresh credentials without polluting loop data contracts.
- Deployments require correct Kubernetes RBAC and secret management discipline.

## Related ADRs

- [ADR 0004 - Provider Registry with Adapter Interface (Codex-First)](0004-provider-registry-and-adapter-interface.md)
- [ADR 0012 - Provider-Specific Runtime Invocation](0012-provider-specific-runtime-invocation.md)
- [ADR 0007 - Run Chat Workloads in a Dedicated smith-chat Service](0007-dedicated-chat-service-boundary.md)
