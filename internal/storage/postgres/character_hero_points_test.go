package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSaveAndLoadHeroPoints verifies round-trip persistence of hero points.
func TestSaveAndLoadHeroPoints(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("hero_rt")))
	require.NoError(t, err)

	err = repo.SaveHeroPoints(ctx, char.ID, 3)
	require.NoError(t, err)

	got, err := repo.LoadHeroPoints(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, got, "loaded hero points must equal saved value")
}

// TestLoadHeroPoints_DefaultZero verifies that a character without a persisted
// hero point count returns 0.
func TestLoadHeroPoints_DefaultZero(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("hero_def")))
	require.NoError(t, err)

	got, err := repo.LoadHeroPoints(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got)
}

// TestSaveHeroPoints_InvalidID verifies that id <= 0 returns an error.
func TestSaveHeroPoints_InvalidID(t *testing.T) {
	repo, _ := setupCharRepos(t)
	err := repo.SaveHeroPoints(context.Background(), 0, 1)
	require.Error(t, err)
}

// TestLoadHeroPoints_InvalidID verifies that id <= 0 returns an error.
func TestLoadHeroPoints_InvalidID(t *testing.T) {
	repo, _ := setupCharRepos(t)
	_, err := repo.LoadHeroPoints(context.Background(), 0)
	require.Error(t, err)
}

// TestSaveAndLoadHeroPoints_Property verifies that any non-negative hero point
// value survives a save/load round-trip.
func TestSaveAndLoadHeroPoints_Property(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("hero_prop")))
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		pts := rapid.IntRange(0, 100).Draw(rt, "heroPoints")
		require.NoError(rt, repo.SaveHeroPoints(ctx, char.ID, pts))
		got, loadErr := repo.LoadHeroPoints(ctx, char.ID)
		require.NoError(rt, loadErr)
		assert.Equal(rt, pts, got)
	})
}
