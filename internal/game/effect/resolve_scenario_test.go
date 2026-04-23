// internal/game/effect/resolve_scenario_test.go
package effect_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/stretchr/testify/assert"
)

// Scenario: two same-type status bonuses — only the highest contributes.
func TestScenario_TwoStatusBonuses(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "heroism", SourceID: "condition:heroism", CasterUID: "kira",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
			{Stat: effect.StatGrit, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "inspire", SourceID: "condition:inspire_courage", CasterUID: "xin",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 1, r.Total)    // only one status bonus contributes
	assert.Len(t, r.Suppressed, 1) // the other is suppressed
}

// Scenario: stacking circumstance penalties — only the worst contributes.
func TestScenario_StackingCircumstancePenalties(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "wet_floor", SourceID: "env:wet_floor", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeCircumstance}},
		DurKind: effect.DurationEncounter})
	s.Apply(effect.Effect{EffectID: "blinded", SourceID: "condition:blinded", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -3, Type: effect.BonusTypeCircumstance}},
		DurKind: effect.DurationRounds, DurRemain: 2})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, -3, r.Total)   // only the worst penalty counts
	assert.Len(t, r.Suppressed, 1)
}

// Scenario: untyped bonuses from different sources all stack.
func TestScenario_UntypedAdditivity(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "haste_dmg", SourceID: "condition:haste", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatDamage, Value: 2, Type: effect.BonusTypeUntyped}},
		DurKind: effect.DurationRounds, DurRemain: 3})
	s.Apply(effect.Effect{EffectID: "rage_dmg", SourceID: "feat:rage", CasterUID: "self",
		Bonuses: []effect.Bonus{{Stat: effect.StatDamage, Value: 3, Type: effect.BonusTypeUntyped}},
		DurKind: effect.DurationEncounter})
	r := effect.Resolve(s, effect.StatDamage)
	assert.Equal(t, 5, r.Total)
	assert.Len(t, r.Contributing, 2)
	assert.Empty(t, r.Suppressed)
}

// Scenario: bonus and penalty from different typed sources — both contribute.
func TestScenario_BonusAndPenaltySameType(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "bless", SourceID: "condition:bless", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "frightened", SourceID: "condition:frightened1", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 2})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 0, r.Total) // +1 status bonus + -1 status penalty
	assert.Len(t, r.Contributing, 2)
}
