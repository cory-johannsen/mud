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

func newBankerTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
	t.Helper()
	svc := testServiceWithAdmin(t, nil)
	svc.npcMgr = npc.NewManager()

	uid := "bank_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: "bank_user",
		CharName: "BankChar",
		RoomID:   "room_a",
		CurrentHP: 10,
		MaxHP:    10,
		Role:     "player",
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 200
	sess.StashBalance = 100

	tmpl := &npc.Template{
		ID:      "test_banker",
		Name:    "Vault Keeper",
		NPCType: "banker",
		MaxHP:   10,
		AC:      10,
		Level:   1,
		Banker:  &npc.BankerConfig{ZoneID: "test", BaseRate: 1.0, RateVariance: 0.05},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.bankerRuntimeStates[inst.ID] = &npc.BankerRuntimeState{CurrentRate: 1.0}

	return svc, uid, inst
}

func TestHandleStashDeposit_SuccessDeductsCreditsAddsStash(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 100})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "deposited")

	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 100, sess.Currency)
	assert.Equal(t, 200, sess.StashBalance)
}

func TestHandleStashDeposit_InsufficientCredits(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 500})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "don't have")
}

func TestHandleStashWithdraw_SuccessDeductsStashAddsCredits(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	evt, err := svc.handleStashWithdraw(uid, &gamev1.StashWithdrawRequest{NpcName: inst.Name(), Amount: 50})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "withdrew")

	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 50, sess.StashBalance)
	assert.Equal(t, 250, sess.Currency)
}

func TestHandleStashWithdraw_InsufficientStash(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	evt, err := svc.handleStashWithdraw(uid, &gamev1.StashWithdrawRequest{NpcName: inst.Name(), Amount: 500})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "don't have enough")
}

func TestHandleStashBalance_DisplaysRateAndBalance(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	evt, err := svc.handleStashBalance(uid, &gamev1.StashBalanceRequest{NpcName: inst.Name()})
	require.NoError(t, err)
	msg := evt.GetMessage().Content
	assert.Contains(t, msg, "100")
	assert.Contains(t, msg, "1.00")
}

func TestHandleStashDeposit_CoweringBankerBlocked(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	inst.Cowering = true
	evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 10})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "cower")
}

func TestProperty_HandleStashDeposit_NeverPanics(t *testing.T) {
	svc, uid, inst := newBankerTestServer(t)
	rapid.Check(t, func(rt *rapid.T) {
		amt := rapid.Int32().Draw(rt, "amount")
		evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: amt})
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}
