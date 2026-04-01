// Package integration_test contains zone-level integration tests that cross-validate
// content YAML files against each other to enforce Oregon Country Fair zone requirements.
//
// REQ-OCF-1 through REQ-OCF-8: zone, faction, substance, and spawn cross-validation.
package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// ocfCombatNPCTemplates is the set of templates considered combat NPCs in the OCF zone.
var ocfCombatNPCTemplates = map[string]bool{
	"juggalo":          true,
	"juggalo_prophet":  true,
	"tweaker":          true,
	"tweaker_paranoid": true,
	"tweaker_cook":     true,
	"wook":             true,
	"wook_enforcer":    true,
	"wook_shaman":      true,
	"violent_jimmy":    true,
	"crystal_karen":    true,
	"spiral_king":      true,
}

// ocfSafeRooms is the set of rooms in the OCF safe cluster that must have no combat spawns.
var ocfSafeRooms = map[string]bool{
	"the_big_top":           true,
	"the_faygo_fountain":    true,
	"the_gathering_ground":  true,
	"the_trailer_cluster":   true,
	"the_cook_shed_anteroom": true,
	"tweaker_command_post":  true,
	"wook_river_camp":       true,
	"the_healing_waters_ocf": true,
	"the_wook_council_fire": true,
}

// loadOCFZone is a test helper that loads all zones and returns the oregon_country_fair zone.
func loadOCFZone(t *testing.T) *world.Zone {
	t.Helper()
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	for _, z := range zones {
		if z.ID == "oregon_country_fair" {
			return z
		}
	}
	t.Fatal("oregon_country_fair zone not found in content/zones")
	return nil
}

// TestOCFZone_Loads verifies the oregon_country_fair zone can be loaded and has correct
// top-level metadata.
//
// REQ-OCF-1: Zone ID must be "oregon_country_fair".
// REQ-OCF-2: Zone faction_id must be "" (neutral zone, no single faction ownership).
// REQ-OCF-3: Zone must contain between 30 and 45 rooms (inclusive).
func TestOCFZone_Loads(t *testing.T) {
	ocf := loadOCFZone(t)

	assert.Equal(t, "oregon_country_fair", ocf.ID)
	assert.Equal(t, "", ocf.FactionID, "OCF zone must have empty faction_id (neutral territory)")
	assert.GreaterOrEqual(t, len(ocf.Rooms), 30, "OCF zone must have at least 30 rooms")
	assert.LessOrEqual(t, len(ocf.Rooms), 45, "OCF zone must have no more than 45 rooms")
}

// TestOCFZone_ThreeBossRooms verifies that all three faction boss rooms exist, are marked
// as boss rooms, and each have at least one hazard.
//
// REQ-OCF-4: violent_jimmys_tent must exist and have boss_room: true with at least 1 hazard.
// REQ-OCF-5: crystal_karens_lab must exist and have boss_room: true with at least 1 hazard.
// REQ-OCF-6: the_spiral_kings_grove must exist and have boss_room: true with at least 1 hazard.
func TestOCFZone_ThreeBossRooms(t *testing.T) {
	ocf := loadOCFZone(t)

	bossRoomIDs := []string{
		"violent_jimmys_tent",
		"crystal_karens_lab",
		"the_spiral_kings_grove",
	}

	for _, roomID := range bossRoomIDs {
		room, ok := ocf.Rooms[roomID]
		require.True(t, ok, "boss room %q must exist in oregon_country_fair zone", roomID)
		assert.True(t, room.BossRoom, "room %q must have boss_room: true", roomID)
		assert.GreaterOrEqual(t, len(room.Hazards), 1,
			"boss room %q must have at least 1 hazard", roomID)
	}
}

// TestOCFZone_AmbientSubstances verifies that all ambient_substance values in OCF rooms
// reference valid substance IDs loaded from content/substances.
//
// REQ-OCF-7: All ambient_substance values in OCF rooms must resolve to valid substance IDs.
func TestOCFZone_AmbientSubstances(t *testing.T) {
	ocf := loadOCFZone(t)

	substanceReg, err := substance.LoadDirectory("../../../content/substances")
	require.NoError(t, err)

	for roomID, room := range ocf.Rooms {
		if room.AmbientSubstance == "" {
			continue
		}
		_, ok := substanceReg.Get(room.AmbientSubstance)
		assert.True(t, ok,
			"room %q: ambient_substance %q not found in substance registry",
			roomID, room.AmbientSubstance)
	}
}

// TestOCFFactions_Load verifies that juggalos, tweakers, and wooks all load and validate.
//
// REQ-OCF-8: juggalos faction must exist in content/factions.
// REQ-OCF-9: tweakers faction must exist in content/factions.
// REQ-OCF-10: wooks faction must exist in content/factions.
func TestOCFFactions_Load(t *testing.T) {
	reg, err := faction.LoadFactions("../../../content/factions")
	require.NoError(t, err)

	factionIDs := []string{"juggalos", "tweakers", "wooks"}
	for _, id := range factionIDs {
		f, ok := reg[id]
		require.True(t, ok, "faction %q must exist in content/factions", id)
		assert.NotEmpty(t, f.ID, "faction %q must have a non-empty ID", id)
		assert.NotEmpty(t, f.Name, "faction %q must have a non-empty name", id)
		assert.NotEmpty(t, f.Tiers, "faction %q must have at least one tier", id)
	}
}

// TestOCFFactions_WooksZoneID verifies that the wooks faction has an empty zone_id,
// reflecting that they are not tied to the OCF zone exclusively.
//
// REQ-OCF-11: wooks faction must have zone_id = "" (cross-zone faction).
func TestOCFFactions_WooksZoneID(t *testing.T) {
	reg, err := faction.LoadFactions("../../../content/factions")
	require.NoError(t, err)

	wooks, ok := reg["wooks"]
	require.True(t, ok, "wooks faction must exist in content/factions")
	assert.Equal(t, "", wooks.ZoneID, "wooks faction must have empty zone_id")
}

// TestOCFSubstance_TweakerCrystal verifies tweaker_crystal loads with category "stimulant".
//
// REQ-OCF-12: tweaker_crystal substance must exist in content/substances.
// REQ-OCF-13: tweaker_crystal category must be "stimulant".
func TestOCFSubstance_TweakerCrystal(t *testing.T) {
	reg, err := substance.LoadDirectory("../../../content/substances")
	require.NoError(t, err)

	tc, ok := reg.Get("tweaker_crystal")
	require.True(t, ok, "tweaker_crystal substance must exist in content/substances")
	assert.Equal(t, "stimulant", tc.Category, "tweaker_crystal category must be \"stimulant\"")
}

// TestOCFZone_SafeClusterRoomsHaveZeroSpawns verifies that the 9 safe cluster rooms
// in the OCF zone contain no combat NPC spawns.
//
// REQ-OCF-14: OCF safe cluster rooms must not spawn combat NPC templates.
func TestOCFZone_SafeClusterRoomsHaveZeroSpawns(t *testing.T) {
	ocf := loadOCFZone(t)

	assert.Len(t, ocfSafeRooms, 9, "test invariant: ocfSafeRooms must define exactly 9 safe rooms")

	for roomID := range ocfSafeRooms {
		room, ok := ocf.Rooms[roomID]
		require.True(t, ok, "expected safe room %q to exist in oregon_country_fair zone", roomID)
		for _, spawn := range room.Spawns {
			assert.False(t, ocfCombatNPCTemplates[spawn.Template],
				"safe room %q must not spawn combat NPC template %q", roomID, spawn.Template)
		}
	}
}

// TestProperty_OCFZone_RoomsNeverPanic verifies that iterating all OCF rooms and
// accessing their fields never causes a panic.
//
// REQ-OCF-15: Iterating all OCF rooms must never panic.
func TestProperty_OCFZone_RoomsNeverPanic(t *testing.T) {
	ocf := loadOCFZone(t)
	rooms := make([]*world.Room, 0, len(ocf.Rooms))
	for _, r := range ocf.Rooms {
		rooms = append(rooms, r)
	}

	rapid.Check(t, func(rt *rapid.T) {
		if len(rooms) == 0 {
			return
		}
		idx := rapid.IntRange(0, len(rooms)-1).Draw(rt, "room_idx")
		room := rooms[idx]

		// Access all fields that integration tests rely on — must not panic.
		_ = room.ID
		_ = room.Title
		_ = room.Description
		_ = room.BossRoom
		_ = room.AmbientSubstance
		_ = room.DangerLevel
		_ = room.Spawns
		_ = room.Hazards
		_ = room.Exits
	})
}
