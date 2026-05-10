# GramFix

> Fix grammar anywhere on your Linux desktop — instantly, fully offline, with a single hotkey.

Select text in any application, press **Alt+G**, and GramFix corrects your grammar using a local [LanguageTool](https://languagetool.org/) engine. No cloud. No API keys. No persistent inference engine.

---

## Features

- **Fully offline** — LanguageTool runs locally via its CLI JAR; no data leaves the machine
- **System-wide** — works in browsers, editors, terminals, chat apps, any focusable field
- **Zero idle footprint** — the grammar engine only lives during a fix, then exits completely
- **Wayland + X11** — intelligent adapter selection at runtime
- **Cross-distro** — Fedora, Ubuntu, Debian, Arch, Mint, Pop!_OS, openSUSE
- **Placeholder UX** — shows `fixing grammar...` immediately while correction runs
- **Clipboard safe** — original clipboard content is restored after every operation
- **Confidence filtering** — only high-confidence corrections are applied; noisy rules suppressed
- **Custom rules** — bundled XML ruleset covering modal+of errors, common misspellings, and more
- **JVM tuned** — serial GC, bounded heap, tier-1 JIT stop: ~400ms faster startup than defaults
- **Optional server mode** — run LanguageTool as a local HTTP server for sub-200ms latency

---

## Quick Start

```bash
git clone https://github.com/AniruthKarthik/gramfix
cd gramfix
bash install.sh
```

After installation:

1. Select text anywhere on your desktop
2. Press **Alt+G**
3. Watch the text correct itself in place

---

## Architecture

```
gramfix/
├── cmd/
│   ├── gramfix/          # Main binary: grammar fix pipeline + --stdin mode
│   ├── gramfix-hotkey/   # Lightweight hotkey daemon
│   └── gramfix-check/    # Installation diagnostic tool
├── internal/
│   ├── core/             # Orchestrator — coordinates all components
│   ├── grammar/          # LanguageTool engine
│   │   ├── engine.go     # CLI invocation, patching, replacement scoring
│   │   ├── server.go     # Optional HTTP client for local LT server
│   │   ├── confidence.go # Per-rule/category confidence scoring
│   │   ├── normalize.go  # Input normalization before LT
│   │   └── validate.go   # Post-correction safety validation
│   ├── env/              # Runtime environment detection
│   └── log/              # Structured logger
├── adapters/
│   ├── clipboard/        # wl-copy/wl-paste/xclip/xsel abstraction
│   ├── injector/         # ydotool/wtype/xdotool abstraction
│   ├── hotkey/           # sxhkd/xbindkeys daemon abstraction
│   └── platform/         # Distro/package-manager detection
├── configs/
│   ├── gramfix.conf      # All runtime configuration options
│   └── rules/
│       └── gramfix-custom.xml  # Custom LT grammar rules
├── scripts/
│   ├── bench.sh                    # Accuracy benchmark runner
│   ├── gramfix-lt-server.service   # Optional LT HTTP server (systemd)
│   └── gramfix-hotkey.service.template
├── testdata/
│   ├── corpus.txt        # 45-sentence benchmark corpus
│   └── expected.txt      # Expected corrections (ground truth)
├── docs/                 # Guides and reference
├── install.sh            # Full installer
├── uninstall.sh          # Uninstaller
└── Makefile
```

### Runtime Flow

```
User selects text
        |
   [Alt+G pressed]
        |
   gramfix-hotkey (systemd user service, ~4MB RAM idle)
        |
        +-->  launches: gramfix binary
                  |
                  +- 1.  Detect environment (Wayland/X11, tools)
                  +- 2.  Read primary selection  (wl-paste --primary)
                  |        fallback: clipboard (wl-paste)
                  +- 3.  Backup current clipboard
                  +- 4.  Normalize input (CRLF, smart quotes)
                  +- 5.  Write "fixing grammar..." -> clipboard
                  +- 6.  Inject Ctrl+V (ydotool) -> replaces selection
                  +- 7.  Try LT HTTP server (if configured, ~150ms)
                  |        fallback: java -jar LT.jar --json (~2-4s)
                  +- 8.  Confidence-filter matches (threshold: 60/100)
                  +- 9.  Apply corrections (byte-offset, end-to-start)
                  +- 10. Validate correction (length ratio, common prefix)
                  +- 11. Write corrected text -> clipboard
                  +- 12. Inject Ctrl+V -> replaces placeholder
                  +- 13. Restore original clipboard
                  +- 14. Exit completely
```

### Adapter Selection

| Component | Wayland Priority     | X11 Priority      |
|-----------|----------------------|-------------------|
| Clipboard | wl-paste / wl-copy   | xclip -> xsel     |
| Injector  | ydotool -> wtype     | xdotool           |
| Hotkey    | sxhkd                | xbindkeys -> sxhkd |

---

## Installation

### Prerequisites

| Tool         | Purpose                 | Package name               |
|--------------|-------------------------|----------------------------|
| `java`       | Run LanguageTool JAR    | `java-21-openjdk-headless` |
| LanguageTool | Grammar correction      | `languagetool`             |
| `wl-copy`    | Wayland clipboard       | `wl-clipboard`             |
| `ydotool`    | Key injection (Wayland) | `ydotool`                  |
| `sxhkd`      | Hotkey daemon           | `sxhkd`                    |
| `xclip`      | X11 clipboard           | `xclip`                    |
| `xdotool`    | X11 key injection       | `xdotool`                  |

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

Or manually: **System Settings -> Shortcuts -> Custom Shortcuts -> New Command/URL**
- Name: `GramFix`
- Trigger: `Alt+G`
- Command: `~/.local/bin/gramfix`

---

## Usage

### Via Hotkey (normal use)

1. Highlight text anywhere
2. Press **Alt+G**
3. Text is corrected in place

### Via Command Line

```bash
# Fix grammar using current primary selection or clipboard
gramfix

# Read from stdin, print corrected text to stdout (useful for scripts)
echo "I could of done it." | gramfix --stdin

# Specify language
gramfix --lang en-GB

# Use n-gram language model for confusion-pair disambiguation (their/there, affect/effect)
gramfix --ngram-dir ~/.local/share/gramfix/ngrams

# Use local LT HTTP server for fast correction (~150ms vs ~3s cold JVM)
gramfix --server http://localhost:8081

# Adjust confidence threshold (default 60; lower = more corrections, higher = fewer)
gramfix --confidence 50

# Debug mode
gramfix --debug

# Log to file (~/.local/share/gramfix/gramfix.log)
gramfix --log
```

### All Configuration Options

All flags can also be set via environment variables or `configs/gramfix.conf`:

| Flag                  | Env var                       | Default    | Description                              |
|-----------------------|-------------------------------|------------|------------------------------------------|
| `--lang`              | `GRAMFIX_LANG`                | `en-US`    | LanguageTool language code               |
| `--stdin`             | —                             | off        | Read from stdin, write corrected to stdout |
| `--ngram-dir`         | `GRAMFIX_NGRAM_DIR`           | (none)     | Path to LT n-gram data directory         |
| `--rules`             | `GRAMFIX_CUSTOM_RULES`        | bundled    | Path to custom LT XML rules file         |
| `--server`            | `GRAMFIX_LT_SERVER_URL`       | (none)     | Local LT HTTP server URL                 |
| `--disabled-rules`    | `GRAMFIX_DISABLED_RULES`      | (defaults) | Comma-separated LT rule IDs to suppress  |
| `--enabled-categories`| `GRAMFIX_ENABLED_CATEGORIES`  | (defaults) | Comma-separated LT category IDs          |
| `--confidence`        | `GRAMFIX_CONFIDENCE`          | `60`       | Minimum match confidence (0-100)         |
| `--jvm-heap`          | `GRAMFIX_JVM_XMX`             | `256m`     | JVM max heap for LT CLI                  |
| `--debug`             | `GRAMFIX_DEBUG`               | off        | Enable debug logging                     |
| `--log`               | `GRAMFIX_LOG_FILE`            | off        | Write logs to file                       |

### Manage the Hotkey Daemon

```bash
systemctl --user status gramfix-hotkey     # check status
systemctl --user restart gramfix-hotkey    # restart
systemctl --user stop gramfix-hotkey       # stop
journalctl --user -u gramfix-hotkey -f     # follow logs
```

### Optional: Local LT HTTP Server

For sub-200ms correction latency (vs ~3s cold JVM), run LanguageTool as a persistent local server:

```bash
make lt-server           # install and enable gramfix-lt-server.service
```

Then set in `configs/gramfix.conf`:
```ini
GRAMFIX_LT_SERVER_URL=http://localhost:8081
```

The server idles at ~120MB RAM. GramFix falls back to the CLI JAR automatically if the server is not running.

### Optional: N-gram Language Model

For improved disambiguation of confusion pairs (`their/there`, `affect/effect`, `its/it's`):

```bash
# Download n-gram data (~1.6 GB compressed, ~8 GB extracted)
# See: https://dev.languagetool.org/finding-errors-using-n-gram-data
mkdir -p ~/.local/share/gramfix/ngrams
# Download ngrams-en-20150817.zip from languagetool.org and extract here
```

Then set:
```ini
GRAMFIX_NGRAM_DIR=~/.local/share/gramfix/ngrams
```

---

## Accuracy

GramFix maximizes LanguageTool's offline accuracy through several layers:

- **Custom XML rules** (`configs/rules/gramfix-custom.xml`): modal+of errors (`could of` -> `could have`), common misspellings (`definately`, `recieve`, `alot`, `seperate`, `untill`, `irregardless`)
- **Category tuning**: enables `TYPOS`, `GRAMMAR`, `CONFUSED_WORDS`, `PUNCTUATION`, `COMPOUNDING`; suppresses noisy rules (`UPPERCASE_SENTENCE_START`, `EN_QUOTES`, `DASH_RULE`) that produce false positives on technical writing
- **Confidence filtering**: matches below the configured threshold (default: 60/100) are silently skipped
- **Case-preserving replacements**: `THEIR` -> `THEY'RE`, `Title` -> `Title`
- **Overlap resolution**: when two matches overlap, the one with higher confidence and longer span wins
- **Byte-offset patching**: corrections applied on raw UTF-8 bytes to avoid position drift with non-ASCII characters
- **Post-correction validation**: output is rejected if it shrinks below 50% or grows above 200% of the original length

### Running the Benchmark

```bash
make bench
```

Runs gramfix against `testdata/corpus.txt` (45 representative sentences) and reports precision, recall, exact matches, and missed/false-positive errors.

---

## Supported Languages

Any language supported by LanguageTool. Common codes:

| Code    | Language        |
|---------|-----------------|
| `en-US` | English (US)    |
| `en-GB` | English (UK)    |
| `de-DE` | German          |
| `fr`    | French          |
| `es`    | Spanish         |
| `pt-BR` | Portuguese (BR) |

Pass `--lang CODE` to gramfix or set `GRAMFIX_LANG` in `configs/gramfix.conf`.

---

## Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md) for detailed diagnostics.

Quick checks:

```bash
gramfix-check              # installation diagnostic
gramfix --debug            # show what's happening step by step
```

### LanguageTool Not Found

```bash
ls /usr/share/languagetool/languagetool-commandline.jar
# Override path:
export GRAMFIX_LT_JAR=/path/to/languagetool-commandline.jar
gramfix
```

### High Latency

JVM startup takes ~2-4s. Options:
- Enable server mode (`make lt-server`) for ~150ms latency
- JVM is already tuned: `-Xms64m -Xmx256m -XX:+UseSerialGC -XX:TieredStopAtLevel=1`
- Nothing can eliminate JVM startup without a persistent process

---

## Building from Source

```bash
git clone https://github.com/anilnair00/gramfix
cd gramfix

make build      # -> build/gramfix  build/gramfix-hotkey  build/gramfix-check
make test       # run unit tests
make vet        # go vet
make fmt        # gofmt
make bench      # accuracy benchmark against testdata/corpus.txt

# Optional services
make lt-server       # install local LT HTTP server (optional, fast mode)
make lt-server-stop  # remove it

# Integration tests
bash scripts/test-integration.sh
```

---

## Distro Compatibility

| Distro        | Status    | Notes                        |
|---------------|-----------|------------------------------|
| Fedora 40+    | Tested    | All tools available via dnf  |
| Ubuntu 22.04+ | Supported | `languagetool` in universe   |
| Debian 12+    | Supported | `languagetool` in main       |
| Arch Linux    | Supported | `languagetool` in community  |
| Linux Mint    | Supported | Ubuntu-based, same packages  |
| Pop!_OS       | Supported | Ubuntu-based                 |
| openSUSE Leap | Supported | `languagetool` via zypper    |

---

## Limitations

- **JVM startup latency**: ~2-4 seconds per invocation in CLI mode. Use server mode (`make lt-server`) to bring this down to ~150ms.
- **wtype on Wayland**: requires compositor support for `zwp_virtual_keyboard_v1`. ydotool is preferred as it bypasses the compositor via `/dev/uinput`.
- **Terminal paste**: Ctrl+V in some terminals pastes literal text. Use Ctrl+Shift+V in those cases. The gramfix-hotkey daemon has a direct-type fallback.
- **ydotoold permissions**: `ydotoold` must run as root, or the user must be in the `input` group to write `/dev/uinput`.
- **No LLMs**: GramFix is intentionally rule-based and statistical. It will not rephrase, paraphrase, or rewrite text — only correct grammar and spelling.

---

## License

MIT License — see [LICENSE](LICENSE)
