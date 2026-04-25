package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestProperty_CoverBonus_MatchesSpec(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]combat.CoverTier{
			combat.Lesser, combat.Standard, combat.Greater,
		}).Draw(rt, "tier")
		ac, qk := combat.CoverBonus(tier)
		switch tier {
		case combat.Lesser:
			if ac != 1 {
				rt.Fatalf("Lesser cover: want AC=1 got %d", ac)
			}
			if qk != 0 {
				rt.Fatalf("Lesser cover: want QK=0 got %d", qk)
			}
		case combat.Standard:
			if ac != 2 {
				rt.Fatalf("Standard cover: want AC=2 got %d", ac)
			}
			if qk != 2 {
				rt.Fatalf("Standard cover: want QK=2 got %d", qk)
			}
		case combat.Greater:
			if ac != 4 {
				rt.Fatalf("Greater cover: want AC=4 got %d", ac)
			}
			if qk != 4 {
				rt.Fatalf("Greater cover: want QK=4 got %d", qk)
			}
		}
	})
}

func TestProperty_CoverTierRoundtrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]combat.CoverTier{
			combat.Lesser, combat.Standard, combat.Greater,
		}).Draw(rt, "tier")
		got := combat.CoverTierFromString(tier.String())
		if got != tier {
			rt.Fatalf("roundtrip: got %v want %v", got, tier)
		}
	})
}

func TestProperty_BuildCoverEffect_NoCoverHasNoBonuses(t *testing.T) {
	e := combat.BuildCoverEffect(combat.NoCover)
	if len(e.Bonuses) != 0 {
		t.Fatalf("NoCover effect must have no bonuses, got %d", len(e.Bonuses))
	}
}

func TestDetermineCoverTier_EmptyStringIsNoCover(t *testing.T) {
	c := &combat.Combatant{CoverTier: ""}
	got := combat.DetermineCoverTier(c)
	if got != combat.NoCover {
		t.Fatalf("empty CoverTier: want NoCover got %v", got)
	}
}

func TestCoverBonus_NoCoverIsZero(t *testing.T) {
	ac, qk := combat.CoverBonus(combat.NoCover)
	if ac != 0 || qk != 0 {
		t.Fatalf("NoCover must yield 0,0 got %d,%d", ac, qk)
	}
}
