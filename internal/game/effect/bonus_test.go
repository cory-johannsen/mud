package effect_test

import (
	"testing"
	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cory-johannsen/mud/internal/game/effect"
)

func TestBonusType_ValidValues(t *testing.T) {
	for _, bt := range []effect.BonusType{
		effect.BonusTypeStatus, effect.BonusTypeCircumstance,
		effect.BonusTypeItem, effect.BonusTypeUntyped,
	} {
		assert.NotEmpty(t, string(bt))
	}
}

func TestBonus_Validate_ZeroValueRejected(t *testing.T) {
	b := effect.Bonus{Stat: effect.StatAttack, Value: 0, Type: effect.BonusTypeUntyped}
	err := b.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value 0")
}

func TestBonus_Validate_ValidBonus(t *testing.T) {
	b := effect.Bonus{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}
	assert.NoError(t, b.Validate())
}

func TestBonus_Validate_ValidPenalty(t *testing.T) {
	b := effect.Bonus{Stat: effect.StatAC, Value: -2, Type: effect.BonusTypeCircumstance}
	assert.NoError(t, b.Validate())
}

func TestBonus_DefaultType_Untyped(t *testing.T) {
	b := effect.Bonus{Stat: effect.StatDamage, Value: 1}
	b.Normalise()
	assert.Equal(t, effect.BonusTypeUntyped, b.Type)
}

func TestProperty_Bonus_NonZeroAlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		v := rapid.Int().Filter(func(x int) bool { return x != 0 }).Draw(rt, "value")
		b := effect.Bonus{Stat: effect.StatAttack, Value: v, Type: effect.BonusTypeStatus}
		assert.NoError(rt, b.Validate())
	})
}

func TestStatMatches_ExactMatch(t *testing.T) {
	assert.True(t, effect.StatMatches(effect.StatAttack, effect.StatAttack))
	assert.True(t, effect.StatMatches(effect.Stat("skill:stealth"), effect.Stat("skill:stealth")))
}

func TestStatMatches_PrefixInheritance(t *testing.T) {
	// querying "skill:stealth" — a bonus to "skill" contributes
	assert.True(t, effect.StatMatches(effect.Stat("skill"), effect.Stat("skill:stealth")))
}

func TestStatMatches_NoCrossSkillInheritance(t *testing.T) {
	// "skill:savvy" does NOT contribute to "skill:stealth"
	assert.False(t, effect.StatMatches(effect.Stat("skill:savvy"), effect.Stat("skill:stealth")))
}
