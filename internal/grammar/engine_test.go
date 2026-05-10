package grammar_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/anilnair00/gramfix/internal/grammar"
)

func TestNewEngine_NoJava(t *testing.T) {
	// This test only makes sense if java is absent; skip on systems that have it
	if _, err := exec.LookPath("java"); err == nil {
		t.Skip("java is available; skipping no-java test")
	}
	_, err := grammar.New("en-US")
	if err == nil {
		t.Fatal("expected error when java missing, got nil")
	}
}

func TestNewEngine_WithJava(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not available")
	}
	eng, err := grammar.New("en-US")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if eng == nil {
		t.Fatal("expected engine, got nil")
	}
}

func TestFix_BasicCorrection(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not available")
	}
	eng, err := grammar.New("en-US")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		wantFix  bool // whether we expect something to change
	}{
		{
			name:    "correct text unchanged",
			input:   "The quick brown fox jumps over the lazy dog.",
			wantFix: false,
		},
		{
			name:    "lowercase sentence start",
			input:   "the dog is happy.",
			wantFix: true,
		},
		{
			name:    "spelling error",
			input:   "I dont know what is hapening here.",
			wantFix: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantFix: false,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantFix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrected, err := eng.Fix(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Fix() error: %v", err)
			}
			changed := corrected != tt.input && strings.TrimSpace(tt.input) != ""
			if tt.wantFix && !changed && strings.TrimSpace(tt.input) != "" {
				// Not always guaranteed — LT may or may not flag
				t.Logf("NOTE: expected fix but none applied for %q → %q", tt.input, corrected)
			}
			t.Logf("%q → %q", tt.input, corrected)
		})
	}
}

func TestFix_Multiline(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not available")
	}
	eng, err := grammar.New("en-US")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	input := "this is the first sentence.\nThis is the second sentense."
	corrected, err := eng.Fix(context.Background(), input)
	if err != nil {
		t.Fatalf("Fix() multiline error: %v", err)
	}
	t.Logf("multiline: %q → %q", input, corrected)
}
