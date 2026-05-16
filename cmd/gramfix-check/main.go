// Command gramfix-check diagnoses the gramfix installation and prints a
// report of what's working and what needs attention.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AniruthKarthik/gramfix/internal/env"
	"github.com/AniruthKarthik/gramfix/internal/grammar"
)

const version = "1.0.0"

type checkResult struct {
	name   string
	ok     bool
	detail string
}

func main() {
	env.LoadDotEnv()
	fmt.Printf("gramfix-check v%s\n\n", version)

	var results []checkResult

	// ── Environment ──────────────────────────────────────────────────────
	sess := env.Detect()

	results = append(results, checkResult{
		name:   "Display server",
		ok:     sess.IsWayland || sess.IsX11,
		detail: sess.Summary(),
	})

	// ── Clipboard tools ──────────────────────────────────────────────────
	if sess.IsWayland {
		results = append(results, checkResult{
			name:   "wl-copy (Wayland clipboard)",
			ok:     sess.HasWlCopy,
			detail: which("wl-copy"),
		})
		results = append(results, checkResult{
			name:   "wl-paste (Wayland clipboard read)",
			ok:     sess.HasWlPaste,
			detail: which("wl-paste"),
		})
	}
	if sess.IsX11 || !sess.IsWayland {
		results = append(results, checkResult{
			name:   "xclip (X11 clipboard)",
			ok:     sess.HasXclip,
			detail: which("xclip"),
		})
	}

	// ── Injection tools ───────────────────────────────────────────────────
	results = append(results, checkResult{
		name:   "ydotool (key injection)",
		ok:     sess.HasYdotool,
		detail: which("ydotool"),
	})
	results = append(results, checkResult{
		name:   "ydotoold socket",
		ok:     sess.YdotoolSocket != "",
		detail: sess.YdotoolSocket,
	})
	if sess.IsWayland {
		results = append(results, checkResult{
			name:   "wtype (Wayland key injection)",
			ok:     sess.HasWtype,
			detail: which("wtype"),
		})
	}
	if !sess.IsWayland || !sess.HasYdotool {
		results = append(results, checkResult{
			name:   "xdotool (X11 key injection)",
			ok:     sess.HasXdotool,
			detail: which("xdotool"),
		})
	}

	// ── Hotkey daemons ────────────────────────────────────────────────────
	hasSxhkd := hasBin("sxhkd")
	hasXbindkeys := hasBin("xbindkeys")
	results = append(results, checkResult{
		name:   "sxhkd (hotkey daemon)",
		ok:     hasSxhkd,
		detail: which("sxhkd"),
	})
	results = append(results, checkResult{
		name:   "xbindkeys (hotkey daemon X11)",
		ok:     hasXbindkeys,
		detail: which("xbindkeys"),
	})

	// ── Java / LanguageTool ───────────────────────────────────────────────
	javaOk := hasBin("java")
	results = append(results, checkResult{
		name:   "java",
		ok:     javaOk,
		detail: javaVersion(),
	})

	_, ltErr := grammar.New("en-US")
	results = append(results, checkResult{
		name:   "LanguageTool JAR",
		ok:     ltErr == nil,
		detail: func() string {
			if ltErr != nil {
				return ltErr.Error()
			}
			return "found"
		}(),
	})

	// ── Cloud Engines ─────────────────────────────────────────────────────
	groqKey := os.Getenv("GROQ_API_KEY")
	results = append(results, checkResult{
		name:   "Groq API Key",
		ok:     groqKey != "",
		detail: func() string {
			if groqKey == "" {
				return "not set (offline-only mode)"
			}
			return "found (masked: " + maskKey(groqKey) + ")"
		}(),
	})

	orKey := os.Getenv("OPENROUTER_API_KEY")
	results = append(results, checkResult{
		name:   "OpenRouter API Key",
		ok:     orKey != "",
		detail: func() string {
			if orKey == "" {
				return "not set"
			}
			return "found (masked: " + maskKey(orKey) + ")"
		}(),
	})

	// ── gramfix binary ────────────────────────────────────────────────────
	gramfixBin, _ := exec.LookPath("gramfix")
	results = append(results, checkResult{
		name:   "gramfix binary",
		ok:     gramfixBin != "",
		detail: gramfixBin,
	})

	// ── systemd service ───────────────────────────────────────────────────
	svcActive := isServiceActive("gramfix-hotkey")
	results = append(results, checkResult{
		name:   "gramfix-hotkey service",
		ok:     svcActive,
		detail: func() string {
			if svcActive {
				return "active (running)"
			}
			return "not running — run: systemctl --user start gramfix-hotkey"
		}(),
	})

	// ── Print results ─────────────────────────────────────────────────────
	fmt.Println("Component Check:")
	fmt.Println("─────────────────────────────────────────────────────")
	passing := 0
	failing := 0
	for _, r := range results {
		icon := "✓"
		if !r.ok {
			icon = "✗"
			failing++
		} else {
			passing++
		}
		fmt.Printf("  %s  %-35s  %s\n", icon, r.name, r.detail)
	}
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("  %d passing, %d failing\n\n", passing, failing)

	if failing > 0 {
		fmt.Println("Fix failing items, then run: gramfix-check")
		fmt.Println("See docs/troubleshooting.md for help.")
		os.Exit(1)
	}

	fmt.Println("All checks passed! GramFix is ready.")
	fmt.Println("Select text anywhere and press Alt+G to fix grammar.")
}

func which(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return "not found"
	}
	return p
}

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func javaVersion() string {
	out, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return "not found"
	}
	lines := splitLines(string(out))
	if len(lines) > 0 {
		return lines[0]
	}
	return "found"
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func isServiceActive(name string) bool {
	cmd := exec.Command("systemctl", "--user", "is-active", name)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	status := ""
	for _, c := range string(out) {
		if c != '\n' && c != '\r' {
			status += string(c)
		}
	}
	return status == "active"
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
