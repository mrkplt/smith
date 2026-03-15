# ADR 0015: Local CI Parity via `act`

- Status: Accepted
- Date: 2026-03-13
- Related commits: `c948785`

## Context

Local validation behavior had diverged from GitHub Actions execution, creating inconsistent pre-merge outcomes and slower troubleshooting.

## Decision

Use `act` as the local CI parity mechanism for running GitHub Actions workflows, replacing custom wrapper behavior.

- Local CI path maps to workflow job semantics.
- Repository tooling and docs align around `make ci-local-act`.
- Docker-backed execution is the expected local parity substrate.

## Consequences

- CI failures are easier to reproduce before push.
- Workflow maintenance is centralized around shared Actions definitions.
- Contributors need `act` and Docker for full local parity runs.

## Related ADRs

- [ADR 0017 - Standardize CI Runtimes and Containerize Docs Build](0017-standardize-ci-runtimes-and-docs-build-path.md)
- [ADR 0013 - Migrate Operator Frontend to Svelte 5](0013-migrate-operator-frontend-to-svelte5.md)
- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
