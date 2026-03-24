package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// newMaterialsRepo creates a CharacterMaterialsRepository using the shared test pool.
func newMaterialsRepo() *pgstore.CharacterMaterialsRepository {
	return pgstore.NewCharacterMaterialsRepository(sharedPool)
}

// TestCharacterMaterials_LoadEmpty verifies that Load returns an empty (non-nil) map
// for a character with no materials.
func TestCharacterMaterials_LoadEmpty(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_empty")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	got, err := repo.Load(context.Background(), char.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// TestCharacterMaterials_AddAndLoad verifies that Add followed by Load returns the
// correct quantity.
func TestCharacterMaterials_AddAndLoad(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_add")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, char.ID, "iron_ore", 5))

	got, err := repo.Load(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, got["iron_ore"])
}

// TestCharacterMaterials_AddAccumulates verifies that repeated Add calls sum quantities.
func TestCharacterMaterials_AddAccumulates(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_accum")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, char.ID, "iron_ore", 3))
	require.NoError(t, repo.Add(ctx, char.ID, "iron_ore", 7))

	got, err := repo.Load(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, got["iron_ore"])
}

// TestCharacterMaterials_DeductMany_RemovesRowAtZero verifies that DeductMany deletes
// a row when the resulting quantity equals zero.
func TestCharacterMaterials_DeductMany_RemovesRowAtZero(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_zero")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, char.ID, "copper_wire", 4))
	require.NoError(t, repo.DeductMany(ctx, char.ID, map[string]int{"copper_wire": 4}))

	got, err := repo.Load(ctx, char.ID)
	require.NoError(t, err)
	_, exists := got["copper_wire"]
	assert.False(t, exists, "row must be deleted when quantity reaches zero")
}

// TestCharacterMaterials_DeductMany_InsufficientRollback verifies that DeductMany
// returns an error and leaves all quantities unchanged when one material has
// insufficient quantity (REQ-CRAFT-12).
func TestCharacterMaterials_DeductMany_InsufficientRollback(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_insuf")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, char.ID, "steel_plate", 2))
	require.NoError(t, repo.Add(ctx, char.ID, "copper_wire", 10))

	err = repo.DeductMany(ctx, char.ID, map[string]int{
		"steel_plate": 5,  // insufficient
		"copper_wire": 3,
	})
	require.Error(t, err, "DeductMany must return an error on insufficient quantity")

	// Quantities must be unchanged due to transaction rollback.
	got, loadErr := repo.Load(ctx, char.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, 2, got["steel_plate"], "steel_plate quantity must be unchanged after rollback")
	assert.Equal(t, 10, got["copper_wire"], "copper_wire quantity must be unchanged after rollback")
}

// TestCharacterMaterials_Property verifies that Add/Load preserves quantity across
// arbitrary material IDs and amounts.
func TestCharacterMaterials_Property(t *testing.T) {
	charRepo, accountID := setupCharRepos(t)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(accountID, uniqueName("mat_prop")))
	require.NoError(t, err)

	repo := newMaterialsRepo()
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		matID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "materialID")
		qty := rapid.IntRange(1, 100).Draw(rt, "quantity")

		// Reset existing quantity for this material to get a clean baseline.
		existing, loadErr := repo.Load(ctx, char.ID)
		require.NoError(rt, loadErr)
		if cur, ok := existing[matID]; ok {
			require.NoError(rt, repo.DeductMany(ctx, char.ID, map[string]int{matID: cur}))
		}

		require.NoError(rt, repo.Add(ctx, char.ID, matID, qty))
		got, loadErr := repo.Load(ctx, char.ID)
		require.NoError(rt, loadErr)
		assert.Equal(rt, qty, got[matID])
	})
}
