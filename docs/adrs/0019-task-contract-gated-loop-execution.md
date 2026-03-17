# ADR 0019: Task-Contract-Gated Loop Execution and Status Synchronization

- Status: Accepted
- Date: 2026-03-17
- Related commits: `97b666b`, `4a74932`

## Context

Direct loop creation lacked a structured approval contract for objective and validation commands, making execution quality and operator traceability inconsistent across ingress paths.

## Decision

Introduce Task Contracts as a first-class control-plane resource and gate loop execution on explicit task approval.

- Add task contract resource model and storage (`draft`, `validated`, `approved`, `running`, `completed`, `blocked`).
- Expose task APIs for create/list/get/patch/approve.
- Require `task_contract_id` to be `approved` when creating a loop from a task.
- Synchronize task execution status from loop lifecycle transitions:
  - `running` -> task `running`
  - `synced` -> task `completed`
  - `flatline`/`cancelled` -> task `blocked`
- Surface task workflow in Console `/tasks` runtime route for operators.

## Consequences

- Loop execution can be preceded by explicit validation/approval workflow.
- Task-to-loop linkage improves auditability and execution traceability.
- API and UI complexity increases due to additional state machine and transition rules.
- CLI parity is partial: provider/project/loop readiness checks exist, but no dedicated `smithctl task` command group yet.

## Related ADRs

- [ADR 0003 - Multi-Ingress and Environment Contract](0003-multi-ingress-and-environment-contract.md)
- [ADR 0008 - smithctl as Primary Operator Interface](0008-smithctl-as-primary-operator-interface.md)
- [ADR 0016 - PRD Readiness as Ingress Gate](0016-prd-readiness-as-ingress-gate.md)
