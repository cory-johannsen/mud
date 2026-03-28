package gameserver

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// heroPointCharSaver is a CharacterSaver test double that records SaveHeroPoints calls.
//
// Precondition: none.
// Postcondition: SaveHeroPoints records call count and last saved value; all other methods no-op.
type heroPointCharSaver struct {
	saveHeroPointsCalls atomic.Int32
	lastHeroPoints      atomic.Int32
	saveProgressCalls   atomic.Int32
}

func (m *heroPointCharSaver) SaveState(_ context.Context, _ int64, _ string, _ int) error {
	return nil
}
func (m *heroPointCharSaver) LoadWeaponPresets(_ context.Context, _ int64, _ *inventory.Registry) (*inventory.LoadoutSet, error) {
	return inventory.NewLoadoutSet(), nil
}
func (m *heroPointCharSaver) SaveWeaponPresets(_ context.Context, _ int64, _ *inventory.LoadoutSet) error {
	return nil
}
func (m *heroPointCharSaver) LoadEquipment(_ context.Context, _ int64) (*inventory.Equipment, error) {
	return inventory.NewEquipment(), nil
}
func (m *heroPointCharSaver) SaveEquipment(_ context.Context, _ int64, _ *inventory.Equipment) error {
	return nil
}
func (m *heroPointCharSaver) LoadInventory(_ context.Context, _ int64) ([]inventory.InventoryItem, error) {
	return nil, nil
}
func (m *heroPointCharSaver) SaveInventory(_ context.Context, _ int64, _ []inventory.InventoryItem) error {
	return nil
}
func (m *heroPointCharSaver) HasReceivedStartingInventory(_ context.Context, _ int64) (bool, error) {
	return false, nil
}
func (m *heroPointCharSaver) MarkStartingInventoryGranted(_ context.Context, _ int64) error {
	return nil
}
func (m *heroPointCharSaver) GetByID(_ context.Context, id int64) (*character.Character, error) {
	return &character.Character{ID: id}, nil
}
func (m *heroPointCharSaver) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
	return nil
}
func (m *heroPointCharSaver) SaveProgress(_ context.Context, _ int64, _, _, _, _ int) error {
	m.saveProgressCalls.Add(1)
	return nil
}
func (m *heroPointCharSaver) SaveDefaultCombatAction(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *heroPointCharSaver) SaveCurrency(_ context.Context, _ int64, _ int) error { return nil }
func (m *heroPointCharSaver) LoadCurrency(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *heroPointCharSaver) SaveGender(_ context.Context, _ int64, _ string) error { return nil }
func (m *heroPointCharSaver) SaveHeroPoints(_ context.Context, _ int64, hp int) error {
	m.saveHeroPointsCalls.Add(1)
	m.lastHeroPoints.Store(int32(hp))
	return nil
}
func (m *heroPointCharSaver) LoadHeroPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *heroPointCharSaver) SaveJobs(_ context.Context, _ int64, _ map[string]int, _ string) error {
	return nil
}
func (m *heroPointCharSaver) SaveInstanceCharges(_ context.Context, _ int64, _, _ string, _ int, _ bool) error {
	return nil
}
func (m *heroPointCharSaver) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	return map[string]int{}, "", nil
}
func (m *heroPointCharSaver) LoadFocusPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (m *heroPointCharSaver) SaveFocusPoints(_ context.Context, _ int64, _ int) error { return nil }
func (m *heroPointCharSaver) SaveHotbar(_ context.Context, _ int64, _ [10]string) error { return nil }
func (m *heroPointCharSaver) LoadHotbar(_ context.Context, _ int64) ([10]string, error) {
	return [10]string{}, nil
}

// TestHandleGrant_XP_LevelUp_AwardsHeroPoint verifies that when granting XP causes
// a level-up, exactly 1 hero point is awarded to the target and SaveHeroPoints is called.
//
// Precondition: target is at level 1 with 0 XP; granting sufficient XP causes level-up.
// Postcondition: target.HeroPoints == 1; SaveHeroPoints called at least once.
func TestHandleGrant_XP_LevelUp_AwardsHeroPoint(t *testing.T) {
	charSaver := &heroPointCharSaver{}
	progressRepo := &grantProgressRepo{}

	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})
	xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
	svc.SetXPService(xpSvc)

	addEditorForGrant(t, svc, "editor_hp_levelup")
	target := addTargetForGrant(t, svc, "target_hp_levelup", "HeroPointLevelUpChar")
	require.Equal(t, 1, target.Level, "precondition: target starts at level 1")
	require.Equal(t, 0, target.HeroPoints, "precondition: target starts with 0 hero points")

	// Grant 1000 XP — enough to trigger at least one level-up (level 2 requires 400 XP with BaseXP=100).
	evt, err := svc.handleGrant("editor_hp_levelup", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "HeroPointLevelUpChar",
		Amount:    1000,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Greater(t, target.Level, 1, "target must have leveled up")
	assert.GreaterOrEqual(t, target.HeroPoints, 1, "target must have at least 1 hero point after level-up")
	assert.GreaterOrEqual(t, charSaver.saveHeroPointsCalls.Load(), int32(1), "SaveHeroPoints must be called at least once")
}

// TestHandleGrant_XP_LevelUp_AwardsHeroPoint_Property is a property-based test verifying that
// for any XP amount that triggers a level-up, HeroPoints increases and SaveHeroPoints is called.
//
// Precondition: xpSvc is configured with BaseXP=100; target starts at level 1 with 0 XP.
// Postcondition: for any amount >= 400 (level 2 threshold), target.HeroPoints >= 1 and SaveHeroPoints called.
func TestHandleGrant_XP_LevelUp_AwardsHeroPoint_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Draw an XP amount large enough to guarantee a level-up (level 2 requires BaseXP*4 = 400).
		amount := rapid.IntRange(400, 5000).Draw(rt, "xp_amount")

		charSaver := &heroPointCharSaver{}
		progressRepo := &grantProgressRepo{}

		svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})
		xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
		svc.SetXPService(xpSvc)

		editorUID := "editor_prop_hp"
		targetUID := "target_prop_hp"
		addEditorForGrant(t, svc, editorUID)
		target := addTargetForGrant(t, svc, targetUID, "PropHeroPointChar")

		evt, err := svc.handleGrant(editorUID, &gamev1.GrantRequest{
			GrantType: "xp",
			CharName:  "PropHeroPointChar",
			Amount:    int32(amount),
		})

		require.NoError(rt, err)
		require.NotNil(rt, evt)
		assert.Greater(rt, target.Level, 1, "must level up with %d XP", amount)
		assert.GreaterOrEqual(rt, target.HeroPoints, 1, "must award hero points on level-up")
		assert.GreaterOrEqual(rt, charSaver.saveHeroPointsCalls.Load(), int32(1), "SaveHeroPoints must be called")
	})
}
