# GramFix

> **Fix grammar anywhere on your Linux desktop — instantly, fully offline, with a single hotkey.**

Select text in any application, press **Alt+G**, and GramFix corrects your grammar using a local [LanguageTool](https://languagetool.org/) engine. No cloud. No API keys. No persistent inference engine.

---

## ✨ Features

- **Fully offline** — LanguageTool runs locally via its CLI JAR
- **System-wide** — works in browsers, editors, terminals, chat apps, any focusable field
- **Zero idle footprint** — the grammar engine only lives during a fix
- **Wayland + X11** — intelligent adapter selection at runtime
- **Cross-distro** — Fedora, Ubuntu, Debian, Arch, Mint, Pop!_OS, openSUSE
- **Placeholder UX** — shows `"fixing grammar…"` immediately while correction runs
- **Clipboard safe** — original clipboard is restored after every operation

---

## 🚀 Quick Start

```bash
git clone https://github.com/AniruthKarthik/gramfix
cd gramfix
bash install.sh
```

After installation:

1. **Select text** anywhere on your desktop
2. **Press Alt+G**
3. Watch the text correct itself in place

---

## 📐 Architecture

```
gramfix/
├── cmd/
│   ├── gramfix/          # Main binary: the grammar fix pipeline
│   └── gramfix-hotkey/   # Lightweight hotkey daemon
├── internal/
│   ├── core/             # Orchestrator — coordinates all components
│   ├── grammar/          # LanguageTool engine (JSON mode)
│   ├── env/              # Runtime environment detection
│   └── log/              # Structured logger
├── adapters/
│   ├── clipboard/        # wl-copy/wl-paste/xclip/xsel abstraction
│   ├── injector/         # ydotool/wtype/xdotool abstraction
│   ├── hotkey/           # sxhkd/xbindkeys daemon abstraction
│   └── platform/         # Distro/package-manager detection
├── scripts/              # install helpers, test suite, KDE shortcut setup
├── docs/                 # Guides and reference
├── install.sh            # Full installer
├── uninstall.sh          # Uninstaller
└── Makefile
```

### Runtime Flow

```
User selects text
        │
   [Alt+G pressed]
        │
   gramfix-hotkey (systemd user service, ~4MB RAM idle)
        │
        └─▶  launches: gramfix binary
                  │
                  ├─ 1. Detect environment (Wayland/X11, tools)
                  ├─ 2. Read primary selection (wl-paste --primary)
                  │       └─ fallback: clipboard (wl-paste)
                  ├─ 3. Backup current clipboard
                  ├─ 4. Write "fixing grammar…" → clipboard
                  ├─ 5. Inject Ctrl+V (ydotool) → replaces selection
                  ├─ 6. Kill intermediate wl-copy
                  ├─ 7. Run LanguageTool (java -jar LT.jar --json)
                  ├─ 8. Apply corrections (reverse-offset patching)
                  ├─ 9. Write corrected text → clipboard
                  ├─ 10. Inject Ctrl+V → replaces placeholder
                  ├─ 11. Kill intermediate wl-copy
                  ├─ 12. Restore original clipboard
                  └─ 13. Exit completely ✓
```

### Adapter Selection

| Component  | Wayland Priority      | X11 Priority           |
|------------|----------------------|------------------------|
| Clipboard  | wl-paste/wl-copy     | xclip → xsel           |
| Injector   | ydotool → wtype      | xdotool                |
| Hotkey     | sxhkd                | xbindkeys → sxhkd      |

---

## 🔧 Installation

### Prerequisites

| Tool          | Purpose                   | Package name            |
|---------------|---------------------------|-------------------------|
| `java`        | Run LanguageTool JAR      | `java-21-openjdk-headless` |
| LanguageTool  | Grammar correction        | `languagetool`          |
| `wl-copy`     | Wayland clipboard         | `wl-clipboard`          |
| `ydotool`     | Key injection (Wayland)   | `ydotool`               |
| `sxhkd`       | Hotkey daemon             | `sxhkd`                 |
| `xclip`       | X11 clipboard             | `xclip`                 |
| `xdotool`     | X11 key injection         | `xdotool`               |

### Install

```bash
bash install.sh          # auto-detects distro, installs deps, builds, sets up hotkey
```

### Manual Install

```bash
# 1. Install Go (https://go.dev/dl/)
# 2. Install LanguageTool
sudo dnf install languagetool      # Fedora
sudo apt install languagetool      # Ubuntu/Debian

# 3. Install input tools
sudo dnf install wl-clipboard ydotool sxhkd xclip xdotool

# 4. Build and install
make build
make install
make systemd              # enable hotkey daemon as systemd user service
```

### KDE Plasma Setup

```bash
bash scripts/setup-kde-shortcut.sh
```

Or manually: **System Settings → Shortcuts → Custom Shortcuts → New Command/URL**
- Name: `GramFix`
- Trigger: `Alt+G`
- Command: `~/.local/bin/gramfix`

---

## 🖥️ Usage

### Via Hotkey (normal use)

1. Highlight text anywhere
2. Press **Alt+G**
3. Text is corrected in place

### Via Command Line

```bash
# Fix grammar using current primary selection
gramfix

# Specify language
gramfix --lang en-GB

# Debug mode
gramfix --debug

# Log to file (~/.local/share/gramfix/gramfix.log)
gramfix --log
```

### Manage the Hotkey Daemon

```bash
systemctl --user status gramfix-hotkey     # check status
systemctl --user restart gramfix-hotkey    # restart
systemctl --user stop gramfix-hotkey       # stop
journalctl --user -u gramfix-hotkey -f     # follow logs
```

---

## 🌍 Supported Languages

Any language supported by LanguageTool. Common codes:

| Code    | Language          |
|---------|-------------------|
| `en-US` | English (US)      |
| `en-GB` | English (UK)      |
| `de-DE` | German            |
| `fr`    | French            |
| `es`    | Spanish           |
| `pt-BR` | Portuguese (BR)   |

Pass `--lang CODE` to gramfix or edit the service file.

---

## 🔍 Troubleshooting

### "Compositor does not support the virtual keyboard protocol" (wtype)

This means wtype can't inject keystrokes. **Fix**: use ydotool instead.

```bash
# Ensure ydotoold is running
systemctl --user status ydotoold 2>/dev/null || sudo systemctl status ydotoold
# Check socket
ls /tmp/.ydotool_socket
```

### Alt+G Not Working

1. Check sxhkd is alive: `ps aux | grep sxhkd`
2. Check hotkey service: `systemctl --user status gramfix-hotkey`
3. On KDE: add a **Custom Shortcut** in System Settings (more reliable than sxhkd on Wayland)
4. On GNOME: add via **Settings → Keyboard → Custom Shortcuts**

### LanguageTool Not Found

```bash
# Check if installed
ls /usr/share/languagetool/languagetool-commandline.jar

# Override path
export GRAMFIX_LT_JAR=/path/to/languagetool-commandline.jar
gramfix
```

### Clipboard Not Restored

Clipboard restoration requires a `wl-copy` process to stay alive on Wayland. This is normal Wayland clipboard behavior — the clipboard provider must remain running. If you kill `gramfix` mid-run, use `wl-copy < /dev/null` to clear the clipboard.

### Debug Mode

```bash
gramfix --debug --log
tail -f ~/.local/share/gramfix/gramfix.log
```

---

## 🏗️ Building from Source

```bash
git clone https://github.com/anilnair00/gramfix
cd gramfix

make build      # → build/gramfix  build/gramfix-hotkey
make test       # run unit tests
make vet        # go vet
make fmt        # gofmt

# Integration tests
bash scripts/test-integration.sh
```

---

## 📦 Distro Compatibility

| Distro         | Status       | Notes                              |
|----------------|--------------|------------------------------------|
| Fedora 40+     | ✅ Tested    | All tools available via dnf        |
| Ubuntu 22.04+  | ✅ Supported | `languagetool` in universe         |
| Debian 12+     | ✅ Supported | `languagetool` in main             |
| Arch Linux     | ✅ Supported | `languagetool` in community        |
| Linux Mint     | ✅ Supported | Ubuntu-based, same packages        |
| Pop!_OS        | ✅ Supported | Ubuntu-based                       |
| openSUSE Leap  | ✅ Supported | `languagetool` via zypper          |

---

## ⚠️ Limitations

- **LanguageTool startup latency**: ~3–5 seconds (JVM startup). This is inherent to running a Java process.
- **wtype on Wayland**: Requires compositor support for `zwp_virtual_keyboard_v1`. KDE Plasma and most compositors support this, but some may not. **ydotool is the preferred injector** as it bypasses the compositor entirely via `/dev/uinput`.
- **Terminal paste**: Ctrl+V in some terminals pastes literal text rather than triggering paste. In those cases use Ctrl+Shift+V (terminal-specific). The gramfix-hotkey daemon handles this via direct type fallback.
- **Root requirement for ydotool**: `ydotoold` must run as root or user must be in `input` group to write `/dev/uinput`.

---

## 📄 License

MIT License — see [LICENSE](LICENSE)
