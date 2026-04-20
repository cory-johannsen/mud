package scripting_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/scripting"
)

// TestGetFactionEnemies_NPCWithFactionSeesHostiles verifies that get_faction_enemies
// returns combatants from hostile factions (REQ-CCF-2a).
func TestGetFactionEnemies_NPCWithFactionSeesHostiles(t *testing.T) {
	mgr, _ := newTestManager(t)

	actorUID := "actor-1"
	hostileUID := "hostile-1"
	allyUID := "ally-1"

	mgr.GetEntityRoom = func(uid string) string { return "room-1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 50, MaxHP: 50, FactionID: "just_clownin"},
			{UID: hostileUID, Kind: "npc", HP: 40, MaxHP: 40, FactionID: "queer_clowning_experience"},
			{UID: allyUID, Kind: "npc", HP: 30, MaxHP: 30, FactionID: "just_clownin"},
		}
	}
	mgr.GetFactionHostiles = func(factionID string) []string {
		if factionID == "just_clownin" {
			return []string{"queer_clowning_experience", "unwoke_maga_clown_army"}
		}
		return nil
	}

	luaSrc := `
function test_faction_enemies(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`
	dir := writeTempLua(t, "faction_test.lua", luaSrc)
	zoneID := "modtest_faction_" + t.Name()
	require.NoError(t, mgr.LoadZone(zoneID, dir, 0))

	result, err := mgr.CallHook(zoneID, "test_faction_enemies", lua.LString(actorUID))
	require.NoError(t, err)
	assert.EqualValues(t, 1, result, "should return exactly 1 faction enemy (the QCE NPC)")
}

// TestGetFactionEnemies_AlreadyInCombatStillIncluded verifies that a hostile-faction
// combatant already in combat with uid is still returned (REQ-CCF-2c).
func TestGetFactionEnemies_AlreadyInCombatStillIncluded(t *testing.T) {
	mgr, _ := newTestManager(t)

	actorUID := "jc-1"
	qceUID := "qce-1"

	mgr.GetEntityRoom = func(uid string) string { return "room-stage" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 200, MaxHP: 546, FactionID: "just_clownin"},
			{UID: qceUID, Kind: "npc", HP: 100, MaxHP: 528, FactionID: "queer_clowning_experience"},
		}
	}
	mgr.GetFactionHostiles = func(factionID string) []string {
		if factionID == "just_clownin" {
			return []string{"queer_clowning_experience"}
		}
		return nil
	}

	luaSrc := `
function count_faction_enemies(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`
	dir := writeTempLua(t, "faction_already_in_combat_test.lua", luaSrc)
	zoneID := "modtest_faction_" + t.Name()
	require.NoError(t, mgr.LoadZone(zoneID, dir, 0))

	result, err := mgr.CallHook(zoneID, "count_faction_enemies", lua.LString(actorUID))
	require.NoError(t, err)
	assert.EqualValues(t, 1, result, "hostile already in combat must still appear in get_faction_enemies")
}

// TestGetFactionEnemies_NoFactionReturnsEmpty verifies that an NPC with no faction
// always returns an empty table (REQ-CCF-2b).
func TestGetFactionEnemies_NoFactionReturnsEmpty(t *testing.T) {
	mgr, _ := newTestManager(t)

	actorUID := "actor-1"
	mgr.GetEntityRoom = func(uid string) string { return "room-1" }
	mgr.GetCombatantsInRoom = func(roomID string) []*scripting.CombatantInfo {
		return []*scripting.CombatantInfo{
			{UID: actorUID, Kind: "npc", HP: 50, MaxHP: 50, FactionID: ""},
			{UID: "other-1", Kind: "npc", HP: 40, MaxHP: 40, FactionID: "queer_clowning_experience"},
		}
	}
	mgr.GetFactionHostiles = func(factionID string) []string { return nil }

	luaSrc := `
function test_no_faction(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count
end
`
	dir := writeTempLua(t, "faction_test.lua", luaSrc)
	zoneID := "modtest_faction_" + t.Name()
	require.NoError(t, mgr.LoadZone(zoneID, dir, 0))

	result, err := mgr.CallHook(zoneID, "test_no_faction", lua.LString(actorUID))
	require.NoError(t, err)
	assert.EqualValues(t, 0, result, "NPC with no faction should see no faction enemies")
}
