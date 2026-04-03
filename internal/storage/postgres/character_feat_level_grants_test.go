package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// NewCharacterFeatLevelGrantsRepository re-exports the constructor for use in tests.
func NewCharacterFeatLevelGrantsRepository(db *pgxpool.Pool) *pgstore.CharacterFeatLevelGrantsRepository {
	return pgstore.NewCharacterFeatLevelGrantsRepository(db)
}

func TestCharacterFeatLevelGrantsRepository(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFeatLevelGrantsRepository(db)

	// Not yet granted.
	ok, err := repo.IsLevelGranted(ctx, ch.ID, 2)
	require.NoError(t, err)
	assert.False(t, ok)

	// Mark level 2.
	require.NoError(t, repo.MarkLevelGranted(ctx, ch.ID, 2))

	ok, err = repo.IsLevelGranted(ctx, ch.ID, 2)
	require.NoError(t, err)
	assert.True(t, ok)

	// Level 4 still not granted.
	ok, err = repo.IsLevelGranted(ctx, ch.ID, 4)
	require.NoError(t, err)
	assert.False(t, ok)

	// Duplicate mark is a no-op.
	require.NoError(t, repo.MarkLevelGranted(ctx, ch.ID, 2))
}
