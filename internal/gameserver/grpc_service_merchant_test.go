package gameserver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"
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
	shop := evt.GetShopView()
	require.NotNil(t, shop, "browse should return a ShopView event")
	var names []string
	for _, item := range shop.Items {
		names = append(names, item.Name)
	}
	assert.Contains(t, names, "Stim Pack", "browse should show display name, not raw item ID")
	for _, item := range shop.Items {
		assert.NotEqual(t, "stim_pack", item.Name, "browse must not show raw item ID as name")
	}
}

func TestHandleBrowse_NpcNotFound(t *testing.T) {
	svc, uid, _ := newMerchantTestServer(t)
	evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: "ghost"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "don't see")
	assert.Nil(t, evt.GetShopView(), "NPC not found should return MessageEvent, not ShopView")
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

// REQ-NPC-BUY-1: A successful purchase MUST add the item to the player's backpack.
func TestHandleBuy_SuccessAddsItemToBackpack(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	require.Empty(t, sess.Backpack.FindByItemDefID("stim_pack"), "backpack must be empty before purchase")

	_, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)

	items := sess.Backpack.FindByItemDefID("stim_pack")
	assert.Len(t, items, 1, "backpack must contain exactly 1 stim_pack after purchase")
	assert.Equal(t, 1, items[0].Quantity)
}

// newArmorMerchantTestServer builds a GameServiceServer with a merchant that sells an armor item (leather_jacket).
// Tests armor purchase flow: ItemDef (kind=armor) must be registered for Backpack.Add to succeed.
func newArmorMerchantTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	invReg := inventory.NewRegistry()
	err := invReg.RegisterItem(&inventory.ItemDef{
		ID:       "leather_jacket",
		Name:     "Leather Jacket",
		Kind:     "armor",
		ArmorRef: "leather_jacket",
		Weight:   1.0,
		MaxStack: 1,
		Value:    150,
	})
	require.NoError(t, err)
	svc.invRegistry = invReg

	uid := "armor_u1"
	_, err = svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "armor_user",
		CharName:  "ArmorChar",
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
		ID:      "test_armor_merchant",
		Name:    "Ellie Mack",
		NPCType: "merchant",
		MaxHP:   20,
		AC:      10,
		Level:   1,
		Merchant: &npc.MerchantConfig{
			MerchantType: "armor",
			SellMargin:   1.3,
			BuyMargin:    0.4,
			Budget:       2000,
			Inventory: []npc.MerchantItem{
				{ItemID: "leather_jacket", BasePrice: 100, InitStock: 3, MaxStock: 5},
			},
			ReplenishRate: npc.ReplenishConfig{MinHours: 8, MaxHours: 16},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.initMerchantRuntimeState(inst)

	return svc, uid, inst
}

// REQ-NPC-BUY-4: Buying an armor item MUST add it to the backpack and deduct currency.
// Regression test for the bug where armor items (non-stackable, kind=armor) were purchased
// (stock decremented, currency deducted) but not added to the backpack.
func TestHandleBuy_ArmorItemAddsToBackpackAndDeductsCredits(t *testing.T) {
	svc, uid, inst := newArmorMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	require.Empty(t, sess.Backpack.FindByItemDefID("leather_jacket"), "backpack must be empty before purchase")
	initialCurrency := sess.Currency

	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "leather_jacket", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy", "purchase should succeed")

	// Item must be in backpack.
	items := sess.Backpack.FindByItemDefID("leather_jacket")
	assert.Len(t, items, 1, "backpack must contain exactly 1 leather_jacket after purchase")
	assert.Equal(t, 1, items[0].Quantity)

	// Currency must be deducted.
	assert.Less(t, sess.Currency, initialCurrency, "currency must be deducted after armor purchase")

	// Stock must be decremented.
	state := svc.merchantStateFor(inst.ID)
	require.NotNil(t, state)
	assert.Equal(t, 2, state.Stock["leather_jacket"], "merchant stock must decrement by 1")
}

// REQ-NPC-BUY-5: When Backpack.Add fails, handleBuy MUST roll back stock and currency.
func TestHandleBuy_RollsBackOnBackpackAddFailure(t *testing.T) {
	svc, uid, inst := newArmorMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Fill the backpack to capacity so Add will fail.
	for i := 0; i < 20; i++ {
		inst2 := &inventory.ItemInstance{
			InstanceID: fmt.Sprintf("filler-%d", i),
			ItemDefID:  "leather_jacket",
			Quantity:   1,
		}
		_ = sess.Backpack.AddInstance(inst2)
	}
	initialCurrency := sess.Currency
	initialStock := svc.merchantStateFor(inst.ID).Stock["leather_jacket"]

	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "leather_jacket", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Purchase failed", "should report failure when backpack is full")

	// Currency must NOT be deducted.
	assert.Equal(t, initialCurrency, sess.Currency, "currency must be restored on backpack add failure")

	// Stock must NOT be decremented.
	finalStock := svc.merchantStateFor(inst.ID).Stock["leather_jacket"]
	assert.Equal(t, initialStock, finalStock, "stock must be restored on backpack add failure")
}

// newWeaponMerchantTestServer builds a GameServiceServer with a merchant that sells a weapon item (scrap_shield).
// Tests weapon purchase flow: ItemDef (kind=weapon) must be registered for Backpack.Add to succeed.
func newWeaponMerchantTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	invReg := inventory.NewRegistry()
	// Register the weapon ItemDef — required for Backpack.Add to succeed (REQ-NPC-BUY-1).
	err := invReg.RegisterItem(&inventory.ItemDef{
		ID:        "scrap_shield",
		Name:      "Scrap Shield",
		Kind:      "weapon",
		WeaponRef: "scrap_shield",
		Weight:    3.0,
		MaxStack:  1,
		Value:     40,
	})
	require.NoError(t, err)
	svc.invRegistry = invReg

	uid := "shield_u1"
	_, err = svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "shield_user",
		CharName:  "ShieldChar",
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
		ID:      "test_arms_dealer",
		Name:    "Arms Dealer",
		NPCType: "merchant",
		MaxHP:   20,
		AC:      10,
		Level:   1,
		Merchant: &npc.MerchantConfig{
			MerchantType: "weapons",
			SellMargin:   1.0,
			BuyMargin:    0.5,
			Budget:       300,
			Inventory: []npc.MerchantItem{
				{ItemID: "scrap_shield", BasePrice: 50, InitStock: 3, MaxStock: 5},
			},
			ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
		},
	}
	inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	svc.initMerchantRuntimeState(inst)

	return svc, uid, inst
}

// REQ-NPC-BUY-3: Buying a weapon item MUST add it to the backpack and deduct currency.
// Regression test for the bug where shields could be purchased (stock decremented) but the
// item was never added to the backpack because no ItemDef was registered for weapon items.
func TestHandleBuy_WeaponItemAddsToBackpackAndDeductsCredits(t *testing.T) {
	svc, uid, inst := newWeaponMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	require.Empty(t, sess.Backpack.FindByItemDefID("scrap_shield"), "backpack must be empty before purchase")
	initialCurrency := sess.Currency

	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "scrap_shield", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy", "purchase should succeed")

	// Item must be in backpack.
	items := sess.Backpack.FindByItemDefID("scrap_shield")
	assert.Len(t, items, 1, "backpack must contain exactly 1 scrap_shield after purchase")
	assert.Equal(t, 1, items[0].Quantity)

	// Currency must be deducted.
	assert.Less(t, sess.Currency, initialCurrency, "currency must be deducted after weapon purchase")

	// Stock must be decremented.
	state := svc.merchantStateFor(inst.ID)
	require.NotNil(t, state)
	assert.Equal(t, 2, state.Stock["scrap_shield"], "merchant stock must decrement by 1")
}

// REQ-NPC-BUY-2: buy MUST match items by display name, slug, or partial case-insensitive name.
func TestHandleBuy_MatchByDisplayName(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	// invRegistry has stim_pack with Name "Stim Pack"; try matching by display name.
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "Stim Pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy", "display name match should succeed")

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 450, sess.Currency, "currency must be deducted on display-name match")
}

func TestHandleBuy_MatchBySlug(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	// "stim-pack" should resolve to "stim_pack" via slug normalization.
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim-pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy", "slug match should succeed")
}

func TestHandleBuy_MatchCaseInsensitive(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "STIM_PACK", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "buy", "case-insensitive item ID match should succeed")
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

	// Player must have the item in their backpack before selling.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	_, err := sess.Backpack.Add("stim_pack", 1, svc.invRegistry)
	require.NoError(t, err)

	evt, sellErr := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, sellErr)
	assert.Contains(t, evt.GetMessage().Content, "buy")

	// 500 + floor(50 * 0.5 * 1) = 525
	assert.Equal(t, 525, sess.Currency)
}

// REQ-NPC-SELL-1: A successful sale MUST remove the sold item from the player's backpack.
func TestHandleSell_SuccessRemovesItemFromBackpack(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	_, err := sess.Backpack.Add("stim_pack", 1, svc.invRegistry)
	require.NoError(t, err)
	require.Len(t, sess.Backpack.FindByItemDefID("stim_pack"), 1, "backpack must have stim_pack before sell")

	_, sellErr := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, sellErr)

	items := sess.Backpack.FindByItemDefID("stim_pack")
	assert.Empty(t, items, "backpack must be empty after selling the only stim_pack")
}

// REQ-NPC-SELL-2: Selling an item the player does not own MUST return an error message.
func TestHandleSell_ItemNotInInventory_ReturnsError(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)

	// Backpack is empty — no stim_pack to sell.
	evt, err := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "don't have")

	// Currency must be unchanged.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 500, sess.Currency)
}

func TestHandleSell_BudgetExhausted(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)
	svc.merchantStateFor(inst.ID).CurrentBudget = 0

	// Player must have the item to reach the budget check.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	_, err := sess.Backpack.Add("stim_pack", 1, svc.invRegistry)
	require.NoError(t, err)

	evt, sellErr := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
	require.NoError(t, sellErr)
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

// REQ-NPC-BUY-3: A successful purchase MUST push an updated ShopView to the client
// so the web UI reflects the decremented stock without requiring the player to
// close and reopen the shop (BUG-102).
func TestHandleBuy_PushesUpdatedShopViewAfterPurchase(t *testing.T) {
	svc, uid, inst := newMerchantTestServer(t)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)

	// Attach a BridgeEntity so push calls are capturable.
	entity := session.NewBridgeEntity(uid, 32)
	sess.Entity = entity

	_, err := svc.handleBuy(uid, &gamev1.BuyRequest{
		NpcName:  inst.Name(),
		ItemId:   "stim_pack",
		Quantity: 1,
	})
	require.NoError(t, err)

	// Drain all pushed events and look for a ShopView.
	entity.Close()
	var foundShopView bool
	for data := range entity.Events() {
		var evt gamev1.ServerEvent
		if unmarshalErr := proto.Unmarshal(data, &evt); unmarshalErr != nil {
			continue
		}
		if sv := evt.GetShopView(); sv != nil {
			foundShopView = true
			// The stock for stim_pack must be 2 (initial 3 minus 1 purchased).
			for _, item := range sv.Items {
				if item.ItemId == "stim_pack" {
					assert.Equal(t, int32(2), item.Stock, "ShopView must reflect decremented stock after purchase")
				}
			}
		}
	}
	assert.True(t, foundShopView, "handleBuy must push a ShopView event after a successful purchase")
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
