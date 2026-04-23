// internal/game/effect/resolve_test.go
package effect_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestResolve_EmptySet_ReturnsZero(t *testing.T) {
	s := effect.NewEffectSet()
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
	assert.Empty(t, r.Contributing)
	assert.Empty(t, r.Suppressed)
}

func TestResolve_NilSet_ReturnsZero(t *testing.T) {
	r := effect.Resolve(nil, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
}

func TestResolve_SingleBonus_Contributes(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:blessed", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 2, r.Total)
	require.Len(t, r.Contributing, 1)
	assert.Nil(t, r.Contributing[0].OverriddenBy)
	assert.Empty(t, r.Suppressed)
}

func TestResolve_SameType_HighestWins(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:heroism", CasterUID: "caster1",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 3, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:inspire_courage", CasterUID: "caster2",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 3, r.Total)
	require.Len(t, r.Contributing, 1)
	require.Len(t, r.Suppressed, 1)
	assert.NotNil(t, r.Suppressed[0].OverriddenBy)
	assert.Equal(t, "e1", r.Suppressed[0].OverriddenBy.EffectID)
}

func TestResolve_UntypedAlwaysStacks(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "feat:inspire", CasterUID: "uid",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeUntyped}},
		DurKind: effect.DurationPermanent})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "feat:focus", CasterUID: "uid",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeUntyped}},
		DurKind: effect.DurationPermanent})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 3, r.Total)
	assert.Len(t, r.Contributing, 2)
	assert.Empty(t, r.Suppressed)
}

func TestResolve_MixedTypes_IndependentBuckets(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:heroism", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "item:plus1sword", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeItem}},
		DurKind: effect.DurationUntilRemove})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, 3, r.Total)
	assert.Len(t, r.Contributing, 2)
}

func TestResolve_Penalty_WorstPenaltyWins(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:frightened2", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -2, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 2})
	s.Apply(effect.Effect{EffectID: "e2", SourceID: "condition:sickened1", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationRounds, DurRemain: 1})
	r := effect.Resolve(s, effect.StatAttack)
	assert.Equal(t, -2, r.Total) // only the worst penalty contributes
	assert.Len(t, r.Contributing, 1)
	assert.Len(t, r.Suppressed, 1)
}

func TestResolve_TieBreak_LexicographicOrder(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e_b", SourceID: "condition:bless", CasterUID: "caster_b",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	s.Apply(effect.Effect{EffectID: "e_a", SourceID: "condition:aid", CasterUID: "caster_a",
		Bonuses: []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationUntilRemove})
	r := effect.Resolve(s, effect.StatAttack)
	require.Len(t, r.Contributing, 1)
	// lex winner: "condition:aid"/"caster_a" < "condition:bless"/"caster_b"
	assert.Equal(t, "e_a", r.Contributing[0].EffectID)
}

func TestResolve_Pure_IdenticalInputSameOutput(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:prone", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatAC, Value: -1, Type: effect.BonusTypeCircumstance}},
		DurKind: effect.DurationRounds, DurRemain: 2})
	r1 := effect.Resolve(s, effect.StatAC)
	r2 := effect.Resolve(s, effect.StatAC)
	assert.Equal(t, r1.Total, r2.Total)
	assert.Equal(t, len(r1.Contributing), len(r2.Contributing))
}

func TestProperty_Resolve_UntypedSumsAll(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := effect.NewEffectSet()
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		total := 0
		for i := 0; i < n; i++ {
			v := rapid.IntRange(-1000, 1000).Filter(func(x int) bool { return x != 0 }).Draw(rt, "v")
			s.Apply(effect.Effect{
				EffectID: fmt.Sprintf("e%d", i),
				SourceID: fmt.Sprintf("src%d", i),
				Bonuses:  []effect.Bonus{{Stat: effect.StatDamage, Value: v, Type: effect.BonusTypeUntyped}},
				DurKind:  effect.DurationPermanent,
			})
			total += v
		}
		r := effect.Resolve(s, effect.StatDamage)
		assert.Equal(rt, total, r.Total)
	})
}

func TestResolve_PrefixOnColon_SkillContributesToSkillStealth(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{EffectID: "e1", SourceID: "condition:skulker", CasterUID: "",
		Bonuses: []effect.Bonus{{Stat: effect.StatSkill, Value: -1, Type: effect.BonusTypeStatus}},
		DurKind: effect.DurationPermanent})
	r := effect.Resolve(s, effect.Stat("skill:stealth"))
	assert.Equal(t, -1, r.Total)
}
