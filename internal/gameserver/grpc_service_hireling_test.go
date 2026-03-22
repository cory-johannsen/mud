package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newHirelingTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcMgr)

	uid := "hl_u1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "hl_user",
		CharName:  "HLChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     50,
		Role:      "player",
		Level:     3,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID:      "test_hireling",
		Name:    "Patch",
		NPCType: "hireling",
		Level:   3,
		MaxHP:   25,
		AC:      12,
		Hireling: &npc.HirelingConfig{
			DailyCost:      50,
			CombatRole:     "melee",
			MaxFollowZones: 2,
		},
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return svc, uid
}

func TestHandleHire_Success(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Patch")
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 450, sess.Currency, "daily cost deducted")
}

func TestHandleHire_AlreadyHired(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	require.NotNil(t, inst)
	svc.initHirelingRuntimeState(inst)
	state := svc.hirelingStateFor(inst.ID)
	require.NotNil(t, state)
	state.HiredByPlayerID = "other_player"

	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "already")
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 500, sess.Currency, "currency unchanged when hire fails")
}

func TestHandleHire_InsufficientCredits(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 10
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "credits")
	assert.Equal(t, 10, sess.Currency)
}

func TestHandleHire_NpcNotFound(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Nobody"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Nobody")
}

func TestHandleDismiss_Success(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	evt, err := svc.handleDismiss(uid, &gamev1.DismissRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "dismiss")

	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	if inst != nil {
		state := svc.hirelingStateFor(inst.ID)
		if state != nil {
			assert.Empty(t, state.HiredByPlayerID)
		}
	}
}

func TestHandleDismiss_NoHireling(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleDismiss(uid, &gamev1.DismissRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no hireling")
}

func TestTickHirelingDailyCost_InsufficientCredits(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 0

	svc.tickHirelingDailyCost()

	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	if inst != nil {
		state := svc.hirelingStateFor(inst.ID)
		if state != nil {
			assert.Empty(t, state.HiredByPlayerID, "hireling auto-dismissed when player can't pay")
		}
	}
}

func TestProperty_HandleHire_CurrencyNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newHirelingTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = rapid.IntRange(0, 500).Draw(rt, "currency")
		_, _ = svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
		sess, _ = svc.sessions.GetPlayer(uid)
		if sess.Currency < 0 {
			rt.Fatalf("currency went negative: %d", sess.Currency)
		}
	})
}

func TestProperty_HandleHire_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newHirelingTestServer(t)
		npcName := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(rt, "npcName")
		_, _ = svc.handleHire(uid, &gamev1.HireRequest{NpcName: npcName})
	})
}
