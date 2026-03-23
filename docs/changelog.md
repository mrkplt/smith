# Changelog

## Unreleased

### Highlights

- Added provider profile CRUD APIs and project-to-provider-profile binding validation.
- Added settings secrets CRUD APIs with masked responses and reference-integrity checks.
- Expanded Console Settings with side-nav sections and live Providers/Projects/Secrets management.
- Added full-screen Assistant route (`/assistant`) while preserving drawer chat flow.
- Added chat defaults for provider profile selection, optional model/API key overrides, and opaque thinking levels.
- Added `smith-daemon` service for retention-based terminal loop cleanup with ConfigMap-driven policy reload.
- Added manual operator cleanup endpoint `POST /v1/loops/cleanup` with active-loop safety guards and audit metadata.
- Added Feature Capability runtime UI surface (`/feature-capability`) and Tasks/Onboarding routes with shared navigation model (API backend for feature-capability endpoints remains pending in `smith-api`).
- Added pod detail terminal attach/detach controls and state-gated lifecycle actions.
- Added task contract APIs/status model (`/v1/tasks`) and task-to-loop execution synchronization.

### Notes

- Settings secrets are write-only from API/UI perspective: values can be created/rotated but are not returned in plaintext.
- Provider profiles cannot reference missing secrets, and referenced secrets/profiles cannot be deleted until dependencies are removed.
- Pods list defaults to healthy-state filtering; destructive cleanup is policy/API-driven rather than tile-level UI actions.

## v0.5.0

Release date: 2026-03-22

This minor release closes open frontend security advisories by upgrading the Vite toolchain and pinning patched transitive dependencies, while advancing chart/image/docs/frontend defaults to the `v0.5.0` line.

### Highlights

- Remediated `GHSA-67mh-4wv8-2f99` by upgrading `vite` to `6.4.1` (bringing `esbuild` to `0.25.12`).
- Remediated `GHSA-pxg6-pf52-xh8x` by overriding transitive `cookie` to `0.7.2`.
- Patched `CVE-2026-33186` across `smith-api`, `smith-chat`, `smith-core`, and `smith-replica` with `google.golang.org/grpc v1.79.3` in shipped binaries.
- Patched `CVE-2026-32767` in `smith-console` by ensuring upgraded runtime packages (`libexpat` and related Alpine libs) are present in the final image layer.
- Updated Svelte build tooling to compatible secure versions (`@sveltejs/kit@2.55.0`, `@sveltejs/vite-plugin-svelte@6.2.4`).
- Bumped chart/image/frontend/docs defaults to `v0.5.0` for release consistency.

### Notes

- `v0.5.0` is a minor release because it combines frontend and container security hardening with coordinated baseline/toolchain updates across release metadata and deployment defaults.

## v0.4.3

Release date: 2026-03-20

This patch release adds repository hygiene for local generated outputs and advances version defaults to keep release metadata aligned on the `0.4.x` line.

### Highlights

- Added ignore rules for local generated outputs (`dist/` and `cmd/smith/.smith/prompts/*.md`).
- Bumped chart/image/frontend/docs defaults to the `v0.4.3` patch line for release consistency.

### Notes

- `v0.4.3` is a patch release because it focuses on release-hygiene and version-reference alignment without introducing new runtime features.

## v0.4.2

Release date: 2026-03-20

This patch release continues post-`v0.4.0` dependency maintenance by updating frontend transitive dependencies while preserving runtime behavior.

### Highlights

- Updated frontend transitive dependency `flatted` in `/frontend` from `3.4.1` to `3.4.2`.
- Bumped chart/image/frontend/docs defaults to the `v0.4.2` patch line for release consistency.

### Included Changes

- `f49b13a` - `build(deps-dev): bump flatted from 3.4.1 to 3.4.2 in /frontend`
- `5fe7d63` - `Merge pull request #206 from callmeradical/dependabot/npm_and_yarn/frontend/flatted-3.4.2`

### Notes

- `v0.4.2` is a patch release because it contains dependency maintenance and version-reference alignment only.

## v0.4.1

Release date: 2026-03-20

This patch release promotes dependency maintenance updates after `v0.4.0`, keeping runtime behavior stable while pulling in the latest indirect parser fix.

### Highlights

- Updated indirect Go dependency `github.com/buger/jsonparser` from `v1.1.1` to `v1.1.2`.
- Bumped chart/image/frontend/docs defaults to the `v0.4.1` patch line for release consistency.

### Included Changes

- `121be38` - `build(deps): bump github.com/buger/jsonparser from 1.1.1 to 1.1.2`
- `48306b5` - `Merge pull request #205 from callmeradical/dependabot/go_modules/github.com/buger/jsonparser-1.1.2`

### Notes

- `v0.4.1` is a patch release because it contains dependency maintenance and version-reference alignment only.

## v0.3.1

Release date: 2026-03-18

This patch release follows `v0.3.0` by gating unfinished console/runtime surfaces behind feature flags so incomplete experiences stay hidden by default while current operator workflows remain intact.

### Highlights

- Gated unfinished UI routes and related navigation affordances behind runtime-config feature flags.
- Preserved the existing assistant/chat flow while reducing accidental exposure to in-progress surfaces.

### Included Changes

- `6aff1ed` - `feat(console): gate unfinished runtime surfaces with configmap flags`
- `7d444ac` - `Merge pull request #196 from callmeradical/feature-flags-gating`

### Notes

- `v0.3.1` is a patch release because it tightens default visibility/guardrails without introducing new public API contracts.

## v0.2.0

Release date: 2026-03-15

This release advances Smith beyond the initial `v0.1.0` baseline with a broader `smithctl` configuration workflow, stronger local/CI validation, and a simplified docs toolchain that no longer requires a host Python runtime.

### Highlights

- Added `smithctl` configuration command scaffolding plus YAML config inspection and context management commands for listing, selecting, setting, renaming, and deleting contexts.
- Expanded automated coverage around config/runtime resolution and frontend helper behavior.
- Standardized CI on `mise` across the workflow and moved the Zensical docs build into a dedicated container.
- Added and documented a docs lifecycle workflow for planning and PRD trigger validation.
- Tightened build/runtime container setup across console, chat, and replica images.
- Fixed the API image build context so generated Swagger docs are available during container builds.
- Repaired local build/deploy workflow regressions and restored the vCluster pre-release gate configuration.

### Included Changes

- `632572e` - `feat(config): add command scaffolding`
- `673952e` - `feat(config): add YAML config view`
- `fb26961` - `feat(config): add context listing commands`
- `7be5a94` - `feat(config): add use-context command`
- `e9227b1` - `feat(config): add set-context command`
- `6eaf813` - `feat(config): add context rename and delete`
- `d9dc730` - `feat(docs): add docs lifecycle workflow`
- `147cb36` - `ci(workflows): standardize jobs on mise`
- `2a819c3` - `build(docs): containerize zensical tooling`
- `78908ec` - `fix(docker): include generated swagger docs`

### Notes

- `v0.2.0` is a minor release because the range since `v0.1.0` adds new user-facing `smithctl` capabilities rather than only internal fixes.
- The docs build now depends on Docker for the Zensical container path instead of a host-installed Python/Zensical toolchain.
- This tag also includes the follow-on fixes required to keep local image builds and the vCluster pre-release workflow healthy.

## v0.1.0

Release date: 2026-03-14

This is the first tagged Smith release. There was no earlier semver tag in the repository, so `v0.1.0` captures the current `main` baseline at commit `c4c6b2a`.

### Highlights

- Added a reusable Go API client under `pkg/client/v1`, generated gRPC/protobuf definitions under `proto/v1`, Swagger artifacts, and a dedicated `smith-mcp` entrypoint for tool-based integrations.
- Completed end-to-end loop execution plumbing across the API, CLI, and replica paths, with stronger protocol and webhook test coverage.
- Added PRD authoring and readiness validation workflows for JSON and markdown, including canonical markdown import/export support.
- Added a release workflow that builds and publishes `smithctl` binaries for supported platforms.
- Expanded operator documentation, including `smithctl` installation/usage, PRD authoring, loop ingress, release gates, and operational runbooks.

### Included Merges

- PR #169, merged in commit `c4c6b2a`: API client library, Swagger, and MCP server.
- PR #168, merged in commit `d70ff4b`: loop execution completion.
- PR #167, merged in commit `a9a0382`: PRD authoring and readiness validation.
- PR #164, merged in commit `547ef6a`: stable short hash ID generation.
- PR #155, merged in commit `6fafacf`: `smithctl` end-to-end workflow.

### Addressed Tasks Confirmed In Local Metadata

- `td-4617fd` - Decouple API client, add Swagger and MCP server.
- `td-040379` - Pre-release system gate (vCluster + non-vCluster parity).
- `td-6de678` - Interactive terminal attach for active loops.
- `td-c38e55` - Helm environment overlays and profiles.
- `td-e0abfb` - Backup/restore and disaster recovery validation.

### Notes

- The release version starts at `v0.1.0` because the repository had no existing git tags, and local version defaults already reference the `0.1.x` line.
- Task references above are limited to items I could confirm from local `td` metadata or the traceability documentation.
