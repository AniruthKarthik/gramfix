package clipboard

import (
	"os"
	"os/exec"
	"strings"
)

type WaylandClipboard struct{}

func (w *WaylandClipboard) IsAvailable() bool {
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if waylandDisplay == "" && sessionType != "wayland" {
		return false
	}
	_, err := exec.LookPath("wl-paste")
	if err != nil {
		return false
	}
	_, err = exec.LookPath("wl-copy")
	return err == nil
}

func (w *WaylandClipboard) GetPrimary() (string, error) {
	cmd := exec.Command("wl-paste", "-p")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (w *WaylandClipboard) GetClipboard() (string, error) {
	cmd := exec.Command("wl-paste")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (w *WaylandClipboard) SetClipboard(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
