package main

import (
	"regexp"
	"strings"
)

var (
	// wordSepRE matches characters that should become word separators (underscores).
	wordSepRE = regexp.MustCompile(`[-_. ]+`)
	// nonAlphanumUnderscoreRE strips any remaining non-alphanumeric, non-underscore chars.
	nonAlphanumUnderscoreRE = regexp.MustCompile(`[^a-z0-9_]`)
	// multiUnderscoreRE collapses consecutive underscores.
	multiUnderscoreRE = regexp.MustCompile(`_+`)
)

// ToSnakeCase converts a human-readable name to a snake_case identifier.
// Hyphens, dots, underscores, and spaces are treated as word separators and
// become underscores; all other non-alphanumeric characters are removed;
// consecutive underscores are collapsed; leading/trailing underscores are trimmed.
func ToSnakeCase(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	withUnder := wordSepRE.ReplaceAllString(lower, "_")
	cleaned := nonAlphanumUnderscoreRE.ReplaceAllString(withUnder, "")
	collapsed := multiUnderscoreRE.ReplaceAllString(cleaned, "_")
	return strings.Trim(collapsed, "_")
}

var traditionSuffixes = []string{
	"_technical", "_neural", "_bio_synthetic", "_fanatic_doctrine",
}

// stripTraditionSuffix removes a known tradition suffix from id, if present.
func stripTraditionSuffix(id string) string {
	for _, s := range traditionSuffixes {
		if strings.HasSuffix(id, s) {
			return strings.TrimSuffix(id, s)
		}
	}
	return id
}

// pf2eKeywords is a deny-list of terms that indicate an un-localized PF2E name.
var pf2eKeywords = []string{
	"firebolt", "fireball", "magic missile", "telekinesis",
	"bestow curse", "mage hand", "shillelagh", "prestidigitation",
	"tongues", "scrying", "antimagic",
}

// IsPF2EFlagged returns true if name appears to be an unlocalised PF2E source name.
// REQ-TIR-PF2: derived new_id matches old_id minus tradition suffix → never localized.
//   This check only applies when old_id actually carries a tradition suffix.
// REQ-TIR-PF3: name contains a known PF2E keyword.
func IsPF2EFlagged(name, oldID string) bool {
	stripped := stripTraditionSuffix(oldID)
	if stripped != oldID && ToSnakeCase(name) == stripped {
		return true
	}
	lower := strings.ToLower(name)
	for _, kw := range pf2eKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
