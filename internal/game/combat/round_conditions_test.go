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
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1, Initiative: 15, WeaponProficiencyRank: "trained"},
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
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 0}, nil, nil)
	assert.True(t, cbt.HasCondition("p1", "prone"), "attacker must be prone after crit failure")
}

func TestResolveRound_CritSuccess_FlatFootedApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Force crit success: AC=12; need atkTotal >= 12+10=22. Intn(20)=19 → d20=20, mods=4 → total=24 >= 22
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil, nil)
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
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil, nil)
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
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil, nil)
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
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil, nil)
	assert.True(t, cbt.HasCondition("p1", "dying"))
	assert.Equal(t, 3, cbt.DyingStacks("p1"), "dying stacks must be 1 + wounded(2) = 3")
}

// TestResolveRound_FlatFooted_CritApplied_PersistsThroughNPCAction verifies
// GH #228: flat_footed applied mid-round without a "combat_start" source tag
// MUST persist through the target NPC's own action resolution in the same
// round, so subsequent attackers in the round benefit from the -2 AC.
//
// Precondition: n1 has flat_footed with default source; n1 queues ActionPass.
// Postcondition: After ResolveRound, n1 still has flat_footed.
func TestResolveRound_FlatFooted_CritApplied_PersistsThroughNPCAction(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("n1", "flat_footed", 1, 1))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 0}, nil, nil)
	assert.True(t, cbt.HasCondition("n1", "flat_footed"),
		"mid-round flat_footed (no combat_start source) must persist through the NPC's own action")
}

// TestResolveRound_FlatFooted_CombatStart_ClearedAfterNPCAction verifies that
// flat_footed tagged with Source="combat_start" (the sucker-punch window) IS
// cleared from the NPC after their first action resolves in the round.
//
// Precondition: n1 has flat_footed (duration -1) with Source "combat_start";
// n1 queues ActionPass.
// Postcondition: After ResolveRound, n1 no longer has flat_footed.
func TestResolveRound_FlatFooted_CombatStart_ClearedAfterNPCAction(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("n1", "flat_footed", 1, -1))
	cbt.Conditions["n1"].SetSource("flat_footed", "combat_start")
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 0}, nil, nil)
	assert.False(t, cbt.HasCondition("n1", "flat_footed"),
		"combat_start flat_footed must be cleared after the NPC's first action")
}

// TestStartRoundWithSrc_FlatFooted_CritApplied_ExpiresAtRoundStart verifies
// that mid-round flat_footed (duration 1, default source) is expired by Tick
// at the start of the next round — the "until target's next turn" semantic.
//
// Precondition: n1 has flat_footed (duration 1) with default source.
// Postcondition: After StartRoundWithSrc bumps the round, n1 no longer has
// flat_footed.
func TestStartRoundWithSrc_FlatFooted_CritApplied_ExpiresAtRoundStart(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("n1", "flat_footed", 1, 1))
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	assert.False(t, cbt.HasCondition("n1", "flat_footed"),
		"flat_footed with duration=1 must be expired by Tick at the next round start")
}

// TestResolveRound_CrossAction_MAP_SecondAttackAt5 verifies GH #232: a second
// Attack action in the same round receives the -5 Multiple Attack Penalty.
//
// Precondition: p1 queues two ActionAttack against the same target in one round.
// Postcondition: second attack's AttackTotal is exactly 5 less than the first.
func TestResolveRound_CrossAction_MAP_SecondAttackAt5(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil, nil)
	var first, second *combat.RoundEvent
	for i := range events {
		if events[i].AttackResult != nil && events[i].ActorID == "p1" {
			if first == nil {
				first = &events[i]
			} else if second == nil {
				second = &events[i]
				break
			}
		}
	}
	require.NotNil(t, first, "first attack event must be present")
	require.NotNil(t, second, "second attack event must be present")
	assert.Equal(t, first.AttackResult.AttackTotal-5, second.AttackResult.AttackTotal,
		"second Attack in the same round must apply the -5 MAP penalty")
}

// TestResolveRound_CrossAction_MAP_ThirdAttackAt10 verifies GH #232: the third
// attack action in the same round receives the -10 MAP penalty.
func TestResolveRound_CrossAction_MAP_ThirdAttackAt10(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil, nil)
	var attackTotals []int
	for i := range events {
		if events[i].AttackResult != nil && events[i].ActorID == "p1" {
			attackTotals = append(attackTotals, events[i].AttackResult.AttackTotal)
		}
	}
	require.Len(t, attackTotals, 3, "expected three attack events")
	assert.Equal(t, attackTotals[0]-5, attackTotals[1],
		"second Attack total must be first - 5")
	assert.Equal(t, attackTotals[0]-10, attackTotals[2],
		"third Attack total must be first - 10 (MAP capped at -10)")
}

// TestResolveRound_CrossAction_MAP_ResetsAtRoundStart verifies that the per-
// round AttacksMadeThisRound counter is cleared by StartRoundWithSrc so the
// MAP penalty does not carry across rounds.
func TestResolveRound_CrossAction_MAP_ResetsAtRoundStart(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil, nil)

	// New round — counter should reset to 0.
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil, nil)
	for i := range events {
		if events[i].AttackResult != nil && events[i].ActorID == "p1" {
			// The very first attack of a round has no MAP: its total equals the
			// first-attack total from the prior round (11 = d20(6) + mods(5)).
			assert.Equal(t, 11, events[i].AttackResult.AttackTotal,
				"MAP counter must reset at round start")
			return
		}
	}
	t.Fatal("no attack event produced in the second round")
}

func TestResolveRound_AttackModifiers_ProneReducesRoll(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1)) // -2 attack
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	// Intn(20)=5 → d20=6, base mods = StrMod(2) + CombatProficiencyBonus(1,"trained")=3 → mods=5, prone=-2 → total = 6+5-2 = 9
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil, nil)
	var attackEvent *combat.RoundEvent
	for i := range events {
		if events[i].AttackResult != nil && events[i].ActorID == "p1" {
			attackEvent = &events[i]
			break
		}
	}
	require.NotNil(t, attackEvent, "attack event must be present")
	assert.Equal(t, 9, attackEvent.AttackResult.AttackTotal, "attack total must include prone penalty of -2")
}
