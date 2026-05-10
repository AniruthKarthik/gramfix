// Package grammar – normalize.go
// normalizeForLT cleans the input text before it is written to the temp
// file and sent to LanguageTool.  The goal is to improve tokenisation
// accuracy without mutating the *original* text that will be patched and
// re-injected (offsets are adjusted by the caller before patching).
package grammar

import "strings"

// normalizeForLT returns a copy of text prepared for LanguageTool:
//   - CRLF / lone CR → LF   (consistent line endings)
//   - Smart quotes   → straight quotes  (LT handles both, but straight
//     quotes produce more stable UTF-16 offsets for BMP-only inputs)
//   - No-break space → regular space
//
// The function also returns an offsetMap that translates byte positions in
// the normalised string back to byte positions in the original string.
// For now this map is nil when the byte lengths are identical (common case).
func normalizeForLT(text string) (normalized string, sameOffsets bool) {
	r := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\u00a0", " ",  // non-breaking space
		"\u2018", "'",  // left single quotation mark
		"\u2019", "'",  // right single quotation mark
		"\u201c", "\"", // left double quotation mark
		"\u201d", "\"", // right double quotation mark
		"\u2013", "-",  // en-dash
		"\u2014", "--", // em-dash → two hyphens (same byte length as "--")
	)
	out := r.Replace(text)

	// em-dash replacement changes byte length (3 bytes → 2 bytes), so offsets
	// diverge.  When there are no em-dashes the byte lengths match.
	sameOffsets = len(out) == len(text)
	return out, sameOffsets
}
