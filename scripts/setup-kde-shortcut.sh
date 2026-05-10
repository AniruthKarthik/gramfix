#!/usr/bin/env bash
# scripts/setup-kde-shortcut.sh
# Sets up Alt+G as a KDE custom keyboard shortcut for gramfix.
# Works on KDE Plasma 5 and 6.
set -euo pipefail

GRAMFIX_BIN="${1:-$HOME/.local/bin/gramfix}"
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

if [[ ! -f "$GRAMFIX_BIN" ]]; then
    echo "gramfix not found at $GRAMFIX_BIN"
    echo "Run 'make install' first, or pass the binary path as argument"
    exit 1
fi

info "Setting up KDE shortcut: Alt+G → $GRAMFIX_BIN"

KHOTKEYS_RC="$HOME/.config/khotkeysrc"

# Method 1: khotkeys (KDE 5/6 custom shortcuts)
if command -v kwriteconfig6 &>/dev/null || command -v kwriteconfig5 &>/dev/null; then
    KWC="kwriteconfig6"
    command -v kwriteconfig6 &>/dev/null || KWC="kwriteconfig5"

    # Check if gramfix shortcut already exists
    if grep -q "gramfix" "$KHOTKEYS_RC" 2>/dev/null; then
        warn "gramfix shortcut already exists in $KHOTKEYS_RC"
    else
        # Append gramfix shortcut to khotkeysrc
        # This uses KDE's khotkeys format
        NEXT_ID=$(grep -c '^\[Data_' "$KHOTKEYS_RC" 2>/dev/null || echo 0)
        NEXT_ID=$((NEXT_ID + 1))

        cat >> "$KHOTKEYS_RC" << EOF

[Data_${NEXT_ID}]
Comment=GramFix shortcut
Enabled=true
Name=GramFix
Type=SIMPLE_ACTION_DATA

[Data_${NEXT_ID}Actions]
ActionsCount=1

[Data_${NEXT_ID}Actions0]
CommandURL=$GRAMFIX_BIN
Type=COMMAND_URL

[Data_${NEXT_ID}Conditions]
Comment=
ConditionsCount=0

[Data_${NEXT_ID}Triggers]
Comment=Simple_action
TriggersCount=1

[Data_${NEXT_ID}Triggers0]
Key=Alt+G
Type=SHORTCUT
Uuid={gramfix-$(uuidgen 2>/dev/null || date +%s)}
EOF
        info "Added gramfix to $KHOTKEYS_RC"
    fi

    # Signal KDE to reload hotkeys
    if command -v qdbus6 &>/dev/null; then
        qdbus6 org.kde.khotkeys /khotkeys reread_configuration 2>/dev/null || true
    elif command -v qdbus &>/dev/null; then
        qdbus org.kde.khotkeys /khotkeys reread_configuration 2>/dev/null || true
    fi
    info "KDE hotkeys reloaded"
fi

# Method 2: Also create an autostart entry that launches gramfix-hotkey
# so it works even without KDE custom shortcuts
AUTOSTART_DIR="$HOME/.config/autostart"
mkdir -p "$AUTOSTART_DIR"
cat > "$AUTOSTART_DIR/gramfix-hotkey.desktop" << EOF
[Desktop Entry]
Type=Application
Name=GramFix Hotkey Daemon
Comment=Listens for Alt+G to fix selected text grammar
Exec=$HOME/.local/bin/gramfix-hotkey --gramfix=$GRAMFIX_BIN
X-KDE-autostart-condition=khotkeysrc:Main:Enabled:true
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
EOF
info "Autostart entry created: $AUTOSTART_DIR/gramfix-hotkey.desktop"

echo ""
info "Setup complete!"
info "• Restart KDE or log out/in for shortcuts to take effect"
info "• gramfix-hotkey daemon is also started on login via autostart"
info ""
info "Alternative: Configure manually in KDE System Settings:"
info "  System Settings → Shortcuts → Custom Shortcuts → + → Command/URL"
info "  Trigger: Alt+G"
info "  Command: $GRAMFIX_BIN"
