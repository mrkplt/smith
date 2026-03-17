# Documentation Audit (chat-wip)

Audit baseline: `chat-wip` at `438edfe` (plus documentation corrections on `td-b2b668-documentation_update`).

## Scope and method

- Reviewed commit range `origin/main..chat-wip`.
- Cross-checked runtime/API/navigation claims against current code in:
  - `cmd/smith-api`
  - `cmd/smith-daemon`
  - `cmd/smithctl`
  - `frontend/src`
  - `helm/smith`
  - `Makefile`
- Reconciled documentation where behavior had drifted.
- `make docs-check` passes after updates.

## High-impact corrections made during audit

- Documented daemon retention cleanup and policy ConfigMap behavior.
- Documented Task Contract lifecycle and loop synchronization behavior.
- Corrected loop ingress doc to use `smithctl` examples consistently.
- Corrected loop ingress API notes: batch create is `POST /v1/loops` with `loops[]` payload (no `/v1/loops/batch` route).
- Added explicit caveat that Feature Capability backend endpoints are not currently implemented by `smith-api`.

## Open caveats (known and now documented)

- Feature Capability UI route exists (`/feature-capability`) and client contract is defined in frontend, but `smith-api` does not currently register `/v1/feature-capability/*` handlers.

## Doc-by-doc status

Status keys:

- `validated-current`: statements align with current code/runtime behavior.
- `validated-current-with-caveat`: aligns, with explicit caveat noted.
- `archival-design-record`: ADR/PRD/spec record; not treated as a live runtime contract.
- `reference-example`: illustrative example content.

### Core and operations docs

- `docs/index.md` - `validated-current`
- `docs/contributing.md` - `validated-current`
- `docs/local-dev-make-workflow.md` - `validated-current`
- `docs/local-integration-environment.md` - `validated-current`
- `docs/make-local-quickstart.md` - `validated-current`
- `docs/loop-ingress-and-cli.md` - `validated-current`
- `docs/settings-api-and-console.md` - `validated-current`
- `docs/smithctl-installation-and-usage.md` - `validated-current`
- `docs/loop-retention-cleanup.md` - `validated-current`
- `docs/task-contracts-and-feature-capability.md` - `validated-current-with-caveat`
- `docs/changelog.md` - `validated-current`
- `docs/prd-implementation-status.md` - `validated-current`
- `docs/agent-provider-auth.md` - `validated-current`
- `docs/deployment-recommendations.md` - `validated-current`
- `docs/helm-upgrade-rollback-runbook.md` - `validated-current`
- `docs/pre-release-system-gate.md` - `validated-current`
- `docs/staging-soak-chaos-runbook.md` - `validated-current`
- `docs/cluster-autoscaler-prerequisites-runbook.md` - `validated-current`
- `docs/distributed-autonomous-orchestration-mvp.md` - `validated-current`
- `docs/prd-authoring-workflow.md` - `validated-current`
- `docs/requirements-fr-nfr.md` - `validated-current`
- `docs/test-harness-strategy.md` - `validated-current`
- `docs/repository-auth-options.md` - `validated-current`
- `docs/requirements-traceability.md` - `validated-current`
- `docs/skill-volume-mounts.md` - `validated-current`
- `docs/reconciliation-loop.md` - `validated-current`
- `docs/mvp-boundary-and-release-gates.md` - `validated-current`
- `docs/multi-provider-skill-mount-abstraction.md` - `validated-current`
- `docs/observability-latency-validation.md` - `validated-current`
- `docs/kubernetes-secrets-encryption-provider-runbook.md` - `validated-current`
- `docs/journal-retention-archival-policy.md` - `validated-current`
- `docs/loop-environment-profiles.md` - `validated-current`
- `docs/lock-strategy.md` - `validated-current`
- `docs/image-tagging-versioning.md` - `validated-current`
- `docs/etcd-key-schema.md` - `validated-current`
- `docs/git-history-policy.md` - `validated-current`
- `docs/backup-restore-dr-runbook.md` - `validated-current`
- `docs/completion-commit-protocol.md` - `validated-current`
- `docs/docs-to-prd-lifecycle.md` - `validated-current`
- `docs/docs-site-github-pages.md` - `validated-current`
- `docs/docs-site-style-sidecar.md` - `validated-current`
- `docs/technology-stack-and-thanks.md` - `validated-current`

### Historical architecture/spec docs

- `docs/prd1.md` - `archival-design-record`

### Approved PRDs

- `docs/prds/approved/smith_mvp_execution_flow_prd.md` - `archival-design-record`
- `docs/prds/approved/smith_settings_ui_architecture_prd.md` - `archival-design-record`
- `docs/prds/approved/smith_github_auth_prd.md` - `archival-design-record`
- `docs/prds/approved/smith_cli_architecture_prd.md` - `archival-design-record`

### ADRs

- `docs/adrs/0001-etcd-state-machine-and-kubernetes-jobs.md` - `archival-design-record`
- `docs/adrs/0002-single-writer-locking-and-reconciliation.md` - `archival-design-record`
- `docs/adrs/0003-multi-ingress-and-environment-contract.md` - `archival-design-record`
- `docs/adrs/0004-provider-registry-and-adapter-interface.md` - `archival-design-record`
- `docs/adrs/0005-completion-saga-for-code-and-state-sync.md` - `archival-design-record`
- `docs/adrs/0006-provider-credentials-in-kubernetes-secrets.md` - `archival-design-record`
- `docs/adrs/0007-dedicated-chat-service-boundary.md` - `archival-design-record`
- `docs/adrs/0008-smithctl-as-primary-operator-interface.md` - `archival-design-record`
- `docs/adrs/0009-contract-first-api-grpc-client-and-mcp.md` - `archival-design-record`
- `docs/adrs/0010-helm-chart-as-deployment-contract.md` - `archival-design-record`
- `docs/adrs/0011-mount-prd-into-runtime-via-configmap.md` - `archival-design-record`
- `docs/adrs/0012-provider-specific-runtime-invocation.md` - `archival-design-record`
- `docs/adrs/0013-migrate-operator-frontend-to-svelte5.md` - `archival-design-record`
- `docs/adrs/0014-stable-short-hash-id-generation.md` - `archival-design-record`
- `docs/adrs/0015-local-ci-parity-via-act.md` - `archival-design-record`
- `docs/adrs/0016-prd-readiness-as-ingress-gate.md` - `archival-design-record`
- `docs/adrs/0017-standardize-ci-runtimes-and-docs-build-path.md` - `archival-design-record`
- `docs/adrs/0018-daemon-retention-cleanup-controller.md` - `archival-design-record`
- `docs/adrs/0019-task-contract-gated-loop-execution.md` - `archival-design-record`

### Examples

- `docs/examples/prd-authoring/valid-prd.md` - `reference-example`

## Recommended ongoing process

- Treat `docs/documentation-audit-chat-wip.md` as the current audit ledger for this branch.
- Re-run this audit on large branch updates, especially when API routes, frontend navigation, or Make/Helm contracts change.
