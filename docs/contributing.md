# Contributing to Smith

This is the canonical contributor workflow for local development, validation, and documentation updates.

## Core Workflow

Smith uses `td` for task management and `make` as the primary local command surface.

At the start of a new work session:

```bash
td usage --new-session
```

For follow-up reads in the same session:

```bash
td usage -q
```

Use `make help` to discover the local workflow entrypoints.

## Required Tooling

Required local tools include:

- `td`
- `git`
- `mise`
- `docker`
- `kubectl`
- `helm`
- `k3d`
- `vcluster`

The repository `mise.toml` is required for the language/runtime layer and manages:

- `go`
- `node`
- `python`

Install and trust those pinned runtimes from the repository root with:

```bash
mise trust mise.toml
mise install
```

`mise` is the required manager for the runtimes it controls. It does not replace the system-managed tools Smith still requires, including `docker`, `act`, `kubectl`, `helm`, `k3d`, and `vcluster`.

Validate the local environment with:

```bash
make doctor
```

Bootstrap local dependencies with:

```bash
make bootstrap
```

`make bootstrap` now installs the repo-pinned `mise` runtimes before preparing the remaining local prerequisites.

## Git Hooks and Local CI

Install the repository-managed git hooks:

```bash
make hooks-install
```

Hook behavior:

- `pre-commit` runs fast local checks directly (`docs-check`, `go vet`, frontend lint/check, and `golangci-lint`).
- `pre-push` runs fast local unit checks directly (`go test ./...` and frontend `test:unit`).
- heavier validation stays in GitHub Actions, including Playwright, acceptance, matrix, integration, image build, and release gates.

The local full CI entrypoint is:

```bash
make ci-local-act
```

`act` and Docker are optional for local hook use, but still required if you want to run the full GitHub Actions-style CI workload locally with `make ci-local-act`.

If you need to bypass hooks temporarily:

```bash
SKIP_GIT_HOOKS=1 git commit -m "..."
SKIP_GIT_HOOKS=1 git push
```

## Frontend Workflow

Frontend dependencies live under `frontend/`.

Install dependencies before frontend build or check commands:

```bash
mise exec -- npm --prefix frontend install
```

Common frontend validation commands:

```bash
mise exec -- npm --prefix frontend run lint
mise exec -- npm --prefix frontend run build
mise exec -- npm --prefix frontend run check
mise exec -- npm --prefix frontend run test:unit
mise exec -- npm --prefix frontend run test:coverage
```

Go analyzer command:

```bash
mise exec -- go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run ./...
```

For frontend or browser-driven tests, use the repo `make` targets where available so artifact paths and environment setup stay consistent with the rest of the project.

Playwright harness files live under `test/playwright/`:

```bash
mise exec -- npm --prefix test/playwright install
mise exec -- npm --prefix test/playwright run test:frontend
```

## Local Development

For a quick local deploy path:

```bash
make cluster-up
make cluster-health
make build-local
make deploy-local
```

For teardown:

```bash
make undeploy-local
make cluster-down
```

Additional environment details and target contracts are documented in:

- [Local Development Reference](local-dev-make-workflow.md)
- [Local Integration Environment](local-integration-environment.md)
- [Local Make Quickstart](make-local-quickstart.md)

## Documentation Maintenance

When updating contributor workflow or local development behavior:

- update this page first;
- update the README only with a short summary and a link back here;
- keep `docs/index.md` aligned so the docs landing page reflects the current structure.
