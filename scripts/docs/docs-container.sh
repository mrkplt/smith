#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE_NAME="${SMITH_DOCS_IMAGE:-smith-docs:local}"
WORKSPACE="${1:-$ROOT_DIR}"
shift || true

if ! command -v docker >/dev/null 2>&1; then
  echo "docs container error: docker is required" >&2
  exit 1
fi

docker build -f "$ROOT_DIR/docker/docs.Dockerfile" -t "$IMAGE_NAME" "$ROOT_DIR" >/tmp/smith-docs-image-build.log

uid="$(id -u)"
gid="$(id -g)"

exec docker run --rm \
  --user "${uid}:${gid}" \
  --volume "${WORKSPACE}:/workspace" \
  --workdir /workspace \
  "$IMAGE_NAME" \
  "$@"
