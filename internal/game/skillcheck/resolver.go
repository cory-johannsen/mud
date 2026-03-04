package skillcheck

// ProficiencyBonus returns the bonus for the given proficiency rank.
// Precondition: rank is a string (may be any value).
// Postcondition: returns a non-negative integer; unknown rank returns 0.
//
// rank values: "untrained"=0, "trained"=+2, "expert"=+4, "master"=+6, "legendary"=+8
func ProficiencyBonus(rank string) int {
	switch rank {
	case "untrained":
		return 0
	case "trained":
		return 2
	case "expert":
		return 4
	case "master":
		return 6
	case "legendary":
		return 8
	default:
		return 0
	}
}

// OutcomeFor returns the CheckOutcome tier given a roll total and DC.
// Precondition: total and dc may be any int.
// Postcondition: exactly one of CritSuccess, Success, Failure, CritFailure is returned.
//
// CritSuccess: total >= dc+10
// Success:     dc <= total < dc+10
// Failure:     dc-10 <= total < dc
// CritFailure: total < dc-10
func OutcomeFor(total, dc int) CheckOutcome {
	switch {
	case total >= dc+10:
		return CritSuccess
	case total >= dc:
		return Success
	case total >= dc-10:
		return Failure
	default:
		return CritFailure
	}
}

// Resolve performs a skill check given the raw roll, ability modifier, proficiency rank,
// difficulty class, and trigger definition.
// Precondition: roll is a d20 result (1-20); abilityMod is in range -5 to +10; rank is a valid rank string; dc >= 1.
// Postcondition: returned CheckResult.Total == roll + abilityMod + ProficiencyBonus(rank)
// and CheckResult.Outcome == OutcomeFor(Total, dc).
func Resolve(roll, abilityMod int, rank string, dc int, def TriggerDef) CheckResult {
	profBonus := ProficiencyBonus(rank)
	total := roll + abilityMod + profBonus
	return CheckResult{
		TriggerDef: def,
		Roll:       roll,
		AbilityMod: abilityMod,
		ProfBonus:  profBonus,
		Total:      total,
		Outcome:    OutcomeFor(total, dc),
	}
}
