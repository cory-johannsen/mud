package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/game/character"
	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
)

// NewCharacterSkillsRepository is a thin wrapper so the skills test file
// can call it without a package qualifier.
func NewCharacterSkillsRepository(db *pgxpool.Pool) *pgstore.CharacterSkillsRepository {
	return pgstore.NewCharacterSkillsRepository(db)
}

// NewCharacterRepository re-exports the constructor for use in skill tests.
func NewCharacterRepository(db *pgxpool.Pool) *pgstore.CharacterRepository {
	return pgstore.NewCharacterRepository(db)
}

// testDB starts a fresh Postgres container, applies all migrations including
// character_skills, and returns the raw pool.
//
// Precondition: Docker must be available.
// Postcondition: Returns a connected pool with schema applied, or fails the test.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pc := testutil.NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	pc.ApplySkillsMigration(t)
	return pc.RawPool
}

// createTestCharacter creates a minimal character in the database and returns it.
//
// Precondition: charRepo must be connected to an initialised test database.
// Postcondition: Returns a persisted character with ID set.
func createTestCharacter(t *testing.T, charRepo *pgstore.CharacterRepository, ctx context.Context) *character.Character {
	t.Helper()
	pool := charRepo.Pool()
	acctRepo := pgstore.NewAccountRepository(pool)
	name := fmt.Sprintf("testuser_%d", time.Now().UnixNano())
	acct, err := acctRepo.Create(ctx, name, "password123")
	if err != nil {
		t.Fatalf("createTestCharacter: create account: %v", err)
	}
	ch := &character.Character{
		AccountID: acct.ID,
		Name:      fmt.Sprintf("hero_%d", time.Now().UnixNano()),
		Region:    "old_town",
		Class:     "ganger",
		Level:     1,
		Location:  "grinders_row",
		Abilities: character.AbilityScores{
			Brutality: 14, Quickness: 12, Grit: 10,
			Reasoning: 10, Savvy: 8, Flair: 12,
		},
		MaxHP:     10,
		CurrentHP: 10,
	}
	created, err := charRepo.Create(ctx, ch)
	if err != nil {
		t.Fatalf("createTestCharacter: create character: %v", err)
	}
	return created
}
