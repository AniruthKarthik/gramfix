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
	// 29 is left ctrl, 47 is v (keycodes might vary, but ydotool supports key combos directly)
	cmd := exec.Command("ydotool", "key", "29:1", "47:1", "47:0", "29:0")
	return cmd.Run()
}
