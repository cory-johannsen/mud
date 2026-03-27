package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// newForgePapersTestSvc creates a GameServiceServer with a safe-tagged room and a player session.
//
// Precondition: none.
// Postcondition: Returns svc and uid; GetPlayer(uid) succeeds; room has "safe" tag.
func newForgePapersTestSvc(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	const uid = "fp_uid"
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession(uid, "room1")
	svc := &GameServiceServer{sessions: sMgr, world: wMgr}
	return svc, uid
}

// TestDowntimeStart_ForgePapers_BlockedWithoutSupplies verifies that starting the forge_papers
// activity without forgery_supplies in the backpack returns an error message and does not set
// DowntimeBusy.
//
// Precondition: sess.DowntimeBusy=false; backpack is empty.
// Postcondition: sess.DowntimeBusy remains false; message contains "forgery supplies".
func TestDowntimeStart_ForgePapers_BlockedWithoutSupplies(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.DowntimeBusy = false

	evt := svc.downtimeStart(uid, sess, "forge")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a message event")
	assert.Contains(t, msg.Content, "forgery supplies")
	assert.False(t, sess.DowntimeBusy, "should not start without supplies")
}

// TestDowntimeStart_ForgePapers_ConsumesSupplies verifies that starting the forge_papers
// activity with one forgery_supplies in the backpack sets DowntimeBusy and removes the supply.
//
// Precondition: sess.DowntimeBusy=false; backpack contains one forgery_supplies instance.
// Postcondition: sess.DowntimeBusy==true; backpack contains zero forgery_supplies instances.
func TestDowntimeStart_ForgePapers_ConsumesSupplies(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.DowntimeBusy = false
	require.NoError(t, sess.Backpack.AddInstance(&inventory.ItemInstance{
		InstanceID: "fs-1",
		ItemDefID:  "forgery_supplies",
		Quantity:   1,
	}))

	evt := svc.downtimeStart(uid, sess, "forge")
	require.NotNil(t, evt)
	assert.True(t, sess.DowntimeBusy, "should start when supplies present")
	found := sess.Backpack.FindByItemDefID("forgery_supplies")
	assert.Empty(t, found, "forgery_supplies should be consumed on activity start")
}
