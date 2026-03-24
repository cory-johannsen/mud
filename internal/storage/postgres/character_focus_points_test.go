package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSaveFocusPoints verifies round-trip persistence of focus points.
func TestSaveFocusPoints(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("fp_rt")))
	require.NoError(t, err)

	err = repo.SaveFocusPoints(ctx, char.ID, 2)
	require.NoError(t, err)

	got, err := repo.LoadFocusPoints(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got, "loaded focus points must equal saved value")
}

// TestLoadFocusPoints_DefaultZero verifies that a newly created character returns 0 focus points.
func TestLoadFocusPoints_DefaultZero(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("fp_def")))
	require.NoError(t, err)

	got, err := repo.LoadFocusPoints(ctx, char.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got)
}

// TestSaveFocusPoints_InvalidID verifies that id <= 0 returns an error.
func TestSaveFocusPoints_InvalidID(t *testing.T) {
	repo, _ := setupCharRepos(t)
	err := repo.SaveFocusPoints(context.Background(), 0, 1)
	require.Error(t, err)
}

// TestLoadFocusPoints_InvalidID verifies that id <= 0 returns an error.
func TestLoadFocusPoints_InvalidID(t *testing.T) {
	repo, _ := setupCharRepos(t)
	_, err := repo.LoadFocusPoints(context.Background(), 0)
	require.Error(t, err)
}

// TestSaveFocusPoints_Property verifies that any non-negative focus point value
// survives a save/load round-trip.
func TestSaveFocusPoints_Property(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	char, err := repo.Create(ctx, makeTestCharacter(accountID, uniqueName("fp_prop")))
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		pts := rapid.IntRange(0, 100).Draw(rt, "focusPoints")
		require.NoError(rt, repo.SaveFocusPoints(ctx, char.ID, pts))
		got, loadErr := repo.LoadFocusPoints(ctx, char.ID)
		require.NoError(rt, loadErr)
		assert.Equal(rt, pts, got)
	})
}
