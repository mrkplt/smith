# ADR 0002: Single-Writer Locking and Reconciliation Repair Loop

- Status: Accepted
- Date: 2026-03-07
- Related commits: `a4ac13f`, `1e10078`

## Context

Multiple controllers and retries can produce duplicate scheduling, race conditions, and zombie runtime jobs unless loop ownership and drift handling are explicit.

## Decision

Enforce one active mutating worker per loop using lease-backed etcd locks with compare-and-swap semantics, and run a reconciliation loop to repair etcd/runtime drift.

- Lock key contract: `/smith/v1/locks/{loop_id}` with holder and heartbeat.
- Mutating state transitions require revision checks and valid ownership.
- Reconciler handles stale/missing/failed runtime states and zombie jobs.

## Consequences

- Split-brain writes and concurrent mutation risk are reduced.
- The system can auto-heal many runtime/state mismatches without operator action.
- Operational complexity increases around lock renewal, stale detection, and reconcile policy tuning.

## Related ADRs

- [ADR 0001 - etcd State Machine with Kubernetes Job Execution](0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0005 - Completion Saga for Code/State Consistency](0005-completion-saga-for-code-and-state-sync.md)
- [ADR 0010 - Helm Chart as the Deployment Contract](0010-helm-chart-as-deployment-contract.md)
