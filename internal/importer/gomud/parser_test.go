package gomud_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

const zoneYAML = `
name: Test Zone
description: A test zone.
rooms:
  - Room A
  - Room B
areas:
  - Area One
`

const areaYAML = `
name: Area One
description: The first area.
rooms:
  - Room A
  - Room B
`

const roomWithExitsYAML = `
name: Room A
description: The first room.
exits:
  North:
    direction: North
    name: Room B
    description: Room B
    target: Room B
`

const roomEmptyExitsYAML = `
name: Room C
description: A dead end.
exits:
`

const roomWithObjectsYAML = `
name: Room D
description: Has objects.
objects:
  - Old couch
  - Lamp
exits:
  South:
    direction: South
    name: Room A
    target: Room A
`

func TestParseZone(t *testing.T) {
	z, err := gomud.ParseZone([]byte(zoneYAML))
	require.NoError(t, err)
	assert.Equal(t, "Test Zone", z.Name)
	assert.Equal(t, "A test zone.", z.Description)
	assert.Equal(t, []string{"Room A", "Room B"}, z.Rooms)
	assert.Equal(t, []string{"Area One"}, z.Areas)
}

func TestParseArea(t *testing.T) {
	a, err := gomud.ParseArea([]byte(areaYAML))
	require.NoError(t, err)
	assert.Equal(t, "Area One", a.Name)
	assert.Equal(t, []string{"Room A", "Room B"}, a.Rooms)
}

func TestParseRoom_WithExits(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomWithExitsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room A", r.Name)
	assert.Equal(t, "The first room.", r.Description)
	require.Contains(t, r.Exits, "North")
	assert.Equal(t, "Room B", r.Exits["North"].Target)
}

func TestParseRoom_EmptyExits(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomEmptyExitsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room C", r.Name)
	assert.Empty(t, r.Exits)
}

func TestParseRoom_ObjectsDropped(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomWithObjectsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room D", r.Name)
	// objects field is silently ignored; exits still parsed
	require.Contains(t, r.Exits, "South")
	assert.Equal(t, "Room A", r.Exits["South"].Target)
}

func TestParseZone_NamePassthrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 '_-]{0,30}`).Draw(t, "name")
		yaml := fmt.Sprintf("name: %q\ndescription: d\nrooms: []\nareas: []\n", name)
		z, err := gomud.ParseZone([]byte(yaml))
		require.NoError(t, err)
		assert.Equal(t, name, z.Name)
	})
}

func TestParseArea_NamePassthrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 '_-]{0,30}`).Draw(t, "name")
		yaml := fmt.Sprintf("name: %q\ndescription: d\nrooms: []\n", name)
		a, err := gomud.ParseArea([]byte(yaml))
		require.NoError(t, err)
		assert.Equal(t, name, a.Name)
	})
}

func TestParseRoom_NamePassthrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 '_-]{0,30}`).Draw(t, "name")
		yaml := fmt.Sprintf("name: %q\ndescription: d\nexits:\n", name)
		r, err := gomud.ParseRoom([]byte(yaml))
		require.NoError(t, err)
		assert.Equal(t, name, r.Name)
	})
}
