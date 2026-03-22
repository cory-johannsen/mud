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

// newGrappleSvc builds a minimal GameServiceServer for handleGrapple tests.
// npcMgr may be nil; combatHandler may be nil.
func newGrappleSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := newTestGameServiceServer(
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
		nil, nil,
	)
	return svc, sessMgr
}

// newGrappleSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newGrappleSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
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
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleGrapple_NoSession verifies that handleGrapple returns an error when the
// player session does not exist.
//
// Precondition: uid "unknown_grp_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleGrapple_NoSession(t *testing.T) {
	svc, _ := newGrappleSvc(t, nil, nil, nil)
	event, err := svc.handleGrapple("unknown_grp_uid", &gamev1.GrappleRequest{Target: "bandit"})
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestHandleGrapple_NotInCombat verifies that handleGrapple returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleGrapple_NotInCombat(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_grp_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleGrapple("u_grp_nc", &gamev1.GrappleRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleGrapple_EmptyTarget verifies that handleGrapple returns an error event
// when no target is specified.
//
// Precondition: player in combat; req.Target == "".
// Postcondition: error event containing "Usage: grapple".
func TestHandleGrapple_EmptyTarget(t *testing.T) {
	svc, sessMgr := newGrappleSvc(t, nil, nil, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_grp_et",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	event, err := svc.handleGrapple("u_grp_et", &gamev1.GrappleRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "Usage: grapple")
}

// TestHandleGrapple_TargetNotFound verifies that handleGrapple returns an error event
// when the named target NPC is not in the player's room, and that AP is NOT spent.
//
// Precondition: player in combat; no NPC named "ghost" in room.
// Postcondition: error event containing "not found"; AP is not decremented.
func TestHandleGrapple_TargetNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newGrappleSvcWithCombat(t, roller)

	const roomID = "room_grp_tnf"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-grp-tnf", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_grp_tnf", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_grp_tnf", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	apBefore := combatHandler.RemainingAP("u_grp_tnf")

	event, err := svc.handleGrapple("u_grp_tnf", &gamev1.GrappleRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected error event for non-existent target")
	assert.Contains(t, errEvt.Message, "not found")

	apAfter := combatHandler.RemainingAP("u_grp_tnf")
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when target is not found")
}

// TestHandleGrapple_RollBelowDC_Failure verifies that handleGrapple returns a failure message
// when the muscle roll total is below the target's Toughness DC.
//
// Precondition: player in combat; NPC Level=5 → DC=15 (Toughness DC: Level=5, Brutality=10, rank=untrained → 10+5+0+0=15); dice returns 0 (roll=1, bonus=0, total=1 < 15).
// Postcondition: message event containing "failure".
func TestHandleGrapple_RollBelowDC_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newGrappleSvcWithCombat(t, roller)

	const roomID = "room_grp_rb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-grp-rb", Name: "Bandit", Level: 5, MaxHP: 20, AC: 13, Awareness: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRB, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_grp_rb", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRB.Status = statusInCombat

	_, err = combatHandler.Attack("u_grp_rb", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleGrapple("u_grp_rb", &gamev1.GrappleRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on failed grapple")
	assert.Contains(t, msgEvt.Content, "failure")
}

// TestHandleGrapple_RollAboveDC_Success verifies that handleGrapple returns a success message
// and applies the grabbed condition when the muscle roll meets or exceeds the Toughness DC.
//
// Precondition: player in combat; NPC Level=1 → DC=11; dice returns 19 (roll=20, bonus=0, total=20 >= 11).
// Postcondition: message event containing "success"; grabbed condition active on target combatant.
func TestHandleGrapple_RollAboveDC_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr, npcMgr, combatHandler := newGrappleSvcWithCombat(t, roller)

	const roomID = "room_grp_ra"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-grp-ra", Name: "Ganger", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
		Abilities: npc.Abilities{Brutality: 10, Quickness: 10, Savvy: 10},
	}, roomID)
	require.NoError(t, err)

	sessRA, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_grp_ra", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sessRA.Status = statusInCombat

	_, err = combatHandler.Attack("u_grp_ra", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	event, err := svc.handleGrapple("u_grp_ra", &gamev1.GrappleRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on successful grapple")
	assert.Contains(t, msgEvt.Content, "success")

	// Verify that ApplyCombatCondition was called: the NPC must have the grabbed condition.
	condSet, ok := combatHandler.GetCombatConditionSet("u_grp_ra", inst.ID)
	require.True(t, ok, "expected to find condition set for NPC after grapple")
	assert.True(t, condSet.Has("grabbed"), "NPC must have grabbed condition after successful grapple")
}

// TestProperty_HandleGrapple_ToughnessDC_Formula verifies that the Toughness DC
// used by handleGrapple equals 10 + level + abilityMod(brutality) + rankBonus.
//
// Precondition: rapid generates level (1-20), brutality (1-20), rank string.
// Postcondition: message content contains the expected DC value.
func TestProperty_HandleGrapple_ToughnessDC_Formula(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		brutality := rapid.IntRange(1, 20).Draw(rt, "brutality")
		rank := rapid.SampledFrom([]string{"", "trained", "expert", "master", "legendary"}).Draw(rt, "rank")

		expectedMod := combat.AbilityMod(brutality)
		expectedRankBonus := skillRankBonus(rank)
		expectedDC := 10 + level + expectedMod + expectedRankBonus

		tmpl := &npc.Template{
			ID: fmt.Sprintf("grp-prop-%d-%d", level, brutality), Name: "Target", Level: level,
			MaxHP: 20, AC: 13, Awareness: 5,
			Abilities:     npc.Abilities{Brutality: brutality, Quickness: 10, Savvy: 10},
			ToughnessRank: rank,
		}

		logger := zaptest.NewLogger(t)
		src := &fixedDiceSource{val: 99}
		roller := dice.NewLoggedRoller(src, logger)
		svc, sessMgr, npcMgr, combatHandler := newGrappleSvcWithCombat(t, roller)

		roomID := fmt.Sprintf("room_grp_prop_%d_%d", level, brutality)
		uid := fmt.Sprintf("u_grp_prop_%d_%d", level, brutality)
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

		event, err := svc.handleGrapple(uid, &gamev1.GrappleRequest{Target: "Target"})
		require.NoError(rt, err)
		require.NotNil(rt, event)
		msgEvt := event.GetMessage()
		require.NotNil(rt, msgEvt)
		assert.Contains(rt, msgEvt.Content, fmt.Sprintf("DC %d", expectedDC),
			"message must include computed Toughness DC")
	})
}
