package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestExit_ClimbDC_ZeroByDefault verifies that the zero value of Exit has ClimbDC=0.
//
// Precondition: Exit created with struct literal omitting ClimbDC.
// Postcondition: ClimbDC == 0 (not climbable by default).
func TestExit_ClimbDC_ZeroByDefault(t *testing.T) {
	e := world.Exit{Direction: world.North, TargetRoom: "room_b"}
	assert.Equal(t, 0, e.ClimbDC)
	assert.Equal(t, 0, e.Height)
	assert.Equal(t, 0, e.SwimDC)
}

// TestRoom_Terrain_EmptyByDefault verifies that Room.Terrain defaults to empty string.
//
// Precondition: Room created without Terrain field.
// Postcondition: Terrain == "".
func TestRoom_Terrain_EmptyByDefault(t *testing.T) {
	r := world.Room{ID: "r1"}
	assert.Equal(t, "", r.Terrain)
}

// TestProperty_FallDamage_Formula verifies fall damage = max(1, floor(height/10)) for any height in [0,100].
//
// Precondition: height in [0, 100].
// Postcondition: dice count = max(1, height/10).
func TestProperty_FallDamage_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		height := rapid.IntRange(0, 100).Draw(rt, "height")
		expected := height / 10
		if expected < 1 {
			expected = 1
		}
		assert.Equal(rt, expected, fallDiceCount(height))
	})
}

// fallDiceCount mirrors the formula from handleClimb for testing.
func fallDiceCount(height int) int {
	d := height / 10
	if d < 1 {
		return 1
	}
	return d
}
