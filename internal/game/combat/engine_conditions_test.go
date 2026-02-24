package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

func makeConditionReg() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	return reg
}

func makeCombatWithConditions(t *testing.T) (*combat.Engine, *combat.Combat) {
	t.Helper()
	reg := makeConditionReg()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 12, CurrentHP: 12, AC: 12, Level: 1, StrMod: 1, DexMod: 0, Initiative: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	require.NoError(t, err)
	return eng, cbt
}

func TestApplyCondition_Prone(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("p1", "prone", 1, -1)
	require.NoError(t, err)
	conds := cbt.GetConditions("p1")
	require.Len(t, conds, 1)
	assert.Equal(t, "prone", conds[0].Def.ID)
}

func TestApplyCondition_UnknownConditionID_ReturnsError(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("p1", "nonexistent", 1, -1)
	assert.Error(t, err)
}

func TestApplyCondition_UnknownUID_ReturnsError(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("nobody", "prone", 1, -1)
	assert.Error(t, err)
}

func TestRemoveCondition_RemovesIt(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	cbt.RemoveCondition("p1", "prone")
	conds := cbt.GetConditions("p1")
	assert.Empty(t, conds)
}

func TestGetConditions_EmptyByDefault(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	conds := cbt.GetConditions("p1")
	assert.Empty(t, conds)
}

func TestHasCondition_TrueAfterApply(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	assert.True(t, cbt.HasCondition("p1", "prone"))
	assert.False(t, cbt.HasCondition("p1", "frightened"))
}

func TestStartRoundWithSrc_TicksConditions(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "flat_footed", 1, 1))
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	assert.False(t, cbt.HasCondition("p1", "flat_footed"), "flat_footed with duration=1 must expire after Tick")
}

func TestStartRoundWithSrc_StunnedReducesAP(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "stunned", 2, 2))
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	q, ok := cbt.ActionQueues["p1"]
	require.True(t, ok)
	assert.Equal(t, 1, q.RemainingPoints(), "3 AP - 2 stunned = 1 remaining")
}

func TestStartRoundWithSrc_DyingRecovery_Success(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// Intn(20) returns 14 → roll = 15 → success (>= 15)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 14})
	assert.False(t, cbt.HasCondition("p1", "dying"), "dying must be removed on success")
	assert.True(t, cbt.HasCondition("p1", "wounded"), "wounded must be applied on success")
	assert.Equal(t, 1, cbt.Combatants[0].CurrentHP, "HP must be restored to 1")
}

func TestStartRoundWithSrc_DyingRecovery_Failure(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// Intn(20) returns 9 → roll = 10 → failure (< 15)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 9})
	assert.True(t, cbt.HasCondition("p1", "dying"))
	assert.Equal(t, 2, cbt.DyingStacks("p1"), "dying must advance from 1 to 2 on failure")
}

func TestStartRoundWithSrc_DyingRecovery_DyingFour_Death(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 3, -1))
	// failure advances dying 3 → dying 4 → dead
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 9})
	assert.True(t, cbt.Combatants[0].IsDead(), "dying 4 must result in death")
}

func TestStartRoundWithSrc_DyingRecovery_CritSuccess(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// Intn(20) returns 19 → roll = 20 → crit success (roll == 20)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 19})
	assert.False(t, cbt.HasCondition("p1", "dying"), "dying removed on crit success")
	assert.False(t, cbt.HasCondition("p1", "wounded"), "wounded NOT applied on crit success")
	assert.Equal(t, 1, cbt.Combatants[0].CurrentHP, "HP restored to 1 on crit success")
}

func TestPropertyDyingStacksNeverExceedFour_MultiRound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rounds := rapid.IntRange(1, 5).Draw(rt, "rounds")
		_, cbt := makeCombatWithConditions(t)
		cbt.Combatants[0].CurrentHP = 0
		require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
		// Run multiple rounds with failure rolls (val=9 → roll=10 < 15 = failure)
		for i := 0; i < rounds; i++ {
			if cbt.Combatants[0].IsDead() {
				break
			}
			_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 9})
		}
		assert.LessOrEqual(rt, cbt.DyingStacks("p1"), 4,
			"dying stacks must never exceed 4 across multiple rounds of failure")
	})
}

func TestPropertyDyingStacksNeverExceedFour(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		_, cbt := makeCombatWithConditions(t)
		stacks := rapid.IntRange(1, 4).Draw(rt, "stacks")
		cbt.Combatants[0].CurrentHP = 0
		require.NoError(rt, cbt.ApplyCondition("p1", "dying", stacks, -1))
		assert.LessOrEqual(rt, cbt.DyingStacks("p1"), 4, "dying stacks must never exceed 4")
	})
}
