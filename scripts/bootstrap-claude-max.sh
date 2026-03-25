#!/usr/bin/env bash
# bootstrap-claude-max.sh — store Claude Max OAuth credentials into etcd via smith CLI
#
# On macOS: reads credentials from the system Keychain (where Claude Code stores them).
# On Linux: reads from $SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR (default: ~/.claude).
#
# Usage: ./scripts/bootstrap-claude-max.sh
# Or via make: make bootstrap-claude-max-local
set -euo pipefail

PROVIDER_ID="${SMITH_CLAUDE_MAX_PROVIDER_ID:-claude-max}"
CONFIG_DIR="${SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR:-$HOME/.claude}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SMITH_BIN="${REPO_ROOT}/bin/smith"
if [[ ! -x "$SMITH_BIN" ]]; then
  SMITH_BIN="$(command -v smith 2>/dev/null || true)"
  [[ -n "$SMITH_BIN" ]] || die "smith binary not found — run 'make build-smith' first"
fi

log()  { echo "[bootstrap-claude-max] $*" >&2; }
die()  { echo "[bootstrap-claude-max] ERROR: $*" >&2; exit 1; }
warn() { echo "[bootstrap-claude-max] WARNING: $*" >&2; }

# ---------------------------------------------------------------------------
# macOS keychain lookup
# Claude Code stores OAuth credentials as a JSON blob in the macOS Keychain.
# Try common service names in order.
# ---------------------------------------------------------------------------
KEYCHAIN_SERVICE_NAMES=(
  "Claude Code-credentials"
  "claude.ai"
  "Claude Code"
  "claude-code"
  "anthropic"
  "Anthropic"
  "claude"
)

read_credentials_from_keychain() {
  local service password
  for service in "${KEYCHAIN_SERVICE_NAMES[@]}"; do
    # -s = service, -w = output password only, suppress error output
    if password="$(security find-generic-password -s "$service" -w 2>/dev/null)"; then
      # Verify it looks like a Claude credentials blob
      if echo "$password" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'claudeAiOauth' in d" 2>/dev/null; then
        echo "$password"
        log "found Claude credentials in keychain (service: '$service')"
        return 0
      fi
    fi
    # Also try with -a flag variants (some keytar versions set account to the app name or "oauth")
    for account in "oauth" "claude-code" "Claude Code" "user"; do
      if password="$(security find-generic-password -s "$service" -a "$account" -w 2>/dev/null)"; then
        if echo "$password" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'claudeAiOauth' in d" 2>/dev/null; then
          echo "$password"
          log "found Claude credentials in keychain (service: '$service', account: '$account')"
          return 0
        fi
      fi
    done
  done
  return 1
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
CREDENTIALS_JSON=""
CREDENTIALS_SOURCE=""

if [[ "$(uname)" == "Darwin" ]]; then
  log "macOS detected — searching Keychain for Claude OAuth credentials..."
  if CREDENTIALS_JSON="$(read_credentials_from_keychain)"; then
    CREDENTIALS_SOURCE="keychain"
  else
    warn "credentials not found in Keychain — falling back to file: $CONFIG_DIR/.credentials.json"
    CREDENTIALS_SOURCE="file"
  fi
else
  CREDENTIALS_SOURCE="file"
fi

if [[ "$CREDENTIALS_SOURCE" == "file" ]]; then
  [[ -f "$CONFIG_DIR/.credentials.json" ]] || die "missing $CONFIG_DIR/.credentials.json — run 'claude login' first"
  CREDENTIALS_JSON="$(cat "$CONFIG_DIR/.credentials.json")"
fi

# Show token info (expiry) before asking for confirmation
EXPIRES_AT="$(echo "$CREDENTIALS_JSON" | python3 -c "
import sys, json, datetime
d = json.load(sys.stdin)
print('structure: top-level keys =', list(d.keys()), file=sys.stderr)
oauth = d.get('claudeAiOauth', {})
if not isinstance(oauth, dict):
    print('structure: claudeAiOauth is', type(oauth).__name__, 'not dict', file=sys.stderr)
    sys.exit(1)
print('structure: oauth keys =', list(oauth.keys()), file=sys.stderr)
exp_ms = oauth.get('expiresAt', 0)
exp_dt = datetime.datetime.fromtimestamp(exp_ms / 1000, tz=datetime.timezone.utc)
now = datetime.datetime.now(tz=datetime.timezone.utc)
delta = exp_dt - now
status = 'EXPIRED' if delta.total_seconds() < 0 else f'valid for {int(delta.total_seconds() // 60)} more minutes'
sub = oauth.get('subscriptionType', 'unknown')
print(f'expires: {exp_dt.strftime(\"%Y-%m-%d %H:%M UTC\")} ({status}) | subscription: {sub}')
" 2>&1 || echo "unable to parse token metadata")"

log "credentials from: $CREDENTIALS_SOURCE"
log "token info: $EXPIRES_AT"

# Warn if token is already expired
if echo "$EXPIRES_AT" | grep -q "EXPIRED"; then
  warn "the OAuth token is expired — you may need to run 'claude' locally to refresh it first"
fi

# Confirmation prompt
echo ""
echo "  Provider:  $PROVIDER_ID"
echo "  Source:    $CREDENTIALS_SOURCE"
echo "  Token:     $EXPIRES_AT"
echo ""
read -r -p "[bootstrap-claude-max] Upload these credentials to etcd? [y/N] " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
  log "aborted — no changes made"
  exit 0
fi

# Write credentials to a temp dir so the smith CLI can read the expected files.
# The directory is removed and confirmed gone before the script exits.
CREDS_TMPDIR="$(mktemp -d)"

cleanup() {
  rm -rf "$CREDS_TMPDIR"
  if [[ -e "$CREDS_TMPDIR" ]]; then
    echo "[bootstrap-claude-max] ERROR: failed to remove temporary credential directory: $CREDS_TMPDIR" >&2
    exit 1
  fi
  log "temporary credential directory removed"
}
trap cleanup EXIT

echo "$CREDENTIALS_JSON" > "$CREDS_TMPDIR/.credentials.json"

# Copy .claude.json and settings.json from CONFIG_DIR if they exist; otherwise write minimal stubs
if [[ -f "$CONFIG_DIR/.claude.json" ]]; then
  cp "$CONFIG_DIR/.claude.json" "$CREDS_TMPDIR/.claude.json"
else
  echo '{}' > "$CREDS_TMPDIR/.claude.json"
fi

if [[ -f "$CONFIG_DIR/settings.json" ]]; then
  cp "$CONFIG_DIR/settings.json" "$CREDS_TMPDIR/settings.json"
else
  echo '{}' > "$CREDS_TMPDIR/settings.json"
fi

log "uploading Claude Max credentials for provider '$PROVIDER_ID'..."

"$SMITH_BIN" provider set-claude-credentials \
  --provider-id "$PROVIDER_ID" \
  --config-dir "$CREDS_TMPDIR"

log "done — credentials stored in etcd under provider '$PROVIDER_ID'"
