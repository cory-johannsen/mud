package combat

import (
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

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

// ExplosiveResult holds the damage dealt to one target by an explosive.
type ExplosiveResult struct {
	// TargetID is the ID of the target combatant.
	TargetID string
	// SaveRoll is the raw d20 result of the Reflex save.
	SaveRoll int
	// SaveTotal is the full save roll: d20 + DexMod.
	SaveTotal int
	// SaveResult is the 4-tier outcome of the save vs SaveDC.
	SaveResult Outcome
	// BaseDamage is the damage after applying the save outcome; never negative.
	BaseDamage int
}

// ResolveFirearmAttack resolves a ranged weapon attack with range-increment penalty.
//
// Precondition: attacker, target, weapon must not be nil; rangeIncrements >= 0.
// Postcondition: uses DexMod for attack bonus; penalty = rangeIncrements * 2 subtracted from AttackTotal.
func ResolveFirearmAttack(attacker, target *Combatant, weapon *inventory.WeaponDef, rangeIncrements int, src Source) AttackResult {
	// Attack roll: d20
	rawRoll := src.Intn(20) + 1

	profBonus := 2 + (attacker.Level-1)/4
	if rangeIncrements < 0 {
		rangeIncrements = 0
	}
	rangePenalty := rangeIncrements * 2
	total := rawRoll + attacker.DexMod + profBonus - rangePenalty

	// Damage roll using weapon's damage dice expression
	dmgRoll, err := dice.RollExpr(weapon.DamageDice, src)
	var dmgTotal int
	var dmgDice []int
	if err == nil {
		dmgTotal = dmgRoll.Total()
		dmgDice = dmgRoll.Dice
	}
	if dmgTotal < 0 {
		dmgTotal = 0
	}

	return AttackResult{
		AttackerID:  attacker.ID,
		TargetID:    target.ID,
		AttackRoll:  rawRoll,
		AttackTotal: total,
		Outcome:     OutcomeFor(total, target.AC),
		BaseDamage:  dmgTotal,
		DamageRoll:  dmgDice,
	}
}

// ResolveExplosive resolves an explosive against all targets.
//
// Precondition: grenade and all targets must not be nil.
// Postcondition: each target makes a Reflex save (1d20 + DexMod) vs grenade.SaveDC;
// damage scaled by save outcome; BaseDamage >= 0.
func ResolveExplosive(grenade *inventory.ExplosiveDef, targets []*Combatant, src Source) []ExplosiveResult {
	// Roll damage once for all targets.
	dmgRoll, err := dice.RollExpr(grenade.DamageDice, src)
	baseDmg := 0
	if err == nil {
		baseDmg = dmgRoll.Total()
	}
	if baseDmg < 0 {
		baseDmg = 0
	}

	results := make([]ExplosiveResult, 0, len(targets))
	for _, target := range targets {
		// Reflex save: 1d20 + DexMod vs SaveDC
		saveRaw := src.Intn(20) + 1
		saveTotal := saveRaw + target.DexMod
		saveOutcome := OutcomeFor(saveTotal, grenade.SaveDC)

		var dmg int
		switch saveOutcome {
		case CritSuccess:
			dmg = 0
		case Success:
			dmg = baseDmg / 2
		case Failure:
			dmg = baseDmg
		case CritFailure:
			dmg = baseDmg * 2
		}
		if dmg < 0 {
			dmg = 0
		}

		results = append(results, ExplosiveResult{
			TargetID:   target.ID,
			SaveRoll:   saveRaw,
			SaveTotal:  saveTotal,
			SaveResult: saveOutcome,
			BaseDamage: dmg,
		})
	}
	return results
}
