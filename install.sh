#!/usr/bin/env bash
# install.sh — GramFix installer
# Installs dependencies, builds binaries, sets up the hotkey daemon.
set -euo pipefail

###############################################################################
# Config
###############################################################################
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
GRAMFIX_BIN="$INSTALL_DIR/gramfix"
HOTKEY_BIN="$INSTALL_DIR/gramfix-hotkey"
SYSTEMD_DIR="$HOME/.config/systemd/user"
SERVICE_FILE="$SYSTEMD_DIR/gramfix-hotkey.service"
LT_JAR_PATH="/usr/share/languagetool/languagetool-commandline.jar"
LT_DOWNLOAD_DIR="/tmp/languagetool-install"
LT_VERSION="6.4"
LT_URL="https://www.languagetool.org/download/LanguageTool-${LT_VERSION}.zip"

###############################################################################
# Helpers
###############################################################################
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
die()   { error "$*"; exit 1; }

has_cmd() { command -v "$1" &>/dev/null; }

###############################################################################
# Distro detection
###############################################################################
detect_distro() {
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        echo "${ID,,}"
    else
        echo "unknown"
    fi
}

detect_pm() {
    local distro
    distro=$(detect_distro)
    case "$distro" in
        fedora|rhel|centos) echo "dnf" ;;
        ubuntu|debian|linuxmint|pop) echo "apt" ;;
        arch|manjaro) echo "pacman" ;;
        opensuse*) echo "zypper" ;;
        *) echo "unknown" ;;
    esac
}

pkg_install() {
    local pm="$1"; shift
    case "$pm" in
        dnf)    sudo dnf install -y "$@" ;;
        apt)    sudo apt-get install -y "$@" ;;
        pacman) sudo pacman -S --noconfirm "$@" ;;
        zypper) sudo zypper install -y "$@" ;;
        *)      warn "Unknown package manager; install manually: $*" ;;
    esac
}

###############################################################################
# Step 1: Detect environment
###############################################################################
info "=== GramFix Installer ==="
DISTRO=$(detect_distro)
PM=$(detect_pm)
SESSION_TYPE="${XDG_SESSION_TYPE:-unknown}"
info "Distro: $DISTRO  |  Package manager: $PM  |  Session: $SESSION_TYPE"

###############################################################################
# Step 2: Install system dependencies
###############################################################################
info "Checking system dependencies..."

MISSING_PKGS=()

# Java (required for LanguageTool)
if ! has_cmd java; then
    case "$PM" in
        dnf)    MISSING_PKGS+=("java-21-openjdk-headless") ;;
        apt)    MISSING_PKGS+=("default-jre-headless") ;;
        pacman) MISSING_PKGS+=("jre-openjdk-headless") ;;
        zypper) MISSING_PKGS+=("java-21-openjdk-headless") ;;
    esac
fi

# Wayland tools
if [[ "$SESSION_TYPE" == "wayland" ]]; then
    has_cmd wl-copy  || MISSING_PKGS+=("wl-clipboard")
    has_cmd wtype    || case "$PM" in
        dnf)    MISSING_PKGS+=("wtype") ;;
        apt)    MISSING_PKGS+=("wtype") ;;
        pacman) MISSING_PKGS+=("wtype") ;;
        zypper) MISSING_PKGS+=("wtype") ;;
    esac
    has_cmd ydotool  || case "$PM" in
        dnf)    MISSING_PKGS+=("ydotool") ;;
        apt)    MISSING_PKGS+=("ydotool") ;;
        pacman) MISSING_PKGS+=("ydotool") ;;
        zypper) MISSING_PKGS+=("ydotool") ;;
    esac
fi

# X11 tools
if [[ "$SESSION_TYPE" == "x11" ]] || [[ -n "${DISPLAY:-}" ]]; then
    has_cmd xclip   || MISSING_PKGS+=("xclip")
    has_cmd xsel    || MISSING_PKGS+=("xsel")
    has_cmd xdotool || MISSING_PKGS+=("xdotool")
    has_cmd xbindkeys || MISSING_PKGS+=("xbindkeys")
fi

if [[ ${#MISSING_PKGS[@]} -gt 0 ]]; then
    info "Installing missing packages: ${MISSING_PKGS[*]}"
    # Deduplicate
    UNIQUE_PKGS=($(printf '%s\n' "${MISSING_PKGS[@]}" | sort -u))
    pkg_install "$PM" "${UNIQUE_PKGS[@]}" || warn "Some packages may have failed to install"
else
    info "All system dependencies present"
fi

###############################################################################
# Step 3: LanguageTool JAR
###############################################################################
info "Checking LanguageTool..."

if [ -f "$LT_JAR_PATH" ]; then
    info "LanguageTool found at $LT_JAR_PATH"
elif has_cmd languagetool; then
    info "System languagetool command available"
else
    # Try package manager first
    LT_INSTALLED=false
    case "$PM" in
        dnf)
            if sudo dnf install -y languagetool 2>/dev/null; then
                LT_INSTALLED=true
            fi
            ;;
        apt)
            if sudo apt-get install -y languagetool 2>/dev/null; then
                LT_INSTALLED=true
            fi
            ;;
        pacman)
            if sudo pacman -S --noconfirm languagetool 2>/dev/null; then
                LT_INSTALLED=true
            fi
            ;;
    esac

    if ! $LT_INSTALLED; then
        warn "LanguageTool not available via package manager"
        if has_cmd curl || has_cmd wget; then
            info "Downloading LanguageTool $LT_VERSION..."
            mkdir -p "$LT_DOWNLOAD_DIR"
            ZIPFILE="$LT_DOWNLOAD_DIR/LanguageTool-${LT_VERSION}.zip"

            if has_cmd curl; then
                curl -fsSL -o "$ZIPFILE" "$LT_URL" || die "Download failed"
            else
                wget -q -O "$ZIPFILE" "$LT_URL" || die "Download failed"
            fi

            if has_cmd unzip; then
                unzip -q "$ZIPFILE" -d "$LT_DOWNLOAD_DIR"
                LT_DIR=$(find "$LT_DOWNLOAD_DIR" -name "languagetool-commandline.jar" -printf '%h\n' | head -1)
                sudo mkdir -p /usr/local/share/languagetool
                sudo cp "$LT_DIR"/*.jar /usr/local/share/languagetool/ 2>/dev/null || true
                sudo cp -r "$LT_DIR"/libs /usr/local/share/languagetool/ 2>/dev/null || true
                info "LanguageTool installed to /usr/local/share/languagetool/"
            else
                die "unzip not found; cannot extract LanguageTool"
            fi
            rm -rf "$LT_DOWNLOAD_DIR"
        else
            die "Cannot download LanguageTool: curl/wget not found. Install manually."
        fi
    fi
fi

# Verify LanguageTool works
if has_cmd java; then
    JAR=$(find /usr/share/languagetool /usr/local/share/languagetool /opt/languagetool \
              -name "languagetool-commandline.jar" 2>/dev/null | head -1)
    if [ -n "$JAR" ]; then
        info "LanguageTool JAR: $JAR"
    else
        warn "LanguageTool JAR not found in standard paths; set GRAMFIX_LT_JAR env var if needed"
    fi
fi

###############################################################################
# Step 4: Build GramFix
###############################################################################
info "Building GramFix..."

if ! has_cmd go; then
    die "Go not found. Install Go from https://go.dev/dl/"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

go mod tidy
make build

###############################################################################
# Step 5: Install binaries
###############################################################################
info "Installing binaries to $INSTALL_DIR..."
mkdir -p "$INSTALL_DIR"
make install INSTALL_DIR="$INSTALL_DIR"

###############################################################################
# Step 6: Set up hotkey
###############################################################################
info "Setting up hotkey daemon..."

# Ensure ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    warn "~/.local/bin is not in PATH"
    warn "Add 'export PATH=\"\$HOME/.local/bin:\$PATH\"' to your ~/.bashrc or ~/.zshrc"
fi

# Systemd user service
mkdir -p "$SYSTEMD_DIR"
sed \
    -e "s|__HOTKEY_BIN__|${HOTKEY_BIN}|g" \
    -e "s|__GRAMFIX_BIN__|${GRAMFIX_BIN}|g" \
    "$SCRIPT_DIR/scripts/gramfix-hotkey.service.template" \
    > "$SERVICE_FILE"

if has_cmd systemctl; then
    systemctl --user daemon-reload
    systemctl --user enable --now gramfix-hotkey.service && \
        info "gramfix-hotkey service enabled and started" || \
        warn "Could not start service automatically; run: systemctl --user start gramfix-hotkey"
else
    warn "systemctl not available; start hotkey daemon manually: $HOTKEY_BIN"
fi

###############################################################################
# Step 7: KDE / GNOME shortcut guidance
###############################################################################
echo ""
info "=== Hotkey Setup ==="
echo ""
case "${XDG_CURRENT_DESKTOP:-}" in
    KDE|PLASMA*)
        info "KDE detected. The systemd service handles Alt+G via xbindkeys."
        info "Alternatively, add a KDE custom shortcut:"
        info "  System Settings → Shortcuts → Custom Shortcuts → New → Command: $GRAMFIX_BIN"
        ;;
    GNOME)
        info "GNOME detected. Add a custom keyboard shortcut:"
        info "  Settings → Keyboard → Keyboard Shortcuts → Custom Shortcuts"
        info "  Command: $GRAMFIX_BIN"
        info "  Shortcut: Alt+G"
        ;;
    *)
        info "Add a keyboard shortcut in your DE settings:"
        info "  Command: $GRAMFIX_BIN"
        info "  Shortcut: Alt+G"
        ;;
esac

echo ""
info "=== Installation Complete ==="
info "gramfix:        $GRAMFIX_BIN"
info "gramfix-hotkey: $HOTKEY_BIN"
echo ""
info "Usage:"
info "  Select text anywhere → press Alt+G"
info "  Or run manually: $GRAMFIX_BIN"
echo ""
info "Troubleshooting: see docs/troubleshooting.md"
