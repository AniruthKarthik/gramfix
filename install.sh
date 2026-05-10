#!/usr/bin/env bash
# install.sh — GramFix installer
# Installs dependencies, builds binaries, deploys config/rules, and sets up the hotkey daemon.
set -euo pipefail

###############################################################################
# Config
###############################################################################
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/gramfix"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/gramfix"
SYSTEMD_DIR="$HOME/.config/systemd/user"
SERVICE_FILE="$SYSTEMD_DIR/gramfix-hotkey.service"

GRAMFIX_BIN="$INSTALL_DIR/gramfix"
HOTKEY_BIN="$INSTALL_DIR/gramfix-hotkey"
CHECK_BIN="$INSTALL_DIR/gramfix-check"

LT_JAR_PATH="/usr/share/languagetool/languagetool-commandline.jar"
LT_DOWNLOAD_DIR="/tmp/languagetool-install"
LT_VERSION="6.6"
LT_URL="https://www.languagetool.org/download/LanguageTool-${LT_VERSION}.zip"

###############################################################################
# Helpers
###############################################################################
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }
die()     { error "$*"; exit 1; }
section() { echo -e "\n${BOLD}$*${NC}"; }

has_cmd() { command -v "$1" &>/dev/null; }

###############################################################################
# Distro / package-manager detection
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
        fedora|rhel|centos|rocky|alma) echo "dnf" ;;
        ubuntu|debian|linuxmint|pop|elementary|zorin) echo "apt" ;;
        arch|manjaro|endeavouros|garuda) echo "pacman" ;;
        opensuse*|sles) echo "zypper" ;;
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
# Step 0: Banner
###############################################################################
echo ""
echo -e "${BOLD}=== GramFix Installer ===${NC}"
DISTRO=$(detect_distro)
PM=$(detect_pm)
SESSION_TYPE="${XDG_SESSION_TYPE:-unknown}"
info "Distro: $DISTRO  |  Package manager: $PM  |  Session: $SESSION_TYPE"
echo ""

###############################################################################
# Step 1: System dependencies
###############################################################################
section "Step 1: Checking system dependencies"

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
    has_cmd ydotool  || MISSING_PKGS+=("ydotool")
fi

# X11 tools
if [[ "$SESSION_TYPE" == "x11" ]] || [[ -n "${DISPLAY:-}" ]]; then
    has_cmd xclip    || MISSING_PKGS+=("xclip")
    has_cmd xdotool  || MISSING_PKGS+=("xdotool")
fi

# Hotkey daemon (works on both Wayland and X11)
has_cmd sxhkd || case "$PM" in
    dnf)    MISSING_PKGS+=("sxhkd") ;;
    apt)    MISSING_PKGS+=("sxhkd") ;;
    pacman) MISSING_PKGS+=("sxhkd") ;;
    zypper) MISSING_PKGS+=("sxhkd") ;;
esac

if [[ ${#MISSING_PKGS[@]} -gt 0 ]]; then
    info "Installing missing packages: ${MISSING_PKGS[*]}"
    UNIQUE_PKGS=($(printf '%s\n' "${MISSING_PKGS[@]}" | sort -u))
    pkg_install "$PM" "${UNIQUE_PKGS[@]}" || warn "Some packages may have failed to install"
else
    info "All system dependencies present"
fi

###############################################################################
# Step 2: LanguageTool JAR
###############################################################################
section "Step 2: Checking LanguageTool"

# Search known installation paths
find_lt_jar() {
    local paths=(
        "/usr/share/languagetool/languagetool-commandline.jar"
        "/usr/share/java/languagetool/languagetool-commandline.jar"
        "/usr/local/share/languagetool/languagetool-commandline.jar"
        "/opt/languagetool/languagetool-commandline.jar"
    )
    for p in "${paths[@]}"; do
        [ -f "$p" ] && echo "$p" && return 0
    done
    return 1
}

if find_lt_jar > /dev/null 2>&1; then
    info "LanguageTool JAR found: $(find_lt_jar)"
else
    LT_INSTALLED=false

    case "$PM" in
        dnf)
            sudo dnf install -y languagetool 2>/dev/null && LT_INSTALLED=true || true ;;
        apt)
            sudo apt-get install -y languagetool 2>/dev/null && LT_INSTALLED=true || true ;;
        pacman)
            sudo pacman -S --noconfirm languagetool 2>/dev/null && LT_INSTALLED=true || true ;;
    esac

    if ! $LT_INSTALLED; then
        warn "LanguageTool not available via package manager; downloading v${LT_VERSION}..."
        if ! has_cmd curl && ! has_cmd wget; then
            die "curl or wget required to download LanguageTool. Install one first."
        fi
        has_cmd unzip || die "unzip required to extract LanguageTool. Install it first."

        mkdir -p "$LT_DOWNLOAD_DIR"
        ZIPFILE="$LT_DOWNLOAD_DIR/LanguageTool-${LT_VERSION}.zip"

        if has_cmd curl; then
            curl -fsSL --progress-bar -o "$ZIPFILE" "$LT_URL" || die "Download failed"
        else
            wget -q --show-progress -O "$ZIPFILE" "$LT_URL" || die "Download failed"
        fi

        unzip -q "$ZIPFILE" -d "$LT_DOWNLOAD_DIR"
        LT_DIR=$(find "$LT_DOWNLOAD_DIR" -name "languagetool-commandline.jar" -printf '%h\n' | head -1)
        sudo mkdir -p /usr/local/share/languagetool
        sudo cp "$LT_DIR"/*.jar /usr/local/share/languagetool/ 2>/dev/null || true
        sudo cp -r "$LT_DIR"/libs /usr/local/share/languagetool/ 2>/dev/null || true
        rm -rf "$LT_DOWNLOAD_DIR"
        info "LanguageTool ${LT_VERSION} installed to /usr/local/share/languagetool/"
    fi
fi

###############################################################################
# Step 3: Build GramFix
###############################################################################
section "Step 3: Building GramFix"

has_cmd go || die "Go not found. Install Go from https://go.dev/dl/ then re-run install.sh"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

go mod tidy 2>/dev/null || true
make build
info "Build complete"

###############################################################################
# Step 4: Install binaries
###############################################################################
section "Step 4: Installing binaries"

mkdir -p "$INSTALL_DIR"
install -m 755 build/gramfix        "$GRAMFIX_BIN"
install -m 755 build/gramfix-hotkey "$HOTKEY_BIN"
install -m 755 build/gramfix-check  "$CHECK_BIN"
info "Installed: $GRAMFIX_BIN"
info "Installed: $HOTKEY_BIN"
info "Installed: $CHECK_BIN"

###############################################################################
# Step 5: Deploy config and custom rules
###############################################################################
section "Step 5: Deploying config and grammar rules"

mkdir -p "$CONFIG_DIR/rules"
mkdir -p "$DATA_DIR"

# Install config file (do not overwrite if already customised)
CONF_DEST="$CONFIG_DIR/gramfix.conf"
if [ ! -f "$CONF_DEST" ]; then
    install -m 644 "$SCRIPT_DIR/configs/gramfix.conf" "$CONF_DEST"
    info "Config installed: $CONF_DEST"
else
    info "Config already exists (not overwritten): $CONF_DEST"
    info "Reference: $SCRIPT_DIR/configs/gramfix.conf"
fi

# Custom grammar rules (always update — these are part of the engine)
RULES_DEST="$CONFIG_DIR/rules/gramfix-custom.xml"
install -m 644 "$SCRIPT_DIR/configs/rules/gramfix-custom.xml" "$RULES_DEST"
info "Grammar rules installed: $RULES_DEST"

# Point GRAMFIX_CUSTOM_RULES to the installed path in the config
if grep -q "^GRAMFIX_CUSTOM_RULES=" "$CONF_DEST" 2>/dev/null; then
    sed -i "s|^GRAMFIX_CUSTOM_RULES=.*|GRAMFIX_CUSTOM_RULES=$RULES_DEST|" "$CONF_DEST"
else
    echo "" >> "$CONF_DEST"
    echo "GRAMFIX_CUSTOM_RULES=$RULES_DEST" >> "$CONF_DEST"
fi

###############################################################################
# Step 6: Systemd hotkey service
###############################################################################
section "Step 6: Setting up hotkey daemon"

# Ensure ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    warn "~/.local/bin is not in PATH"
    warn "Add this to your ~/.bashrc or ~/.zshrc:"
    warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

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
    warn "systemctl not available; start the hotkey daemon manually: $HOTKEY_BIN"
fi

###############################################################################
# Step 7: Verify installation
###############################################################################
section "Step 7: Verifying installation"

if "$CHECK_BIN" 2>/dev/null; then
    info "gramfix-check: all checks passed"
else
    warn "gramfix-check reported issues above; check the output before using GramFix"
fi

###############################################################################
# Step 8: Desktop environment shortcut guidance
###############################################################################
echo ""
info "=== Hotkey Setup ==="
echo ""
case "${XDG_CURRENT_DESKTOP:-}" in
    KDE|KDE5|PLASMA*)
        info "KDE detected. The systemd service handles Alt+G."
        info "For a more reliable shortcut, add a KDE Custom Shortcut:"
        info "  System Settings > Shortcuts > Custom Shortcuts > New Command/URL"
        info "  Trigger: Alt+G  |  Command: $GRAMFIX_BIN"
        ;;
    GNOME)
        info "GNOME detected. Add a custom keyboard shortcut:"
        info "  Settings > Keyboard > Keyboard Shortcuts > Custom Shortcuts"
        info "  Name: GramFix  |  Command: $GRAMFIX_BIN  |  Shortcut: Alt+G"
        ;;
    *)
        info "Add a keyboard shortcut in your DE settings:"
        info "  Command: $GRAMFIX_BIN  |  Shortcut: Alt+G"
        ;;
esac

###############################################################################
# Done
###############################################################################
echo ""
echo -e "${BOLD}=== Installation Complete ===${NC}"
echo ""
info "Binaries:"
info "  gramfix:        $GRAMFIX_BIN"
info "  gramfix-hotkey: $HOTKEY_BIN"
info "  gramfix-check:  $CHECK_BIN"
echo ""
info "Config:           $CONF_DEST"
info "Grammar rules:    $RULES_DEST"
echo ""
info "Usage:"
info "  Highlight text anywhere, press Alt+G — corrected in place"
info "  Or: echo 'text to fix' | gramfix --stdin"
echo ""
info "Troubleshooting:  $GRAMFIX_BIN --debug"
info "Reference:        docs/troubleshooting.md"
echo ""
