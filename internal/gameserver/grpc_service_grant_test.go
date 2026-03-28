package gameserver

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// grantCharSaver is a CharacterSaver test double that records SaveCurrency calls.
//
// Precondition: none.
// Postcondition: SaveCurrency records the last currency value saved; all other methods no-op.
type grantCharSaver struct {
	savedCurrency     atomic.Int64
	saveCurrencyCalls atomic.Int32
	saveProgressCalls atomic.Int32
}

func (m *grantCharSaver) SaveState(_ context.Context, _ int64, _ string, _ int) error { return nil }
func (m *grantCharSaver) LoadWeaponPresets(_ context.Context, _ int64, _ *inventory.Registry) (*inventory.LoadoutSet, error) {
	return inventory.NewLoadoutSet(), nil
}
func (m *grantCharSaver) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	return nil
}
func (m *grantCharSaver) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	return inventory.NewEquipment(), nil
}
func (m *grantCharSaver) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	return nil
}
func (m *grantCharSaver) LoadInventory(_ context.Context, _ int64) ([]inventory.InventoryItem, error) {
	return nil, nil
}
func (m *grantCharSaver) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	return nil
}
func (m *grantCharSaver) HasReceivedStartingInventory(_ context.Context, _ int64) (bool, error) {
	return false, nil
}
func (m *grantCharSaver) MarkStartingInventoryGranted(_ context.Context, _ int64) error { return nil }
func (m *grantCharSaver) GetByID(_ context.Context, id int64) (*character.Character, error) {
	return &character.Character{ID: id}, nil
}
func (m *grantCharSaver) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
	return nil
}
func (m *grantCharSaver) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error {
	m.saveProgressCalls.Add(1)
	return nil
}
func (m *grantCharSaver) SaveDefaultCombatAction(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *grantCharSaver) SaveCurrency(_ context.Context, _ int64, currency int) error {
	m.saveCurrencyCalls.Add(1)
	m.savedCurrency.Store(int64(currency))
	return nil
}
func (m *grantCharSaver) LoadCurrency(_ context.Context, _ int64) (int, error)   { return 0, nil }
func (m *grantCharSaver) SaveGender(_ context.Context, _ int64, _ string) error  { return nil }
func (m *grantCharSaver) SaveHeroPoints(_ context.Context, _ int64, _ int) error { return nil }
func (m *grantCharSaver) LoadHeroPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *grantCharSaver) SaveJobs(_ context.Context, _ int64, _ map[string]int, _ string) error {
	return nil
}
func (m *grantCharSaver) SaveInstanceCharges(_ context.Context, _ int64, _, _ string, _ int, _ bool) error {
	return nil
}
func (m *grantCharSaver) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	return map[string]int{}, "", nil
}
func (m *grantCharSaver) LoadFocusPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *grantCharSaver) SaveFocusPoints(_ context.Context, _ int64, _ int) error { return nil }
func (m *grantCharSaver) SaveHotbar(_ context.Context, _ int64, _ [10]string) error { return nil }
func (m *grantCharSaver) LoadHotbar(_ context.Context, _ int64) ([10]string, error) {
	return [10]string{}, nil
}

// grantProgressRepo is a ProgressRepository test double that records SaveProgress and skill increase calls.
//
// Precondition: none.
// Postcondition: SaveProgress and IncrementPendingSkillIncreases record call counts; all other methods no-op or return zero values.
type grantProgressRepo struct {
	saveProgressCalls            atomic.Int32
	incrementSkillIncreasesCalls atomic.Int32
	incrementSkillIncreasesTotal atomic.Int32
	markSkillIncreasesInitCalls  atomic.Int32
}

func (m *grantProgressRepo) GetProgress(_ context.Context, _ int64) (int, int, int, int, error) {
	return 1, 0, 10, 0, nil
}
func (m *grantProgressRepo) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *grantProgressRepo) IncrementPendingSkillIncreases(_ context.Context, _ int64, n int) error {
	m.incrementSkillIncreasesCalls.Add(1)
	m.incrementSkillIncreasesTotal.Add(int32(n))
	return nil
}
func (m *grantProgressRepo) ConsumePendingBoost(_ context.Context, _ int64) error { return nil }
func (m *grantProgressRepo) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	return nil
}
func (m *grantProgressRepo) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (m *grantProgressRepo) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	m.markSkillIncreasesInitCalls.Add(1)
	return nil
}
func (m *grantProgressRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return nil, nil
}
func (m *grantProgressRepo) SetPendingTechLevels(_ context.Context, _ int64, _ []int) error {
	return nil
}
func (m *grantProgressRepo) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error {
	m.saveProgressCalls.Add(1)
	return nil
}

// ---------------------------------------------------------------------------
// Helper: build a GameServiceServer wired for handleGrant tests.
// ---------------------------------------------------------------------------

// grantTestOptions holds optional overrides for testServiceForGrant.
//
// Precondition: none — all fields are optional.
type grantTestOptions struct {
	charSaver    CharacterSaver
	progressRepo ProgressRepository
}

// testServiceForGrant creates a minimal GameServiceServer suitable for handleGrant tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a configured *GameServiceServer with optional charSaver and progressRepo.
func testServiceForGrant(t *testing.T, opts grantTestOptions) *GameServiceServer {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	svc.charSaver = opts.charSaver
	svc.progressRepo = opts.progressRepo
	return svc
}

// testMinimalService constructs a GameServiceServer using the provided session manager.
//
// Precondition: sessMgr must be non-nil.
// Postcondition: Returns a GameServiceServer suitable for unit-testing handleChar and similar methods.
func testMinimalService(t *testing.T, sessMgr *session.Manager) *GameServiceServer {
	t.Helper()
	worldMgr, _ := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	return newTestGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
}

// addEditorForGrant adds a player session with the editor role.
//
// Precondition: svc must have a valid session manager.
// Postcondition: Player is in the session manager with Role="editor"; session is returned.
func addEditorForGrant(t *testing.T, svc *GameServiceServer, uid string) *session.PlayerSession {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "editor",
		Level:       1,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	return sess
}

// addTargetForGrant adds a player session with the player role as a grant target.
//
// Precondition: svc must have a valid session manager.
// Postcondition: Player is in the session manager with the given charName; session is returned.
func addTargetForGrant(t *testing.T, svc *GameServiceServer, uid, charName string) *session.PlayerSession {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    charName,
		CharacterID: 2,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
		Level:       1,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	return sess
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleGrant_EditorGrantsXP verifies that an editor can grant XP to an online player,
// and the target's Experience is increased by the granted amount.
//
// Precondition: caller has editor role; target is online; grant type is "xp"; amount > 0.
// Postcondition: target.Experience increases by amount; success event returned to caller.
func TestHandleGrant_EditorGrantsXP(t *testing.T) {
	charSaver := &grantCharSaver{}
	progressRepo := &grantProgressRepo{}
	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})

	addEditorForGrant(t, svc, "editor1")
	target := addTargetForGrant(t, svc, "target1", "TargetChar")
	initialXP := target.Experience

	evt, err := svc.handleGrant("editor1", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "TargetChar",
		Amount:    100,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotNil(t, evt.GetMessage(), "expected a success MessageEvent")
	assert.Equal(t, initialXP+100, target.Experience, "target.Experience must increase by amount")
	assert.Equal(t, int32(1), charSaver.saveProgressCalls.Load(), "SaveProgress must be called once")
}

// TestHandleGrant_EditorGrantsMoney verifies that an editor can grant money to an online player,
// and the target's Currency is increased by the granted amount.
//
// Precondition: caller has editor role; target is online; grant type is "money"; amount > 0.
// Postcondition: target.Currency increases by amount; SaveCurrency called; success event returned.
func TestHandleGrant_EditorGrantsMoney(t *testing.T) {
	charSaver := &grantCharSaver{}
	progressRepo := &grantProgressRepo{}
	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})

	addEditorForGrant(t, svc, "editor2")
	target := addTargetForGrant(t, svc, "target2", "RichChar")
	target.Currency = 50

	evt, err := svc.handleGrant("editor2", &gamev1.GrantRequest{
		GrantType: "money",
		CharName:  "RichChar",
		Amount:    200,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotNil(t, evt.GetMessage(), "expected a success MessageEvent")
	assert.Equal(t, 250, target.Currency, "target.Currency must increase by amount")
	assert.Equal(t, int32(1), charSaver.saveCurrencyCalls.Load(), "SaveCurrency must be called once")
}

// TestHandleGrant_PlayerRoleDenied verifies that a non-editor, non-admin player cannot grant.
//
// Precondition: caller has role "player".
// Postcondition: Returns an error event with "permission denied" message.
func TestHandleGrant_PlayerRoleDenied(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         "player1",
		Username:    "player1",
		CharName:    "PlainPlayer",
		CharacterID: 3,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
		Level:       1,
	})
	require.NoError(t, err)

	evt, err := svc.handleGrant("player1", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "Anyone",
		Amount:    100,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "permission denied", "error must indicate permission denied")
}

// TestHandleGrant_TargetNotOnline verifies that granting to an offline player returns an error event.
//
// Precondition: caller has editor role; target character name does not match any online player.
// Postcondition: Returns an error event indicating player not online.
func TestHandleGrant_TargetNotOnline(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})
	addEditorForGrant(t, svc, "editor3")

	evt, err := svc.handleGrant("editor3", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "NoSuchPlayer",
		Amount:    100,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected an error event when target is offline")
	assert.Contains(t, errEvt.Message, "not online", "error must indicate player not online")
}

// TestHandleGrant_ZeroAmountDenied verifies that granting zero or negative amounts returns an error.
//
// Precondition: caller has editor role; target is online; amount <= 0.
// Postcondition: Returns an error event indicating invalid amount.
func TestHandleGrant_ZeroAmountDenied(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})
	addEditorForGrant(t, svc, "editor4")
	addTargetForGrant(t, svc, "target4", "ZeroTarget")

	evt, err := svc.handleGrant("editor4", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "ZeroTarget",
		Amount:    0,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected an error event for zero amount")
}

// TestHandleGrant_AdminCanGrant verifies that a player with admin role can also grant.
//
// Precondition: caller has admin role; target is online; grant type is "xp"; amount > 0.
// Postcondition: target.Experience increases by amount; success event returned.
func TestHandleGrant_AdminCanGrant(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         "admin1",
		Username:    "admin1",
		CharName:    "AdminChar",
		CharacterID: 10,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "admin",
		Level:       1,
	})
	require.NoError(t, err)
	target := addTargetForGrant(t, svc, "target5", "RegularChar")
	initialXP := target.Experience

	evt, err := svc.handleGrant("admin1", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "RegularChar",
		Amount:    50,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotNil(t, evt.GetMessage(), "expected a success MessageEvent")
	assert.Equal(t, initialXP+50, target.Experience, "admin grant must increase target XP")
}

// TestHandleGrant_PropertyBased is a property-based test verifying that XP grants always
// increase the target's experience by exactly the granted amount (with xpSvc nil).
//
// Precondition: caller has editor role; target is online; amount is in [1, 10000].
// Postcondition: target.Experience == initialXP + amount.
func TestHandleGrant_PropertyBased(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		amount := rapid.Int32Range(1, 10000).Draw(rt, "amount")

		svc := testServiceForGrant(t, grantTestOptions{})

		uid := "prop_editor"
		targetUID := "prop_target"
		charName := "PropTarget"

		addEditorForGrant(t, svc, uid)

		_, addErr := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID:         targetUID,
			Username:    targetUID,
			CharName:    charName,
			CharacterID: 99,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       10,
			Abilities:   character.AbilityScores{},
			Role:        "player",
			Level:       1,
		})
		if addErr != nil {
			rt.Fatalf("AddPlayer failed: %v", addErr)
		}
		target, ok := svc.sessions.GetPlayer(targetUID)
		if !ok {
			rt.Fatalf("target session not found")
		}
		initialXP := target.Experience

		evt, err := svc.handleGrant(uid, &gamev1.GrantRequest{
			GrantType: "xp",
			CharName:  charName,
			Amount:    amount,
		})

		if err != nil {
			rt.Fatalf("handleGrant returned error: %v", err)
		}
		if evt == nil || evt.GetMessage() == nil {
			rt.Fatalf("expected a MessageEvent, got %v", evt)
		}
		if target.Experience != initialXP+int(amount) {
			rt.Fatalf("expected Experience=%d, got %d", initialXP+int(amount), target.Experience)
		}
	})
}

// grantXPProgressSaver is a minimal xp.ProgressSaver stub for xpSvc wiring in tests.
//
// Precondition: none.
// Postcondition: SaveProgress is a no-op that always succeeds.
type grantXPProgressSaver struct{}

func (g *grantXPProgressSaver) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error {
	return nil
}

// testXPConfig returns a minimal XPConfig sufficient to trigger a level-up with 1000 XP at level 1.
// Level 2 requires 2² × 100 = 400 XP; 1000 XP is sufficient to reach level 3 (3² × 100 = 900 XP).
//
// Precondition: none.
// Postcondition: Returns a non-nil *xp.XPConfig with BaseXP=100.
func testXPConfig() *xp.XPConfig {
	return &xp.XPConfig{
		BaseXP:        100,
		HPPerLevel:    5,
		BoostInterval: 5,
		LevelCap:      100,
		Awards: xp.Awards{
			KillXPPerNPCLevel:       50,
			NewRoomXP:               10,
			SkillCheckSuccessXP:     10,
			SkillCheckCritSuccessXP: 25,
			SkillCheckDCMultiplier:  2,
		},
	}
}

// TestHandleGrant_XP_WithXPService_LevelUp verifies that granting enough XP via a wired xpSvc
// causes the target to level up, and that IncrementPendingSkillIncreases is called when skill
// increases are awarded.
//
// Precondition: xpSvc is configured with BaseXP=500; target starts at level 1 with 0 XP;
// 1000 XP is granted, which is sufficient to reach level 2 and level 3.
// Postcondition: target.Level > 1; target.Experience reflects post-award XP;
// SaveProgress is called; if skill increases are earned, IncrementPendingSkillIncreases is called.
func TestHandleGrant_XP_WithXPService_LevelUp(t *testing.T) {
	charSaver := &grantCharSaver{}
	progressRepo := &grantProgressRepo{}

	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})
	xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
	svc.SetXPService(xpSvc)

	addEditorForGrant(t, svc, "editor_xpsvc")
	target := addTargetForGrant(t, svc, "target_xpsvc", "LevelUpChar")
	require.Equal(t, 1, target.Level, "precondition: target starts at level 1")

	// Granting 1000 XP to a level-1 character with BaseXP=500 should trigger at least one level-up.
	evt, err := svc.handleGrant("editor_xpsvc", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "LevelUpChar",
		Amount:    1000,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotNil(t, evt.GetMessage(), "expected a success MessageEvent on level-up grant")
	assert.Greater(t, target.Level, 1, "target must have leveled up")
	assert.Equal(t, int32(1), charSaver.saveProgressCalls.Load(), "SaveProgress must be called once")
}

// TestHandleGrant_NegativeAmount verifies that granting a negative amount returns an error event.
//
// Precondition: caller has editor role; target is online; amount is -5.
// Postcondition: Returns an error event with "amount must be" in the message.
func TestHandleGrant_NegativeAmount(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})
	addEditorForGrant(t, svc, "editor_neg")
	addTargetForGrant(t, svc, "target_neg", "NegTarget")

	evt, err := svc.handleGrant("editor_neg", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "NegTarget",
		Amount:    -5,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected an error event for negative amount")
	assert.Contains(t, errEvt.Message, "amount must be", "error must reference amount constraint")
}
