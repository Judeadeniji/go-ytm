#!/usr/bin/env bash
# Local development launcher. The Go binary starts ytm-api itself when YTM_DEV=1.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/bin/ytm}"

export YTM_DEV=1
export PATH="${HOME}/.local/bin:${ROOT}/bin:${PATH}"

if [[ -z "${YTM_API_HOME:-}" ]]; then
	export YTM_API_HOME="$ROOT/ytm-api"
fi

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: required command not found: $1" >&2
		exit 1
	fi
}

echo "==> Checking dependencies..."
require_cmd mpv
require_cmd python3

if ! command -v yt-dlp >/dev/null 2>&1; then
	echo "warning: yt-dlp not found — optional fallback for some tracks" >&2
fi

if [[ ! -x "$BIN" ]]; then
	echo "error: binary not found: $BIN (run make build first)" >&2
	exit 1
fi

echo "==> Starting ${BIN##*/} (API supervised by Go)..."
exec "$BIN"
