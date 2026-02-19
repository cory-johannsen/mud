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
