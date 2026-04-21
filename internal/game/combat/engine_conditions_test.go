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

// TestStartRoundWithSrc_ResetsACModAndAttackMod verifies that ACMod and AttackMod
// are zeroed on all living combatants at the start of each round.
//
// Precondition: Combatants have non-zero ACMod and AttackMod set from a prior round.
// Postcondition: StartRoundWithSrc zeros ACMod and AttackMod on all living combatants.
func TestStartRoundWithSrc_ResetsACModAndAttackMod(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)

	// Simulate mods applied during the previous round.
	cbt.Combatants[0].ACMod = 2
	cbt.Combatants[0].AttackMod = -1
	cbt.Combatants[1].ACMod = 1
	cbt.Combatants[1].AttackMod = 3

	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})

	assert.Equal(t, 0, cbt.Combatants[0].ACMod, "ACMod must be zeroed at round start")
	assert.Equal(t, 0, cbt.Combatants[0].AttackMod, "AttackMod must be zeroed at round start")
	assert.Equal(t, 0, cbt.Combatants[1].ACMod, "ACMod must be zeroed at round start for NPC")
	assert.Equal(t, 0, cbt.Combatants[1].AttackMod, "AttackMod must be zeroed at round start for NPC")
}

// TestProperty_StartRound_AlwaysResetsACModAndAttackMod verifies that no matter
// what ACMod/AttackMod values are set, StartRoundWithSrc always zeros them.
//
// Precondition: Combatants may have arbitrary non-zero ACMod/AttackMod values.
// Postcondition: After StartRoundWithSrc, ACMod == 0 and AttackMod == 0 for all living combatants.
func TestProperty_StartRound_AlwaysResetsACModAndAttackMod(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		_, cbt := makeCombatWithConditions(t)
		for _, c := range cbt.Combatants {
			c.ACMod = rapid.Int().Draw(rt, "acmod")
			c.AttackMod = rapid.Int().Draw(rt, "attackmod")
		}
		_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
		for _, c := range cbt.Combatants {
			assert.Equal(rt, 0, c.ACMod, "ACMod must be 0 after StartRound")
			assert.Equal(rt, 0, c.AttackMod, "AttackMod must be 0 after StartRound")
		}
	})
}

// TestStartRoundWithSrc_ProneDeductsOneAP verifies that a combatant starting a
// round while prone pays 1 AP as the stand-up cost, matching the Prone condition
// specification (prone.yaml) and the critical-miss narrative in round.go.
//
// Precondition: p1 is prone when StartRoundWithSrc is called with 3 AP.
// Postcondition: p1's ActionQueue has 3 - 1 = 2 AP remaining.
func TestStartRoundWithSrc_ProneDeductsOneAP(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	q, ok := cbt.ActionQueues["p1"]
	require.True(t, ok)
	assert.Equal(t, 2, q.RemainingPoints(), "3 AP - 1 stand-up = 2 remaining")
}

// TestStartRoundWithSrc_ProneClearsCondition verifies that once the stand-up AP
// cost is paid, the prone condition is removed from the combatant so they act
// upright for the remainder of the round.
//
// Precondition: p1 is prone when StartRoundWithSrc is called.
// Postcondition: p1 no longer has the prone condition.
func TestStartRoundWithSrc_ProneClearsCondition(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	assert.False(t, cbt.HasCondition("p1", "prone"),
		"prone must be cleared after paying the round-start stand-up cost")
}

// TestStartRoundWithSrc_ProneEmitsRemovalEvent verifies that standing up from
// prone at round start produces a RoundConditionEvent with Applied=false so
// clients render the stand-up narrative.
//
// Postcondition: events contain a removal entry for the "prone" condition on p1.
func TestStartRoundWithSrc_ProneEmitsRemovalEvent(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	events := cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})

	found := false
	for _, ev := range events {
		if ev.UID == "p1" && ev.ConditionID == "prone" && !ev.Applied {
			found = true
			break
		}
	}
	assert.True(t, found,
		"expected prone removal event for p1 after round-start stand-up; got %+v", events)
}

// TestStartRoundWithSrc_NotProne_NoDeduction verifies that combatants without
// the prone condition receive the full actions-per-round AP allotment.
//
// Postcondition: p1 is not prone; p1's ActionQueue has actionsPerRound remaining.
func TestStartRoundWithSrc_NotProne_NoDeduction(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	q, ok := cbt.ActionQueues["p1"]
	require.True(t, ok)
	assert.Equal(t, 3, q.RemainingPoints(), "no prone, no stand-up cost")
}

// TestStartRoundWithSrc_ProneAndStunnedStackAP verifies that the prone stand-up
// cost composes additively with stunned AP reduction.
//
// Precondition: p1 has both prone (1 AP) and stunned (2 stacks, 2 AP reduction).
// Postcondition: p1's ActionQueue has 4 - 1 - 2 = 1 AP remaining; prone cleared;
// stunned retained (stunned is ticked, not cleared, by the stand-up logic).
func TestStartRoundWithSrc_ProneAndStunnedStackAP(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	require.NoError(t, cbt.ApplyCondition("p1", "stunned", 2, 2))
	_ = cbt.StartRoundWithSrc(4, &fixedSrc{val: 0})
	q, ok := cbt.ActionQueues["p1"]
	require.True(t, ok)
	assert.Equal(t, 1, q.RemainingPoints(), "4 AP - 1 prone - 2 stunned = 1 remaining")
	assert.False(t, cbt.HasCondition("p1", "prone"), "prone cleared after stand-up")
}

// TestProperty_ProneStandUp_APNeverNegative verifies that no combination of
// prone + stunned stacks produces a negative AP balance; AP is clamped to 0.
//
// Postcondition: For any actionsPerRound in [0,5] and stunned stacks in [0,3],
// the resulting ActionQueue RemainingPoints is >= 0.
func TestProperty_ProneStandUp_APNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		actionsPerRound := rapid.IntRange(0, 5).Draw(rt, "ap")
		stunStacks := rapid.IntRange(0, 3).Draw(rt, "stun")
		applyProne := rapid.Bool().Draw(rt, "prone")

		_, cbt := makeCombatWithConditions(t)
		if applyProne {
			require.NoError(rt, cbt.ApplyCondition("p1", "prone", 1, -1))
		}
		if stunStacks > 0 {
			require.NoError(rt, cbt.ApplyCondition("p1", "stunned", stunStacks, 2))
		}

		_ = cbt.StartRoundWithSrc(actionsPerRound, &fixedSrc{val: 0})
		q, ok := cbt.ActionQueues["p1"]
		require.True(rt, ok)
		assert.GreaterOrEqual(rt, q.RemainingPoints(), 0,
			"AP must never be negative regardless of condition stacks")
	})
}
