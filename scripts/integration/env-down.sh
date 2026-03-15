#!/usr/bin/env bash
set -euo pipefail

CLUSTER_PROVIDER="${SMITH_CLUSTER_PROVIDER:-current}"
K3D_CLUSTER_NAME="${SMITH_K3D_CLUSTER_NAME:-smith-int}"
VCLUSTER_NAME="${SMITH_VCLUSTER_NAME:-smith-vc}"
VCLUSTER_NAMESPACE="${SMITH_VCLUSTER_NAMESPACE:-smith-vcluster}"
ETCD_NAMESPACE="${SMITH_ETCD_NAMESPACE:-smith-system}"
ETCD_RELEASE_NAME="${SMITH_ETCD_RELEASE_NAME:-smith-etcd}"
USE_VCLUSTER="${SMITH_USE_VCLUSTER:-false}"

info() { echo "[env-down] $*"; }
warn() { echo "[env-down] WARN: $*" >&2; }

run_with_timeout() {
  local seconds="$1"
  shift
  (
    "$@" &
    local pid=$!
    (
      sleep "$seconds"
      kill -TERM "$pid" >/dev/null 2>&1 || true
    ) &
    local watcher=$!
    wait "$pid" >/dev/null 2>&1 || true
    kill -TERM "$watcher" >/dev/null 2>&1 || true
  )
}

has_kube_access() {
  command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1
}

has_k3d_cluster() {
  command -v k3d >/dev/null 2>&1 && k3d cluster list | awk 'NR>1 {print $1}' | grep -q "^${K3D_CLUSTER_NAME}$"
}

if command -v helm >/dev/null 2>&1; then
  if helm status "$ETCD_RELEASE_NAME" -n "$ETCD_NAMESPACE" >/dev/null 2>&1; then
    run_with_timeout 60 helm uninstall "$ETCD_RELEASE_NAME" -n "$ETCD_NAMESPACE"
  else
    info "helm release ${ETCD_RELEASE_NAME} not present in namespace ${ETCD_NAMESPACE}; skipping uninstall"
  fi
fi

if [[ "$USE_VCLUSTER" == "true" ]] && command -v vcluster >/dev/null 2>&1; then
  run_with_timeout 90 vcluster delete "$VCLUSTER_NAME" -n "$VCLUSTER_NAMESPACE" >/dev/null 2>&1 || \
    warn "vcluster ${VCLUSTER_NAME} was not deleted cleanly; continuing teardown"
fi

if has_kube_access; then
  run_with_timeout 45 kubectl delete namespace "$ETCD_NAMESPACE" --ignore-not-found
  if [[ "$USE_VCLUSTER" == "true" ]]; then
    run_with_timeout 45 kubectl delete namespace "$VCLUSTER_NAMESPACE" --ignore-not-found
  fi
elif command -v kubectl >/dev/null 2>&1; then
  info "kubernetes cluster is already unreachable; skipping namespace cleanup"
fi

if [[ "$CLUSTER_PROVIDER" == "k3d" ]] && has_k3d_cluster; then
  info "deleting k3d cluster ${K3D_CLUSTER_NAME}"
  run_with_timeout 90 k3d cluster delete "$K3D_CLUSTER_NAME"
elif [[ "$CLUSTER_PROVIDER" == "k3d" ]] && command -v k3d >/dev/null 2>&1; then
  info "k3d cluster ${K3D_CLUSTER_NAME} not present; skipping delete"
fi

info "environment removed"
