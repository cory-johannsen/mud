package technology

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestTechnologyDef_PassiveBonuses_Field confirms that TechnologyDef has
// a PassiveBonuses field and can be populated with struct literals.
func TestTechnologyDef_PassiveBonuses_Field(t *testing.T) {
	tech := &TechnologyDef{
		ID:        "test_passive_tech",
		Name:      "Test Passive Tech",
		Tradition: TraditionTechnical,
		Level:     1,
		UsageType: UsageHardwired,
		Range:     RangeSelf,
		Targets:   TargetsSingle,
		Duration:  "permanent",
		Passive:   true,
		PassiveBonuses: []effect.Bonus{
			{
				Stat:  effect.StatGrit,
				Value: 2,
				Type:  effect.BonusTypeStatus,
			},
		},
	}

	if len(tech.PassiveBonuses) != 1 {
		t.Fatalf("PassiveBonuses: got %d entries, want 1", len(tech.PassiveBonuses))
	}

	b := tech.PassiveBonuses[0]
	if b.Stat != effect.StatGrit {
		t.Errorf("bonus.Stat: got %q, want %q", b.Stat, effect.StatGrit)
	}
	if b.Value != 2 {
		t.Errorf("bonus.Value: got %d, want 2", b.Value)
	}
	if b.Type != effect.BonusTypeStatus {
		t.Errorf("bonus.Type: got %q, want %q", b.Type, effect.BonusTypeStatus)
	}
}
