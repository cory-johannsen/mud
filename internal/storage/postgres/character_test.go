package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func setupCharRepos(t *testing.T) (*postgres.CharacterRepository, int64) {
	t.Helper()
	pool := testutil.NewPool(t)
	acctRepo := postgres.NewAccountRepository(pool)
	acct, err := acctRepo.Create(context.Background(), uniqueName("user"), "password123")
	require.NoError(t, err)
	return postgres.NewCharacterRepository(pool), acct.ID
}

func makeTestCharacter(accountID int64, name string) *character.Character {
	return &character.Character{
		AccountID: accountID,
		Name:      name,
		Region:    "old_town",
		Class:     "ganger",
		Level:     1,
		Location:  "grinders_row",
		Abilities: character.AbilityScores{
			Strength: 14, Dexterity: 12, Constitution: 10,
			Intelligence: 10, Wisdom: 8, Charisma: 12,
		},
		MaxHP:     10,
		CurrentHP: 10,
	}
}

func TestCharacterRepository_Create(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID, "Zara")
	created, err := repo.Create(ctx, c)
	require.NoError(t, err)

	assert.Greater(t, created.ID, int64(0))
	assert.Equal(t, accountID, created.AccountID)
	assert.Equal(t, "Zara", created.Name)
	assert.Equal(t, "old_town", created.Region)
	assert.Equal(t, "ganger", created.Class)
	assert.Equal(t, 1, created.Level)
	assert.Equal(t, "grinders_row", created.Location)
	assert.Equal(t, 14, created.Abilities.Strength)
	assert.Equal(t, 10, created.MaxHP)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestCharacterRepository_DuplicateNameError(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	c := makeTestCharacter(accountID, "Zara")
	_, err := repo.Create(ctx, c)
	require.NoError(t, err)

	_, err = repo.Create(ctx, c) // same name, same account
	require.Error(t, err)
	assert.ErrorIs(t, err, postgres.ErrCharacterNameTaken)
}

func TestCharacterRepository_ListByAccount(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, makeTestCharacter(accountID, "Alpha"))
	require.NoError(t, err)
	_, err = repo.Create(ctx, makeTestCharacter(accountID, "Beta"))
	require.NoError(t, err)

	chars, err := repo.ListByAccount(ctx, accountID)
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestCharacterRepository_ListByAccount_Empty(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	chars, err := repo.ListByAccount(context.Background(), accountID)
	require.NoError(t, err)
	assert.NotNil(t, chars)
	assert.Empty(t, chars)
}

func TestCharacterRepository_GetByID(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, makeTestCharacter(accountID, "Zara"))
	require.NoError(t, err)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Zara", fetched.Name)
	assert.Equal(t, 14, fetched.Abilities.Strength)
}

func TestCharacterRepository_GetByID_NotFound(t *testing.T) {
	repo, _ := setupCharRepos(t)
	_, err := repo.GetByID(context.Background(), 99999999)
	require.Error(t, err)
	assert.ErrorIs(t, err, postgres.ErrCharacterNotFound)
}

func TestCharacterRepository_SaveState(t *testing.T) {
	repo, accountID := setupCharRepos(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, makeTestCharacter(accountID, "Zara"))
	require.NoError(t, err)

	err = repo.SaveState(ctx, created.ID, "broadway_ruins", 7)
	require.NoError(t, err)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "broadway_ruins", fetched.Location)
	assert.Equal(t, 7, fetched.CurrentHP)
}

func TestCharacterRepository_SaveState_NotFound(t *testing.T) {
	repo, _ := setupCharRepos(t)
	err := repo.SaveState(context.Background(), 99999999, "grinders_row", 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, postgres.ErrCharacterNotFound)
}

// setupCharReposShared creates a single pool and account repository for use across
// multiple rapid iterations within one property test. Each iteration creates a fresh
// account to ensure isolation without spawning a new container per iteration.
func setupCharReposShared(t *testing.T) (*postgres.CharacterRepository, *postgres.AccountRepository) {
	t.Helper()
	pool := testutil.NewPool(t)
	return postgres.NewCharacterRepository(pool), postgres.NewAccountRepository(pool)
}

// TestCharacterRepository_Property_CreateThenGetByID verifies that for any valid
// character fields, Create followed by GetByID returns a character equal to the one created.
func TestCharacterRepository_Property_CreateThenGetByID(t *testing.T) {
	charRepo, acctRepo := setupCharReposShared(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("user"), "pass")
		require.NoError(t, err)

		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,10}`).Draw(rt, "name")
		hp := rapid.IntRange(1, 100).Draw(rt, "hp")
		c := &character.Character{
			AccountID: acct.ID,
			Name:      name,
			Region:    "old_town",
			Class:     "ganger",
			Level:     1,
			Location:  "grinders_row",
			Abilities: character.AbilityScores{
				Strength: 10, Dexterity: 10, Constitution: 10,
				Intelligence: 10, Wisdom: 10, Charisma: 10,
			},
			MaxHP:     hp,
			CurrentHP: hp,
		}

		created, err := charRepo.Create(ctx, c)
		require.NoError(t, err)

		fetched, err := charRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, fetched.ID)
		assert.Equal(t, name, fetched.Name)
		assert.Equal(t, hp, fetched.MaxHP)
		assert.Equal(t, hp, fetched.CurrentHP)
		assert.Equal(t, "grinders_row", fetched.Location)
	})
}

// TestCharacterRepository_Property_ListCountMatchesCreates verifies that ListByAccount
// returns exactly as many characters as were created for a given account.
func TestCharacterRepository_Property_ListCountMatchesCreates(t *testing.T) {
	charRepo, acctRepo := setupCharReposShared(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("user"), "pass")
		require.NoError(t, err)

		n := rapid.IntRange(1, 5).Draw(rt, "n")
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("char_%d_%d", i, time.Now().UnixNano())
			_, err := charRepo.Create(ctx, makeTestCharacter(acct.ID, name))
			require.NoError(t, err)
		}

		chars, err := charRepo.ListByAccount(ctx, acct.ID)
		require.NoError(t, err)
		assert.Len(t, chars, n)
	})
}

// TestCharacterRepository_Property_DuplicateNameAlwaysErrors verifies that creating
// two characters with the same account+name always returns ErrCharacterNameTaken.
func TestCharacterRepository_Property_DuplicateNameAlwaysErrors(t *testing.T) {
	charRepo, acctRepo := setupCharReposShared(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("user"), "pass")
		require.NoError(t, err)

		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,10}`).Draw(rt, "name")
		c := makeTestCharacter(acct.ID, name)

		_, err = charRepo.Create(ctx, c)
		require.NoError(t, err)

		_, err = charRepo.Create(ctx, c)
		require.Error(t, err)
		assert.ErrorIs(t, err, postgres.ErrCharacterNameTaken)
	})
}

// TestCharacterRepository_Property_SaveStatePersists verifies that SaveState followed by
// GetByID always reflects the new location and currentHP values.
func TestCharacterRepository_Property_SaveStatePersists(t *testing.T) {
	charRepo, acctRepo := setupCharReposShared(t)
	ctx := context.Background()

	rapid.Check(t, func(rt *rapid.T) {
		acct, err := acctRepo.Create(ctx, uniqueName("user"), "pass")
		require.NoError(t, err)

		created, err := charRepo.Create(ctx, makeTestCharacter(acct.ID, "Prop"))
		require.NoError(t, err)

		newHP := rapid.IntRange(0, created.MaxHP).Draw(rt, "hp")
		newLoc := rapid.StringMatching(`[a-z_]{3,20}`).Draw(rt, "loc")

		err = charRepo.SaveState(ctx, created.ID, newLoc, newHP)
		require.NoError(t, err)

		fetched, err := charRepo.GetByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, newLoc, fetched.Location)
		assert.Equal(t, newHP, fetched.CurrentHP)
	})
}
