#!/usr/bin/env bash
# Bootstrap for local development: ytm-api (Python) + Go TUI.
# mpv is started by the Go app itself; we only verify it is installed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${1:-$ROOT/bin/ytm-tui}"
API_HOST="127.0.0.1"
API_PORT="${YTM_API_PORT:-8000}"
API_URL="http://${API_HOST}:${API_PORT}"
VENV="$ROOT/ytm-api/venv"
API_LOG="$ROOT/tmp/ytm-api.log"
API_PIDFILE="$ROOT/tmp/ytm-api.pid"
API_PID=""
STARTED_API=0

# Prefer a real system interpreter. Some IDE shells hijack bare `python3`
# into an AppImage; always use an absolute path when possible.
if [[ -x /usr/bin/python3.14 ]]; then
	SYSTEM_PYTHON=/usr/bin/python3.14
elif [[ -x /usr/bin/python3 ]]; then
	SYSTEM_PYTHON=/usr/bin/python3
else
	SYSTEM_PYTHON="$(command -v python3)"
fi

# Run Python tooling in a clean env so IDE shells can't rewrite sys.executable.
run_py() {
	env -i \
		PATH="/usr/bin:/bin:${VENV}/bin" \
		HOME="${HOME:-/tmp}" \
		TERM="${TERM:-xterm}" \
		LANG="${LANG:-C.UTF-8}" \
		VIRTUAL_ENV="$VENV" \
		"$SYSTEM_PYTHON" "$@"
}

cleanup() {
	local code=$?
	if [[ "$STARTED_API" -eq 1 && -n "$API_PID" ]]; then
		echo "==> Stopping ytm-api (pid $API_PID)..."
		kill "$API_PID" 2>/dev/null || true
		wait "$API_PID" 2>/dev/null || true
		rm -f "$API_PIDFILE"
	fi
	exit "$code"
}
trap cleanup EXIT INT TERM

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "error: required command not found: $1" >&2
		exit 1
	fi
}

api_ready() {
	curl -sf "${API_URL}/health" >/dev/null 2>&1
}

# True when venv has a real interpreter (not an AppImage) and uvicorn installed.
venv_ok() {
	local py="$VENV/bin/python3.14"
	[[ -e "$py" || -e "$VENV/bin/python" ]] || return 1
	[[ -x "$VENV/bin/uvicorn" ]] || return 1
	local target
	target="$(readlink -f "$VENV/bin/python" 2>/dev/null || true)"
	[[ "$target" != *AppImage* ]] || return 1
	return 0
}

ensure_venv() {
	if venv_ok; then
		return 0
	fi

	echo "==> Setting up ytm-api virtualenv..."
	rm -rf "$VENV"
	run_py -m venv "$VENV"

	# pip lives inside the new venv; invoke it with a clean env too.
	env -i \
		PATH="$VENV/bin:/usr/bin:/bin" \
		HOME="${HOME:-/tmp}" \
		TERM="${TERM:-xterm}" \
		LANG="${LANG:-C.UTF-8}" \
		VIRTUAL_ENV="$VENV" \
		"$VENV/bin/pip" install -q -r "$ROOT/ytm-api/requirements.txt"

	if ! venv_ok; then
		echo "error: failed to create a usable ytm-api venv" >&2
		exit 1
	fi
}

echo "==> Checking dependencies..."
require_cmd curl
require_cmd mpv

# Ensure user-local and project binaries are visible (yt-dlp lives here often).
export PATH="${HOME}/.local/bin:${ROOT}/bin:${PATH}"

if ! command -v yt-dlp >/dev/null 2>&1; then
	echo "warning: yt-dlp not found — needed when YouTube extraction falls back (age-gated tracks, etc.)" >&2
	echo "         install with: pacman -S yt-dlp   # or: curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o ~/.local/bin/yt-dlp && chmod +x ~/.local/bin/yt-dlp" >&2
fi

if [[ ! -x "$SYSTEM_PYTHON" ]]; then
	echo "error: python3 not found (looked for /usr/bin/python3*)" >&2
	exit 1
fi

if [[ ! -x "$BIN" ]]; then
	echo "error: binary not found: $BIN (run make build first)" >&2
	exit 1
fi

ensure_venv
mkdir -p "$ROOT/tmp"

if api_ready; then
	echo "==> ytm-api already running at ${API_URL}"
else
	if [[ -f "$API_PIDFILE" ]]; then
		old_pid="$(cat "$API_PIDFILE" 2>/dev/null || true)"
		if [[ -n "${old_pid}" ]] && kill -0 "$old_pid" 2>/dev/null; then
			kill "$old_pid" 2>/dev/null || true
			wait "$old_pid" 2>/dev/null || true
		fi
		rm -f "$API_PIDFILE"
	fi

	echo "==> Starting ytm-api on ${API_URL}..."
	(
		cd "$ROOT/ytm-api"
		exec env -i \
			PATH="$VENV/bin:/usr/bin:/bin" \
			HOME="${HOME:-/tmp}" \
			TERM="${TERM:-xterm}" \
			LANG="${LANG:-C.UTF-8}" \
			VIRTUAL_ENV="$VENV" \
			"$VENV/bin/uvicorn" main:app --host "$API_HOST" --port "$API_PORT"
	) >>"$API_LOG" 2>&1 &
	API_PID=$!
	echo "$API_PID" >"$API_PIDFILE"
	STARTED_API=1

	deadline=$((SECONDS + 45))
	until api_ready; do
		if ! kill -0 "$API_PID" 2>/dev/null; then
			echo "error: ytm-api exited early; see $API_LOG" >&2
			tail -n 40 "$API_LOG" >&2 || true
			exit 1
		fi
		if (( SECONDS >= deadline )); then
			echo "error: timed out waiting for ytm-api; see $API_LOG" >&2
			tail -n 40 "$API_LOG" >&2 || true
			exit 1
		fi
		sleep 0.2
	done
	echo "==> ytm-api ready (logs: $API_LOG)"
fi

echo "==> Starting ${BIN##*/} (mpv via Go IPC)..."
"$BIN"
