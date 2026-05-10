// Package hotkey provides a lightweight hotkey listener daemon.
// It monitors for Alt+G and launches the gramfix binary on each press.
// The daemon itself is minimal — no GUI, near-zero CPU/RAM idle usage.
package hotkey

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anilnair00/gramfix/internal/env"
	"github.com/anilnair00/gramfix/internal/log"
)

// Daemon is a lightweight hotkey listener.
type Daemon struct {
	session    *env.Session
	gramfixBin string
	lang       string
	debug      bool
}

// New creates a new hotkey Daemon.
// gramfixBin is the path to the gramfix binary to launch on hotkey.
func New(s *env.Session, gramfixBin, lang string, debug bool) *Daemon {
	return &Daemon{
		session:    s,
		gramfixBin: gramfixBin,
		lang:       lang,
		debug:      debug,
	}
}

// Run starts the hotkey listener and blocks until ctx is cancelled.
// It tries sxhkd first, then xbindkeys, then a fallback polling loop.
func (d *Daemon) Run(ctx context.Context) error {
	switch {
	case d.session.IsX11 && d.session.HasXbindkeys:
		return d.runXbindkeys(ctx)
	case d.session.HasSxhkd:
		return d.runSxhkd(ctx)
	default:
		// Last resort: use a simple polling loop with xdotool/ydotool
		// (covers Wayland when sxhkd is not available)
		log.Warn("no dedicated hotkey daemon found; using evdev polling (requires root or uinput group)")
		return d.runEvdevFallback(ctx)
	}
}

// runXbindkeys launches xbindkeys with a temporary config for Alt+G.
func (d *Daemon) runXbindkeys(ctx context.Context) error {
	cfgPath, err := d.writeXbindkeysConfig()
	if err != nil {
		return fmt.Errorf("xbindkeys config: %w", err)
	}
	defer os.Remove(cfgPath)

	log.Info("hotkey: using xbindkeys (X11)")
	cmd := exec.CommandContext(ctx, "xbindkeys", "--nodaemon", "-f", cfgPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runSxhkd launches sxhkd with a temporary hotkeys file for Alt+G.
func (d *Daemon) runSxhkd(ctx context.Context) error {
	cfgPath, err := d.writeSxhkdConfig()
	if err != nil {
		return fmt.Errorf("sxhkd config: %w", err)
	}
	defer os.Remove(cfgPath)

	log.Info("hotkey: using sxhkd")
	cmd := exec.CommandContext(ctx, "sxhkd", "-c", cfgPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runEvdevFallback is a simple polling approach using xinput or /dev/input.
// On Wayland without sxhkd, we suggest the user set up a shortcut manually.
func (d *Daemon) runEvdevFallback(ctx context.Context) error {
	log.Info("hotkey: evdev fallback not implemented — please configure a system shortcut")
	log.Info("  shortcut command: %s", d.gramfixBin)
	log.Info("  suggested key:   Alt+G")
	fmt.Println("No hotkey daemon available. Please configure a system keyboard shortcut to run:")
	fmt.Printf("  %s\n", d.gramfixBin)
	<-ctx.Done()
	return nil
}

// writeXbindkeysConfig writes a temporary xbindkeys config file.
func (d *Daemon) writeXbindkeysConfig() (string, error) {
	args := d.buildArgs()
	content := fmt.Sprintf(`# gramfix hotkey (Alt+G)
"%s %s"
  alt + g
`, d.gramfixBin, args)

	f, err := os.CreateTemp("", "gramfix-xbindkeys-*.rc")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// writeSxhkdConfig writes a temporary sxhkd config file.
func (d *Daemon) writeSxhkdConfig() (string, error) {
	args := d.buildArgs()
	content := fmt.Sprintf(`# gramfix Alt+G
alt + g
	%s %s
`, d.gramfixBin, args)

	f, err := os.CreateTemp("", "gramfix-sxhkd-*.conf")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func (d *Daemon) buildArgs() string {
	parts := []string{}
	if d.lang != "" && d.lang != "en-US" {
		parts = append(parts, "--lang", d.lang)
	}
	if d.debug {
		parts = append(parts, "--debug")
	}
	return strings.Join(parts, " ")
}

// WriteSystemdUnit writes a systemd user service file for the hotkey daemon.
func WriteSystemdUnit(daemonBin, gramfixBin, lang string, debug bool) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	debugFlag := ""
	if debug {
		debugFlag = " --debug"
	}

	content := fmt.Sprintf(`[Unit]
Description=GramFix Hotkey Daemon (Alt+G grammar correction)
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=%s --gramfix=%s --lang=%s%s
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=graphical-session.target
`, daemonBin, gramfixBin, lang, debugFlag)

	unitPath := filepath.Join(dir, "gramfix-hotkey.service")
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return unitPath, nil
}

// WatchPid monitors a PID file written by a child process for debugging.
func WatchPid(ctx context.Context, pidFile string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := os.ReadFile(pidFile)
			if err == nil {
				log.Debug("child pid: %s", strings.TrimSpace(string(data)))
			}
		}
	}
}

// RunInBackground executes the gramfix binary in the background and returns immediately.
// It captures output to the log file if available.
func RunInBackground(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch gramfix: %w", err)
	}

	// Wait in a goroutine so zombies are reaped
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Warn("gramfix exited with error: %v", err)
		}
	}()
	return nil
}

// ScanTriggerFile reads trigger lines from a named pipe or file.
// Used for custom trigger integration (e.g. KDE shortcuts calling "echo trigger > pipe").
func ScanTriggerFile(ctx context.Context, path string, cb func()) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open trigger file %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "trigger" || line == "fix" {
				cb()
			}
		}
	}
}
