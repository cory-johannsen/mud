package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

func prone() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0}
}

func frightened() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4}
}

func dying() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4}
}

func TestActiveSet_Apply_Permanent(t *testing.T) {
	s := condition.NewActiveSet()
	err := s.Apply("testuid", prone(), 1, -1)
	require.NoError(t, err)
	assert.True(t, s.Has("prone"))
	assert.Equal(t, 1, s.Stacks("prone"))
}

func TestActiveSet_Apply_Rounds(t *testing.T) {
	s := condition.NewActiveSet()
	err := s.Apply("testuid", frightened(), 2, 3)
	require.NoError(t, err)
	assert.True(t, s.Has("frightened"))
	assert.Equal(t, 2, s.Stacks("frightened"))
}

func TestActiveSet_Apply_StacksCapped(t *testing.T) {
	s := condition.NewActiveSet()
	// MaxStacks=4 for dying; request 5, expect capped to 4
	err := s.Apply("testuid", dying(), 5, -1)
	require.NoError(t, err)
	assert.Equal(t, 4, s.Stacks("dying"))
}

func TestActiveSet_Apply_ZeroMaxStacks_AlwaysOne(t *testing.T) {
	// MaxStacks=0 means unstackable; stacks is always 1
	s := condition.NewActiveSet()
	err := s.Apply("testuid", prone(), 3, -1)
	require.NoError(t, err)
	assert.Equal(t, 1, s.Stacks("prone"))
}

func TestActiveSet_Remove(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", prone(), 1, -1))
	s.Remove("testuid", "prone")
	assert.False(t, s.Has("prone"))
	assert.Equal(t, 0, s.Stacks("prone"))
}

func TestActiveSet_Remove_NotPresent_NoOp(t *testing.T) {
	s := condition.NewActiveSet()
	s.Remove("testuid", "nonexistent") // must not panic
	assert.False(t, s.Has("nonexistent"))
}

func TestActiveSet_Tick_DecrementsRounds(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", frightened(), 2, 3))
	expired := s.Tick("testuid")
	assert.Empty(t, expired)
	assert.True(t, s.Has("frightened")) // still present
}

func TestActiveSet_Tick_ExpiresAtZero(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", frightened(), 1, 1))
	expired := s.Tick("testuid")
	assert.Equal(t, []string{"frightened"}, expired)
	assert.False(t, s.Has("frightened"))
}

func TestActiveSet_Tick_PermanentNotExpired(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", prone(), 1, -1))
	expired := s.Tick("testuid")
	assert.Empty(t, expired)
	assert.True(t, s.Has("prone"))
}

func TestActiveSet_Tick_UntilSaveNotExpired(t *testing.T) {
	// until_save conditions are not expired by Tick — they require explicit Remove
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", dying(), 1, -1))
	expired := s.Tick("testuid")
	assert.Empty(t, expired)
	assert.True(t, s.Has("dying"))
}

func TestActiveSet_All_ReturnsCopy(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply("testuid", prone(), 1, -1))
	require.NoError(t, s.Apply("testuid", frightened(), 2, 2))
	all := s.All()
	assert.Len(t, all, 2)
	// Mutating the returned slice must not affect the ActiveSet
	all[0] = nil
	all2 := s.All()
	assert.Len(t, all2, 2)
	for _, ac := range all2 {
		assert.NotNil(t, ac, "ActiveSet must not be corrupted by mutating the returned slice")
	}
}

func TestActiveSet_IncrementDyingStacks(t *testing.T) {
	s := condition.NewActiveSet()
	d := dying()
	require.NoError(t, s.Apply("testuid", d, 1, -1))
	require.NoError(t, s.Apply("testuid", d, 1, -1)) // apply again to increment
	assert.Equal(t, 2, s.Stacks("dying"))
}

func TestPropertyActiveSet_TickNeverBelowMinusOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := rapid.IntRange(1, 10).Draw(t, "duration")
		ticks := rapid.IntRange(1, 20).Draw(t, "ticks")
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply("testuid", frightened(), 1, duration))
		for i := 0; i < ticks; i++ {
			s.Tick("testuid")
		}
		for _, ac := range s.All() {
			assert.GreaterOrEqual(t, ac.DurationRemaining, -1,
				"DurationRemaining must never go below -1")
		}
	})
}

func TestPropertyActiveSet_ApplyRemove_HasFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply("testuid", prone(), 1, -1))
		s.Remove("testuid", "prone")
		assert.False(t, s.Has("prone"),
			"Has must return false after Remove")
	})
}

func TestPropertyActiveSet_ReapplyStacksCapped(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxStacks := rapid.IntRange(1, 4).Draw(t, "max_stacks")
		applies := rapid.IntRange(2, 6).Draw(t, "applies")
		def := &condition.ConditionDef{
			ID: "test", Name: "Test", DurationType: "rounds", MaxStacks: maxStacks,
		}
		s := condition.NewActiveSet()
		for i := 0; i < applies; i++ {
			require.NoError(t, s.Apply("testuid", def, 1, 5))
		}
		assert.LessOrEqual(t, s.Stacks("test"), maxStacks,
			"stacks must never exceed MaxStacks even after multiple Apply calls")
	})
}

func TestActiveSet_Apply_DurationMaxOnReapply(t *testing.T) {
	s := condition.NewActiveSet()
	d := frightened()
	require.NoError(t, s.Apply("testuid", d, 1, 3))
	// Re-apply with longer duration — should extend
	require.NoError(t, s.Apply("testuid", d, 1, 5))
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, 5, all[0].DurationRemaining, "longer re-apply duration must win")
}

func TestActiveSet_Apply_DurationShorterOnReapply_NotReduced(t *testing.T) {
	s := condition.NewActiveSet()
	d := frightened()
	require.NoError(t, s.Apply("testuid", d, 1, 5))
	// Re-apply with shorter duration — must not reduce
	require.NoError(t, s.Apply("testuid", d, 1, 2))
	all := s.All()
	require.Len(t, all, 1)
	assert.Equal(t, 5, all[0].DurationRemaining, "shorter re-apply duration must not reduce existing")
}

func TestPropertyActiveSet_StacksNeverExceedMaxStacks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxStacks := rapid.IntRange(1, 4).Draw(t, "max_stacks")
		stacks := rapid.IntRange(1, 8).Draw(t, "stacks")
		def := &condition.ConditionDef{
			ID: "test", Name: "Test", DurationType: "rounds", MaxStacks: maxStacks,
		}
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply("testuid", def, stacks, 5))
		actual := s.Stacks("test")
		assert.LessOrEqual(t, actual, maxStacks,
			"stacks must never exceed MaxStacks")
	})
}
