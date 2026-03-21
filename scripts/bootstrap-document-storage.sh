#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Bootstrap Smith document-storage dependencies and secrets.

Usage:
  scripts/bootstrap-document-storage.sh [flags]

Flags:
  --env-file <path>              Path to .env file (default: ~/.smith/.env)
  --namespace <name>             Kubernetes namespace (default: smith-system)
  --release <name>               Helm release name prefix (default: smith)
  --runtime-secret-name <name>   Runtime secret name (default: smith-runtime-external)
  --documents-secret-name <name> Document credentials secret name (default: smith-stage-documents)
  --garage-bucket <name>         Garage bucket to create/allow (default: smith-documents-staging)
  --garage-key-name <name>       Garage key name (default: smith-documents-key)
  --garage-zone <name>           Garage layout zone (default: dc1)
  --garage-capacity <value>      Garage layout capacity (default: 5G)
  --wait-timeout <duration>      Wait timeout for Garage pod readiness (default: 300s)
  --skip-secrets                 Skip Kubernetes Secret creation/update
  --skip-garage                  Skip Garage layout/key/bucket bootstrap
  --help                         Show this help

Expected env vars in env-file:
  SMITH_LOCAL_GIT_PAT
  SMITH_LOCAL_RUNTIME_CREDENTIALS
  SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE (optional)
  SMITH_DOCUMENTS_POSTGRES_PASSWORD
  SMITH_DOCUMENTS_POSTGRES_DSN
  SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID
  SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY
EOF
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

info() {
  printf 'INFO: %s\n' "$*"
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "required command not found: $cmd"
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    die "required env var is missing or empty: $name"
  fi
}

NAMESPACE="${SMITH_NAMESPACE:-smith-system}"
RELEASE="${SMITH_RELEASE:-smith}"
ENV_FILE="${SMITH_ENV_FILE:-$HOME/.smith/.env}"
RUNTIME_SECRET_NAME="${SMITH_RUNTIME_SECRET_NAME:-smith-runtime-external}"
DOCUMENTS_SECRET_NAME="${SMITH_DOCUMENTS_SECRET_NAME:-smith-stage-documents}"
GARAGE_BUCKET="${SMITH_DOCUMENTS_GARAGE_BUCKET:-smith-documents-staging}"
GARAGE_KEY_NAME="${SMITH_GARAGE_KEY_NAME:-smith-documents-key}"
GARAGE_ZONE="${SMITH_GARAGE_ZONE:-dc1}"
GARAGE_CAPACITY="${SMITH_GARAGE_CAPACITY:-5G}"
WAIT_TIMEOUT="${SMITH_GARAGE_WAIT_TIMEOUT:-300s}"
SKIP_SECRETS=false
SKIP_GARAGE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --release)
      RELEASE="$2"
      shift 2
      ;;
    --runtime-secret-name)
      RUNTIME_SECRET_NAME="$2"
      shift 2
      ;;
    --documents-secret-name)
      DOCUMENTS_SECRET_NAME="$2"
      shift 2
      ;;
    --garage-bucket)
      GARAGE_BUCKET="$2"
      shift 2
      ;;
    --garage-key-name)
      GARAGE_KEY_NAME="$2"
      shift 2
      ;;
    --garage-zone)
      GARAGE_ZONE="$2"
      shift 2
      ;;
    --garage-capacity)
      GARAGE_CAPACITY="$2"
      shift 2
      ;;
    --wait-timeout)
      WAIT_TIMEOUT="$2"
      shift 2
      ;;
    --skip-secrets)
      SKIP_SECRETS=true
      shift
      ;;
    --skip-garage)
      SKIP_GARAGE=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

require_cmd kubectl

[[ -f "$ENV_FILE" ]] || die "env file not found: $ENV_FILE"

set -a
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a

require_env SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID
require_env SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY

garage_access_key_id="${SMITH_DOCUMENTS_GARAGE_ACCESS_KEY_ID}"
if [[ "$garage_access_key_id" != GK* ]]; then
  garage_access_key_id="GK${garage_access_key_id}"
fi

if ! $SKIP_SECRETS; then
  require_env SMITH_LOCAL_GIT_PAT
  require_env SMITH_LOCAL_RUNTIME_CREDENTIALS
  require_env SMITH_DOCUMENTS_POSTGRES_PASSWORD
  require_env SMITH_DOCUMENTS_POSTGRES_DSN

  info "Upserting runtime secret: $RUNTIME_SECRET_NAME"
  runtime_secret_args=(
    --from-literal=git_pat="$SMITH_LOCAL_GIT_PAT"
    --from-literal=runtime_credentials="$SMITH_LOCAL_RUNTIME_CREDENTIALS"
  )
  if [[ -n "${SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE:-}" ]]; then
    runtime_secret_args+=(--from-literal=runtime_credentials_claude="$SMITH_LOCAL_RUNTIME_CREDENTIALS_CLAUDE")
  fi
  kubectl -n "$NAMESPACE" create secret generic "$RUNTIME_SECRET_NAME" \
    "${runtime_secret_args[@]}" \
    --dry-run=client -o yaml | kubectl apply -f -

  info "Upserting document credentials secret: $DOCUMENTS_SECRET_NAME"
  kubectl -n "$NAMESPACE" create secret generic "$DOCUMENTS_SECRET_NAME" \
    --from-literal=postgres_password="$SMITH_DOCUMENTS_POSTGRES_PASSWORD" \
    --from-literal=postgres_dsn="$SMITH_DOCUMENTS_POSTGRES_DSN" \
    --from-literal=garage_access_key_id="$garage_access_key_id" \
    --from-literal=garage_secret_access_key="$SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

if $SKIP_GARAGE; then
  info "Skipping Garage bootstrap (--skip-garage)."
  info "Done."
  exit 0
fi

postgres_selector="app.kubernetes.io/component=documents-postgres"
postgres_pod="$(kubectl -n "$NAMESPACE" get pods -l "$postgres_selector" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -n "$postgres_pod" ]]; then
  info "Waiting for Postgres pod readiness: $postgres_pod"
  kubectl -n "$NAMESPACE" wait --for=condition=ready "pod/$postgres_pod" --timeout="$WAIT_TIMEOUT"
else
  info "Postgres pod not found (selector: $postgres_selector); continuing with Garage bootstrap."
fi

garage_selector="app.kubernetes.io/component=documents-garage"
garage_pod="$(kubectl -n "$NAMESPACE" get pods -l "$garage_selector" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -z "$garage_pod" ]]; then
  fallback_garage_pod="${RELEASE}-${RELEASE}-documents-garage-0"
  if kubectl -n "$NAMESPACE" get pod "$fallback_garage_pod" >/dev/null 2>&1; then
    garage_pod="$fallback_garage_pod"
  fi
fi
[[ -n "$garage_pod" ]] || die "no Garage pod found in namespace '$NAMESPACE' with selector '$garage_selector'"

info "Waiting for Garage pod readiness: $garage_pod"
kubectl -n "$NAMESPACE" wait --for=condition=ready "pod/$garage_pod" --timeout="$WAIT_TIMEOUT"

run_garage() {
  kubectl -n "$NAMESPACE" exec "$garage_pod" -- /garage -c /etc/garage/garage.toml "$@"
}

capacity_to_bytes() {
  local value="$1"
  python3 - "$value" <<'PY'
import re
import sys

raw = sys.argv[1].strip().upper().replace(" ", "")
match = re.fullmatch(r"([0-9]+(?:\.[0-9]+)?)([KMGT]?I?B?)", raw)
if not match:
    print("")
    raise SystemExit(0)

num = float(match.group(1))
unit = match.group(2)
if unit in ("", "B"):
    mul = 1
elif unit in ("K", "KB", "KIB"):
    mul = 1024
elif unit in ("M", "MB", "MIB"):
    mul = 1024**2
elif unit in ("G", "GB", "GIB"):
    mul = 1024**3
elif unit in ("T", "TB", "TIB"):
    mul = 1024**4
else:
    print("")
    raise SystemExit(0)

print(int(num * mul))
PY
}

node_id="$(run_garage status | awk '/==== HEALTHY NODES ====/{seen=1;next} seen && $1 ~ /^[0-9a-f]+$/ {print $1; exit}')"
[[ -n "$node_id" ]] || die "failed to discover Garage node id from status output"

layout_show_output="$(run_garage layout show 2>/dev/null || true)"
current_version="$(printf '%s\n' "$layout_show_output" | sed -n 's/^Current cluster layout version:[[:space:]]*//p' | head -n 1)"
node_line="$(printf '%s\n' "$layout_show_output" | awk -v node="$node_id" '$1==node {print; exit}')"
layout_needs_update=true
if [[ -n "$node_line" ]]; then
  current_zone="$(printf '%s\n' "$node_line" | awk '{print $3}')"
  current_capacity="$(printf '%s\n' "$node_line" | awk '{print $4 $5}')"
  requested_bytes="$(capacity_to_bytes "$GARAGE_CAPACITY")"
  current_bytes="$(capacity_to_bytes "$current_capacity")"
  if [[ -n "$requested_bytes" && -n "$current_bytes" && "$current_zone" == "$GARAGE_ZONE" && "$requested_bytes" == "$current_bytes" ]]; then
    layout_needs_update=false
  fi
fi

if $layout_needs_update; then
  info "Applying Garage layout assignment (zone=$GARAGE_ZONE capacity=$GARAGE_CAPACITY node=$node_id)"
  run_garage layout assign -z "$GARAGE_ZONE" -c "$GARAGE_CAPACITY" "$node_id" || true

  layout_applied=false
  start_version=1
  if [[ "$current_version" =~ ^[0-9]+$ ]]; then
    start_version=$((current_version + 1))
  fi
  end_version=$((start_version + 20))
  for version in $(seq "$start_version" "$end_version"); do
    if run_garage layout apply --version "$version" >/dev/null 2>&1; then
      info "Applied Garage layout version $version"
      layout_applied=true
      break
    fi
  done
  if ! $layout_applied; then
    info "Garage layout apply skipped (already applied or no staged changes)."
  fi
else
  info "Garage layout already matches zone/capacity; skipping assign/apply."
fi

if ! run_garage bucket info "$GARAGE_BUCKET" >/dev/null 2>&1; then
  info "Creating Garage bucket: $GARAGE_BUCKET"
  run_garage bucket create "$GARAGE_BUCKET"
else
  info "Garage bucket already exists: $GARAGE_BUCKET"
fi

key_info_tmp="$(mktemp)"
if run_garage key info "$GARAGE_KEY_NAME" >"$key_info_tmp" 2>/dev/null; then
  existing_key_id="$(sed -n 's/^Key ID:[[:space:]]*//p' "$key_info_tmp" | head -n 1 | tr -d '[:space:]')"
  if [[ -n "$existing_key_id" && "$existing_key_id" != "$garage_access_key_id" ]]; then
    rm -f "$key_info_tmp"
    die "Garage key '$GARAGE_KEY_NAME' already exists with key id '$existing_key_id' (expected '$garage_access_key_id')"
  fi
  info "Garage key already exists: $GARAGE_KEY_NAME"
else
  info "Importing Garage key: $GARAGE_KEY_NAME"
  run_garage key import --yes -n "$GARAGE_KEY_NAME" "$garage_access_key_id" "$SMITH_DOCUMENTS_GARAGE_SECRET_ACCESS_KEY"
fi
rm -f "$key_info_tmp"

info "Granting bucket permissions to key"
run_garage bucket allow --read --write --owner "$GARAGE_BUCKET" --key "$GARAGE_KEY_NAME"

run_garage bucket info "$GARAGE_BUCKET" >/dev/null
run_garage key info "$GARAGE_KEY_NAME" >/dev/null

info "Done."
