// Package core orchestrates the full gramfix pipeline:
//
//  1. Capture selected text (primary selection → clipboard fallback)
//  2. Replace selection with placeholder via clipboard+paste
//  3. Fix grammar locally using LanguageTool
//  4. Replace placeholder with corrected text
//  5. Restore original clipboard
//  6. Exit cleanly — no orphan processes
package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anilnair00/gramfix/adapters/clipboard"
	"github.com/anilnair00/gramfix/adapters/injector"
	"github.com/anilnair00/gramfix/internal/env"
	"github.com/anilnair00/gramfix/internal/grammar"
	"github.com/anilnair00/gramfix/internal/log"
)

// Placeholder is the temporary text shown while grammar correction runs.
const Placeholder = "fixing grammar…"

// Config holds runtime configuration for the orchestrator.
type Config struct {
	Language string // LanguageTool language code, e.g. "en-US"
	Debug    bool
}

// DefaultConfig returns sane defaults.
func DefaultConfig() Config {
	return Config{
		Language: "en-US",
	}
}

// Orchestrator coordinates all components.
type Orchestrator struct {
	cfg     Config
	session *env.Session
	clip    *clipboard.Manager
	inj     *injector.Injector
	grammar *grammar.Engine
}

// New builds and validates all components.
func New(cfg Config) (*Orchestrator, error) {
	sess := env.Detect()
	log.Info("environment: %s", sess.Summary())

	clip := clipboard.New(sess)
	inj := injector.New(sess)

	eng, err := grammar.New(cfg.Language)
	if err != nil {
		return nil, fmt.Errorf("grammar engine: %w", err)
	}

	return &Orchestrator{
		cfg:     cfg,
		session: sess,
		clip:    clip,
		inj:     inj,
		grammar: eng,
	}, nil
}

// Run executes the complete pipeline.
func (o *Orchestrator) Run(ctx context.Context) error {
	// ── Step 1: Snapshot current clipboard to restore later ──────────────
	originalClip, _ := o.clip.ReadClipboard()
	log.Debug("clipboard backup: %d chars", len(originalClip))

	// ── Step 2: Read selected text from primary selection ─────────────────
	selected, err := o.captureSelection()
	if err != nil {
		return fmt.Errorf("capture selection: %w", err)
	}
	if strings.TrimSpace(selected) == "" {
		log.Info("no text selected, nothing to do")
		return nil
	}
	log.Info("captured selection: %q (len=%d)", truncate(selected, 60), len(selected))

	// ── Step 3: Replace selection with placeholder ─────────────────────────
	// Write placeholder to clipboard, then simulate Ctrl+V to paste over selection.
	// Kill the wl-copy serving this immediately after paste (corrected text replaces it).
	if err := o.injectText(Placeholder, true); err != nil {
		log.Warn("placeholder injection failed: %v — continuing anyway", err)
	}

	// ── Step 4: Run grammar correction ─────────────────────────────────────
	corrected, err := o.grammar.Fix(ctx, selected)
	if err != nil {
		log.Error("grammar fix failed: %v", err)
		// Restore original text and clipboard
		_ = o.injectText(selected, true)
		_ = o.restoreClipboard(originalClip)
		return fmt.Errorf("grammar fix: %w", err)
	}
	log.Info("corrected: %q", truncate(corrected, 80))

	// ── Step 5: Replace placeholder with corrected text ────────────────────
	// Kill wl-copy after paste — clipboard restore will take over.
	if err := o.injectText(corrected, true); err != nil {
		return fmt.Errorf("inject corrected text: %w", err)
	}

	// ── Step 6: Restore original clipboard ─────────────────────────────────
	// This wl-copy stays alive (normal) — it's the user's original clipboard.
	if err := o.restoreClipboard(originalClip); err != nil {
		log.Warn("clipboard restore failed: %v", err)
	}

	log.Info("pipeline complete — process exiting")
	return nil
}

// captureSelection reads the primary selection (selected text).
// Falls back to clipboard if primary is empty or unavailable.
func (o *Orchestrator) captureSelection() (string, error) {
	// Try primary selection first (this is what's highlighted in the UI)
	text, err := o.clip.ReadPrimary()
	if err == nil && strings.TrimSpace(text) != "" {
		log.Debug("read from primary selection (%d chars)", len(text))
		return strings.TrimRight(text, "\n"), nil
	}
	if err != nil {
		log.Debug("primary read: %v", err)
	} else {
		log.Debug("primary selection empty")
	}

	// Fallback: try clipboard (user may have Ctrl+C'd the text)
	text, err = o.clip.ReadClipboard()
	if err != nil {
		return "", fmt.Errorf("all clipboard sources failed: %w", err)
	}
	if strings.TrimSpace(text) != "" {
		log.Debug("read from clipboard fallback (%d chars)", len(text))
		return strings.TrimRight(text, "\n"), nil
	}

	return "", nil
}

// injectText writes text to clipboard and pastes it, replacing the current selection.
// If killAfterPaste is true, the background wl-copy is killed immediately after
// the paste completes (since the text is now in the focused field, not the clipboard).
func (o *Orchestrator) injectText(text string, killAfterPaste bool) error {
	// Write to clipboard (starts background wl-copy on Wayland)
	if err := o.clip.WriteClipboard(text); err != nil {
		return fmt.Errorf("write to clipboard: %w", err)
	}

	// Stabilize: let compositor register the clipboard change
	time.Sleep(100 * time.Millisecond)

	// Paste via Ctrl+V — replaces the active selection in the focused field
	if err := o.inj.PasteFromClipboard(); err != nil {
		log.Warn("ctrl+v failed (%v), trying direct type", err)
		if killAfterPaste {
			o.clip.Cleanup()
		}
		return o.inj.TypeText(text)
	}

	// Let the paste land before the next operation
	time.Sleep(150 * time.Millisecond)

	// Kill the intermediate wl-copy — it served its purpose
	if killAfterPaste {
		o.clip.Cleanup()
	}

	return nil
}

// restoreClipboard puts the original clipboard content back.
// The resulting wl-copy process is intentionally left alive — it's the user's clipboard.
func (o *Orchestrator) restoreClipboard(original string) error {
	if original == "" {
		// Nothing to restore; make sure no intermediate wl-copy is lingering
		o.clip.Cleanup()
		return nil
	}
	return o.clip.WriteClipboard(original)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
