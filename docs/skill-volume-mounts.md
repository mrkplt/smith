# Skill Volume Mounts for Loop Runtime

## Goal

Allow loops to mount skill bundles at runtime so agents can access curated instructions/assets/scripts during execution.

## MVP Scope

- Provider support: Codex and Claude.
- Skills can be mounted either:
  - from loop definition (`loop.skills`), or
  - from runtime policy/default presets.
- Provider default mount path is used unless explicitly overridden.

## Proposed Loop Schema

`loop.skills` array entries:
- `name` (required): logical skill identifier
- `source` (required): source reference (registry/path/artifact ID)
- `mount_path` (optional): provider-specific mount target
- `read_only` (default `true`)
- `version` (optional): pinned skill version/hash

Example:

```json
{
  "skills": [
    {
      "name": "commit",
      "source": "local://skills/commit",
      "version": "v1.2.0",
      "mount_path": "/workspace/.agents/skills/commit",
      "read_only": true
    }
  ]
}
```

Precedence:

- `loop.skills[*].mount_path` (explicit) overrides provider default mountpoint.
- If no `mount_path` is provided, provider default path is used.
- `loop.skills` entries are the authoritative request payload in MVP; runtime policy presets may append/override only in future follow-up work.

## Default Behavior by Provider

- Codex default mountpoint: `/workspace/.agents/skills/<skill-name>`.
- Claude default mountpoint: `/workspace/.claude/skills/<skill-name>`.
- Mount skills as read-only volumes by default.
- Journal resolved skill list and versions in loop metadata/handoff.

## smithctl Skill Flags

`smithctl loop create` supports repeatable `--skill` entries:

```bash
smithctl loop create \
  --title "Skill run" \
  --source-type interactive \
  --source-ref terminal/session-02 \
  --skill name=commit,source=local://skills/commit
```

Supported `--skill` fields:

- `name` (required)
- `source` (required)
- `version` (optional)
- `mount_path` (optional; if omitted, provider defaults are applied server-side)
- `read_only` (optional `true|false`; policy-gated)

## Runtime Integration

- Agent Core resolves requested skills before Replica job creation.
- K8s Job spec includes volume + volumeMount entries for resolved skills.
- Missing or invalid skills fail fast with actionable error state.

### Skills seed container

Smith also supports a workspace seed image that preloads skill bundles into provider-default paths at loop startup:

- Codex path: `/workspace/.agents/skills/`
- Claude path: `/workspace/.claude/skills/`

Repository Dockerfile: `docker/skills.Dockerfile`

Bundled skills currently include:

- `task`
- `tdd` (from `mattpocock/skills/tdd`)

For local deployments, `helm/smith/values/local.yaml` points `core.workspaceSeedImage` at `smith-skills:local`.

## Validation and Security

- Restrict allowed skill sources via allowlist policy.
- Enforce read-only mount by default; writable mounts require explicit policy.
- Prevent path traversal and unsafe mount targets.
- Record mount decisions in audit/journal.

Current validation rules:

- `name` is required and must match `[a-zA-Z0-9._-]+`.
- `source` is required.
- Duplicate skill names (case-insensitive) are rejected.
- `mount_path` must be an absolute, normalized path and cannot be `/` or include `..`.
- Supported providers for `loop.skills`: `codex`, `claude`; other providers reject with a clear error.

## Non-Goals (MVP)

- Multi-agent dynamic mount translation beyond Codex/Claude defaults.
- Arbitrary privileged volume mounts.

Post-MVP multi-provider design: `docs/multi-provider-skill-mount-abstraction.md`.
