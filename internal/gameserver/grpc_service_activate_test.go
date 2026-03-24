package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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
