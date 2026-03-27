package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// newWorldWithWorkshopRoom creates a world.Manager with a room tagged "safe,workshop".
//
// Precondition: zoneID and roomID must be non-empty strings.
// Postcondition: The room at roomID has Properties["tags"] = "safe,workshop".
func newWorldWithWorkshopRoom(zoneID, roomID string) *world.Manager {
	r := &world.Room{
		ID:          roomID,
		ZoneID:      zoneID,
		Title:       "Workshop",
		Description: "A workshop for crafting.",
		MapX:        0,
		MapY:        0,
		Properties:  map[string]string{"tags": "safe,workshop"},
	}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: roomID,
		Rooms:     map[string]*world.Room{roomID: r},
	}
	mgr, err := world.NewManager([]*world.Zone{z})
	if err != nil {
		panic("newWorldWithWorkshopRoom: " + err.Error())
	}
	return mgr
}

// newCraftDowntimeTestService builds a minimal GameServiceServer with crafting, recipe, and material support.
//
// Precondition: none.
// Postcondition: Returns svc and uid; GetPlayer(uid) succeeds; svc.recipeReg is populated.
func newCraftDowntimeTestService(t testing.TB) (*GameServiceServer, string) {
	t.Helper()
	const uid = "craft_uid"
	wMgr := newWorldWithWorkshopRoom("zone1", "room1")
	sMgr := addPlayerToSession(uid, "room1")

	recipes := []*crafting.Recipe{
		{
			ID:           "smoke_grenade",
			Name:         "Smoke Grenade",
			OutputItemID: "smoke_grenade",
			OutputCount:  1,
			Complexity:   1,
			DC:           12,
			Materials: []crafting.RecipeMaterial{
				{ID: "scrap_metal", Quantity: 2},
				{ID: "wire", Quantity: 1},
			},
		},
	}
	recipeReg := crafting.NewRecipeRegistryFromSlice(recipes)

	invReg := inventory.NewRegistry()
	if err := invReg.RegisterItem(&inventory.ItemDef{
		ID:        "smoke_grenade",
		Name:      "Smoke Grenade",
		Kind:      "consumable",
		Stackable: true,
		MaxStack:  10,
		Weight:    0.5,
	}); err != nil {
		t.Fatalf("newCraftDowntimeTestService: RegisterItem: %v", err)
	}

	svc := &GameServiceServer{
		sessions:    sMgr,
		world:       wMgr,
		recipeReg:   recipeReg,
		invRegistry: invReg,
	}

	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Materials = map[string]int{
		"scrap_metal": 4,
		"wire":        2,
	}

	return svc, uid
}

// TestDowntimeCraft_NoRecipeArg_ReturnsError verifies that starting craft without a recipe arg returns an error.
//
// Precondition: svc.recipeReg is non-nil; activityArgs is empty.
// Postcondition: Returns a message containing "recipe"; sess.DowntimeBusy remains false.
func TestDowntimeCraft_NoRecipeArg_ReturnsError(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "craft", "")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "recipe")
}

// TestDowntimeCraft_UnknownRecipe_ReturnsError verifies that starting craft with an unknown recipe ID returns an error.
//
// Precondition: svc.recipeReg is non-nil; activityArgs is a non-existent recipe ID.
// Postcondition: Returns a message containing "recipe"; sess.DowntimeBusy remains false.
func TestDowntimeCraft_UnknownRecipe_ReturnsError(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "craft", "no_such_recipe")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "recipe")
	assert.False(t, sess.DowntimeBusy)
}

// TestDowntimeCraft_MissingMaterials_ReturnsError verifies that starting craft with insufficient materials returns an error.
//
// Precondition: sess.Materials lacks required quantities.
// Postcondition: Returns a message containing "material"; sess.DowntimeBusy remains false.
func TestDowntimeCraft_MissingMaterials_ReturnsError(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Materials = map[string]int{"scrap_metal": 1} // missing wire, only 1 scrap

	evt := svc.downtimeStart(uid, sess, "craft", "smoke_grenade")
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Missing materials")
	assert.False(t, sess.DowntimeBusy)
}

// TestDowntimeCraft_SufficientMaterials_StartsAndConsumes verifies that craft starts and consumes materials when all are present.
//
// Precondition: sess.Materials contains at least the required quantities.
// Postcondition: sess.DowntimeBusy==true; sess.DowntimeActivityID=="craft"; sess.DowntimeMetadata=="smoke_grenade";
//
//	required materials deducted from sess.Materials.
func TestDowntimeCraft_SufficientMaterials_StartsAndConsumes(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)

	evt := svc.downtimeStart(uid, sess, "craft", "smoke_grenade")
	require.NotNil(t, evt)
	assert.True(t, sess.DowntimeBusy)
	assert.Equal(t, "craft", sess.DowntimeActivityID)
	assert.Equal(t, "smoke_grenade", sess.DowntimeMetadata)
	// Materials consumed at start: scrap_metal 4-2=2, wire 2-1=1
	assert.Equal(t, 2, sess.Materials["scrap_metal"])
	assert.Equal(t, 1, sess.Materials["wire"])
}

// TestProperty_DowntimeCraft_MaterialsAlwaysConsumedOnStart is a property-based test verifying
// that when craft starts successfully, all required materials are fully consumed from sess.Materials.
//
// Precondition: sess.Materials contains exactly the required quantities.
// Postcondition: If sess.DowntimeBusy, then all required material quantities are zero in sess.Materials.
func TestProperty_DowntimeCraft_MaterialsAlwaysConsumedOnStart(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newCraftDowntimeTestService(t)
		sess, _ := svc.sessions.GetPlayer(uid)

		// Generate material quantities >= required (scrap_metal >= 2, wire >= 1).
		scrapQty := rapid.IntRange(2, 20).Draw(rt, "scrap_qty")
		wireQty := rapid.IntRange(1, 10).Draw(rt, "wire_qty")
		sess.Materials = map[string]int{"scrap_metal": scrapQty, "wire": wireQty}

		evt := svc.downtimeStart(uid, sess, "craft", "smoke_grenade")
		require.NotNil(rt, evt)

		if sess.DowntimeBusy {
			// All recipe materials (scrap_metal: 2, wire: 1) must be consumed.
			assert.Equal(rt, scrapQty-2, sess.Materials["scrap_metal"],
				"scrap_metal should be reduced by recipe quantity 2")
			assert.Equal(rt, wireQty-1, sess.Materials["wire"],
				"wire should be reduced by recipe quantity 1")
		}
	})
}

// TestDowntimeCraft_ResolveCritSuccess_ItemDeliveredAndMaterialsRefunded verifies that a
// critical success delivers outputCount+1 items and refunds all recipe materials.
//
// Precondition: sess.DowntimeMetadata="smoke_grenade"; dice forced to roll 20; Savvy=14 (mod+2); DC=12.
// Postcondition: smoke_grenade in backpack; scrap_metal==2; wire==1 (refunded).
func TestDowntimeCraft_ResolveCritSuccess_ItemDeliveredAndMaterialsRefunded(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = "smoke_grenade"
	sess.Materials = map[string]int{}
	// Savvy=14 → abilityMod=2; roll=20; profBonus=0 (untrained) → total=22 >= DC+10=22 → CritSuccess.
	sess.Abilities.Savvy = 14
	svc.dice = dice.NewRoller(dice.NewDeterministicSource([]int{19})) // Intn(20)=19 → roll=20

	svc.resolveDowntimeCraft(uid, sess)

	items := sess.Backpack.FindByItemDefID("smoke_grenade")
	assert.NotEmpty(t, items, "expected smoke_grenade in backpack on crit success")
	// Materials refunded (one batch: scrap_metal=2, wire=1).
	assert.Equal(t, 2, sess.Materials["scrap_metal"], "crit success should refund scrap_metal")
	assert.Equal(t, 1, sess.Materials["wire"], "crit success should refund wire")
}

// TestDowntimeCraft_ResolveSuccess_ItemDeliveredNoRefund verifies that a normal success delivers
// outputCount items without refunding materials.
//
// Precondition: sess.DowntimeMetadata="smoke_grenade"; dice forced to roll 11; Savvy=14 (mod+2); DC=12.
// Postcondition: smoke_grenade in backpack; materials unchanged (no refund).
func TestDowntimeCraft_ResolveSuccess_ItemDeliveredNoRefund(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = "smoke_grenade"
	sess.Materials = map[string]int{}
	// Savvy=14 → abilityMod=2; roll=11; total=13 >= 12, < 22 → Success.
	sess.Abilities.Savvy = 14
	svc.dice = dice.NewRoller(dice.NewDeterministicSource([]int{10})) // Intn(20)=10 → roll=11

	svc.resolveDowntimeCraft(uid, sess)

	items := sess.Backpack.FindByItemDefID("smoke_grenade")
	assert.NotEmpty(t, items, "expected smoke_grenade on success")
	assert.Equal(t, 0, sess.Materials["scrap_metal"])
	assert.Equal(t, 0, sess.Materials["wire"])
}

// TestDowntimeCraft_ResolveFailure_NoItemNoRefund verifies that a failure produces no items and
// no material refund.
//
// Precondition: sess.DowntimeMetadata="smoke_grenade"; dice forced to roll 1; Savvy=10 (mod=0); DC=12.
// Postcondition: backpack empty; materials unchanged.
func TestDowntimeCraft_ResolveFailure_NoItemNoRefund(t *testing.T) {
	svc, uid := newCraftDowntimeTestService(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.DowntimeMetadata = "smoke_grenade"
	sess.Materials = map[string]int{}
	// Savvy=10 → abilityMod=0; roll=1; total=1 < 12 → Failure.
	svc.dice = dice.NewRoller(dice.NewDeterministicSource([]int{0})) // Intn(20)=0 → roll=1

	svc.resolveDowntimeCraft(uid, sess)

	items := sess.Backpack.FindByItemDefID("smoke_grenade")
	assert.Empty(t, items, "no item on failure")
	assert.Equal(t, 0, sess.Materials["scrap_metal"])
}

// TestProperty_DowntimeCraft_CritSuccessAlwaysRefundsMaterials is a property-based test verifying
// that when dice are forced to produce a critical success, materials are always refunded.
//
// Precondition: sess.DowntimeMetadata="smoke_grenade"; dice forced for crit success; Savvy=14.
// Postcondition: scrap_metal==2, wire==1 for every run.
func TestProperty_DowntimeCraft_CritSuccessAlwaysRefundsMaterials(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newCraftDowntimeTestService(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.DowntimeMetadata = "smoke_grenade"
		sess.Materials = map[string]int{}
		// Savvy=14 → abilityMod=2; roll=20; total=22 → CritSuccess (DC+10=22).
		sess.Abilities.Savvy = 14
		svc.dice = dice.NewRoller(dice.NewDeterministicSource([]int{19})) // Intn(20)=19 → roll=20

		svc.resolveDowntimeCraft(uid, sess)

		// Since roll is forced to 20 + abilityMod=2 = 22 = DC+10, always crit success.
		assert.Equal(rt, 2, sess.Materials["scrap_metal"])
		assert.Equal(rt, 1, sess.Materials["wire"])
	})
}
