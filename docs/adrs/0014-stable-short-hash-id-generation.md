# ADR 0014: Stable Short-Hash ID Generation

- Status: Accepted
- Date: 2026-03-13
- Related commits: `fb23533`, `d179b4d`

## Context

Loop/task identifiers needed to remain compact for operator use while avoiding instability and collisions caused by ad-hoc ID generation.

## Decision

Use a stable short-hash strategy for generated IDs.

- IDs are derived deterministically from stable input.
- Output remains concise for CLI/UI usage.
- Generation avoids weak/legacy hash behavior previously used in paths.

## Consequences

- Operator and automation references are more predictable.
- Correlation across logs/state/artifacts is easier.
- ID format and derivation become a compatibility contract across components.

## Related ADRs

- [ADR 0001 - etcd State Machine with Kubernetes Job Execution](0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0005 - Completion Saga for Code/State Consistency](0005-completion-saga-for-code-and-state-sync.md)
- [ADR 0009 - Contract-First API Surface (Swagger, gRPC, Go Client, MCP)](0009-contract-first-api-grpc-client-and-mcp.md)
