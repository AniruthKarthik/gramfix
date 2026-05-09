# Project Requirements: Gramfix

This document provides a detailed blueprint and set of instructions for an AI agent to build and maintain the **Gramfix** project.

## 1. Project Overview
**Gramfix** is a global, offline grammar correction utility for Linux. It allows users to highlight text in any application, press a hotkey (Alt+G), and have that text automatically corrected. 

### User-Level Workflow (The Goal)
1.  **User Action:** Highlights a sentence with a typo (e.g., "This are wrong").
2.  **User Action:** Presses **Alt+G**.
3.  **Visual Feedback (Immediate):** The highlighted text is instantly replaced by the string `fixing grammar...`.
4.  **Processing:** The application runs a local grammar check (LanguageTool).
5.  **Final Result (Automatic):** The string `fixing grammar...` is automatically backspaced and replaced with the corrected text (e.g., "This is wrong").
6.  **Clipboard Safety:** The user's original clipboard content is preserved and restored after the operation.

---

## 2. Technical Architecture & Building Instructions

### Language & Framework
- **Language:** Go (Golang).
- **Architecture:** Adapter-based design to handle different Linux display servers (Wayland vs. X11).

### Core Components to Implement

#### A. The Orchestrator (`internal/core/orchestrator.go`)
This is the main logic loop. It must:
1.  Wait ~500ms after start to avoid "modifier key collision" (where the user still holding Alt interferes with the app's Ctrl+V).
2.  **Capture Phase:**
    - Try `GetPrimary()` (Selection buffer).
    - If empty, trigger `SendCopy()` (Ctrl+C) and `GetClipboard()`.
3.  **Placeholder Phase:**
    - `SetClipboard("fixing grammar...")`.
    - `SendPaste()` (Ctrl+V).
4.  **Correction Phase:**
    - Call the Grammar Engine.
5.  **Replacement Phase:**
    - `SendBackspaces(17)` (the length of "fixing grammar...").
    - `SetClipboard(correctedText)`.
    - `SendPaste()`.
6.  **Restoration Phase:**
    - Restore original clipboard.

#### B. The Adapters (`internal/adapters/`)
1.  **Clipboard Adapter:**
    - **Wayland:** Use `wl-copy` and `wl-paste`.
    - **X11:** Use `xclip` or `xsel`.
2.  **Injector Adapter (Crucial for Wayland):**
    - **Priority 1: `ydotool`**. **Must** look for the socket at `/tmp/.ydotool_socket` or `/run/user/$UID/.ydotool_socket`. Set the `YDOTOOL_SOCKET` env var accordingly.
    - **Priority 2: `wtype`**.
    - **Priority 3: `xdotool`**.
3.  **Grammar Engine:**
    - Must use local `LanguageTool` (via Java JAR).
    - Must parse JSON output to accurately apply multiple corrections to a string.

---

## 3. Environment & Integration (System Level)

### Target Environment
- Primary focus: **KDE Plasma 6 on Wayland (Fedora/Arch/Debian)**.
- Secondary focus: GNOME and X11 sessions.

### Installation Requirements
- **Binary Path:** `/usr/local/bin/gramfix`.
- **Desktop File:** `~/.local/share/applications/gramfix.desktop`. 
    - *Note:* Do NOT set the executable bit on the `.desktop` file itself in Plasma 6, as it causes "Exec format error". Point the shortcut to the binary path.
- **Dependencies:** `java`, `languagetool`, `wl-clipboard`, `ydotool` (with `ydotoold` running).

### Shortcut Setup (Agent Instructions)
To automate KDE shortcut setup, use:
```bash
kwriteconfig6 --file kglobalshortcutsrc --group "services" --group "gramfix.desktop" --key "_launch" "Alt+G,none,Gramfix"
```

---

## 4. Expected Developer/AI Agent Behavior
1.  **Logging:** Always log to `/tmp/gramfix.log`. Use timestamps with milliseconds.
2.  **Error Handling:** If grammar correction fails, the app must gracefully put back the original text rather than leaving the placeholder.
3.  **Timing:** Use generous `time.Sleep` calls (200ms - 500ms) between input injections. Wayland is sensitive to rapid-fire synthetic inputs.
4.  **Verification:** Test by simulating text selection and running the binary manually before confirming the shortcut works.

## 5. User-Level Expectations
- The app should be **silent**. No popups or notifications (unless there is a fatal error).
- The transition from "fixing grammar..." to the result should feel "fluid" and robotic (auto-typed).
- It must work in terminal emulators, browsers, and text editors alike.
