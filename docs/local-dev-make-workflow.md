# Local Development Reference

This page describes the current make-first local development workflow. For the canonical contributor checklist, git hooks, and frontend commands, start with [Contributing to Smith](contributing.md).

## Principles

- Use `make` as the default command surface.
- Prefer deterministic, repeatable local validation over ad hoc shell sequences.
- Use the repo-managed git hooks and `act`-backed local CI flow.
- Keep quickstart guidance separate from deeper environment reference material.

## Standard Entry Points

### One-Shot Bootstrap (Recommended for New Contributors)

- `./scripts/setup.sh`
  - End-to-end local bootstrap: installs all prerequisites, brings up a k3d cluster, deploys Smith, and activates Claude Max OAuth if credentials are available. See [make-local-quickstart.md](make-local-quickstart.md) for a full description of each phase.
  - Set `SMITH_SKIP_CLUSTER=true` to run only the prerequisite checks without touching the cluster.

### Environment and Tooling

- `make doctor`
  - Validate required local tools and fail fast on missing dependencies.
- `make bootstrap`
  - Install or prepare the local dependencies used by Smith workflows.
- `make hooks-install`
  - Install the repository-managed git hooks.

### Cluster Lifecycle

- `make cluster-up`
  - Start the default local environment on the current `kubectl` context.
- `make cluster-up-k3d`
  - Start a disposable `k3d + etcd` environment.
- `make cluster-up-vcluster`
  - Start the explicit `k3d + vcluster + etcd` compatibility environment.
- `make cluster-health`
  - Verify the current cluster, namespace, and etcd health.
- `make cluster-down`
  - Tear down the default local environment resources.
- `make cluster-reset`
  - Recreate the cluster stack from scratch.

### Build and Deploy

- `make build-local`
  - Build local artifacts used by the local cluster workflow.
- `make deploy-local`
  - Build and deploy Smith into the active cluster, importing images only for `k3d`.
- `make deploy-local-document-storage`
  - Ordered local deploy path for in-cluster Postgres + Garage: pre-deploy secret sync, Helm deploy, then post-deploy Garage bootstrap.
- `make daemon-deploy-local`
  - Build/load/restart only `smith-daemon` for retention-policy iteration without full stack redeploy.
- `make undeploy-local`
  - Remove the local deployment from the cluster.

### Validation

- `make test`
  - Run the default local non-cluster test matrix.
- `make test-integration`
  - Run the integration suite against the local environment.
- `make test-e2e`
  - Run end-to-end scenarios.
- `make test-frontend`
  - Run the frontend/browser test workflow.
- `make ci-local-act`
  - Run the full local CI flow through `act`.

## Recommended Local Flow

For a fresh local environment:

```bash
make doctor
make bootstrap
make hooks-install
```

For local deployment work:

```bash
make cluster-up
make cluster-health
make build-local
make deploy-local
```

For local document-storage deployment work (`postgres-garage` overlay + 1Password-backed env file):

```bash
set -a
source ~/.smith/.env
set +a
make deploy-local-document-storage
```

`deploy-local-document-storage` defaults to idempotent behavior (no forced rollout-id mutation and no forced rollout restart). Override only when you intentionally want restarts:

```bash
SMITH_FORCE_HELM_ROLLOUT_ID=true SMITH_FORCE_ROLLOUT=true make deploy-local-document-storage
```

For local validation:

```bash
make test
make cluster-up-vcluster
make test-integration
make test-e2e
make ci-local-act
```

For frontend-only work:

```bash
npm --prefix frontend install
npm --prefix frontend run build
npm --prefix frontend run check
```

## Related Docs

- [Contributing to Smith](contributing.md)
- [Local Make Quickstart](make-local-quickstart.md)
- [Local Integration Environment](local-integration-environment.md)
