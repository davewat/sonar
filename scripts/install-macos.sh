#!/bin/sh
# Installer for sonar on macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/davewat/sonar/main/scripts/install-macos.sh | sh
#
# Ensures a Go toolchain is present (via Homebrew), builds the sonar binary
# from source, installs it to $HOME/.local/bin (or $SONAR_INSTALL_DIR) —
# writable without sudo — and removes all build artifacts it created.
# Leaves only the installed binary behind.
#
# Note: `sudo` cannot prompt for a password when this script is read from a
# pipe (its stdin is the piped script, not a terminal). If you don't have
# passwordless sudo and installation fails for that reason, download and
# run it as a normal script instead:
#   curl -fsSL https://raw.githubusercontent.com/davewat/sonar/main/scripts/install-macos.sh -o install-macos.sh
#   sh install-macos.sh
#
# This script deliberately does not attempt to install Homebrew itself:
# Homebrew's installer can trigger an interactive Xcode Command Line Tools
# prompt with no scriptable bypass. Install Homebrew first (https://brew.sh)
# if you don't already have it.

set -eu

REPO_URL="https://github.com/davewat/sonar"
REPO_REF="${SONAR_REF:-main}"
INSTALL_DIR="${SONAR_INSTALL_DIR:-$HOME/.local/bin}"

log() { echo "sonar-install: $*"; }
err() { echo "sonar-install: $*" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/sonar-install.XXXXXX")
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT INT TERM

if ! have go; then
	if have brew; then
		log "Go toolchain not found; installing via Homebrew..."
		brew install go
	else
		err "Go is not installed and Homebrew was not found."
		err "Install Homebrew (https://brew.sh) or Go directly (https://go.dev/dl/), then retry."
		exit 1
	fi
fi

if ! have curl; then
	err "curl is required but was not found."
	exit 1
fi
if ! have tar; then
	err "tar is required but was not found."
	exit 1
fi

log "Downloading sonar source (ref: $REPO_REF)..."
curl -fsSL "$REPO_URL/archive/refs/heads/$REPO_REF.tar.gz" | tar xz -C "$tmpdir" --strip-components=1

log "Building sonar..."
(cd "$tmpdir" && go build -trimpath -ldflags="-s -w" -o sonar ./cmd/sonar)

log "Installing to $INSTALL_DIR/sonar..."
mkdir -p "$INSTALL_DIR" 2>/dev/null || true
if [ -w "$INSTALL_DIR" ]; then
	install -m 0755 "$tmpdir/sonar" "$INSTALL_DIR/sonar"
elif have sudo && sudo -n true 2>/dev/null; then
	sudo install -m 0755 "$tmpdir/sonar" "$INSTALL_DIR/sonar"
else
	err "Cannot write to $INSTALL_DIR and no passwordless sudo is available."
	err "Re-run as a two-step download instead:"
	err "  curl -fsSL $REPO_URL/raw/$REPO_REF/scripts/install-macos.sh -o install-macos.sh && sh install-macos.sh"
	exit 1
fi

log "sonar installed to $INSTALL_DIR/sonar"
case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) log "Note: $INSTALL_DIR is not on your PATH. Add this to your shell profile:" ;
	   log "  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
log "Run 'sonar --help' to get started. Binding to ports below 1024 (default 443) needs sudo."
