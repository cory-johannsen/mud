package technology_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/aoe"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

func baseTech() *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:        "aoe_test_tech",
		Name:      "AOE Test Tech",
		Tradition: technology.TraditionTechnical,
		Level:     1,
		UsageType: technology.UsagePrepared,
		Range:     technology.RangeRanged,
		Targets:   technology.TargetsAllEnemies,
		Duration:  "instant",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectDamage, Dice: "2d6", DamageType: "fire"},
			},
		},
	}
}

func TestTechnology_LegacyAoeRadiusBackCompat(t *testing.T) {
	td := baseTech()
	td.AoeRadius = 10
	if err := td.Validate(); err != nil {
		t.Fatalf("expected nil for legacy aoe_radius; got %v", err)
	}
	if td.AoeShape != aoe.AoeShapeNone {
		t.Fatalf("expected AoeShapeNone; got %q", td.AoeShape)
	}
}

func TestTechnology_ConeRequiresLength(t *testing.T) {
	td := baseTech()
	td.AoeShape = aoe.AoeShapeCone
	err := td.Validate()
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error; got %v", err)
	}
}

func TestTechnology_ConeWithLengthOK(t *testing.T) {
	td := baseTech()
	td.AoeShape = aoe.AoeShapeCone
	td.AoeLength = 30
	if err := td.Validate(); err != nil {
		t.Fatalf("expected nil; got %v", err)
	}
}

func TestTechnology_BurstWithLengthRejected(t *testing.T) {
	td := baseTech()
	td.AoeShape = aoe.AoeShapeBurst
	td.AoeRadius = 10
	td.AoeLength = 20
	if err := td.Validate(); err == nil {
		t.Fatal("expected error for burst with length")
	}
}
