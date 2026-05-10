#!/usr/bin/env bash
# uninstall.sh — Remove GramFix
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
SYSTEMD_DIR="$HOME/.config/systemd/user"

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

info "Stopping and disabling gramfix-hotkey service..."
systemctl --user disable --now gramfix-hotkey.service 2>/dev/null || true
rm -f "$SYSTEMD_DIR/gramfix-hotkey.service"
systemctl --user daemon-reload 2>/dev/null || true

info "Removing binaries..."
rm -f "$INSTALL_DIR/gramfix"
rm -f "$INSTALL_DIR/gramfix-hotkey"

# Remove xbindkeys config if it was created by gramfix
if [ -f "$HOME/.xbindkeysrc.gramfix" ]; then
    rm -f "$HOME/.xbindkeysrc.gramfix"
fi

info "GramFix uninstalled."
info "Log files remain at ~/.local/share/gramfix/ (remove manually if desired)"
