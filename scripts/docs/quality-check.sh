#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if command -v mise >/dev/null 2>&1; then
  NODE_CMD=(mise exec -- node)
else
  NODE_CMD=(node)
fi

echo "Validating lifecycle documentation metadata"
"${NODE_CMD[@]}" ./scripts/docs/validate-lifecycle-docs.mjs

echo "Running source docs build validation"
./scripts/docs/docs-container.sh "$ROOT_DIR" build >/tmp/smith-docs-build.log

echo "Running public docs build validation"
"${NODE_CMD[@]}" ./scripts/docs/build-public-site.mjs >/tmp/smith-public-docs-build.log

echo "Docs quality checks passed"
