# gramfix

A lightweight, cross-distro Linux offline grammar correction utility.

## Overview
`gramfix` allows you to correct text globally across your Linux desktop. Simply select the text, press a hotkey (like `Ctrl+G`), and the text will be automatically corrected and replaced—completely offline, with no persistent daemons or API calls.

## Architecture
- **Offline & Local**: Uses `LanguageTool` locally to perform grammar checks. No data leaves your machine.
- **Cross-Platform**: Automatically detects your display server and tools:
  - **Wayland**: Uses `wl-clipboard` for text retrieval and `wtype` (or `ydotool`) for injection.
  - **X11**: Uses `xclip`/`xsel` for text retrieval and `xdotool` for injection.

## Installation
Run the install script, which auto-detects your distro (Debian/Ubuntu, Fedora, Arch) and installs dependencies.

```bash
chmod +x scripts/install.sh
./scripts/install.sh
```

## Setup Hotkey
Bind the `/usr/local/bin/gramfix` command to `Ctrl+G` using your Desktop Environment's custom keyboard shortcuts.

### GNOME
1. Settings -> Keyboard -> Keyboard Shortcuts -> View and Customize Shortcuts -> Custom Shortcuts.
2. Add a new shortcut: Name: "Gramfix", Command: `/usr/local/bin/gramfix`, Shortcut: `Ctrl+G`.

### KDE Plasma
1. System Settings -> Shortcuts -> Custom Shortcuts.
2. Add a new global shortcut with the command `/usr/local/bin/gramfix`.

## Limitations
- Depending on the text field and framework, text selection replacing via `Ctrl+V` might behave differently.
- Requires Java to run `LanguageTool` CLI (though no background Java daemon runs continuously).
