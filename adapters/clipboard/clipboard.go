// Package clipboard provides cross-platform clipboard read/write adapters.
// On Wayland, wl-copy stays alive to "serve" clipboard content until replaced.
// We handle this by starting it detached and killing it when done.
package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anilnair00/gramfix/internal/env"
	"github.com/anilnair00/gramfix/internal/log"
)

const readTimeout = 3 * time.Second

// Manager handles clipboard and primary selection operations.
type Manager struct {
	session *env.Session

	mu        sync.Mutex
	wlCopyPID *os.Process // tracks background wl-copy process
}

// New returns a new clipboard Manager.
func New(s *env.Session) *Manager {
	return &Manager{session: s}
}

// ReadPrimary reads the current primary (X selection / Wayland primary) buffer.
func (m *Manager) ReadPrimary() (string, error) {
	cmd, args, ok := m.session.PrimaryReader()
	if !ok {
		return "", fmt.Errorf("no primary clipboard reader available")
	}
	return runRead(cmd, args)
}

// ReadClipboard reads the clipboard (Ctrl+C buffer).
func (m *Manager) ReadClipboard() (string, error) {
	cmd, args, ok := m.session.ClipboardReader()
	if !ok {
		return "", fmt.Errorf("no clipboard reader available")
	}
	return runRead(cmd, args)
}

// WriteClipboard writes text to the clipboard buffer.
// On Wayland, wl-copy is started in the background (it must stay alive to serve clipboard).
// On X11, the write is synchronous.
func (m *Manager) WriteClipboard(text string) error {
	if m.session.IsWayland && m.session.HasWlCopy {
		return m.writeWayland(text, false)
	}
	cmd, args, ok := m.session.ClipboardWriter()
	if !ok {
		return fmt.Errorf("no clipboard writer available")
	}
	return runWriteSync(cmd, args, text)
}

// WritePrimary writes text to the primary selection buffer.
func (m *Manager) WritePrimary(text string) error {
	if m.session.IsWayland && m.session.HasWlCopy {
		return m.writeWayland(text, true)
	}
	cmd, args, ok := m.session.PrimaryWriter()
	if !ok {
		return fmt.Errorf("no primary writer available")
	}
	return runWriteSync(cmd, args, text)
}

// writeWayland starts wl-copy detached. It kills any previous wl-copy instance
// we spawned so we don't orphan processes.
func (m *Manager) writeWayland(text string, primary bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Kill previous wl-copy we spawned (replacing clipboard kills it anyway, but be explicit)
	if m.wlCopyPID != nil {
		_ = m.wlCopyPID.Kill()
		_, _ = m.wlCopyPID.Wait()
		m.wlCopyPID = nil
	}

	args := []string{}
	if primary {
		args = append(args, "--primary")
	}

	log.Debug("wl-copy detached: primary=%v len=%d", primary, len(text))
	cmd := exec.Command("wl-copy", args...)
	cmd.Stdin = bytes.NewBufferString(text)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("wl-copy start: %w", err)
	}

	// Reap in background to avoid zombies; also track PID for cleanup
	m.wlCopyPID = cmd.Process
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if m.wlCopyPID == cmd.Process {
			m.wlCopyPID = nil
		}
		m.mu.Unlock()
	}()

	// Give wl-copy a moment to register with the compositor
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Cleanup kills any background wl-copy processes spawned by this manager.
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.wlCopyPID != nil {
		_ = m.wlCopyPID.Kill()
		_, _ = m.wlCopyPID.Wait()
		m.wlCopyPID = nil
	}
}

func runRead(cmd string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()

	log.Debug("clipboard read: %s %v", cmd, args)
	out, err := exec.CommandContext(ctx, cmd, args...).Output()
	if err != nil {
		// Empty clipboard returns exit status 1 for some tools — treat as empty
		if strings.Contains(err.Error(), "exit status") {
			return "", nil
		}
		return "", fmt.Errorf("clipboard read (%s): %w", cmd, err)
	}
	return string(out), nil
}

func runWriteSync(cmd string, args []string, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	log.Debug("clipboard write sync: %s %v (len=%d)", cmd, args, len(text))
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdin = bytes.NewBufferString(text)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clipboard write (%s): %w\noutput: %s", cmd, err, string(out))
	}
	return nil
}
