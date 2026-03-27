package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/character"
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

	evt := svc.downtimeStart(uid, sess, "forge", "")
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

	evt := svc.downtimeStart(uid, sess, "forge", "")
	require.NotNil(t, evt)
	assert.True(t, sess.DowntimeBusy, "should start when supplies present")
	found := sess.Backpack.FindByItemDefID("forgery_supplies")
	assert.Empty(t, found, "forgery_supplies should be consumed on activity start")
}

// TestResolveForgePapers_CritSuccess_DeliverItemsAndRefund verifies that a critical success
// on forge_papers delivers one "undetectable_forgery" and one "forgery_supplies" refund.
//
// Precondition: sess.Backpack is empty; forced crit-success (roll=20, Flair=20, legendary hustle).
// Postcondition: backpack contains 1 undetectable_forgery and 1 forgery_supplies.
func TestResolveForgePapers_CritSuccess_DeliverItemsAndRefund(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Force crit success: roll=20, Flair=20 (+5 mod), legendary hustle (+8) → total=33 >= DC(15)+10=25.
	sess.Abilities = character.AbilityScores{Flair: 20}
	sess.Skills = map[string]string{"hustle": "legendary"}
	svc.dice = newFixedDiceRoller(20)

	svc.resolveForgePapers(uid, sess)

	undetectable := sess.Backpack.FindByItemDefID("undetectable_forgery")
	assert.Len(t, undetectable, 1, "should receive undetectable_forgery on crit success")
	refund := sess.Backpack.FindByItemDefID("forgery_supplies")
	assert.Len(t, refund, 1, "should receive forgery_supplies refund on crit success")
}

// TestResolveForgePapers_Success_DeliverConvincingForgery verifies that a success
// on forge_papers delivers one "convincing_forgery" and no other items.
//
// Precondition: sess.Backpack is empty; forced success (roll=15, Flair=10, no skill → total=15 >= DC, < DC+10).
// Postcondition: backpack contains 1 convincing_forgery; undetectable_forgery and forgery_supplies absent.
func TestResolveForgePapers_Success_DeliverConvincingForgery(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Force success: roll=15, Flair=10 (+0 mod), untrained hustle (+0) → total=15 == DC=15, < 25.
	sess.Abilities = character.AbilityScores{Flair: 10}
	sess.Skills = map[string]string{}
	svc.dice = newFixedDiceRoller(15)

	svc.resolveForgePapers(uid, sess)

	convincing := sess.Backpack.FindByItemDefID("convincing_forgery")
	assert.Len(t, convincing, 1, "should receive convincing_forgery on success")
	undetectable := sess.Backpack.FindByItemDefID("undetectable_forgery")
	assert.Empty(t, undetectable, "should not receive undetectable_forgery on mere success")
	refund := sess.Backpack.FindByItemDefID("forgery_supplies")
	assert.Empty(t, refund, "should not receive forgery_supplies refund on mere success")
}

// TestResolveForgePapers_Failure_NoItems verifies that a failure on forge_papers
// delivers no items to the backpack.
//
// Precondition: sess.Backpack is empty; forced failure (roll=5, Flair=10 → total=5 < DC=15, >= DC-10=5).
// Postcondition: backpack remains empty.
func TestResolveForgePapers_Failure_NoItems(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Force failure: roll=5, Flair=10 (+0 mod), no skill → total=5 < 15, >= 5 (not crit fail).
	sess.Abilities = character.AbilityScores{Flair: 10}
	sess.Skills = map[string]string{}
	svc.dice = newFixedDiceRoller(5)

	svc.resolveForgePapers(uid, sess)

	assert.Empty(t, sess.Backpack.FindByItemDefID("undetectable_forgery"), "no items on failure")
	assert.Empty(t, sess.Backpack.FindByItemDefID("convincing_forgery"), "no items on failure")
	assert.Empty(t, sess.Backpack.FindByItemDefID("forgery_supplies"), "no refund on failure")
}

// TestResolveForgePapers_CritFailure_NoItems verifies that a critical failure on forge_papers
// delivers no items to the backpack.
//
// Precondition: sess.Backpack is empty; forced crit-failure (roll=1, Flair=1 → total=-4 < DC-10=5).
// Postcondition: backpack remains empty.
func TestResolveForgePapers_CritFailure_NoItems(t *testing.T) {
	svc, uid := newForgePapersTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Force crit failure: roll=1, Flair=1 (-5 mod), no skill → total=-4 < DC(15)-10=5.
	sess.Abilities = character.AbilityScores{Flair: 1}
	sess.Skills = map[string]string{}
	svc.dice = newFixedDiceRoller(1)

	svc.resolveForgePapers(uid, sess)

	assert.Empty(t, sess.Backpack.FindByItemDefID("undetectable_forgery"), "no items on crit failure")
	assert.Empty(t, sess.Backpack.FindByItemDefID("convincing_forgery"), "no items on crit failure")
	assert.Empty(t, sess.Backpack.FindByItemDefID("forgery_supplies"), "no refund on crit failure")
}
