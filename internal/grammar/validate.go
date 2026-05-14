package grammar

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// validateCorrection checks that the corrected text is a plausible minimal
// edit of the original.  It is a safety net — not a quality gate — meant to
// catch edge cases such as:
//   - LT completely erasing the text
//   - Offset bugs that produce garbled output
//   - Unexpectedly large rewrites
//
// Returns a non-nil error if the correction should be rejected.
func validateCorrection(original, corrected string) error {
	origRunes := utf8.RuneCountInString(original)
	corrRunes := utf8.RuneCountInString(corrected)

	// Guard: corrected must not be empty when original was non-empty
	if strings.TrimSpace(corrected) == "" && strings.TrimSpace(original) != "" {
		return fmt.Errorf("correction produced empty output (original: %d chars)", origRunes)
	}

	// Guard: length must stay within a factor of 2 in either direction.
	// Grammar corrections are minimal edits, not paraphrases.
	if origRunes > 0 {
		ratio := float64(corrRunes) / float64(origRunes)
		if ratio < 0.5 {
			return fmt.Errorf("corrected text is %.0f%% of original length (min 50%%)", ratio*100)
		}
		if ratio > 2.0 {
			return fmt.Errorf("corrected text is %.0f%% of original length (max 200%%)", ratio*100)
		}
	}

	// Guard: corrected must share a meaningful common prefix or suffix with
	// the original (ensures it is the same document, not a total replacement).
	// We only enforce this for texts longer than 20 chars.
	if origRunes > 20 && corrRunes > 20 {
		if commonPrefixRunes(original, corrected) == 0 && commonSuffixRunes(original, corrected) == 0 {
			return fmt.Errorf("corrected text shares no prefix or suffix with original (likely offset corruption)")
		}
	}

	return nil
}

func commonPrefixRunes(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	n := len(ra)
	if len(rb) < n {
		n = len(rb)
	}
	for i := 0; i < n; i++ {
		if ra[i] != rb[i] {
			return i
		}
	}
	return n
}

func commonSuffixRunes(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	la, lb := len(ra), len(rb)
	n := la
	if lb < n {
		n = lb
	}
	count := 0
	for i := 1; i <= n; i++ {
		if ra[la-i] != rb[lb-i] {
			break
		}
		count++
	}
	return count
}
