package world

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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
      map_x: 0
      map_y: 0
    - id: room_b
      title: "Room B"
      description: "This is room B."
      exits:
        - direction: south
          target: room_a
      map_x: 0
      map_y: 2
    - id: room_c
      title: "Room C"
      description: "This is room C."
      exits:
        - direction: west
          target: room_a
        - direction: north
          target: room_b
          locked: true
      map_x: 2
      map_y: 0
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
      map_x: 0
      map_y: 0
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
      map_x: 0
      map_y: 0
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

func TestLoadZoneFromBytes_DuplicateMapCoords_ReturnsError(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test
  description: Test zone
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 4
      map_y: 4
    - id: r2
      title: Room 2
      description: Desc
      map_x: 4
      map_y: 4
`)
	_, err := LoadZoneFromBytes(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share map coordinates")
}

func TestLoadZoneFromBytes_MissingMapCoords_ReturnsError(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test
  description: Test zone
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
`)
	_, err := LoadZoneFromBytes(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "map_x")
}

func TestLoadZoneFromBytes_MissingMapY_ReturnsError(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test
  description: Test zone
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 5
`)
	_, err := LoadZoneFromBytes(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "map_y")
}

func TestProperty_LoadZoneFromBytes_CoordinatesRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		x := rapid.Int().Draw(t, "x")
		y := rapid.Int().Draw(t, "y")
		data := []byte(fmt.Sprintf(`
zone:
  id: test
  name: Test
  description: Test zone
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: %d
      map_y: %d
`, x, y))
		z, err := LoadZoneFromBytes(data)
		if err != nil {
			t.Fatalf("unexpected error for coords (%d,%d): %v", x, y, err)
		}
		if z.Rooms["r1"].MapX != x {
			t.Fatalf("MapX: got %d, want %d", z.Rooms["r1"].MapX, x)
		}
		if z.Rooms["r1"].MapY != y {
			t.Fatalf("MapY: got %d, want %d", z.Rooms["r1"].MapY, y)
		}
	})
}

func TestLoadZoneFromBytes_WithMapCoords_ParsesCorrectly(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test
  description: Test zone
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 5
      map_y: 3
`)
	z, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	require.Equal(t, 5, z.Rooms["r1"].MapX)
	require.Equal(t, 3, z.Rooms["r1"].MapY)
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
      map_x: 0
      map_y: 0
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
	assert.Len(t, zone.Rooms, 13)

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
      map_x: 0
      map_y: 0
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
      map_x: 0
      map_y: 0
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
      map_x: 0
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["r1"]
	assert.Empty(t, room.Spawns)
}

func TestLoader_ParsesRoomEquipment(t *testing.T) {
	zones, err := LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)
	var found *RoomEquipmentConfig
	for _, z := range zones {
		for _, r := range z.Rooms {
			for i := range r.Equipment {
				found = &r.Equipment[i]
				break
			}
			if found != nil {
				break
			}
		}
		if found != nil {
			break
		}
	}
	require.NotNil(t, found, "expected at least one room with equipment defined")
	assert.NotEmpty(t, found.ItemID)
	assert.Greater(t, found.MaxCount, 0)
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
      map_x: 0
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(yamlData)
	require.NoError(t, err)
	assert.Equal(t, "", zone.ScriptDir)
	assert.Equal(t, 0, zone.ScriptInstructionLimit)
}

func TestRoom_ParsesSkillChecks(t *testing.T) {
	raw := `
zone:
  id: test_zone
  name: Test Zone
  description: A test zone.
  start_room: room1
  rooms:
  - id: room1
    title: Test Room
    description: A test room.
    map_x: 0
    map_y: 0
    skill_checks:
    - skill: parkour
      dc: 14
      trigger: on_enter
      outcomes:
        success:
          message: "You pass."
        failure:
          message: "You fail."
          effect:
            type: damage
            formula: "1d4"
`
	zone, err := LoadZoneFromBytes([]byte(raw))
	require.NoError(t, err)
	room := zone.Rooms["room1"]
	require.Len(t, room.SkillChecks, 1)
	sc := room.SkillChecks[0]
	assert.Equal(t, "parkour", sc.Skill)
	assert.Equal(t, 14, sc.DC)
	assert.Equal(t, "on_enter", sc.Trigger)
	require.NotNil(t, sc.Outcomes.Success)
	assert.Equal(t, "You pass.", sc.Outcomes.Success.Message)
	require.NotNil(t, sc.Outcomes.Failure)
	assert.Equal(t, "You fail.", sc.Outcomes.Failure.Message)
	require.NotNil(t, sc.Outcomes.Failure.Effect)
	assert.Equal(t, "damage", sc.Outcomes.Failure.Effect.Type)
	assert.Equal(t, "1d4", sc.Outcomes.Failure.Effect.Formula)
}

func TestProperty_Room_SkillChecksCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "n")
		entries := ""
		for i := 0; i < n; i++ {
			entries += fmt.Sprintf(`
    - skill: skill%d
      dc: %d
      trigger: on_enter
      outcomes:
        success:
          message: "ok"
`, i, 10+i)
		}
		skillChecksBlock := ""
		if n > 0 {
			skillChecksBlock = "    skill_checks:" + entries
		}
		data := []byte(fmt.Sprintf(`
zone:
  id: prop_zone
  name: Prop Zone
  description: property test zone.
  start_room: r1
  rooms:
  - id: r1
    title: Room 1
    description: A room.
    map_x: 0
    map_y: 0
%s`, skillChecksBlock))
		z, err := LoadZoneFromBytes(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := len(z.Rooms["r1"].SkillChecks)
		if got != n {
			t.Fatalf("expected %d SkillChecks, got %d", n, got)
		}
	})
}

func TestLoadZone_RoomEquipment_DescriptionParsed(t *testing.T) {
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
      equipment:
        - item_id: zone_map
          description: Zone Map
          max_count: 1
          immovable: true
      map_x: 0
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["r1"]
	require.Len(t, room.Equipment, 1)
	assert.Equal(t, "zone_map", room.Equipment[0].ItemID)
	assert.Equal(t, "Zone Map", room.Equipment[0].Description)
}

func TestLoadZone_RoomEquipment_Description_AbsentIsEmpty(t *testing.T) {
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
      equipment:
        - item_id: zone_map
          max_count: 1
          immovable: true
      map_x: 0
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["r1"]
	require.Len(t, room.Equipment, 1)
	assert.Equal(t, "zone_map", room.Equipment[0].ItemID)
	assert.Equal(t, "", room.Equipment[0].Description)
}

// Ensure the skillcheck import is used; ForOutcome is referenced via the type.
var _ = skillcheck.TriggerDef{}
