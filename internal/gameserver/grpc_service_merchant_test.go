package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/inventory"
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

	// Register a stim_pack item so browse can resolve display names.
	invReg := inventory.NewRegistry()
	err := invReg.RegisterItem(&inventory.ItemDef{
		ID:       "stim_pack",
		Name:     "Stim Pack",
		Kind:     "consumable",
		MaxStack: 10,
	})
	require.NoError(t, err)
	svc.invRegistry = invReg

	uid := "merch_u1"
	_, err = svc.sessions.AddPlayer(session.AddPlayerOptions{
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
	assert.Contains(t, evt.GetMessage().Content, "Stim Pack", "browse should show display name, not raw item ID")
	assert.NotContains(t, evt.GetMessage().Content, "stim_pack", "browse must not show raw item ID")
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

// newMerchantTestServerWithMaterialReg builds a GameServiceServer with a materialReg and a
// merchant that has a MaterialStock entry for the given material.
//
// Precondition: t must be non-nil; matReg must be non-nil.
// Postcondition: Returns server, uid, and the spawned merchant instance.
func newMerchantTestServerWithMaterialReg(t *testing.T, matReg *crafting.MaterialRegistry) (*GameServiceServer, string, *npc.Instance) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcManager, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger,
		nil, nil, nil, npcManager,
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.materialReg = matReg

	uid := "matmerch_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: "mat_user",
		CharName: "MatChar",
		RoomID:   "room_a",
		CurrentHP: 10,
		MaxHP:    10,
		Role:     "player",
	})
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID:      "mat_merchant",
		Name:    "MaterialShop",
		NPCType: "merchant",
		MaxHP:   20,
		AC:      10,
		Level:   1,
		Merchant: &npc.MerchantConfig{
			MerchantType: "consumables",
			SellMargin:   1.0,
			BuyMargin:    0.5,
			Budget:       300,
			Inventory:    []npc.MerchantItem{},
			MaterialStock: []npc.MaterialStockItem{
				{ID: "scrap_metal", Price: 30, RestockQuantity: 10},
			},
			ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.initMerchantRuntimeState(inst)

	return svc, uid, inst
}

func TestHandleBuy_Material_Success(t *testing.T) {
	matReg := crafting.NewMaterialRegistryFromSlice([]*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "metal", Value: 10},
	})
	svc, uid, inst := newMerchantTestServerWithMaterialReg(t, matReg)

	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "Scrap Metal", Quantity: 1})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "buy")
	assert.Contains(t, evt.GetMessage().Content, "Scrap Metal")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 470, sess.Currency)
	assert.Equal(t, 1, sess.Materials["scrap_metal"])
}

func TestHandleBuy_Material_InsufficientCredits(t *testing.T) {
	matReg := crafting.NewMaterialRegistryFromSlice([]*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "metal", Value: 10},
	})
	svc, uid, inst := newMerchantTestServerWithMaterialReg(t, matReg)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 0

	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "Scrap Metal", Quantity: 1})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "afford")
}

func TestHandleBuy_Material_NilRegistry(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcManager, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(worldMgr, sessMgr, cmdRegistry, worldHandler, chatHandler, logger,
		nil, nil, nil, npcManager,
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	// materialReg intentionally left nil

	uid := "nilreg_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: "nilreg_user",
		CharName: "NilChar",
		RoomID:   "room_a",
		CurrentHP: 10,
		MaxHP:    10,
		Role:     "player",
	})
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID:      "nilreg_merchant",
		Name:    "NilRegShop",
		NPCType: "merchant",
		MaxHP:   20,
		AC:      10,
		Level:   1,
		Merchant: &npc.MerchantConfig{
			MerchantType: "consumables",
			SellMargin:   1.0,
			BuyMargin:    0.5,
			Budget:       300,
			Inventory:    []npc.MerchantItem{},
			MaterialStock: []npc.MaterialStockItem{
				{ID: "scrap_metal", Price: 30, RestockQuantity: 10},
			},
			ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.initMerchantRuntimeState(inst)

	// Must not panic; materialReg is nil so the item should not be found.
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "Scrap Metal", Quantity: 1})
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetMessage().Content, "doesn't sell")
}

func TestProperty_HandleBuy_MaterialStock_NeverPanics(t *testing.T) {
	matReg := crafting.NewMaterialRegistryFromSlice([]*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "metal", Value: 10},
		{ID: "bleach", Name: "Bleach", Category: "chemical", Value: 5},
	})
	svc, uid, inst := newMerchantTestServerWithMaterialReg(t, matReg)

	rapid.Check(t, func(rt *rapid.T) {
		itemID := rapid.String().Draw(rt, "item_id")
		credits := rapid.IntRange(0, 1000).Draw(rt, "credits")
		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(t, ok)
		sess.Currency = credits

		evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{
			NpcName:  inst.Name(),
			ItemId:   itemID,
			Quantity: 1,
		})
		assert.NoError(t, err)
		assert.NotNil(t, evt)
	})
}
