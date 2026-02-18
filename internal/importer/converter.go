package importer

import "strings"

// NameToID converts a display name to a stable snake_case identifier.
//
// Postcondition: result is lowercase, contains only [a-z0-9_], and is
// idempotent (NameToID(NameToID(s)) == NameToID(s)).
func NameToID(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
