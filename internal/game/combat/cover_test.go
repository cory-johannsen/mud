package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestCombatantCoverFields(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tier := rapid.SampledFrom([]string{"", "lesser", "standard", "greater"}).Draw(t, "tier")
		equipID := rapid.String().Draw(t, "equipID")
		c := &combat.Combatant{
			CoverEquipmentID: equipID,
			CoverTier:        tier,
		}
		if c.CoverEquipmentID != equipID {
			t.Errorf("CoverEquipmentID: got %q, want %q", c.CoverEquipmentID, equipID)
		}
		if c.CoverTier != tier {
			t.Errorf("CoverTier: got %q, want %q", c.CoverTier, tier)
		}
	})
}
