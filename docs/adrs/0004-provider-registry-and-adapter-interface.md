# ADR 0004: Provider Registry with Adapter Interface (Codex-First)

- Status: Accepted
- Date: 2026-03-07
- Related commits: `b21ab62`

## Context

The runtime needed to support an MVP provider immediately while avoiding a hard-coded provider dependency in core execution paths.

## Decision

Introduce a provider registry and adapter interface, with Codex as the initial provider implementation.

- Loops carry `provider_id` and `model` fields.
- Provider adapters implement a common session/turn/stream/validation contract.
- Core runtime stays provider-neutral and routes through the selected adapter.

## Consequences

- MVP delivery is unblocked with Codex while preserving a multi-provider extension path.
- Provider-specific behavior is isolated from orchestration state logic.
- Adapter conformance and capability metadata become part of the long-term compatibility surface.

## Related ADRs

- [ADR 0006 - Store Provider Credentials in Kubernetes Secrets](0006-provider-credentials-in-kubernetes-secrets.md)
- [ADR 0012 - Provider-Specific Runtime Invocation](0012-provider-specific-runtime-invocation.md)
- [ADR 0009 - Contract-First API Surface (Swagger, gRPC, Go Client, MCP)](0009-contract-first-api-grpc-client-and-mcp.md)
