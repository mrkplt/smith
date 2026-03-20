# Smith Deployment Recommendations

## Recommended Production Posture

Smith is optimized for cloud deployment on Kubernetes with autoscaling enabled.

Preferred baseline:
- Managed Kubernetes cluster in cloud environment.
- Cluster autoscaling enabled (node-level scaling).
- Horizontal Pod Autoscaling (HPA) enabled for Smith control-plane services.
- Managed PostgreSQL + S3-compatible object storage (Garage) when running document backend in `postgres-garage` mode.

## Why

- Loop demand is bursty; autoscaling avoids overprovisioning while preserving throughput.
- Smith executes many parallel replicas; node-level elasticity is required for scale.
- Agent Core and supporting APIs can become CPU/memory bound under high anomaly concurrency.
- Separating document metadata/content from etcd reduces control-plane watch pressure and etcd compaction/WAL growth under heavy document editing.

## Guidance

### Cluster Autoscaling
- Enable cluster/node autoscaler for worker node groups.
- Set min/max node boundaries aligned with expected loop concurrency.
- Reserve headroom for system components to avoid scheduler starvation.
- Use `docs/cluster-autoscaler-prerequisites-runbook.md` as the operational runbook for rollout, failure modes, and troubleshooting.

Recommended node group bounds:
- `local`: min 1, max 3 nodes.
- `stage`: min 3, max 12 nodes.
- `prod`: min 6, max 60 nodes.

Node pool split for production:
- Control plane pool (`smith-core`, `smith-api`, `smith-chat`, `smith-console`): min 3, max 15.
- Replica pool (`smith-replica` Jobs): min 3, max 45.

### HPA
- Configure HPA for Agent Core and API/chat services.
- Start with CPU and memory targets; add custom metrics over time (queue depth, active anomalies).
- Define scale-up/down stabilization windows to avoid oscillation.

Recommended production targets:
- `core`: min 3 / max 30, CPU 60%, memory 70%, scale-down stabilization 600s.
- `api`: min 3 / max 40, CPU 55%, memory 65%, scale-down stabilization 300s.
- `console`: min 2 / max 10, CPU 65%, memory 75%, scale-down stabilization 300s.
- `chat`: min 2 / max 10, CPU 65%, memory 75%, scale-down stabilization 300s.

Rollout policy:
- Deploy HPA with conservative max bounds first.
- Observe saturation and queue depth for 3 business days.
- Increase `maxReplicas` in +25% steps when p95 queue delay exceeds SLO.
- Do not change node autoscaler and HPA bounds in the same rollout window.

### Capacity Planning
- Define resource classes for replicas (small/medium/large loops).
- Use namespace quotas and priority classes to protect critical control-plane services.
- Validate behavior under load with scheduled stress tests.

### Reliability
- Pair autoscaling with reconciliation and drift detection.
- Ensure observability dashboards include scaling events, queue depth, and anomaly completion rate.

### Document Storage Dependencies
- If `api.documents.backend=postgres-garage`, choose one posture:
  - Managed/external Postgres + Garage (recommended for staging/prod).
  - In-chart single-node dependencies (`documentDependencies.postgres.enabled=true`, `documentDependencies.garage.enabled=true`) for local/dev.
- Prefer Kubernetes Secret wiring for document credentials (`documentDependencies.credentials.existingSecret` or `documentDependencies.credentials.create=true`) instead of plaintext values.
- Validate dependencies before rollout:
  - Postgres reachable from `smith-api` with write permissions.
  - Garage endpoint reachable with valid bucket credentials.
  - Bucket exists or can be created by API credentials.
- For in-chart Garage, enable bootstrap job (`documentDependencies.garage.bootstrap.enabled=true`) to automate layout/key/bucket bootstrap, or run CLI steps manually if bootstrap is disabled.
- For local/staging operator workflows, `scripts/bootstrap-document-storage.sh` can upsert runtime/document secrets from `~/.smith/.env` and perform idempotent Garage layout/key/bucket bootstrap.
- For migration rollouts, keep read-through/list fallback enabled until backfill verification completes.
- Full credential-flow details and deferred hardening options are captured in `docs/secret-reference-system.md`.

### Secret Encryption at Rest
- Enable Kubernetes API server encryption providers for `Secret` resources.
- Prefer KMS-backed provider in production, with `identity` fallback only as last provider.
- Apply the implementation steps in `docs/kubernetes-secrets-encryption-provider-runbook.md`.

## Non-Cloud / Local

Local and CI environments (for example the default current-context local workflow, `k3d + etcd`, and `k3d + vCluster`) remain valid for development and verification, but are not a substitute for production autoscaling validation.
