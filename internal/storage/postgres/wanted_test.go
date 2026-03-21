package postgres_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// setupWantedRepos creates an account and character for use in wanted tests.
// Returns the WantedRepository and the ID of a persisted test character.
func setupWantedRepos(t *testing.T) (*postgres.WantedRepository, int64) {
	t.Helper()
	acctRepo := postgres.NewAccountRepository(sharedPool)
	acct, err := acctRepo.Create(context.Background(), uniqueName("wanted_user"), "password123")
	require.NoError(t, err)
	charRepo := postgres.NewCharacterRepository(sharedPool)
	char, err := charRepo.Create(context.Background(), makeTestCharacter(acct.ID, uniqueName("WantedChar")))
	require.NoError(t, err)
	return postgres.NewWantedRepository(sharedPool), char.ID
}

func TestWantedRepository_Load_Empty(t *testing.T) {
	repo := postgres.NewWantedRepository(sharedPool)
	ctx := context.Background()

	// Use a character ID that does not exist — Load must return non-nil empty map.
	levels, err := repo.Load(ctx, 99999)
	require.NoError(t, err)
	require.NotNil(t, levels)
	require.Empty(t, levels)
}

func TestWantedRepository_Upsert_And_Load(t *testing.T) {
	repo, charID := setupWantedRepos(t)
	ctx := context.Background()

	const zoneID = "test_zone_wanted"

	// Clean state: ensure no row exists.
	require.NoError(t, repo.Upsert(ctx, charID, zoneID, 0))

	// Load returns empty map initially.
	levels, err := repo.Load(ctx, charID)
	require.NoError(t, err)
	require.Equal(t, 0, levels[zoneID])

	// Upsert level=2 stores it.
	require.NoError(t, repo.Upsert(ctx, charID, zoneID, 2))
	levels, err = repo.Load(ctx, charID)
	require.NoError(t, err)
	require.Equal(t, 2, levels[zoneID])

	// Upsert level=3 updates it.
	require.NoError(t, repo.Upsert(ctx, charID, zoneID, 3))
	levels, err = repo.Load(ctx, charID)
	require.NoError(t, err)
	require.Equal(t, 3, levels[zoneID])

	// Upsert level=0 deletes the row.
	require.NoError(t, repo.Upsert(ctx, charID, zoneID, 0))
	levels, err = repo.Load(ctx, charID)
	require.NoError(t, err)
	require.Equal(t, 0, levels[zoneID])
}

func TestWantedRepository_MultipleZones(t *testing.T) {
	repo, charID := setupWantedRepos(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, charID, "zone_a", 1))
	require.NoError(t, repo.Upsert(ctx, charID, "zone_b", 4))

	levels, err := repo.Load(ctx, charID)
	require.NoError(t, err)
	require.Equal(t, 1, levels["zone_a"])
	require.Equal(t, 4, levels["zone_b"])
}

// TestWantedRepository_Property_UpsertThenLoad verifies that for any valid
// (zoneID, level) pair, Upsert followed by Load always returns the stored level.
func TestWantedRepository_Property_UpsertThenLoad(t *testing.T) {
	acctRepo := postgres.NewAccountRepository(sharedPool)
	charRepo := postgres.NewCharacterRepository(sharedPool)
	repo := postgres.NewWantedRepository(sharedPool)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("prop_wanted_user"), "pass")
		require.NoError(t, err)
		char, err := charRepo.Create(ctx, makeTestCharacter(acct.ID, uniqueName("PropWantedChar")))
		require.NoError(t, err)

		zoneID := rapid.StringMatching(`[a-z][a-z0-9_]{1,10}`).Draw(rt, "zoneID")
		level := rapid.IntRange(1, 4).Draw(rt, "level")

		err = repo.Upsert(ctx, char.ID, zoneID, level)
		require.NoError(t, err)

		levels, err := repo.Load(ctx, char.ID)
		require.NoError(t, err)
		require.Equal(t, level, levels[zoneID])
	})
}
