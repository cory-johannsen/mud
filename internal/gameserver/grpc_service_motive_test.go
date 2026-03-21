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
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleMotive_NoSession verifies that handleMotive returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_mot_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleMotive_NoSession(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)
	svc, _, _, _ := newMotiveSvcWithCombat(t, roller)

	event, err := svc.handleMotive("unknown_mot_uid", &gamev1.MotiveRequest{Target: "Ganger"})
	require.Error(t, err)
	assert.Nil(t, event)
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
		ID: "ganger-mot-tnf", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
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

// TestHandleMotive_OutOfCombat_NoTarget verifies that handleMotive returns an error event
// when no target is specified out of combat.
//
// Precondition: player out of combat; req.Target == "".
// Postcondition: error event containing "Usage".
func TestHandleMotive_OutOfCombat_NoTarget(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 17}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, _, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_mot_oc_nt",
		Username: "Scout",
		CharName: "Scout",
		RoomID:   "room_mot_oc_nt",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleMotive("u_mot_oc_nt", &gamev1.MotiveRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage")
}

// TestHandleMotive_OutOfCombat_TargetNotFound verifies that handleMotive returns an error event
// when the named NPC is not in the player's room out of combat.
//
// Precondition: player out of combat; no NPC named "ghost" in room.
// Postcondition: error event containing "ghost".
func TestHandleMotive_OutOfCombat_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 17}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, _, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_mot_oc_tnf",
		Username: "Scout",
		CharName: "Scout",
		RoomID:   "room_mot_oc_tnf",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleMotive("u_mot_oc_tnf", &gamev1.MotiveRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "ghost")
}

// TestHandleMotive_OutOfCombat_Success verifies that handleMotive reveals NPC disposition on success.
//
// Precondition: player out of combat; NPC Hustle=0 → DC=10; fixedDiceSource{val:17} → roll=18 >= 10;
//
//	NPC Disposition="neutral".
//
// Postcondition: message event containing "neutral" and "success".
func TestHandleMotive_OutOfCombat_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=17: Intn(20)=17 → d20=18. DC=10+0=10; total=18 >= 10 → success.
	src := &fixedDiceSource{val: 17}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_oc_succ"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-oc-succ", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Hustle: 0, Disposition: "neutral",
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_oc_succ", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleMotive("u_mot_oc_succ", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful out-of-combat motive check")
	assert.Contains(t, msgEvt.Content, "success")
	assert.Contains(t, msgEvt.Content, "neutral")
}

// TestHandleMotive_OutOfCombat_CritFailure_FlipsDisposition verifies that a crit failure
// out of combat flips a neutral NPC disposition to hostile.
//
// Precondition: player out of combat; NPC Hustle=0 → DC=10; fixedDiceSource{val:0} → roll=1 < DC-10=0 → CritFailure;
//
//	NPC Disposition="neutral".
//
// Postcondition: inst.Disposition=="hostile"; message event containing "critical failure".
func TestHandleMotive_OutOfCombat_CritFailure_FlipsDisposition(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=0: Intn(20)=0 → d20=1. DC=10; OutcomeFor(1,10): 1 < 10-10=0? No. 1 < 10? Yes → Failure not CritFailure.
	// To get CritFailure we need total < dc-10 = 0, which is impossible with d20 (min=1).
	// With PF2E: CritFailure = total < dc-10. DC=10, so need total < 0 — impossible for d20.
	// Use Hustle=5 → DC=15; CritFailure when total < 5. roll=1+bonus=0 → total=1 < 5 → CritFailure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_oc_cf"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-oc-cf", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Hustle: 5, Disposition: "neutral",
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_oc_cf", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleMotive("u_mot_oc_cf", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit failure out-of-combat motive check")
	assert.Contains(t, msgEvt.Content, "critical failure")
	assert.Equal(t, "hostile", inst.Disposition, "disposition must flip to hostile on crit failure")
}

// TestHandleMotive_InCombat_Success_RevealsNextAction verifies that a success in combat
// reveals the NPC's next intended action.
//
// Precondition: player in combat; NPC Hustle=0 → DC=10; fixedDiceSource{val:17} → roll=18 >= 10 → success;
//
//	NPC at full HP (100%) → motiveNextAction returns "focused on the fight".
//
// Postcondition: message event containing "success" and "focused on the fight".
func TestHandleMotive_InCombat_Success_RevealsNextAction(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=17: Intn(20)=17 → d20=18. DC=10; total=18 >= 10 → success.
	src := &fixedDiceSource{val: 17}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_ic_succ"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-ic-succ", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Hustle: 0,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_ic_succ", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_ic_succ", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_ic_succ", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful in-combat motive check")
	assert.Contains(t, msgEvt.Content, "success")
	assert.Contains(t, msgEvt.Content, "focused on the fight")
}

// TestHandleMotive_InCombat_CritFailure_SetsMotiveBonus verifies that a crit failure in combat
// sets inst.MotiveBonus to 2.
//
// Precondition: player in combat; NPC Hustle=5 → DC=15; fixedDiceSource{val:0} → roll=1 < DC-10=5 → CritFailure.
// Postcondition: inst.MotiveBonus==2; message event containing "critical failure".
func TestHandleMotive_InCombat_CritFailure_SetsMotiveBonus(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=0: Intn(20)=0 → d20=1. DC=15; OutcomeFor(1,15): 1 < 15-10=5 → CritFailure.
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_ic_cf"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-ic-cf", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Hustle: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_ic_cf", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_ic_cf", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_ic_cf", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit failure in-combat motive check")
	assert.Contains(t, msgEvt.Content, "critical failure")
	assert.Equal(t, 2, inst.MotiveBonus, "MotiveBonus must be set to 2 on in-combat crit failure")
}

// TestHandleMotive_InCombat_Failure verifies that handleMotive returns a failure message
// when the awareness roll falls below the Hustle DC but not by 10+.
//
// Precondition: player in combat; NPC Hustle=0 → DC=10; fixedDiceSource{val:4} → roll=5 → Failure (5 < 10, 5 >= 0).
// Postcondition: message event containing "failure".
func TestHandleMotive_InCombat_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=4: Intn(20)=4 → d20=5. DC=10; OutcomeFor(5,10): 5 < 10, 5 >= 0 → Failure.
	src := &fixedDiceSource{val: 4}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_ic_fail"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-ic-fail", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_ic_fail", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_ic_fail", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_ic_fail", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed motive check")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleMotive_InCombat_CritSuccess_RevealsAbilities verifies that a crit success in combat
// reveals the NPC's special abilities, resistances, and weaknesses.
//
// Precondition: player in combat; NPC Hustle=0 → DC=10; fixedDiceSource{val:19} → roll=20 >= 20 (DC+10) → CritSuccess;
//
//	NPC has SpecialAbilities=["Rage"], Resistances={"fire":5}, Weaknesses={"cold":3}.
//
// Postcondition: message event containing "critical success", "Rage", "fire", "cold".
func TestHandleMotive_InCombat_CritSuccess_RevealsAbilities(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=19: Intn(20)=19 → d20=20. DC=10; OutcomeFor(20,10): 20 >= 10+10=20 → CritSuccess.
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

	const roomID = "room_mot_ic_cs"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-mot-ic-cs", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Hustle:           0,
		SpecialAbilities: []string{"Rage"},
	}, roomID)
	require.NoError(t, err)
	inst.Resistances = map[string]int{"fire": 5}
	inst.Weaknesses = map[string]int{"cold": 3}

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_ic_cs", Username: "Scout", CharName: "Scout",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_mot_ic_cs", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleMotive("u_mot_ic_cs", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit success in-combat motive check")
	assert.Contains(t, msgEvt.Content, "critical success")
	assert.Contains(t, msgEvt.Content, "Rage")
	assert.Contains(t, msgEvt.Content, "fire")
	assert.Contains(t, msgEvt.Content, "cold")
}

// TestProperty_Motive_InCombat_SuccessAlwaysRevealsNextAction is a property-based test verifying
// that when the roll always beats the DC (roll=18 with Hustle DC 0–7), the message
// always contains the next action phrase.
//
// Precondition: hustle in [0,7]; fixedDiceSource{val:17} → roll=18; DC=10+hustle <= 17, so 18 > DC → success.
// Postcondition: message contains one of the next-action strings.
func TestProperty_Motive_InCombat_SuccessAlwaysRevealsNextAction(t *testing.T) {
	nextActions := []string{"focused on the fight", "ready to flee", "holding something back"}

	rapid.Check(t, func(rt *rapid.T) {
		hustle := rapid.IntRange(0, 7).Draw(rt, "hustle")
		suffix := rapid.StringMatching(`[a-z]{4}`).Draw(rt, "suffix")

		logger := zaptest.NewLogger(t)
		// val=17: d20=18. Max DC = 10+7=17; total=18 > 17 → always success (not crit: 18 < 17+10=27).
		src := &fixedDiceSource{val: 17}
		roller := dice.NewLoggedRoller(src, logger)

		svc, sessMgr, npcMgr, combatHandler := newMotiveSvcWithCombat(t, roller)

		roomID := "room_prop_mot_ic_" + suffix

		_, err := npcMgr.Spawn(&npc.Template{
			ID:         "npc-prop-mot-ic-" + suffix,
			Name:       "Ganger",
			Level:      1,
			MaxHP:      20,
			AC:         13,
			Awareness: 2,
			Hustle:     hustle,
		}, roomID)
		require.NoError(rt, err)

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       "u_prop_mot_ic_" + suffix,
			Username:  "Scout",
			CharName:  "Scout",
			RoomID:    roomID,
			CurrentHP: 10,
			MaxHP:     20,
			Role:      "player",
		})
		require.NoError(rt, err)
		sess.Status = statusInCombat

		_, err = combatHandler.Attack("u_prop_mot_ic_"+suffix, "Ganger")
		require.NoError(rt, err)
		combatHandler.cancelTimer(roomID)

		event, err := svc.handleMotive("u_prop_mot_ic_"+suffix, &gamev1.MotiveRequest{Target: "Ganger"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt, "expected a message event on successful motive check")

		found := false
		for _, na := range nextActions {
			if strings.Contains(msgEvt.Content, na) {
				found = true
				break
			}
		}
		assert.True(rt, found, "expected message to contain one of %v, got: %q", nextActions, msgEvt.Content)
	})
}
