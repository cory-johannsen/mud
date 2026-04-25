package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestCoverBonus_RaisesEffectiveAC verifies that BuildCoverEffect produces a
// circumstance-typed AC bonus that flows through effect.Resolve to inflate the
// target's effective AC (COVER-4, COVER-8). This is the documentation anchor
// for the AC/AttackTotal inversion fix that Tasks 3-5 wire into round.go.
func TestCoverBonus_RaisesEffectiveAC(t *testing.T) {
	tier := combat.Standard
	ac, _ := combat.CoverBonus(tier)
	if ac != 2 {
		t.Fatalf("Standard cover AC bonus should be 2, got %d", ac)
	}

	e := combat.BuildCoverEffect(tier)
	var found bool
	for _, b := range e.Bonuses {
		if b.Stat == effect.StatAC && b.Value == 2 && b.Type == effect.BonusTypeCircumstance {
			found = true
		}
	}
	if !found {
		t.Error("BuildCoverEffect(Standard) must produce a circumstance +2 AC bonus")
	}
}

// TestCoverBonus_AppliedToTargetEffects_ResolvesToHigherAC verifies the round-trip
// from BuildCoverEffect → target.Effects.Apply → effect.Resolve(StatAC).Total: the
// resolved AC delta must equal CoverBonus(tier).ac.
func TestCoverBonus_AppliedToTargetEffects_ResolvesToHigherAC(t *testing.T) {
	for _, tier := range []combat.CoverTier{combat.Lesser, combat.Standard, combat.Greater} {
		t.Run(tier.String(), func(t *testing.T) {
			target := &combat.Combatant{
				ID:      "target",
				Name:    "target",
				Kind:    combat.KindNPC,
				AC:      14,
				MaxHP:   20,
				Effects: effect.NewEffectSet(),
			}
			expectedAC, expectedQK := combat.CoverBonus(tier)

			target.Effects.Apply(combat.BuildCoverEffect(tier))
			defer target.Effects.Remove(combat.CoverSourceID(tier), "")

			gotAC := effect.Resolve(target.Effects, effect.StatAC).Total
			if gotAC != expectedAC {
				t.Errorf("tier %s: AC bonus from Effects = %d, want %d", tier, gotAC, expectedAC)
			}
			gotQK := effect.Resolve(target.Effects, effect.StatQuickness).Total
			if gotQK != expectedQK {
				t.Errorf("tier %s: Quickness bonus from Effects = %d, want %d", tier, gotQK, expectedQK)
			}
		})
	}
}
