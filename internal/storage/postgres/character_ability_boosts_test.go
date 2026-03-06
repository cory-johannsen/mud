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

		// Generate a map of source → distinct abilities to write.
		written := rapid.MapOf(
			rapid.StringMatching(`[a-z_]{1,16}`),
			rapid.SliceOfDistinct(
				rapid.StringMatching(`[a-z_]{1,16}`),
				func(a string) string { return a },
			),
		).Draw(rt, "boosts")

		for src, abilities := range written {
			for _, ab := range abilities {
				if err := repo.Add(ctx, ch.ID, src, ab); err != nil {
					rt.Fatalf("Add(%q, %q): %v", src, ab, err)
				}
			}
		}

		got, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}

		// No extra sources beyond what was written.
		for src := range got {
			if _, ok := written[src]; !ok {
				rt.Fatalf("unexpected source %q in result", src)
			}
		}

		// Each source has exactly the abilities written — no extras.
		for src, wantAbilities := range written {
			gotAbilities, ok := got[src]
			if len(wantAbilities) == 0 {
				// A source with no abilities written should not appear.
				if ok && len(gotAbilities) > 0 {
					rt.Fatalf("source %q has unexpected abilities: %v", src, gotAbilities)
				}
				continue
			}
			if !ok {
				rt.Fatalf("source %q not found in result", src)
			}
			want := make([]string, len(wantAbilities))
			copy(want, wantAbilities)
			sort.Strings(want)
			if len(gotAbilities) != len(want) {
				rt.Fatalf("source %q: got %d abilities, want %d: got=%v want=%v",
					src, len(gotAbilities), len(want), gotAbilities, want)
			}
			for i := range want {
				if gotAbilities[i] != want[i] {
					rt.Fatalf("source %q abilities mismatch: got=%v want=%v", src, gotAbilities, want)
				}
			}
		}
	})
}

func TestCharacterAbilityBoostsRepo_InvalidInputs(t *testing.T) {
	pc := testDBWithAbilityBoosts(t)
	ctx := context.Background()
	repo := pgstore.NewCharacterAbilityBoostsRepository(pc.RawPool)

	t.Run("Add_characterID_zero_returns_error", func(t *testing.T) {
		err := repo.Add(ctx, 0, "archetype", "brutality")
		require.Error(t, err)
	})

	t.Run("Add_empty_source_returns_error", func(t *testing.T) {
		err := repo.Add(ctx, 1, "", "brutality")
		require.Error(t, err)
	})

	t.Run("Add_empty_ability_returns_error", func(t *testing.T) {
		err := repo.Add(ctx, 1, "archetype", "")
		require.Error(t, err)
	})

	t.Run("GetAll_characterID_zero_returns_error", func(t *testing.T) {
		_, err := repo.GetAll(ctx, 0)
		require.Error(t, err)
	})
}
