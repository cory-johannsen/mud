package inventory

import (
	"fmt"
	"strings"
)

const (
	// RoundsPerClip is the number of base-unit Rounds in one Clip.
	RoundsPerClip = 25
	// RoundsPerCrate is the number of base-unit Rounds in one Crate (20 Clips).
	RoundsPerCrate = 500
)

// DecomposeRounds converts a total round count into display tiers.
//
// Precondition: total >= 0.
// Postcondition: crates*500 + clips*25 + rounds == total; 0 <= clips < 20; 0 <= rounds < 25.
func DecomposeRounds(total int) (crates, clips, rounds int) {
	crates = total / RoundsPerCrate
	remainder := total % RoundsPerCrate
	clips = remainder / RoundsPerClip
	rounds = remainder % RoundsPerClip
	return crates, clips, rounds
}

// FormatRounds returns a human-readable currency string for the given total rounds.
//
// Precondition: total >= 0.
// Postcondition: returned string uses singular/plural forms and omits zero-valued
// higher tiers (except Rounds, which always appears).
func FormatRounds(total int) string {
	crates, clips, rounds := DecomposeRounds(total)

	var parts []string
	if crates > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", crates, plural(crates, "Crate")))
	}
	if clips > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", clips, plural(clips, "Clip")))
	}
	parts = append(parts, fmt.Sprintf("%d %s", rounds, plural(rounds, "Round")))

	return strings.Join(parts, ", ")
}

// plural returns the singular form if n == 1, otherwise appends "s".
func plural(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}
