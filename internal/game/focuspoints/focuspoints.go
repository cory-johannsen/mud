package focuspoints

const maxCap = 3

// Outcome represents the result of a Refocus action.
type Outcome int

const (
	OutcomeCritSuccess Outcome = iota
	OutcomeSuccess
	OutcomeFailure
	OutcomeCritFailure
)

// ComputeMax returns the maximum focus points for a character with grantCount
// focus-granting feats/abilities, capped at maxCap.
func ComputeMax(grantCount int) int {
	if grantCount > maxCap {
		return maxCap
	}
	return grantCount
}

// Spend attempts to spend one focus point. Returns the new current value and
// whether the spend succeeded (false when current == 0).
func Spend(current, max int) (int, bool) {
	if current == 0 {
		return 0, false
	}
	return current - 1, true
}

// Restore returns the new current focus points after a Refocus action with the
// given outcome. CritSuccess/Success restores to max; Failure adds 1 (capped);
// CritFailure leaves current unchanged.
func Restore(current, max int, outcome Outcome) int {
	switch outcome {
	case OutcomeCritSuccess, OutcomeSuccess:
		return max
	case OutcomeFailure:
		if current+1 > max {
			return max
		}
		return current + 1
	default:
		return current
	}
}

// Clamp ensures current does not exceed max.
func Clamp(current, max int) int {
	if current > max {
		return max
	}
	return current
}
