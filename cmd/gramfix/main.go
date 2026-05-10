// Command gramfix captures selected text, fixes grammar locally using
// LanguageTool, and injects the corrected text back — all offline.
//
// A lock file prevents simultaneous runs from key-repeat events.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anilnair00/gramfix/internal/core"
	"github.com/anilnair00/gramfix/internal/log"
)

const version = "1.0.0"

// lockFile prevents concurrent runs from key-repeat events.
const lockFile = "/tmp/gramfix.lock"

// lockTimeout is how long we wait for an existing run to finish before aborting.
const lockTimeout = 2 * time.Second

func main() {
	// ── CLI flags ────────────────────────────────────────────────────────
	var (
		lang    = flag.String("lang", "en-US", "LanguageTool language code")
		debug   = flag.Bool("debug", false, "enable debug logging")
		ver     = flag.Bool("version", false, "print version and exit")
		logFile = flag.Bool("log", false, "write logs to ~/.local/share/gramfix/gramfix.log")
	)
	flag.Parse()

	if *ver {
		fmt.Printf("gramfix v%s\n", version)
		os.Exit(0)
	}

	if *debug {
		log.SetDebug(true)
	}

	if *logFile {
		f, err := log.LogFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "log file error: %v\n", err)
		} else {
			defer f.Close()
		}
	}

	// ── Single-instance lock ─────────────────────────────────────────────
	// sxhkd fires on every key-repeat event. We use an atomic lock file to
	// ensure only one gramfix process runs at a time.
	if !acquireLock() {
		log.Info("another gramfix instance is running — skipping (key repeat?)")
		os.Exit(0)
	}
	defer releaseLock()

	log.Info("gramfix v%s starting", version)

	// ── Context with signal cancellation ────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Warn("signal received: %v, cancelling", sig)
		cancel()
	}()

	// ── Build orchestrator ───────────────────────────────────────────────
	cfg := core.Config{
		Language: *lang,
		Debug:    *debug,
	}

	orch, err := core.New(cfg)
	if err != nil {
		log.Error("initialisation failed: %v", err)
		os.Exit(1)
	}

	// ── Run pipeline ─────────────────────────────────────────────────────
	if err := orch.Run(ctx); err != nil {
		log.Error("pipeline error: %v", err)
		os.Exit(1)
	}

	log.Info("gramfix done")
	// Process terminates cleanly here — no daemons, no background goroutines.
}

// acquireLock creates an exclusive lock file. Returns false if already locked.
func acquireLock() bool {
	// Write our PID to a temp file, then atomically rename — this is race-safe.
	tmp := fmt.Sprintf("%s.%d.tmp", lockFile, os.Getpid())
	content := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return true // can't write tmp; proceed anyway
	}

	// os.Rename on Linux is atomic
	if err := os.Link(tmp, lockFile); err != nil {
		_ = os.Remove(tmp)
		// Lock exists — check if the PID is still alive
		return !isLockAlive(lockFile)
	}
	_ = os.Remove(tmp)

	// Also write the real PID so callers can verify
	_ = os.WriteFile(lockFile, []byte(content), 0o644)
	return true
}

// isLockAlive returns true if the lock file contains a live process PID.
func isLockAlive(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	// Check if process exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Send signal 0 to test if process is alive
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// releaseLock removes the lock file.
func releaseLock() {
	lockPath := filepath.Join(os.TempDir(), "gramfix.lock")
	_ = os.Remove(lockPath)
	_ = os.Remove(lockFile)
}
