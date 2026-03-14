package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// addHeroPointSession adds a player session with given hero point state and returns it.
//
// Precondition: sessMgr must be non-nil; uid must not already exist.
// Postcondition: session with requested fields is stored; returned session is non-nil.
func addHeroPointSession(t *testing.T, sessMgr *session.Manager, uid string, heroPoints int, dead bool, lastRoll int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_hp_test",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.HeroPoints = heroPoints
	sess.Dead = dead
	sess.LastCheckRoll = lastRoll
	return sess
}

// TestHandleHeroPoint_NoSession verifies that handleHeroPoint returns an error
// when the player session does not exist.
//
// Precondition: uid "unknown_hp_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleHeroPoint_NoSession(t *testing.T) {
	svc, _ := newGrappleSvc(t, nil, nil, nil)
	event, err := svc.handleHeroPoint("unknown_hp_uid", &gamev1.HeroPointRequest{Subcommand: "reroll"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleHeroPointReroll_NoPoints verifies that reroll with 0 hero points returns an error event.
//
// Precondition: sess.HeroPoints == 0.
// Postcondition: error event containing "no hero points".
func TestHandleHeroPointReroll_NoPoints(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	addHeroPointSession(t, sessMgr, "u_hp_rr_np", 0, false, 10)

	event, err := svc.handleHeroPoint("u_hp_rr_np", &gamev1.HeroPointRequest{Subcommand: "reroll"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event")
	assert.Contains(t, errEvt.Message, "no hero points")
}

// TestHandleHeroPointReroll_NoRecentCheck verifies that reroll with LastCheckRoll==0 returns an error event.
//
// Precondition: sess.HeroPoints >= 1; sess.LastCheckRoll == 0.
// Postcondition: error event containing "no recent check".
func TestHandleHeroPointReroll_NoRecentCheck(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	addHeroPointSession(t, sessMgr, "u_hp_rr_nr", 1, false, 0)

	event, err := svc.handleHeroPoint("u_hp_rr_nr", &gamev1.HeroPointRequest{Subcommand: "reroll"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event")
	assert.Contains(t, errEvt.Message, "no recent check")
}

// TestHandleHeroPointReroll_NewRollWins verifies that when new roll > old roll,
// the message reports keeping the new roll.
//
// Precondition: sess.HeroPoints==1; LastCheckRoll==5; dice returns 19 (roll=20).
// Postcondition: message contains "keeping 20"; HeroPoints decremented to 0.
func TestHandleHeroPointReroll_NewRollWins(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newGrappleSvc(t, roller, nil, nil)
	sess := addHeroPointSession(t, sessMgr, "u_hp_rr_nw", 1, false, 5)

	event, err := svc.handleHeroPoint("u_hp_rr_nw", &gamev1.HeroPointRequest{Subcommand: "reroll"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected message event")
	assert.Contains(t, msgEvt.Content, "keeping 20")
	assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointReroll_OldRollWins verifies that when old roll > new roll,
// the message reports keeping the old roll.
//
// Precondition: sess.HeroPoints==1; LastCheckRoll==18; dice returns 0 (roll=1).
// Postcondition: message contains "keeping 18"; HeroPoints decremented to 0.
func TestHandleHeroPointReroll_OldRollWins(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newGrappleSvc(t, roller, nil, nil)
	sess := addHeroPointSession(t, sessMgr, "u_hp_rr_ow", 1, false, 18)

	event, err := svc.handleHeroPoint("u_hp_rr_ow", &gamev1.HeroPointRequest{Subcommand: "reroll"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected message event")
	assert.Contains(t, msgEvt.Content, "keeping 18")
	assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointStabilize_NoPoints verifies that stabilize with 0 hero points returns an error event.
//
// Precondition: sess.HeroPoints == 0; sess.Dead == true.
// Postcondition: error event containing "no hero points".
func TestHandleHeroPointStabilize_NoPoints(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	addHeroPointSession(t, sessMgr, "u_hp_st_np", 0, true, 0)

	event, err := svc.handleHeroPoint("u_hp_st_np", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event")
	assert.Contains(t, errEvt.Message, "no hero points")
}

// TestHandleHeroPointStabilize_NotDying verifies that stabilize when not Dead returns an error event.
//
// Precondition: sess.HeroPoints >= 1; sess.Dead == false.
// Postcondition: error event containing "not dying".
func TestHandleHeroPointStabilize_NotDying(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	addHeroPointSession(t, sessMgr, "u_hp_st_nd", 1, false, 0)

	event, err := svc.handleHeroPoint("u_hp_st_nd", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event")
	assert.Contains(t, errEvt.Message, "not dying")
}

// TestHandleHeroPointStabilize_Success verifies that stabilize when Dead==true
// clears Dead, sets HP to 0, and decrements HeroPoints.
//
// Precondition: sess.HeroPoints==1; sess.Dead==true.
// Postcondition: sess.Dead==false; sess.CurrentHP==0; sess.HeroPoints==0; message event.
func TestHandleHeroPointStabilize_Success(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	sess := addHeroPointSession(t, sessMgr, "u_hp_st_ok", 1, true, 0)

	event, err := svc.handleHeroPoint("u_hp_st_ok", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected message event")
	assert.Contains(t, msgEvt.Content, "stabilize")
	assert.False(t, sess.Dead)
	assert.Equal(t, 0, sess.CurrentHP)
	assert.Equal(t, 0, sess.HeroPoints)
}

// TestProperty_HeroPointStabilize_Invariants verifies stabilize invariants:
// Dead becomes false, CurrentHP becomes 0, HeroPoints decrements by 1.
//
// Precondition: sess.Dead == true; sess.HeroPoints >= 1; sess.CurrentHP < 0.
// Postcondition: sess.Dead == false; sess.CurrentHP == 0; sess.HeroPoints == heroPoints-1.
func TestProperty_HeroPointStabilize_Invariants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		heroPoints := rapid.IntRange(1, 10).Draw(rt, "heroPoints")
		startHP := rapid.IntRange(-20, -1).Draw(rt, "startHP")

		svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
		uid := fmt.Sprintf("u_stab_prop_%d_%d", heroPoints, -startHP)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: "P", CharName: "P",
			RoomID: "room_stab_prop", Role: "player",
		})
		require.NoError(rt, err)
		sess.HeroPoints = heroPoints
		sess.Dead = true
		sess.CurrentHP = startHP

		event, err := svc.handleHeroPoint(uid, &gamev1.HeroPointRequest{Subcommand: "stabilize"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt, "stabilize must return message event")

		assert.False(rt, sess.Dead, "Dead must be false after stabilize")
		assert.Equal(rt, 0, sess.CurrentHP, "CurrentHP must be 0 after stabilize")
		assert.Equal(rt, heroPoints-1, sess.HeroPoints, "HeroPoints must decrement by 1")
	})
}

// TestProperty_HeroPointReroll_Invariants verifies: after reroll, HeroPoints decrements by 1
// and the winner equals max(old, new).
//
// Precondition: heroPoints >= 1; lastRoll in [1,20]; diceVal in [0,19].
// Postcondition: HeroPoints == heroPoints-1; message contains "keeping {max(lastRoll,newRoll)}".
func TestProperty_HeroPointReroll_Invariants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		heroPoints := rapid.IntRange(1, 5).Draw(rt, "heroPoints")
		lastRoll := rapid.IntRange(1, 20).Draw(rt, "lastRoll")
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		newRoll := diceVal + 1

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: diceVal}
		roller := dice.NewLoggedRoller(src, logger)

		svc, sessMgr := newGrappleSvc(t, roller, nil, nil)
		uid := fmt.Sprintf("u_prop_hp_rr_%d_%d_%d", heroPoints, lastRoll, diceVal)
		sess := addHeroPointSession(t, sessMgr, uid, heroPoints, false, lastRoll)

		event, err := svc.handleHeroPoint(uid, &gamev1.HeroPointRequest{Subcommand: "reroll"})
		require.NoError(rt, err)
		require.NotNil(rt, event)

		// HeroPoints must decrement by exactly 1.
		assert.Equal(rt, heroPoints-1, sess.HeroPoints)

		// Winner is max(lastRoll, newRoll).
		winner := lastRoll
		if newRoll > lastRoll {
			winner = newRoll
		}
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("keeping %d", winner))
	})
}
