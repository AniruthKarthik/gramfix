# GramFix Architecture

## Overview

GramFix is a cross-platform Linux desktop utility that fixes grammar in selected text using a local LanguageTool installation. It is designed around a modular adapter pattern that cleanly separates environment detection from tool implementation.

## Component Map

```
┌─────────────────────────────────────────────────────────────┐
│                    gramfix-hotkey (daemon)                   │
│   ~4MB RAM idle · systemd user service · sxhkd/xbindkeys    │
└──────────────────────┬──────────────────────────────────────┘
                       │  launches on Alt+G
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                      gramfix (binary)                        │
│                                                             │
│  ┌──────────┐   ┌───────────┐   ┌───────────┐             │
│  │   env/   │──▶│   core/   │──▶│  grammar/ │             │
│  │ detect   │   │orchestrate│   │LanguageTool│            │
│  └──────────┘   └─────┬─────┘   └───────────┘             │
│                        │                                    │
│             ┌──────────┴──────────┐                        │
│             ▼                     ▼                        │
│      ┌────────────┐      ┌──────────────┐                 │
│      │ clipboard/ │      │  injector/   │                 │
│      │ wl-copy    │      │  ydotool     │                 │
│      │ xclip/xsel │      │  wtype       │                 │
│      └────────────┘      │  xdotool     │                 │
│                          └──────────────┘                 │
└─────────────────────────────────────────────────────────────┘
```

## Package Responsibilities

### `internal/env`
- Detects Wayland vs X11 from environment variables
- Probes availability of all required tools
- Detects distro from `/etc/os-release`
- Returns ordered fallback chains for clipboard/injector tools

### `internal/core`
- Orchestrates the full pipeline
- Manages clipboard state (backup, restore)
- Coordinates timing between injection steps
- Handles error recovery (restore original text on failure)

### `internal/grammar`
- Wraps LanguageTool's command-line JAR
- Runs in JSON output mode for reliable parsing
- Applies corrections in reverse-offset order (preserves string positions)
- Handles JVM startup timeout, encoding, and multiline text

### `internal/log`
- Lightweight structured logger (no dependencies)
- Writes to stderr + optional file (`~/.local/share/gramfix/gramfix.log`)

### `adapters/clipboard`
- Abstracts wl-copy/wl-paste (Wayland) and xclip/xsel (X11)
- Handles Wayland's "clipboard server" model: `wl-copy` runs as background process
- Tracks and kills intermediate wl-copy processes to prevent orphans
- Preserves final clipboard restore wl-copy (intentional, normal behavior)

### `adapters/injector`
- Abstracts text injection and key simulation
- Priority: ydotool (uinput) → wtype (virtual keyboard) → xdotool (X11)
- ydotool is preferred because it works without compositor support
- Sets `YDOTOOL_SOCKET` automatically from detected socket path

### `adapters/hotkey`
- Writes temporary config files for sxhkd/xbindkeys
- Cleans up temp files on exit
- Generates systemd user service unit file

### `adapters/platform`
- Distro-to-package-manager mapping
- Dependency validation helpers

## Key Design Decisions

### Why ydotool over wtype?
`wtype` requires `zwp_virtual_keyboard_v1` Wayland protocol, which is not supported by all compositors or when running from a service context. `ydotool` uses the kernel's `/dev/uinput` interface, bypassing the Wayland compositor entirely — more reliable for system-wide hotkey use.

### Why clipboard + Ctrl+V over direct typing?
Direct typing with `wtype`/`ydotool type` is character-by-character and can be slow or miss special characters. Using clipboard + Ctrl+V is the standard "paste" action that all applications understand natively, including rich text editors, web browsers, and IDEs.

### Why keep wl-copy alive?
On Wayland, the clipboard model requires the "clipboard owner" (wl-copy) to remain running to serve paste requests from other applications. When gramfix pastes the corrected text, the wl-copy serving it can be killed (the text is now in the focused field). The clipboard-restore wl-copy is intentionally left running so the user's original clipboard remains accessible.

### Why reverse-offset correction?
LanguageTool returns corrections with byte offsets into the original text. Applying them forward would shift subsequent offsets. Sorting by descending offset and applying from end-to-start guarantees offset accuracy.

## Process Lifecycle

```
gramfix-hotkey (always running, ~4MB)
│
└── [Alt+G] → fork gramfix
                │
                ├── Read clipboard + primary selection
                ├── Write placeholder → wl-copy (bg) → Ctrl+V → kill wl-copy
                ├── Run LanguageTool JAR (java process, ~300MB, 3-5s)
                ├── Write corrected → wl-copy (bg) → Ctrl+V → kill wl-copy
                ├── Write original → wl-copy (bg) [stays alive: user's clipboard]
                └── exit(0) — total runtime: ~4-6s
```

After gramfix exits:
- No grammar engine running
- No temp files (cleaned up)
- One wl-copy process per clipboard buffer (normal system behavior)
- Zero orphan processes from gramfix
