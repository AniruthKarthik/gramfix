# GramFix

> Fix grammar anywhere on your Linux desktop — instantly, with a single hotkey. Now powered by Groq for lightning-fast cloud corrections with a local offline fallback.

Select text in any application, press **Alt+G**, and GramFix corrects your grammar. It prioritizes the ultra-fast **Groq API** (Llama 3/4) and automatically falls back to a local [LanguageTool](https://languagetool.org/) engine if offline.

---

## Features

- **Groq-Powered** — lightning-fast cloud corrections using Llama 3.3/4 via Groq LPU.
- **Hybrid Engines** — use Groq API, OpenRouter, or local LanguageTool (fully offline fallback).
- **System-wide** — works in browsers, editors, terminals, chat apps, any focusable field.
- **Zero idle footprint** — the grammar engine only lives during a fix, then exits completely.
- **Audit Logging** — simple, structured history of all corrections.
- **Wayland + X11** — intelligent adapter selection at runtime.
- **Cross-distro** — Fedora, Ubuntu, Debian, Arch, Mint, Pop!_OS, openSUSE.
- **Placeholder UX** — shows `fixing grammar...` immediately while correction runs.
- **Clipboard safe** — original clipboard content is restored after every operation.

---

## Installation

```bash
git clone https://github.com/anilnair00/gramfix
cd gramfix
./install.sh   # Installs system dependencies
make install   # Builds binaries and prompts for GROQ_API_KEY
make systemd   # Enables Alt+G hotkey daemon
```

`make install` will automatically create `~/.config/gramfix/.env` and ask for your Groq API key.

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
│   ├── grammar/          # Grammar engines
│   │   ├── groq.go       # Groq API client (Primary)
│   │   ├── openrouter.go # OpenRouter API client (Fallback)
│   │   ├── engine.go     # Local LanguageTool engine (Offline Fallback)
│   │   └── validate.go   # Post-correction safety validation
│   ├── env/              # Environment detection and .env loading
│   └── log/              # Structured audit logger
```

### Runtime Flow

```
User selects text -> [Alt+G pressed]
        |
   gramfix binary starts
        |
        +- 1. Load configuration from .env (~/.config/gramfix/.env)
        +- 2. Capture selection and show "fixing grammar..."
        +- 3. Try Groq API (llama-3.3-70b-versatile, ~300ms)
        |       fallback: OpenRouter API
        |       fallback: Local LanguageTool
        +- 4. Validate correction (safety checks)
        +- 5. Inject corrected text back into app
        +- 6. Audit log: date, time, sentence, method, corrected
        +- 7. Exit
```

---

## Logging

GramFix maintains a simple audit log of all corrections in `~/.local/share/gramfix/gramfix.log`.

**Format:**
`date, time, original sentence, method, corrected version`

**Example:**
`2026-05-14, 10:15:00, "he are man", Local CLI, "he is man"`
`2026-05-14, 10:16:22, "he dont know", Groq API (llama-3.3-70b-versatile), "He doesn't know"`

---

## Usage

### Via Hotkey (normal use)

1. Highlight text anywhere
2. Press **Alt+G**
3. Text is corrected in place

### Via Command Line

```bash
# Read from stdin, print corrected text to stdout
echo "I could of done it." | gramfix --stdin

# Force a specific Groq model
gramfix --groq-model llama-3.1-8b-instant

# Debug mode (verbose)
gramfix --debug
```

---

## Troubleshooting

```bash
gramfix-check              # installation diagnostic
journalctl --user -u gramfix-hotkey -f  # follow logs
```

---

## License

MIT License — see [LICENSE](LICENSE)
