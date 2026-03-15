# ADR 0008: Make `smithctl` the Primary Operator Interface

- Status: Accepted
- Date: 2026-03-11
- Related commits: `5eb3d59`, `ac29bf9`

## Context

Operators needed a stable, scriptable, automation-friendly interface for loop and PRD workflows that works consistently across local, CI, and cluster environments.

## Decision

Use `smithctl` (kubectl-style CLI) as the primary operator surface for lifecycle and ingress workflows, with context/config management and machine-readable output.

- Resource-oriented commands for loop and PRD operations.
- Context-aware configuration for multi-environment usage.
- JSON-capable output for automation pipelines.

## Consequences

- Operational workflows are reproducible and easy to automate.
- API behavior must remain aligned with CLI UX contracts.
- CLI compatibility and config migration become long-term product commitments.

## Related ADRs

- [ADR 0003 - Multi-Ingress Loop Creation with Explicit Environment Contract](0003-multi-ingress-and-environment-contract.md)
- [ADR 0009 - Contract-First API Surface (Swagger, gRPC, Go Client, MCP)](0009-contract-first-api-grpc-client-and-mcp.md)
- [ADR 0016 - Enforce PRD Readiness as an Ingress Gate](0016-prd-readiness-as-ingress-gate.md)
