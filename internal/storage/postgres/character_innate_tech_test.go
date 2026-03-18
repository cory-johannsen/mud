package postgres_test

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestInnateUsesRemaining_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(pool)

	if err := repo.Set(ctx, ch.ID, "acid_spit", 3); err != nil {
		t.Fatalf("Set: %v", err)
	}

	slots, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	s, ok := slots["acid_spit"]
	if !ok {
		t.Fatalf("expected acid_spit slot, got none")
	}
	if s.MaxUses != 3 {
		t.Errorf("MaxUses: want 3, got %d", s.MaxUses)
	}
	if s.UsesRemaining != 3 {
		t.Errorf("UsesRemaining after Set: want 3, got %d", s.UsesRemaining)
	}

	if err := repo.Decrement(ctx, ch.ID, "acid_spit"); err != nil {
		t.Fatalf("Decrement: %v", err)
	}

	slots2, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after Decrement: %v", err)
	}
	s2 := slots2["acid_spit"]
	if s2.UsesRemaining != 2 {
		t.Errorf("UsesRemaining after Decrement: want 2, got %d", s2.UsesRemaining)
	}
	if s2.MaxUses != 3 {
		t.Errorf("MaxUses after Decrement: want 3, got %d", s2.MaxUses)
	}

	if err := repo.RestoreAll(ctx, ch.ID); err != nil {
		t.Fatalf("RestoreAll: %v", err)
	}

	slots3, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after RestoreAll: %v", err)
	}
	s3 := slots3["acid_spit"]
	if s3.UsesRemaining != 3 {
		t.Errorf("UsesRemaining after RestoreAll: want 3, got %d", s3.UsesRemaining)
	}
}

func TestInnateDecrement_NeverBelowZero_Property(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(pool)

	rapid.Check(t, func(rt *rapid.T) {
		if err := repo.DeleteAll(ctx, ch.ID); err != nil {
			rt.Fatalf("DeleteAll: %v", err)
		}
		n := rapid.IntRange(1, 5).Draw(rt, "maxUses")
		calls := rapid.IntRange(0, 8).Draw(rt, "decrementCalls")

		if err := repo.Set(ctx, ch.ID, "acid_spit", n); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		for i := 0; i < calls; i++ {
			if err := repo.Decrement(ctx, ch.ID, "acid_spit"); err != nil {
				rt.Fatalf("Decrement %d: %v", i, err)
			}
		}
		slots, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		s := slots["acid_spit"]
		expected := n - calls
		if expected < 0 {
			expected = 0
		}
		if s.UsesRemaining != expected {
			rt.Errorf("UsesRemaining: want %d, got %d (n=%d, calls=%d)", expected, s.UsesRemaining, n, calls)
		}
		if s.UsesRemaining < 0 {
			rt.Errorf("UsesRemaining went below zero: %d", s.UsesRemaining)
		}
	})
}
