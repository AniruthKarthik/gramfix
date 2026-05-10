// Package injector provides cross-platform text injection adapters.
// It types text and sends key combos into the currently focused window.
// Supports: ydotool (Wayland+X11 via uinput), wtype (Wayland), xdotool (X11).
package injector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/anilnair00/gramfix/internal/env"
	"github.com/anilnair00/gramfix/internal/log"
)

const injectTimeout = 15 * time.Second

// ydotoolSocket is the default daemon socket path.
const ydotoolSocket = "/tmp/.ydotool_socket"

// Linux key codes for modifier release (from input-event-codes.h)
// We release all modifiers before injecting Ctrl+V so the hotkey's Alt
// doesn't interfere (Alt+Ctrl+V ≠ Ctrl+V).
const (
	keyCodeLeftCtrl  = 29
	keyCodeV         = 47
	keyCodeLeftAlt   = 56
	keyCodeRightAlt  = 100
	keyCodeLeftShift = 42
	keyCodeRightShift = 54
	keyCodeLeftMeta  = 125
	keyCodeRightMeta = 126
)

// Injector types text into the currently focused window.
type Injector struct {
	session *env.Session
}

// New returns a new Injector.
func New(s *env.Session) *Injector {
	return &Injector{session: s}
}

// bestTool returns the best available injection tool name.
// Priority: ydotool (works everywhere via uinput) → wtype (Wayland) → xdotool (X11)
func (inj *Injector) bestTool() (string, error) {
	if inj.session.HasYdotool && ydotoolAvailable() {
		return "ydotool", nil
	}
	if inj.session.IsWayland && inj.session.HasWtype {
		return "wtype", nil
	}
	if inj.session.HasXdotool {
		return "xdotool", nil
	}
	return "", fmt.Errorf("no text injector available; install ydotool, wtype, or xdotool")
}

func ydotoolAvailable() bool {
	sock := ydotoolSocketPath()
	if sock == "" {
		return false
	}
	_, err := os.Stat(sock)
	return err == nil
}

func ydotoolSocketPath() string {
	if s := os.Getenv("YDOTOOL_SOCKET"); s != "" {
		return s
	}
	if _, err := os.Stat(ydotoolSocket); err == nil {
		return ydotoolSocket
	}
	return ""
}

// ReleaseModifiers releases all modifier keys before injection.
// This is critical: when triggered via Alt+G hotkey, the Alt key is still
// logically pressed. Without release, Ctrl+V becomes Alt+Ctrl+V (wrong).
func (inj *Injector) ReleaseModifiers() {
	tool, err := inj.bestTool()
	if err != nil {
		return
	}

	switch tool {
	case "ydotool":
		// Release all common modifiers via raw key codes
		// Format: keycode:0 means key released
		releaseKeys := fmt.Sprintf("%d:0 %d:0 %d:0 %d:0 %d:0 %d:0 %d:0 %d:0",
			keyCodeLeftAlt, keyCodeRightAlt,
			keyCodeLeftShift, keyCodeRightShift,
			keyCodeLeftCtrl,
			keyCodeLeftMeta, keyCodeRightMeta,
			keyCodeV, // ensure V is released too (from Alt+G → V key can stick)
		)
		args := []string{"key"}
		for _, k := range splitKeys(releaseKeys) {
			args = append(args, k)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "ydotool", args...)
		setYdotoolEnv(cmd)
		_ = cmd.Run()
		time.Sleep(50 * time.Millisecond)

	case "wtype":
		// wtype: release all modifiers
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "wtype", "-m", "alt", "-m", "shift", "-m", "ctrl", "-m", "super").Run()
		time.Sleep(50 * time.Millisecond)

	case "xdotool":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "xdotool", "keyup", "alt", "shift", "ctrl", "super").Run()
		time.Sleep(50 * time.Millisecond)
	}
}

// TypeText types text into the focused field, replacing any selection.
func (inj *Injector) TypeText(text string) error {
	tool, err := inj.bestTool()
	if err != nil {
		return err
	}
	return inj.runType(tool, text)
}

// PasteFromClipboard sends Ctrl+V to paste clipboard content.
// It first releases any held modifier keys (Alt from hotkey trigger).
func (inj *Injector) PasteFromClipboard() error {
	tool, err := inj.bestTool()
	if err != nil {
		return err
	}
	return inj.runPaste(tool)
}

// Backspace sends the backspace key 'count' times to delete text.
func (inj *Injector) Backspace(count int) error {
	if count <= 0 {
		return nil
	}
	tool, err := inj.bestTool()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), injectTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch tool {
	case "ydotool":
		args := []string{"key", "--key-delay=12"}
		for i := 0; i < count; i++ {
			args = append(args, "14:1", "14:0")
		}
		cmd = exec.CommandContext(ctx, "ydotool", args...)
		setYdotoolEnv(cmd)
	case "wtype":
		args := []string{}
		for i := 0; i < count; i++ {
			args = append(args, "-k", "BackSpace")
		}
		cmd = exec.CommandContext(ctx, "wtype", args...)
	case "xdotool":
		cmd = exec.CommandContext(ctx, "xdotool", "key", "--repeat", fmt.Sprintf("%d", count), "--delay", "12", "BackSpace")
	default:
		return fmt.Errorf("unknown tool: %s", tool)
	}

	log.Debug("inject backspace via %s (%d times)", tool, count)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inject backspace (%s): %w\noutput: %s", tool, err, string(out))
	}
	return nil
}

// runType executes the text injection command.
func (inj *Injector) runType(tool, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), injectTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch tool {
	case "ydotool":
		cmd = exec.CommandContext(ctx, "ydotool", "type", "--key-delay=12", "--", text)
		setYdotoolEnv(cmd)
	case "wtype":
		cmd = exec.CommandContext(ctx, "wtype", "--", text)
	case "xdotool":
		cmd = exec.CommandContext(ctx, "xdotool", "type", "--clearmodifiers", "--delay", "12", "--", text)
	default:
		return fmt.Errorf("unknown tool: %s", tool)
	}

	log.Debug("inject type via %s (%d chars)", tool, len(text))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inject type (%s): %w\noutput: %s", tool, err, string(out))
	}
	return nil
}

// runPaste sends a clean Ctrl+V with all modifiers properly released first.
func (inj *Injector) runPaste(tool string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch tool {
	case "ydotool":
		// Sequence:
		//   1. Release Alt (keycode 56:0) — it was held for the Alt+G hotkey
		//   2. Release G key (keycode 34:0) — just in case
		//   3. Press Ctrl+V using raw keycodes (29 = LEFTCTRL, 47 = V)
		cmd = exec.CommandContext(ctx, "ydotool", "key",
			"--key-delay=30",
			// Release modifiers first
			fmt.Sprintf("%d:0", keyCodeLeftAlt),
			fmt.Sprintf("%d:0", keyCodeRightAlt),
			fmt.Sprintf("%d:0", 34), // KEY_G = 34
			// Then press Ctrl+V
			fmt.Sprintf("%d:1", keyCodeLeftCtrl),
			fmt.Sprintf("%d:1", keyCodeV),
			fmt.Sprintf("%d:0", keyCodeV),
			fmt.Sprintf("%d:0", keyCodeLeftCtrl),
		)
		setYdotoolEnv(cmd)

	case "wtype":
		// wtype: release alt modifier, then send Ctrl+V
		cmd = exec.CommandContext(ctx, "wtype", "-m", "ctrl", "-k", "v", "-m", "ctrl")

	case "xdotool":
		// xdotool: --clearmodifiers handles this
		cmd = exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v")

	default:
		return fmt.Errorf("unknown tool for paste: %s", tool)
	}

	log.Debug("inject paste via %s", tool)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inject paste (%s): %w\noutput: %s", tool, err, string(out))
	}
	return nil
}

// setYdotoolEnv sets YDOTOOL_SOCKET in the command environment.
func setYdotoolEnv(cmd *exec.Cmd) {
	sock := ydotoolSocketPath()
	if sock != "" {
		cmd.Env = append(os.Environ(), "YDOTOOL_SOCKET="+sock)
	}
}

// splitKeys splits a space-separated keycode string into individual tokens.
func splitKeys(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
