package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func makeCombatForRoundConditions(t *testing.T) *combat.Combat {
	t.Helper()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 12, CurrentHP: 12, AC: 12, Level: 1, StrMod: 1, DexMod: 0, Initiative: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeConditionReg(), nil, "")
	require.NoError(t, err)
	return cbt
}

func TestResolveRound_CritFailure_ProneApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Force crit failure: set Ganger AC very high so attacker always crits fail
	cbt.Combatants[1].AC = 30 // AC-10=20; any roll <= 20 is at most Failure; need roll < AC-10=20 for CritFailure
	// With AC=30: CritFailure when atkTotal < 30-10=20. Intn(20)=0 → d20=1, mods=4 → total=5 < 20 → CritFailure
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 0}, nil)
	assert.True(t, cbt.HasCondition("p1", "prone"), "attacker must be prone after crit failure")
}

func TestResolveRound_CritSuccess_FlatFootedApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Force crit success: AC=12; need atkTotal >= 12+10=22. Intn(20)=19 → d20=20, mods=4 → total=24 >= 22
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	assert.True(t, cbt.HasCondition("n1", "flat_footed"), "target must be flat-footed after crit success")
}

func TestResolveRound_PlayerZeroHP_DyingApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Force player to die: low HP, low AC
	cbt.Combatants[0].CurrentHP = 1
	cbt.Combatants[0].AC = 1 // guarantee any hit kills
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}))
	// Intn(20)=19 → d20=20, NPC mods=3 → total=23 vs AC=1 → CritSuccess (23 >= 1+10=11)
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	assert.True(t, cbt.HasCondition("p1", "dying"), "player at 0 HP must get dying condition")
	assert.False(t, cbt.Combatants[0].IsDead(), "player must NOT be immediately dead — dying chain handles this")
}

func TestResolveRound_NPCZeroHP_NoDyingCondition(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	cbt.Combatants[1].CurrentHP = 1
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	// Intn(20)=19 → crit success on Ganger → double damage kills it
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	assert.True(t, cbt.Combatants[1].IsDead(), "NPC must die at 0 HP")
	assert.False(t, cbt.HasCondition("n1", "dying"), "NPCs must NOT get dying condition — they just die")
}

func TestResolveRound_PlayerZeroHP_DyingStacksIncludeWounded(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Pre-apply wounded 2 to player
	require.NoError(t, cbt.ApplyCondition("p1", "wounded", 2, -1))
	cbt.Combatants[0].CurrentHP = 1
	cbt.Combatants[0].AC = 1 // guarantee hit
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}))
	// Intn(20)=19 → crit success → double damage kills player
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	assert.True(t, cbt.HasCondition("p1", "dying"))
	assert.Equal(t, 3, cbt.DyingStacks("p1"), "dying stacks must be 1 + wounded(2) = 3")
}

func TestResolveRound_AttackModifiers_ProneReducesRoll(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1)) // -2 attack
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	// Intn(20)=5 → d20=6, base mods = StrMod(2) + Prof(2) = 4, prone=-2 → total = 6+4-2 = 8
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)
	var attackEvent *combat.RoundEvent
	for i := range events {
		if events[i].AttackResult != nil && events[i].ActorID == "p1" {
			attackEvent = &events[i]
			break
		}
	}
	require.NotNil(t, attackEvent, "attack event must be present")
	assert.Equal(t, 8, attackEvent.AttackResult.AttackTotal, "attack total must include prone penalty of -2")
}
