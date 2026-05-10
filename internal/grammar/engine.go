// Package grammar provides a local, offline grammar correction engine
// backed by LanguageTool's command-line JAR.
package grammar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anilnair00/gramfix/internal/log"
)

// Well-known paths where LanguageTool JARs may be installed.
var ltJarPaths = []string{
	"/usr/share/languagetool/languagetool-commandline.jar",
	"/usr/share/java/languagetool/languagetool-commandline.jar",
	"/opt/languagetool/languagetool-commandline.jar",
	"/usr/local/share/languagetool/languagetool-commandline.jar",
}

// Engine wraps the LanguageTool command-line tool.
type Engine struct {
	jarPath string
	lang    string
	java    string
}

// New creates a new grammar Engine. lang should be e.g. "en-US".
func New(lang string) (*Engine, error) {
	java, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	jar := findJar()
	if jar == "" {
		return nil, fmt.Errorf("languagetool-commandline.jar not found; run install.sh first")
	}

	log.Debug("grammar engine: java=%s jar=%s lang=%s", java, jar, lang)
	return &Engine{jarPath: jar, lang: lang, java: java}, nil
}

// findJar searches known paths for the LanguageTool CLI JAR.
func findJar() string {
	// Allow override via env var
	if p := os.Getenv("GRAMFIX_LT_JAR"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, p := range ltJarPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Glob search under /usr and /opt
	patterns := []string{
		"/usr/**/languagetool-commandline.jar",
		"/opt/**/languagetool-commandline.jar",
	}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

// Fix runs grammar correction on the provided text and returns corrected text.
// It uses LanguageTool's JSON output mode for reliable parsing.
func (e *Engine) Fix(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	// Write input to a temp file to avoid shell escaping issues
	tmpFile, err := os.CreateTemp("", "gramfix-*.txt")
	if err != nil {
		return text, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return text, fmt.Errorf("write temp: %w", err)
	}
	tmpFile.Close()

	// Run LanguageTool with JSON output
	ltCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ltCtx, e.java,
		"-jar", e.jarPath,
		"--language", e.lang,
		"--json",
		tmpFile.Name(),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug("running languagetool on %d chars", len(text))
	if err := cmd.Run(); err != nil {
		// LT exits non-zero if it finds errors; that's fine, we still get JSON
		if ltCtx.Err() == context.DeadlineExceeded {
			return text, fmt.Errorf("languagetool timeout")
		}
		// Try to parse JSON anyway
		if stdout.Len() == 0 {
			return text, fmt.Errorf("languagetool failed: %w\nstderr: %s", err, stderr.String())
		}
	}

	corrected, err := applyCorrections(text, stdout.Bytes())
	if err != nil {
		log.Warn("correction parse error: %v — returning original", err)
		return text, nil
	}

	log.Info("grammar fix: %d chars → %d chars", len(text), len(corrected))
	return corrected, nil
}

// ltResponse is the top-level LanguageTool JSON output.
type ltResponse struct {
	Matches []ltMatch `json:"matches"`
}

// ltMatch describes a single grammar/spelling issue.
type ltMatch struct {
	Offset       int          `json:"offset"`
	Length       int          `json:"length"`
	Message      string       `json:"message"`
	Replacements []ltReplacement `json:"replacements"`
}

// ltReplacement is a suggested correction.
type ltReplacement struct {
	Value string `json:"value"`
}

// applyCorrections applies LanguageTool matches to the original text.
// Matches are applied in reverse offset order to preserve positions.
func applyCorrections(text string, jsonData []byte) (string, error) {
	var resp ltResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return text, fmt.Errorf("json parse: %w", err)
	}

	if len(resp.Matches) == 0 {
		log.Debug("no grammar issues found")
		return text, nil
	}

	log.Debug("found %d grammar matches", len(resp.Matches))

	// Sort by offset descending so we apply from end to start
	sort.Slice(resp.Matches, func(i, j int) bool {
		return resp.Matches[i].Offset > resp.Matches[j].Offset
	})

	runes := []rune(text)
	for _, m := range resp.Matches {
		if len(m.Replacements) == 0 {
			continue
		}
		rep := m.Replacements[0].Value
		start := m.Offset
		end := m.Offset + m.Length

		if start < 0 || end > len(runes) || start > end {
			log.Warn("skipping out-of-bounds match offset=%d len=%d", m.Offset, m.Length)
			continue
		}

		log.Debug("fix: %q → %q", string(runes[start:end]), rep)
		repRunes := []rune(rep)
		runes = append(runes[:start], append(repRunes, runes[end:]...)...)
	}

	return string(runes), nil
}
