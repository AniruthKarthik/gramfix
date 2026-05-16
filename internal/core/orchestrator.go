// Package core orchestrates the full gramfix pipeline:
//
//  1. Release held modifier keys (Alt from the hotkey)
//  2. Capture selected text (primary selection → clipboard fallback)
//  3. Replace selection with placeholder via clipboard+paste
//  4. Fix grammar locally using LanguageTool
//  5. Replace placeholder with corrected text
//  6. Restore original clipboard
//  7. Exit cleanly — no orphan processes
package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AniruthKarthik/gramfix/adapters/clipboard"
	"github.com/AniruthKarthik/gramfix/adapters/injector"
	"github.com/AniruthKarthik/gramfix/internal/env"
	"github.com/AniruthKarthik/gramfix/internal/grammar"
	"github.com/AniruthKarthik/gramfix/internal/log"
)

// Placeholder is the temporary text shown while grammar correction runs.
const Placeholder = "fixing grammar…"

// modifierSettleDelay is the time we wait after releasing Alt before injecting.
// This gives the compositor/X11 time to process the key-up events.
const modifierSettleDelay = 150 * time.Millisecond

// Config holds runtime configuration for the orchestrator.
type Config struct {
	// EngineConfig is passed directly to the grammar engine.
	grammar.EngineConfig
	Debug bool
}

// DefaultConfig returns sane defaults.
func DefaultConfig() Config {
	return Config{
		EngineConfig: grammar.DefaultEngineConfig(),
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

	eng, err := grammar.NewWithConfig(cfg.EngineConfig)
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
	// ── Step 0: Release held modifier keys ────────────────────────────────
	// When gramfix is launched by Alt+G, the Alt key is still held at the
	// OS level. We release all modifiers so subsequent Ctrl+V isn't received
	// as Alt+Ctrl+V by the target application.
	log.Debug("releasing modifier keys")
	o.inj.ReleaseModifiers()
	time.Sleep(modifierSettleDelay)

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
	if err := o.injectText(Placeholder, true); err != nil {
		log.Warn("placeholder injection failed: %v — continuing anyway", err)
	}

	// ── Step 4: Run grammar correction ─────────────────────────────────────
	corrected, err := o.grammar.Fix(ctx, selected)
	if err != nil {
		log.Error("grammar fix failed: %v", err)
		// Delete placeholder and restore original text
		_ = o.inj.Backspace(len([]rune(Placeholder)))
		_ = o.injectText(selected, true)
		_ = o.restoreClipboard(originalClip)
		return fmt.Errorf("grammar fix: %w", err)
	}
	log.Info("corrected: %q", truncate(corrected, 80))

	// ── Step 5: Replace placeholder with corrected text ────────────────────
	// Delete the placeholder first
	placeholderLen := len([]rune(Placeholder))
	log.Debug("deleting placeholder (%d chars)", placeholderLen)
	if err := o.inj.Backspace(placeholderLen); err != nil {
		log.Warn("failed to delete placeholder: %v", err)
	}

	if err := o.injectText(corrected, true); err != nil {
		return fmt.Errorf("inject corrected text: %w", err)
	}

	// ── Step 6: Restore original clipboard ─────────────────────────────────
	if err := o.restoreClipboard(originalClip); err != nil {
		log.Warn("clipboard restore failed: %v", err)
	}

	log.Info("pipeline complete — process exiting")
	return nil
}

// captureSelection reads the primary selection (highlighted text).
// Falls back to clipboard if primary is empty.
func (o *Orchestrator) captureSelection() (string, error) {
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

// injectText writes text to clipboard then pastes it via Ctrl+V.
// If killAfterPaste is true, the intermediate wl-copy is killed immediately.
func (o *Orchestrator) injectText(text string, killAfterPaste bool) error {
	if err := o.clip.WriteClipboard(text); err != nil {
		return fmt.Errorf("write to clipboard: %w", err)
	}

	// Let the compositor register the clipboard change
	time.Sleep(100 * time.Millisecond)

	// Paste — this uses ydotool with proper modifier release built in
	if err := o.inj.PasteFromClipboard(); err != nil {
		log.Warn("paste failed (%v), trying direct type", err)
		if killAfterPaste {
			o.clip.Cleanup()
		}
		return o.inj.TypeText(text)
	}

	// Let the paste land
	time.Sleep(150 * time.Millisecond)

	if killAfterPaste {
		o.clip.Cleanup()
	}
	return nil
}

// restoreClipboard puts the original clipboard content back.
func (o *Orchestrator) restoreClipboard(original string) error {
	if original == "" {
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
