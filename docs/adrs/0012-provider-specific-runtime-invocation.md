# ADR 0012: Provider-Specific Runtime Invocation

- Status: Accepted
- Date: 2026-03-09
- Related commits: `97e75db`

## Context

Different agent providers do not share identical runtime invocation semantics, but loop execution still needs a stable orchestration contract.

## Decision

Select runtime agent command/invocation behavior by provider while keeping orchestration lifecycle contracts provider-neutral.

- Provider is resolved from loop/provider configuration.
- Runtime command selection is delegated to provider-specific behavior.
- Core loop state transitions remain unchanged regardless of provider.

## Consequences

- New providers can integrate without rewriting core lifecycle logic.
- Provider behavior differences are isolated to invocation/adapters.
- Runtime validation and testing must cover provider-specific invocation paths.

## Related ADRs

- [ADR 0004 - Provider Registry with Adapter Interface (Codex-First)](0004-provider-registry-and-adapter-interface.md)
- [ADR 0006 - Store Provider Credentials in Kubernetes Secrets](0006-provider-credentials-in-kubernetes-secrets.md)
- [ADR 0003 - Multi-Ingress Loop Creation with Explicit Environment Contract](0003-multi-ingress-and-environment-contract.md)
