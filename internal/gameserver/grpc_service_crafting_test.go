package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildCraftingServer creates a GameServiceServer configured for crafting handler tests.
//
// Precondition: t must be non-nil; matReg may be nil (registry skipped); zonePool may be nil.
// Postcondition: Returns a non-nil *GameServiceServer and the uid of the registered player.
func buildCraftingServer(
	t *testing.T,
	matReg *crafting.MaterialRegistry,
	matRepo CharacterMaterialsRepository,
	zonePool *world.MaterialPool,
) (*GameServiceServer, string) {
	t.Helper()

	zone := &world.Zone{
		ID:           "test",
		Name:         "Test",
		Description:  "Test zone",
		StartRoom:    "room_a",
		MaterialPool: zonePool,
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Exits:       []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties:  map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties:  map[string]string{},
			},
		},
	}

	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		worldHandler,
		chatHandler,
		logger,
		nil, nil, nil, nil, nil, nil,
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
	svc.materialReg = matReg
	svc.materialRepo = matRepo

	uid := "craft_u1"
	_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "crafter",
		CharName:    "Crafter",
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Role:        "player",
	})
	require.NoError(t, addErr)
	return svc, uid
}

// stubMaterialsRepo is a no-op CharacterMaterialsRepository used in crafting tests.
type stubMaterialsRepo struct{}

func (r *stubMaterialsRepo) Load(_ context.Context, _ int64) (map[string]int, error) {
	return make(map[string]int), nil
}

func (r *stubMaterialsRepo) Add(_ context.Context, _ int64, _ string, _ int) error {
	return nil
}

func (r *stubMaterialsRepo) DeductMany(_ context.Context, _ int64, _ map[string]int) error {
	return nil
}

// newMaterialRegistry builds a MaterialRegistry directly from a slice of Materials
// without requiring a YAML file, for use in tests only.
func newMaterialRegistry(mats []*crafting.Material) *crafting.MaterialRegistry {
	return crafting.NewMaterialRegistryFromSlice(mats)
}

// TestHandleMaterials_NoFilter verifies that requesting materials without a category
// filter shows all materials the player holds, including names and quantities.
//
// Precondition: sess.Materials = {"scrap_metal": 3, "bleach": 1}; both are in materialReg.
// Postcondition: Response message contains "Scrap Metal" and quantity 3.
func TestHandleMaterials_NoFilter(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
		{ID: "bleach", Name: "Bleach", Category: "chemical"},
	}
	matReg := newMaterialRegistry(mats)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Materials = map[string]int{
		"scrap_metal": 3,
		"bleach":      1,
	}

	evt, err := svc.handleMaterials(uid, &gamev1.MaterialsRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event")

	assert.Contains(t, msg.Content, "Scrap Metal", "must show material name")
	assert.Contains(t, msg.Content, "3", "must show quantity 3")
}

// TestHandleMaterials_EmptyInventory verifies that a player with no materials receives
// an appropriate "no materials" message.
//
// Precondition: sess.Materials is empty.
// Postcondition: Response message indicates no materials held.
func TestHandleMaterials_EmptyInventory(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
	}
	matReg := newMaterialRegistry(mats)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Materials = map[string]int{}

	evt, err := svc.handleMaterials(uid, &gamev1.MaterialsRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, strings.ToLower(msg.Content), "no materials")
}

// TestHandleMaterials_CategoryFilter verifies that a category filter restricts results
// to only materials of that category.
//
// Precondition: sess has mechanical and chemical materials; filter set to "mechanical".
// Postcondition: Response contains "Scrap Metal" and does not contain "Bleach".
func TestHandleMaterials_CategoryFilter(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
		{ID: "bleach", Name: "Bleach", Category: "chemical"},
	}
	matReg := newMaterialRegistry(mats)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Materials = map[string]int{
		"scrap_metal": 2,
		"bleach":      1,
	}

	evt, err := svc.handleMaterials(uid, &gamev1.MaterialsRequest{Category: "mechanical"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Scrap Metal")
	assert.NotContains(t, msg.Content, "Bleach")
}

// TestHandleScavenge_ExhaustedRoom_Rejected verifies that a player who already scavenged
// the current room cannot scavenge again (REQ-CRAFT-11).
//
// Precondition: sess.ScavengeExhaustedRoomID == sess.RoomID ("room_a").
// Postcondition: Response is an error event containing "already scavenged".
func TestHandleScavenge_ExhaustedRoom_Rejected(t *testing.T) {
	svc, uid := buildCraftingServer(t, nil, &stubMaterialsRepo{}, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.ScavengeExhaustedRoomID = "room_a" // same as RoomID

	evt, err := svc.handleScavenge(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected error event for exhausted room")
	assert.Contains(t, strings.ToLower(errEvt.Message), "already")
}

// TestHandleScavenge_NoPool_YieldsNothing verifies that scavenging in a zone without a
// material pool results in a "nothing to scavenge" message (REQ-CRAFT-9).
//
// Precondition: Zone has no material_pool (nil); player has not previously scavenged.
// Postcondition: Response is a message event containing "nothing" or equivalent.
func TestHandleScavenge_NoPool_YieldsNothing(t *testing.T) {
	// nil pool means no material pool in zone
	svc, uid := buildCraftingServer(t, nil, &stubMaterialsRepo{}, nil)

	evt, err := svc.handleScavenge(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event for no pool")
	assert.Contains(t, strings.ToLower(msg.Content), "nothing")
}

// TestHandleScavenge_EmptyDrops_YieldsNothing verifies that a pool with an empty drops
// list results in a "nothing useful" message.
//
// Precondition: Zone has MaterialPool with DC=10 but empty Drops slice.
// Postcondition: Response message contains "nothing".
func TestHandleScavenge_EmptyDrops_YieldsNothing(t *testing.T) {
	pool := &world.MaterialPool{DC: 10, Drops: []world.MaterialPoolDrop{}}
	svc, uid := buildCraftingServer(t, nil, &stubMaterialsRepo{}, pool)

	evt, err := svc.handleScavenge(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, strings.ToLower(msg.Content), "nothing")
}

// TestHandleScavenge_MarksRoomExhausted verifies that after a scavenge attempt the
// session's ScavengeExhaustedRoomID is set to the current room.
//
// Precondition: ScavengeExhaustedRoomID is empty; zone has no pool (simplest failure path).
// Postcondition: ScavengeExhaustedRoomID == "room_a".
func TestHandleScavenge_MarksRoomExhausted(t *testing.T) {
	svc, uid := buildCraftingServer(t, nil, &stubMaterialsRepo{}, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	require.Empty(t, sess.ScavengeExhaustedRoomID)

	_, err := svc.handleScavenge(uid)
	require.NoError(t, err)

	assert.Equal(t, "room_a", sess.ScavengeExhaustedRoomID)
}

// newRecipeRegistry builds a RecipeRegistry from a slice of recipes for use in tests.
func newRecipeRegistry(recipes []*crafting.Recipe) *crafting.RecipeRegistry {
	return crafting.NewRecipeRegistryFromSlice(recipes)
}

// TestHandleCraftList_ShowsDowntimeOnly verifies that when a player's rigging rank is
// below the recipe's EffectiveMinRank the listing shows a "[downtime only]" marker.
//
// Precondition: player rigging rank = "untrained"; recipe EffectiveMinRank = "trained" (complexity=2).
// Postcondition: response message contains "[downtime only]".
func TestHandleCraftList_ShowsDowntimeOnly(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
	}
	matReg := newMaterialRegistry(mats)
	recipes := []*crafting.Recipe{
		{
			ID:         "stimpack",
			Name:       "Stimpack",
			Category:   "medical",
			Complexity: 2, // EffectiveMinRank = "trained"
			DC:         15,
			Materials:  []crafting.RecipeMaterial{{ID: "scrap_metal", Quantity: 2}},
		},
	}
	recipeReg := newRecipeRegistry(recipes)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)
	svc.recipeReg = recipeReg
	svc.craftEngine = crafting.NewEngine()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills = map[string]string{"rigging": "untrained"}
	sess.Materials = map[string]int{}

	evt, err := svc.handleCraftList(uid, &gamev1.CraftListRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event")
	assert.Contains(t, msg.Content, "[downtime only]")
}

// TestHandleCraftList_ShowsMissingMaterials verifies that when a player has none of the
// required materials the listing shows a "[missing: N]" marker for those materials.
//
// Precondition: player has 0 materials; recipe requires 2 distinct material types.
// Postcondition: response message contains "[missing: 2]".
func TestHandleCraftList_ShowsMissingMaterials(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
		{ID: "bleach", Name: "Bleach", Category: "chemical"},
	}
	matReg := newMaterialRegistry(mats)
	recipes := []*crafting.Recipe{
		{
			ID:         "cleaner_bomb",
			Name:       "Cleaner Bomb",
			Category:   "chemical",
			Complexity: 1, // EffectiveMinRank = "untrained"
			DC:         10,
			Materials: []crafting.RecipeMaterial{
				{ID: "scrap_metal", Quantity: 1},
				{ID: "bleach", Quantity: 1},
			},
		},
	}
	recipeReg := newRecipeRegistry(recipes)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)
	svc.recipeReg = recipeReg
	svc.craftEngine = crafting.NewEngine()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills = map[string]string{"rigging": "trained"}
	sess.Materials = map[string]int{} // no materials

	evt, err := svc.handleCraftList(uid, &gamev1.CraftListRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event")
	assert.Contains(t, msg.Content, "[missing: 2]")
}

// TestHandleCraft_InsufficientMaterials_Fails verifies that when a player lacks required
// materials the craft command returns an error and does not set PendingCraftRecipeID.
//
// Precondition: player has 0 scrap_metal; recipe requires 2.
// Postcondition: error event contains "missing"; PendingCraftRecipeID remains empty.
func TestHandleCraft_InsufficientMaterials_Fails(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
	}
	matReg := newMaterialRegistry(mats)
	recipes := []*crafting.Recipe{
		{
			ID:         "pipe_bomb",
			Name:       "Pipe Bomb",
			Category:   "explosive",
			Complexity: 1,
			DC:         12,
			Materials:  []crafting.RecipeMaterial{{ID: "scrap_metal", Quantity: 2}},
		},
	}
	recipeReg := newRecipeRegistry(recipes)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)
	svc.recipeReg = recipeReg
	svc.craftEngine = crafting.NewEngine()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills = map[string]string{"rigging": "untrained"}
	sess.Materials = map[string]int{} // no scrap_metal

	evt, err := svc.handleCraft(uid, &gamev1.CraftRequest{RecipeId: "pipe_bomb"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected error event for missing materials")
	assert.Contains(t, strings.ToLower(errEvt.Message), "missing")
	assert.Empty(t, sess.PendingCraftRecipeID, "PendingCraftRecipeID must remain empty")
}

// TestHandleCraftConfirm_NoPending_Fails verifies that confirm with no pending recipe
// returns an appropriate error.
//
// Precondition: PendingCraftRecipeID == "".
// Postcondition: error event contains "no pending" or similar.
func TestHandleCraftConfirm_NoPending_Fails(t *testing.T) {
	svc, uid := buildCraftingServer(t, nil, &stubMaterialsRepo{}, nil)
	svc.recipeReg = newRecipeRegistry(nil)
	svc.craftEngine = crafting.NewEngine()

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingCraftRecipeID = ""

	evt, err := svc.handleCraftConfirm(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected error event for no pending craft")
	assert.Contains(t, strings.ToLower(errEvt.Message), "no pending")
}

// TestHandleCraftConfirm_QuickCraft_Success verifies that with all required materials and
// a sufficient rank, craft confirm deducts materials, clears the pending recipe, and
// returns a success message.
//
// Precondition: player has 2 scrap_metal; rigging rank = "trained" (meets complexity=1 recipe);
// dice roll is forced via nil dice (uses fallback roll=10, abilityMod=0 → total=10 vs DC=5 → Success).
// Postcondition: PendingCraftRecipeID is cleared; materials deducted; message contains craft result.
func TestHandleCraftConfirm_QuickCraft_Success(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
	}
	matReg := newMaterialRegistry(mats)
	recipes := []*crafting.Recipe{
		{
			ID:          "shiv",
			Name:        "Shiv",
			Category:    "weapon",
			Complexity:  1, // EffectiveMinRank = "untrained"
			DC:          5,
			OutputCount: 1,
			Materials:   []crafting.RecipeMaterial{{ID: "scrap_metal", Quantity: 2}},
		},
	}
	recipeReg := newRecipeRegistry(recipes)
	svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, nil)
	svc.recipeReg = recipeReg
	svc.craftEngine = crafting.NewEngine()
	// dice is nil — handleCraftConfirm must use a fixed roll when dice is nil

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Skills = map[string]string{"rigging": "trained"}
	sess.Materials = map[string]int{"scrap_metal": 2}
	sess.Abilities = character.AbilityScores{Savvy: 10} // abilityMod = 0
	sess.PendingCraftRecipeID = "shiv"

	evt, err := svc.handleCraftConfirm(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected message event on success")
	assert.Empty(t, sess.PendingCraftRecipeID, "pending recipe must be cleared after confirm")
	// Materials should be deducted (full deduction on success).
	assert.Equal(t, 0, sess.Materials["scrap_metal"], "scrap_metal must be deducted")
}

// TestHandleScavenge_Property_ExhaustedAfterFirstAttempt verifies via property testing
// that a second scavenge in the same room is always rejected.
//
// Precondition: any zone pool configuration.
// Postcondition: second scavenge in same room always returns error event.
func TestHandleScavenge_Property_ExhaustedAfterFirstAttempt(t *testing.T) {
	mats := []*crafting.Material{
		{ID: "scrap_metal", Name: "Scrap Metal", Category: "mechanical"},
	}
	matReg := newMaterialRegistry(mats)
	pool := &world.MaterialPool{
		DC: 1, // very low DC so first attempt always succeeds on skill check
		Drops: []world.MaterialPoolDrop{
			{ID: "scrap_metal", Weight: 1},
		},
	}

	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := buildCraftingServer(t, matReg, &stubMaterialsRepo{}, pool)
		sess, ok := svc.sessions.GetPlayer(uid)
		require.True(rt, ok)
		sess.Materials = make(map[string]int)

		// First attempt — may succeed or fail skill check, but should NOT return error event
		// (room was not exhausted yet).
		evt1, err1 := svc.handleScavenge(uid)
		assert.NoError(rt, err1)
		assert.NotNil(rt, evt1)
		if errEvt := evt1.GetError(); errEvt != nil {
			// If error, the room was exhausted before, which cannot happen on first call.
			rt.Fatalf("first scavenge returned error: %s", errEvt.Message)
		}

		// Second attempt — room is now exhausted, must return error event.
		evt2, err2 := svc.handleScavenge(uid)
		assert.NoError(rt, err2)
		assert.NotNil(rt, evt2)
		errEvt2 := evt2.GetError()
		if errEvt2 == nil {
			rt.Fatalf("second scavenge in same room did not return error event")
		}
	})
}
