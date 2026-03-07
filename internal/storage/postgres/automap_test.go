package postgres_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// setupAutomapRepos creates a pool, account, and character for use in automap tests.
// Returns the AutomapRepository and the ID of a persisted test character.
func setupAutomapRepos(t *testing.T) (*postgres.AutomapRepository, int64) {
	t.Helper()
	acctRepo := postgres.NewAccountRepository(sharedPool)
	acct, err := acctRepo.Create(context.Background(), uniqueName("automap_user"), "password123")
	require.NoError(t, err)
	charRepo := postgres.NewCharacterRepository(sharedPool)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(acct.ID, uniqueName("AutomapChar")))
	require.NoError(t, err)
	return postgres.NewAutomapRepository(sharedPool), char.ID
}

func TestAutomapRepository_Insert_And_LoadAll(t *testing.T) {
	repo, charID := setupAutomapRepos(t)
	ctx := context.Background()

	err := repo.Insert(ctx, charID, "downtown", "pioneer_square")
	require.NoError(t, err)

	// Duplicate insert is idempotent.
	err = repo.Insert(ctx, charID, "downtown", "pioneer_square")
	require.NoError(t, err)

	discovered, err := repo.LoadAll(ctx, charID)
	require.NoError(t, err)
	require.True(t, discovered["downtown"]["pioneer_square"])
}

func TestAutomapRepository_BulkInsert(t *testing.T) {
	repo, charID := setupAutomapRepos(t)
	ctx := context.Background()

	roomIDs := []string{"room_a", "room_b", "room_c"}
	err := repo.BulkInsert(ctx, charID, "eastside", roomIDs)
	require.NoError(t, err)

	discovered, err := repo.LoadAll(ctx, charID)
	require.NoError(t, err)
	assert.True(t, discovered["eastside"]["room_a"])
	assert.True(t, discovered["eastside"]["room_b"])
	assert.True(t, discovered["eastside"]["room_c"])
	assert.Len(t, discovered["eastside"], 3)
}

func TestAutomapRepository_BulkInsert_EmptyRoomIDs_IsNoOp(t *testing.T) {
	repo, charID := setupAutomapRepos(t)
	ctx := context.Background()

	err := repo.BulkInsert(ctx, charID, "downtown", []string{})
	require.NoError(t, err)

	discovered, err := repo.LoadAll(ctx, charID)
	require.NoError(t, err)
	require.NotNil(t, discovered)
	assert.Empty(t, discovered)
}

func TestAutomapRepository_LoadAll_Empty(t *testing.T) {
	repo := postgres.NewAutomapRepository(sharedPool)
	ctx := context.Background()

	// Use a character ID that does not exist — LoadAll must return non-nil empty map.
	discovered, err := repo.LoadAll(ctx, 99999)
	require.NoError(t, err)
	require.NotNil(t, discovered)
	assert.Empty(t, discovered)
}

func TestAutomapRepository_LoadAll_MultipleZones(t *testing.T) {
	repo, charID := setupAutomapRepos(t)
	ctx := context.Background()

	require.NoError(t, repo.Insert(ctx, charID, "zone_a", "room_1"))
	require.NoError(t, repo.Insert(ctx, charID, "zone_b", "room_2"))
	require.NoError(t, repo.Insert(ctx, charID, "zone_b", "room_3"))

	discovered, err := repo.LoadAll(ctx, charID)
	require.NoError(t, err)
	assert.True(t, discovered["zone_a"]["room_1"])
	assert.True(t, discovered["zone_b"]["room_2"])
	assert.True(t, discovered["zone_b"]["room_3"])
	assert.Len(t, discovered, 2)
	assert.Len(t, discovered["zone_b"], 2)
}

// TestAutomapRepository_Property_InsertThenLoad verifies that for any valid
// (zoneID, roomID) pair, Insert followed by LoadAll always returns the room as discovered.
func TestAutomapRepository_Property_InsertThenLoad(t *testing.T) {
	acctRepo := postgres.NewAccountRepository(sharedPool)
	charRepo := postgres.NewCharacterRepository(sharedPool)
	repo := postgres.NewAutomapRepository(sharedPool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("prop_user"), "pass")
		require.NoError(t, err)
		char, err := charRepo.Create(ctx, makeTestCharacter(acct.ID, uniqueName("PropChar")))
		require.NoError(t, err)

		zoneID := rapid.StringMatching(`[a-z][a-z0-9_]{1,10}`).Draw(rt, "zoneID")
		roomID := rapid.StringMatching(`[a-z][a-z0-9_]{1,10}`).Draw(rt, "roomID")

		err = repo.Insert(ctx, char.ID, zoneID, roomID)
		require.NoError(t, err)

		discovered, err := repo.LoadAll(ctx, char.ID)
		require.NoError(t, err)
		assert.True(t, discovered[zoneID][roomID])
	})
}
