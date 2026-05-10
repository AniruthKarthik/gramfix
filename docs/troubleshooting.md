# GramFix Troubleshooting Guide

## Quick Diagnostic

```bash
# Run in debug mode to see what's happening
gramfix --debug

# Check hotkey daemon status
systemctl --user status gramfix-hotkey

# Check ydotoold
ls /tmp/.ydotool_socket && echo "ydotool socket OK"

# Check LanguageTool
ls /usr/share/languagetool/languagetool-commandline.jar && echo "LanguageTool OK"

# Check java
java -version
```

---

## Issue: Alt+G does nothing

**Step 1**: Is the hotkey daemon running?
```bash
systemctl --user status gramfix-hotkey
# If not running:
systemctl --user start gramfix-hotkey
```

**Step 2**: Is sxhkd active?
```bash
ps aux | grep sxhkd
```

**Step 3**: On KDE Plasma — add a Custom Shortcut (most reliable):
- System Settings → Shortcuts → Custom Shortcuts
- New → Command/URL
- Trigger: Alt+G
- Command: `~/.local/bin/gramfix`

**Step 4**: On GNOME:
- Settings → Keyboard → Keyboard Shortcuts → Custom Shortcuts
- Add shortcut: Command `gramfix`, Key `Alt+G`

**Step 5**: Test manually:
```bash
echo -n "this is a test" | wl-copy --primary
gramfix --debug
```

---

## Issue: "Compositor does not support the virtual keyboard protocol"

`wtype` is failing. This is common on some Wayland compositors.

**Fix**: Use ydotool instead (already preferred in gramfix, but ensure ydotoold is running):

```bash
# Check ydotoold
ls /tmp/.ydotool_socket

# If socket missing, start ydotoold
sudo ydotoold &
# Or as a user service (if packaged):
systemctl --user start ydotoold
```

If ydotoold is not available, run as root:
```bash
sudo ydotoold --socket /tmp/.ydotool_socket &
```

---

## Issue: LanguageTool not found

```bash
# Check standard paths
ls /usr/share/languagetool/languagetool-commandline.jar 2>/dev/null || \
ls /usr/local/share/languagetool/languagetool-commandline.jar 2>/dev/null || \
echo "Not found"

# Install via package manager
sudo dnf install languagetool          # Fedora
sudo apt install languagetool          # Ubuntu/Debian
sudo pacman -S languagetool            # Arch
sudo zypper install languagetool       # openSUSE

# Override path
export GRAMFIX_LT_JAR=/path/to/languagetool-commandline.jar
gramfix
```

---

## Issue: Text not replaced (paste not working)

**Symptom**: gramfix runs, grammar is corrected, but the original text is not replaced.

**Possible causes**:

1. **ydotool Ctrl+V not reaching the focused window**
   - The window may have lost focus during the ~4s grammar correction
   - Click in the target field, then try again

2. **Application uses Ctrl+Shift+V for paste** (some terminals)
   - For terminals: select text, run `gramfix`, it should work via Ctrl+Shift+V fallback
   - Alternatively, use a text editor instead of terminal

3. **Rich text app ignoring paste** (some Electron apps)
   - Try copying text, running `gramfix`, then pasting manually

---

## Issue: Clipboard not restored

After gramfix runs, clipboard should be restored to its pre-correction state.

**Debug**:
```bash
echo -n "my original clipboard" | wl-copy
echo -n "text to fix" | wl-copy --primary
gramfix --debug
wl-paste   # should show "my original clipboard"
```

If not restored, check logs:
```bash
gramfix --debug --log
tail ~/.local/share/gramfix/gramfix.log
```

---

## Issue: "no text selected, nothing to do"

gramfix found no text in the primary selection or clipboard.

**Fix**:
1. Make sure text is **highlighted** (not just cursor positioned in field)
2. On Wayland, highlight with mouse (this writes to primary selection)
3. On X11, highlighted text is automatically in primary selection
4. Alternatively, Ctrl+C the text first (clipboard fallback)

---

## Issue: High latency (takes more than 10 seconds)

LanguageTool has JVM startup overhead (~3-5s normally). Longer times indicate:

1. **System under memory pressure** — JVM startup is slower with low RAM
2. **Very long text** — LanguageTool processes the full text; limit to paragraphs
3. **Cold JVM** — first run is always slower; subsequent runs use JVM class cache

**There is no way to eliminate JVM startup latency** without running a persistent server (which violates the no-persistent-process design). This is a known trade-off.

---

## Issue: Service fails to start after reboot

```bash
# Check service file
cat ~/.config/systemd/user/gramfix-hotkey.service

# Verify binary paths exist
ls ~/.local/bin/gramfix
ls ~/.local/bin/gramfix-hotkey

# Reload and restart
systemctl --user daemon-reload
systemctl --user restart gramfix-hotkey
```

---

## Debug Log Location

```bash
# Enable logging
gramfix --log --debug

# View logs
tail -f ~/.local/share/gramfix/gramfix.log

# Service logs
journalctl --user -u gramfix-hotkey -f
```

---

## Reinstall from Scratch

```bash
bash uninstall.sh
bash install.sh
```
