# ADR 0001: etcd State Machine with Kubernetes Job Execution

- Status: Accepted
- Date: 2026-03-07
- Related commits: `d8424b3`, `a4ac13f`, `57a5775`, `e5e79aa`

## Context

Smith needed a distributed control model that could survive process restarts, support multiple controllers/workers, and preserve an auditable lifecycle for every loop.

## Decision

Use `etcd` as the authoritative state machine for loop lifecycle and metadata, and use Kubernetes Jobs as the execution substrate for replicas.

- Loop lifecycle is persisted in etcd (`unresolved -> running -> synced|flatline|cancelled`).
- `smith-core` watches unresolved work in etcd and schedules `smith-replica` Jobs.
- Kubernetes runtime state is treated as operational reality that must reconcile back to etcd state.

## Consequences

- Recovery and replay are deterministic because loop truth is persisted in etcd.
- Horizontal scale is straightforward because work scheduling and execution are decoupled.
- The platform must maintain clear etcd schema contracts and controller reconciliation logic.

## Related ADRs

- [ADR 0002 - Single-Writer Locking and Reconciliation](0002-single-writer-locking-and-reconciliation.md)
- [ADR 0005 - Completion Saga for Code/State Consistency](0005-completion-saga-for-code-and-state-sync.md)
- [ADR 0010 - Helm Chart as the Deployment Contract](0010-helm-chart-as-deployment-contract.md)
