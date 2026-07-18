#!/usr/bin/env bash
# Stage a clean ytm-api tree for GoReleaser archives (no venv / caches).
# Output layout: .release/ytm-api/{main.py,deps.py,routers/,...}
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/.release/ytm-api"
rm -rf "$ROOT/.release"
mkdir -p "$OUT"
tar -C "$ROOT/ytm-api" \
	--exclude='venv' \
	--exclude='__pycache__' \
	--exclude='*.pyc' \
	--exclude='test_*.py' \
	-cf - . | tar -C "$OUT" -xf -
echo "==> Staged $OUT"
find "$OUT" -type f | sort | head -30
