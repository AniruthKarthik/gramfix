package grammar

// categoryConfidence maps LanguageTool rule category IDs to a 0-100
// confidence score.  Only matches whose category scores at or above the
// engine's configured threshold are applied.
//
// Scores are based on empirical false-positive rates from the LT community:
//
//	100 - extremely reliable  (clear spelling errors, unambiguous grammar)
//	 80 - highly reliable     (contraction errors, apostrophe, agreement)
//	 60 - reliable            (confused words, punctuation)
//	 40 - moderate            (style and redundancy suggestions)
//	 20 - noisy               (typography, casing preferences)
var categoryConfidence = map[string]int{
	"TYPOS":            100,
	"GRAMMAR":           85,
	"CASING":            80,
	"COMPOUNDING":       80,
	"CONFUSED_WORDS":    70,
	"PUNCTUATION":       65,
	"SEMANTICS":         60,
	"COLLOQUIALISMS":    55,
	"REDUNDANCY":        40,
	"STYLE":             35,
	"TYPOGRAPHY":        20,
	"TEXT_ANALYSIS":     30,
	"AMERICAN_ENGLISH":  60,
	"BRITISH_ENGLISH":   60,
	"CREATIVE_WRITING":  15,
	"PLAIN_ENGLISH":     15,
	// GramFix custom categories
	"GRAMFIX_MODAL":       98,
	"GRAMFIX_THEIR_THEYRE": 90,
	"GRAMFIX_SV_AGREEMENT": 80,
	"GRAMFIX_SPELLING":     95,
	"GRAMFIX_CONFUSION":    75,
	"GRAMFIX_GENERAL":      85,
}

// ruleConfidenceOverride lets specific rule IDs override their category score.
// Use this to promote high-value rules or demote known noisy ones.
var ruleConfidenceOverride = map[string]int{
	// Spelling
	"MORFOLOGIK_RULE_EN_US": 100,
	"MORFOLOGIK_RULE_EN_GB": 100,

	// Contractions: dont/cant/wont/wouldnt
	"EN_CONTRACTION_SPELLING": 98,

	// it's vs its
	"IT_IS":       97,
	"IT_IS_2":     97,
	"ITS_TO_IT_S": 95,

	// their/they're/there
	"THEIR_THEYRE":     95,
	"THEIR_THERE":      95,
	"THERE_THEIR":      95,
	"CONFUSION_OF_ME_I": 90,

	// Subject-verb agreement
	"AGREEMENT_SENT_START":       90,
	"EN_SUBJECT_VERB_AGREEMENT":  85,

	// Double comparative
	"MOST_COMPARATIVE": 90,

	// Irregular verb forms
	"EN_IRREGULAR_VERBS": 90,

	// Fake adverbs
	"FASTLY": 88,

	// Repeated words
	"ENGLISH_WORD_REPEAT_RULE":           95,
	"ENGLISH_WORD_REPEAT_BEGINNING_RULE": 90,

	// Sentence fragment
	"SENTENCE_FRAGMENT": 80,

	// a vs an
	"EN_A_VS_AN": 92,

	// Modal + of -> modal + have
	"MODAL_OF":  98,
	"COULD_OF":  98,
	"SHOULD_OF": 98,
	"WOULD_OF":  98,

	// Custom rules: always apply
	"GRAMFIX_COULD_OF":    98,
	"GRAMFIX_SHOULD_OF":   98,
	"GRAMFIX_WOULD_OF":    98,
	"GRAMFIX_MUST_OF":     98,
	"GRAMFIX_MIGHT_OF":    98,
	"GRAMFIX_THEIR_VERB":  88,
	"GRAMFIX_ALOT":        95,
	"GRAMFIX_IRREGARDLESS": 95,
	"GRAMFIX_SUPPOSABLY":  90,
	"GRAMFIX_DEFINATELY":  95,
	"GRAMFIX_OCCURANCE":   95,
	"GRAMFIX_OCCURED":     95,
	"GRAMFIX_RECIEVE":     95,
	"GRAMFIX_SEPERATE":    95,
	"GRAMFIX_UNTILL":      95,
	"GRAMFIX_ANALYSE_EN_US": 85,
	"GRAMFIX_CHILDRENS":   90,
	"GRAMFIX_INFORMATIONS": 90,
	"GRAMFIX_ADVISE_NOUN": 85,
	"GRAMFIX_EFFECT_VERB": 80,
	"GRAMFIX_EVERYDAY_ADV": 80,

	// Everyday vs every day (built-in)
	"EVERYDAY_EVERY_DAY": 85,
	"COULD_CARE_LESS":    85,

	// Demote: known to fire on technical writing
	"UPPERCASE_SENTENCE_START":     5,
	"WORD_CONTAINS_UPPERCASE":      5,
	"EN_QUOTES":                   10,
	"DASH_RULE":                   10,
	"UNLIKELY_OPENING_PUNCTUATION": 15,
	"ARROWS":                       15,
	"WHITESPACE_RULE":              10,

	// Style-only
	"SENT_START_CONJUNCTIVE_LINKING_ADVERB_COMMA": 15,
	"COMMA_PARENTHESIS_WHITESPACE":                20,
	"FOR_FREE":                                    40,
	"DIFFERENT_THAN":                              45,
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
