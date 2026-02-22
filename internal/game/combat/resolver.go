package combat

// AttackResult holds the outcome of a single attack action.
type AttackResult struct {
	// AttackerID is the attacking combatant's ID.
	AttackerID string
	// TargetID is the defending combatant's ID.
	TargetID string
	// AttackRoll is the raw d20 result before modifiers.
	AttackRoll int
	// AttackTotal is the full attack roll: d20 + modifiers.
	AttackTotal int
	// Outcome is the PF2E 4-tier result.
	Outcome Outcome
	// BaseDamage is the raw damage roll + STR modifier.
	BaseDamage int
	// DamageRoll holds the individual die values.
	DamageRoll []int
}

// EffectiveDamage returns the damage dealt after applying the outcome multiplier.
//
// Postcondition: Returns >= 0.
func (r AttackResult) EffectiveDamage() int {
	switch r.Outcome {
	case CritSuccess:
		return r.BaseDamage * 2
	case Success:
		return r.BaseDamage
	default:
		return 0
	}
}

// Source is the subset of dice.Source used by the resolver.
// Using a local interface avoids a circular import.
type Source interface {
	Intn(n int) int
}

// ResolveAttack performs a full attack roll and damage roll for attacker vs target.
// Attack roll: d20 + STR modifier + proficiency bonus vs target AC.
// Damage: 1d6 + STR modifier (unarmed baseline; weapons come in Stage 7).
//
// Precondition: attacker and target must be non-nil and not dead; src must be non-nil.
// Postcondition: Returns a fully populated AttackResult.
func ResolveAttack(attacker, target *Combatant, src Source) AttackResult {
	// Attack roll: d20 + STR modifier + proficiency bonus
	d20 := src.Intn(20) + 1
	atkMod := attacker.StrMod + ProficiencyBonus(attacker.Level)
	atkTotal := d20 + atkMod
	outcome := OutcomeFor(atkTotal, target.AC)

	// Damage roll: 1d6 + STR modifier (unarmed baseline)
	dmgDie := src.Intn(6) + 1
	strMod := attacker.StrMod
	if strMod < 0 {
		strMod = 0
	}
	baseDmg := dmgDie + strMod

	return AttackResult{
		AttackerID:  attacker.ID,
		TargetID:    target.ID,
		AttackRoll:  d20,
		AttackTotal: atkTotal,
		Outcome:     outcome,
		BaseDamage:  baseDmg,
		DamageRoll:  []int{dmgDie},
	}
}
