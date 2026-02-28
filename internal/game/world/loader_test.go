package world

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validZoneYAML = `
zone:
  id: test
  name: "Test Zone"
  description: "A test zone for testing."
  start_room: room_a
  rooms:
    - id: room_a
      title: "Room A"
      description: |
        This is room A.
        It has two lines.
      exits:
        - direction: north
          target: room_b
        - direction: east
          target: room_c
          hidden: true
      properties:
        lighting: bright
    - id: room_b
      title: "Room B"
      description: "This is room B."
      exits:
        - direction: south
          target: room_a
    - id: room_c
      title: "Room C"
      description: "This is room C."
      exits:
        - direction: west
          target: room_a
        - direction: north
          target: room_b
          locked: true
`

func TestLoadZoneFromBytes_Valid(t *testing.T) {
	zone, err := LoadZoneFromBytes([]byte(validZoneYAML))
	require.NoError(t, err)

	assert.Equal(t, "test", zone.ID)
	assert.Equal(t, "Test Zone", zone.Name)
	assert.Equal(t, "room_a", zone.StartRoom)
	assert.Len(t, zone.Rooms, 3)

	roomA := zone.Rooms["room_a"]
	assert.Equal(t, "Room A", roomA.Title)
	assert.Contains(t, roomA.Description, "This is room A.")
	assert.Len(t, roomA.Exits, 2)
	assert.Equal(t, "bright", roomA.Properties["lighting"])

	// Verify hidden exit
	exit, ok := roomA.ExitForDirection(East)
	assert.True(t, ok)
	assert.True(t, exit.Hidden)

	// Verify locked exit
	roomC := zone.Rooms["room_c"]
	exit, ok = roomC.ExitForDirection(North)
	assert.True(t, ok)
	assert.True(t, exit.Locked)
}

func TestLoadZoneFromBytes_InvalidYAML(t *testing.T) {
	_, err := LoadZoneFromBytes([]byte("not: [valid yaml"))
	assert.Error(t, err)
}

func TestLoadZoneFromBytes_MissingID(t *testing.T) {
	yaml := `
zone:
  name: "No ID"
  description: "Missing ID"
  start_room: room_a
  rooms:
    - id: room_a
      title: "Room"
      description: "A room"
`
	_, err := LoadZoneFromBytes([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zone ID must not be empty")
}

func TestLoadZoneFromBytes_CrossZoneExitAllowed(t *testing.T) {
	yaml := `
zone:
  id: test
  name: "Test"
  description: "Test"
  start_room: room_a
  rooms:
    - id: room_a
      title: "Room A"
      description: "A room"
      exits:
        - direction: north
          target: other_zone_room
`
	zone, err := LoadZoneFromBytes([]byte(yaml))
	assert.NoError(t, err, "cross-zone exit targets must be allowed at zone level")
	assert.NotNil(t, zone)
}

func TestLoadZoneFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validZoneYAML), 0644))

	zone, err := LoadZoneFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test", zone.ID)
}

func TestLoadZoneFromFile_NotFound(t *testing.T) {
	_, err := LoadZoneFromFile("/nonexistent/zone.yaml")
	assert.Error(t, err)
}

func TestLoadZonesFromDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zone1.yaml"), []byte(validZoneYAML), 0644))

	zone2 := `
zone:
  id: zone2
  name: "Zone 2"
  description: "Second zone"
  start_room: start
  rooms:
    - id: start
      title: "Start"
      description: "Starting room"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zone2.yaml"), []byte(zone2), 0644))

	zones, err := LoadZonesFromDir(dir)
	require.NoError(t, err)
	assert.Len(t, zones, 2)
}

func TestLoadZonesFromDir_Empty(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadZonesFromDir(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no zone files found")
}

func TestLoadZonesFromDir_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("not valid zone"), 0644))
	_, err := LoadZonesFromDir(dir)
	assert.Error(t, err)
}

func TestLoadZonesFromDir_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zone.yaml"), []byte(validZoneYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644))

	zones, err := LoadZonesFromDir(dir)
	require.NoError(t, err)
	assert.Len(t, zones, 1)
}

func TestLoadActualDowntownZone(t *testing.T) {
	zone, err := LoadZoneFromFile("../../../content/zones/downtown.yaml")
	require.NoError(t, err)

	assert.Equal(t, "downtown", zone.ID)
	assert.Equal(t, "Downtown Portland", zone.Name)
	assert.Equal(t, "pioneer_square", zone.StartRoom)
	assert.Len(t, zone.Rooms, 10)

	// Verify start room exists and has exits
	start := zone.Rooms["pioneer_square"]
	require.NotNil(t, start)
	assert.Equal(t, "Pioneer Courthouse Square", start.Title)
	assert.GreaterOrEqual(t, len(start.Exits), 2)

	// Verify all exit targets are valid (zone.Validate() already checks this)
	require.NoError(t, zone.Validate())
}

func TestLoadZone_ScriptFields_Populated(t *testing.T) {
	yamlData := []byte(`
zone:
  id: scripted_zone
  name: Scripted Zone
  description: A zone with scripts.
  start_room: r1
  script_dir: content/scripts/zones/scripted_zone
  script_instruction_limit: 50000
  rooms:
    - id: r1
      title: Start Room
      description: The beginning.
      exits: []
`)
	zone, err := LoadZoneFromBytes(yamlData)
	require.NoError(t, err)
	assert.Equal(t, "content/scripts/zones/scripted_zone", zone.ScriptDir)
	assert.Equal(t, 50000, zone.ScriptInstructionLimit)
}

func TestLoadZone_RoomSpawns_ParsedCorrectly(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test Zone
  description: desc
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: A room.
      spawns:
        - template: ganger
          count: 2
          respawn_after: "3m"
        - template: scavenger
          count: 1
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["r1"]
	require.Len(t, room.Spawns, 2)
	assert.Equal(t, "ganger", room.Spawns[0].Template)
	assert.Equal(t, 2, room.Spawns[0].Count)
	assert.Equal(t, "3m", room.Spawns[0].RespawnAfter)
	assert.Equal(t, "scavenger", room.Spawns[1].Template)
	assert.Equal(t, 1, room.Spawns[1].Count)
	assert.Equal(t, "", room.Spawns[1].RespawnAfter)
}

func TestLoadZone_Room_NoSpawns_EmptySlice(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test Zone
  description: desc
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: A room.
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["r1"]
	assert.Empty(t, room.Spawns)
}

func TestLoadZone_ScriptFieldsAbsent_ZeroValue(t *testing.T) {
	yamlData := []byte(`
zone:
  id: plain_zone
  name: Plain Zone
  description: No scripts.
  start_room: r1
  rooms:
    - id: r1
      title: Start Room
      description: The beginning.
      exits: []
`)
	zone, err := LoadZoneFromBytes(yamlData)
	require.NoError(t, err)
	assert.Equal(t, "", zone.ScriptDir)
	assert.Equal(t, 0, zone.ScriptInstructionLimit)
}
