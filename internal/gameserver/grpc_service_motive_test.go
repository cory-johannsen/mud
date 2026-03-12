package gameserver

import (
	"strings"
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

// newMotiveSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newMotiveSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleMotive_NotInCombat verifies that handleMotive returns a stub message
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: message event containing "no one".
func TestHandleMotive_NotInCombat(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, _, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_mot_nc",
		Username: "Scout",
		CharName: "Scout",
		RoomID:   "room_mot_nc",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleMotive("u_mot_nc", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event out of combat")
	assert.Contains(t, msgEvt.Content, "no one")
}

// TestHandleMotive_NoTarget verifies that handleMotive returns an error event
// when no target is specified while in combat.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage".
func TestHandleMotive_NoTarget(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, _, _ := newMotiveSvcWithCombat(t, roller)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_mot_nt",
		Username: "Scout",
		CharName: "Scout",
		RoomID:   "room_mot_nt",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleMotive("u_mot_nt", &gamev1.MotiveRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage")
}

// TestHandleMotive_TargetNotFound verifies that handleMotive returns an error event
// when the named target NPC is not in the player's room.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "ghost".
func TestHandleMotive_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-tnf", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_tnf", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_tnf", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_tnf", &gamev1.MotiveRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "ghost")
}

// TestHandleMotive_RollFailure verifies that handleMotive returns a failure message
// when the awareness roll falls below the Deception DC.
//
// Precondition: player in combat; NPC Deception=0 → DC=10; fixedDiceSource{val:0} → roll=1, bonus=0, total=1 < 10.
// Postcondition: message event containing "failure".
func TestHandleMotive_RollFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=0: Intn returns 0, meaning d20 = 1. DC = 10+0 = 10; total=1 < 10 → failure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_rf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-rf", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_rf", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_rf", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_rf", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed motive check")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleMotive_RollSuccess_HPTier verifies that handleMotive returns a success message
// containing the NPC's HP tier when the roll meets or exceeds the DC.
//
// Precondition: player in combat; NPC Deception=0 → DC=10; fixedDiceSource{val:19} → roll=20 >= 10; NPC at full HP.
// Postcondition: message event containing "success" and "unharmed".
func TestHandleMotive_RollSuccess_HPTier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=19: Intn returns 19, meaning d20 = 20. DC = 10+0 = 10; total=20 >= 10 → success.
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_rs"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-rs", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_rs", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_rs", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_rs", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful motive check")
	assert.Contains(t, msgEvt.Content, "success")
	assert.Contains(t, msgEvt.Content, "unharmed")
}

// TestProperty_Motive_SuccessAlwaysRevealsHPTier is a property-based test verifying
// that when the roll always beats the DC (roll=20 with any Deception 0–9), the message
// always contains one of the known HP tier strings.
//
// Precondition: deception in [0,9]; fixedDiceSource{val:19} → roll=20; DC=10+deception <= 19, so 20 always succeeds.
// Postcondition: message contains one of the HP tier strings.
func TestProperty_Motive_SuccessAlwaysRevealsHPTier(t *testing.T) {
	tiers := []string{"unharmed", "lightly wounded", "bloodied", "badly wounded"}

	rapid.Check(t, func(rt *rapid.T) {
		deception := rapid.IntRange(0, 9).Draw(rt, "deception")
		suffix := rapid.StringMatching(`[a-z]{4}`).Draw(rt, "suffix")

		logger := zaptest.NewLogger(t)
		// val=19: d20 = 20. Max DC = 10+9 = 19; total=20 >= 19 → always success.
		src := &fixedDiceSource{val: 19}
		roller := dice.NewLoggedRoller(src, logger)

		svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

		roomID := "room_prop_mot_" + suffix

		_, err := npcMgr.Spawn(&npc.Template{
			ID:        "npc-prop-mot-" + suffix,
			Name:      "Ganger",
			Level:     1,
			MaxHP:     20,
			AC:        13,
			Perception: 2,
			Deception: deception,
		}, roomID)
		require.NoError(rt, err)

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       "u_prop_mot_" + suffix,
			Username:  "Scout",
			CharName:  "Scout",
			RoomID:    roomID,
			CurrentHP: 10,
			MaxHP:     20,
			Role:      "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat

		_, err = combatHandler.Attack("u_prop_mot_"+suffix, "Ganger")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleMotive("u_prop_mot_"+suffix, &gamev1.MotiveRequest{Target: "Ganger"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt, "expected a message event on successful motive check")

		found := false
		for _, tier := range tiers {
			if strings.Contains(msgEvt.Content, tier) {
				found = true
				break
			}
		}
		assert.True(rt, found, "expected message to contain one of %v, got: %q", tiers, msgEvt.Content)
	})
}
