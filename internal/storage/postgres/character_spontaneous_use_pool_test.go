package postgres_test

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// TestSpontaneousUsePool_RoundTrip verifies basic Set/GetAll/Decrement semantics (REQ-SUC6).
func TestSpontaneousUsePool_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterSpontaneousUsePoolRepository(pool)

	if err := repo.Set(ctx, ch.ID, 1, 3, 5); err != nil {
		t.Fatalf("Set: %v", err)
	}

	pools, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	got, ok := pools[1]
	if !ok {
		t.Fatalf("expected pool for level 1, got none")
	}
	if got.Remaining != 3 {
		t.Errorf("Remaining: want 3, got %d", got.Remaining)
	}
	if got.Max != 5 {
		t.Errorf("Max: want 5, got %d", got.Max)
	}

	if err := repo.Decrement(ctx, ch.ID, 1); err != nil {
		t.Fatalf("Decrement: %v", err)
	}

	pools2, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after Decrement: %v", err)
	}
	got2 := pools2[1]
	if got2.Remaining != 2 {
		t.Errorf("Remaining after Decrement: want 2, got %d", got2.Remaining)
	}
	if got2.Max != 5 {
		t.Errorf("Max after Decrement: want 5, got %d", got2.Max)
	}
}

// TestSpontaneousUsePool_DecrementBelowZero_Property verifies that Decrement never
// drives uses_remaining below zero (REQ-SUC7).
func TestSpontaneousUsePool_DecrementBelowZero_Property(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		repo := pgstore.NewCharacterSpontaneousUsePoolRepository(pool)

		n := rapid.IntRange(1, 10).Draw(rt, "maxUses")
		calls := rapid.IntRange(0, 15).Draw(rt, "decrementCalls")

		if err := repo.Set(ctx, ch.ID, 1, n, n); err != nil {
			rt.Fatalf("Set: %v", err)
		}

		for i := 0; i < calls; i++ {
			if err := repo.Decrement(ctx, ch.ID, 1); err != nil {
				rt.Fatalf("Decrement call %d: %v", i, err)
			}
		}

		pools, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		got := pools[1]

		expected := n - calls
		if expected < 0 {
			expected = 0
		}
		if got.Remaining != expected {
			rt.Errorf("Remaining: want %d, got %d (n=%d, calls=%d)", expected, got.Remaining, n, calls)
		}
	})
}

// TestSpontaneousUsePool_RestoreAll verifies that RestoreAll resets all pools to max.
func TestSpontaneousUsePool_RestoreAll(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterSpontaneousUsePoolRepository(pool)

	if err := repo.Set(ctx, ch.ID, 1, 3, 5); err != nil {
		t.Fatalf("Set level 1: %v", err)
	}
	if err := repo.Set(ctx, ch.ID, 2, 2, 4); err != nil {
		t.Fatalf("Set level 2: %v", err)
	}

	if err := repo.Decrement(ctx, ch.ID, 1); err != nil {
		t.Fatalf("Decrement level 1: %v", err)
	}
	if err := repo.Decrement(ctx, ch.ID, 2); err != nil {
		t.Fatalf("Decrement level 2: %v", err)
	}

	if err := repo.RestoreAll(ctx, ch.ID); err != nil {
		t.Fatalf("RestoreAll: %v", err)
	}

	pools, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after RestoreAll: %v", err)
	}

	for _, lvl := range []int{1, 2} {
		p, ok := pools[lvl]
		if !ok {
			t.Errorf("level %d: pool missing after RestoreAll", lvl)
			continue
		}
		if p.Remaining != p.Max {
			t.Errorf("level %d: Remaining=%d want Max=%d", lvl, p.Remaining, p.Max)
		}
	}
}

// TestSpontaneousUsePool_DeleteAll verifies that DeleteAll removes all pool rows.
func TestSpontaneousUsePool_DeleteAll(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)

	repo := pgstore.NewCharacterSpontaneousUsePoolRepository(pool)

	if err := repo.Set(ctx, ch.ID, 1, 3, 5); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := repo.DeleteAll(ctx, ch.ID); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	pools, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after DeleteAll: %v", err)
	}
	if len(pools) != 0 {
		t.Errorf("expected empty map after DeleteAll, got %d entries", len(pools))
	}
}
