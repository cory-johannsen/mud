package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

func TestAttackBonus_NoConditions_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.AttackBonus(s))
}

func TestAttackBonus_Frightened2_MinusTwoToAttack(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1}
	require.NoError(t, s.Apply(def, 2, 3))
	// frightened 2 = 2 stacks × 1 penalty = -2
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestAttackBonus_Prone_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2}
	require.NoError(t, s.Apply(def, 1, -1))
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestACBonus_NoConditions_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.ACBonus(s))
}

func TestACBonus_FlatFooted_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2}
	require.NoError(t, s.Apply(def, 1, 1))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestACBonus_Frightened2_MinusTwoToAC(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, ACPenalty: 1}
	require.NoError(t, s.Apply(def, 2, 3))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestIsActionRestricted_Stunned_BlocksAttack(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3, RestrictActions: []string{"attack", "strike"}}
	require.NoError(t, s.Apply(def, 1, 2))
	assert.True(t, condition.IsActionRestricted(s, "attack"))
	assert.True(t, condition.IsActionRestricted(s, "strike"))
	assert.False(t, condition.IsActionRestricted(s, "pass"))
}

func TestIsActionRestricted_NoConditions_False(t *testing.T) {
	s := condition.NewActiveSet()
	assert.False(t, condition.IsActionRestricted(s, "attack"))
}

func TestStunnedAPReduction_ReturnsStacks(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3}
	require.NoError(t, s.Apply(def, 2, 1))
	assert.Equal(t, 2, condition.StunnedAPReduction(s))
}

func TestStunnedAPReduction_NoStunned_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.StunnedAPReduction(s))
}

func TestPropertyAttackBonus_AlwaysNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, AttackPenalty: penalty}
		require.NoError(t, s.Apply(def, stacks, -1))
		bonus := condition.AttackBonus(s)
		assert.LessOrEqual(t, bonus, 0, "AttackBonus must always be <= 0")
	})
}

func TestPropertyACBonus_AlwaysNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, ACPenalty: penalty}
		require.NoError(t, s.Apply(def, stacks, -1))
		bonus := condition.ACBonus(s)
		assert.LessOrEqual(t, bonus, 0, "ACBonus must always be <= 0")
	})
}
