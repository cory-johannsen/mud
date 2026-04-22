// internal/game/reaction/budget_test.go
package reaction_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestProperty_Budget_SpentNeverExceedsMax(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		max := rapid.IntRange(0, 10).Draw(t, "max")
		b := &reaction.Budget{}
		b.Reset(max)
		ops := rapid.SliceOfN(rapid.IntRange(0, 1), 0, 20).Draw(t, "ops") // 0=TrySpend, 1=Refund
		for _, op := range ops {
			switch op {
			case 0:
				b.TrySpend()
			case 1:
				b.Refund()
			}
			if b.Spent < 0 {
				t.Fatalf("Spent went negative: %d", b.Spent)
			}
			if b.Spent > b.Max {
				t.Fatalf("Spent (%d) > Max (%d)", b.Spent, b.Max)
			}
		}
	})
}

func TestProperty_Budget_TrySpendIdempotentAtMax(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		max := rapid.IntRange(0, 5).Draw(t, "max")
		b := &reaction.Budget{}
		b.Reset(max)
		for i := 0; i < max; i++ {
			if !b.TrySpend() {
				t.Fatalf("TrySpend returned false before reaching max (i=%d, Spent=%d, Max=%d)", i, b.Spent, b.Max)
			}
		}
		for i := 0; i < 5; i++ {
			if b.TrySpend() {
				t.Fatalf("TrySpend returned true when already at max (Spent=%d, Max=%d)", b.Spent, b.Max)
			}
		}
	})
}

func TestProperty_Budget_RefundNoOpAtZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := &reaction.Budget{}
		b.Reset(rapid.IntRange(0, 5).Draw(t, "max"))
		b.Refund() // call on a fresh (Spent==0) budget
		if b.Spent < 0 {
			t.Fatalf("Spent went negative after Refund on zero: %d", b.Spent)
		}
	})
}

func TestProperty_Budget_RemainingConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		max := rapid.IntRange(0, 10).Draw(t, "max")
		b := &reaction.Budget{}
		b.Reset(max)
		ops := rapid.SliceOfN(rapid.IntRange(0, 1), 0, 20).Draw(t, "ops")
		for _, op := range ops {
			switch op {
			case 0:
				b.TrySpend()
			case 1:
				b.Refund()
			}
			if b.Remaining() != b.Max-b.Spent {
				t.Fatalf("Remaining() = %d, want Max-Spent = %d (Max=%d Spent=%d)",
					b.Remaining(), b.Max-b.Spent, b.Max, b.Spent)
			}
		}
	})
}
