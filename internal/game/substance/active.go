package substance

import "time"

// ActiveSubstance tracks one consumed substance entry in a player session.
type ActiveSubstance struct {
	SubstanceID    string
	DoseCount      int
	OnsetAt        time.Time
	ExpiresAt      time.Time
	EffectsApplied bool
}

// SubstanceAddiction tracks the addiction state for one substance per player session.
type SubstanceAddiction struct {
	// Status is "" (clean), "at_risk", "addicted", or "withdrawal".
	Status          string
	WithdrawalUntil time.Time
}
