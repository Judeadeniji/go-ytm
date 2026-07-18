#!/usr/bin/env bash
# Stage a clean ytm-api tree for GoReleaser archives (no venv / caches).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/.release-api"
rm -rf "$OUT"
mkdir -p "$OUT"
tar -C "$ROOT/ytm-api" \
	--exclude='venv' \
	--exclude='__pycache__' \
	--exclude='*.pyc' \
	--exclude='test_*.py' \
	-cf - . | tar -C "$OUT" -xf -
echo "==> Staged $OUT"
