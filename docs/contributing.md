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
- `go`
- `node`
- `docker`
- `act`
- `kubectl`
- `helm`
- `k3d`
- `vcluster`

Validate the local environment with:

```bash
make doctor
```

Bootstrap local dependencies with:

```bash
make bootstrap
```

## Git Hooks and Local CI

Install the repository-managed git hooks:

```bash
make hooks-install
```

Hook behavior:

- `pre-commit` runs the `lint-and-check` job via `act`.
- `pre-push` runs the parallel unit-test jobs via `act` (`go-unit-tests`, `node-unit-tests`, `playwright-tests`).

The local full CI entrypoint is:

```bash
make ci-local-act
```

`act` and Docker are mandatory for the hook workflow because the hooks execute local CI jobs through GitHub Actions-compatible runners.

If you need to bypass hooks temporarily:

```bash
SKIP_GIT_HOOKS=1 git commit -m "..."
SKIP_GIT_HOOKS=1 git push
```

## Frontend Workflow

Frontend dependencies live under `frontend/`.

Install dependencies before frontend build or check commands:

```bash
npm --prefix frontend install
```

Common frontend validation commands:

```bash
npm --prefix frontend run lint
npm --prefix frontend run build
npm --prefix frontend run check
npm --prefix frontend run test:unit
npm --prefix frontend run test:coverage
```

Go analyzer command:

```bash
golangci-lint run ./...
```

For frontend or browser-driven tests, use the repo `make` targets where available so artifact paths and environment setup stay consistent with the rest of the project.

Playwright harness files live under `test/playwright/`:

```bash
npm --prefix test/playwright install
npm --prefix test/playwright run test:frontend
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
