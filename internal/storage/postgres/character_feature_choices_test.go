package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDBWithFeatureChoices(_ *testing.T) *pgxpool.Pool {
	return sharedPool
}

func TestCharacterFeatureChoicesRepo_GetAll_EmptyForNew(t *testing.T) {
	pool := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCharacterFeatureChoicesRepo_Set_And_GetAll(t *testing.T) {
	pool := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pool)

	err := repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "human")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Contains(t, got, "predators_eye")
	assert.Equal(t, "human", got["predators_eye"]["favored_target"])
}

func TestCharacterFeatureChoicesRepo_Set_IsIdempotent(t *testing.T) {
	pool := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pool)

	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "robot"))
	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "mutant"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "mutant", got["predators_eye"]["favored_target"])
}

func TestCharacterFeatureChoicesRepo_MultipleFeatures(t *testing.T) {
	pool := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pool)

	require.NoError(t, repo.Set(ctx, ch.ID, "predators_eye", "favored_target", "animal"))
	require.NoError(t, repo.Set(ctx, ch.ID, "weapon_focus", "weapon_group", "rifle"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "animal", got["predators_eye"]["favored_target"])
	assert.Equal(t, "rifle", got["weapon_focus"]["weapon_group"])
}

func TestPropertyCharacterFeatureChoicesRepo_RoundTrip(t *testing.T) {
	pool := testDBWithFeatureChoices(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pool)
	repo := pgstore.NewCharacterFeatureChoicesRepo(pool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		featureID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "featureID")
		choiceKey := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "choiceKey")
		value := rapid.StringMatching(`[a-z]{1,32}`).Draw(rt, "value")

		if err := repo.Set(ctx, ch.ID, featureID, choiceKey, value); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		if got[featureID][choiceKey] != value {
			rt.Fatalf("value mismatch: got %q want %q", got[featureID][choiceKey], value)
		}
	})
}
