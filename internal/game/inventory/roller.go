package inventory

// Roller is the dice-rolling abstraction used by DurabilityManager and other
// game mechanics that require randomness. Implementations must be safe for
// concurrent use.
//
// All functions that require dice rolls accept a Roller to enable deterministic
// testing with stub implementations.
type Roller interface {
	// Roll evaluates a dice expression (e.g. "2d6+4", "1d6") and returns the result.
	Roll(dice string) int
	// RollD20 rolls a single d20 and returns the result in [1, 20].
	RollD20() int
	// RollFloat returns a random float64 in [0.0, 1.0) for probability checks.
	RollFloat() float64
}
