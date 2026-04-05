package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// makeFlankingCombat creates a combat with three combatants arranged for flanking tests.
// target: NPC at (5,5); attacker: player at (4,4); ally: player at (6,6).
// When ally is at (6,6), attacker and ally are in opposite quadrants relative to target → flanked.
func makeFlankingCombat(t *testing.T, allyX, allyY int) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{
			ID: "player1", Kind: combat.KindPlayer, Name: "Player",
			MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1,
			GridX: 4, GridY: 4,
		},
		{
			ID: "target", Kind: combat.KindNPC, Name: "Goblin",
			MaxHP: 20, CurrentHP: 20, AC: 15, Level: 1,
			GridX: 5, GridY: 5,
		},
		{
			ID: "player2", Kind: combat.KindPlayer, Name: "Ally",
			MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1,
			GridX: allyX, GridY: allyY,
		},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	require.NoError(t, err, "StartCombat must succeed")
	_ = cbt.StartRound(3)
	return cbt
}

// TestFlanking_AttackRollBonusApplied verifies that a flanked target gives +2 to the attacker's roll,
// visible in the narrative as "(flanking +2)" and sets RoundEvent.Flanking = true.
func TestFlanking_AttackRollBonusApplied(t *testing.T) {
	// ally at (6,6) is in the opposite quadrant from attacker at (4,4) relative to target at (5,5) → flanked.
	cbt := makeFlankingCombat(t, 6, 6)

	err := cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"})
	require.NoError(t, err, "QueueAction must succeed")
	err = cbt.QueueAction("target", combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err, "QueueAction target must succeed")
	err = cbt.QueueAction("player2", combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err, "QueueAction ally must succeed")

	// Use a fixed random source that always rolls 10 (d20=10).
	events := combat.ResolveRound(cbt, fixedSrc{val: 10}, nil, nil)

	// Find the attack event.
	var atkEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAttack && events[i].ActorID == "player1" {
			atkEvent = &events[i]
			break
		}
	}
	assert.NotNil(t, atkEvent, "expected an attack event")
	if atkEvent != nil {
		assert.Contains(t, atkEvent.Narrative, "flanking +2", "flanking bonus must appear in narrative")
		assert.True(t, atkEvent.Flanking, "RoundEvent.Flanking must be true")
	}
}

// TestFlanking_NoBonus_WhenNotFlanked verifies that a non-flanked target does not give a flanking bonus.
func TestFlanking_NoBonus_WhenNotFlanked(t *testing.T) {
	// ally at (4,6): same column-side as attacker at (4,4) relative to target at (5,5) → NOT flanked.
	// Both are left of and on vertical sides of target, not opposite quadrants.
	cbt := makeFlankingCombat(t, 4, 6)

	err := cbt.QueueAction("player1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"})
	require.NoError(t, err, "QueueAction must succeed")
	err = cbt.QueueAction("target", combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err, "QueueAction target must succeed")
	err = cbt.QueueAction("player2", combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err, "QueueAction ally must succeed")

	events := combat.ResolveRound(cbt, fixedSrc{val: 10}, nil, nil)

	var atkEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAttack && events[i].ActorID == "player1" {
			atkEvent = &events[i]
			break
		}
	}
	assert.NotNil(t, atkEvent, "expected an attack event")
	if atkEvent != nil {
		assert.NotContains(t, atkEvent.Narrative, "flanking +2", "flanking bonus must NOT appear when not flanked")
		assert.False(t, atkEvent.Flanking, "RoundEvent.Flanking must be false when not flanked")
	}
}
