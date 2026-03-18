# Feature Flags Map

This map defines user-facing feature gates used to protect runtime UX while features are still maturing.

All console runtime flags are persisted in Helm-managed ConfigMap `smith-<release>-console-config` and injected into the console container via `configMapKeyRef`.

## Console runtime flags

These values are injected into `window.__SMITH_CONFIG__`.

| Feature Surface | Runtime Config Key | Env Var | Helm Value | Default |
| --- | --- | --- | --- | --- |
| Tasks route and navigation (`/tasks`) | `featureTasksEnabled` | `SMITH_FEATURE_TASKS_ENABLED` | `console.featureFlags.tasks` | `false` |
| Feature Capability route and navigation (`/feature-capability`) | `featureCapabilityEnabled` | `SMITH_FEATURE_CAPABILITY_ENABLED` | `console.featureFlags.featureCapability` | `false` |
| Feature Capability permission override | `featureCapabilityAccess` | `SMITH_FEATURE_CAPABILITY_ACCESS` | `console.featureCapabilityAccess` | `false` |
| Operator permission list for gated capabilities | `operatorPermissions` | `SMITH_OPERATOR_PERMISSIONS` | `console.operatorPermissions` | `""` |
| Provider "coming soon" visual hints | `showComingSoonProviders` | `SMITH_SHOW_COMING_SOON_PROVIDERS` | `console.showComingSoonProviders` | `false` |

## Visibility behavior

- `tasks` nav item and `/tasks` route are hidden/redirected when disabled.
- `feature-capability` nav item and route are hidden when the feature flag is disabled.
- Feature Capability additionally requires access (`featureCapabilityAccess=true` or `feature_capability:access` permission) when enabled.

## Local override examples

Enable Tasks only:

```yaml
console:
  featureFlags:
    tasks: true
    featureCapability: false
```

Enable Feature Capability for an operator:

```yaml
console:
  featureFlags:
    featureCapability: true
  featureCapabilityAccess: true
```
