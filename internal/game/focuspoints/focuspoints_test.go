package focuspoints_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/focuspoints"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestComputeMax(t *testing.T) {
	assert.Equal(t, 0, focuspoints.ComputeMax(0))
	assert.Equal(t, 1, focuspoints.ComputeMax(1))
	assert.Equal(t, 3, focuspoints.ComputeMax(3))
	assert.Equal(t, 3, focuspoints.ComputeMax(4)) // capped at 3
	assert.Equal(t, 3, focuspoints.ComputeMax(10))
}

func TestSpend(t *testing.T) {
	cur, ok := focuspoints.Spend(2, 3)
	assert.True(t, ok)
	assert.Equal(t, 1, cur)

	cur, ok = focuspoints.Spend(0, 3)
	assert.False(t, ok)
	assert.Equal(t, 0, cur)
}

func TestRestore_CritSuccessAndSuccess(t *testing.T) {
	cur := focuspoints.Restore(1, 3, focuspoints.OutcomeCritSuccess)
	assert.Equal(t, 3, cur)
	cur = focuspoints.Restore(0, 3, focuspoints.OutcomeSuccess)
	assert.Equal(t, 3, cur)
}

func TestRestore_Failure(t *testing.T) {
	cur := focuspoints.Restore(1, 3, focuspoints.OutcomeFailure)
	assert.Equal(t, 2, cur)
	cur = focuspoints.Restore(3, 3, focuspoints.OutcomeFailure) // capped
	assert.Equal(t, 3, cur)
}

func TestRestore_CritFailure(t *testing.T) {
	cur := focuspoints.Restore(1, 3, focuspoints.OutcomeCritFailure)
	assert.Equal(t, 1, cur) // no change
}

func TestClamp(t *testing.T) {
	assert.Equal(t, 2, focuspoints.Clamp(5, 2))
	assert.Equal(t, 2, focuspoints.Clamp(2, 3))
	assert.Equal(t, 0, focuspoints.Clamp(0, 0))
}

func TestSpendProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cur := rapid.IntRange(0, 10).Draw(t, "cur")
		max := rapid.IntRange(0, 10).Draw(t, "max")
		next, ok := focuspoints.Spend(cur, max)
		if cur > 0 {
			assert.True(t, ok)
			assert.Equal(t, cur-1, next)
		} else {
			assert.False(t, ok)
			assert.Equal(t, 0, next)
		}
	})
}
