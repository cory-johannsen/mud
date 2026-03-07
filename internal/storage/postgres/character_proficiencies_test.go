package postgres_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterProficienciesRepository_GetAll_NonPositiveIDReturnsEmpty(t *testing.T) {
	// GetAll with characterID <= 0 must return empty map without hitting DB.
	repo := postgres.NewCharacterProficienciesRepository(nil)
	profs, err := repo.GetAll(context.Background(), 0)
	require.NoError(t, err)
	assert.Empty(t, profs)

	profs, err = repo.GetAll(context.Background(), -1)
	require.NoError(t, err)
	assert.Empty(t, profs)
}

func TestCharacterProficienciesRepository_Upsert_ValidatesInputs(t *testing.T) {
	repo := postgres.NewCharacterProficienciesRepository(nil)
	ctx := context.Background()

	err := repo.Upsert(ctx, 0, "light_armor", "trained")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "characterID must be > 0")

	err = repo.Upsert(ctx, 1, "", "trained")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "category must not be empty")

	err = repo.Upsert(ctx, 1, "light_armor", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rank must not be empty")
}
