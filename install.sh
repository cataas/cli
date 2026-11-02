#!/usr/bin/env bash
# Install the cataas CLI: cats in your terminal (https://cataas.com).
#
#   curl -fsSL https://raw.githubusercontent.com/cataas/cli/main/install.sh | bash
#
# Environment variables:
#   CATAAS_INSTALL_DIR  where to install (default: /usr/local/bin if writable, else ~/.local/bin)
#   CATAAS_REF          git branch or tag to install from (default: main)
#   CATAAS_DOWNLOAD_URL override the download location of the binaries (mirror)
set -euo pipefail

REPO="cataas/cli"
REF="${CATAAS_REF:-main}"
BASE_URL="${CATAAS_DOWNLOAD_URL:-https://raw.githubusercontent.com/${REPO}/${REF}/bin}"

info() { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported operating system: $(uname -s) (supported: Linux, macOS)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture: $(uname -m) (supported: amd64, arm64)" ;;
esac

if [ -n "${CATAAS_INSTALL_DIR:-}" ]; then
  dir="$CATAAS_INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  dir=/usr/local/bin
else
  dir="$HOME/.local/bin"
fi
mkdir -p "$dir"

if command -v curl >/dev/null 2>&1; then
  download() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget -qO "$2" "$1"; }
else
  fail "curl or wget is required"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

name="cataas-${os}-${arch}"
info "Downloading ${name} (${REF})"
download "${BASE_URL}/${name}" "$tmp/$name" || fail "download failed: ${BASE_URL}/${name}"
download "${BASE_URL}/SHA256SUMS" "$tmp/SHA256SUMS" || fail "download failed: ${BASE_URL}/SHA256SUMS"

expected="$(awk -v n="$name" '$2 == n {print $1}' "$tmp/SHA256SUMS")"
[ -n "$expected" ] || fail "no checksum for ${name}"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$name" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmp/$name" | awk '{print $1}')"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for ${name}"

chmod +x "$tmp/$name"
mv "$tmp/$name" "$dir/cataas"
info "Installed $("$dir/cataas" version) to $dir/cataas"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) printf '\nAdd %s to your PATH:\n  export PATH="%s:$PATH"\n' "$dir" "$dir" ;;
esac

printf '\nTry it: cataas cute\n'
