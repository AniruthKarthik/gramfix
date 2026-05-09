package clipboard

import (
	"os/exec"
	"strings"
)

type X11Clipboard struct {
	tool string // "xclip" or "xsel"
}

func (x *X11Clipboard) IsAvailable() bool {
	if _, err := exec.LookPath("xclip"); err == nil {
		x.tool = "xclip"
		return true
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		x.tool = "xsel"
		return true
	}
	return false
}

func (x *X11Clipboard) GetPrimary() (string, error) {
	var cmd *exec.Cmd
	if x.tool == "xclip" {
		cmd = exec.Command("xclip", "-o", "-selection", "primary")
	} else {
		cmd = exec.Command("xsel", "-o", "-p")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (x *X11Clipboard) GetClipboard() (string, error) {
	var cmd *exec.Cmd
	if x.tool == "xclip" {
		cmd = exec.Command("xclip", "-o", "-selection", "clipboard")
	} else {
		cmd = exec.Command("xsel", "-o", "-b")
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (x *X11Clipboard) SetClipboard(text string) error {
	var cmd *exec.Cmd
	if x.tool == "xclip" {
		cmd = exec.Command("xclip", "-selection", "clipboard")
	} else {
		cmd = exec.Command("xsel", "-i", "-b")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
