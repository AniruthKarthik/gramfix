// Package env detects the runtime desktop environment, display protocol,
// and availability of required helper tools.
package env

import (
	"os"
	"os/exec"
	"strings"
)

// Session describes the detected environment.
type Session struct {
	IsWayland bool
	IsX11     bool

	// Clipboard
	HasWlCopy  bool
	HasWlPaste bool
	HasXclip   bool
	HasXsel    bool

	// Injector
	HasWtype      bool
	HasYdotool    bool
	HasXdotool    bool
	YdotoolSocket string // path to ydotoold daemon socket

	// Hotkey
	HasXbindkeys bool
	HasSxhkd     bool

	// Display
	WaylandDisplay string
	X11Display     string

	// Distro
	Distro string
}

// Detect inspects the runtime environment and returns a Session.
func Detect() *Session {
	s := &Session{}

	s.WaylandDisplay = os.Getenv("WAYLAND_DISPLAY")
	s.X11Display = os.Getenv("DISPLAY")
	sessionType := strings.ToLower(os.Getenv("XDG_SESSION_TYPE"))

	if s.WaylandDisplay != "" || sessionType == "wayland" {
		s.IsWayland = true
	}
	if s.X11Display != "" || sessionType == "x11" {
		s.IsX11 = true
	}
	// In XWayland, both can be set; prefer Wayland
	if s.IsWayland && s.IsX11 {
		s.IsX11 = false
	}

	// Probe tools
	s.HasWlCopy = hasCmd("wl-copy")
	s.HasWlPaste = hasCmd("wl-paste")
	s.HasXclip = hasCmd("xclip")
	s.HasXsel = hasCmd("xsel")
	s.HasWtype = hasCmd("wtype")
	s.HasYdotool = hasCmd("ydotool")
	s.HasXdotool = hasCmd("xdotool")
	s.HasXbindkeys = hasCmd("xbindkeys")
	s.HasSxhkd = hasCmd("sxhkd")
	s.YdotoolSocket = detectYdotoolSocket()

	s.Distro = detectDistro()

	return s
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// detectYdotoolSocket finds the ydotoold socket path.
func detectYdotoolSocket() string {
	if s := os.Getenv("YDOTOOL_SOCKET"); s != "" {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	for _, p := range []string{"/tmp/.ydotool_socket", "/run/ydotool.socket"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func detectDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			return strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), "\""))
		}
	}
	return "unknown"
}

// ClipboardReader returns the best available clipboard read command args.
func (s *Session) ClipboardReader() (string, []string, bool) {
	if s.IsWayland && s.HasWlPaste {
		return "wl-paste", []string{"--no-newline"}, true
	}
	if s.HasXclip {
		return "xclip", []string{"-selection", "clipboard", "-o"}, true
	}
	if s.HasXsel {
		return "xsel", []string{"--clipboard", "--output"}, true
	}
	return "", nil, false
}

// PrimaryReader returns the best available primary-selection (selected text) reader.
func (s *Session) PrimaryReader() (string, []string, bool) {
	if s.IsWayland && s.HasWlPaste {
		return "wl-paste", []string{"--no-newline", "--primary"}, true
	}
	if s.HasXclip {
		return "xclip", []string{"-selection", "primary", "-o"}, true
	}
	if s.HasXsel {
		return "xsel", []string{"--primary", "--output"}, true
	}
	return "", nil, false
}

// ClipboardWriter returns the best available clipboard write command args.
func (s *Session) ClipboardWriter() (string, []string, bool) {
	if s.IsWayland && s.HasWlCopy {
		return "wl-copy", []string{}, true
	}
	if s.HasXclip {
		return "xclip", []string{"-selection", "clipboard", "-i"}, true
	}
	if s.HasXsel {
		return "xsel", []string{"--clipboard", "--input"}, true
	}
	return "", nil, false
}

// PrimaryWriter returns the best available primary-selection writer.
func (s *Session) PrimaryWriter() (string, []string, bool) {
	if s.IsWayland && s.HasWlCopy {
		return "wl-copy", []string{"--primary"}, true
	}
	if s.HasXclip {
		return "xclip", []string{"-selection", "primary", "-i"}, true
	}
	if s.HasXsel {
		return "xsel", []string{"--primary", "--input"}, true
	}
	return "", nil, false
}

// TextInjector returns the best available text injection tool.
// Priority: ydotool (uinput-based, works everywhere) → wtype (Wayland) → xdotool (X11)
func (s *Session) TextInjector() (string, bool) {
	// ydotool works on Wayland and X11 via uinput — preferred when daemon is running
	if s.HasYdotool && s.YdotoolSocket != "" {
		return "ydotool", true
	}
	if s.IsWayland && s.HasWtype {
		return "wtype", true
	}
	if s.HasXdotool {
		return "xdotool", true
	}
	return "", false
}

// Summary returns a human-readable summary of the detected environment.
func (s *Session) Summary() string {
	var sb strings.Builder
	if s.IsWayland {
		sb.WriteString("display=wayland")
	} else {
		sb.WriteString("display=x11")
	}
	sb.WriteString(" distro=" + s.Distro)
	if s.YdotoolSocket != "" {
		sb.WriteString(" ydotool=ok")
	}
	return sb.String()
}
