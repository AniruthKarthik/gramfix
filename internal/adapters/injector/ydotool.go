package injector

import (
	"os/exec"
)

type YdotoolInjector struct{}

func (y *YdotoolInjector) IsAvailable() bool {
	_, err := exec.LookPath("ydotool")
	return err == nil
}

func (y *YdotoolInjector) SendPaste() error {
	// 29 is left ctrl, 47 is v
	cmd := exec.Command("ydotool", "key", "29:1", "47:1", "47:0", "29:0")
	return cmd.Run()
}

func (y *YdotoolInjector) SendCopy() error {
	// 29 is left ctrl, 46 is c
	cmd := exec.Command("ydotool", "key", "29:1", "46:1", "46:0", "29:0")
	return cmd.Run()
}
