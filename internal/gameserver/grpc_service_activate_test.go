package gameserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// newActivateTestServer creates a server with an invRegistry and a player
// session with no equipped items (so any item query returns "not found").
func newActivateTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	reg := inventory.NewRegistry()
	_ = reg.RegisterItem(&inventory.ItemDef{
		ID:             "stim_rod",
		Name:           "Stim Rod",
		ActivationCost: 2,
		Charges:        3,
		ActivationEffect: &inventory.ConsumableEffect{Heal: "1d4"},
		OnDeplete:      "destroy",
	})
	svc.invRegistry = reg

	uid := "act_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "act_user", CharName: "ActChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// TestHandleActivate_NotFound verifies that activating a non-existent item
// returns a narrative error event (no Go error).
// The handler uses errorEvent() which wraps the message in ServerEvent_Error.
func TestHandleActivate_NotFound(t *testing.T) {
	svc, uid := newActivateTestServer(t)

	evt, err := svc.handleActivate(uid, &gamev1.ActivateItemRequest{ItemQuery: "nonexistent_item"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotEmpty(t, evt.GetError().GetMessage())
}

// TestHandleActivate_SessionNotFound verifies that an unknown UID returns an
// error narrative rather than a Go error.
func TestHandleActivate_SessionNotFound(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	evt, err := svc.handleActivate("nobody", &gamev1.ActivateItemRequest{ItemQuery: "stim_rod"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotEmpty(t, evt.GetError().GetMessage())
}

// TestHandleActivate_ZoneMapScript_PassesUID verifies that activating an item
// with an ActivationScript passes the player uid to the Lua hook, enabling
// engine.map.reveal_zone to populate the player's AutomapCache (BUG-15).
//
// Precondition: zone "test" has two rooms; item ActivationScript="zone_map_use";
//
//	player session exists in zone "test".
//
// Postcondition: After handleActivate, AutomapCache["test"] contains all zone room IDs.
func TestHandleActivate_ZoneMapScript_PassesUID(t *testing.T) {
	// Build a script manager with a zone_map_use Lua function that calls reveal_zone with its uid.
	luaSrc := `
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "test")
    return "You study the map carefully."
end
`
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)
	scriptMgr := scripting.NewManager(roller, logger)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zone_map.lua"), []byte(luaSrc), 0644))
	require.NoError(t, scriptMgr.LoadZone("test", dir, 0))

	worldMgr, sessMgr := testWorldAndSession(t)

	// Register a zone map item that uses an ActivationScript.
	// ActivationCost must be > 0 so HandleActivate treats it as activatable.
	// MaxStack must be >= 1 per ItemDef.Validate.
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID:               "zone_map",
		Name:             "Zone Map",
		Kind:             inventory.KindJunk,
		ActivationCost:   1,
		Charges:          1,
		MaxStack:         1,
		ActivationScript: "zone_map_use",
	}))

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		nil, nil, nil, logger,
		nil, roller, nil, npc.NewManager(), nil, scriptMgr,
		nil, nil, nil, nil, reg, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	const uid = "act_map_uid"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Mapper", CharName: "Mapper",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Add zone map to backpack and equip it in an accessory slot so HandleActivate resolves it.
	// ChargesRemaining must be -1 (uninitialized sentinel) so HandleActivate initialises it from def.Charges.
	inst, addErr := sess.Backpack.Add("zone_map", 1, reg)
	require.NoError(t, addErr)
	inst.ChargesRemaining = -1
	sess.Equipment.Accessories[inventory.SlotNeck] = &inventory.SlottedItem{
		ItemDefID:  "zone_map",
		Name:       "Zone Map",
		InstanceID: inst.InstanceID,
	}

	evt, err := svc.handleActivate(uid, &gamev1.ActivateItemRequest{ItemQuery: "zone_map"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	// Must not return an error event.
	assert.Empty(t, evt.GetError().GetMessage(), "activate must succeed, not return an error")

	// wireRevealZone populates AutomapCache via the RevealZoneMap callback.
	// Both rooms in zone "test" must be revealed after the zone map is activated.
	assert.Contains(t, sess.AutomapCache["test"], "room_a", "room_a must be revealed (BUG-15)")
	assert.Contains(t, sess.AutomapCache["test"], "room_b", "room_b must be revealed (BUG-15)")
}
