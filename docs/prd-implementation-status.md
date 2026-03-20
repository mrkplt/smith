# PRD Implementation Status

This checklist tracks implementation progress for approved PRDs that are actively being delivered.

> **Notice (Gated / Under Development):** Several surfaces listed here (for example `/tasks`, `/feature-capability`, `/assistant`, and Settings Secrets) are behind feature flags and may be disabled by default.

## Smith Settings UI Architecture PRD

Source: `docs/prds/approved/smith_settings_ui_architecture_prd.md`

### Completed

- [x] Top nav focused on runtime surfaces with Settings as configuration hub.
- [x] Settings sidebar sections added: General, Providers, Projects, Chat, Integrations, Secrets.
- [x] Providers moved under Settings and implemented as Provider Profiles.
- [x] Projects moved under Settings and reference `provider_profile_id`.
- [x] Providers/Projects continue using card + drawer interaction model.
- [x] Secrets section now has real CRUD and provider `secret_ref` validation.
- [x] Full-screen assistant route (`/assistant`) and drawer chat coexist.
- [x] Chat Settings are profile-driven (provider profile + optional model/API key override + thinking level), used by drawer, full-screen, and PRD chat modals.

### Remaining

- [ ] Integrations section: GitHub App status, authorization, and repository sync actions.
- [ ] General section: fully live installation metadata (installation ID, cluster context, upgrade channel, health).

## Smith GitHub Authentication PRD

Source: `docs/prds/approved/smith_github_auth_prd.md`

### Completed / Present Foundation

- [x] Settings architecture has dedicated Integrations and Projects surfaces to host GitHub App workflows.
- [x] Project model supports migration away from PAT-centric provider configuration via profile references.

### Remaining

- [ ] GitHub OAuth login flow in Smith UI.
- [ ] GitHub App installation flow and installation ID capture.
- [ ] Authorized repository discovery and project onboarding from installation repos.
- [ ] Installation-token-on-demand workflow integrated for GitHub operations.
- [ ] PAT fallback clearly marked as transitional/deprecated path.

## Smith MVP Execution Flow PRD

Source: `docs/prds/approved/smith_mvp_execution_flow_prd.md`

### Completed

- [x] Replica git workspace bootstrap and completion hardening for PR creation flows.
- [x] PRD-ingress execution path hardening (non-interactive PRD gate for autonomous PRD loops).
- [x] Runtime compatibility updates for replica execution base image and auth priming.
- [x] Loop display metadata surfaced in API (`display_title`, `current_count`, `target_count`) and consumed by Console pods view.
- [x] Terminal attach/detach and command controls integrated on pod detail.
- [x] Loop ID normalization deduplicates repeated adjacent segments.
- [x] Retention cleanup daemon (`smith-daemon`) introduced with ConfigMap-driven policy.
- [x] Task contract lifecycle APIs and `/tasks` runtime surface integrated with loop creation/status sync.
- [x] Onboarding readiness route (`/onboarding`) and root-runtime gating behavior integrated in Console shell.

### Remaining

- [ ] Expand runtime artifact retention/cleanup observability (metrics dashboards and alert thresholds).
