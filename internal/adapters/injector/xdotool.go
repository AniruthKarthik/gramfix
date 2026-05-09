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
	cmd := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+v")
	return cmd.Run()
}

func (x *XdotoolInjector) SendCopy() error {
	cmd := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+c")
	return cmd.Run()
}
