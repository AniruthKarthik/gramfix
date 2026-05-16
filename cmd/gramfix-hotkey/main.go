// Command gramfix-hotkey is the lightweight hotkey listener daemon.
// It listens for Alt+G and launches the gramfix binary on each press.
//
// This binary is meant to be run as a systemd user service or autostart entry.
// It uses near-zero CPU/RAM at idle.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AniruthKarthik/gramfix/adapters/hotkey"
	"github.com/AniruthKarthik/gramfix/internal/env"
	"github.com/AniruthKarthik/gramfix/internal/log"
)

const version = "1.0.0"

func main() {
	env.LoadDotEnv()
	var (
		gramfixBin = flag.String("gramfix", "", "path to gramfix binary (auto-detected if empty)")
		lang       = flag.String("lang", "en-US", "LanguageTool language code")
		debug      = flag.Bool("debug", false, "enable debug logging")
		logFile    = flag.Bool("log", false, "write logs to file")
		ver        = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *ver {
		fmt.Printf("gramfix-hotkey v%s\n", version)
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

	// Auto-detect gramfix binary
	bin := *gramfixBin
	if bin == "" {
		bin = findGramfixBin()
	}
	if bin == "" {
		fmt.Fprintln(os.Stderr, "gramfix binary not found; install gramfix first or use --gramfix flag")
		os.Exit(1)
	}
	log.Info("gramfix-hotkey v%s", version)
	log.Info("gramfix binary: %s", bin)

	sess := env.Detect()
	log.Info("environment: %s", sess.Summary())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("signal: %v — shutting down", sig)
		cancel()
	}()

	daemon := hotkey.New(sess, bin, *lang, *debug)
	if err := daemon.Run(ctx); err != nil {
		log.Error("hotkey daemon error: %v", err)
		os.Exit(1)
	}
}

// findGramfixBin searches common install locations for the gramfix binary.
func findGramfixBin() string {
	candidates := []string{
		// Same directory as this binary
		filepath.Join(filepath.Dir(os.Args[0]), "gramfix"),
		// User local
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "gramfix"),
		// System
		"/usr/local/bin/gramfix",
		"/usr/bin/gramfix",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Try PATH
	if p, err := exec.LookPath("gramfix"); err == nil {
		return p
	}
	return ""
}
