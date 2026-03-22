package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// testServiceWithNPCMgr creates a GameServiceServer with the given npcMgr injected.
func testServiceWithNPCMgr(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager, npcManager *npc.Manager) *GameServiceServer {
	t.Helper()
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcManager, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	return newTestGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger,
		nil, nil, nil, npcManager,
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// newMerchantTestServer builds a GameServiceServer with a real npc.Manager and
// a seeded merchant instance in room_a. Returns the server, the player UID, and
// the spawned merchant instance.
func newMerchantTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "merch_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "merch_user",
		CharName:  "MerchChar",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Role:      "player",
	})
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID:      "test_merchant",
		Name:    "Shopkeeper",
		NPCType: "merchant",
		MaxHP:   20,
		AC:      10,
		Level:   1,
		Merchant: &npc.MerchantConfig{
			MerchantType: "consumables",
			SellMargin:   1.0,
			BuyMargin:    0.5,
			Budget:       300,
			Inventory: []npc.MerchantItem{
				{ItemID: "stim_pack", BasePrice: 50, InitStock: 3, MaxStock: 5},
			},
			ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.initMerchantRuntimeState(inst)

	return svc, uid, inst
}

func TestHandleBrowse_ListsInventory(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: inst.Name()})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "stim_pack")
}

func TestHandleBrowse_NpcNotFound(t *testing.T) {
	svc, uid, _ := newMerchantTestServer(t)
	evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: "ghost"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "don't see")
}

func TestHandleBuy_SuccessDeductsCreditsAndStock(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 450, sess.Currency)

	state := svc.merchantStateFor(inst.ID)
	require.NotNil(t, state)
	assert.Equal(t, 2, state.Stock["stim_pack"])
}

func TestHandleBuy_InsufficientCredits(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 10
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "afford")
}

func TestHandleBuy_OutOfStock(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	svc.merchantStateFor(inst.ID).Stock["stim_pack"] = 0
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "out of stock")
}

func TestHandleSell_SuccessPaysPlayer(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	evt, err := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// 500 + floor(50 * 0.5 * 1) = 525
	assert.Equal(t, 525, sess.Currency)
}

func TestHandleSell_BudgetExhausted(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	svc.merchantStateFor(inst.ID).CurrentBudget = 0
	evt, err := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "can't afford")
}

func TestHandleNegotiate_OnlyOncePerVisit(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.NegotiatedMerchantID = inst.ID

	evt, err := svc.handleNegotiate(uid, &gamev1.NegotiateRequest{NpcName: inst.Name(), Skill: "smooth_talk"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "already tried")
}

func TestHandleNegotiate_CoweringMerchantBlocked(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	inst.Cowering = true
	evt, err := svc.handleNegotiate(uid, &gamev1.NegotiateRequest{NpcName: inst.Name()})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "cower")
}

func TestNegotiateModifier_ClearedOnRoomTransition(t *testing.T) {
	svc, uid, _ := newMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.NegotiateModifier = 0.2
	sess.NegotiatedMerchantID = "some-merchant-id"

	svc.clearNegotiateState(sess)

	assert.Equal(t, 0.0, sess.NegotiateModifier)
	assert.Equal(t, "", sess.NegotiatedMerchantID)
}

func TestProperty_HandleBrowse_NeverPanics(t *testing.T) {
	svc, uid, _ := newMerchantTestServer(t)
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.String().Draw(rt, "name")
		evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: name})
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}
