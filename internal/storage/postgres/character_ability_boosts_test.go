package postgres_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
)

func testDBWithAbilityBoosts(t *testing.T) *testutil.PostgresContainer {
	t.Helper()
	pc := testutil.NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	pc.ApplyAbilityBoostsMigration(t)
	return pc
}

func TestCharacterAbilityBoostsRepo_GetAll_EmptyForNew(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCharacterAbilityBoostsRepo_Add_And_GetAll(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	err := repo.Add(ctx, ch.ID, "archetype", "brutality")
	require.NoError(t, err)

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Contains(t, got, "archetype")
	assert.Equal(t, []string{"brutality"}, got["archetype"])
}

func TestCharacterAbilityBoostsRepo_Add_Idempotent(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	require.NoError(t, repo.Add(ctx, ch.ID, "archetype", "quickness"))
	require.NoError(t, repo.Add(ctx, ch.ID, "archetype", "quickness"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"quickness"}, got["archetype"])
}

func TestCharacterAbilityBoostsRepo_GetAll_SortedAbilities(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	abilities := []string{"savvy", "brutality", "grit", "flair"}
	for _, a := range abilities {
		require.NoError(t, repo.Add(ctx, ch.ID, "region", a))
	}

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	require.Contains(t, got, "region")

	want := make([]string, len(abilities))
	copy(want, abilities)
	sort.Strings(want)
	assert.Equal(t, want, got["region"])
}

func TestCharacterAbilityBoostsRepo_GetAll_MultipleSources(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	require.NoError(t, repo.Add(ctx, ch.ID, "archetype", "brutality"))
	require.NoError(t, repo.Add(ctx, ch.ID, "region", "reasoning"))

	got, err := repo.GetAll(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"brutality"}, got["archetype"])
	assert.Equal(t, []string{"reasoning"}, got["region"])
}

func TestPropertyCharacterAbilityBoostsRepo_RoundTrip(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(pc.RawPool)
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		source := rapid.StringMatching(`[a-z_]{1,16}`).Draw(rt, "source")
		ability := rapid.StringMatching(`[a-z_]{1,16}`).Draw(rt, "ability")

		if err := repo.Add(ctx, ch.ID, source, ability); err != nil {
			rt.Fatalf("Add: %v", err)
		}
		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		abilities, ok := got[source]
		if !ok {
			rt.Fatalf("source %q not found in result", source)
		}
		found := false
		for _, a := range abilities {
			if a == ability {
				found = true
				break
			}
		}
		if !found {
			rt.Fatalf("ability %q not found under source %q: %v", ability, source, abilities)
		}
	})
}
