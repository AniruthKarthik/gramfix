// Command gramfix captures selected text, fixes grammar locally using
// LanguageTool, and injects the corrected text back — all offline.
//
// A lock file prevents simultaneous runs from key-repeat events.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AniruthKarthik/gramfix/internal/core"
	"github.com/AniruthKarthik/gramfix/internal/env"
	"github.com/AniruthKarthik/gramfix/internal/grammar"
	"github.com/AniruthKarthik/gramfix/internal/log"
)

const version = "1.1.0"

// lockFile prevents concurrent runs from key-repeat events.
const lockFile = "/tmp/gramfix.lock"



func main() {
	// ── Load .env ────────────────────────────────────────────────────────
	env.LoadDotEnv()

	// ── CLI flags ────────────────────────────────────────────────────────
	var (
		lang          = flag.String("lang", envOr("GRAMFIX_LANG", "en-US"), "LanguageTool language code")
		debug         = flag.Bool("debug", envBool("GRAMFIX_DEBUG"), "enable debug logging")
		ver           = flag.Bool("version", false, "print version and exit")
		logFile       = flag.Bool("log", envBool("GRAMFIX_LOG_FILE"), "write logs to ~/.local/share/gramfix/gramfix.log")
		ngramDir      = flag.String("ngram-dir", os.Getenv("GRAMFIX_NGRAM_DIR"), "path to LT n-gram data directory")
		customRules   = flag.String("rules", os.Getenv("GRAMFIX_CUSTOM_RULES"), "path to custom LT XML rules file")
		serverURL     = flag.String("server", os.Getenv("GRAMFIX_LT_SERVER_URL"), "local LT server URL (e.g. http://localhost:8081)")
		disabledRules = flag.String("disabled-rules", os.Getenv("GRAMFIX_DISABLED_RULES"), "comma-separated LT rule IDs to disable")
		enabledRules  = flag.String("enabled-rules", os.Getenv("GRAMFIX_ENABLED_RULES"), "comma-separated LT rule IDs to force-enable")
		enabledCats   = flag.String("enabled-categories", os.Getenv("GRAMFIX_ENABLED_CATEGORIES"), "comma-separated LT category IDs to enable")
		confidence    = flag.Int("confidence", envInt("GRAMFIX_CONFIDENCE", 55), "minimum match confidence 0-100")
		jvmMaxHeap    = flag.String("jvm-heap", envOr("GRAMFIX_JVM_XMX", "256m"), "JVM max heap for LT CLI (e.g. 256m)")
		picky         = flag.Bool("picky", !envBool("GRAMFIX_NO_PICKY"), "enable LT PICKY level (more rules, default: on)")
		enableTempOff = flag.Bool("temp-off", envBool("GRAMFIX_ENABLE_TEMP_OFF"), "enable LT TEMP_OFF rules (experimental)")
		multiPass     = flag.Bool("multi-pass", !envBool("GRAMFIX_NO_MULTI_PASS"), "run correction in multiple passes (default: on)")
		maxPasses     = flag.Int("max-passes", envInt("GRAMFIX_MAX_PASSES", 2), "maximum number of correction passes")
		stdinMode     = flag.Bool("stdin", false, "read text from stdin, write corrected text to stdout, then exit (no clipboard/injection)")
		orAPIKey      = flag.String("openrouter-key", os.Getenv("OPENROUTER_API_KEY"), "OpenRouter API key")
		orModel       = flag.String("openrouter-model", envOr("OPENROUTER_MODEL", ""), "OpenRouter model (overrides default free list)")
		groqAPIKey    = flag.String("groq-key", os.Getenv("GROQ_API_KEY"), "Groq API key")
		groqModel     = flag.String("groq-model", envOr("GROQ_MODEL", ""), "Groq model (overrides default list)")
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
			fmt.Fprintf(os.Stderr, "log file error: %v (dir: %s)\n", err, log.LogDir())
		} else {
			defer f.Close()
		}
	}

	// Build engine config (shared by all modes)
	engCfg := grammar.DefaultEngineConfig()
	engCfg.Lang = *lang
	engCfg.NgramDir = *ngramDir
	engCfg.CustomRulesFile = *customRules
	engCfg.ServerURL = *serverURL
	engCfg.JVMMaxHeap = *jvmMaxHeap
	engCfg.Picky = *picky
	engCfg.EnableTempOff = *enableTempOff
	engCfg.MultiPass = *multiPass
	engCfg.MaxPasses = *maxPasses
	engCfg.OpenRouterAPIKey = *orAPIKey
	engCfg.OpenRouterModel = *orModel
	engCfg.GroqAPIKey = *groqAPIKey
	engCfg.GroqModel = *groqModel
	if *confidence > 0 {
		engCfg.ConfidenceMin = *confidence
	}
	if *disabledRules != "" {
		engCfg.DisabledRules = splitCSV(*disabledRules)
	}
	if *enabledRules != "" {
		engCfg.EnabledRules = splitCSV(*enabledRules)
	}
	if *enabledCats != "" {
		engCfg.EnabledCategories = splitCSV(*enabledCats)
	}

	// ── stdin mode: pipe-friendly correction without clipboard/injection ──
	if *stdinMode {
		runStdinMode(engCfg)
		return
	}

	// ── Single-instance lock ─────────────────────────────────────────────
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
		EngineConfig: engCfg,
		Debug:        *debug,
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

// ── stdin mode ───────────────────────────────────────────────────────────────

// runStdinMode reads all text from stdin, runs grammar correction, and prints
// the corrected text to stdout.  It bypasses the clipboard/injection pipeline
// entirely, making it suitable for piped usage and the bench.sh benchmark.
func runStdinMode(cfg grammar.EngineConfig) {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}
	text := strings.TrimRight(string(input), "\n")
	if strings.TrimSpace(text) == "" {
		fmt.Print(text)
		return
	}

	eng, err := grammar.NewWithConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "engine init: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	corrected, err := eng.Fix(ctx, text)
	if err != nil {
		// On error, return original text unchanged (safe fallback)
		fmt.Print(text)
		return
	}
	fmt.Print(corrected)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "true" || v == "1" || v == "yes"
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ── Lock file ────────────────────────────────────────────────────────────────

func acquireLock() bool {
	tmp := fmt.Sprintf("%s.%d.tmp", lockFile, os.Getpid())
	content := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return true // can't write tmp; proceed anyway
	}
	if err := os.Link(tmp, lockFile); err != nil {
		_ = os.Remove(tmp)
		return !isLockAlive(lockFile)
	}
	_ = os.Remove(tmp)
	_ = os.WriteFile(lockFile, []byte(content), 0o644)
	return true
}

func isLockAlive(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func releaseLock() {
	lockPath := filepath.Join(os.TempDir(), "gramfix.lock")
	_ = os.Remove(lockPath)
	_ = os.Remove(lockFile)
}
