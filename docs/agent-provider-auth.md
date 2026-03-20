# Agent Provider Credentials (API Key + Label)

## Goal

Define the current provider credential model used by Smith across Codex and other providers.

Current direction: provider credentials are API-key based and stored as reusable secrets; provider profiles reference those secrets by label/identifier.

## Current Credential Model

- Provider profiles require `secret_ref` for all provider types (`codex`, `claude`, `gemini`).
- Secret values are stored via Settings Secrets APIs and are never returned in plaintext from list/get responses.
- Model inventory and runtime execution resolve provider credentials from `secret_ref` only.
- Codex connected-auth endpoints are no longer part of the active workflow.

## Operator UX

Provider configuration in Settings is API-key first:

- Enter provider profile information (`id`, `name`, provider type, defaults).
- Enter a credential label/identifier (stored as `secret_ref`).
- Enter API key.

On save, the Console writes/updates the secret value and then saves the provider profile pointing at that `secret_ref`.

For existing profiles:

- Keeping the same credential label allows profile updates without rotating the key.
- Changing credential label requires entering an API key for the new label.
- Entering a new API key rotates the secret value for that credential label.

## API Surface (Current)

Provider profiles and project binding:

- `GET /v1/providers`
- `POST /v1/providers`
- `GET /v1/providers/{id}`
- `PUT /v1/providers/{id}`
- `DELETE /v1/providers/{id}`
- `GET /v1/projects`
- `POST /v1/projects`
- `GET /v1/projects/{id}`
- `PUT /v1/projects/{id}`
- `DELETE /v1/projects/{id}`

Secrets (write-only values, masked reads):

- `GET /v1/secrets`
- `POST /v1/secrets`
- `GET /v1/secrets/{id}`
- `PUT /v1/secrets/{id}`
- `DELETE /v1/secrets/{id}`

## Validation Rules

- Provider profile `secret_ref` is required and must reference an existing secret.
- Unsupported provider types are rejected.
- Deleting a secret is rejected when referenced by any provider profile.
- Deleting a provider profile is rejected when referenced by any project.

## Storage Backends and Environment

- `SMITH_AUTH_STORE_BACKEND` selects storage backend (`file` or `k8s-secret`) for provider/project/credential settings data.
- `SMITH_AUTH_STORE_PATH` configures file-backed credential/settings persistence.
- `SMITH_AUTH_STORE_K8S_NAMESPACE`, `SMITH_AUTH_STORE_K8S_SECRET`, `SMITH_AUTH_STORE_K8S_KEY` configure Kubernetes-backed persistence.

## Audit

- Secret mutations emit `create-secret`, `update-secret`, and `delete-secret` audit actions.
- Project Git credential mutations emit `update-project-credential`, `delete-project-credential`, and `test-project-credential` audit actions.
