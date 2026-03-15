# ADR 0017: Standardize CI Runtimes and Containerize Docs Build

- Status: Accepted
- Date: 2026-03-15
- Related commits: `147cb36`, `2a819c3`

## Context

CI and docs build behavior varied by host environment, which made failures harder to reproduce and introduced avoidable dependency drift.

## Decision

Standardize CI jobs on repository-managed `mise` runtimes and run the docs site build through a dedicated containerized tooling path.

- Workflow jobs resolve runtime versions via `mise`.
- Docs generation no longer depends on host Python/Zensical installation.
- Build behavior is aligned between local and CI environments.

## Consequences

- Pipeline behavior is more deterministic and portable.
- Docs publishing is less sensitive to developer machine setup.
- CI/docs tooling images and runtime pins require routine maintenance.

## Related ADRs

- [ADR 0015 - Local CI Parity via act](0015-local-ci-parity-via-act.md)
- [ADR 0009 - Contract-First API Surface (Swagger, gRPC, Go Client, MCP)](0009-contract-first-api-grpc-client-and-mcp.md)
- [ADR 0010 - Helm Chart as the Deployment Contract](0010-helm-chart-as-deployment-contract.md)
