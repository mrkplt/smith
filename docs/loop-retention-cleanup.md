# Loop Retention and Cleanup

Smith separates runtime artifact cleanup from control-plane record cleanup.

- Runtime artifacts (Kubernetes Jobs/Pods) are deleted by control-plane cleanup paths.
- Loop records (state, journal, audit pointers) are retained by policy and removed after retention expires.

## Components

- `smith-daemon` performs scheduled retention cleanup for terminal loops.
- `smith-api` exposes manual operator cleanup via `POST /v1/loops/cleanup`.

The Console intentionally does not expose bulk delete/clear actions on the Pods page. Cleanup is policy-driven (`smith-daemon`) or explicit operator API action.

## Default retention policy

Helm defaults (see `helm/smith/values.yaml`):

- `flatline`: `48h`
- `cancelled`: `48h`
- `synced`: `0s` (disabled)

Daemon cadence defaults:

- cleanup interval: `10m`
- cleanup timeout: `30s`
- max deletes per pass: `200`

## ConfigMap-driven policy

Retention policy is mounted from ConfigMap `smith-<release>-daemon-policy` as `/etc/smith-daemon/policy.yaml`.

Rendered shape:

```yaml
retention:
  flatline: "48h"
  cancelled: "48h"
  synced: "0s"
```

`smith-daemon` reloads this file before each cleanup pass, so retention changes apply without rebuilding the daemon image.

## Manual cleanup endpoint

`POST /v1/loops/cleanup` supports:

- explicit loop IDs (`loop_ids`),
- state selectors (`states`),
- actor attribution (`actor`).

Safety behavior:

- active loops are skipped,
- unknown IDs are reported as `not_found`,
- deleted/skipped counts are returned in the response,
- audit records include cleanup metadata.

## Operational checks

Verify daemon deployment:

```bash
kubectl get deployment smith-smith-daemon -n smith-system
kubectl logs deployment/smith-smith-daemon -n smith-system --tail=100
```

Inspect effective policy ConfigMap:

```bash
kubectl get configmap smith-smith-daemon-policy -n smith-system -o yaml
```

Update policy with Helm values and rollout:

```bash
helm upgrade --install smith ./helm/smith -n smith-system -f helm/smith/values/local.yaml
kubectl rollout status deployment/smith-smith-daemon -n smith-system
```
