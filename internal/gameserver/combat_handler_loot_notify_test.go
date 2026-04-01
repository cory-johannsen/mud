package gameserver

// REQ-BUG67-1: After an NPC with a loot table dies, each living combat participant MUST
// receive a MessageEvent containing the names of items dropped on the floor.
// REQ-BUG67-2: The loot notification MUST be sent per living participant, not broadcast.
// REQ-BUG67-3: When invRegistry is nil or an item def is not found, the item's ItemDefID
// MUST be used as the display name so the message is never suppressed.
// REQ-BUG67-4: When no items are generated (e.g. loot table has only currency), no loot
// notification message MUST be sent.

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// makeLootNotifyHandler constructs a CombatHandler with an inventory registry,
// npc manager, session manager, and optional floor manager for loot notification tests.
//
// Precondition: reg and floorMgr may be nil.
// Postcondition: Returns a non-nil CombatHandler configured with the given registry and floor manager.
func makeLootNotifyHandler(
	t *testing.T,
	reg *inventory.Registry,
	floorMgr *inventory.FloorManager,
	broadcastFn func(string, []*gamev1.CombatEvent),
) *CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgrLocal := npc.NewManager()
	sessMgrLocal := session.NewManager()
	return NewCombatHandler(
		engine, npcMgrLocal, sessMgrLocal, roller, broadcastFn,
		testRoundDuration,
		makeTestConditionRegistry(),
		nil, nil,
		reg,
		nil,
		nil,
		floorMgr,
		nil,
	)
}

// drainMessageContents drains all pending MessageEvent contents from a BridgeEntity
// channel within the given timeout.
//
// Precondition: ch must be a valid BridgeEntity events channel.
// Postcondition: Returns all MessageEvent content strings received before timeout.
func drainMessageContents(t *testing.T, ch <-chan []byte, timeout time.Duration) []string {
	t.Helper()
	var contents []string
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return contents
			}
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			if m, ok := evt.Payload.(*gamev1.ServerEvent_Message); ok {
				contents = append(contents, m.Message.Content)
			}
		case <-deadline.C:
			return contents
		}
	}
}

// equipLootTestPistol registers a pistol loadout for uid on handler h.
// The pistol has RangeIncrement=30 so attacks succeed at the initial combat distance.
//
// Precondition: h and uid must be non-nil/non-empty.
// Postcondition: uid's loadout is set to a pistol preset with a full magazine.
func equipLootTestPistol(t *testing.T, h *CombatHandler, uid string) {
	t.Helper()
	def := &inventory.WeaponDef{
		ID: "loot-test-pistol", Name: "Test Pistol",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 30, ReloadActions: 1, MagazineCapacity: 30,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
	preset := inventory.NewWeaponPreset()
	if err := preset.EquipMainHand(def); err != nil {
		t.Fatalf("equipLootTestPistol: %v", err)
	}
	h.RegisterLoadout(uid, preset)
}

// waitForNPCRemovedInternal polls npcMgr until the instance is gone or timeout elapses.
//
// Precondition: npcMgr must not be nil; timeout must be > 0.
// Postcondition: Returns true when the instance is no longer present within timeout.
func waitForNPCRemovedInternal(t *testing.T, nm *npc.Manager, instID string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, ok := nm.Get(instID); !ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestLootNotification_ItemNamesResolvedViaRegistry verifies that after an NPC with a
// loot table dies, a loot notification message containing the item's display name is
// pushed to the surviving player's entity stream (REQ-BUG67-1, REQ-BUG67-2).
//
// Precondition: NPC has a guaranteed item loot entry; player session has a BridgeEntity;
// invRegistry has the item definition with a display name.
// Postcondition: Player receives a MessageEvent whose Content contains "You looted:" and the item name.
func TestLootNotification_ItemNamesResolvedViaRegistry(t *testing.T) {
	reg := inventory.NewRegistry()
	err := reg.RegisterItem(&inventory.ItemDef{
		ID:       "tactical_boots",
		Name:     "Tactical Boots",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	})
	require.NoError(t, err)

	floorMgr := inventory.NewFloorManager()

	var broadcastMu sync.Mutex
	var broadcastEvts [][]*gamev1.CombatEvent
	broadcastFn := func(_ string, evts []*gamev1.CombatEvent) {
		broadcastMu.Lock()
		defer broadcastMu.Unlock()
		broadcastEvts = append(broadcastEvts, evts)
	}

	h := makeLootNotifyHandler(t, reg, floorMgr, broadcastFn)
	const roomID = "room-loot-notify-1"

	tmpl := &npc.Template{
		ID:       "loot-notify-npc",
		Name:     "LootNotifyGanger",
		Level:    1,
		MaxHP:    1,
		AC:       1,
		Awareness: 2,
		Loot: &npc.LootTable{
			Items: []npc.ItemDrop{
				{ItemID: "tactical_boots", Chance: 1.0, MinQty: 1, MaxQty: 1},
			},
		},
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess, addErr := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         "player-loot-notify-1",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   100,
		MaxHP:       100,
		Role:        "player",
	})
	require.NoError(t, addErr)
	sess.Entity = session.NewBridgeEntity("player-loot-notify-1", 256)

	equipLootTestPistol(t, h,"player-loot-notify-1")

	_, err = h.Attack("player-loot-notify-1", "LootNotifyGanger")
	require.NoError(t, err)

	removed := waitForNPCRemovedInternal(t, h.npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC to be removed")

	msgs := drainMessageContents(t, sess.Entity.Events(), 500*time.Millisecond)

	lootMsgs := []string{}
	for _, m := range msgs {
		if strings.Contains(m, "You looted:") {
			lootMsgs = append(lootMsgs, m)
		}
	}
	require.Len(t, lootMsgs, 1, "expected exactly one loot notification message; got: %v", msgs)
	assert.Contains(t, lootMsgs[0], "Tactical Boots", "expected item display name in loot message")
}

// TestLootNotification_FallbackToItemDefID verifies that when invRegistry is nil,
// the item's ItemDefID is used as the display name (REQ-BUG67-3).
//
// Precondition: CombatHandler has nil invRegistry; NPC has a guaranteed item loot entry.
// Postcondition: Player receives a loot notification message containing the ItemDefID.
func TestLootNotification_FallbackToItemDefID(t *testing.T) {
	floorMgr := inventory.NewFloorManager()

	var broadcastMu sync.Mutex
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {
		broadcastMu.Lock()
		defer broadcastMu.Unlock()
	}

	// nil registry — names fall back to ItemDefID
	h := makeLootNotifyHandler(t, nil, floorMgr, broadcastFn)
	const roomID = "room-loot-notify-fallback"

	tmpl := &npc.Template{
		ID:       "loot-notify-npc-fallback",
		Name:     "FallbackGanger",
		Level:    1,
		MaxHP:    1,
		AC:       1,
		Awareness: 2,
		Loot: &npc.LootTable{
			Items: []npc.ItemDrop{
				{ItemID: "stim_pack", Chance: 1.0, MinQty: 2, MaxQty: 2},
			},
		},
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess, addErr := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         "player-loot-fallback",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   100,
		MaxHP:       100,
		Role:        "player",
	})
	require.NoError(t, addErr)
	sess.Entity = session.NewBridgeEntity("player-loot-fallback", 256)

	equipLootTestPistol(t, h,"player-loot-fallback")

	_, err = h.Attack("player-loot-fallback", "FallbackGanger")
	require.NoError(t, err)

	removed := waitForNPCRemovedInternal(t, h.npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC to be removed")

	msgs := drainMessageContents(t, sess.Entity.Events(), 500*time.Millisecond)

	lootMsgs := []string{}
	for _, m := range msgs {
		if strings.Contains(m, "You looted:") {
			lootMsgs = append(lootMsgs, m)
		}
	}
	require.Len(t, lootMsgs, 1, "expected exactly one loot notification message; got: %v", msgs)
	assert.Contains(t, lootMsgs[0], "stim_pack", "expected ItemDefID as fallback name in loot message")
}

// TestLootNotification_CurrencyOnlyNoItemMessage verifies that when the loot table
// produces only currency and no items, no loot notification message is sent (REQ-BUG67-4).
//
// Precondition: NPC loot table has only currency (no items); player session has a BridgeEntity.
// Postcondition: No MessageEvent containing "You looted:" is pushed to the player.
func TestLootNotification_CurrencyOnlyNoItemMessage(t *testing.T) {
	floorMgr := inventory.NewFloorManager()

	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}

	h := makeLootNotifyHandler(t, nil, floorMgr, broadcastFn)
	const roomID = "room-loot-notify-currency-only"

	tmpl := &npc.Template{
		ID:       "loot-currency-only",
		Name:     "CurrencyGanger",
		Level:    1,
		MaxHP:    1,
		AC:       1,
		Awareness: 2,
		Loot: &npc.LootTable{
			Currency: &npc.CurrencyDrop{Min: 10, Max: 10},
		},
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	sess, addErr := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         "player-loot-currency-only",
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   100,
		MaxHP:       100,
		Role:        "player",
	})
	require.NoError(t, addErr)
	sess.Entity = session.NewBridgeEntity("player-loot-currency-only", 256)

	equipLootTestPistol(t, h,"player-loot-currency-only")

	_, err = h.Attack("player-loot-currency-only", "CurrencyGanger")
	require.NoError(t, err)

	removed := waitForNPCRemovedInternal(t, h.npcMgr, inst.ID, 3*time.Second)
	require.True(t, removed, "expected dead NPC to be removed")

	msgs := drainMessageContents(t, sess.Entity.Events(), 500*time.Millisecond)

	for _, m := range msgs {
		if strings.Contains(m, "You looted:") {
			t.Errorf("unexpected loot message when only currency was dropped: %q", m)
		}
	}
}
