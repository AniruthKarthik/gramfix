# Gramfix: Project Specification & Architecture

This document serves as the foundational mandate and technical blueprint for `gramfix`. Any agent or developer should be able to reconstruct the entire project based on these requirements.

## 1. Project Objective
`gramfix` is a lightweight, cross-platform (Linux focus) grammar correction utility. It operates globally across the OS via a hotkey, capturing selected text, correcting it offline using LanguageTool, and replacing the original text with a visual feedback mechanism.

## 2. Core Functional Requirements
- **Global Activation**: Triggered by a system-level hotkey (default: `Alt+G`).
- **Text Capture**: 
    1. Try capturing text from the "Primary Selection" (middle-click buffer).
    2. Fallback: Trigger `Ctrl+C` to copy text to the clipboard.
- **Visual Feedback (Two-Stage Replacement)**:
    - **Stage 1**: Immediately replace the selected text with the placeholder: `fixing grammar...`.
    - **Stage 2**: Once correction is complete, backspace the placeholder and inject the final corrected text.
- **Offline Correction**: Use a local `LanguageTool` Java instance to process text without external API calls.
- **Clipboard Preservation**: If the clipboard was used for capture/pasting, restore the user's original clipboard content after the operation finishes.

## 3. Technical Architecture (Linux/Wayland/X11)
The application must dynamically detect the environment and choose the appropriate adapters.

### A. Input Injection (The "Injector")
Critical for bypassing Wayland security restrictions.
- **Priority 1: `ydotool`**: Must support a local daemon (`ydotoold`). The app must search for the socket at `/tmp/.ydotool_socket` or `/run/user/$UID/.ydotool_socket`. Used for `Ctrl+C`, `Ctrl+V`, and sending `Backspaces`.
- **Priority 2: `wtype`**: Native Wayland virtual keyboard tool.
- **Priority 3: `xdotool`**: Fallback for X11 sessions.

### B. Clipboard Management
- **Wayland**: Use `wl-copy` and `wl-paste`.
- **X11**: Use `xclip` or `xsel`.

### C. Grammar Engine
- Use `LanguageTool` (CLI or JAR).
- Implementation must parse JSON output from LT to handle multiple/overlapping corrections accurately.

## 4. Implementation Details (Go)
- **Concurrency/Timing**: 
    - Include a 500ms delay after hotkey trigger to prevent modifier keys (Alt) from interfering with injected keys (Ctrl+V).
    - Use `time.Sleep` between clipboard sets and paste injections to ensure system sync.
- **Logging**: Maintain a persistent log at `/tmp/gramfix.log` with high-resolution timestamps for debugging shortcut trigger failures.

## 5. System Integration (KDE Plasma 6 Focus)
- **Shortcut Registration**: Shortcuts must be registered in `~/.config/kglobalshortcutsrc` under the `[services]` group, pointing to a `.desktop` file or the binary path directly.
- **Desktop Entry**: A `gramfix.desktop` file should exist in `~/.local/share/applications/` to allow the OS to treat the binary as a recognized application.

## 6. Dependency List
- `go` (v1.20+)
- `languagetool` (java-based)
- `wl-clipboard` (Wayland)
- `xdotool` / `xclip` (X11)
- `ydotool` (Virtual input - requires `ydotoold` running)
- `wtype` (Alternative Wayland input)

## 7. Execution Workflow (The "Loop")
1. **Hotkey Pressed** (`Alt+G`) -> System executes `/usr/local/bin/gramfix`.
2. **Log Start** -> Capture Environment info.
3. **Capture** -> `GetPrimary()` OR `SendCopy()` -> `GetClipboard()`.
4. **Placeholder** -> `SetClipboard("fixing grammar...")` -> `SendPaste()`.
5. **Process** -> `LanguageTool.Correct(text)`.
6. **Final Replace** -> `SendBackspaces(17)` -> `SetClipboard(corrected)` -> `SendPaste()`.
7. **Cleanup** -> Restore `OldClip`.
