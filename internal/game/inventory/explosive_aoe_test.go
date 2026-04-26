package inventory_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/aoe"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func baseExplosive() *inventory.ExplosiveDef {
	return &inventory.ExplosiveDef{
		ID:         "frag",
		Name:       "Frag Grenade",
		DamageDice: "2d6",
		DamageType: "piercing",
		AreaType:   inventory.AreaTypeBurst,
		SaveType:   "reflex",
		SaveDC:     17,
		Fuse:       inventory.FuseImmediate,
	}
}

func TestExplosive_LegacyAoeRadiusBackCompat(t *testing.T) {
	e := baseExplosive()
	e.AoERadius = 10
	if err := e.Validate(); err != nil {
		t.Fatalf("expected nil for legacy aoe_radius; got %v", err)
	}
	if e.AoeShape != aoe.AoeShapeNone {
		t.Fatalf("expected AoeShapeNone; got %q", e.AoeShape)
	}
}

func TestExplosive_ConeRequiresLength(t *testing.T) {
	e := baseExplosive()
	e.AoeShape = aoe.AoeShapeCone
	err := e.Validate()
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error; got %v", err)
	}
}

func TestExplosive_ConeWithLengthOK(t *testing.T) {
	e := baseExplosive()
	e.AoeShape = aoe.AoeShapeCone
	e.AoeLength = 30
	if err := e.Validate(); err != nil {
		t.Fatalf("expected nil; got %v", err)
	}
}

func TestExplosive_LineNoLengthRejected(t *testing.T) {
	e := baseExplosive()
	e.AoeShape = aoe.AoeShapeLine
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for line without aoe_length")
	}
}
