package env_test

import (
	"testing"

	"github.com/anilnair00/gramfix/internal/env"
)

func TestDetect(t *testing.T) {
	s := env.Detect()
	if s == nil {
		t.Fatal("Detect() returned nil")
	}
	t.Logf("session: %+v", s.Summary())
	t.Logf("wayland=%v x11=%v distro=%s", s.IsWayland, s.IsX11, s.Distro)
}

func TestClipboardReader(t *testing.T) {
	s := env.Detect()
	cmd, args, ok := s.ClipboardReader()
	if !ok {
		t.Log("no clipboard reader available (may be CI)")
		return
	}
	t.Logf("clipboard reader: %s %v", cmd, args)
}

func TestPrimaryReader(t *testing.T) {
	s := env.Detect()
	cmd, args, ok := s.PrimaryReader()
	if !ok {
		t.Log("no primary reader available")
		return
	}
	t.Logf("primary reader: %s %v", cmd, args)
}

func TestTextInjector(t *testing.T) {
	s := env.Detect()
	inj, ok := s.TextInjector()
	if !ok {
		t.Log("no text injector available")
		return
	}
	t.Logf("text injector: %s", inj)
}
