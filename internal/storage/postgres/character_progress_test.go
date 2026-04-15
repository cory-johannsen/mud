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

func TestGetPendingSkillIncreases_DefaultsToZero(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	n, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestIncrementPendingSkillIncreases_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, repo.IncrementPendingSkillIncreases(ctx, ch.ID, 3))
	n, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestIncrementPendingSkillIncreases_InvalidID(t *testing.T) {
	ctx := context.Background()
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	assert.Error(t, repo.IncrementPendingSkillIncreases(ctx, 0, 1))
	assert.Error(t, repo.IncrementPendingSkillIncreases(ctx, -1, 1))
}

func TestIncrementPendingSkillIncreases_InvalidN(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	assert.Error(t, repo.IncrementPendingSkillIncreases(ctx, ch.ID, 0))
}

func TestConsumePendingSkillIncrease_Decrements(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, repo.IncrementPendingSkillIncreases(ctx, ch.ID, 2))
	require.NoError(t, repo.ConsumePendingSkillIncrease(ctx, ch.ID))
	n, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestConsumePendingSkillIncrease_NoneAvailable_ReturnsError(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	err := repo.ConsumePendingSkillIncrease(ctx, ch.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending skill increases")
}

func TestPropertyIncrementPendingSkillIncreases(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		require.NoError(rt, repo.IncrementPendingSkillIncreases(ctx, ch.ID, n))
		got, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
		require.NoError(rt, err)
		if got != n {
			rt.Fatalf("expected %d, got %d", n, got)
		}
	})
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

func TestPendingTechSlots_AddAndGet(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	// Add a slot.
	err := repo.AddPendingTechSlot(ctx, ch.ID, 3, 2, "neural", "prepared")
	require.NoError(t, err)

	// Get slots — should have exactly one.
	slots, err := repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, slots, 1)
	assert.Equal(t, 3, slots[0].CharLevel)
	assert.Equal(t, 2, slots[0].TechLevel)
	assert.Equal(t, "neural", slots[0].Tradition)
	assert.Equal(t, "prepared", slots[0].UsageType)
	assert.Equal(t, 1, slots[0].Remaining)

	// Add the same slot again — remaining should become 2.
	err = repo.AddPendingTechSlot(ctx, ch.ID, 3, 2, "neural", "prepared")
	require.NoError(t, err)

	slots, err = repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, slots, 1)
	assert.Equal(t, 2, slots[0].Remaining)

	// Decrement once — remaining should become 1.
	err = repo.DecrementPendingTechSlot(ctx, ch.ID, 3, 2, "neural", "prepared")
	require.NoError(t, err)

	slots, err = repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, slots, 1)
	assert.Equal(t, 1, slots[0].Remaining)

	// Decrement again — remaining reaches 0 → row deleted.
	err = repo.DecrementPendingTechSlot(ctx, ch.ID, 3, 2, "neural", "prepared")
	require.NoError(t, err)

	slots, err = repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestPendingTechSlots_DeleteAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, repo.AddPendingTechSlot(ctx, ch.ID, 2, 2, "cyber", "spontaneous"))
	require.NoError(t, repo.AddPendingTechSlot(ctx, ch.ID, 4, 3, "bio", "prepared"))

	slots, err := repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, slots, 2)

	require.NoError(t, repo.DeleteAllPendingTechSlots(ctx, ch.ID))

	slots, err = repo.GetPendingTechSlots(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, slots)
}

func TestProperty_PendingTechSlots_AddIncrementsRemaining(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterProgressRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		for i := 0; i < n; i++ {
			if err := repo.AddPendingTechSlot(ctx, ch.ID, 3, 2, "neural", "prepared"); err != nil {
				rt.Fatalf("AddPendingTechSlot: %v", err)
			}
		}
		slots, err := repo.GetPendingTechSlots(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetPendingTechSlots: %v", err)
		}
		if len(slots) != 1 {
			rt.Fatalf("expected 1 slot, got %d", len(slots))
		}
		if slots[0].Remaining != n {
			rt.Fatalf("expected remaining=%d, got %d", n, slots[0].Remaining)
		}
	})
}
