package gameserver_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const lootTestRoundDuration = 200 * time.Millisecond

// waitForNPCRemoved polls npcMgr until the given instance ID is no longer present
// or the timeout elapses. This ensures the timer goroutine has finished
// removeDeadNPCsLocked (and loot generation) before assertions run.
//
// Precondition: npcMgr must not be nil; timeout must be > 0.
// Postcondition: Returns true if the NPC was removed within the timeout.
func waitForNPCRemoved(t *testing.T, npcMgr *npc.Manager, instID string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, ok := npcMgr.Get(instID); !ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// makeLootHandler constructs a CombatHandler with the given npcMgr, respawnMgr, and floorMgr.
//
// Precondition: npcMgr and sessMgr must be non-nil.
// Postcondition: Returns a non-nil CombatHandler.
func makeLootHandler(
	t *testing.T,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	respawnMgr *npc.RespawnManager,
	floorMgr *inventory.FloorManager,
	broadcastFn func(string, []*gamev1.CombatEvent),
) *gameserver.CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	return gameserver.NewCombatHandler(
		engine, npcMgr, sessMgr, roller, broadcastFn,
		lootTestRoundDuration,
		makeRespawnTestConditionRegistry(),
		nil, nil, nil, nil,
		respawnMgr,
		floorMgr,
	)
}

// TestCombatHandler_LootGeneration_CurrencyAndItems verifies that when an NPC
// with a guaranteed loot table dies in combat, currency is awarded to the
// surviving player and items are dropped on the room floor.
//
// Postcondition: Player currency increases by the loot table amount; floor
// contains the expected item; NPC instance is removed from npcMgr.
func TestCombatHandler_LootGeneration_CurrencyAndItems(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	floorMgr := inventory.NewFloorManager()

	const roomID = "room-loot-1"
	const templateID = "loot-ganger"

	tmpl := &npc.Template{
		ID:           templateID,
		Name:         "LootGanger",
		Level:        1,
		MaxHP:        1, // dies in one hit
		AC:           1, // very low so player always hits
		Perception:   2,
		RespawnDelay: "1m",
		Loot: &npc.LootTable{
			Currency: &npc.CurrencyDrop{Min: 100, Max: 100},
			Items: []npc.ItemDrop{
				{ItemID: "junk-scrap", Chance: 1.0, MinQty: 1, MaxQty: 1},
			},
		},
	}

	spawns := map[string][]npc.RoomSpawn{
		roomID: {{TemplateID: templateID, Max: 1, RespawnDelay: time.Minute}},
	}
	templates := map[string]*npc.Template{templateID: tmpl}
	respawnMgr := npc.NewRespawnManager(spawns, templates)

	broadcastFn, getEvents := makeBroadcastCapture()
	h := makeLootHandler(t, npcMgr, sessMgr, respawnMgr, floorMgr, broadcastFn)

	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess := addRespawnTestPlayer(t, sessMgr, "player-loot-1", roomID, 100)
	initialCurrency := sess.Currency

	_, err = h.Attack("player-loot-1", "LootGanger")
	require.NoError(t, err)

	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	// Wait for NPC removal which happens after loot generation in the timer goroutine.
	removed := waitForNPCRemoved(t, npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC instance to be removed from npcMgr")

	// Player must have received currency.
	assert.Equal(t, initialCurrency+100, sess.Currency, "expected player to receive 100 currency from loot")

	// Floor must have the dropped item.
	items := floorMgr.ItemsInRoom(roomID)
	require.Len(t, items, 1, "expected exactly one item on the floor")
	assert.Equal(t, "junk-scrap", items[0].ItemDefID)
	assert.Equal(t, 1, items[0].Quantity)
	assert.NotEmpty(t, items[0].InstanceID, "expected item to have a non-empty InstanceID")
}

// TestCombatHandler_LootGeneration_NilFloorMgr_NoPanic verifies that loot
// generation does not panic when floorMgr is nil.
//
// Postcondition: No panic; currency is still awarded to the player.
func TestCombatHandler_LootGeneration_NilFloorMgr_NoPanic(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()

	const roomID = "room-loot-nil"
	const templateID = "loot-ganger-nil"

	tmpl := &npc.Template{
		ID:           templateID,
		Name:         "LootGangerNil",
		Level:        1,
		MaxHP:        1,
		AC:           1,
		Perception:   2,
		RespawnDelay: "1m",
		Loot: &npc.LootTable{
			Currency: &npc.CurrencyDrop{Min: 50, Max: 50},
			Items: []npc.ItemDrop{
				{ItemID: "junk-scrap", Chance: 1.0, MinQty: 1, MaxQty: 1},
			},
		},
	}

	spawns := map[string][]npc.RoomSpawn{
		roomID: {{TemplateID: templateID, Max: 1, RespawnDelay: time.Minute}},
	}
	templates := map[string]*npc.Template{templateID: tmpl}
	respawnMgr := npc.NewRespawnManager(spawns, templates)

	broadcastFn, getEvents := makeBroadcastCapture()
	// Pass nil floorMgr to exercise the nil guard.
	h := makeLootHandler(t, npcMgr, sessMgr, respawnMgr, nil, broadcastFn)

	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess := addRespawnTestPlayer(t, sessMgr, "player-loot-nil", roomID, 100)

	_, err = h.Attack("player-loot-nil", "LootGangerNil")
	require.NoError(t, err)

	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	// Wait for NPC removal to ensure loot generation has completed.
	removed := waitForNPCRemoved(t, npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC to be removed")

	// Currency should still be awarded even without a floor manager.
	assert.Equal(t, 50, sess.Currency, "expected player to receive 50 currency from loot")
}

// TestCombatHandler_NoLootTable_NoEffect verifies that NPCs without a loot
// table do not generate any loot or crash.
//
// Postcondition: No panic; player currency remains 0; floor is empty.
func TestCombatHandler_NoLootTable_NoEffect(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	floorMgr := inventory.NewFloorManager()

	const roomID = "room-no-loot"

	broadcastFn, getEvents := makeBroadcastCapture()
	h := makeLootHandler(t, npcMgr, sessMgr, nil, floorMgr, broadcastFn)

	tmpl := &npc.Template{
		ID:         "no-loot-ganger",
		Name:       "NoLootGanger",
		Level:      1,
		MaxHP:      1,
		AC:         1,
		Perception: 2,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess := addRespawnTestPlayer(t, sessMgr, "player-no-loot", roomID, 100)

	_, err = h.Attack("player-no-loot", "NoLootGanger")
	require.NoError(t, err)

	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	// Wait for NPC removal to ensure removeDeadNPCsLocked has completed.
	removed := waitForNPCRemoved(t, npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC to be removed")

	assert.Equal(t, 0, sess.Currency, "expected no currency when NPC has no loot table")
	items := floorMgr.ItemsInRoom(roomID)
	assert.Empty(t, items, "expected no items on floor when NPC has no loot table")
}
