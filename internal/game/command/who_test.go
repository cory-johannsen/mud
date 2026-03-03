package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHealthLabel_AllThresholds(t *testing.T) {
	cases := []struct {
		current, max int
		want         string
	}{
		{10, 10, "Uninjured"},
		{9, 10, "Lightly Wounded"},
		{75, 100, "Lightly Wounded"},
		{74, 100, "Wounded"},
		{50, 100, "Wounded"},
		{49, 100, "Badly Wounded"},
		{25, 100, "Badly Wounded"},
		{24, 100, "Near Death"},
		{0, 100, "Near Death"},
		{1, 100, "Near Death"},
	}
	for _, tc := range cases {
		got := command.HealthLabel(tc.current, tc.max)
		assert.Equal(t, tc.want, got, "HealthLabel(%d, %d)", tc.current, tc.max)
	}
}

func TestHealthLabel_ZeroMaxHP_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { command.HealthLabel(0, 0) })
}

func TestStatusLabel_AllValues(t *testing.T) {
	assert.Equal(t, "Idle", command.StatusLabel(0))
	assert.Equal(t, "Idle", command.StatusLabel(1))
	assert.Equal(t, "In Combat", command.StatusLabel(2))
	assert.Equal(t, "Resting", command.StatusLabel(3))
	assert.Equal(t, "Unconscious", command.StatusLabel(4))
	assert.Equal(t, "Idle", command.StatusLabel(99)) // unknown → Idle
}

func TestHandleWho_EmptyList(t *testing.T) {
	result := command.HandleWho(nil)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Nobody")
}

func TestProperty_HealthLabel_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		current := rapid.Int().Draw(rt, "current")
		max := rapid.Int().Draw(rt, "max")
		_ = command.HealthLabel(current, max)
	})
}

func TestProperty_StatusLabel_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		status := rapid.Int32().Draw(rt, "status")
		_ = command.StatusLabel(status)
	})
}
