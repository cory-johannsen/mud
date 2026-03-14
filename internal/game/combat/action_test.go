package combat_test

import (
	"math/rand"
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

func TestActionUseAbility_Cost(t *testing.T) {
	tests := []struct {
		cost     int
		expected int
	}{
		{1, 1},
		{2, 2},
		{3, 3},
	}
	for _, tt := range tests {
		qa := combat.QueuedAction{Type: combat.ActionUseAbility, AbilityID: "surge", AbilityCost: tt.cost}
		q := combat.NewActionQueue("player1", 3)
		if err := q.Enqueue(qa); err != nil {
			t.Errorf("cost=%d: unexpected Enqueue error: %v", tt.cost, err)
		}
		if q.RemainingPoints() != 3-tt.cost {
			t.Errorf("cost=%d: remaining=%d, want %d", tt.cost, q.RemainingPoints(), 3-tt.cost)
		}
	}
}

func TestActionUseAbility_InsufficientAP(t *testing.T) {
	q := combat.NewActionQueue("player1", 1)
	qa := combat.QueuedAction{Type: combat.ActionUseAbility, AbilityID: "surge", AbilityCost: 2}
	if err := q.Enqueue(qa); err == nil {
		t.Error("expected insufficient AP error, got nil")
	}
}

func TestActionUseAbility_String(t *testing.T) {
	if combat.ActionUseAbility.String() != "use_ability" {
		t.Errorf("String(): got %q, want %q", combat.ActionUseAbility.String(), "use_ability")
	}
}

func TestPropertyActionUseAbility_RemainingNeverNegative(t *testing.T) {
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		startingPoints := rand.Intn(5) + 1 // 1..5
		cost := rand.Intn(startingPoints+2) // 0..startingPoints+1 (may exceed)
		q := combat.NewActionQueue("prop-test", startingPoints)
		qa := combat.QueuedAction{
			Type:        combat.ActionUseAbility,
			AbilityID:   "test_ability",
			AbilityCost: cost,
		}
		err := q.Enqueue(qa)
		if err != nil {
			// Enqueue failed (cost > remaining); remaining must be unchanged.
			if q.RemainingPoints() != startingPoints {
				t.Errorf("iter %d: failed enqueue left remaining=%d, want %d",
					i, q.RemainingPoints(), startingPoints)
			}
			continue
		}
		// Enqueue succeeded; remaining must equal startingPoints - cost, and >= 0.
		want := startingPoints - cost
		if q.RemainingPoints() != want {
			t.Errorf("iter %d: remaining=%d, want %d", i, q.RemainingPoints(), want)
		}
		if q.RemainingPoints() < 0 {
			t.Errorf("iter %d: remaining went negative: %d", i, q.RemainingPoints())
		}
	}
}

func TestActionQueue_DeductAP_Success(t *testing.T) {
	q := combat.NewActionQueue("u1", 3)
	err := q.DeductAP(1)
	require.NoError(t, err)
	assert.Equal(t, 2, q.RemainingPoints())
}

func TestActionQueue_DeductAP_InsufficientAP(t *testing.T) {
	q := combat.NewActionQueue("u1", 1)
	err := q.DeductAP(2)
	require.Error(t, err)
	assert.Equal(t, 1, q.RemainingPoints(), "AP must not change on failure")
}

func TestActionQueue_DeductAP_ZeroCost_ReturnsError(t *testing.T) {
	q := combat.NewActionQueue("u1", 3)
	err := q.DeductAP(0)
	require.Error(t, err)
	assert.Equal(t, 3, q.RemainingPoints(), "AP must not change on zero-cost call")
}

func TestActionQueue_DeductAP_NegativeCost_ReturnsError(t *testing.T) {
	q := combat.NewActionQueue("u1", 3)
	err := q.DeductAP(-1)
	require.Error(t, err)
	assert.Equal(t, 3, q.RemainingPoints(), "AP must not change on negative-cost call")
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

func TestClearActions_EmptyQueue(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestClearActions_AfterEnqueue(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"})
	require.NoError(t, err)
	require.Greater(t, len(q.QueuedActions()), 0)

	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

func TestClearActions_AfterPass(t *testing.T) {
	q := combat.NewActionQueue("uid1", 3)
	err := q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
	require.NoError(t, err)
	require.True(t, q.IsSubmitted())

	q.ClearActions()
	assert.Equal(t, 0, len(q.QueuedActions()))
	assert.Equal(t, q.MaxPoints, q.RemainingPoints())
	assert.False(t, q.IsSubmitted())
}

// TestActionQueue_AddAP_IncreasesRemaining verifies AddAP adds to remaining AP.
//
// Precondition: queue with 2 remaining; AddAP(1) called.
// Postcondition: RemainingPoints() == 3.
func TestActionQueue_AddAP_IncreasesRemaining(t *testing.T) {
	q := combat.NewActionQueue("u1", 3)
	_ = q.DeductAP(1) // remaining = 2
	q.AddAP(1)
	assert.Equal(t, 3, q.RemainingPoints())
}

// TestActionQueue_AddAP_Zero_NoChange verifies AddAP(0) is a no-op.
//
// Precondition: queue with 3 remaining; AddAP(0).
// Postcondition: RemainingPoints() == 3.
func TestActionQueue_AddAP_Zero_NoChange(t *testing.T) {
	q := combat.NewActionQueue("u1", 3)
	q.AddAP(0)
	assert.Equal(t, 3, q.RemainingPoints())
}

func TestProperty_ClearActions_AlwaysRestoresMaxPoints(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxAP := rapid.IntRange(1, 5).Draw(rt, "maxAP")
		q := combat.NewActionQueue("uid", maxAP)
		if rapid.Bool().Draw(rt, "enqueue_pass") {
			_ = q.Enqueue(combat.QueuedAction{Type: combat.ActionPass})
		}
		q.ClearActions()
		assert.Equal(rt, 0, len(q.QueuedActions()), "queue must be empty after ClearActions")
		assert.Equal(rt, maxAP, q.RemainingPoints(), "remaining must equal MaxPoints after ClearActions")
		assert.False(rt, q.IsSubmitted(), "IsSubmitted must be false after ClearActions")
	})
}
