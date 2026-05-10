// Package platform provides cross-distro dependency management helpers.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PackageManager describes a distro package manager.
type PackageManager struct {
	Name    string
	Install []string // command prefix for install
}

// Distros maps ID strings to package managers.
var Distros = map[string]PackageManager{
	"fedora":    {Name: "dnf", Install: []string{"sudo", "dnf", "install", "-y"}},
	"rhel":      {Name: "dnf", Install: []string{"sudo", "dnf", "install", "-y"}},
	"centos":    {Name: "dnf", Install: []string{"sudo", "dnf", "install", "-y"}},
	"ubuntu":    {Name: "apt", Install: []string{"sudo", "apt-get", "install", "-y"}},
	"debian":    {Name: "apt", Install: []string{"sudo", "apt-get", "install", "-y"}},
	"linuxmint": {Name: "apt", Install: []string{"sudo", "apt-get", "install", "-y"}},
	"pop":       {Name: "apt", Install: []string{"sudo", "apt-get", "install", "-y"}},
	"arch":      {Name: "pacman", Install: []string{"sudo", "pacman", "-S", "--noconfirm"}},
	"manjaro":   {Name: "pacman", Install: []string{"sudo", "pacman", "-S", "--noconfirm"}},
	"opensuse-leap":    {Name: "zypper", Install: []string{"sudo", "zypper", "install", "-y"}},
	"opensuse-tumbleweed": {Name: "zypper", Install: []string{"sudo", "zypper", "install", "-y"}},
}

// WaylandPackages maps distro package managers to Wayland tool packages.
var WaylandPackages = map[string][]string{
	"dnf":    {"wl-clipboard", "wtype", "ydotool"},
	"apt":    {"wl-clipboard", "wtype", "ydotool"},
	"pacman": {"wl-clipboard", "wtype", "ydotool"},
	"zypper": {"wl-clipboard", "wtype", "ydotool"},
}

// X11Packages maps package managers to X11 tool packages.
var X11Packages = map[string][]string{
	"dnf":    {"xclip", "xsel", "xdotool", "xbindkeys"},
	"apt":    {"xclip", "xsel", "xdotool", "xbindkeys"},
	"pacman": {"xclip", "xsel", "xdotool", "xbindkeys"},
	"zypper": {"xclip", "xsel", "xdotool", "xbindkeys"},
}

// DetectDistro reads /etc/os-release to determine the distro ID.
func DetectDistro() string {
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

// GetPackageManager returns the PackageManager for the current distro.
func GetPackageManager() (PackageManager, bool) {
	id := DetectDistro()
	pm, ok := Distros[id]
	return pm, ok
}

// CheckDeps verifies that required binaries are available.
// Returns list of missing commands.
func CheckDeps(cmds ...string) []string {
	var missing []string
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			missing = append(missing, c)
		}
	}
	return missing
}

// InstallPackages installs system packages using the distro package manager.
func InstallPackages(pm PackageManager, pkgs []string) error {
	args := append(pm.Install, pkgs...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install %v: %w", pkgs, err)
	}
	return nil
}
