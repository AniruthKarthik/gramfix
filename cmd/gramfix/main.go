// Command gramfix captures selected text, fixes grammar locally using
// LanguageTool, and injects the corrected text back — all offline.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anilnair00/gramfix/internal/core"
	"github.com/anilnair00/gramfix/internal/log"
)

const version = "1.0.0"

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
