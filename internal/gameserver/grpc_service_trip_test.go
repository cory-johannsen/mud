package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newTripSvc builds a minimal GameServiceServer for handleTrip tests.
// npcMgr may be nil; combatHandler may be nil.
func newTripSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newTripSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newTripSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleTrip_NoSession verifies that handleTrip returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_trp_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleTrip_NoSession(t *testing.T) {
	svc, _ := newTripSvc(t, nil, nil, nil)
	event, err := svc.handleTrip("unknown_trp_uid", &gamev1.TripRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleTrip_NotInCombat verifies that handleTrip returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleTrip_NotInCombat(t *testing.T) {
	svc, sessMgr := newTripSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_trp_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleTrip("u_trp_nc", &gamev1.TripRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleTrip_EmptyTarget verifies that handleTrip returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: trip".
func TestHandleTrip_EmptyTarget(t *testing.T) {
	svc, sessMgr := newTripSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_trp_et",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleTrip("u_trp_et", &gamev1.TripRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: trip")
}

// TestHandleTrip_TargetNotFound verifies that handleTrip returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleTrip_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTripSvcWithCombat(t, roller)

	const roomID = "room_trp_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-trp-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_trp_tnf", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_trp_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_trp_tnf")

	event, err := svc.handleTrip("u_trp_tnf", &gamev1.TripRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_trp_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")
}

// TestHandleTrip_RollBelowDC_Failure verifies that handleTrip returns a failure message
// when the muscle roll total is below the target's Hustle DC.
//
// Precondition: player in combat; NPC Level=5 → DC=15 (Hustle DC: Level=5, Quickness=10, rank=untrained → 10+5+0+0=15); dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure".
func TestHandleTrip_RollBelowDC_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTripSvcWithCombat(t, roller)

	const roomID = "room_trp_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-trp-rb", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRB, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_trp_rb", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRB.Status = statusInCombat

	_, err = combatHandler.Attack("u_trp_rb", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleTrip("u_trp_rb", &gamev1.TripRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed trip")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleTrip_RollAboveDC_Success verifies that handleTrip returns a success message
// and applies the prone condition when the muscle roll meets or exceeds the Hustle DC.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 19 (roll=20, bonus=0, total=20 >= 11).
// Postcondition: message event containing "success"; prone condition active on target combatant.
func TestHandleTrip_RollAboveDC_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newTripSvcWithCombat(t, roller)

	const roomID = "room_trp_ra"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-trp-ra", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRA, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_trp_ra", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRA.Status = statusInCombat

	_, err = combatHandler.Attack("u_trp_ra", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleTrip("u_trp_ra", &gamev1.TripRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful trip")
	assert.Contains(t, msgEvt.Content, "success")

	// Verify that ApplyCombatCondition was called: the NPC must have the prone condition.
	condSet, ok := combatHandler.GetCombatConditionSet("u_trp_ra", inst.ID)
	require.True(t, ok, "expected to find condition set for NPC after trip")
	assert.True(t, condSet.Has("prone"), "NPC must have prone condition after successful trip")
}

// TestProperty_HandleTrip_HustleDC_Formula verifies that the Hustle DC
// used by handleTrip equals 10 + level + abilityMod(quickness) + rankBonus.
//
// Precondition: rapid generates level (1-20), quickness (1-20), rank string.
// Postcondition: message content contains the expected DC value.
func TestProperty_HandleTrip_HustleDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		quickness := rapid.IntRange(1, 20).Draw(rt, "quickness")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := combat.AbilityMod(quickness)
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: fmt.Sprintf("trip-prop-%d-%d", level, quickness), Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Perception: 5,
			Abilities:  npc.Abilities{Brutality: 10, Quickness: quickness, Savvy: 10},
			HustleRank: rank,
		}

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		svc, sessMgr, npcMgr, combatHandler := newTripSvcWithCombat(t, roller)

		roomID := fmt.Sprintf("room_trip_prop_%d_%d", level, quickness)
		uid := fmt.Sprintf("u_trip_prop_%d_%d", level, quickness)
		_, err := npcMgr.Spawn(tmpl, roomID)
		require.NoError(rt, err)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: "F", CharName: "F",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat
		_, err = combatHandler.Attack(uid, "Target")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleTrip(uid, &gamev1.TripRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC),
			"message must include computed Hustle DC")
	})
}
