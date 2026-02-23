package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestActionType_Cost(t *testing.T) {
	assert.Equal(t, 1, combat.ActionAttack.Cost())
	assert.Equal(t, 2, combat.ActionStrike.Cost())
	assert.Equal(t, 0, combat.ActionPass.Cost())
}

func TestActionType_String(t *testing.T) {
	assert.Equal(t, "attack", combat.ActionAttack.String())
	assert.Equal(t, "strike", combat.ActionStrike.String())
	assert.Equal(t, "pass", combat.ActionPass.String())
}

func TestActionQueue_Enqueue_Success(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)
	require.Equal(t, 3, q.Remaining)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 2, q.Remaining)
	assert.Len(t, q.Actions, 1)
	assert.Equal(t, combat.ActionAttack, q.Actions[0].Type)
	assert.Equal(t, "goblin", q.Actions[0].Target)
}

func TestActionQueue_Enqueue_InsufficientAP(t *testing.T) {
	q := combat.NewActionQueue("player1", 1)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStrike, Target: "goblin"})
	require.Error(t, err)
	assert.Equal(t, 1, q.Remaining)
	assert.Empty(t, q.Actions)
}

func TestActionQueue_IsSubmitted_AfterPass(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	// AP remaining but pass was queued — submitted
	assert.Equal(t, 0, q.Remaining)
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_FullSpend(t *testing.T) {
	q := combat.NewActionQueue("player1", 2)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStrike, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 0, q.Remaining)
	assert.True(t, q.IsSubmitted())
}

func TestActionQueue_IsSubmitted_NotYet(t *testing.T) {
	q := combat.NewActionQueue("player1", 3)
	require.NotNil(t, q)

	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "goblin"})
	require.NoError(t, err)
	assert.Equal(t, 2, q.Remaining)
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
			// ignore error — insufficient AP is a valid outcome
			_ = q.Enqueue(combat.QueuedAction{Type: at, Target: target})
			assert.GreaterOrEqual(rt, q.Remaining, 0, "Remaining must never be negative")
		}
	})
}
