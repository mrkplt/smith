# Changelog

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
