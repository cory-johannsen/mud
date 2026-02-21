// Package dice provides the core randomness abstraction and roll-result types
// for the Gunchete MUD combat engine.
package dice

import "fmt"

// RollResult holds the full audit trail for a single dice roll evaluation.
//
// Postcondition: Total() == sum(Dice) + Modifier.
type RollResult struct {
	Expression string // original expression string, e.g. "2d6+3"
	Dice       []int  // individual die results before modifier
	Modifier   int    // flat modifier (may be negative)
}

// Total returns the sum of all die results plus the modifier.
//
// Postcondition: return value == sum(r.Dice) + r.Modifier.
func (r RollResult) Total() int {
	total := r.Modifier
	for _, d := range r.Dice {
		total += d
	}
	return total
}

// String returns a human-readable audit string in the format:
//
//	"2d6+3 → [4 5] +3 = 12"
//
// Precondition: r.Expression is non-empty.
func (r RollResult) String() string {
	if r.Expression == "" {
		panic("dice: RollResult.String() precondition violated: Expression must be non-empty")
	}
	diceStr := fmt.Sprintf("%v", r.Dice)
	modStr := fmt.Sprintf("%+d", r.Modifier)
	return fmt.Sprintf("%s \u2192 %s %s = %d", r.Expression, diceStr, modStr, r.Total())
}

// Source is the randomness provider for dice rolls.
//
// Implementations MUST be safe for concurrent use.
type Source interface {
	// Intn returns a non-negative random int in [0, n).
	//
	// Precondition: n > 0.
	Intn(n int) int
}
