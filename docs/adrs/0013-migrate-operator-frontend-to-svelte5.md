# ADR 0013: Migrate Operator Frontend to Svelte 5

- Status: Accepted
- Date: 2026-03-10
- Related commits: `f884288`, `a09c7dd`

## Context

The prior console implementation was limiting iteration speed for chat/documents/operator UX and had accumulated complexity in routing and UI logic.

## Decision

Adopt a Svelte 5 frontend architecture under `frontend/` and retire the legacy console path.

- Frontend source of truth moves to `frontend/`.
- Legacy console implementation is removed after migration.
- Operator UI evolution continues on Svelte component boundaries.

## Consequences

- Frontend iteration and testing are more consistent with modern toolchain workflows.
- UI architecture is clearer for subsequent chat/documents features.
- Build/test/deploy contracts now depend on the Svelte/Vite frontend pipeline.

## Related ADRs

- [ADR 0007 - Run Chat Workloads in a Dedicated smith-chat Service](0007-dedicated-chat-service-boundary.md)
- [ADR 0008 - Make smithctl the Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
- [ADR 0015 - Local CI Parity via act](0015-local-ci-parity-via-act.md)
