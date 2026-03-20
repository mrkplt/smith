# Document Storage Bootstrap Script

Use `scripts/bootstrap-document-storage.sh` to bootstrap local/staging document-storage dependencies when running in-chart Postgres + Garage.

The script handles:

- Runtime secret upsert (`smith-runtime-external` by default).
- Document credential secret upsert (`smith-stage-documents` by default).
- Garage layout assignment + apply.
- Garage key import/verification and bucket permission grant.

The script is idempotent and safe to rerun.

## Prerequisites

- `kubectl` configured to the target cluster.
- Deployed Garage StatefulSet (for Garage bootstrap phase).
- `~/.smith/.env` populated with:
  - `SMITH_LOCAL_GIT_PAT`
  - `SMITH_LOCAL_RUNTIME_CREDENTIALS`
  - `SMITH_DOCUMENTS_POSTGRES_PASSWORD`
  - `SMITH_DOCUMENTS_POSTGRES_DSN`
  - `SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID`
  - `SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY`

If you manage values in 1Password Environments, populate the local file with:

```bash
mkdir -p ~/.smith
op environment read <environment-id> > ~/.smith/.env
chmod 600 ~/.smith/.env
```

## Typical Flow

If you use the Make workflow, `make deploy-local-document-storage` already runs the same sequence (pre-secrets -> deploy -> post-bootstrap).

The make target defaults to idempotent behavior. To force restarts, set:

```bash
SMITH_FORCE_HELM_ROLLOUT_ID=true SMITH_FORCE_ROLLOUT=true make deploy-local-document-storage
```

1. Pre-deploy: create/update secrets only.

```bash
./scripts/bootstrap-document-storage.sh \
  --env-file ~/.smith/.env \
  --namespace smith-system \
  --release smith \
  --skip-garage
```

2. Deploy Helm chart.

```bash
helm upgrade --install smith ./helm/smith \
  -n smith-system \
  -f ./helm/smith/values/local.yaml \
  -f ./helm/smith/values/local-document-storage-1password.yaml \
  --wait --timeout 10m
```

3. Post-deploy: run Garage layout/key/bucket bootstrap.

```bash
./scripts/bootstrap-document-storage.sh \
  --env-file ~/.smith/.env \
  --namespace smith-system \
  --release smith \
  --skip-secrets
```

## Notes

- If `SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID` does not start with `GK`, the script normalizes it automatically.
- If an existing Garage key uses a different key id than expected, the script exits with an explicit error.
- Use `--help` to view all script flags.
