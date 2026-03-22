# Environment Variables (Important)

This page documents high-impact environment variables that affect runtime behavior.

It is intentionally focused on **important** variables (not every internal knob).

## Shared foundation

| Variable | Used by | Purpose |
| --- | --- | --- |
| `SMITH_ETCD_ENDPOINTS` | api, core, daemon, replica | Comma-separated etcd endpoints for state/audit/journal storage. |
| `SMITH_ETCD_DIAL_TIMEOUT` | api, core, daemon | etcd connection timeout duration (for example `5s`). |
| `SMITH_NAMESPACE` | core, api, daemon | Kubernetes namespace context fallback for runtime operations. |
| `SMITH_RUNTIME_NAMESPACE` | api, daemon | Explicit namespace for runtime Jobs/Pods cleanup and terminal attach operations. |

## Console runtime UX flags

These are persisted in ConfigMap `smith-<release>-console-config` and injected into the console container.

> **Notice (Gated / Under Development):** Feature-flagged console surfaces listed below are currently treated as under development and default to disabled unless explicitly enabled.

| Variable | Helm value | Default | Purpose |
| --- | --- | --- | --- |
| `SMITH_API_BASE_URL` | `console.apiBaseUrl` | `/api` | Base path for API calls from the console. |
| `SMITH_CHAT_BASE_URL` | `console.chatBaseUrl` | `/chat` | Base path for chat service calls. |
| `SMITH_OPERATOR_PERMISSIONS` | `console.operatorPermissions` | `""` | Comma-separated permission set used by runtime capability checks. |
| `SMITH_FEATURE_TASKS_ENABLED` | `console.featureFlags.tasks` | `false` | Shows/hides Tasks route + nav and enables route access. |
| `SMITH_FEATURE_CAPABILITY_ENABLED` | `console.featureFlags.featureCapability` | `false` | Shows/hides Feature Capability route + nav and enables route access. |
| `SMITH_FEATURE_CHAT_ENABLED` | `console.featureFlags.chat` | `false` | Shows/hides operator chat surfaces (TopBar chat button, assistant route, and settings chat section). |
| `SMITH_FEATURE_INTEGRATIONS_ENABLED` | `console.featureFlags.integrations` | `false` | Shows/hides Integrations section in Settings. |
| `SMITH_FEATURE_SECRETS_ENABLED` | `console.featureFlags.secrets` | `false` | Shows/hides Secrets section in Settings while retaining backend secret-ref support. |
| `SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED` | `console.featureFlags.providerClaude` | `false` | Shows/hides Claude provider type in console provider configuration surfaces. |
| `SMITH_FEATURE_PROVIDER_GEMINI_ENABLED` | `console.featureFlags.providerGemini` | `false` | Shows/hides Gemini provider type in console provider configuration surfaces. |
| `SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED` | `console.featureFlags.prdDiagnosticResolve` | `false` | Enables per-diagnostic `Resolve` action in Documents PRD readiness notifications. |
| `SMITH_FEATURE_CAPABILITY_ACCESS` | `console.featureCapabilityAccess` | `false` | Explicit override for Feature Capability access check when feature is enabled. |
| `SMITH_SHOW_COMING_SOON_PROVIDERS` | `console.showComingSoonProviders` | `false` | Toggles provider “coming soon” UX hints. |

## smith-api (control plane API)

| Variable | Default | Purpose |
| --- | --- | --- |
| `SMITH_API_PORT` | `8080` | HTTP API listen port. |
| `SMITH_GRPC_PORT` | `9090` | gRPC listen port for contract-first clients. |
| `SMITH_OPERATOR_TOKEN` | empty | Bearer token required for protected operator endpoints. |
| `SMITH_DEFAULT_ENV_PRESET` | empty | Default loop environment preset when not explicitly provided. |
| `SMITH_RUNTIME_CONTAINER_NAME` | `replica` | Container name used for attach/command/detach operations. |
| `SMITH_PROVIDER_CLAUDE_ENABLED` | `false` | Enables/disables Claude provider type in API provider catalog and provider validation. |
| `SMITH_PROVIDER_GEMINI_ENABLED` | `false` | Enables/disables Gemini provider type in API provider catalog and provider validation. |
| `SMITH_AUTH_STORE_BACKEND` | `file` | Settings credential store backend (`file` or `k8s-secret`) for provider/project credential data. |
| `SMITH_AUTH_STORE_PATH` | `/tmp/smith-auth/tokens.json` | File path for credential/settings persistence when backend is file. |
| `SMITH_AUTH_STORE_K8S_NAMESPACE` | `POD_NAMESPACE`/`default` | Namespace for Kubernetes-backed credential/settings store. |
| `SMITH_AUTH_STORE_K8S_SECRET` | `smith-auth-store` | Secret name for Kubernetes-backed credential/settings store. |
| `SMITH_AUTH_STORE_K8S_KEY` | `tokens.json` | Secret key field used for serialized credential/settings JSON. |
| `SMITH_SKILL_ALLOWED_SOURCES` | defaults from policy | Restricts allowed skill mount source prefixes. |
| `SMITH_SKILL_ALLOW_WRITABLE` | policy default | Enables/disables writable skill mounts. |
| `SMITH_DOCUMENT_STORE_BACKEND` | `etcd` | Document store backend selector (`etcd` or `postgres-garage`). |
| `SMITH_DOCUMENTS_POSTGRES_DSN` | empty | PostgreSQL DSN for document metadata storage (`postgres-garage` backend). |
| `SMITH_DOCUMENTS_POSTGRES_MAX_CONNS` | `10` | Max PostgreSQL pool connections for document metadata store. |
| `SMITH_DOCUMENTS_GARAGE_ENDPOINT` | empty | Garage S3-compatible endpoint URL for document content blobs. |
| `SMITH_DOCUMENTS_GARAGE_REGION` | `us-east-1` | Region value used by S3 client for Garage requests. |
| `SMITH_DOCUMENTS_GARAGE_BUCKET` | empty | Bucket name used to store document content blobs. |
| `SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID` | empty | Access key for Garage S3 API auth. |
| `SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY` | empty | Secret key for Garage S3 API auth. |
| `SMITH_DOCUMENTS_GARAGE_FORCE_PATH_STYLE` | `true` | Enables path-style S3 requests (recommended for Garage). |
| `SMITH_DOCUMENTS_WATCH_POLL_INTERVAL` | `2s` | Poll interval for document stream watchers in `postgres-garage` mode. |
| `SMITH_DOCUMENTS_MIGRATION_BACKFILL_ON_STARTUP` | `true` | Runs etcd-to-Postgres/Garage backfill at API startup in `postgres-garage` mode. |
| `SMITH_DOCUMENTS_MIGRATION_READ_THROUGH_ON_MISS` | `true` | On read miss in Postgres/Garage, falls back to etcd and backfills migrated copy. |
| `SMITH_DOCUMENTS_MIGRATION_MERGE_LIST_FALLBACK` | `true` | Merges list responses with etcd during migration window. |
| `SMITH_DOCUMENTS_MIGRATION_DUAL_WRITE_ETCD` | `false` | Writes document mutations to etcd as fallback shadow writes. |

Helm note: when `documentDependencies.postgres.enabled` and/or `documentDependencies.garage.enabled` are set, chart templates can auto-populate DSN/endpoint/region/bucket env values from `documentDependencies.*` when corresponding `api.documents.*` values are left empty. Credential values can be sourced from a Kubernetes Secret via `documentDependencies.credentials.*` and injected with `valueFrom.secretKeyRef`.

## smith-core (loop orchestrator)

| Variable | Default | Purpose |
| --- | --- | --- |
| `SMITH_CORE_PORT` | `8083` | Core health/metrics server port. |
| `SMITH_CORE_HOLDER_ID` | hostname-derived | Leader/lock holder identity for state transitions. |
| `SMITH_REPLICA_IMAGE` | `ghcr.io/smith/replica:v0.4.3` | Runtime replica image used for loop Jobs. |
| `SMITH_REPLICA_IMAGE_PULL_POLICY` | `IfNotPresent` | Pull policy for replica image. |
| `SMITH_RUNTIME_SECRET_NAME` | empty | Secret name used by core when wiring runtime credentials into replica jobs. |
| `SMITH_RUNTIME_CREDENTIALS_KEY` | `runtime_credentials` | Secret key field used for runtime credential lookup in replica jobs. |
| `SMITH_RUNTIME_CREDENTIALS` | empty | Legacy inline runtime credential fallback (used only when runtime secret name is unset). |
| `SMITH_GIT_PAT_SECRET_NAME` | empty | Secret name containing git PAT for replica setup. |
| `SMITH_GIT_PAT_SECRET_KEY` | `git_pat` | Secret key field for git PAT lookup. |
| `SMITH_DOCKERFILE_BUILD_ENABLED` | `false` | Enables dockerfile-based execution image build path. |
| `SMITH_LOOP_POLICY_MAX_ATTEMPTS` | `3` | Default max attempts for unresolved/running loops. |
| `SMITH_LOOP_POLICY_BACKOFF_INITIAL` | `5s` | Initial retry backoff. |
| `SMITH_LOOP_POLICY_BACKOFF_MAX` | `2m` | Maximum retry backoff. |
| `SMITH_LOOP_POLICY_TIMEOUT` | `30m` | Loop execution timeout policy. |
| `SMITH_LOOP_POLICY_TERMINATE_ON_ERROR` | `false` | Whether errors force terminal transition. |

## smith-replica (runtime worker)

| Variable | Purpose |
| --- | --- |
| `SMITH_LOOP_ID` | **Required** loop identifier for runtime execution context. |
| `SMITH_CORRELATION_ID` | Correlates loop execution with ingress/task/audit records. |
| `SMITH_WORKSPACE` | Workspace path used for git/bootstrap and agent execution. |
| `SMITH_GIT_REPOSITORY` / `SMITH_GIT_BRANCH` / `SMITH_GIT_PAT` | Git bootstrap and completion credentials/targets. |
| `SMITH_GIT_USER_NAME` / `SMITH_GIT_USER_EMAIL` | Commit identity for completion protocol (defaults: `SMITH` / `smith@cromleylabs.com`). |
| `SMITH_GIT_CREATE_PR` | Enables PR creation during completion when branch changes exist. |
| `SMITH_LOOP_PROVIDER` / `SMITH_LOOP_MODEL` | Selected provider/model invocation parameters. |
| `SMITH_LOOP_INVOCATION_METHOD` | Provider invocation method for loop execution (`sdk`, `cli`, etc.). |
| `SMITH_ISSUE_WORKFLOW_ENABLED` | Enables issue workflow path for issue/PRD ingress execution. |
| `SMITH_ISSUE_PRD_INTERACTIVE` | Enables/disables interactive PRD gate behavior. |
| `SMITH_LOOP_MAX_ITERATIONS` / `SMITH_LOOP_ITERATION_WAIT` | Iteration controls for loop execution cadence. |
| `SMITH_AGENT_CLI_CMD` | Global override for runtime agent command. |

## smith-daemon (retention cleanup controller)

| Variable | Default | Purpose |
| --- | --- | --- |
| `SMITH_DAEMON_PORT` | `8082` | Daemon health/readiness server port. |
| `SMITH_DAEMON_CLEANUP_INTERVAL` | `10m` | Interval between retention cleanup passes. |
| `SMITH_DAEMON_CLEANUP_TIMEOUT` | `30s` | Timeout per cleanup pass. |
| `SMITH_DAEMON_CLEANUP_MAX_DELETES` | `200` | Max loop deletions per pass. |
| `SMITH_DAEMON_CLEANUP_DRY_RUN` | `false` | Logs candidates without deleting records. |
| `SMITH_DAEMON_CLEANUP_ACTOR` | `smith-daemon` | Actor recorded in audit events for retention deletions. |
| `SMITH_DAEMON_POLICY_PATH` | empty | Path to YAML policy file (ConfigMap mount) for retention override. |
| `SMITH_DAEMON_RETENTION_FLATLINE` | `48h` | Retention for flatlined loops (used if policy file is absent). |
| `SMITH_DAEMON_RETENTION_CANCELLED` | `48h` | Retention for cancelled loops (used if policy file is absent). |
| `SMITH_DAEMON_RETENTION_SYNCED` | `0s` | Retention for synced loops (used if policy file is absent). |

## smith-chat service

`smith-chat` currently uses startup flags instead of `SMITH_*` env vars:

- `--port`
- `--etcd-endpoints`
- `--api-url`

These are set by deployment command/manifest and should be treated as the operational equivalents for chat runtime configuration.

## Local make workflow controls

These variables are consumed by `make deploy-local` / `make deploy-local-document-storage` in developer workflows:

| Variable | Default | Purpose |
| --- | --- | --- |
| `SMITH_BOOTSTRAP_DOCUMENT_STORAGE` | `false` | Enables ordered document-storage bootstrap flow inside `deploy-local` (pre-secrets -> deploy -> post-bootstrap). |
| `SMITH_BOOTSTRAP_ENV_FILE` | `~/.smith/.env` | Env file consumed by `scripts/bootstrap-document-storage.sh` when bootstrap mode is enabled. |
| `SMITH_BOOTSTRAP_SCRIPT` | `./scripts/bootstrap-document-storage.sh` | Script path used for document-storage bootstrap orchestration. |
| `SMITH_LOCAL_OVERLAY` | empty | Optional extra Helm values file passed to `deploy-local` (for example `helm/smith/values/local-document-storage-1password.yaml`). |
| `SMITH_FORCE_HELM_ROLLOUT_ID` | `true` | When `true`, forces Helm rollout id mutation on each `deploy-local`; set `false` for idempotent runs. |
| `SMITH_FORCE_ROLLOUT` | `true` | When `true`, forces `rollout-local` restarts after Helm deploy; set `false` for idempotent runs. |
