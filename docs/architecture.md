# GramFix Architecture

## Overview

GramFix is a cross-platform Linux desktop utility that fixes grammar in selected text using a local LanguageTool installation. It is designed around a modular adapter pattern that cleanly separates environment detection from tool implementation, and an ephemeral process model that produces zero idle RAM usage.

## Component Map

```
+-------------------------------------------------------------+
|                   gramfix-hotkey (daemon)                    |
|  ~4MB RAM idle  |  systemd user service  |  sxhkd/xbindkeys |
+------------------------------+------------------------------+
                               | launches on Alt+G
                               v
+-------------------------------------------------------------+
|                      gramfix (binary)                        |
|                                                             |
|  +---------+   +-----------+   +------------------------+  |
|  |  env/   +-->|   core/   +-->|        grammar/        |  |
|  | detect  |   |orchestrate|   |  engine  + server      |  |
|  +---------+   +-----+-----+   |  confidence + validate |  |
|                      |         |  normalize             |  |
|           +----------+------+  +------------------------+  |
|           v                 v                               |
|    +------------+   +--------------+                        |
|    | clipboard/ |   |  injector/   |                        |
|    | wl-copy    |   |  ydotool     |                        |
|    | xclip/xsel |   |  wtype       |                        |
|    +------------+   |  xdotool     |                        |
|                     +--------------+                        |
+-------------------------------------------------------------+
```

## Package Responsibilities

### `internal/env`

- Detects Wayland vs X11 from environment variables
- Probes availability of all required tools (`wl-copy`, `ydotool`, `xclip`, ...)
- Detects distro from `/etc/os-release`
- Returns ordered fallback chains for clipboard/injector tools

### `internal/core`

- Orchestrates the full pipeline (steps 1-14 in the flow diagram)
- Manages clipboard state (backup, restore)
- Coordinates timing between injection steps
- Handles error recovery (restores original text on any failure)

### `internal/grammar`

The grammar package is split into focused files:

| File             | Responsibility                                                   |
|------------------|------------------------------------------------------------------|
| `engine.go`      | `EngineConfig`, CLI invocation, byte-offset patching, JVM flags |
| `server.go`      | HTTP client for optional local LT server; falls back to CLI     |
| `openrouter.go`  | Cloud-enhanced grammar correction via OpenRouter API            |
| `confidence.go`  | Per-category and per-rule confidence scores (0-100)             |
| `normalize.go`   | Input normalization before sending to LT (CRLF, smart quotes)   |
| `validate.go`    | Post-correction sanity check (length ratio, prefix/suffix)      |

**Correction pipeline** (inside `engine.go::applyCorrections`):

1. Parse LT JSON output
2. Filter matches below `ConfidenceMin` threshold
3. Sort by confidence desc, then span length desc, then offset desc
4. Re-sort by offset descending for end-to-start patching
5. Skip overlapping matches (lower-priority match loses)
6. Apply byte-offset patches to UTF-8 byte slice
7. Adjust replacement capitalisation to match original

### `internal/log`

- Lightweight structured logger (no external dependencies)
- Writes to stderr + optional file (`~/.local/share/gramfix/gramfix.log`)

### `adapters/clipboard`

- Abstracts `wl-copy`/`wl-paste` (Wayland) and `xclip`/`xsel` (X11)
- Handles Wayland's clipboard-server model: `wl-copy` runs as a background process
- Tracks and kills intermediate `wl-copy` processes to prevent orphans
- Preserves the final clipboard-restore `wl-copy` (intentional: serves user's clipboard)

### `adapters/injector`

- Abstracts text injection and key simulation
- Priority: ydotool (uinput) -> wtype (virtual keyboard) -> xdotool (X11)
- ydotool is preferred: it bypasses the compositor entirely via `/dev/uinput`
- Sets `YDOTOOL_SOCKET` automatically from the detected socket path

### `adapters/hotkey`

- Writes temporary config files for sxhkd/xbindkeys
- Cleans up temp files on exit
- Generates the systemd user service unit file

### `adapters/platform`

- Distro-to-package-manager mapping
- Dependency validation helpers

## Configuration

`EngineConfig` (defined in `internal/grammar/engine.go`) is the single source of truth for all engine parameters. It is populated from:

1. `DefaultEngineConfig()` (hardcoded production-safe defaults)
2. Environment variables (`GRAMFIX_*`) read in `cmd/gramfix/main.go`
3. CLI flags (same names, take precedence over env vars)

Key defaults:

| Field               | Default                                         |
|---------------------|-------------------------------------------------|
| `Lang`              | `en-US`                                         |
| `JVMMaxHeap`        | `256m`                                          |
| `ConfidenceMin`     | `60`                                            |
| `DisabledRules`     | UPPERCASE_SENTENCE_START, EN_QUOTES, DASH_RULE, ... |
| `EnabledCategories` | TYPOS, GRAMMAR, CONFUSED_WORDS, PUNCTUATION, ... |

## Key Design Decisions

### Why ydotool over wtype?

`wtype` requires the `zwp_virtual_keyboard_v1` Wayland protocol, which is not supported by all compositors or when running from a service context. `ydotool` uses the kernel's `/dev/uinput` interface, bypassing the Wayland compositor entirely — more reliable for system-wide hotkey use.

### Why clipboard + Ctrl+V over direct typing?

Direct typing with `wtype`/`ydotool type` is character-by-character and can be slow or miss special characters. Using clipboard + Ctrl+V is the standard paste action that all applications understand natively, including rich text editors, web browsers, and IDEs.

### Why keep wl-copy alive?

On Wayland, the clipboard model requires the "clipboard owner" (`wl-copy`) to remain running to serve paste requests. When gramfix pastes the corrected text, the intermediate `wl-copy` is killed (the text is now in the focused field). The clipboard-restore `wl-copy` is intentionally left running so the user's original clipboard remains accessible.

### Why byte-offset patching?

LanguageTool 6.x returns `offset` and `length` values as byte positions in the UTF-8 input file. Patching on `[]byte` is correct for all inputs including multi-byte characters (accented letters, smart quotes, em-dashes). Prior rune-based patching diverged for supplementary-plane characters.

### Why confidence filtering?

LanguageTool's full ruleset includes style preferences, typographic choices, and low-recall heuristics that produce false positives on technical writing. Assigning confidence scores (0-100) per category/rule lets users tune the precision-recall trade-off without editing XML. The default threshold (60) suppresses STYLE and TYPOGRAPHY while keeping TYPOS and GRAMMAR active.

### Why an ephemeral process model?

Zero idle RAM. The grammar engine (JVM + LT) consumes 200-300MB while running. Keeping it alive between corrections would permanently consume that memory. Since corrections are infrequent (< once per minute for typical users), the ~3s JVM startup is an acceptable trade-off. Users who need faster response can opt into the local LT HTTP server (`make lt-server`).

## Process Lifecycle

```
gramfix-hotkey (always running, ~4MB)
|
+-- [Alt+G] -> fork gramfix
                |
                +-- Read clipboard + primary selection
                +-- Normalize input (CRLF, smart quotes, NBSP)
                +-- Write placeholder -> wl-copy (bg) -> Ctrl+V -> kill wl-copy
                +-- Try OpenRouter API (if OPENROUTER_API_KEY is set, ~1-2s)
                |     fallback: local LanguageTool
                +-- Try LT HTTP server (if GRAMFIX_LT_SERVER_URL is set, ~150ms)
                |     fallback: java -jar LT.jar (JVM, ~200-300MB, 2-4s)
                +-- Filter by confidence, resolve overlaps, patch bytes
                +-- Validate correction (length ratio guard)
                +-- Write corrected -> wl-copy (bg) -> Ctrl+V -> kill wl-copy
                +-- Write original -> wl-copy (bg) [stays alive: user's clipboard]
                +-- exit(0)  -- total runtime: ~3-5s CLI, ~1-2s OpenRouter, ~0.5s server mode
```

After gramfix exits:
- No grammar engine running
- No temp files (cleaned up via `defer os.Remove`)
- One wl-copy process serving the restored clipboard (normal system behavior)
- Zero orphan processes from gramfix

## File Layout Reference

```
configs/
  gramfix.conf                  All runtime configuration keys with documentation
  rules/
    gramfix-custom.xml          Custom LT rules (modal+of, misspellings, etc.)

testdata/
  corpus.txt                    45-sentence benchmark input (one sentence per line)
  expected.txt                  Expected corrected output (parallel to corpus.txt)

scripts/
  bench.sh                      Accuracy benchmark: runs gramfix --stdin per sentence,
                                computes precision/recall against expected.txt
  gramfix-lt-server.service     Systemd user unit for optional local LT HTTP server
  gramfix-hotkey.service.template
  setup-kde-shortcut.sh
  test-integration.sh
```
