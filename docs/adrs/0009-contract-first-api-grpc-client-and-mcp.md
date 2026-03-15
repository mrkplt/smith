# ADR 0009: Contract-First API Surface (Swagger, gRPC, Go Client, MCP)

- Status: Accepted
- Date: 2026-03-14
- Related commits: `6290b4a`, `1cb4ac7`

## Context

As integration points expanded, Smith needed a clear and reusable contract surface for human operators, automation clients, and tool-based AI integrations.

## Decision

Standardize on a contract-first API strategy with generated/maintained Swagger artifacts, gRPC support, a typed Go client (`pkg/client/v1`), and a dedicated MCP server entrypoint.

- Keep API contracts explicit and discoverable.
- Support both HTTP/OpenAPI and gRPC consumers.
- Provide strongly-typed client access and tool-integration bridge.

## Consequences

- Integrations are easier to build and less coupled to internal package layout.
- Contract drift is more visible and testable across services.
- API evolution must be managed carefully to preserve client compatibility.

## Related ADRs

- [ADR 0004 - Provider Registry with Adapter Interface (Codex-First)](0004-provider-registry-and-adapter-interface.md)
- [ADR 0007 - Run Chat Workloads in a Dedicated smith-chat Service](0007-dedicated-chat-service-boundary.md)
- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
