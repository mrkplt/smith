# Local Development Reference

This page describes the current make-first local development workflow. For the canonical contributor checklist, git hooks, and frontend commands, start with [Contributing to Smith](contributing.md).

## Principles

- Use `make` as the default command surface.
- Prefer deterministic, repeatable local validation over ad hoc shell sequences.
- Use the repo-managed git hooks and `act`-backed local CI flow.
- Keep quickstart guidance separate from deeper environment reference material.

## Standard Entry Points

### Environment and Tooling

- `make doctor`
  - Validate required local tools and fail fast on missing dependencies.
- `make bootstrap`
  - Install or prepare the local dependencies used by Smith workflows.
- `make hooks-install`
  - Install the repository-managed git hooks.

### Cluster Lifecycle

- `make cluster-up`
  - Start the local `k3d + vcluster + etcd` stack.
- `make cluster-health`
  - Verify the local cluster, namespace, and etcd health.
- `make cluster-down`
  - Tear down the local cluster resources.
- `make cluster-reset`
  - Recreate the cluster stack from scratch.

### Build and Deploy

- `make build-local`
  - Build local artifacts used by the local cluster workflow.
- `make deploy-local`
  - Build, import, and deploy Smith into the local cluster.
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

For local validation:

```bash
make test
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
