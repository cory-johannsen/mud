package combat_test

import (
	"fmt"

	"testing"

	"github.com/stretchr/testify/assert"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// hookCombatConditionRegistry builds a Registry with the four conditions used in hook tests.
// Precondition: none.
// Postcondition: Returns a non-nil Registry with prone, flat_footed, dying, wounded registered.
func hookCombatConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	for _, id := range []string{"prone", "flat_footed", "dying", "wounded"} {
		reg.Register(&condition.ConditionDef{
			ID: id, Name: id, DurationType: "permanent", MaxStacks: 4,
		})
	}
	return reg
}

// startHookCombat creates a Combat with player p1 (Alice, AC 10) and NPC n1 (Bob, AC 30).
// Bob's AC 30 ensures normal level-1 rolls almost always miss without hook intervention.
// Precondition: mgr must be non-nil and have "room1" zone loaded.
// Postcondition: Returns a non-nil Combat ready for action queuing.
func startHookCombat(t *testing.T, mgr *scripting.Manager) *combat.Combat {
	t.Helper()
	engine := combat.NewEngine()
	cbt, err := engine.StartCombat("room1",
		[]*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 100, CurrentHP: 100, AC: 10, Level: 1},
			{ID: "n1", Kind: combat.KindNPC, Name: "Bob", MaxHP: 100, CurrentHP: 100, AC: 30, Level: 1},
		},
		hookCombatConditionRegistry(),
		mgr,
		"room1",
	)
	require.NoError(t, err)
	return cbt
}

// TestResolveRound_AttackRollHook_ForcesHit verifies that returning 999 from on_attack_roll
// forces a hit against AC 30, which a level-1 roll of 5 (fixedSrc{val:5}) would never achieve.
// Postcondition: At least one attack event for p1 must have Outcome Success or CritSuccess.
func TestResolveRound_AttackRollHook_ForcesHit(t *testing.T) {
	mgr := newScriptMgr(t, `
		function on_attack_roll(attacker_uid, target_uid, roll_total, ac)
			return 999
		end
	`)
	cbt := startHookCombat(t, mgr)
	// Use StartRoundWithSrc so initiative rolls don't use nil.
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 5})

	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Bob"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)
	require.NotEmpty(t, events)

	hitFound := false
	for _, e := range events {
		if e.ActorID == "p1" && e.AttackResult != nil {
			assert.True(t,
				e.AttackResult.Outcome == combat.Success || e.AttackResult.Outcome == combat.CritSuccess,
				"on_attack_roll returning 999 must force a hit or crit; got outcome %v", e.AttackResult.Outcome,
			)
			hitFound = true
		}
	}
	assert.True(t, hitFound, "expected an attack event with AttackResult for p1")
}

// TestResolveRound_DamageRollHook_OverridesDamage verifies that returning 50 from on_damage_roll
// causes exactly 50 HP of damage to Bob regardless of the natural roll.
// Postcondition: Bob's CurrentHP must equal 50 after the round.
func TestResolveRound_DamageRollHook_OverridesDamage(t *testing.T) {
	mgr := newScriptMgr(t, `
		function on_attack_roll(attacker_uid, target_uid, roll_total, ac)
			return 999
		end
		function on_damage_roll(attacker_uid, target_uid, damage)
			return 50
		end
	`)
	cbt := startHookCombat(t, mgr)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 5})

	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Bob"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)

	for _, c := range cbt.Combatants {
		if c.ID == "n1" {
			assert.Equal(t, 50, c.CurrentHP,
				"on_damage_roll returning 50 must result in Bob having exactly 50 HP remaining")
		}
	}
}

// TestResolveRound_ConditionApplyHook_CancelsCondition verifies that returning false from
// on_condition_apply prevents the flat_footed condition from being applied on a CritSuccess.
// A CritSuccess requires roll_total >= AC + 10; with hook override 999 and AC 30, outcome is CritSuccess.
// Postcondition: Bob must NOT have the flat_footed condition after the round.
func TestResolveRound_ConditionApplyHook_CancelsCondition(t *testing.T) {
	mgr := newScriptMgr(t, `
		function on_attack_roll(attacker_uid, target_uid, roll_total, ac)
			return 999
		end
		function on_condition_apply(uid, cond_id, stacks)
			return false
		end
	`)
	cbt := startHookCombat(t, mgr)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 5})

	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Bob"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)

	assert.False(t, cbt.HasCondition("n1", "flat_footed"),
		"on_condition_apply returning false must cancel flat_footed application")
}

// TestProperty_AttackRollHook_OutcomeMatchesHookValue verifies that the outcome produced by
// ResolveRound matches OutcomeFor(hookVal, 30) for every hookVal in [1,50].
// Precondition: on_attack_roll hook always returns hookVal; Bob's AC is 30.
// Postcondition: Outcome for p1's attack event must equal OutcomeFor(hookVal, 30).
func TestProperty_AttackRollHook_OutcomeMatchesHookValue(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hookVal := rapid.IntRange(1, 50).Draw(rt, "hookVal")
		luaSrc := fmt.Sprintf(`function on_attack_roll(a, b, roll, ac) return %d end`, hookVal)
		mgr := newScriptMgr(t, luaSrc)
		cbt := startHookCombat(t, mgr)
		cbt.StartRound(3)
		require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Bob"}))
		require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

		events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)

		// Bob's AC = 30; hookVal in [1,50], so hookVal < 30 = miss, hookVal >= 30 = hit
		for _, e := range events {
			if e.ActorID == "p1" && e.AttackResult != nil {
				expectedOutcome := combat.OutcomeFor(hookVal, 30)
				assert.Equal(t, expectedOutcome, e.AttackResult.Outcome,
					"outcome must match OutcomeFor(hookVal=%d, ac=30)", hookVal)
			}
		}
	})
}
