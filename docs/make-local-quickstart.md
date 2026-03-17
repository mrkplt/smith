# Local Make Quickstart: Deploy and Run a Loop

This quickstart is copy/paste oriented for a fresh local machine.

## 1. Prerequisites and Bootstrap

```bash
make doctor
make bootstrap
```

If `make doctor` fails, follow the remediation lines printed in output.

## 2. Start Local Cluster and Deploy Smith

Set local runtime credentials in your shell (kept out of tracked values files):

```bash
export SMITH_LOCAL_GIT_PAT="<your-github-pat>"
export SMITH_LOCAL_RUNTIME_CREDENTIALS="<runtime-credential>"
```

```bash
make cluster-up
make cluster-health
make build-local
make deploy-local
```

`make cluster-up` now targets the current `kubectl` context by default, which fits Docker Desktop Kubernetes well. `make deploy-local` builds the Smith images locally and skips k3d import unless you intentionally use `make cluster-up-k3d` or `make cluster-up-vcluster`.

For an isolated disposable cluster instead of Docker Desktop Kubernetes:

```bash
make cluster-up-k3d
```

For the explicit nested vCluster test path:

```bash
make cluster-up-vcluster
```

## 3. Expose API, Chat, and Console Locally

In a separate terminal:

```bash
kubectl -n smith-system port-forward svc/smith-smith-api 8080:8080
kubectl -n smith-system port-forward svc/smith-smith-chat 8081:8081
kubectl -n smith-system port-forward svc/smith-smith-console 3000:3000
```

Keep this running while issuing `smithctl` commands.

Verify control-plane deployments (including retention daemon):

```bash
kubectl get deployments -n smith-system
kubectl get deployment smith-smith-daemon -n smith-system
```

Quick chat sanity check (new terminal):

```bash
curl -sS -X POST http://127.0.0.1:8081/v1/chat/sessions \
  -H 'Content-Type: application/json' \
  -d '{"type":"prd-refinement","context":{}}'
```

Console routing sanity check (`/readyz` and `/chat/v1/chat/...`):

```bash
curl -sS http://127.0.0.1:3000/readyz
curl -sS -X POST http://127.0.0.1:3000/chat/v1/chat/sessions \
  -H 'Content-Type: application/json' \
  -d '{"type":"prd-refinement","context":{}}'
```

## 4. Create and Inspect a Sample Loop

```bash
smithctl --server http://127.0.0.1:8080 --output json loop create \
  --title "Quickstart loop" \
  --description "Validate local make workflow" \
  --source-type interactive \
  --source-ref terminal/quickstart-01
```

Capture the returned `loop_id`, then:

```bash
smithctl --server http://127.0.0.1:8080 --output json loop get <loop_id>
smithctl --server http://127.0.0.1:8080 --output json loop logs <loop_id>
```

## 5. Run Local Validation Suites

```bash
make test
make test-e2e
make cluster-up-vcluster
make test-integration
```

Each target prints an `artifacts:` path for debugging evidence.

## 6. Stop and Clean Up

```bash
make undeploy-local
make cluster-down
```

If you only changed daemon policy/runtime and want a targeted restart:

```bash
make daemon-deploy-local
```

## Troubleshooting

- `kubectl cannot reach a cluster context`:
  - run `make cluster-up`, then `make cluster-health`.
- `missing required command` in doctor:
  - run `make bootstrap` and re-run `make doctor`.
- API calls fail on `127.0.0.1:8080` or chat calls fail on `127.0.0.1:8081`:
  - verify `kubectl port-forward` is active for the matching service.
- e2e/integration failures:
  - inspect the artifact path printed by make targets.
