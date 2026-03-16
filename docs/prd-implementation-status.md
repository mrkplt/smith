# PRD Implementation Status

This checklist tracks implementation progress for approved PRDs that are actively being delivered.

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
