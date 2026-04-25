package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestCoverBonus_StandardCover_RaisesEffectiveAC confirms that applying
// BuildCoverEffect to target.Effects increases the resolved AC bonus by
// the expected magnitude per tier.
func TestCoverBonus_StandardCover_RaisesEffectiveAC(t *testing.T) {
	target := &combat.Combatant{
		ID: "target", Name: "target", Kind: combat.KindNPC,
		AC: 16, MaxHP: 30,
		CoverTier: "standard", CoverEquipmentID: "crate_01",
		Effects: effect.NewEffectSet(),
	}
	coverTier := combat.DetermineCoverTier(target)
	if coverTier != combat.Standard {
		t.Fatalf("expected Standard cover tier, got %v", coverTier)
	}
	target.Effects.Apply(combat.BuildCoverEffect(coverTier))
	defer target.Effects.Remove(combat.CoverSourceID(coverTier), "")

	effectiveAC := target.AC + target.InitiativeBonus + effect.Resolve(target.Effects, effect.StatAC).Total
	if effectiveAC != 18 {
		t.Fatalf("expected effectiveAC=18 with standard cover (16+2), got %d", effectiveAC)
	}
}

// TestCoverBonus_PerTargetIndependent verifies that applying cover to one
// target does not leak the AC bonus to a different target's Effects.
func TestCoverBonus_PerTargetIndependent(t *testing.T) {
	targetA := &combat.Combatant{
		ID: "a", Name: "a", Kind: combat.KindNPC, AC: 14, MaxHP: 20,
		CoverTier: "standard", CoverEquipmentID: "wall",
		Effects: effect.NewEffectSet(),
	}
	targetB := &combat.Combatant{
		ID: "b", Name: "b", Kind: combat.KindNPC, AC: 14, MaxHP: 20,
		Effects: effect.NewEffectSet(),
	}
	coverA := combat.DetermineCoverTier(targetA)
	targetA.Effects.Apply(combat.BuildCoverEffect(coverA))
	acA := effect.Resolve(targetA.Effects, effect.StatAC).Total
	targetA.Effects.Remove(combat.CoverSourceID(coverA), "")

	acB := effect.Resolve(targetB.Effects, effect.StatAC).Total

	if acA <= acB {
		t.Fatalf("target A with cover should have higher AC bonus (%d) than B (%d)", acA, acB)
	}
	if acAfter := effect.Resolve(targetA.Effects, effect.StatAC).Total; acAfter != 0 {
		t.Fatalf("cover effect must be cleared after removal; got residual AC %d", acAfter)
	}
}
