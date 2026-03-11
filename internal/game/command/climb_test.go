package command_test

import (
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleClimb_AlwaysReturnsRequest(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleClimb(args)
		assert.NoError(rt, err)
		assert.NotNil(rt, req)
	})
}

// TestClimbOutcomes verifies 4-tier outcome thresholds for the climb skill check.
// CritSuccess: total >= dc+10; Success: dc <= total < dc+10;
// Failure: dc-10 <= total < dc; CritFailure: total < dc-10.
func TestClimbOutcomes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dc := rapid.IntRange(5, 30).Draw(rt, "dc")
		roll := rapid.IntRange(1, 30).Draw(rt, "roll")
		// CritSuccess boundary.
		critSuccessRoll := dc + 10
		assert.Equal(rt, combat.CritSuccess, combat.OutcomeFor(critSuccessRoll, dc))
		// Success boundary.
		successRoll := dc
		assert.Equal(rt, combat.Success, combat.OutcomeFor(successRoll, dc))
		// Failure boundary.
		failureRoll := dc - 1
		assert.Equal(rt, combat.Failure, combat.OutcomeFor(failureRoll, dc))
		// CritFailure boundary (< dc-10).
		critFailRoll := dc - 11
		assert.Equal(rt, combat.CritFailure, combat.OutcomeFor(critFailRoll, dc))
		// Arbitrary roll is one of the four outcomes.
		o := combat.OutcomeFor(roll, dc)
		assert.True(rt, o == combat.CritSuccess || o == combat.Success || o == combat.Failure || o == combat.CritFailure)
		_ = dc
	})
}
