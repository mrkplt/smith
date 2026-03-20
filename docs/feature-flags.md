# Feature Flags Map

> **Notice (Gated / Under Development):** Every feature listed on this page is currently feature-flagged and treated as under development. Default posture is disabled unless explicitly enabled for controlled testing.

This map defines runtime feature gates used to control incomplete or optional surfaces while they are being finished and hardened.

## Current policy

- Feature-flagged surfaces are **off by default** in base Helm values.
- Enabling a feature flag is an operator decision and should be done intentionally per environment.
- Documentation for any feature listed below should be interpreted as preview behavior unless explicitly marked GA.

## Console runtime flags

These values are injected into `window.__SMITH_CONFIG__` from ConfigMap `smith-<release>-console-config`.

| Feature Surface | Runtime Config Key | Env Var | Helm Value | Default |
| --- | --- | --- | --- | --- |
| Tasks route and navigation (`/tasks`) | `featureTasksEnabled` | `SMITH_FEATURE_TASKS_ENABLED` | `console.featureFlags.tasks` | `false` |
| Feature Capability route and navigation (`/feature-capability`) | `featureCapabilityEnabled` | `SMITH_FEATURE_CAPABILITY_ENABLED` | `console.featureFlags.featureCapability` | `false` |
| Operator chat surfaces (`/assistant`, chat drawer, document chat entry points) | `featureChatEnabled` | `SMITH_FEATURE_CHAT_ENABLED` | `console.featureFlags.chat` | `false` |
| Settings Secrets section visibility | `featureSecretsEnabled` | `SMITH_FEATURE_SECRETS_ENABLED` | `console.featureFlags.secrets` | `false` |
| Claude provider visibility in provider selector/editor | `featureProviderClaudeEnabled` | `SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED` | `console.featureFlags.providerClaude` | `false` |
| Gemini provider visibility in provider selector/editor | `featureProviderGeminiEnabled` | `SMITH_FEATURE_PROVIDER_GEMINI_ENABLED` | `console.featureFlags.providerGemini` | `false` |
| Documents PRD diagnostic `Resolve` action | `featurePRDDiagnosticResolveEnabled` | `SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED` | `console.featureFlags.prdDiagnosticResolve` | `false` |
| Feature Capability access override | `featureCapabilityAccess` | `SMITH_FEATURE_CAPABILITY_ACCESS` | `console.featureCapabilityAccess` | `false` |
| Provider "coming soon" visual hints | `showComingSoonProviders` | `SMITH_SHOW_COMING_SOON_PROVIDERS` | `console.showComingSoonProviders` | `false` |

## API feature flags

These flags gate provider types at the API contract level.

| Feature Surface | Env Var | Helm Value | Default |
| --- | --- | --- | --- |
| Claude provider support in provider catalog + validation | `SMITH_PROVIDER_CLAUDE_ENABLED` | `api.featureFlags.providerClaude` | `false` |
| Gemini provider support in provider catalog + validation | `SMITH_PROVIDER_GEMINI_ENABLED` | `api.featureFlags.providerGemini` | `false` |

## Draft PRDs by flag

Each feature-flagged surface has a draft PRD with explicit value statement, rollout goals, and acceptance criteria.

| Flag | Draft PRD |
| --- | --- |
| `SMITH_FEATURE_TASKS_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-tasks-enabled-prd.md`](prds/draft/feature-flag-smith-feature-tasks-enabled-prd.md) |
| `SMITH_FEATURE_CAPABILITY_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-capability-enabled-prd.md`](prds/draft/feature-flag-smith-feature-capability-enabled-prd.md) |
| `SMITH_FEATURE_CAPABILITY_ACCESS` | [`docs/prds/draft/feature-flag-smith-feature-capability-access-prd.md`](prds/draft/feature-flag-smith-feature-capability-access-prd.md) |
| `SMITH_FEATURE_CHAT_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-chat-enabled-prd.md`](prds/draft/feature-flag-smith-feature-chat-enabled-prd.md) |
| `SMITH_FEATURE_SECRETS_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-secrets-enabled-prd.md`](prds/draft/feature-flag-smith-feature-secrets-enabled-prd.md) |
| `SMITH_FEATURE_PROVIDER_CLAUDE_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-provider-claude-enabled-prd.md`](prds/draft/feature-flag-smith-feature-provider-claude-enabled-prd.md) |
| `SMITH_FEATURE_PROVIDER_GEMINI_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-provider-gemini-enabled-prd.md`](prds/draft/feature-flag-smith-feature-provider-gemini-enabled-prd.md) |
| `SMITH_FEATURE_PRD_DIAGNOSTIC_RESOLVE_ENABLED` | [`docs/prds/draft/feature-flag-smith-feature-prd-diagnostic-resolve-enabled-prd.md`](prds/draft/feature-flag-smith-feature-prd-diagnostic-resolve-enabled-prd.md) |
| `SMITH_SHOW_COMING_SOON_PROVIDERS` | [`docs/prds/draft/feature-flag-smith-show-coming-soon-providers-prd.md`](prds/draft/feature-flag-smith-show-coming-soon-providers-prd.md) |
| `SMITH_PROVIDER_CLAUDE_ENABLED` | [`docs/prds/draft/feature-flag-smith-provider-claude-enabled-prd.md`](prds/draft/feature-flag-smith-provider-claude-enabled-prd.md) |
| `SMITH_PROVIDER_GEMINI_ENABLED` | [`docs/prds/draft/feature-flag-smith-provider-gemini-enabled-prd.md`](prds/draft/feature-flag-smith-provider-gemini-enabled-prd.md) |

## Visibility behavior

- `tasks` nav item and `/tasks` route are hidden/redirected when disabled.
- `feature-capability` nav item and route are hidden when disabled.
- Feature Capability additionally requires access (`featureCapabilityAccess=true` or `feature_capability:access` permission) when enabled.
- Chat entry points (TopBar button, `/assistant`, and document chat controls) are hidden when chat is disabled.
- Settings Secrets section is hidden when secrets is disabled.
- Claude/Gemini provider types only appear when both UI and API provider flags are enabled for that provider type.

## Local override examples

Enable chat + secrets in a local test overlay:

```yaml
console:
  featureFlags:
    chat: true
    secrets: true
```

Enable Claude provider in UI and API:

```yaml
api:
  featureFlags:
    providerClaude: true
console:
  featureFlags:
    providerClaude: true
```
