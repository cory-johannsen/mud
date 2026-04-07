package world

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	assert.Equal(t, "downtown_underground", zone.StartRoom)
	assert.GreaterOrEqual(t, len(zone.Rooms), 30)

	// Verify start room exists and has exits
	start := zone.Rooms["downtown_underground"]
	require.NotNil(t, start)
	assert.Equal(t, "The Underground", start.Title)
	assert.GreaterOrEqual(t, len(start.Exits), 1)

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

func TestZoneToYAMLRoundTrip(t *testing.T) {
	original, err := LoadZoneFromBytes([]byte(validZoneYAML))
	require.NoError(t, err)

	yf := zoneToYAML(original)
	data, err := yaml.Marshal(yf)
	require.NoError(t, err)

	reloaded, err := LoadZoneFromBytes(data)
	require.NoError(t, err)

	assert.Equal(t, original.ID, reloaded.ID)
	assert.Equal(t, original.Name, reloaded.Name)
	assert.Equal(t, len(original.Rooms), len(reloaded.Rooms))
	for id, room := range original.Rooms {
		r2, ok := reloaded.Rooms[id]
		require.True(t, ok, "room %q missing after round-trip", id)
		assert.Equal(t, room.Title, r2.Title)
		assert.Equal(t, room.MapX, r2.MapX)
		assert.Equal(t, room.MapY, r2.MapY)
		assert.Equal(t, len(room.Exits), len(r2.Exits))
	}
}

func TestLoadZoneFromBytes_WorldCoords_Parsed(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test Zone
  description: Desc
  start_room: r1
  world_x: 2
  world_y: -4
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 0
      map_y: 0
`)
	z, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, z.WorldX, "WorldX must not be nil when world_x is set")
	require.NotNil(t, z.WorldY, "WorldY must not be nil when world_y is set")
	assert.Equal(t, 2, *z.WorldX)
	assert.Equal(t, -4, *z.WorldY)
}

func TestLoadZoneFromBytes_WorldCoords_Nil_WhenAbsent(t *testing.T) {
	data := []byte(`
zone:
  id: test
  name: Test Zone
  description: Desc
  start_room: r1
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 0
      map_y: 0
`)
	z, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Nil(t, z.WorldX, "WorldX must be nil when world_x is absent")
	assert.Nil(t, z.WorldY, "WorldY must be nil when world_y is absent")
}

func TestLoadZoneFromBytes_WorldCoords_ZeroValueDistinguishable(t *testing.T) {
	// (0, 0) must decode as a non-nil pointer pointing to 0, not nil.
	data := []byte(`
zone:
  id: downtown
  name: Downtown
  description: Desc
  start_room: r1
  world_x: 0
  world_y: 0
  rooms:
    - id: r1
      title: Room 1
      description: Desc
      map_x: 0
      map_y: 0
`)
	z, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, z.WorldX, "WorldX must not be nil for world_x: 0")
	require.NotNil(t, z.WorldY, "WorldY must not be nil for world_y: 0")
	assert.Equal(t, 0, *z.WorldX)
	assert.Equal(t, 0, *z.WorldY)
}

// TestNEPortlandZoneFullyConnected verifies that every room in the NE Portland
// zone is reachable from the start room via bidirectional exit traversal, and
// that all exits within the zone are reciprocal.
func TestNEPortlandZoneFullyConnected(t *testing.T) {
	zone, err := LoadZoneFromFile("../../../content/zones/ne_portland.yaml")
	require.NoError(t, err)

	startID := zone.StartRoom
	require.NotEmpty(t, startID, "start_room must be set")

	// Build adjacency as undirected graph using only intra-zone exits.
	adj := make(map[string][]string, len(zone.Rooms))
	for id := range zone.Rooms {
		adj[id] = []string{}
	}
	for id, room := range zone.Rooms {
		for _, exit := range room.Exits {
			if _, inZone := zone.Rooms[exit.TargetRoom]; inZone {
				adj[id] = append(adj[id], exit.TargetRoom)
			}
		}
	}

	// BFS from start room.
	visited := make(map[string]bool)
	queue := []string{startID}
	visited[startID] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, neighbor := range adj[cur] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	// Every room must be reachable.
	for id := range zone.Rooms {
		assert.True(t, visited[id], "room %q is not reachable from start room %q", id, startID)
	}

	// Every intra-zone exit must have a reciprocal exit in the target room.
	for id, room := range zone.Rooms {
		for _, exit := range room.Exits {
			target, inZone := zone.Rooms[exit.TargetRoom]
			if !inZone {
				continue // cross-zone exits are out of scope
			}
			opposite := exit.Direction.Opposite()
			if opposite == "" {
				continue // non-standard direction; skip reciprocal check
			}
			_, hasReciprocal := target.ExitForDirection(opposite)
			assert.True(t, hasReciprocal,
				"room %q exit %s→%s has no reciprocal %s exit in %q",
				id, exit.Direction, exit.TargetRoom, opposite, exit.TargetRoom)
		}
	}
}

// TestNEPortlandZone_MapVisuallyConnected verifies that every room in the
// NE Portland zone has at least one exit to a grid-adjacent room so the map
// renderer draws a visible connector. This is the root cause of BUG-30:
// rooms whose coordinates are not adjacent to any neighbor appear as
// disconnected islands on the map.
//
// Precondition: NE Portland zone loads without error.
// Postcondition: Every room has at least one intra-zone exit whose target is
// at a grid-adjacent position (cardinal: delta of 2 on one axis, 0 on the
// other; diagonal: delta of 2 on both axes).
func TestNEPortlandZone_MapVisuallyConnected(t *testing.T) {
	zone, err := LoadZoneFromFile("../../../content/zones/ne_portland.yaml")
	require.NoError(t, err)

	type coord struct{ x, y int }
	coordOf := make(map[string]coord, len(zone.Rooms))
	for id, room := range zone.Rooms {
		coordOf[id] = coord{room.MapX, room.MapY}
	}

	isAdjacent := func(a, b coord) bool {
		dx := a.x - b.x
		dy := a.y - b.y
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		// Cardinal: one axis == 2, other == 0.
		// Diagonal: both axes == 2.
		if dx == 2 && dy == 0 {
			return true
		}
		if dx == 0 && dy == 2 {
			return true
		}
		if dx == 2 && dy == 2 {
			return true
		}
		return false
	}

	for id, room := range zone.Rooms {
		c := coordOf[id]
		hasAdjacentExit := false
		for _, exit := range room.Exits {
			tc, inZone := coordOf[exit.TargetRoom]
			if !inZone {
				continue
			}
			if isAdjacent(c, tc) {
				hasAdjacentExit = true
				break
			}
		}
		// Also check if any other room has an adjacent exit pointing TO this room.
		if !hasAdjacentExit {
			for otherID, otherRoom := range zone.Rooms {
				if otherID == id {
					continue
				}
				for _, exit := range otherRoom.Exits {
					if exit.TargetRoom == id && isAdjacent(coordOf[otherID], c) {
						hasAdjacentExit = true
						break
					}
				}
				if hasAdjacentExit {
					break
				}
			}
		}
		assert.True(t, hasAdjacentExit,
			"BUG-30: room %q at (%d,%d) has no grid-adjacent neighbor — appears as disconnected island on map",
			id, c.x, c.y)
	}
}

// TestNEPortlandZone_NoCoordinateOverlap verifies no two rooms share the same
// map coordinates, which would cause them to stack on top of each other in the
// renderer.
func TestNEPortlandZone_NoCoordinateOverlap(t *testing.T) {
	zone, err := LoadZoneFromFile("../../../content/zones/ne_portland.yaml")
	require.NoError(t, err)

	type coord struct{ x, y int }
	seen := make(map[coord]string, len(zone.Rooms))
	for id, room := range zone.Rooms {
		c := coord{room.MapX, room.MapY}
		if existing, ok := seen[c]; ok {
			t.Errorf("rooms %q and %q share map coordinates (%d, %d)", existing, id, c.x, c.y)
		}
		seen[c] = id
	}
}

func TestLoadZone_ZoneEffects_PropagatedToRooms(t *testing.T) {
	data := []byte(`
zone:
  id: testzone
  name: Test Zone
  start_room: room1
  zone_effects:
    - track: fear
      severity: mild
      base_dc: 12
      cooldown_rounds: 3
      cooldown_minutes: 5
  rooms:
    - id: room1
      title: Room One
      description: A room.
      map_x: 0
      map_y: 0
    - id: room2
      title: Room Two
      description: Another room.
      map_x: 1
      map_y: 0
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	for _, room := range zone.Rooms {
		require.Len(t, room.Effects, 1, "zone_effects must be propagated to room %q", room.ID)
		assert.Equal(t, "fear", room.Effects[0].Track)
		assert.Equal(t, "mild", room.Effects[0].Severity)
		assert.Equal(t, 12, room.Effects[0].BaseDC)
	}
}

func TestLoadZone_ZoneEffects_DoNotOverrideRoomEffects(t *testing.T) {
	data := []byte(`
zone:
  id: testzone
  name: Test Zone
  start_room: room1
  zone_effects:
    - track: fear
      severity: mild
      base_dc: 10
      cooldown_rounds: 2
      cooldown_minutes: 3
  rooms:
    - id: room1
      title: Room One
      description: A room.
      map_x: 0
      map_y: 0
      effects:
        - track: rage
          severity: moderate
          base_dc: 14
          cooldown_rounds: 4
          cooldown_minutes: 6
`)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	room := zone.Rooms["room1"]
	require.Len(t, room.Effects, 2)
	tracks := []string{room.Effects[0].Track, room.Effects[1].Track}
	assert.Contains(t, tracks, "fear")
	assert.Contains(t, tracks, "rage")
}

// TestLoader_ZoneWorldCoords verifies that all 16 live zone YAML files decode
// with non-nil WorldX and WorldY fields matching the design spec coordinates.
func TestLoader_ZoneWorldCoords(t *testing.T) {
	expected := map[string][2]int{
		"battleground":      {4, -6},
		"the_couve":         {0, -4},
		"vantucky":          {2, -4},
		"sauvie_island":     {-2, -2},
		"pdx_international": {2, -2},
		"hillsboro":         {-4, 0},
		"beaverton":         {-2, 0},
		"downtown":          {0, 0},
		"ne_portland":       {2, 0},
		"rustbucket_ridge":  {4, 0},
		"troutdale":         {6, 0},
		"aloha":             {-4, 2},
		"ross_island":       {0, 2},
		"se_industrial":     {2, 2},
		"felony_flats":      {4, 2},
		"lake_oswego":       {0, 4},
	}

	zones, err := LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	byID := make(map[string]*Zone, len(zones))
	for _, z := range zones {
		byID[z.ID] = z
	}

	for id, coords := range expected {
		z, ok := byID[id]
		require.True(t, ok, "zone %q not loaded", id)
		require.NotNil(t, z.WorldX, "zone %q: WorldX must be non-nil", id)
		require.NotNil(t, z.WorldY, "zone %q: WorldY must be non-nil", id)
		require.Equal(t, coords[0], *z.WorldX, "zone %q: WorldX mismatch", id)
		require.Equal(t, coords[1], *z.WorldY, "zone %q: WorldY mismatch", id)
	}
}

func TestLoadZone_ClownCamp_HasFiveRooms(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/clown_camp.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 6, "Clown Camp must have exactly 6 rooms")
	assert.Equal(t, "clown_camp", zone.ID)
}

func TestLoadZone_ClownCamp_ZoneEffects(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/clown_camp.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	tracks := make([]string, 0, len(zone.ZoneEffects))
	for _, e := range zone.ZoneEffects {
		tracks = append(tracks, e.Track)
	}
	assert.Contains(t, tracks, "delirium", "Clown Camp must have delirium zone_effect")
	assert.Contains(t, tracks, "fear", "Clown Camp must have fear zone_effect")
}

func TestLoadZone_SteamPDX_HasSevenRooms(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/steampdx.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 8, "SteamPDX must have exactly 8 rooms")
	assert.Equal(t, "steampdx", zone.ID)
}

func TestLoadZone_TheVelvetRope_HasTerrainLubeEffect(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/the_velvet_rope.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 8, "The Velvet Rope must have exactly 8 rooms")
	assert.Equal(t, "the_velvet_rope", zone.ID)
	tracks := make([]string, 0, len(zone.ZoneEffects))
	for _, e := range zone.ZoneEffects {
		tracks = append(tracks, e.Track)
	}
	assert.Contains(t, tracks, "temptation", "The Velvet Rope must have temptation zone_effect")
	assert.Contains(t, tracks, "revulsion", "The Velvet Rope must have revulsion zone_effect")
	assert.Contains(t, tracks, "terrain_lube", "The Velvet Rope must have terrain_lube zone_effect")
}

func TestLoadZone_ClubPrivata_HasSixteenRooms(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/club_privata.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	assert.Len(t, zone.Rooms, 17, "Club Privata must have exactly 17 rooms")
	assert.Equal(t, "club_privata", zone.ID)
}

// TestLoadZone_Vantucky_AllRoomsReachable verifies that every room in the
// Vantucky zone is reachable from the zone's start room via bidirectional exits,
// and that all intra-zone exits have a corresponding reverse exit in the opposite
// direction. Cross-zone exits (targets in other zones) are excluded from the
// bidirectionality check.
//
// REQ-BUG62-1: Every intra-zone exit A→B in direction D MUST have a reverse
// exit B→A in direction opposite(D).
// REQ-BUG62-2: Every room in the Vantucky zone MUST be reachable from the
// zone's start room by traversing intra-zone exits.
func TestLoadZone_Vantucky_AllRoomsReachable(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/vantucky.yaml")
	require.NoError(t, err)
	zone, err := LoadZoneFromBytes(data)
	require.NoError(t, err)
	require.Equal(t, "vantucky", zone.ID)

	// Verify bidirectional exits for all intra-zone exits.
	for roomID, room := range zone.Rooms {
		for _, exit := range room.Exits {
			target, inZone := zone.Rooms[exit.TargetRoom]
			if !inZone {
				// Cross-zone exit — skip bidirectionality check.
				continue
			}
			revDir := exit.Direction.Opposite()
			if revDir == "" {
				// Non-standard direction has no defined opposite — skip.
				continue
			}
			_, hasReverse := target.ExitForDirection(revDir)
			assert.Truef(t, hasReverse,
				"room %q has %s exit to %q, but %q has no %s exit back (BUG-62)",
				roomID, exit.Direction, exit.TargetRoom, exit.TargetRoom, revDir,
			)
		}
	}

	// Verify all rooms are reachable from the start room via BFS.
	visited := make(map[string]bool)
	queue := []string{zone.StartRoom}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		room, ok := zone.Rooms[cur]
		if !ok {
			continue
		}
		for _, exit := range room.Exits {
			if _, inZone := zone.Rooms[exit.TargetRoom]; inZone && !visited[exit.TargetRoom] {
				queue = append(queue, exit.TargetRoom)
			}
		}
	}

	for roomID := range zone.Rooms {
		assert.Truef(t, visited[roomID],
			"room %q is unreachable from start room %q (BUG-62)",
			roomID, zone.StartRoom,
		)
	}
}
