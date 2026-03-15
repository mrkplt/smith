# ADR 0005: Completion Saga for Code/State Consistency

- Status: Accepted
- Date: 2026-03-09
- Related commits: `e049050`

## Context

A loop can produce valid code changes while orchestration state finalization fails, creating split-brain outcomes between Git history and etcd lifecycle state.

## Decision

Use a completion saga that treats `git push` and etcd terminal state transition as a coordinated workflow with explicit compensation.

- Only transition to `synced` after remote push succeeds.
- Persist completion phases for crash-safe recovery.
- If state finalize fails after push, trigger compensation (`git revert`) and return loop to retryable state.

## Consequences

- A loop is never marked `synced` unless repository and orchestration state agree.
- Recovery behavior is explicit and auditable across failures.
- Completion path is more complex and requires robust compensation/error handling.

## Related ADRs

- [ADR 0001 - etcd State Machine with Kubernetes Job Execution](0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0002 - Single-Writer Locking and Reconciliation](0002-single-writer-locking-and-reconciliation.md)
- [ADR 0014 - Stable Short-Hash ID Generation](0014-stable-short-hash-id-generation.md)
