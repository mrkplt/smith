# Helm Upgrade, Rollback, and Zero-Downtime Runbook

This runbook covers operational rollout and recovery for Smith Helm releases.

## Scope

- Chart: `helm/smith`
- Environments: `local`, `staging`, `prod`
- Profiles: `helm/smith/values/{local,staging,prod}.yaml`

## Compatibility and Ordering Constraints

1. Use immutable image tags for production rollouts (`v*` or `sha-*`), not branch tags.
2. Keep all control-plane components (`core`, `api`, `chat`, `console`) on the same chart revision.
3. Apply schema-compatible releases only:
   - state/journal/handoff are `v1` records;
   - rolling versions must continue to read existing `v1` data.
4. Rotate secrets before upgrade when changing secret names referenced by values.

## Preflight Checklist

1. Confirm target context and namespace:
   - `kubectl config current-context`
   - `kubectl get ns smith-system`
2. Validate chart and selected profile:
   - `helm lint ./helm/smith -f ./helm/smith/values/<profile>.yaml`
3. Render manifests for review:
   - `helm template smith ./helm/smith -n smith-system -f ./helm/smith/values/<profile>.yaml >/tmp/smith-<profile>.yaml`
4. Verify runtime secret exists for non-local profiles:
   - `kubectl -n smith-system get secret smith-stage-runtime` (staging)
   - `kubectl -n smith-system get secret smith-prod-runtime` (prod)
5. If `api.documents.backend=postgres-garage`, verify dependency connectivity:
    - Postgres DSN resolves from cluster network.
    - Garage endpoint and bucket are reachable with configured credentials.
    - Document credentials are supplied via Kubernetes Secret (`documentDependencies.credentials.*` or pre-created secret).
    - If using a pre-created document secret, ensure it contains keys for `postgres_password`, `postgres_dsn`, `garage_access_key_id`, and `garage_secret_access_key` (or custom key names configured under `documentDependencies.credentials.keys`).
    - `SMITH_DOCUMENTS_MIGRATION_*` flags match migration phase plan.
    - If in-chart dependencies are enabled (`documentDependencies.*.enabled=true`), confirm Garage bootstrap mode:
      - automated hook job enabled (`documentDependencies.garage.bootstrap.enabled=true`), or
      - manual bootstrap procedure planned.
6. Capture release baseline:
   - `helm -n smith-system list`
   - `helm -n smith-system history smith`

## Upgrade Procedure

Use one profile per rollout:

```bash
helm upgrade --install smith ./helm/smith \
  -n smith-system \
  --create-namespace \
  -f ./helm/smith/values/staging.yaml \
  --wait --timeout 10m
```

Post-upgrade checks:

1. `helm -n smith-system status smith`
2. `kubectl -n smith-system get deploy,pod,svc`
3. `kubectl -n smith-system rollout status deploy/smith-smith-core --timeout=5m`
4. `kubectl -n smith-system rollout status deploy/smith-smith-api --timeout=5m`
5. `kubectl -n smith-system rollout status deploy/smith-smith-chat --timeout=5m`
6. `kubectl -n smith-system rollout status deploy/smith-smith-console --timeout=5m`
7. If enabled: `kubectl -n smith-system get sts smith-smith-documents-postgres smith-smith-documents-garage`

### Garage Bootstrap (in-chart dependency mode)

Automated bootstrap script (recommended):

1. Ensure `~/.smith/.env` contains runtime + document credentials.
2. Pre-deploy: upsert required Kubernetes secrets only:
   - `./scripts/bootstrap-document-storage.sh --env-file ~/.smith/.env --namespace smith-system --release smith --skip-garage`
3. Deploy/upgrade Helm release.
4. Post-deploy: run Garage layout/key/bucket bootstrap:
   - `./scripts/bootstrap-document-storage.sh --env-file ~/.smith/.env --namespace smith-system --release smith --skip-secrets`

The script is idempotent and safe to rerun.

Preferred (automated):

1. Set `documentDependencies.garage.bootstrap.enabled=true`.
2. Provide stable Garage API key credentials in the document credentials secret (`documentDependencies.credentials.*`):
   - `garageAccessKeyId`
   - `garageSecretAccessKey`
3. Verify hook job success:
   - `kubectl -n smith-system get job smith-smith-documents-garage-bootstrap`
   - `kubectl -n smith-system logs job/smith-smith-documents-garage-bootstrap`

Manual fallback (if bootstrap hook disabled):

1. `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml status`
2. Extract node id from status output, then assign/apply layout:
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml layout assign -z dc1 -c 1G <node_id>`
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml layout apply --version 1`
3. Import or create key and allow bucket access:
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml key import --yes -n smith-documents-key <key-id> <secret-key>`
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml bucket create <bucket>`
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml bucket allow --read --write --owner <bucket> --key smith-documents-key`

Local shortcut profile:

- Use `helm/smith/values/document-storage-local.yaml` together with `helm/smith/values/local.yaml` to enable `postgres-garage` backend + in-chart dependencies + bootstrap job in one overlay.
- Use `helm/smith/values/staging-document-storage.yaml` together with `helm/smith/values/staging.yaml` for staging in-cluster Postgres + Garage rollout.
- For local make workflow, use `make deploy-local-document-storage` to enforce ordered pre-secrets/deploy/post-bootstrap steps.

### Document Backend Health Checks

Run these checks after upgrade when `api.documents.backend=postgres-garage`:

1. Confirm API is serving readiness:
   - `kubectl -n smith-system run smith-api-readyz --rm -i --restart=Never --image=curlimages/curl:8.12.1 --command -- sh -c 'curl -sf http://smith-smith-api:8080/readyz'`
2. Confirm document backend env wiring on API pod:
   - `kubectl -n smith-system exec deploy/smith-smith-api -- printenv SMITH_DOCUMENT_STORE_BACKEND`
   - `kubectl -n smith-system exec deploy/smith-smith-api -- printenv SMITH_DOCUMENTS_GARAGE_ENDPOINT`
   - `kubectl -n smith-system exec deploy/smith-smith-api -- printenv SMITH_DOCUMENTS_GARAGE_BUCKET`
3. Confirm dependency pods are healthy when in-chart mode is enabled:
   - `kubectl -n smith-system get pod smith-smith-documents-postgres-0`
   - `kubectl -n smith-system get pod smith-smith-documents-garage-0`
4. Confirm Garage bucket/key provisioning:
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml bucket info <bucket>`
   - `kubectl -n smith-system exec -it sts/smith-smith-documents-garage -- /garage -c /etc/garage/garage.toml key info <key-name>`
5. Confirm bootstrap hook result (if enabled):
   - `kubectl -n smith-system get job smith-smith-documents-garage-bootstrap`
   - `kubectl -n smith-system logs job/smith-smith-documents-garage-bootstrap`

## Zero-Downtime Guidance

1. Prefer rolling upgrades with `--wait` and generous timeout.
2. Keep at least two API and chat replicas in staging/prod during rollouts.
3. Avoid simultaneous disruptive changes:
   - do not rotate secrets and chart structure in one deploy;
   - do not change autoscaling bounds and resource limits in the same window.
4. Roll environments in order:
   - `local` -> `staging` -> `prod`.
5. Watch live service health during rollout:
   - `kubectl -n smith-system get pods -w`
   - verify `/readyz` on API and chat services from an internal probe job or port-forward.

## Rollback Procedure

1. Identify target revision:
   - `helm -n smith-system history smith`
2. Roll back:

```bash
helm -n smith-system rollback smith <REVISION> --wait --timeout 10m
```

3. Re-run post-upgrade checks (`status`, `rollout status`, API/chat health).
4. Confirm image tags and values match expected rollback baseline.
5. Record incident details and failed revision in ops notes.

## Known Failure Modes and Recovery

1. `ImagePullBackOff`
   - Cause: invalid tag or missing pull secret.
   - Recovery: fix image tag/secret, then `helm upgrade` again.
2. Pods fail readiness after secret changes
   - Cause: missing key names in runtime secret.
   - Recovery: restore secret keys expected by `secrets.keys.*`; redeploy.
3. HPA thrash during rollout
   - Cause: overly aggressive stabilization/window settings.
   - Recovery: revert autoscaling changes or rollback release revision.
4. API or chat unavailable during rollout
    - Cause: single replica + restart window.
   - Recovery: raise replica count in profile before next upgrade window.

## Non-Prod Validation Record

Validation run date: 2026-03-08

Executed successfully:

1. `helm lint ./helm/smith -f ./helm/smith/values/local.yaml`
2. `helm lint ./helm/smith -f ./helm/smith/values/staging.yaml`
3. `helm lint ./helm/smith -f ./helm/smith/values/prod.yaml`
4. `helm template smith ./helm/smith -n smith-system -f ./helm/smith/values/staging.yaml >/tmp/smith-staging.yaml`
5. `helm template smith ./helm/smith -n smith-system -f ./helm/smith/values/local.yaml -f ./helm/smith/values/document-storage-local.yaml --set secrets.managed.gitPat=dummy --set secrets.managed.runtimeCredentials=dummy >/tmp/smith-local-doc-storage.yaml`

This validates chart/profile integrity in non-prod tooling before cluster rollout.
