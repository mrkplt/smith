# ADR 0018: Retention Cleanup via smith-daemon with ConfigMap Policy Reload

- Status: Accepted
- Date: 2026-03-17
- Related commits: `97b666b`, `4a74932`

## Context

Terminal loops (`flatline`, `cancelled`, optionally `synced`) accumulated in control-plane state without a policy-driven cleanup controller. Runtime artifacts and control-plane records required cleanup, but destructive UI actions on the Pods surface created safety and UX concerns.

## Decision

Introduce a dedicated `smith-daemon` service to perform retention-based cleanup of terminal loops, with policy sourced from a ConfigMap.

- Run cleanup on a fixed interval with pass timeout and max-delete caps.
- Delete only non-active loops and keep audit trail metadata for each deletion.
- Perform best-effort runtime artifact cleanup (worker Job + labeled Pods) when loop runtime metadata exists.
- Load retention policy from `/etc/smith-daemon/policy.yaml` (ConfigMap mount) and reload policy before each pass.
- Keep manual operator endpoint (`POST /v1/loops/cleanup`) for explicit controlled cleanup, but remove bulk clear/delete from Console UX.

## Consequences

- Cleanup behavior is centralized, consistent, and policy-driven.
- Retention tuning becomes operational configuration (Helm/ConfigMap) instead of code edits.
- Daemon introduces a new always-on control-plane component that must be monitored and rolled out.
- Runtime cleanup remains best-effort; failures are captured via logs/audit metadata and do not block record deletion.

## Related ADRs

- [ADR 0001 - etcd State Machine with Kubernetes Job Execution](0001-etcd-state-machine-and-kubernetes-jobs.md)
- [ADR 0005 - Completion Saga for Code and State Sync](0005-completion-saga-for-code-and-state-sync.md)
- [ADR 0010 - Helm Chart as Deployment Contract](0010-helm-chart-as-deployment-contract.md)
