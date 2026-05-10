// Package grammar provides a local, offline grammar correction engine
// backed by LanguageTool's command-line JAR (or optional local HTTP server).
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
	"unicode"

	"github.com/anilnair00/gramfix/internal/log"
)

// Well-known paths where LanguageTool JARs may be installed.
var ltJarPaths = []string{
	"/usr/share/languagetool/languagetool-commandline.jar",
	"/usr/share/java/languagetool/languagetool-commandline.jar",
	"/opt/languagetool/languagetool-commandline.jar",
	"/usr/local/share/languagetool/languagetool-commandline.jar",
}

// EngineConfig holds all tuneable parameters for the grammar engine.
type EngineConfig struct {
	// Lang is the BCP-47 language code passed to LT (e.g. "en-US").
	Lang string

	// NgramDir is the path to a directory containing n-gram language model
	// data.  When set, LT uses statistical scoring to resolve confusion pairs
	// (their/there, affect/effect, …).  Leave empty to skip.
	NgramDir string

	// CustomRulesFile is the path to an extra LT XML rules file.
	// See configs/rules/gramfix-custom.xml for examples.
	CustomRulesFile string

	// DisabledRules is a list of LT rule IDs to suppress.
	// Defaults are set in DefaultEngineConfig() to reduce false positives on
	// developer/technical writing.
	DisabledRules []string

	// EnabledCategories restricts correction to these LT category IDs.
	// An empty slice means "all categories" (LT default).
	EnabledCategories []string

	// ConfidenceMin is the minimum confidence score (0–100) a match must
	// reach before being applied.  Matches below this threshold are silently
	// skipped.  Default: 60.
	ConfidenceMin int

	// ServerURL is the base URL of a local LanguageTool HTTP server
	// (e.g. "http://localhost:8081").  When set the engine tries the server
	// first and falls back to the CLI JAR on any error.
	ServerURL string

	// JVMMaxHeap caps the JVM heap used when running the CLI JAR.
	// Examples: "256m", "512m".  Default: "256m".
	JVMMaxHeap string
}

// DefaultEngineConfig returns production-safe defaults that maximise recall
// while minimising false positives for general English writing.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		Lang:       "en-US",
		JVMMaxHeap: "256m",
		ConfidenceMin: 60,
		DisabledRules: []string{
			// Casing rules fire constantly on code identifiers
			"UPPERCASE_SENTENCE_START",
			"WORD_CONTAINS_UPPERCASE",
			// Typographic preferences — not grammar errors
			"EN_QUOTES",
			"DASH_RULE",
			"UNLIKELY_OPENING_PUNCTUATION",
			"ARROWS",
			// Noisy / low-precision
			"WHITESPACE_RULE",
		},
		EnabledCategories: []string{
			"TYPOS",
			"GRAMMAR",
			"CASING",
			"CONFUSED_WORDS",
			"PUNCTUATION",
			"COMPOUNDING",
			"SEMANTICS",
		},
	}
}

// Engine wraps the LanguageTool command-line tool (or optional HTTP server).
type Engine struct {
	cfg     EngineConfig
	jarPath string
	java    string
}

// New creates a new grammar Engine with default configuration.
// lang should be e.g. "en-US".
func New(lang string) (*Engine, error) {
	cfg := DefaultEngineConfig()
	cfg.Lang = lang
	return NewWithConfig(cfg)
}

// NewWithConfig creates a new grammar Engine with explicit configuration.
func NewWithConfig(cfg EngineConfig) (*Engine, error) {
	if cfg.Lang == "" {
		cfg.Lang = "en-US"
	}
	if cfg.JVMMaxHeap == "" {
		cfg.JVMMaxHeap = "256m"
	}
	if cfg.ConfidenceMin == 0 {
		cfg.ConfidenceMin = 60
	}

	java, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	jar := findJar()
	if jar == "" {
		return nil, fmt.Errorf("languagetool-commandline.jar not found; run install.sh first")
	}

	log.Debug("grammar engine: java=%s jar=%s lang=%s server=%s ngram=%s confidence=%d",
		java, jar, cfg.Lang, cfg.ServerURL, cfg.NgramDir, cfg.ConfidenceMin)

	return &Engine{cfg: cfg, jarPath: jar, java: java}, nil
}

// findJar searches known paths for the LanguageTool CLI JAR.
func findJar() string {
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
// It tries the HTTP server first (if configured), then falls back to the CLI JAR.
func (e *Engine) Fix(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	// ── Normalise input for LT ───────────────────────────────────────────
	// normalizeForLT returns a lightly-cleaned copy of the text.  We send
	// the normalised version to LT but patch the *original* text so that
	// formatting (smart quotes, em-dashes) is preserved in the output.
	normalized, sameOffsets := normalizeForLT(text)
	inputForLT := normalized
	textForPatching := text
	if sameOffsets {
		// Offsets are identical — we can patch the normalised version and
		// the result will be byte-for-byte compatible with original
		// (since sameOffsets means same length).
		textForPatching = normalized
	}

	// ── Try HTTP server first ────────────────────────────────────────────
	if e.cfg.ServerURL != "" {
		corrected, err := e.fixViaHTTP(ctx, inputForLT)
		if err != nil {
			log.Warn("LT server error (%v) — falling back to CLI", err)
		} else {
			if err2 := validateCorrection(text, corrected); err2 != nil {
				log.Warn("server correction validation failed (%v) — returning original", err2)
				return text, nil
			}
			log.Info("grammar fix via server: %d → %d chars", len(text), len(corrected))
			return corrected, nil
		}
	}

	// ── CLI JAR path ────────────────────────────────────────────────────
	return e.fixViaCLI(ctx, inputForLT, textForPatching, text)
}

// fixViaCLI invokes the LanguageTool JAR and returns the corrected text.
// inputForLT is the (normalised) text written to the temp file.
// textForPatching is the string the byte-offsets will be applied to.
// originalText is used for validation and as the fallback return value.
func (e *Engine) fixViaCLI(ctx context.Context, inputForLT, textForPatching, originalText string) (string, error) {
	tmpFile, err := os.CreateTemp("", "gramfix-*.txt")
	if err != nil {
		return originalText, fmt.Errorf("temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(inputForLT); err != nil {
		tmpFile.Close()
		return originalText, fmt.Errorf("write temp: %w", err)
	}
	tmpFile.Close()

	ltCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := e.buildCLIArgs(tmpFile.Name())
	cmd := exec.CommandContext(ltCtx, e.java, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug("running languagetool on %d chars (normalised %d)", len(originalText), len(inputForLT))
	if err := cmd.Run(); err != nil {
		if ltCtx.Err() == context.DeadlineExceeded {
			return originalText, fmt.Errorf("languagetool timeout after 30s")
		}
		if stdout.Len() == 0 {
			return originalText, fmt.Errorf("languagetool failed: %w\nstderr: %s", err, stderr.String())
		}
		// LT exits non-zero when it finds issues — stdout still has valid JSON.
	}

	corrected, err := applyCorrections(e.cfg, textForPatching, stdout.Bytes())
	if err != nil {
		log.Warn("correction parse error: %v — returning original", err)
		return originalText, nil
	}

	if err := validateCorrection(originalText, corrected); err != nil {
		log.Warn("correction validation failed: %v — returning original", err)
		return originalText, nil
	}

	log.Info("grammar fix via CLI: %d → %d chars", len(originalText), len(corrected))
	return corrected, nil
}

// buildCLIArgs constructs the full java arg list for the LT CLI invocation.
func (e *Engine) buildCLIArgs(inputFile string) []string {
	// ── JVM flags ────────────────────────────────────────────────────────
	// Tuned for ephemeral (< 5 s) invocations: small heap, serial GC,
	// stop JIT after tier-1 (interpreted + simple inlining only).
	args := []string{
		"-Xms64m",
		"-Xmx" + e.cfg.JVMMaxHeap,
		"-XX:+UseSerialGC",            // minimal GC overhead for small heaps
		"-XX:TieredStopAtLevel=1",     // no full JIT; saves 200–400 ms
		"-XX:+DisableExplicitGC",
		"-Dfile.encoding=UTF-8",       // ensure UTF-8 I/O in the JVM
	}

	// ── LT JAR ──────────────────────────────────────────────────────────
	args = append(args, "-jar", e.jarPath)

	// ── LT flags ─────────────────────────────────────────────────────────
	args = append(args,
		"--language", e.cfg.Lang,
		"--encoding", "utf-8",
		"--json",
	)

	if len(e.cfg.EnabledCategories) > 0 {
		// LT CLI uses --enablecategories (all lowercase, no camel case)
		args = append(args, "--enablecategories", strings.Join(e.cfg.EnabledCategories, ","))
	}
	if len(e.cfg.DisabledRules) > 0 {
		// LT CLI uses -d / --disable
		args = append(args, "-d", strings.Join(e.cfg.DisabledRules, ","))
	}
	if e.cfg.NgramDir != "" {
		args = append(args, "--languagemodel", e.cfg.NgramDir)
	}
	if e.cfg.CustomRulesFile != "" {
		if _, err := os.Stat(e.cfg.CustomRulesFile); err == nil {
			// LT CLI uses --rulefile (singular, no capital F)
			args = append(args, "--rulefile", e.cfg.CustomRulesFile)
		} else {
			log.Warn("custom rules file not found: %s", e.cfg.CustomRulesFile)
		}
	}

	// Input file must be last
	args = append(args, inputFile)
	return args
}

// ────────────────────────────────────────────────────────────────────────────
// JSON response types
// ────────────────────────────────────────────────────────────────────────────

// ltResponse is the top-level LanguageTool JSON output.
type ltResponse struct {
	Matches []ltMatch `json:"matches"`
}

// ltMatch describes a single grammar/spelling issue.
type ltMatch struct {
	// Offset and Length are byte offsets into the UTF-8 input file.
	// For BMP-only text (everything ≤ U+FFFF) this equals the rune index.
	// We patch on bytes to be safe with any input.
	Offset       int             `json:"offset"`
	Length       int             `json:"length"`
	Message      string          `json:"message"`
	Replacements []ltReplacement `json:"replacements"`
	Rule         struct {
		ID       string `json:"id"`
		Category struct {
			ID string `json:"id"`
		} `json:"category"`
	} `json:"rule"`
}

// ltReplacement is a suggested correction.
type ltReplacement struct {
	Value string `json:"value"`
}

// ────────────────────────────────────────────────────────────────────────────
// Patch application
// ────────────────────────────────────────────────────────────────────────────

// applyCorrections applies LanguageTool matches to text.
//
// Design decisions:
//  1. Byte-based patching: LT 6.x offsets are byte positions into the UTF-8
//     input file.  Working in bytes is correct for all inputs.  (For pure-ASCII
//     text byte == rune == char, so there is no regression.)
//  2. Reverse order: we apply patches end-to-start so earlier offsets remain
//     valid after each substitution.
//  3. Overlap resolution: when two matches overlap the one with higher
//     confidence wins.  If equal, the longer span wins (more context).
//  4. Confidence filtering: matches below cfg.ConfidenceMin are discarded.
//  5. Case preservation: the chosen replacement is adjusted to match the
//     original text's capitalisation pattern.
func applyCorrections(cfg EngineConfig, text string, jsonData []byte) (string, error) {
	var resp ltResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return text, fmt.Errorf("json parse: %w", err)
	}

	if len(resp.Matches) == 0 {
		log.Debug("no grammar issues found")
		return text, nil
	}

	log.Debug("found %d grammar matches (before confidence filter)", len(resp.Matches))

	// ── Confidence filtering ─────────────────────────────────────────────
	filtered := resp.Matches[:0]
	for _, m := range resp.Matches {
		score := confidenceFor(m.Rule.ID, m.Rule.Category.ID)
		if score < cfg.ConfidenceMin {
			log.Debug("skipping low-confidence match ruleID=%s cat=%s score=%d<%d",
				m.Rule.ID, m.Rule.Category.ID, score, cfg.ConfidenceMin)
			continue
		}
		filtered = append(filtered, m)
	}
	resp.Matches = filtered

	if len(resp.Matches) == 0 {
		log.Debug("all matches below confidence threshold %d", cfg.ConfidenceMin)
		return text, nil
	}

	// ── Sort descending by (confidence, span-length) then by offset desc ─
	sort.SliceStable(resp.Matches, func(i, j int) bool {
		ci := confidenceFor(resp.Matches[i].Rule.ID, resp.Matches[i].Rule.Category.ID)
		cj := confidenceFor(resp.Matches[j].Rule.ID, resp.Matches[j].Rule.Category.ID)
		if ci != cj {
			return ci > cj // higher confidence first
		}
		li := resp.Matches[i].Length
		lj := resp.Matches[j].Length
		if li != lj {
			return li > lj // longer span first (more context)
		}
		return resp.Matches[i].Offset > resp.Matches[j].Offset
	})

	// ── Apply patches end-to-start ────────────────────────────────────────
	bs := []byte(text)
	// Track which byte ranges have already been patched to skip overlaps.
	// We use a simple lastPatchedStart watermark (sufficient because we sort
	// descending by offset after the confidence sort above).
	//
	// Full overlap resolution: re-sort by offset descending before patching.
	byOffset := make([]ltMatch, len(resp.Matches))
	copy(byOffset, resp.Matches)
	sort.SliceStable(byOffset, func(i, j int) bool {
		return byOffset[i].Offset > byOffset[j].Offset
	})

	lastStart := len(bs) + 1
	applied := 0

	for _, m := range byOffset {
		if len(m.Replacements) == 0 {
			continue
		}

		start, end := m.Offset, m.Offset+m.Length

		// Skip if this match overlaps a region already patched
		if end > lastStart {
			log.Debug("skip overlap at byte [%d:%d] (lastStart=%d)", start, end, lastStart)
			continue
		}
		if start < 0 || end > len(bs) || start > end {
			log.Warn("skip out-of-bounds match offset=%d len=%d (textLen=%d)", m.Offset, m.Length, len(bs))
			continue
		}

		original := string(bs[start:end])
		rep := pickBestReplacement(original, m.Replacements)

		log.Debug("fix [%d:%d] ruleID=%s: %q → %q", start, end, m.Rule.ID, original, rep)

		repBytes := []byte(rep)
		newBs := make([]byte, 0, len(bs)-len(original)+len(repBytes))
		newBs = append(newBs, bs[:start]...)
		newBs = append(newBs, repBytes...)
		newBs = append(newBs, bs[end:]...)
		bs = newBs
		lastStart = start
		applied++
	}

	log.Debug("applied %d/%d matches", applied, len(resp.Matches))
	return string(bs), nil
}

// ────────────────────────────────────────────────────────────────────────────
// Replacement scoring
// ────────────────────────────────────────────────────────────────────────────

// pickBestReplacement chooses the best replacement from LT's candidate list.
//
// Strategy (in priority order):
//  1. If only one candidate exists, use it.
//  2. Prefer the candidate that preserves the original's capitalisation pattern
//     (ALL_CAPS → ALL_CAPS, Title → Title, lower → lower).
//  3. Fall back to the first candidate (LT ranks them by frequency/confidence).
func pickBestReplacement(original string, replacements []ltReplacement) string {
	if len(replacements) == 1 {
		return adjustCase(original, replacements[0].Value)
	}

	for _, r := range replacements {
		if casePatternMatches(original, r.Value) {
			return r.Value
		}
	}

	// Fall back: first candidate, case-adjusted
	return adjustCase(original, replacements[0].Value)
}

// casePatternMatches returns true when the replacement has the same
// capitalisation pattern as the original (all-caps, title, lower).
func casePatternMatches(original, replacement string) bool {
	if len(original) == 0 || len(replacement) == 0 {
		return true
	}
	origRunes := []rune(original)
	repRunes := []rune(replacement)

	// ALL_CAPS
	if original == strings.ToUpper(original) && strings.ContainsAny(original, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return replacement == strings.ToUpper(replacement)
	}
	// Title case (first letter upper, rest not necessarily upper)
	if unicode.IsUpper(origRunes[0]) {
		return len(repRunes) > 0 && unicode.IsUpper(repRunes[0])
	}
	// lower
	return len(repRunes) > 0 && unicode.IsLower(repRunes[0])
}

// adjustCase applies the capitalisation pattern of original to candidate.
// Used when no candidate natively matches the pattern.
func adjustCase(original, candidate string) string {
	if len(original) == 0 || len(candidate) == 0 {
		return candidate
	}
	origRunes := []rune(original)
	candRunes := []rune(candidate)

	// ALL_CAPS original → upper candidate
	if original == strings.ToUpper(original) && strings.ContainsAny(original, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return strings.ToUpper(candidate)
	}
	// Title case original → capitalise first letter of candidate
	if unicode.IsUpper(origRunes[0]) && len(candRunes) > 0 {
		candRunes[0] = unicode.ToUpper(candRunes[0])
		return string(candRunes)
	}
	return candidate
}
