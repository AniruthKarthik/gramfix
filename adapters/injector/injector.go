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
	// ydotool works on both Wayland and X11 via the uinput kernel module
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

// ydotoolAvailable checks if ydotoold daemon socket is accessible.
func ydotoolAvailable() bool {
	sock := ydotoolSocketPath()
	if sock == "" {
		return false
	}
	_, err := os.Stat(sock)
	return err == nil
}

// ydotoolSocketPath returns the ydotool socket path from env or default.
func ydotoolSocketPath() string {
	if s := os.Getenv("YDOTOOL_SOCKET"); s != "" {
		return s
	}
	// Check default
	if _, err := os.Stat(ydotoolSocket); err == nil {
		return ydotoolSocket
	}
	return ""
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
func (inj *Injector) PasteFromClipboard() error {
	tool, err := inj.bestTool()
	if err != nil {
		return err
	}
	return inj.runKey(tool, "ctrl+v")
}

// runType executes the text injection command.
func (inj *Injector) runType(tool, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), injectTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch tool {
	case "ydotool":
		args := []string{"type", "--key-delay=12", "--", text}
		cmd = exec.CommandContext(ctx, "ydotool", args...)
		setYdotoolEnv(cmd)

	case "wtype":
		// wtype types the literal text
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

// runKey sends a key combination.
func (inj *Injector) runKey(tool, key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch tool {
	case "ydotool":
		// ydotool key uses XF86 keysym names: ctrl+v → KEY_LEFTCTRL:1 KEY_V:1 KEY_V:0 KEY_LEFTCTRL:0
		// But ydotool key accepts "ctrl+v" shorthand since newer versions
		cmd = exec.CommandContext(ctx, "ydotool", "key", "--key-delay=50", "ctrl+v")
		setYdotoolEnv(cmd)

	case "wtype":
		// wtype key requires separate modifier flags
		// wtype -M ctrl -k v -m ctrl
		cmd = exec.CommandContext(ctx, "wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl")

	case "xdotool":
		cmd = exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v")

	default:
		return fmt.Errorf("unknown tool for key: %s", tool)
	}

	log.Debug("inject key %s via %s", key, tool)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inject key (%s via %s): %w\noutput: %s", key, tool, err, string(out))
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
