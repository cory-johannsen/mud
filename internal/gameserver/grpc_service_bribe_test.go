package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newBribeTestServer builds a GameServiceServer with a test world, session manager,
// and NPC manager. The player is placed in "room_a" with WantedLevel 2 in zone "zone_a".
//
// Precondition: t must be non-nil.
// Postcondition: Returns a configured server and the player UID.
func newBribeTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "bribe_u1"
	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "bribe_user", CharName: "BribeChar",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 1000
	// zone_a is the zone containing room_a (see testWorldAndSession / world_handler_test.go)
	// "room_a" is in zone "test" (see testWorldAndSession).
	sess.WantedLevel["test"] = 2
	return svc, uid
}

// spawnFixer spawns a fixer NPC in "room_a" on the given server and returns the instance.
//
// Precondition: svc must be non-nil.
// Postcondition: Returns a spawned fixer instance with BaseCosts for levels 1-4.
func spawnFixer(t *testing.T, svc *GameServiceServer) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:      "test_fixer",
		Name:    "Remy",
		NPCType: "fixer",
		Level:   3,
		MaxHP:   20,
		AC:      12,
		Fixer: &npc.FixerConfig{
			BaseCosts:      map[int]int{1: 100, 2: 250, 3: 500, 4: 1000},
			NPCVariance:    1.0,
			MaxWantedLevel: 4,
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return inst
}

// spawnGuard spawns a bribeable guard NPC in "room_a" on the given server and returns the instance.
//
// Precondition: svc must be non-nil.
// Postcondition: Returns a spawned bribeable guard instance.
func spawnGuard(t *testing.T, svc *GameServiceServer) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:      "test_guard",
		Name:    "Officer Vance",
		NPCType: "guard",
		Level:   2,
		MaxHP:   15,
		AC:      14,
		Guard: &npc.GuardConfig{
			WantedThreshold:     1,
			Bribeable:           true,
			MaxBribeWantedLevel: 3,
			BaseCosts:           map[int]int{1: 80, 2: 200, 3: 400, 4: 800},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return inst
}

// TestHandleBribe_WantedLevelZero verifies that bribing when wanted level is 0 returns an error message.
//
// Precondition: player WantedLevel["zone_a"] == 0.
// Postcondition: event message contains "not wanted".
func TestHandleBribe_WantedLevelZero(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.WantedLevel["test"] = 0

	spawnFixer(t, svc)

	evt, err := svc.handleBribe(uid, &gamev1.BribeRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "not wanted")
}

// TestHandleBribe_NoBribeableNPC verifies that bribing with no bribeable NPC present returns appropriate message.
//
// Precondition: no NPC in room; player WantedLevel["zone_a"] == 2.
// Postcondition: event message contains "no one".
func TestHandleBribe_NoBribeableNPC(t *testing.T) {
	svc, uid := newBribeTestServer(t)

	evt, err := svc.handleBribe(uid, &gamev1.BribeRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no one")
}

// TestHandleBribe_ShowsCostAndPrompt verifies that a valid bribe shows cost, sets pending state,
// and prompts for confirmation.
//
// Precondition: fixer in room; player WantedLevel["zone_a"] == 2; currency == 1000.
// Postcondition: sess.PendingBribeNPCName == "Remy"; sess.PendingBribeAmount > 0;
// event message contains cost and "confirm".
func TestHandleBribe_ShowsCostAndPrompt(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	spawnFixer(t, svc)

	evt, err := svc.handleBribe(uid, &gamev1.BribeRequest{NpcName: "Remy"})
	require.NoError(t, err)

	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, "Remy", sess.PendingBribeNPCName)
	assert.Greater(t, sess.PendingBribeAmount, 0)
	assert.Contains(t, evt.GetMessage().Content, "confirm")
}

// TestHandleBribeConfirm_NoPendingBribe verifies that confirming a bribe with no pending state
// returns an appropriate error message.
//
// Precondition: sess.PendingBribeNPCName == "".
// Postcondition: event message contains "no pending bribe".
func TestHandleBribeConfirm_NoPendingBribe(t *testing.T) {
	svc, uid := newBribeTestServer(t)

	evt, err := svc.handleBribeConfirm(uid, &gamev1.BribeConfirmRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no pending bribe")
}

// TestHandleBribeConfirm_InsufficientCredits verifies that confirming a bribe with insufficient
// credits returns a failure message without deducting credits or changing wanted level.
//
// Precondition: sess.PendingBribeAmount == 500; sess.Currency == 100.
// Postcondition: sess.Currency == 100; sess.WantedLevel["test"] == 2; event contains credit amount.
func TestHandleBribeConfirm_InsufficientCredits(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.PendingBribeNPCName = "Remy"
	sess.PendingBribeAmount = 500
	sess.Currency = 100

	evt, err := svc.handleBribeConfirm(uid, &gamev1.BribeConfirmRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "credits")
	assert.Equal(t, 100, sess.Currency, "credits must not be deducted")
	assert.Equal(t, 2, sess.WantedLevel["test"], "wanted level must not change")
}

// TestHandleBribe_WantedLevelExceedsCap verifies that bribing returns a rejection message when the
// player's WantedLevel exceeds the target NPC's cap (MaxWantedLevel for fixer,
// MaxBribeWantedLevel for guard).
//
// Precondition: fixer with MaxWantedLevel=2 in room; player WantedLevel["test"] == 4.
// Postcondition: event message contains "no one" (NPC is excluded from candidates).
func TestHandleBribe_WantedLevelExceedsCap(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	// Elevate wanted level above the fixer's cap of 2.
	sess.WantedLevel["test"] = 4

	// Spawn a fixer whose MaxWantedLevel is 2; player at level 4 exceeds it.
	tmpl := &npc.Template{
		ID:      "test_fixer_lowcap",
		Name:    "LowCap Remy",
		NPCType: "fixer",
		Level:   3,
		MaxHP:   20,
		AC:      12,
		Fixer: &npc.FixerConfig{
			BaseCosts:      map[int]int{1: 100, 2: 250},
			NPCVariance:    1.0,
			MaxWantedLevel: 2,
		},
	}
	_, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	evt, err := svc.handleBribe(uid, &gamev1.BribeRequest{NpcName: "LowCap Remy"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no one")
}

// TestHandleBribe_Disambiguation verifies that bribing without an NPC name returns a
// disambiguation message listing all bribeable NPC names when multiple are present.
//
// Precondition: fixer "Remy" and bribeable guard "Officer Vance" both in room;
// player WantedLevel["test"] == 2.
// Postcondition: event message contains "Multiple" and lists both NPC names.
func TestHandleBribe_Disambiguation(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	spawnFixer(t, svc)
	spawnGuard(t, svc)

	evt, err := svc.handleBribe(uid, &gamev1.BribeRequest{})
	require.NoError(t, err)

	content := evt.GetMessage().Content
	assert.Contains(t, content, "Multiple")
	assert.Contains(t, content, "Remy")
	assert.Contains(t, content, "Officer Vance")
}

// TestHandleBribeConfirm_Success verifies that a successful bribe confirmation deducts credits,
// decrements wanted level, and clears pending state.
//
// Precondition: fixer in room; sess.PendingBribeNPCName == "Remy"; sess.PendingBribeAmount == 250;
// sess.Currency == 1000; sess.WantedLevel["test"] == 2.
// Postcondition: sess.Currency == 750; sess.WantedLevel["test"] == 1;
// sess.PendingBribeNPCName == ""; sess.PendingBribeAmount == 0.
func TestHandleBribeConfirm_Success(t *testing.T) {
	svc, uid := newBribeTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.PendingBribeNPCName = "Remy"
	sess.PendingBribeAmount = 250
	sess.Currency = 1000

	evt, err := svc.handleBribeConfirm(uid, &gamev1.BribeConfirmRequest{})
	require.NoError(t, err)
	assert.Equal(t, 750, sess.Currency, "credits must be deducted")
	assert.Equal(t, 1, sess.WantedLevel["test"], "wanted level must decrement by 1")
	assert.Equal(t, "", sess.PendingBribeNPCName, "pending NPC name must be cleared")
	assert.Equal(t, 0, sess.PendingBribeAmount, "pending amount must be cleared")
	assert.NotEmpty(t, evt.GetMessage().Content)
}
