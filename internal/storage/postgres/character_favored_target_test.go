package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/testutil"
)

// NewCharacterFavoredTargetRepo re-exports the constructor for use in favored target tests.
func NewCharacterFavoredTargetRepo(db *pgxpool.Pool) *pgstore.CharacterFavoredTargetRepo {
	return pgstore.NewCharacterFavoredTargetRepo(db)
}

// testDBWithFavoredTarget returns a pool with all migrations including character_favored_target applied.
//
// Precondition: Docker must be available.
// Postcondition: Returns a connected pool with schema applied, or fails the test.
func testDBWithFavoredTarget(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pc := testutil.NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	pc.ApplySkillsMigration(t)
	pc.ApplyFeatsMigration(t)
	pc.ApplyFavoredTargetMigration(t)
	return pc.RawPool
}

func TestCharacterFavoredTargetRepo_Get_ReturnsEmptyWhenNoRow(t *testing.T) {
	ctx := context.Background()
	db := testDBWithFavoredTarget(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFavoredTargetRepo(db)

	got, err := repo.Get(ctx, ch.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for new character, got %q", got)
	}
}

func TestCharacterFavoredTargetRepo_Get_ReturnsStoredTypeAfterSet(t *testing.T) {
	ctx := context.Background()
	db := testDBWithFavoredTarget(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFavoredTargetRepo(db)

	if err := repo.Set(ctx, ch.ID, "human"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := repo.Get(ctx, ch.ID)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got != "human" {
		t.Errorf("expected %q, got %q", "human", got)
	}
}

func TestCharacterFavoredTargetRepo_Set_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testDBWithFavoredTarget(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFavoredTargetRepo(db)

	if err := repo.Set(ctx, ch.ID, "robot"); err != nil {
		t.Fatalf("first Set: %v", err)
	}
	if err := repo.Set(ctx, ch.ID, "mutant"); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	got, err := repo.Get(ctx, ch.ID)
	if err != nil {
		t.Fatalf("Get after second Set: %v", err)
	}
	if got != "mutant" {
		t.Errorf("expected %q after second Set, got %q", "mutant", got)
	}
}

// TestCharacterFavoredTargetRepo_AllValidTypes is a property-style test verifying
// each of the four valid target types can be stored and retrieved.
func TestCharacterFavoredTargetRepo_AllValidTypes(t *testing.T) {
	validTypes := []string{"human", "robot", "animal", "mutant"}

	ctx := context.Background()
	db := testDBWithFavoredTarget(t)
	charRepo := NewCharacterRepository(db)
	repo := NewCharacterFavoredTargetRepo(db)

	for _, tt := range validTypes {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			ch := createTestCharacter(t, charRepo, ctx)
			if err := repo.Set(ctx, ch.ID, tt); err != nil {
				t.Fatalf("Set(%q): %v", tt, err)
			}
			got, err := repo.Get(ctx, ch.ID)
			if err != nil {
				t.Fatalf("Get after Set(%q): %v", tt, err)
			}
			if got != tt {
				t.Errorf("expected %q, got %q", tt, got)
			}
		})
	}
}
