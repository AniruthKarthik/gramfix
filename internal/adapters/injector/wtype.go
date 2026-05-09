package injector

import (
	"os"
	"os/exec"
)

type WtypeInjector struct{}

func (w *WtypeInjector) IsAvailable() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("XDG_SESSION_TYPE") != "wayland" {
		return false
	}
	_, err := exec.LookPath("wtype")
	return err == nil
}

func (w *WtypeInjector) SendPaste() error {
	// wtype supports modifiers
	cmd := exec.Command("wtype", "-M", "ctrl", "v", "-m", "ctrl")
	return cmd.Run()
}
