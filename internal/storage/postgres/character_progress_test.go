package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestSaveProgress_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	err := progressRepo.SaveProgress(ctx, ch.ID, 5, 2500, 35, 1)
	require.NoError(t, err)

	level, xp, maxHP, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, level)
	assert.Equal(t, 2500, xp)
	assert.Equal(t, 35, maxHP)
	assert.Equal(t, 1, boosts)
}

func TestSaveProgress_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 2, 400, 15, 0))
	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 3, 900, 20, 0))

	level, xp, _, _, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, level)
	assert.Equal(t, 900, xp)
}

func TestGetProgress_DefaultsForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	level, xp, maxHP, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, ch.Level, level)
	assert.Equal(t, ch.Experience, xp)
	assert.Equal(t, ch.MaxHP, maxHP)
	assert.Equal(t, 0, boosts)
}

func TestConsumePendingBoost_DecrementsCount(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 5, 2500, 35, 2))
	require.NoError(t, progressRepo.ConsumePendingBoost(ctx, ch.ID))

	_, _, _, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, boosts)
}

func TestConsumePendingBoost_NoneAvailableReturnsError(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	err := progressRepo.ConsumePendingBoost(ctx, ch.ID)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "no pending boosts")
}

func TestSaveProgress_InvalidID(t *testing.T) {
	ctx := context.Background()
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	assert.Error(t, progressRepo.SaveProgress(ctx, 0, 1, 0, 10, 0))
	assert.Error(t, progressRepo.SaveProgress(ctx, -1, 1, 0, 10, 0))
}

func TestGetProgress_InvalidID(t *testing.T) {
	ctx := context.Background()
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	_, _, _, _, err := progressRepo.GetProgress(ctx, 0)
	assert.Error(t, err)
}

func TestProperty_SaveProgress_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		level := rapid.IntRange(1, 100).Draw(rt, "level")
		xpVal := rapid.IntRange(0, 1_000_000).Draw(rt, "xp")
		maxHP := rapid.IntRange(1, 500).Draw(rt, "maxHP")
		boosts := rapid.IntRange(0, 20).Draw(rt, "boosts")

		if err := progressRepo.SaveProgress(ctx, ch.ID, level, xpVal, maxHP, boosts); err != nil {
			rt.Fatalf("SaveProgress: %v", err)
		}
		gotLevel, gotXP, gotMaxHP, gotBoosts, err := progressRepo.GetProgress(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetProgress: %v", err)
		}
		if gotLevel != level || gotXP != xpVal || gotMaxHP != maxHP || gotBoosts != boosts {
			rt.Fatalf("mismatch: got level=%d xp=%d maxHP=%d boosts=%d, want level=%d xp=%d maxHP=%d boosts=%d",
				gotLevel, gotXP, gotMaxHP, gotBoosts, level, xpVal, maxHP, boosts)
		}
	})
}
