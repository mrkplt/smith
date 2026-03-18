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

| Variable | Helm value | Default | Purpose |
| --- | --- | --- | --- |
| `SMITH_API_BASE_URL` | `console.apiBaseUrl` | `/api` | Base path for API calls from the console. |
| `SMITH_CHAT_BASE_URL` | `console.chatBaseUrl` | `/chat` | Base path for chat service calls. |
| `SMITH_OPERATOR_PERMISSIONS` | `console.operatorPermissions` | `""` | Comma-separated permission set used by runtime capability checks. |
| `SMITH_FEATURE_TASKS_ENABLED` | `console.featureFlags.tasks` | `false` | Shows/hides Tasks route + nav and enables route access. |
| `SMITH_FEATURE_CAPABILITY_ENABLED` | `console.featureFlags.featureCapability` | `false` | Shows/hides Feature Capability route + nav and enables route access. |
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
| `SMITH_AUTH_STORE_BACKEND` | `file` | Auth token store backend (`file` or `k8s-secret`). |
| `SMITH_AUTH_STORE_PATH` | `/tmp/smith-auth/tokens.json` | File path for auth token persistence when backend is file. |
| `SMITH_AUTH_STORE_K8S_NAMESPACE` | `POD_NAMESPACE`/`default` | Namespace for auth secret backend. |
| `SMITH_AUTH_STORE_K8S_SECRET` | `smith-auth-store` | Secret name for auth store backend. |
| `SMITH_AUTH_STORE_K8S_KEY` | `tokens.json` | Secret key field used for auth token JSON. |
| `SMITH_SKILL_ALLOWED_SOURCES` | defaults from policy | Restricts allowed skill mount source prefixes. |
| `SMITH_SKILL_ALLOW_WRITABLE` | policy default | Enables/disables writable skill mounts. |

## smith-core (loop orchestrator)

| Variable | Default | Purpose |
| --- | --- | --- |
| `SMITH_CORE_PORT` | `8083` | Core health/metrics server port. |
| `SMITH_CORE_HOLDER_ID` | hostname-derived | Leader/lock holder identity for state transitions. |
| `SMITH_REPLICA_IMAGE` | `ghcr.io/smith/replica:v0.3.1` | Runtime replica image used for loop Jobs. |
| `SMITH_REPLICA_IMAGE_PULL_POLICY` | `IfNotPresent` | Pull policy for replica image. |
| `SMITH_RUNTIME_CREDENTIALS` | empty | Runtime provider credentials passed to loop execution environment. |
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
| `SMITH_GIT_USER_NAME` / `SMITH_GIT_USER_EMAIL` | Commit identity for completion protocol. |
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
