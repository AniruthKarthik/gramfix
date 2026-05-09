package injector

import (
	"os/exec"
)

type WtypeInjector struct{}

func (w *WtypeInjector) IsAvailable() bool {
	_, err := exec.LookPath("wtype")
	return err == nil
}

func (w *WtypeInjector) SendPaste() error {
	// wtype supports modifiers
	cmd := exec.Command("wtype", "-M", "ctrl", "v", "-m", "ctrl")
	return cmd.Run()
}
