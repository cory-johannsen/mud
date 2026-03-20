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
	require.NoError(t, s.Apply("testuid", def, 2, 3))
	// frightened 2 = 2 stacks × 1 penalty = -2
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestAttackBonus_Prone_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2}
	require.NoError(t, s.Apply("testuid", def, 1, -1))
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestACBonus_NoConditions_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.ACBonus(s))
}

func TestACBonus_FlatFooted_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2}
	require.NoError(t, s.Apply("testuid", def, 1, 1))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestACBonus_Frightened2_MinusTwoToAC(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, ACPenalty: 1}
	require.NoError(t, s.Apply("testuid", def, 2, 3))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestIsActionRestricted_Stunned_BlocksAttack(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3, RestrictActions: []string{"attack", "strike"}}
	require.NoError(t, s.Apply("testuid", def, 1, 2))
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
	require.NoError(t, s.Apply("testuid", def, 2, 1))
	assert.Equal(t, 2, condition.StunnedAPReduction(s))
}

func TestStunnedAPReduction_NoStunned_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.StunnedAPReduction(s))
}

func TestAttackBonus_NilActiveSet_ReturnsZero(t *testing.T) {
	result := condition.AttackBonus(nil)
	if result != 0 {
		t.Errorf("expected 0 for nil ActiveSet, got %d", result)
	}
}

func TestAttackBonus_WithBonus_ReturnsPositive(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "aided", Name: "Aided", DurationType: "rounds", MaxStacks: 0, AttackBonus: 2}
	require.NoError(t, s.Apply("testuid", def, 1, 1))
	assert.Equal(t, 2, condition.AttackBonus(s))
}

func TestAttackBonus_PenaltyOnly_ReturnsNegative(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "feared", Name: "Feared", DurationType: "rounds", MaxStacks: 0, AttackPenalty: 1}
	require.NoError(t, s.Apply("testuid", def, 1, 1))
	assert.Equal(t, -1, condition.AttackBonus(s))
}

func TestPropertyAttackBonus_PenaltyOnlyIsNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, AttackPenalty: penalty}
		require.NoError(t, s.Apply("testuid", def, stacks, -1))
		bonus := condition.AttackBonus(s)
		assert.LessOrEqual(t, bonus, 0, "AttackBonus with only penalties must be <= 0")
	})
}

func TestPropertyACBonus_AlwaysNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, ACPenalty: penalty}
		require.NoError(t, s.Apply("testuid", def, stacks, -1))
		bonus := condition.ACBonus(s)
		assert.LessOrEqual(t, bonus, 0, "ACBonus must always be <= 0")
	})
}

func TestDamageBonus_ZeroWhenNoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.DamageBonus(s))
}

func TestDamageBonus_AppliedCondition(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{
		ID:           "surge",
		Name:         "Surge",
		DamageBonus:  3,
		DurationType: "encounter",
		MaxStacks:    0,
	}
	err := s.Apply("uid1", def, 1, -1)
	assert.NoError(t, err)
	assert.Equal(t, 3, condition.DamageBonus(s))
}

func TestProperty_DamageBonus_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		bonus := rapid.IntRange(-10, 10).Draw(rt, "bonus")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{
			ID: "test", Name: "Test", DamageBonus: bonus,
			DurationType: "permanent", MaxStacks: 0,
		}
		_ = s.Apply("uid", def, 1, -1)
		got := condition.DamageBonus(s)
		assert.GreaterOrEqual(t, got, 0, "DamageBonus should be non-negative")
	})
}

func TestReflexBonus(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bonus := rapid.IntRange(-5, 10).Draw(t, "bonus")
		def := &condition.ConditionDef{ID: "test_cover", ReflexBonus: bonus}
		s := condition.NewActiveSet()
		_ = s.Apply("uid", def, 1, -1)
		got := condition.ReflexBonus(s)
		want := bonus
		if want < 0 {
			want = 0
		}
		if got != want {
			t.Errorf("ReflexBonus: got %d, want %d (bonus=%d)", got, want, bonus)
		}
	})
}

func TestStealthBonus(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bonus := rapid.IntRange(-5, 10).Draw(t, "bonus")
		def := &condition.ConditionDef{ID: "test_cover", StealthBonus: bonus}
		s := condition.NewActiveSet()
		_ = s.Apply("uid", def, 1, -1)
		got := condition.StealthBonus(s)
		want := bonus
		if want < 0 {
			want = 0
		}
		if got != want {
			t.Errorf("StealthBonus: got %d, want %d (bonus=%d)", got, want, bonus)
		}
	})
}

func TestReflexBonusNoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	if got := condition.ReflexBonus(s); got != 0 {
		t.Errorf("empty set: got %d, want 0", got)
	}
}

func TestStealthBonusNoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	if got := condition.StealthBonus(s); got != 0 {
		t.Errorf("empty set: got %d, want 0", got)
	}
}

func TestAPReduction_NoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.APReduction(s))
}

func TestAPReduction_WithCondition(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "test_ap", APReduction: 2, DurationType: "rounds"}
	require.NoError(t, s.Apply("uid", def, 1, 3))
	assert.Equal(t, 2, condition.APReduction(s))
}

func TestSkipTurn_False(t *testing.T) {
	s := condition.NewActiveSet()
	assert.False(t, condition.SkipTurn(s))
}

func TestSkipTurn_True(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "test_skip", SkipTurn: true, DurationType: "rounds"}
	require.NoError(t, s.Apply("uid", def, 1, 3))
	assert.True(t, condition.SkipTurn(s))
}

func TestSkillPenalty_NoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.SkillPenalty(s))
}

func TestSkillPenalty_WithCondition(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "test_skill", SkillPenalty: 2, DurationType: "rounds"}
	require.NoError(t, s.Apply("uid", def, 1, 3))
	assert.Equal(t, 2, condition.SkillPenalty(s))
}

func TestForcedActionType_NoConditions(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, "", condition.ForcedActionType(s))
}

func TestForcedActionType_WithForcedCondition(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{
		ID:           "fear_panicked",
		Name:         "Panicked",
		ForcedAction: "random_attack",
		DurationType: "rounds",
	}
	require.NoError(t, s.Apply("uid", def, 1, 3))
	assert.Equal(t, "random_attack", condition.ForcedActionType(s))
}

func TestForcedActionType_NilSet(t *testing.T) {
	assert.Equal(t, "", condition.ForcedActionType(nil))
}

func TestForcedActionType_MultipleConditions_ReturnsNonEmpty(t *testing.T) {
	s := condition.NewActiveSet()
	def1 := &condition.ConditionDef{ID: "c1", ForcedAction: "random_attack", DurationType: "rounds"}
	def2 := &condition.ConditionDef{ID: "c2", ForcedAction: "lowest_hp_attack", DurationType: "rounds"}
	require.NoError(t, s.Apply("uid", def1, 1, 3))
	require.NoError(t, s.Apply("uid", def2, 1, 3))
	got := condition.ForcedActionType(s)
	assert.True(t, got == "random_attack" || got == "lowest_hp_attack", "expected a non-empty forced action type, got %q", got)
}

func TestProperty_ForcedActionType_AlwaysValidOrEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		forcedAction := rapid.SampledFrom([]string{"", "random_attack", "lowest_hp_attack"}).Draw(rt, "forced_action")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test_forced", ForcedAction: forcedAction, DurationType: "rounds"}
		require.NoError(t, s.Apply("uid", def, 1, 3))
		got := condition.ForcedActionType(s)
		valid := got == "" || got == "random_attack" || got == "lowest_hp_attack"
		assert.True(t, valid, "ForcedActionType returned unexpected value %q", got)
	})
}
