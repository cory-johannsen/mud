package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestActionType_Cost(t *testing.T) {
	assert.Equal(t, 0, combat.ActionUnknown.Cost())
	assert.Equal(t, 1, combat.ActionAttack.Cost())
	assert.Equal(t, 2, combat.ActionStrike.Cost())
	assert.Equal(t, 0, combat.ActionPass.Cost())
}

func TestActionType_String(t *testing.T) {
	assert.Equal(t, "unknown", combat.ActionUnknown.String())
	assert.Equal(t, "attack", combat.ActionAttack.String())
	assert.Equal(t, "strike", combat.ActionStrike.String())
	assert.Equal(t, "pass", combat.ActionPass.String())
}

func TestActionQueue_Enqueue_Success(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)
	require.Equal(t, 3, q.RemainingPoints())

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 2, q.RemainingPoints())
	actions := q.QueuedActions()
	assert.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	assert.Equal(t, "goblin", actions[0].Target)
}

func TestActionQueue_Enqueue_InsufficientAP(t *testing.T) {
	q := combat.NewActionQueue("player1", 1)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStrike, Target: "goblin"})
	require.Error(t, err)
	assert.Equal(t, 1, q.RemainingPoints())
	assert.Empty(t, q.QueuedActions())
}

func TestActionQueue_Enqueue_RejectsActionUnknown(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionUnknown})
	require.Error(t, err)
	assert.Equal(t, 3, q.RemainingPoints())
	assert.Empty(t, q.QueuedActions())
}

func TestActionQueue_IsSubmitted_AfterPass(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	// AP remaining but pass was queued — submitted
	assert.Equal(t, 0, q.RemainingPoints())
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_FullSpend(t *testing.T) {
	q := combat.NewActionQueue("player1", 2)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStrike, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 0, q.RemainingPoints())
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_NotYet(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 2, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestActionQueue_HasPoints(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)
	assert.True(t, q.HasPoints())

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	assert.False(t, q.HasPoints())
}

func TestPropertyActionQueue_RemainingNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxPoints := rapid.IntRange(1, 6).Draw(rt, "maxPoints")
		q := combat.NewActionQueue("player1", maxPoints)

		actionTypes := []combat.ActionType{combat.ActionAttack, combat.ActionStrike, combat.ActionPass}
		numActions := rapid.IntRange(0, 10).Draw(rt, "numActions")

		for i := 0; i < numActions; i++ {
			if q.IsSubmitted() {
				break
			}
			at := actionTypes[rapid.IntRange(0, 2).Draw(rt, "actionType")]
			target := ""
			if at != combat.ActionPass {
				target = "goblin"
			}

			prevRemaining := q.RemainingPoints()
			prevLen := len(q.QueuedActions())

			err := q.Enqueue(combat.QueuedAction{Type: at, Target: target})

			// Postcondition: remaining is never negative.
			assert.GreaterOrEqual(rt, q.RemainingPoints(), 0, "RemainingPoints must never be negative")

			if err == nil {
				// Success-path postconditions:
				// 1. Queue length increased by exactly 1.
				assert.Equal(rt, prevLen+1, len(q.QueuedActions()), "QueuedActions length must increase by 1 on success")
				// 2. RemainingPoints decreased by exactly the action cost
				//    (for ActionPass the cost is 0 but remaining is set to 0, so use direct check).
				if at == combat.ActionPass {
					assert.Equal(rt, 0, q.RemainingPoints(), "RemainingPoints must be 0 after ActionPass")
				} else {
					assert.Equal(rt, prevRemaining-at.Cost(), q.RemainingPoints(), "RemainingPoints must decrease by action cost on success")
				}
			}
		}
	})
}
