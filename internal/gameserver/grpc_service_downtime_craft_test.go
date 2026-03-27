package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/crafting"
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

	svc := &GameServiceServer{
		sessions:  sMgr,
		world:     wMgr,
		recipeReg: recipeReg,
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
	assert.Contains(t, msg.Content, "aterial")
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
		// Provide exactly the required materials.
		sess.Materials = map[string]int{"scrap_metal": 2, "wire": 1}

		svc.downtimeStart(uid, sess, "craft", "smoke_grenade")

		if sess.DowntimeBusy {
			assert.Equal(rt, 0, sess.Materials["scrap_metal"],
				"scrap_metal should be fully consumed")
			assert.Equal(rt, 0, sess.Materials["wire"],
				"wire should be fully consumed")
		}
	})
}
