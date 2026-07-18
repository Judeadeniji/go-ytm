#!/usr/bin/env bash
# Install ytm from GitHub Releases into ~/.local
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Judeadeniji/go-ytm/main/scripts/install.sh | bash
#   VERSION=v0.1.0 bash scripts/install.sh
set -euo pipefail

REPO="${YTM_REPO:-Judeadeniji/go-ytm}"
PREFIX="${PREFIX:-${HOME}/.local}"
BIN_DIR="${PREFIX}/bin"
SHARE_DIR="${PREFIX}/share/go-ytm"
VERSION="${VERSION:-}"

red() { printf '\033[31m%s\033[0m\n' "$*" >&2; }
info() { printf '==> %s\n' "$*"; }

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		red "error: required command not found: $1"
		exit 1
	fi
}

detect_arch() {
	local os arch
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m)"
	case "$os" in
		linux) ;;
		*)
			red "error: unsupported OS '$os' (Linux only for now)"
			exit 1
			;;
	esac
	case "$arch" in
		x86_64|amd64) arch=amd64 ;;
		aarch64|arm64) arch=arm64 ;;
		*)
			red "error: unsupported architecture '$arch'"
			exit 1
			;;
	esac
	echo "${os}_${arch}"
}

latest_tag() {
	curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
		| head -n1
}

asset_url() {
	local tag="$1" target="$2"
	# Prefer GoReleaser name: ytm_<version>_<os>_<arch>.tar.gz (version without leading v in some configs)
	local ver="${tag#v}"
	echo "https://github.com/${REPO}/releases/download/${tag}/ytm_${ver}_${target}.tar.gz"
}

checksum_url() {
	local tag="$1"
	echo "https://github.com/${REPO}/releases/download/${tag}/checksums.txt"
}

print_dep_hints() {
	echo ""
	echo "System dependencies:"
	echo "  pacman -S mpv python yt-dlp"
	echo "  sudo apt install mpv python3 yt-dlp"
	echo "  sudo dnf install mpv python3 yt-dlp"
	echo ""
}

need_cmd curl
need_cmd tar
need_cmd mktemp

TARGET="$(detect_arch)"
if [[ -z "$VERSION" ]]; then
	info "Resolving latest release for ${REPO}..."
	VERSION="$(latest_tag)"
	if [[ -z "$VERSION" ]]; then
		red "error: could not resolve latest release (have any tags been published?)"
		red "        Build from source: git clone https://github.com/${REPO}.git && cd go-ytm && make install"
		exit 1
	fi
fi

info "Installing ytm ${VERSION} (${TARGET}) into ${PREFIX}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ARCHIVE_URL="$(asset_url "$VERSION" "$TARGET")"
ARCHIVE="$TMP/ytm.tar.gz"
SUMS="$TMP/checksums.txt"

info "Downloading ${ARCHIVE_URL}"
if ! curl -fsSL -o "$ARCHIVE" "$ARCHIVE_URL"; then
	red "error: download failed"
	red "        Check that release ${VERSION} exists for ${TARGET}"
	exit 1
fi

if curl -fsSL -o "$SUMS" "$(checksum_url "$VERSION")" 2>/dev/null; then
	info "Verifying checksum..."
	# checksums.txt lines: "<sha256>  <filename>"
	base="$(basename "$ARCHIVE_URL")"
	expect="$(awk -v f="$base" '$2 == f || $2 == "./"f { print $1; exit }' "$SUMS")"
	if [[ -n "$expect" ]] && command -v sha256sum >/dev/null 2>&1; then
		got="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
		if [[ "$got" != "$expect" ]]; then
			red "error: checksum mismatch for ${base}"
			red "  expected ${expect}"
			red "  got      ${got}"
			exit 1
		fi
		info "Checksum OK"
	fi
else
	info "No checksums.txt found; skipping verification"
fi

info "Extracting..."
mkdir -p "$TMP/extract"
tar -xzf "$ARCHIVE" -C "$TMP/extract"

# Archive may contain a top-level dir or flat files
ROOT="$TMP/extract"
if [[ -d "$TMP/extract/ytm-api" ]] || [[ -f "$TMP/extract/ytm" ]]; then
	ROOT="$TMP/extract"
else
	# single nested directory
	sub="$(find "$TMP/extract" -mindepth 1 -maxdepth 1 -type d | head -n1)"
	if [[ -n "$sub" ]]; then
		ROOT="$sub"
	fi
fi

if [[ ! -f "$ROOT/ytm" ]]; then
	red "error: archive missing ytm binary"
	ls -la "$ROOT" >&2 || true
	exit 1
fi

mkdir -p "$BIN_DIR" "$SHARE_DIR"
install -m 755 "$ROOT/ytm" "$BIN_DIR/ytm"

if [[ -d "$ROOT/ytm-api" ]]; then
	info "Installing ytm-api..."
	rm -rf "$SHARE_DIR/ytm-api"
	mkdir -p "$SHARE_DIR/ytm-api"
	# Copy without venv / caches
	tar -C "$ROOT/ytm-api" \
		--exclude='venv' \
		--exclude='__pycache__' \
		--exclude='*.pyc' \
		--exclude='test_*.py' \
		-cf - . | tar -C "$SHARE_DIR/ytm-api" -xf -
else
	red "warning: archive has no ytm-api/; set YTM_API_HOME after install"
fi

missing=0
if ! command -v mpv >/dev/null 2>&1; then
	red "warning: mpv not found"
	missing=1
fi
if ! command -v python3 >/dev/null 2>&1; then
	red "warning: python3 not found"
	missing=1
fi
if ! command -v yt-dlp >/dev/null 2>&1; then
	info "note: yt-dlp not found (optional fallback for some tracks)"
fi
if [[ "$missing" -eq 1 ]]; then
	print_dep_hints
fi

case ":${PATH}:" in
	*":${BIN_DIR}:"*) ;;
	*)
		echo ""
		echo "Add ${BIN_DIR} to your PATH, e.g.:"
		echo "  echo 'export PATH=\"${BIN_DIR}:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
		;;
esac

echo ""
info "Installed: ${BIN_DIR}/ytm"
info "API data:  ${SHARE_DIR}/ytm-api"
echo ""
echo "Next:"
echo "  ytm doctor    # verify setup"
echo "  ytm           # launch (first run creates the Python venv)"
echo "  Then open Settings to authenticate."
echo ""
