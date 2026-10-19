#!/usr/bin/env bash
set -euo pipefail

# LogCaTool Build & Install Script
# Usage: ./install.sh [--uninstall] [--prefix /usr/local]

APP_NAME="LogcatTool"
BINARY_NAME="LogcatTool"
VERSION="0.1.0"

# Default install prefix
PREFIX="${PREFIX:-/usr/local}"
BINDIR="$PREFIX/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; }

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
    --uninstall       Remove installed binary
    --prefix PATH     Install prefix (default: /usr/local)
    --user            Install to \$HOME/.local/bin instead
    -h, --help        Show this help

Examples:
    $0                    # Install to /usr/local/bin (may need sudo)
    $0 --user             # Install to ~/.local/bin
    $0 --prefix /opt      # Install to /opt/bin
    $0 --uninstall        # Remove from /usr/local/bin
    $0 --uninstall --user # Remove from ~/.local/bin
EOF
}

# Parse args
UNINSTALL=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --uninstall) UNINSTALL=true; shift ;;
        --prefix) PREFIX="$2"; BINDIR="$PREFIX/bin"; shift 2 ;;
        --user) PREFIX="$HOME/.local"; BINDIR="$PREFIX/bin"; shift ;;
        -h|--help) usage; exit 0 ;;
        *) err "Unknown option: $1"; usage; exit 1 ;;
    esac
done

# Uninstall mode
if [[ "$UNINSTALL" == true ]]; then
    TARGET="$BINDIR/$BINARY_NAME"
    if [[ -f "$TARGET" ]]; then
        info "Removing $TARGET ..."
        rm -f "$TARGET"
        ok "Uninstalled $APP_NAME"
    else
        warn "$TARGET not found, nothing to uninstall"
    fi
    exit 0
fi

# Check Go
info "Checking Go toolchain ..."
if ! command -v go &>/dev/null; then
    err "Go is not installed. Please install Go 1.25+ first."
    err "  https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
info "Found Go $GO_VERSION"

# Check minimum Go version (1.25+)
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
if [[ "$GO_MAJOR" -lt 1 ]] || [[ "$GO_MAJOR" -eq 1 && "$GO_MINOR" -lt 25 ]]; then
    err "Go 1.25+ required, found $GO_VERSION"
    exit 1
fi

# Ensure bindir exists
if [[ ! -d "$BINDIR" ]]; then
    info "Creating $BINDIR ..."
    mkdir -p "$BINDIR"
fi

# Build
info "Building $APP_NAME v$VERSION ..."
cd "$(dirname "$0")"
go build -ldflags="-s -w -X main.version=$VERSION" -o "$BINARY_NAME" .
ok "Build successful: $(pwd)/$BINARY_NAME"

# Install
TARGET="$BINDIR/$BINARY_NAME"
info "Installing to $TARGET ..."

# Check if we need sudo for system-wide install
if [[ "$BINDIR" == /usr/* || "$BINDIR" == /opt/* ]]; then
    if [[ ! -w "$BINDIR" ]]; then
        warn "Need sudo to write to $BINDIR"
        sudo cp "$BINARY_NAME" "$TARGET"
        sudo chmod +x "$TARGET"
    else
        cp "$BINARY_NAME" "$TARGET"
        chmod +x "$TARGET"
    fi
else
    cp "$BINARY_NAME" "$TARGET"
    chmod +x "$TARGET"
fi

ok "Installed: $TARGET"

# Verify
if command -v "$BINARY_NAME" &>/dev/null; then
    INSTALLED_VERSION=$("$BINARY_NAME" -v 2>/dev/null || true)
    ok "Verification: $INSTALLED_VERSION"
else
    warn "$BINARY_NAME is not in your PATH"
    info "Add this to your shell profile:"
    echo "    export PATH=\"\$PATH:$BINDIR\""
fi

# Config directory
CONFIG_DIR="$HOME/.config/logcatool"
if [[ ! -d "$CONFIG_DIR" ]]; then
    info "Creating config directory: $CONFIG_DIR"
    mkdir -p "$CONFIG_DIR"
fi

cat <<EOF

${GREEN}✓ Installation complete!${NC}

Quick start:
    $BINARY_NAME              # Live logcat from connected device
    $BINARY_NAME -f log.txt   # Read from file
    $BINARY_NAME -s <serial>  # Specific device
    $BINARY_NAME --debug      # Enable debug logging

Config file:
    ~/.config/logcatool/config.json

To uninstall:
    $0 --uninstall
EOF
