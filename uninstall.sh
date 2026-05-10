#!/usr/bin/env bash
# uninstall.sh — Remove GramFix
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/gramfix"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/gramfix"
SYSTEMD_DIR="$HOME/.config/systemd/user"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

has_cmd() { command -v "$1" &>/dev/null; }

echo ""
echo -e "${BOLD}=== GramFix Uninstaller ===${NC}"
echo ""

# ── Services ────────────────────────────────────────────────────────────────
info "Stopping services..."
if has_cmd systemctl; then
    systemctl --user disable --now gramfix-hotkey.service  2>/dev/null || true
    systemctl --user disable --now gramfix-lt-server.service 2>/dev/null || true
    systemctl --user daemon-reload 2>/dev/null || true
fi

rm -f "$SYSTEMD_DIR/gramfix-hotkey.service"
rm -f "$SYSTEMD_DIR/gramfix-lt-server.service"

# ── Binaries ─────────────────────────────────────────────────────────────────
info "Removing binaries..."
rm -f "$INSTALL_DIR/gramfix"
rm -f "$INSTALL_DIR/gramfix-hotkey"
rm -f "$INSTALL_DIR/gramfix-check"

# ── Config and rules ─────────────────────────────────────────────────────────
if [ -d "$CONFIG_DIR" ]; then
    echo ""
    echo -e "${YELLOW}Config directory found: $CONFIG_DIR${NC}"
    echo "  Contains: gramfix.conf and any custom grammar rules."
    read -r -p "  Remove config and rules? [y/N] " REPLY
    if [[ "${REPLY,,}" == "y" ]]; then
        rm -rf "$CONFIG_DIR"
        info "Config removed: $CONFIG_DIR"
    else
        info "Config kept: $CONFIG_DIR"
    fi
fi

# ── Data / logs ───────────────────────────────────────────────────────────────
if [ -d "$DATA_DIR" ]; then
    echo ""
    echo -e "${YELLOW}Data directory found: $DATA_DIR${NC}"
    echo "  Contains: gramfix.log and any cached data."
    read -r -p "  Remove logs and data? [y/N] " REPLY
    if [[ "${REPLY,,}" == "y" ]]; then
        rm -rf "$DATA_DIR"
        info "Data removed: $DATA_DIR"
    else
        info "Data kept: $DATA_DIR"
    fi
fi

# ── xbindkeys remnants ────────────────────────────────────────────────────────
rm -f "$HOME/.xbindkeysrc.gramfix"

# ── Lock file ─────────────────────────────────────────────────────────────────
rm -f /tmp/gramfix.lock

echo ""
info "GramFix uninstalled."
echo ""
