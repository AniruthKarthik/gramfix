package injector

import (
	"os/exec"
)

type XdotoolInjector struct{}

func (x *XdotoolInjector) IsAvailable() bool {
	_, err := exec.LookPath("xdotool")
	return err == nil
}

func (x *XdotoolInjector) SendPaste() error {
	cmd := exec.Command("xdotool", "key", "ctrl+v")
	return cmd.Run()
}
