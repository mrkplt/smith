# ADR 0007: Run Chat Workloads in a Dedicated `smith-chat` Service

- Status: Accepted
- Date: 2026-03-11
- Related commits: `557a56c`, `a465fc9`

## Context

PRD chat/session traffic and streaming behavior have different scaling and operational characteristics from core loop lifecycle APIs.

## Decision

Split chat responsibilities into a dedicated `smith-chat` service and route chat/session endpoints there.

- `smith-api` remains focused on loop ingress/control and operator surfaces.
- `smith-chat` handles chat session creation, streaming, and chat-action forwarding.
- Deployment and routing are managed as separate service components.

## Consequences

- Chat and control-plane APIs can scale and evolve independently.
- Failure domains are cleaner between chat traffic and lifecycle orchestration.
- Platform topology is more complex (additional service, deployment, and routing contracts).

## Related ADRs

- [ADR 0013 - Migrate Operator Frontend to Svelte 5](0013-migrate-operator-frontend-to-svelte5.md)
- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
- [ADR 0009 - Contract-First API Surface (Swagger, gRPC, Go Client, MCP)](0009-contract-first-api-grpc-client-and-mcp.md)
