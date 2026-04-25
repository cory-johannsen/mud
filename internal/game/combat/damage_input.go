package combat

// BuildDamageOpts carries the inputs for assembling a DamageInput at a call site.
type BuildDamageOpts struct {
	Actor             *Combatant
	Target            *Combatant
	AttackResult      AttackResult
	ConditionDmgBonus int            // from condition.DamageBonus(...) or effect.Resolve(actor.Effects, StatDamage).Total
	WeaponModBonus    int            // from weaponModifierDamageBonus(actor)
	ExtraDiceRolled   int            // pre-rolled extra weapon dice value (caller rolls with src)
	PassiveFeatBonus  int            // from applyPassiveFeats return value
	Halvers           []DamageHalver // tech halvers (e.g. from save success)
}

// BuildDamageInput assembles a DamageInput from round.go call-site data.
// On a miss (Failure or CritFailure), BaseDamage is 0 from the resolver, producing a
// zero-damage input which ResolveDamage correctly resolves to 0.
//
// Precondition: opts.Actor, opts.Target, opts.AttackResult must be set.
// Postcondition: returns a fully populated DamageInput.
func BuildDamageInput(opts BuildDamageOpts) DamageInput {
	r := opts.AttackResult
	target := opts.Target

	var additives []DamageAdditive
	additives = append(additives, DamageAdditive{
		Label:  "dice + mods",
		Value:  r.BaseDamage,
		Source: "attack:base",
	})
	if opts.WeaponModBonus != 0 {
		additives = append(additives, DamageAdditive{
			Label:  "weapon modifier",
			Value:  opts.WeaponModBonus,
			Source: "item:weapon_modifier",
		})
	}
	if opts.ConditionDmgBonus != 0 {
		additives = append(additives, DamageAdditive{
			Label:  "condition bonus",
			Value:  opts.ConditionDmgBonus,
			Source: "condition:damage",
		})
	}
	if opts.ExtraDiceRolled != 0 {
		additives = append(additives, DamageAdditive{
			Label:  "extra weapon dice",
			Value:  opts.ExtraDiceRolled,
			Source: "feat:extra_dice",
		})
	}
	if opts.PassiveFeatBonus != 0 {
		additives = append(additives, DamageAdditive{
			Label:  "passive feat",
			Value:  opts.PassiveFeatBonus,
			Source: "feat:passive",
		})
	}

	var multipliers []DamageMultiplier
	if r.Outcome == CritSuccess {
		multipliers = append(multipliers, DamageMultiplier{
			Label:  "critical hit",
			Factor: 2.0,
			Source: "engine:crit",
		})
	}

	halvers := opts.Halvers

	weakness := 0
	resistance := 0
	if r.DamageType != "" && target != nil {
		if target.Weaknesses != nil {
			weakness = target.Weaknesses[r.DamageType]
		}
		if target.Resistances != nil {
			resistance = target.Resistances[r.DamageType]
		}
	}

	return DamageInput{
		Additives:   additives,
		Multipliers: multipliers,
		Halvers:     halvers,
		DamageType:  r.DamageType,
		Weakness:    weakness,
		Resistance:  resistance,
	}
}
