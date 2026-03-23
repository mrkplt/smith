#!/usr/bin/env bash
# setup.sh — end-to-end local bootstrap for Smith development
# Usage: ./scripts/setup.sh
#
# Environment variables (all optional — script will prompt or use defaults):
#   SMITH_LOCAL_GIT_PAT                  GitHub PAT for replica git operations
#   SMITH_LOCAL_RUNTIME_CREDENTIALS      AI provider credential (defaults to "placeholder" when using Claude Max)
#   SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR    Claude OAuth config dir (default: ~/.claude)
#   SMITH_SKIP_CLUSTER                   Set to "true" to skip cluster bring-up and deploy
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SMITH_MIN_GO_VERSION="${SMITH_MIN_GO_VERSION:-1.22.0}"
SMITH_MIN_KUBECTL_VERSION="${SMITH_MIN_KUBECTL_VERSION:-1.29.0}"
SMITH_MIN_HELM_VERSION="${SMITH_MIN_HELM_VERSION:-3.13.0}"
SMITH_VCLUSTER_VERSION="${SMITH_VCLUSTER_VERSION:-0.32.1}"
SMITH_NAMESPACE="${SMITH_NAMESPACE:-smith-system}"
SMITH_RELEASE="${SMITH_RELEASE:-smith}"
SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR="${SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR:-$HOME/.claude}"
SMITH_SKIP_CLUSTER="${SMITH_SKIP_CLUSTER:-false}"

# ── helpers ──────────────────────────────────────────────────────────────────

log()  { echo "[setup] $*"; }
ok()   { echo "[setup] OK: $*"; }
warn() { echo "[setup] WARN: $*" >&2; }
die()  { echo "[setup] ERROR: $*" >&2; exit 1; }

has() { command -v "$1" >/dev/null 2>&1; }

ver_ge() {
  local current="$1" required="$2"
  [[ "$(printf '%s\n%s\n' "$required" "$current" | sort -V | head -n1)" == "$required" ]]
}

prompt_secret() {
  local var="$1" label="$2"
  if [[ -n "${!var:-}" ]]; then
    return
  fi
  if [[ ! -t 0 ]]; then
    die "$var is required but not set and stdin is not a terminal. Export it before running setup."
  fi
  echo -n "[setup] Enter $label: "
  read -rs value
  echo
  [[ -n "$value" ]] || die "$label cannot be empty"
  export "$var"="$value"
}

# ── OS detection ─────────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)       ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

IS_MAC=false
[[ "$OS" == "darwin" ]] && IS_MAC=true

# ── installers ───────────────────────────────────────────────────────────────

install_brew_or_curl() {
  local name="$1" brew_pkg="$2" curl_cmd="$3"
  if $IS_MAC && has brew && [[ -n "$brew_pkg" ]]; then
    log "installing $name via Homebrew..."
    brew install $brew_pkg
  elif [[ -n "$curl_cmd" ]]; then
    log "installing $name via curl..."
    eval "$curl_cmd"
  else
    die "$name is not installed and no automated installer is available for $OS/$ARCH. Install it manually."
  fi
}

# ── mise ─────────────────────────────────────────────────────────────────────

ensure_mise() {
  if has mise; then
    ok "mise $(mise --version 2>/dev/null | head -n1)"
    return
  fi
  log "mise not found — installing..."
  if $IS_MAC && has brew; then
    brew install mise
  elif has curl; then
    curl -fsSL https://mise.run | sh
    export PATH="$HOME/.local/bin:$PATH"
  else
    die "mise is not installed. Install it from https://mise.jdx.dev/getting-started.html"
  fi
  has mise || die "mise installation succeeded but binary not found in PATH. Open a new shell or add mise to PATH."
  ok "mise installed: $(mise --version 2>/dev/null | head -n1)"
}

# ── container runtime (docker or podman) ─────────────────────────────────────

ensure_container_runtime() {
  local runtime="" version=""

  if has docker && docker info >/dev/null 2>&1; then
    runtime="docker"
    version="$(docker version --format '{{.Client.Version}}' 2>/dev/null)"
  elif has podman && podman info >/dev/null 2>&1; then
    runtime="podman"
    version="$(podman version --format '{{.Client.Version}}' 2>/dev/null)"
  fi

  if [[ -n "$runtime" ]]; then
    ok "$runtime $version"
    return
  fi

  if has docker; then
    warn "docker is installed but the daemon is not running. Start Docker Desktop (or 'dockerd') and re-run this script."
    return
  fi
  if has podman; then
    warn "podman is installed but the machine is not running. Run 'podman machine start' and re-run this script."
    return
  fi

  log "no container runtime found — installing Docker..."
  if $IS_MAC && has brew; then
    brew install --cask docker
    warn "Docker Desktop installed. Launch it from Applications, then re-run this script to verify the daemon is running."
  else
    die "No container runtime found. Install Docker (https://docs.docker.com/engine/install/) or Podman (https://podman.io/docs/installation) then re-run this script."
  fi
}

# ── kubectl ──────────────────────────────────────────────────────────────────

ensure_kubectl() {
  if has kubectl; then
    local v
    v="$(kubectl version --client=true -o yaml 2>/dev/null | awk '/gitVersion:/{print $2; exit}' | sed 's/^v//')"
    if [[ -n "$v" ]] && ver_ge "$v" "$SMITH_MIN_KUBECTL_VERSION"; then
      ok "kubectl $v"
      return
    fi
    warn "kubectl $v is below required $SMITH_MIN_KUBECTL_VERSION — upgrading..."
  else
    log "kubectl not found — installing..."
  fi
  install_brew_or_curl "kubectl" "kubectl" \
    "curl -fsSL \"https://dl.k8s.io/release/\$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/${OS}/${ARCH}/kubectl\" -o /tmp/kubectl && chmod +x /tmp/kubectl && sudo mv /tmp/kubectl /usr/local/bin/kubectl"
  ok "kubectl $(kubectl version --client=true -o yaml 2>/dev/null | awk '/gitVersion:/{print $2; exit}')"
}

# ── helm ─────────────────────────────────────────────────────────────────────

ensure_helm() {
  if has helm; then
    local v
    v="$(helm version --short 2>/dev/null | sed -E 's/^v([0-9]+\.[0-9]+\.[0-9]+).*/\1/')"
    if [[ -n "$v" ]] && ver_ge "$v" "$SMITH_MIN_HELM_VERSION"; then
      ok "helm $v"
      return
    fi
    warn "helm $v is below required $SMITH_MIN_HELM_VERSION — upgrading..."
  else
    log "helm not found — installing..."
  fi
  install_brew_or_curl "helm" "helm" \
    "curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
  ok "helm $(helm version --short 2>/dev/null)"
}

# ── act ──────────────────────────────────────────────────────────────────────

ensure_act() {
  if has act; then
    ok "act $(act --version 2>/dev/null)"
    return
  fi
  log "act not found — installing..."
  install_brew_or_curl "act" "act" \
    "curl -fsSL https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash"
  ok "act $(act --version 2>/dev/null)"
}

# ── mise-managed runtimes (Go + Node) ────────────────────────────────────────

ensure_mise_runtimes() {
  log "trusting mise config..."
  mise trust --quiet "$REPO_ROOT/mise.toml" || die "mise trust failed. Run 'mise trust mise.toml' manually from the repo root."

  log "installing mise-managed runtimes (Go, Node)..."
  mise install --cd "$REPO_ROOT"

  local go_v
  go_v="$(mise exec --no-prepare -- go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')"
  if [[ -z "$go_v" ]]; then
    die "Go is still not available through mise after 'mise install'. Check mise output above."
  fi
  if ! ver_ge "$go_v" "$SMITH_MIN_GO_VERSION"; then
    die "Go $go_v is below required $SMITH_MIN_GO_VERSION. Update the pinned version in mise.toml or repair the mise runtime."
  fi
  ok "go $go_v (via mise)"

  local node_v
  node_v="$(mise exec --no-prepare -- node --version 2>/dev/null | sed 's/^v//')"
  ok "node $node_v (via mise)"
}

# ── k3d + vcluster ───────────────────────────────────────────────────────────

ensure_k3d_vcluster() {
  log "ensuring k3d and vcluster..."
  SMITH_VCLUSTER_VERSION="$SMITH_VCLUSTER_VERSION" bash "$SCRIPT_DIR/integration/prereqs.sh"
}

# ── smith config ─────────────────────────────────────────────────────────────

ensure_smith_config() {
  mkdir -p "$HOME/.smith"
  if [[ ! -f "$HOME/.smith/config.json" ]]; then
    printf '%s\n' '{"current_context":"default","contexts":{"default":{"server":"http://127.0.0.1:8080","token":""}}}' \
      > "$HOME/.smith/config.json"
    ok "created ~/.smith/config.json"
  else
    ok "preserved existing ~/.smith/config.json"
  fi
}

# ── Claude Max OAuth credentials ─────────────────────────────────────────────
#
# Checks SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR (default: ~/.claude) for OAuth
# credential files produced by `claude login`.
#
# If credentials are missing, spins up a minimal container with the Claude CLI
# installed, mounts SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR into it, and drops the
# user into a shell to run `claude login`. Credentials written inside the
# container are persisted via the bind-mount and picked up after exit.
#
# Non-fatal — the Anthropic API key path still works without these files.

CLAUDE_MAX_AVAILABLE=false
CLAUDE_LOGIN_IMAGE="smith-claude-login:local"

claude_creds_exist() {
  local dir="$1"
  [[ -f "$dir/.credentials.json" && -f "$dir/.claude.json" ]]
}

container_runtime() {
  if has docker && docker info >/dev/null 2>&1; then echo "docker"
  elif has podman && podman info >/dev/null 2>&1; then echo "podman"
  else echo ""
  fi
}

extract_keychain_credentials() {
  # macOS only: extract Claude OAuth credentials from the system Keychain and
  # write them to the expected .credentials.json file.
  [[ "$OS" == "darwin" ]] || return 1
  has security || return 1

  local payload
  payload="$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null)" || return 1
  [[ -n "$payload" ]] || return 1

  mkdir -p "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"
  printf '%s' "$payload" > "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR/.credentials.json"
  chmod 600 "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR/.credentials.json"

  # Ensure .claude.json exists (Claude CLI expects it; create a minimal one if absent)
  if [[ ! -f "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR/.claude.json" ]]; then
    echo '{}' > "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR/.claude.json"
  fi
}

ensure_claude_credentials() {
  # 1. Credential files already present — nothing to do.
  if claude_creds_exist "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"; then
    CLAUDE_MAX_AVAILABLE=true
    ok "Claude Max credentials found at $SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"
    return
  fi

  # 2. macOS: extract from system Keychain.
  if $IS_MAC; then
    log "checking macOS Keychain for Claude Max credentials..."
    if extract_keychain_credentials; then
      CLAUDE_MAX_AVAILABLE=true
      ok "Claude Max credentials extracted from Keychain to $SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"
      return
    fi
    # Not in Keychain yet — fall through to container login below.
    log "credentials not found in Keychain — will obtain via container"
  fi

  # 3. No credentials found — spin up a container to run claude login.
  local runtime
  runtime="$(container_runtime)"
  if [[ -z "$runtime" ]]; then
    warn "Claude Max credentials not found and no container runtime is running — skipping Claude Max setup"
    return
  fi

  if [[ ! -t 0 ]]; then
    warn "stdin is not a terminal — cannot run interactive claude login. Run setup manually then re-run."
    return
  fi

  log "Building login container..."
  "$runtime" build -q -t "$CLAUDE_LOGIN_IMAGE" \
    -f "$SCRIPT_DIR/claude-login.Dockerfile" "$SCRIPT_DIR" >/dev/null

  mkdir -p "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"

  log "Launching container. Run 'claude login', then 'exit' when done."
  echo
  "$runtime" run --rm -it \
    -v "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR:/home/claude/.claude" \
    -e CLAUDE_CONFIG_DIR=/home/claude/.claude \
    "$CLAUDE_LOGIN_IMAGE" \
    bash -c '
      rm -f "$HOME/.claude.json"
      [[ -f "$CLAUDE_CONFIG_DIR/.claude.json" ]] || echo "{}" > "$CLAUDE_CONFIG_DIR/.claude.json"
      ln -s "$CLAUDE_CONFIG_DIR/.claude.json" "$HOME/.claude.json"
      exec bash
    '
  echo

  # On macOS, re-attempt Keychain extraction in case login wrote there.
  if $IS_MAC && ! claude_creds_exist "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"; then
    extract_keychain_credentials || true
  fi

  if claude_creds_exist "$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"; then
    CLAUDE_MAX_AVAILABLE=true
    ok "Claude Max credentials obtained at $SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR"
  else
    warn "claude login did not produce credentials — Claude Max mode will not be available"
  fi
}

# ── cluster ──────────────────────────────────────────────────────────────────

ensure_cluster() {
  if kubectl cluster-info >/dev/null 2>&1; then
    ok "cluster reachable: $(kubectl config current-context 2>/dev/null)"
    return
  fi
  log "no reachable cluster — starting k3d..."
  SMITH_CLUSTER_PROVIDER=k3d SMITH_USE_VCLUSTER=false \
    bash "$SCRIPT_DIR/integration/env-up.sh"
  ok "k3d cluster ready"
}

# ── deploy ───────────────────────────────────────────────────────────────────

ensure_deployed() {
  # Prompt for required credentials if not already set.
  prompt_secret SMITH_LOCAL_GIT_PAT "GitHub PAT (repo scope, for replica git operations)"

  # When using Claude Max, SMITH_LOCAL_RUNTIME_CREDENTIALS is not used for
  # Claude loops but the Helm chart still requires a non-empty value.
  if [[ -z "${SMITH_LOCAL_RUNTIME_CREDENTIALS:-}" ]]; then
    if $CLAUDE_MAX_AVAILABLE; then
      export SMITH_LOCAL_RUNTIME_CREDENTIALS="placeholder"
      log "Claude Max mode active — using placeholder for SMITH_LOCAL_RUNTIME_CREDENTIALS"
    else
      prompt_secret SMITH_LOCAL_RUNTIME_CREDENTIALS "runtime credential (Anthropic API key or other provider key)"
    fi
  fi

  log "deploying Smith into cluster..."
  make -C "$REPO_ROOT" --no-print-directory deploy-local \
    SMITH_LOCAL_GIT_PAT="$SMITH_LOCAL_GIT_PAT" \
    SMITH_LOCAL_RUNTIME_CREDENTIALS="$SMITH_LOCAL_RUNTIME_CREDENTIALS" \
    SMITH_FORCE_HELM_ROLLOUT_ID=true \
    SMITH_FORCE_ROLLOUT=true
  ok "Smith deployed"
}

# ── Claude Max credential seeding ─────────────────────────────────────────────

ensure_claude_max_seeded() {
  if ! $CLAUDE_MAX_AVAILABLE; then
    return
  fi
  log "seeding Claude Max credentials into cluster secret..."
  SMITH_NAMESPACE="$SMITH_NAMESPACE" \
  SMITH_RELEASE="$SMITH_RELEASE" \
  SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR="$SMITH_LOCAL_CLAUDE_MAX_CONFIG_DIR" \
    bash "$SCRIPT_DIR/bootstrap-claude-max.sh"
  log "restarting smith-core to pick up Claude Max credentials..."
  kubectl rollout restart "deployment/${SMITH_RELEASE}-smith-core" -n "$SMITH_NAMESPACE"
  kubectl rollout status "deployment/${SMITH_RELEASE}-smith-core" -n "$SMITH_NAMESPACE"
  ok "Claude Max credentials active"
}

# ── main ─────────────────────────────────────────────────────────────────────

main() {
  log "Smith local bootstrap starting..."
  echo

  # Phase 1: local prerequisites
  ensure_mise
  ensure_container_runtime
  ensure_kubectl
  ensure_helm
  ensure_act
  ensure_mise_runtimes
  ensure_k3d_vcluster
  ensure_smith_config
  ensure_claude_credentials

  if [[ "$SMITH_SKIP_CLUSTER" == "true" ]]; then
    echo
    log "SMITH_SKIP_CLUSTER=true — skipping cluster bring-up and deploy"
    log "Run the following when ready:"
    echo "  make cluster-up-k3d"
    echo "  make cluster-health"
    echo "  SMITH_LOCAL_GIT_PAT=<pat> make deploy-local"
    $CLAUDE_MAX_AVAILABLE && echo "  make bootstrap-claude-max-local"
    return
  fi

  echo
  # Phase 2: cluster + deploy + credential seeding
  ensure_cluster
  ensure_deployed
  ensure_claude_max_seeded

  echo
  ok "Smith is running. Access the console at http://localhost:8080"
}

main "$@"
