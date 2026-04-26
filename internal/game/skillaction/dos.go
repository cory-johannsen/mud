package skillaction

// DoS computes the four-tier degree-of-success result from a d20 roll, an
// additive bonus, and a target DC, applying the PF2E natural-1 / natural-20
// step rule per spec NCA-7.
//
// Bands are:
//   total >= dc+10 → CritSuccess
//   total >= dc    → Success
//   total >= dc-10 → Failure
//   total <  dc-10 → CritFailure
//
// Then:
//   nat 20 bumps the result up one step (capped at CritSuccess);
//   nat 1  bumps the result down one step (capped at CritFailure).
//
// Precondition: roll is the raw d20 value (typically 1..20); bonus and dc may be any int.
// Postcondition: returned value is one of {CritSuccess, Success, Failure, CritFailure}.
func DoS(roll, bonus, dc int) DegreeOfSuccess {
	total := roll + bonus
	band := computeBand(total, dc) // 2 = critSuccess, 1 = success, 0 = failure, -1 = critFailure
	switch roll {
	case 20:
		if band < 2 {
			band++
		}
	case 1:
		if band > -1 {
			band--
		}
	}
	return bandToDoS(band)
}

func computeBand(total, dc int) int {
	switch {
	case total >= dc+10:
		return 2
	case total >= dc:
		return 1
	case total >= dc-10:
		return 0
	default:
		return -1
	}
}

func bandToDoS(b int) DegreeOfSuccess {
	switch b {
	case 2:
		return CritSuccess
	case 1:
		return Success
	case 0:
		return Failure
	default:
		return CritFailure
	}
}
