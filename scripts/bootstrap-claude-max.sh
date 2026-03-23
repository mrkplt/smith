#!/usr/bin/env bash
# bootstrap-claude-max.sh — patch the Smith runtime Secret with Claude Max OAuth credentials
# Reads .credentials.json, .claude.json, and settings.json from SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR
# (default: ~/.claude) and writes them into the cluster Secret as opaque string data.
#
# Usage: ./scripts/bootstrap-claude-max.sh
# Or via make: make bootstrap-claude-max-local
set -euo pipefail

NAMESPACE="${SMITH_NAMESPACE:-smith-system}"
RELEASE="${SMITH_RELEASE:-smith}"
CONFIG_DIR="${SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR:-$HOME/.claude}"

log() { echo "[bootstrap-claude-max] $*"; }
die() { echo "[bootstrap-claude-max] ERROR: $*" >&2; exit 1; }

# Resolve the runtime secret name from the running core deployment, falling back to convention.
resolve_secret_name() {
  local name
  name="$(kubectl -n "$NAMESPACE" get deploy/"${RELEASE}-smith-core" \
    -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="SMITH_RUNTIME_SECRET_NAME")].value}' \
    2>/dev/null || true)"
  if [[ -z "$name" ]]; then
    name="${RELEASE}-smith-runtime"
  fi
  echo "$name"
}

for f in .credentials.json .claude.json settings.json; do
  [[ -f "$CONFIG_DIR/$f" ]] || die "missing $CONFIG_DIR/$f — run 'claude login' first"
done

SECRET_NAME="$(resolve_secret_name)"
log "targeting secret $SECRET_NAME in namespace $NAMESPACE"

# Build a JSON patch using python3 (available on macOS without extra deps).
PATCH="$(python3 - "$CONFIG_DIR" <<'PYEOF'
import json, sys, pathlib

config_dir = pathlib.Path(sys.argv[1])

credentials = (config_dir / ".credentials.json").read_text()
claude_cfg  = (config_dir / ".claude.json").read_text()
settings    = (config_dir / "settings.json").read_text()

patch = {
    "stringData": {
        "claude_max_credentials_json": credentials,
        "claude_max_claude_json":      claude_cfg,
        "claude_max_settings_json":    settings,
    }
}
print(json.dumps(patch))
PYEOF
)"

kubectl -n "$NAMESPACE" patch secret "$SECRET_NAME" --type=merge -p "$PATCH"
log "patched $SECRET_NAME with Claude Max credentials from $CONFIG_DIR"
log "restart smith-core to pick up the new secret: kubectl rollout restart deployment/${RELEASE}-smith-core -n $NAMESPACE"
