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
