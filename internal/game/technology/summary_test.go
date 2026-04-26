package technology

import (
	"strings"
	"testing"
)

// TestFormatEffectsSummary_ProjectilesShown verifies that a damage effect with
// projectiles > 0 surfaces the count in the summary (#363).
func TestFormatEffectsSummary_ProjectilesShown(t *testing.T) {
	def := &TechnologyDef{
		ID: "multi_round_kinetic_volley", Name: "Multi-Round Kinetic Volley",
		ActionCost: 2, Range: RangeRanged, Targets: TargetsSingle,
		Resolution: "none",
		Effects: TieredEffects{
			OnApply: []TechEffect{
				{Type: EffectDamage, Dice: "1d4+1", DamageType: "force", Projectiles: 3},
			},
		},
	}
	got := FormatEffectsSummary(def)
	if !strings.Contains(got, "× 3 projectiles") {
		t.Fatalf("summary must show projectile count, got:\n%s", got)
	}
}

// TestFormatEffectsSummary_AoeBurstShown verifies that the burst radius
// surfaces in the header line (#363, generic across all AoE techs).
func TestFormatEffectsSummary_AoeBurstShown(t *testing.T) {
	def := &TechnologyDef{
		ID: "frag_grenade_tech", Name: "Frag Grenade",
		ActionCost: 1, Range: RangeRanged, Targets: TargetsSingle,
		Resolution: "none",
		AoeRadius:  10,
	}
	got := FormatEffectsSummary(def)
	if !strings.Contains(got, "burst 10 ft") {
		t.Fatalf("summary must show burst radius, got:\n%s", got)
	}
}

// TestFormatEffectsSummary_AoeConeShown verifies that an AoE cone shape +
// length surfaces in the header (#363).
func TestFormatEffectsSummary_AoeConeShown(t *testing.T) {
	def := &TechnologyDef{
		ID: "shock_cone_tech", Name: "Shock Cone",
		ActionCost: 2, Range: RangeMelee, Targets: TargetsSingle,
		Resolution: "save", SaveType: "quickness", SaveDC: 15,
		AoeShape:  "cone",
		AoeLength: 30,
	}
	got := FormatEffectsSummary(def)
	if !strings.Contains(got, "cone 30 ft") {
		t.Fatalf("summary must show cone length, got:\n%s", got)
	}
}

// TestFormatEffectsSummary_AoeLineShown verifies that an AoE line + width
// surfaces in the header (#363).
func TestFormatEffectsSummary_AoeLineShown(t *testing.T) {
	def := &TechnologyDef{
		ID: "laser_sweep_tech", Name: "Laser Sweep",
		ActionCost: 2, Range: RangeRanged, Targets: TargetsSingle,
		Resolution: "save", SaveType: "quickness", SaveDC: 15,
		AoeShape:  "line",
		AoeLength: 25,
		AoeWidth:  5,
	}
	got := FormatEffectsSummary(def)
	if !strings.Contains(got, "line 25x5 ft") {
		t.Fatalf("summary must show line length + width, got:\n%s", got)
	}
}
