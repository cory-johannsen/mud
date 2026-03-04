package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// NewCharacterFeatsRepository re-exports the constructor for use in feats tests.
func NewCharacterFeatsRepository(db *pgxpool.Pool) *pgstore.CharacterFeatsRepository {
	return pgstore.NewCharacterFeatsRepository(db)
}

func TestCharacterFeatsRepository_HasFeats_FalseForNew(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFeatsRepository(db)

	has, err := repo.HasFeats(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasFeats: %v", err)
	}
	if has {
		t.Error("expected HasFeats=false for new character")
	}
}

func TestCharacterFeatsRepository_SetAll_And_GetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFeatsRepository(db)

	feats := []string{"toughness", "quick_dodge", "combat_patch"}
	if err := repo.SetAll(ctx, ch.ID, feats); err != nil {
		t.Fatalf("SetAll: %v", err)
	}

	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != len(feats) {
		t.Errorf("expected %d feats, got %d", len(feats), len(got))
	}
	for _, id := range feats {
		var found bool
		for _, g := range got {
			if g == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected feat %q in GetAll result", id)
		}
	}
}

func TestCharacterFeatsRepository_HasFeats_TrueAfterSetAll(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFeatsRepository(db)

	if err := repo.SetAll(ctx, ch.ID, []string{"toughness"}); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	has, err := repo.HasFeats(ctx, ch.ID)
	if err != nil {
		t.Fatalf("HasFeats: %v", err)
	}
	if !has {
		t.Error("expected HasFeats=true after SetAll")
	}
}

func TestCharacterFeatsRepository_SetAll_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	charRepo := NewCharacterRepository(db)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := NewCharacterFeatsRepository(db)

	first := []string{"toughness", "fleet"}
	second := []string{"quick_dodge"}

	if err := repo.SetAll(ctx, ch.ID, first); err != nil {
		t.Fatalf("first SetAll: %v", err)
	}
	if err := repo.SetAll(ctx, ch.ID, second); err != nil {
		t.Fatalf("second SetAll: %v", err)
	}
	got, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(got) != 1 || got[0] != "quick_dodge" {
		t.Errorf("expected only [quick_dodge] after second SetAll, got %v", got)
	}
}
