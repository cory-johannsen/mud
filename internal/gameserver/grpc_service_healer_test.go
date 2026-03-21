package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newHealerTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "heal_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "healer_user", CharName: "HealChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID: "test_healer", Name: "Clutch", NPCType: "healer",
		Level: 4, MaxHP: 22, AC: 11,
		Healer: &npc.HealerConfig{PricePerHP: 5, DailyCapacity: 100},
	}
	_, err = npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return svc, uid
}

func TestHandleHeal_Success(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 100, sess.CurrentHP)
	assert.Equal(t, 250, sess.Currency) // 500 - 5*50
	assert.Contains(t, evt.GetMessage().Content, "100")
}

func TestHandleHeal_AlreadyFullHP(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.CurrentHP = 100
	evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "full health")
}

func TestHandleHeal_InsufficientCredits(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 10
	evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "credits")
}

func TestHandleHeal_NpcNotFound(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Nobody"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Nobody")
}

func TestHandleHealAmount_Success(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	evt, err := svc.handleHealAmount(uid, &gamev1.HealAmountRequest{NpcName: "Clutch", Amount: 20})
	require.NoError(t, err)
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 70, sess.CurrentHP)  // 50 + 20
	assert.Equal(t, 400, sess.Currency)  // 500 - 5*20
	assert.Contains(t, evt.GetMessage().Content, "70")
}

func TestHandleHealAmount_ZeroAmount(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	evt, err := svc.handleHealAmount(uid, &gamev1.HealAmountRequest{NpcName: "Clutch", Amount: 0})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "positive")
}

func TestTickHealerCapacity_ResetsToZero(t *testing.T) {
	svc, _ := newHealerTestServer(t)
	// Manually set some capacity used
	healerRuntimeMu.Lock()
	for _, state := range svc.healerRuntimeStates {
		state.CapacityUsed = 50
	}
	healerRuntimeMu.Unlock()
	// If no states yet, add one directly
	if len(svc.healerRuntimeStates) == 0 {
		healerRuntimeMu.Lock()
		svc.healerRuntimeStates["test_inst"] = &npc.HealerRuntimeState{CapacityUsed: 50}
		healerRuntimeMu.Unlock()
	}
	svc.tickHealerCapacity()
	healerRuntimeMu.RLock()
	for _, state := range svc.healerRuntimeStates {
		assert.Equal(t, 0, state.CapacityUsed)
	}
	healerRuntimeMu.RUnlock()
}
