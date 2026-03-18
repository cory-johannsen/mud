package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// --- Hardwired repo ---

func TestCharacterHardwiredTechRepo_GetAll_EmptyForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCharacterHardwiredTechRepo_SetAll_And_GetAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

	require.NoError(t, repo.SetAll(ctx, ch.ID, []string{"neural_shock", "mind_spike"}))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"neural_shock", "mind_spike"}, got)
}

func TestPropertyCharacterHardwiredTechRepo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterHardwiredTechRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		ids := make([]string, n)
		for i := range ids {
			ids[i] = rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
		}
		// Deduplicate expected values to match SetAll semantics.
		seen := make(map[string]struct{}, len(ids))
		unique := ids[:0:0]
		for _, id := range ids {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				unique = append(unique, id)
			}
		}
		if err := repo.SetAll(ctx, ch.ID, ids); err != nil {
			rt.Fatalf("SetAll: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		assert.ElementsMatch(rt, unique, got)
	})
}

// --- Prepared repo ---

func TestCharacterPreparedTechRepo_GetAll_EmptyForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCharacterPreparedTechRepo_Set_And_GetAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

	require.NoError(t, repo.Set(ctx, ch.ID, 1, 0, "neural_shock"))
	require.NoError(t, repo.Set(ctx, ch.ID, 1, 1, "mind_spike"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, got[1], 2)
	assert.Equal(t, "neural_shock", got[1][0].TechID)
	assert.Equal(t, "mind_spike", got[1][1].TechID)
}

func TestCharacterPreparedTechRepo_DeleteAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

	require.NoError(t, repo.Set(ctx, ch.ID, 1, 0, "neural_shock"))
	require.NoError(t, repo.DeleteAll(ctx, ch.ID))
	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPropertyCharacterPreparedTechRepo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterPreparedTechRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		level := rapid.IntRange(1, 10).Draw(rt, "level")
		index := rapid.IntRange(0, 4).Draw(rt, "index")
		techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
		if err := repo.Set(ctx, ch.ID, level, index, techID); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		require.True(rt, len(got[level]) > index, "slot not found at level %d index %d", level, index)
		assert.Equal(rt, techID, got[level][index].TechID)
	})
}

// --- Spontaneous repo ---

func TestCharacterSpontaneousTechRepo_GetAll_EmptyForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCharacterSpontaneousTechRepo_Add_And_GetAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

	require.NoError(t, repo.Add(ctx, ch.ID, "battle_fervor", 1))
	require.NoError(t, repo.Add(ctx, ch.ID, "acid_spray", 1))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"battle_fervor", "acid_spray"}, got[1])
}

func TestCharacterSpontaneousTechRepo_DeleteAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

	require.NoError(t, repo.Add(ctx, ch.ID, "battle_fervor", 1))
	require.NoError(t, repo.DeleteAll(ctx, ch.ID))
	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPropertyCharacterSpontaneousTechRepo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterSpontaneousTechRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
		level := rapid.IntRange(1, 10).Draw(rt, "level")
		if err := repo.Add(ctx, ch.ID, techID, level); err != nil {
			rt.Fatalf("Add: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		assert.Contains(rt, got[level], techID)
	})
}

// --- Innate repo ---

func TestCharacterInnateTechRepo_GetAll_EmptyForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCharacterInnateTechRepo_Set_And_GetAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

	require.NoError(t, repo.Set(ctx, ch.ID, "acid_spray", 3))
	require.NoError(t, repo.Set(ctx, ch.ID, "neural_shock", 0))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, &session.InnateSlot{MaxUses: 3, UsesRemaining: 3}, got["acid_spray"])
	assert.Equal(t, &session.InnateSlot{MaxUses: 0, UsesRemaining: 0}, got["neural_shock"])
}

func TestCharacterInnateTechRepo_DeleteAll(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

	require.NoError(t, repo.Set(ctx, ch.ID, "neural_shock", 2))
	require.NoError(t, repo.DeleteAll(ctx, ch.ID))
	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPropertyCharacterInnateTechRepo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	repo := pgstore.NewCharacterInnateTechRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		techID := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "techID")
		maxUses := rapid.IntRange(0, 10).Draw(rt, "maxUses")
		if err := repo.Set(ctx, ch.ID, techID, maxUses); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		require.Contains(rt, got, techID)
		assert.Equal(rt, maxUses, got[techID].MaxUses)
	})
}
