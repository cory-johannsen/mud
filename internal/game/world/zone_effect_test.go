package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const zoneWithEffectsYAML = `
zone:
  id: test_zone
  name: Test Zone
  description: A test zone.
  start_room: room_a
  rooms:
    - id: room_a
      title: Room A
      description: A room with a zone effect.
      map_x: 0
      map_y: 0
      effects:
        - track: despair
          severity: mild
          base_dc: 12
          cooldown_rounds: 3
          cooldown_minutes: 5
    - id: room_b
      title: Room B
      description: A room with no effects.
      map_x: 1
      map_y: 0
`

func TestRoomEffect_LoadedFromYAML(t *testing.T) {
	zone, err := world.LoadZoneFromBytes([]byte(zoneWithEffectsYAML))
	require.NoError(t, err)

	roomA, ok := zone.Rooms["room_a"]
	require.True(t, ok)
	require.Len(t, roomA.Effects, 1)

	e := roomA.Effects[0]
	assert.Equal(t, "despair", e.Track)
	assert.Equal(t, "mild", e.Severity)
	assert.Equal(t, 12, e.BaseDC)
	assert.Equal(t, 3, e.CooldownRounds)
	assert.Equal(t, 5, e.CooldownMinutes)
}

func TestRoomEffect_NoEffects_EmptySlice(t *testing.T) {
	zone, err := world.LoadZoneFromBytes([]byte(zoneWithEffectsYAML))
	require.NoError(t, err)

	roomB, ok := zone.Rooms["room_b"]
	require.True(t, ok)
	assert.Empty(t, roomB.Effects, "room with no effects declaration should have empty slice")
}

func TestRoomEffect_MultipleEffects(t *testing.T) {
	const twoEffectsYAML = `
zone:
  id: test
  name: Test
  description: Test.
  start_room: r
  rooms:
    - id: r
      title: R
      description: R.
      map_x: 0
      map_y: 0
      effects:
        - track: despair
          severity: mild
          base_dc: 12
          cooldown_rounds: 3
          cooldown_minutes: 5
        - track: delirium
          severity: moderate
          base_dc: 14
          cooldown_rounds: 4
          cooldown_minutes: 10
`
	zone, err := world.LoadZoneFromBytes([]byte(twoEffectsYAML))
	require.NoError(t, err)
	r, ok := zone.Rooms["r"]
	require.True(t, ok)
	assert.Len(t, r.Effects, 2)
}

func TestRoomEffect_ZeroEffects_NoYAMLKey(t *testing.T) {
	const noEffectsYAML = `
zone:
  id: test
  name: Test
  description: Test.
  start_room: r
  rooms:
    - id: r
      title: R
      description: R.
      map_x: 0
      map_y: 0
`
	zone, err := world.LoadZoneFromBytes([]byte(noEffectsYAML))
	require.NoError(t, err)
	r, ok := zone.Rooms["r"]
	require.True(t, ok)
	assert.Empty(t, r.Effects)
}

// Ensure RoomEffect fields are exported (compile-time check via field access).
func TestRoomEffect_FieldsExported(t *testing.T) {
	e := world.RoomEffect{
		Track:           "rage",
		Severity:        "mild",
		BaseDC:          10,
		CooldownRounds:  2,
		CooldownMinutes: 3,
	}
	assert.Equal(t, "rage", e.Track)
	_, _ = yaml.Marshal(e) // must not panic
}
