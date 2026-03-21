package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestHandlerReady_Registered(t *testing.T) {
	reg := command.DefaultRegistry()
	found := false
	for _, c := range reg.Commands() {
		if c.Handler == command.HandlerReady {
			found = true
			break
		}
	}
	assert.True(t, found, "HandlerReady must be registered in command registry")
}

func TestPlayerSession_ReadiedFields_DefaultEmpty(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1_user", CharName: "u1_char",
		CharacterID: 1, RoomID: "r1",
		CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{},
		Role: "player", Level: 1,
	})
	require.NoError(t, err)
	sess, ok := mgr.GetPlayer("u1")
	require.True(t, ok)
	assert.Equal(t, "", sess.ReadiedTrigger, "ReadiedTrigger must default to empty")
	assert.Equal(t, "", sess.ReadiedAction, "ReadiedAction must default to empty")
}

func TestReadyAction_ClearReadiedAction_ClearsFields(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "u1_user", CharName: "u1_char",
		CharacterID: 1, RoomID: "r1",
		CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{},
		Role: "player", Level: 1,
	})
	require.NoError(t, err)
	sess, ok := mgr.GetPlayer("u1")
	require.True(t, ok)
	sess.ReadiedTrigger = "enemy_enters"
	sess.ReadiedAction = "strike"

	clearReadiedAction(sess)

	assert.Equal(t, "", sess.ReadiedTrigger)
	assert.Equal(t, "", sess.ReadiedAction)
}

// makeReadySvc builds a minimal GameServiceServer with a real CombatHandler for ready action tests.
func makeReadySvc(t *testing.T) (*GameServiceServer, *CombatHandler) {
	t.Helper()
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		nil,
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
	return svc, combatHandler
}

// setupReadyCombat adds an NPC and player to room_a, puts the player in combat, and returns the session.
func setupReadyCombat(t *testing.T, svc *GameServiceServer, combatHandler *CombatHandler, uid string) *session.PlayerSession {
	t.Helper()
	npcMgr := svc.npcMgr
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-ready", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_a")
	require.NoError(t, err)
	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Status = statusInCombat
	_, err = combatHandler.Attack(uid, "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer("room_a")
	return sess
}

func TestHandleReady_NotInCombat(t *testing.T) {
	svc, _ := makeReadySvc(t)
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "rp1", Username: "rp1", CharName: "rp1", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	ev, err := svc.handleReady("rp1", &gamev1.ReadyRequest{Action: "strike", Trigger: "enemy_enters"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "must be in combat")
}

func TestHandleReady_InsufficientAP(t *testing.T) {
	svc, combatHandler := makeReadySvc(t)
	sess := setupReadyCombat(t, svc, combatHandler, "rp2")

	// Drain AP down to 1 by spending all but 1.
	rem := combatHandler.RemainingAP("rp2")
	require.Greater(t, rem, 0)
	if rem > 1 {
		require.NoError(t, combatHandler.SpendAP("rp2", rem-1))
	}
	require.Equal(t, 1, combatHandler.RemainingAP("rp2"))
	_ = sess

	ev, err := svc.handleReady("rp2", &gamev1.ReadyRequest{Action: "strike", Trigger: "enemy_enters"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "need at least 2 AP")
}

func TestHandleReady_InvalidAction(t *testing.T) {
	svc, combatHandler := makeReadySvc(t)
	setupReadyCombat(t, svc, combatHandler, "rp3")
	require.GreaterOrEqual(t, combatHandler.RemainingAP("rp3"), 2)

	ev, err := svc.handleReady("rp3", &gamev1.ReadyRequest{Action: "fireball", Trigger: "enemy_enters"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "Unknown action")
}

func TestHandleReady_InvalidTrigger(t *testing.T) {
	svc, combatHandler := makeReadySvc(t)
	setupReadyCombat(t, svc, combatHandler, "rp4")
	require.GreaterOrEqual(t, combatHandler.RemainingAP("rp4"), 2)

	ev, err := svc.handleReady("rp4", &gamev1.ReadyRequest{Action: "strike", Trigger: "foo"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "Unknown trigger")
}

func TestHandleReady_AlreadyReadied(t *testing.T) {
	svc, combatHandler := makeReadySvc(t)
	sess := setupReadyCombat(t, svc, combatHandler, "rp5")
	require.GreaterOrEqual(t, combatHandler.RemainingAP("rp5"), 2)
	sess.ReadiedTrigger = "enemy_enters"

	ev, err := svc.handleReady("rp5", &gamev1.ReadyRequest{Action: "strike", Trigger: "enemy_enters"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "already have a readied action")
}

func TestHandleReady_Success(t *testing.T) {
	svc, combatHandler := makeReadySvc(t)
	sess := setupReadyCombat(t, svc, combatHandler, "rp6")
	require.GreaterOrEqual(t, combatHandler.RemainingAP("rp6"), 2)

	apBefore := combatHandler.RemainingAP("rp6")
	ev, err := svc.handleReady("rp6", &gamev1.ReadyRequest{Action: "strike", Trigger: "enemy_enters"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "You ready a Strike for when an enemy enters the room.")
	assert.Equal(t, "enemy_enters", sess.ReadiedTrigger)
	assert.Equal(t, "strike", sess.ReadiedAction)
	assert.Equal(t, apBefore-2, combatHandler.RemainingAP("rp6"))
}
