package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
)

// validCategories enumerates the known proficiency category values.
var validCategories = []string{
	"simple_weapons", "simple_ranged", "martial_weapons", "martial_ranged",
	"martial_melee", "unarmed", "specialized",
	"unarmored", "light_armor", "medium_armor", "heavy_armor",
}

// validRanks enumerates the known proficiency rank values.
var validRanks = []string{"untrained", "trained", "expert", "master", "legendary"}

func testDBWithProficiencies(t *testing.T) *testutil.PostgresContainer {
	t.Helper()
	pc := testutil.NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	pc.ApplyProficienciesMigration(t)
	return pc
}

// --- nil-pool precondition tests (no DB required) ---

func TestCharacterProficienciesRepository_GetAll_NonPositiveIDReturnsEmpty(t *testing.T) {
	repo := pgstore.NewCharacterProficienciesRepository(nil)
	profs, err := repo.GetAll(context.Background(), 0)
	require.NoError(t, err)
	assert.Empty(t, profs)

	profs, err = repo.GetAll(context.Background(), -1)
	require.NoError(t, err)
	assert.Empty(t, profs)
}

func TestCharacterProficienciesRepository_Upsert_ValidatesInputs(t *testing.T) {
	repo := pgstore.NewCharacterProficienciesRepository(nil)
	ctx := context.Background()

	err := repo.Upsert(ctx, 0, "light_armor", "trained")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "characterID must be > 0")

	err = repo.Upsert(ctx, -1, "light_armor", "trained")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "characterID must be > 0")

	err = repo.Upsert(ctx, 1, "", "trained")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "category must not be empty")

	err = repo.Upsert(ctx, 1, "light_armor", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rank must not be empty")
}

// --- DB integration tests ---

func TestCharacterProficienciesRepository_GetAll_EmptyForNew(t *testing.T) {
	pc := testDBWithProficiencies(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterProficienciesRepository(pc.RawPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCharacterProficienciesRepository_Upsert_RoundTrip(t *testing.T) {
	pc := testDBWithProficiencies(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterProficienciesRepository(pc.RawPool)

	err := repo.Upsert(ctx, ch.ID, "light_armor", "trained")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Contains(t, got, "light_armor")
	assert.Equal(t, "trained", got["light_armor"])
}

func TestCharacterProficienciesRepository_Upsert_UpdatesExisting(t *testing.T) {
	pc := testDBWithProficiencies(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterProficienciesRepository(pc.RawPool)

	require.NoError(t, repo.Upsert(ctx, ch.ID, "heavy_armor", "untrained"))
	require.NoError(t, repo.Upsert(ctx, ch.ID, "heavy_armor", "expert"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "expert", got["heavy_armor"])
}

// --- property-based tests ---

func TestProperty_CharacterProficienciesRepository_UpsertIdempotent(t *testing.T) {
	pc := testDBWithProficiencies(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	repo := pgstore.NewCharacterProficienciesRepository(pc.RawPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)

		category := rapid.SampledFrom(validCategories).Draw(rt, "category")
		rank := rapid.SampledFrom(validRanks).Draw(rt, "rank")

		if err := repo.Upsert(ctx, ch.ID, category, rank); err != nil {
			rt.Fatalf("first Upsert(%q, %q): %v", category, rank, err)
		}
		if err := repo.Upsert(ctx, ch.ID, category, rank); err != nil {
			rt.Fatalf("second Upsert(%q, %q): %v", category, rank, err)
		}

		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}

		gotRank, ok := got[category]
		if !ok {
			rt.Fatalf("category %q not found in result", category)
		}
		if gotRank != rank {
			rt.Fatalf("category %q: got rank %q, want %q", category, gotRank, rank)
		}
	})
}
