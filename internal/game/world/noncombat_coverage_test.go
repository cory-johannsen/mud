package world

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod not found)")
		}
		dir = parent
	}
}

func TestAllZonesHaveAtLeastOneSafeRoom(t *testing.T) {
	// new zones are intentionally all-dangerous per zones-new spec
	exemptZones := map[string]bool{
		"clown_camp":      true,
		"steampdx":        true,
		"the_velvet_rope": true,
		"club_privata":    true,
	}
	zonesDir := filepath.Join(repoRoot(t), "content", "zones")
	entries, err := os.ReadDir(zonesDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(zonesDir, entry.Name()))
		require.NoError(t, err)
		zone, err := LoadZoneFromBytes(data)
		require.NoError(t, err, "zone file: %s", entry.Name())
		if exemptZones[zone.ID] {
			continue
		}
		hasSafe := false
		for _, room := range zone.Rooms {
			if room.DangerLevel == "safe" {
				hasSafe = true
				break
			}
		}
		require.True(t, hasSafe, "zone %q must have at least one safe room (REQ-NCNAZ-1)", zone.ID)
	}
}

func TestAllZonesHaveRequiredNPCTypes(t *testing.T) {
	zones := []string{
		"aloha", "beaverton", "battleground", "downtown", "felony_flats",
		"hillsboro", "lake_oswego", "ne_portland", "pdx_international",
		"ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
		"the_couve", "troutdale", "vantucky",
	}
	required := []string{"merchant", "healer", "job_trainer", "banker"}
	root := repoRoot(t)
	for _, zoneID := range zones {
		path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
		data, err := os.ReadFile(path)
		require.NoError(t, err, "non_combat NPC file missing for zone %q", zoneID)
		var templates []struct {
			ID      string `yaml:"id"`
			NPCType string `yaml:"npc_type"`
		}
		require.NoError(t, yaml.Unmarshal(data, &templates))
		typeSet := make(map[string]bool)
		for _, tmpl := range templates {
			typeSet[tmpl.NPCType] = true
		}
		for _, req := range required {
			require.True(t, typeSet[req],
				"zone %q missing required npc_type %q (REQ-NCNAZ-4)", zoneID, req)
		}
	}
}

func TestNonCombatNPCTemplateIDs(t *testing.T) {
	zones := []string{
		"aloha", "beaverton", "battleground", "downtown", "felony_flats",
		"hillsboro", "lake_oswego", "ne_portland", "pdx_international",
		"ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
		"the_couve", "troutdale", "vantucky",
	}
	root := repoRoot(t)
	for _, zoneID := range zones {
		path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var templates []struct {
			ID      string `yaml:"id"`
			NPCType string `yaml:"npc_type"`
		}
		require.NoError(t, yaml.Unmarshal(data, &templates))
		for _, tmpl := range templates {
			expected := zoneID + "_" + tmpl.NPCType
			// tech_trainer allows tradition-prefixed IDs (e.g. {zone}_{tradition}_tech_trainer)
			// to support multiple trainers per zone across different tech traditions.
			if tmpl.NPCType == "tech_trainer" {
				prefix := zoneID + "_"
				suffix := "_" + tmpl.NPCType
				validExact := tmpl.ID == expected
				validPrefixed := strings.HasPrefix(tmpl.ID, prefix) && strings.HasSuffix(tmpl.ID, suffix)
				require.True(t, validExact || validPrefixed,
					"zone %q tech_trainer ID %q must be %q or match %q*%q (REQ-NCNAZ-7)", zoneID, tmpl.ID, expected, prefix, suffix)
			} else {
				require.Equal(t, expected, tmpl.ID,
					"zone %q template ID must be %q (REQ-NCNAZ-7)", zoneID, expected)
			}
		}
	}
}

func TestNonCombatNPCsNoQuestGiverOrCrafter(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "content", "npcs", "non_combat")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		var templates []struct {
			ID      string `yaml:"id"`
			NPCType string `yaml:"npc_type"`
		}
		require.NoError(t, yaml.Unmarshal(data, &templates))
		for _, tmpl := range templates {
			require.NotEqual(t, "quest_giver", tmpl.NPCType,
				"quest_giver template %q MUST NOT be placed (REQ-NCNAZ-6)", tmpl.ID)
			require.NotEqual(t, "crafter", tmpl.NPCType,
				"crafter template %q MUST NOT be placed (REQ-NCNAZ-6)", tmpl.ID)
		}
	}
}

func TestProperty_AllNonCombatTemplatesHaveNeutralDisposition(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "content", "npcs", "non_combat")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	type minTemplate struct {
		ID           string `yaml:"id"`
		Disposition  string `yaml:"disposition"`
		RespawnDelay string `yaml:"respawn_delay"`
	}
	var allTemplates []minTemplate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		var templates []minTemplate
		require.NoError(t, yaml.Unmarshal(data, &templates))
		allTemplates = append(allTemplates, templates...)
	}
	require.NotEmpty(t, allTemplates)
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(allTemplates)-1).Draw(rt, "idx")
		tmpl := allTemplates[idx]
		if tmpl.Disposition != "neutral" {
			rt.Fatalf("template %q: disposition must be 'neutral', got %q (REQ-NCNAZ-9)", tmpl.ID, tmpl.Disposition)
		}
		if tmpl.RespawnDelay != "" {
			rt.Fatalf("template %q: respawn_delay must be empty for permanent non-combat NPCs, got %q (REQ-NCNAZ-8)", tmpl.ID, tmpl.RespawnDelay)
		}
	})
}

func TestOptionalNPCTypesOnlyInAuthorizedZones(t *testing.T) {
	guardZones := map[string]bool{"aloha": true, "battleground": true, "beaverton": true, "downtown": true, "hillsboro": true, "pdx_international": true, "se_industrial": true, "the_couve": true}
	hirelingZones := map[string]bool{"beaverton": true, "hillsboro": true, "lake_oswego": true, "ross_island": true, "rustbucket_ridge": true, "se_industrial": true, "vantucky": true}
	fixerZones := map[string]bool{"aloha": true, "battleground": true, "beaverton": true, "downtown": true, "felony_flats": true, "hillsboro": true, "lake_oswego": true, "ne_portland": true, "pdx_international": true, "ross_island": true, "rustbucket_ridge": true, "sauvie_island": true, "se_industrial": true, "the_couve": true, "troutdale": true, "vantucky": true}
	brothelKeeperZones := map[string]bool{"aloha": true, "battleground": true, "beaverton": true, "downtown": true, "felony_flats": true, "hillsboro": true, "lake_oswego": true, "ne_portland": true, "pdx_international": true, "ross_island": true, "rustbucket_ridge": true, "sauvie_island": true, "se_industrial": true, "the_couve": true, "troutdale": true, "vantucky": true}

	zones := []string{
		"aloha", "beaverton", "battleground", "downtown", "felony_flats",
		"hillsboro", "lake_oswego", "ne_portland", "pdx_international",
		"ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
		"the_couve", "troutdale", "vantucky",
	}
	root := repoRoot(t)
	for _, zoneID := range zones {
		path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var templates []struct {
			ID      string `yaml:"id"`
			NPCType string `yaml:"npc_type"`
		}
		require.NoError(t, yaml.Unmarshal(data, &templates))
		for _, tmpl := range templates {
			switch tmpl.NPCType {
			case "guard":
				require.True(t, guardZones[zoneID], "guard %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
			case "hireling":
				require.True(t, hirelingZones[zoneID], "hireling %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
			case "fixer":
				require.True(t, fixerZones[zoneID], "fixer %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
			case "brothel_keeper":
				require.True(t, brothelKeeperZones[zoneID], "brothel_keeper %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
			}
		}
	}
}

func TestZoneMapRoomIsSafe(t *testing.T) {
	zonesDir := filepath.Join(repoRoot(t), "content", "zones")
	entries, err := os.ReadDir(zonesDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(zonesDir, entry.Name()))
		require.NoError(t, err)
		zone, err := LoadZoneFromBytes(data)
		require.NoError(t, err, "zone file: %s", entry.Name())
		for _, room := range zone.Rooms {
			for _, eq := range room.Equipment {
				if eq.ItemID == "zone_map" {
					require.Equal(t, "safe", room.DangerLevel,
						"zone %q room %q contains zone_map equipment but is not danger_level: safe (BUG-26)",
						zone.ID, room.ID)
				}
			}
		}
	}
}

func TestNewSafeRoomsConnectedBidirectionally(t *testing.T) {
	type safeRoomSpec struct {
		zoneFile    string
		newRoomID   string
		anchorID    string
		anchorToNew string
		newToAnchor string
	}
	specs := []safeRoomSpec{
		{"beaverton", "beav_free_market", "beav_canyon_road_east", "north", "south"},
		{"downtown", "downtown_underground", "morrison_bridge", "north", "south"},
		{"hillsboro", "hills_the_keep", "hills_tv_highway_east", "south", "north"},
		{"ne_portland", "ne_corner_store", "ne_alberta_street", "north", "south"},
		{"pdx_international", "pdx_terminal_b", "pdx_airport_way_west", "south", "north"},
		{"ross_island", "ross_dock_shack", "ross_bridge_east", "east", "west"},
		{"rustbucket_ridge", "rust_scrap_office", "last_stand_lodge", "east", "west"},
		{"sauvie_island", "sauvie_farm_stand", "sauvie_bridge_south", "south", "north"},
		{"se_industrial", "sei_break_room", "sei_holgate_blvd", "east", "west"},
		{"the_couve", "couve_the_crossing", "couve_interstate_bridge_south", "west", "east"},
		{"troutdale", "trout_truck_stop", "trout_i84_west", "north", "south"},
		{"vantucky", "vantucky_the_compound", "vantucky_fourth_plain_west", "north", "south"},
	}
	root := repoRoot(t)
	for _, s := range specs {
		data, err := os.ReadFile(filepath.Join(root, "content", "zones", s.zoneFile+".yaml"))
		require.NoError(t, err)
		zone, err := LoadZoneFromBytes(data)
		require.NoError(t, err)

		newRoom, ok := zone.Rooms[s.newRoomID]
		require.True(t, ok, "zone %q missing new safe room %q (REQ-NCNAZ-1)", s.zoneFile, s.newRoomID)
		require.Equal(t, "safe", newRoom.DangerLevel, "new room %q must have danger_level: safe (REQ-NCNAZ-1)", s.newRoomID)

		hasReverseExit := false
		for _, exit := range newRoom.Exits {
			if string(exit.Direction) == s.newToAnchor && exit.TargetRoom == s.anchorID {
				hasReverseExit = true
				break
			}
		}
		require.True(t, hasReverseExit, "room %q must have %s exit to %q (REQ-NCNAZ-13)", s.newRoomID, s.newToAnchor, s.anchorID)

		anchor, ok := zone.Rooms[s.anchorID]
		require.True(t, ok, "zone %q missing anchor room %q", s.zoneFile, s.anchorID)
		hasForwardExit := false
		for _, exit := range anchor.Exits {
			if string(exit.Direction) == s.anchorToNew && exit.TargetRoom == s.newRoomID {
				hasForwardExit = true
				break
			}
		}
		require.True(t, hasForwardExit, "anchor %q must have %s exit to %q (REQ-NCNAZ-13)", s.anchorID, s.anchorToNew, s.newRoomID)
	}
}

func TestNewSafeRoomDescriptions(t *testing.T) {
	type roomDesc struct {
		zoneFile string
		roomID   string
		desc     string
	}
	cases := []roomDesc{
		{"beaverton", "beav_free_market", "An open-air block of vendor stalls under corrugated aluminum roofing. The smell of hot food and machine oil. People come here to trade, not fight."},
		{"downtown", "downtown_underground", "A repurposed parking garage two levels below street level. Strip lighting, folding tables, and the low hum of people who need things and people who have them."},
		{"hillsboro", "hills_the_keep", "A fortified community hall at the edge of the Hillsboro enclave. Stone walls and firelight. A place of order, or something close to it."},
		{"ne_portland", "ne_corner_store", "A converted convenience store with the shelving pushed to the walls. Locals come here to restock, get patched up, and hear what's going on in the neighborhood."},
		{"pdx_international", "pdx_terminal_b", "A section of the airport terminal cordoned off from the main concourse. Chairs bolted to the floor, vending machines that still work, and people who've learned to wait."},
		{"ross_island", "ross_dock_shack", "A weathered shack at the island's main landing. Nets hang on the walls, a woodstove burns in the corner, and someone is always willing to do business."},
		{"rustbucket_ridge", "rust_scrap_office", "A repurposed foreman's office at the edge of the ridge. Metal desk, fluorescent light, and a corkboard full of job postings nobody's taken down."},
		{"sauvie_island", "sauvie_farm_stand", "A roadside stand that evolved into a community hub. Folding tables with produce, herbs, and handmade goods. Calm enough that people leave their weapons at the door."},
		{"se_industrial", "sei_break_room", "A cinder-block room with folding chairs and a microwave that runs off a generator. Shift workers and traders share the same coffee and the same fatigue."},
		{"the_couve", "couve_the_crossing", "A checkpoint building at the Washington end of the bridge. The Couve faction controls it, but they're practical: trade is welcome, trouble is not."},
		{"troutdale", "trout_truck_stop", "A diesel-soaked rest stop with a diner counter, a parts wall, and a back room where deals get made. Everyone passes through Troutdale eventually."},
		{"vantucky", "vantucky_the_compound", "The Vantucky militia's main compound. Spare and functional. They'll trade, train, and bank here — loyalty is assumed, not enforced."},
	}
	root := repoRoot(t)
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(root, "content", "zones", c.zoneFile+".yaml"))
		require.NoError(t, err)
		zone, err := LoadZoneFromBytes(data)
		require.NoError(t, err)
		found, ok := zone.Rooms[c.roomID]
		require.True(t, ok, "room %q not found in zone %q", c.roomID, c.zoneFile)
		require.Equal(t, c.desc, strings.TrimSpace(found.Description),
			"room %q description mismatch (REQ-NCNAZ-3)", c.roomID)
	}
}
