# Settings API and Console Configuration

## Goal

Document the API contracts used by the Console Settings surface (`/settings`) for provider profiles, projects, and reusable secrets.

## Authentication

Settings endpoints follow the same operator auth policy as the rest of `smith-api`:

- If `SMITH_OPERATOR_TOKEN` is set, send `Authorization: Bearer <token>`.
- If `SMITH_OPERATOR_TOKEN` is empty, local/dev access is open.

## Provider Profiles

Provider profiles are reusable model/auth/runtime definitions referenced by projects.

### Endpoints

- `GET /v1/providers`
- `GET /v1/providers/catalog`
- `POST /v1/providers`
- `GET /v1/providers/{id}`
- `PUT /v1/providers/{id}`
- `DELETE /v1/providers/{id}`

Provider catalog response fields:

- `id`
- `display_name`
- `default_model`
- `required_config_fields` (string array)

### Contract

Provider profile payload fields:

- `id` (required)
- `name`
- `provider_type` (`codex`, `claude`, `gemini`)
- `endpoint`
- `default_model`
- `capabilities` (string array)
- `secret_ref` (optional; must reference an existing secret)
- `options` (string map)
- `updated_at`

### Validation and lifecycle rules

- Missing `id` is rejected.
- If `name` is omitted, it defaults to `id`.
- If `provider_type` is omitted, it defaults to `codex`.
- Unsupported provider types are rejected; legacy aliases normalize as: `openai -> codex`, `anthropic -> claude`, `google -> gemini`.
- If `default_model`/`capabilities` are omitted, provider-specific defaults are applied.
- `DELETE /v1/providers/{id}` returns conflict when any project references the profile.
- `codex-default` is protected and cannot be deleted.

## Projects

Projects represent repository/runtime context and now bind to provider profiles.

### Endpoints

- `GET /v1/projects`
- `POST /v1/projects`
- `GET /v1/projects/{id}`
- `PUT /v1/projects/{id}`
- `DELETE /v1/projects/{id}`

### Contract

Common fields:

- `id` (required on create)
- `name`
- `repo_url`
- `provider_profile_id`
- `github_user`
- `runtime_image`
- `runtime_pull_policy`
- `skills_image`
- `skills_pull_policy`
- `updated_at`

### Validation/default behavior

- `provider_profile_id` defaults to `codex-default` when omitted.
- Project create/update rejects unknown `provider_profile_id`.
- `runtime_pull_policy` and `skills_pull_policy` default to `IfNotPresent`.

## Project Git Credentials

Project-level Git credentials are managed separately from project metadata.

### Endpoints

- `GET /v1/projects/credentials/github?project_id=<id>`
- `POST /v1/projects/credentials/github`
- `DELETE /v1/projects/credentials/github?project_id=<id>`
- `POST /v1/projects/credentials/github/test`

### Connection test behavior

- `POST /v1/projects/credentials/github/test` validates repository access for the stored project PAT.
- Validation currently targets `github.com` repositories.
- Responses are actionable (`valid=false` with a message for missing credential, invalid/expired token, forbidden access, missing repo, or API connectivity failures).
- Credential test calls emit audit action `test-project-credential`.

## Settings Secrets

Settings secrets are reusable references for provider profiles and integrations.

### Endpoints

- `GET /v1/secrets`
- `POST /v1/secrets`
- `GET /v1/secrets/{id}`
- `PUT /v1/secrets/{id}`
- `DELETE /v1/secrets/{id}`

### Contract

Write payload fields:

- `id` (required)
- `name`
- `description`
- `value` (required on create)

Read response fields:

- `id`
- `name`
- `description`
- `updated_at`
- `has_value` (boolean)
- `value_masked` (when value exists)

### Secret value handling

- Secret values are write-only from API/UI perspective.
- Plaintext values are not returned by `GET` list/get endpoints.
- `PUT /v1/secrets/{id}` with empty `value` keeps the existing stored value.
- `DELETE /v1/secrets/{id}` returns conflict when referenced by any provider profile.

### Audit events

- Secret mutations emit audit actions: `create-secret`, `update-secret`, `delete-secret`.
- Project Git credential mutations emit audit actions: `update-project-credential`, `delete-project-credential`.

## Console Settings UI

The web console centralizes configuration at `/settings` with section navigation:

- `General`
- `Providers`
- `Projects`
- `Chat`
- `Integrations`
- `Secrets`

Compatibility redirects:

- `/providers` redirects to `/settings?section=providers`
- `/projects` redirects to `/settings?section=projects`

Chat UX notes:

- Drawer chat remains default.
- Full-screen assistant route is `/assistant`.
- Chat defaults include provider profile selection, optional model override, optional API key override, and thinking level (`quick`, `balanced`, `deep`).

Provider credential UX notes:

- Provider profile editor includes a credential status panel for Codex with masked key/account metadata.
- Operators can rotate credentials by providing a new API key and saving the provider profile.
- Operators can revoke Codex credentials directly from the provider editor.

## Backing stores

Settings APIs use the auth store backend selector:

- `SMITH_AUTH_STORE_BACKEND=file` uses in-process file/memory-backed behavior.
- `SMITH_AUTH_STORE_BACKEND=kubernetes` persists settings objects in-cluster:
  - provider profiles in ConfigMap `smith-provider-profiles`
  - projects in ConfigMap `smith-projects`
  - settings secrets in Secret `smith-settings-secrets`

Namespace is controlled by `SMITH_AUTH_STORE_K8S_NAMESPACE`.

## API examples

Examples assume `smith-api` is reachable at `http://localhost:8080`.

Create a secret:

```bash
curl -sS -X POST http://localhost:8080/v1/secrets \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "openai-key",
    "name": "OpenAI API Key",
    "description": "Primary model credential",
    "value": "sk-..."
  }'
```

Create a provider profile that references the secret:

```bash
curl -sS -X POST http://localhost:8080/v1/providers \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "openai-work",
    "name": "OpenAI Work",
    "provider_type": "openai",
    "default_model": "gpt-5.4",
    "secret_ref": "openai-key"
  }'
```

Create a project bound to that provider profile:

```bash
curl -sS -X POST http://localhost:8080/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "acme-api",
    "name": "Acme API",
    "repo_url": "https://github.com/acme/api",
    "provider_profile_id": "openai-work"
  }'
```

Rotate secret value while keeping the same secret ID:

```bash
curl -sS -X PUT http://localhost:8080/v1/secrets/openai-key \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "openai-key",
    "name": "OpenAI API Key",
    "value": "sk-new-..."
  }'
```

List secrets (masked values only):

```bash
curl -sS http://localhost:8080/v1/secrets
```
