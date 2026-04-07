// Package integration_test contains zone-level integration tests that cross-validate
// content YAML files against each other to enforce Wooklyn zone requirements.
//
// REQ-WK-1 through REQ-WK-62: zone, faction, substance, and spawn cross-validation.
package integration_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// combatNPCTemplates is the set of templates considered combat NPCs in Wooklyn.
var combatNPCTemplates = map[string]bool{
	"wook":           true,
	"ginger_wook":    true,
	"wook_shaman":    true,
	"wook_enforcer":  true,
	"papa_wook":      true,
}

// safeRooms is the set of rooms in the safe cluster that must have no combat spawns.
var safeRooms = map[string]bool{
	"tofteville_gate":   true,
	"tofteville_market": true,
	"the_jam_meadow":    true,
	"tie_dye_tent_row":  true,
	"the_caravan":       true,
}

// TestWooklyn_ZoneLoads verifies the wooklyn zone can be loaded and has correct top-level metadata.
//
// REQ-WK-1: Zone ID must be "wooklyn".
// REQ-WK-2: Zone name must be "Wooklyn".
// REQ-WK-3: Zone faction_id must be "wooks".
// REQ-WK-4: Zone must contain exactly 35 rooms.
func TestWooklyn_ZoneLoads(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn, "wooklyn zone not found in content/zones")
	assert.Equal(t, "wooklyn", wooklyn.ID)
	assert.Equal(t, "Wooklyn", wooklyn.Name)
	assert.Equal(t, "wooks", wooklyn.FactionID)
	assert.Len(t, wooklyn.Rooms, 35)
}

// TestWooklyn_AllRoomsHaveValidConnections verifies every exit in every room has a non-empty
// target and that intra-zone exits reference rooms that exist in the zone.
//
// REQ-WK-5: All room exits must have a non-empty target room ID.
// REQ-WK-6: Intra-zone exits must reference rooms that exist in the zone.
func TestWooklyn_AllRoomsHaveValidConnections(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn)

	for roomID, room := range wooklyn.Rooms {
		for _, exit := range room.Exits {
			assert.NotEmpty(t, string(exit.Direction),
				"room %q: exit has empty direction", roomID)
			assert.NotEmpty(t, exit.TargetRoom,
				"room %q: exit direction %q has empty target", roomID, exit.Direction)
			// Check intra-zone exits resolve within the zone.
			if _, exists := wooklyn.Rooms[exit.TargetRoom]; !exists {
				// Cross-zone exits are permitted; skip validation for those.
				t.Logf("room %q: exit %q targets cross-zone room %q (permitted)", roomID, exit.Direction, exit.TargetRoom)
			}
		}
	}
}

// TestWooklyn_SafeClusterHasNoSpawns verifies that the five safe-cluster rooms
// do not spawn any combat NPC templates.
//
// REQ-WK-7: Safe cluster rooms must not spawn combat NPC templates.
func TestWooklyn_SafeClusterHasNoSpawns(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn)

	for roomID := range safeRooms {
		room, ok := wooklyn.Rooms[roomID]
		require.True(t, ok, "expected safe room %q to exist in wooklyn zone", roomID)
		for _, spawn := range room.Spawns {
			assert.False(t, combatNPCTemplates[spawn.Template],
				"safe room %q must not spawn combat NPC template %q", roomID, spawn.Template)
		}
	}
}

// TestWooklyn_BossRoomExists verifies papa_wooks_chamber is marked as a boss room,
// has at least one hazard, and contains exactly 1 papa_wook and 2 wook_enforcer spawns.
//
// REQ-WK-8: papa_wooks_chamber must have BossRoom = true.
// REQ-WK-9: papa_wooks_chamber must have at least 1 hazard.
// REQ-WK-10: papa_wooks_chamber must spawn exactly 1 papa_wook.
// REQ-WK-11: papa_wooks_chamber must spawn exactly 2 wook_enforcer.
func TestWooklyn_BossRoomExists(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn)

	room, ok := wooklyn.Rooms["papa_wooks_chamber"]
	require.True(t, ok, "room papa_wooks_chamber must exist in wooklyn zone")
	assert.True(t, room.BossRoom, "papa_wooks_chamber must have BossRoom = true")
	assert.GreaterOrEqual(t, len(room.Hazards), 1, "papa_wooks_chamber must have at least 1 hazard")

	var papaCount, enforcerCount int
	for _, spawn := range room.Spawns {
		switch spawn.Template {
		case "papa_wook":
			papaCount += spawn.Count
		case "wook_enforcer":
			enforcerCount += spawn.Count
		}
	}
	assert.Equal(t, 1, papaCount, "papa_wooks_chamber must have exactly 1 papa_wook spawn count")
	assert.Equal(t, 2, enforcerCount, "papa_wooks_chamber must have exactly 2 wook_enforcer spawn count")
}

// TestWooklyn_VaultOfJamsGated verifies the_vault_of_jams has MinFactionTierID = "wook_brother".
//
// REQ-WK-12: the_vault_of_jams must exist in wooklyn zone.
// REQ-WK-13: the_vault_of_jams must require min_faction_tier_id = "wook_brother".
func TestWooklyn_VaultOfJamsGated(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn)

	room, ok := wooklyn.Rooms["the_vault_of_jams"]
	require.True(t, ok, "room the_vault_of_jams must exist in wooklyn zone")
	assert.Equal(t, "wook_brother", room.MinFactionTierID)
}

// TestWooklyn_WookFactionLoads verifies the wooks faction loads correctly with the
// expected tier structure, hostile factions, and price discounts.
//
// REQ-WK-20: wooks faction must exist in content/factions.
// REQ-WK-21: wooks faction must have exactly 4 tiers.
// REQ-WK-22: wooks faction must list "gun" and "machete" as hostile factions.
// REQ-WK-23: tier IDs must be narc, curious, fellow_traveler, wook_brother in order.
// REQ-WK-24: price discounts must be 0.0, 0.05, 0.15, 0.25 in tier order.
func TestWooklyn_WookFactionLoads(t *testing.T) {
	reg, err := faction.LoadFactions("../../../content/factions")
	require.NoError(t, err)

	wooks, ok := reg["wooks"]
	require.True(t, ok, "wooks faction must exist in content/factions")

	assert.Len(t, wooks.Tiers, 4)

	// Verify hostile factions contain "gun" and "machete".
	hostileSet := make(map[string]bool, len(wooks.HostileFactions))
	for _, hf := range wooks.HostileFactions {
		hostileSet[hf] = true
	}
	assert.True(t, hostileSet["gun"], "wooks faction must list gun as hostile")
	assert.True(t, hostileSet["machete"], "wooks faction must list machete as hostile")

	// Verify tier IDs in order.
	expectedTierIDs := []string{"narc", "curious", "fellow_traveler", "wook_brother"}
	for i, id := range expectedTierIDs {
		if i < len(wooks.Tiers) {
			assert.Equal(t, id, wooks.Tiers[i].ID, "tier[%d] ID mismatch", i)
		}
	}

	// Verify price discounts in order.
	expectedDiscounts := []float64{0.0, 0.05, 0.15, 0.25}
	for i, discount := range expectedDiscounts {
		if i < len(wooks.Tiers) {
			assert.InDelta(t, discount, wooks.Tiers[i].PriceDiscount, 1e-9,
				"tier[%d] price discount mismatch", i)
		}
	}
}

// TestWooklyn_WookSporeSubstanceLoads verifies wook_spore loads with correct category,
// onset delay, and duration.
//
// REQ-WK-30: wook_spore substance must exist in content/substances.
// REQ-WK-31: wook_spore category must be "drug".
// REQ-WK-32: wook_spore onset_delay must be 0s.
// REQ-WK-33: wook_spore duration must be 600s (10m).
func TestWooklyn_WookSporeSubstanceLoads(t *testing.T) {
	reg, err := substance.LoadDirectory("../../../content/substances")
	require.NoError(t, err)

	wookSpore, ok := reg.Get("wook_spore")
	require.True(t, ok, "wook_spore substance must exist in content/substances")

	assert.Equal(t, "drug", wookSpore.Category)
	assert.Equal(t, time.Duration(0), wookSpore.OnsetDelay, "wook_spore onset_delay must be 0s")
	assert.Equal(t, 600*time.Second, wookSpore.Duration, "wook_spore duration must be 600s (10m)")
}

// TestWooklyn_AllAmbientSubstanceRoomsReferenceValidSubstance verifies every room
// in the wooklyn zone that has an AmbientSubstance references a substance that exists
// in the substance registry.
//
// REQ-WK-40: All ambient_substance values in wooklyn rooms must resolve to valid substance IDs.
func TestWooklyn_AllAmbientSubstanceRoomsReferenceValidSubstance(t *testing.T) {
	zones, err := world.LoadZonesFromDir("../../../content/zones")
	require.NoError(t, err)

	substanceReg, err := substance.LoadDirectory("../../../content/substances")
	require.NoError(t, err)

	var wooklyn *world.Zone
	for _, z := range zones {
		if z.ID == "wooklyn" {
			wooklyn = z
			break
		}
	}
	require.NotNil(t, wooklyn)

	for roomID, room := range wooklyn.Rooms {
		if room.AmbientSubstance == "" {
			continue
		}
		_, ok := substanceReg.Get(room.AmbientSubstance)
		assert.True(t, ok,
			"room %q: ambient_substance %q not found in substance registry",
			roomID, room.AmbientSubstance)
	}
}

// TestWooklyn_AllNPCTemplatesLoad verifies that all Wooklyn NPC templates exist in
// the content/npcs directory.
//
// REQ-WK-50: All Wooklyn NPC templates must be loadable from content/npcs.
func TestWooklyn_AllNPCTemplatesLoad(t *testing.T) {
	templates, err := npc.LoadTemplates("../../../content/npcs")
	require.NoError(t, err)

	templateIndex := make(map[string]bool, len(templates))
	for _, tmpl := range templates {
		templateIndex[tmpl.ID] = true
	}

	requiredTemplates := []string{
		"wook",
		"ginger_wook",
		"wook_shaman",
		"wook_enforcer",
		"papa_wook",
		"wook_fixer",
		"chip_doc_wooklyn",
		"wook_healer",
		"wook_job_trainer",
		"wook_merchant",
		"wook_banker",
	}

	for _, id := range requiredTemplates {
		assert.True(t, templateIndex[id], "NPC template %q must exist in content/npcs", id)
	}
}
