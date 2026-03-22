# Local Integration Environment

This environment provides a reproducible local target for Smith integration/e2e tests.

For a copy/paste deployment walkthrough, use `docs/make-local-quickstart.md`.

## Make-First Workflow (Standard)

Use `make` as the primary local workflow entrypoint:

```bash
make help
```

Target matrix:

| Target | Contract | Prerequisites |
| --- | --- | --- |
| `make doctor` | Fails fast when required local tools are missing. | `go`, `kubectl`, `helm`, `docker` in `PATH` |
| `make bootstrap` | Installs required runtimes and optional `k3d`/`vcluster` helpers. | `brew` or `curl` available |
| `make cluster-up` (`make cluster`) | Provisions etcd on the current `kubectl` context. | Doctor checks pass |
| `make cluster-up-k3d` | Provisions local `k3d + etcd`. | `k3d` installed |
| `make cluster-up-vcluster` | Provisions local `k3d + vcluster + etcd`. | `k3d` and `vcluster` installed |
| `make cluster-down` | Removes etcd from the current `kubectl` context. | None (best-effort) |
| `make cluster-down-k3d` | Removes local `k3d + etcd`. | None (best-effort) |
| `make cluster-down-vcluster` | Removes local `k3d + vcluster + etcd`. | None (best-effort) |
| `make cluster-reset` | Rebuilds the default local environment (`cluster-down` then `cluster-up`). | Same as `cluster-up` |
| `make cluster-health` | Verifies cluster API, node readiness, and etcd readiness. | Reachable Kubernetes context |
| `make build-local` | Builds local Smith binaries used by local deploy workflows. | Go toolchain |
| `make deploy-local` | Builds local Smith images, imports them when using k3d, then installs/upgrades the Helm release with the local values profile (`SMITH_LOCAL_VALUES`). | Reachable Kubernetes cluster + Helm + Docker |
| `make daemon-build-local` | Builds only the local `smith-daemon` image. | Docker |
| `make daemon-load-local` | Imports the local daemon image when using k3d. | `k3d` cluster context |
| `make daemon-rollout-local` | Restarts and waits for daemon rollout. | Deployed local Helm release |
| `make daemon-deploy-local` | Build + load + rollout only for `smith-daemon`. | Docker + running local cluster |
| `make deploy-staging` | Installs/upgrades Helm release with staging profile (`SMITH_STAGING_VALUES`). | Reachable Kubernetes cluster + Helm + pre-created runtime secret |
| `make deploy-prod` | Installs/upgrades Helm release with production profile (`SMITH_PROD_VALUES`). | Reachable Kubernetes cluster + Helm + pre-created runtime secret |
| `make undeploy-local` | Removes local Helm release from namespace. | Reachable Kubernetes cluster + Helm |
| `make deploy` | Installs/upgrades Helm release into namespace (`SMITH_NAMESPACE`, `SMITH_RELEASE`, `SMITH_VALUES`). | Reachable Kubernetes cluster + Helm |
| `make test` (`make test-matrix`) | Runs local non-cluster matrix (fixtures, verification, e2e scripts). | Go toolchain + local repo dependencies |
| `make test-integration` | Runs the vCluster-backed integration workflow. | `cluster-up-vcluster` completed |
| `make test-observability-latency` | Measures journal-to-console propagation latency and reports p95/p99. | Running API + active test loop |
| `make teardown` | Removes Helm release and tears down local cluster stack. | None; best-effort cleanup |

Default configurable vars:

- `SMITH_NAMESPACE` (default `smith-system`)
- `SMITH_RELEASE` (default `smith`)
- `SMITH_VALUES` (default `helm/smith/values/local.yaml`)
- `SMITH_LOCAL_GIT_PAT` (required when `secrets.create=true` for local overlay)
- `SMITH_LOCAL_RUNTIME_CREDENTIALS` (required when `secrets.create=true` for local overlay)
- `SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE` (optional Claude runtime credential for `ANTHROPIC_API_KEY` injection)
- `SMITH_VCLUSTER_VERSION` (default `0.32.1`, used by `scripts/integration/prereqs.sh`)
- `SMITH_TEST_ARTIFACTS_DIR` (default `/tmp/smith-test-artifacts`)
- `SMITH_FIXTURE_DIR` (default `/tmp/smith-test-repo`)

## Prerequisites

Required CLI tools:

- `kubectl`
- `helm`

Optional for alternative local providers:

- `k3d`
- `vcluster`

Install missing tools:

```bash
./scripts/integration/prereqs.sh
```

If `env-up` fails with disk-pressure taints, clean local Docker storage and retry:

```bash
docker system prune -af
```

## Bring Up Environment

Before `make deploy-local`, set local credential values in your shell (instead of committing them in values files):

```bash
export SMITH_LOCAL_GIT_PAT="<your-github-pat>"
export SMITH_LOCAL_RUNTIME_CREDENTIALS="<runtime-credential>"
export SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE="<anthropic-api-key>"
```

```bash
make cluster-up
make cluster-health
make build-local
make deploy-local
```

Default mode deploys into the current `kubectl` context, which is a good fit for Docker Desktop Kubernetes.

Alternative providers:

```bash
make cluster-up-k3d
make cluster-up-vcluster
```

`make deploy-local` builds `smith-*:local` images locally. When using `k3d`, those images are imported into the `smith-int` cluster. When using the current cluster provider, image import is skipped and the active cluster must already be able to resolve local images.

Verify daemon health and retention policy after deploy:

```bash
kubectl get deployment smith-smith-daemon -n smith-system
kubectl logs deployment/smith-smith-daemon -n smith-system --tail=100
kubectl get configmap smith-smith-daemon-policy -n smith-system -o yaml
```

Optional chat provider/model overrides for Helm deployment:

```bash
helm upgrade --install smith ./helm/smith \
  -f helm/smith/values/local.yaml \
  --set chat.goose.provider=codex \
  --set chat.goose.model=gpt-5-mini
```

Console-side chat defaults can also be configured at `Settings -> Chat` (provider profile + model) without editing Helm values.

Optional: enable Kubernetes Secret encryption at rest in local `k3d`:

```bash
./scripts/integration/enable-k3d-secrets-encryption.sh
```

Expected deploy-local output includes a Helm success line similar to:

```text
Release "smith" has been upgraded. Happy Helming!
```

Default `cluster-up` creates:

- etcd in namespace `smith-system` on the current cluster context

`cluster-up-k3d` creates:

- k3d host cluster: `smith-int`
- etcd in namespace `smith-system`

`cluster-up-vcluster` creates:

- k3d host cluster: `smith-int`
- vCluster: `smith-vc` in namespace `smith-vcluster`
- etcd in namespace `smith-system`

Default etcd endpoint in-cluster:

`http://smith-etcd.smith-system.svc.cluster.local:2379`

## Tear Down Environment

```bash
make undeploy-local
make cluster-down
```

Expected undeploy-local output includes a Helm uninstall line similar to:

```text
release "smith" uninstalled
```

## Deterministic Test Runs

Use the test matrix harness:

```bash
./scripts/integration/run-tests.sh
```

For non-cluster local validation (default path):

```bash
./scripts/test/run-matrix.sh
```

Run the vCluster watch/reconcile integration target directly:

```bash
./scripts/integration/test-watch-reconcile.sh
```

Run disaster-recovery backup/restore validation drill:

```bash
./scripts/integration/dr-restore-drill.sh
```

Run observability latency benchmark:

```bash
./scripts/integration/measure-observability-latency.sh
```

## Configuration Overrides

Scripts accept environment variable overrides:

- `SMITH_CLUSTER_PROVIDER` (`current` default, `k3d` optional)
- `SMITH_K3D_CLUSTER_NAME`
- `SMITH_K3D_SERVERS`
- `SMITH_K3D_AGENTS`
- `SMITH_K3D_PORT_HTTP`
- `SMITH_K3D_PORT_HTTPS`
- `SMITH_VCLUSTER_NAME`
- `SMITH_VCLUSTER_NAMESPACE`
- `SMITH_ETCD_NAMESPACE`
- `SMITH_ETCD_RELEASE_NAME`
- `SMITH_ETCD_STORAGE_CLASS`
- `SMITH_ETCD_VERSION` (optional chart pin; defaults to latest from repo index)
- `SMITH_ETCD_PERSISTENCE_ENABLED` (default `false` for ephemeral environments)
- `SMITH_ETCD_WAIT_TIMEOUT` (default `8m`)
- `SMITH_ETCD_MODE` (`simple` default, `helm` optional)
- `SMITH_ETCD_IMAGE` (used when `SMITH_ETCD_MODE=simple`)
- `SMITH_VCLUSTER_KUBECONFIG` (path for kubeconfig emitted by `vcluster connect --print`)
