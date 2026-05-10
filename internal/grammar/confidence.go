package grammar

// categoryConfidence maps LanguageTool rule category IDs to a 0-100
// confidence score.  Only matches whose category scores at or above the
// engine's configured threshold are applied.
//
// Scores are based on empirical false-positive rates from the LT community:
//   100 – extremely reliable  (typos, clear spelling)
//    80 – highly reliable     (grammar rules, apostrophe errors)
//    60 – reliable            (confused words, punctuation)
//    40 – moderate            (redundancy, style)
//    20 – noisy               (typography, casing preferences)
var categoryConfidence = map[string]int{
	"TYPOS":                    100,
	"GRAMMAR":                   85,
	"CASING":                    80,
	"COMPOUNDING":               80,
	"CONFUSED_WORDS":            65,
	"PUNCTUATION":               65,
	"SEMANTICS":                 60,
	"COLLOQUIALISMS":            55,
	"REDUNDANCY":                40,
	"STYLE":                     20,
	"TYPOGRAPHY":                20,
	"TEXT_ANALYSIS":             30,
	"AMERICAN_ENGLISH":          50,
	"BRITISH_ENGLISH":           50,
	"CREATIVE_WRITING":          15,
	"PLAIN_ENGLISH":             15,
}

// ruleConfidenceOverride lets specific rule IDs override their category score.
// Use this to promote high-value rules or demote known noisy ones.
var ruleConfidenceOverride = map[string]int{
	// Promote: these are almost always correct
	"MORFOLOGIK_RULE_EN_US":         100, // Hunspell spelling
	"EN_CONTRACTION_SPELLING":        95,
	"AGREEMENT_SENT_START":           90,
	"SENTENCE_FRAGMENT":              80,
	"ENGLISH_WORD_REPEAT_RULE":       90,
	"COMMA_PARENTHESIS_WHITESPACE":   70,

	// Demote: known to fire on legitimate developer/technical writing
	"UPPERCASE_SENTENCE_START":        5,
	"WORD_CONTAINS_UPPERCASE":         5,
	"EN_QUOTES":                      10,
	"DASH_RULE":                      10,
	"UNLIKELY_OPENING_PUNCTUATION":   15,
	"ARROWS":                         15,
}

// confidenceFor returns the effective confidence score for a match.
func confidenceFor(ruleID, categoryID string) int {
	if v, ok := ruleConfidenceOverride[ruleID]; ok {
		return v
	}
	if v, ok := categoryConfidence[categoryID]; ok {
		return v
	}
	return 50 // unknown category — neutral
}
