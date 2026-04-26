package ruleset_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/aoe"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadClassFeatures_LegacyAoeRadiusBackCompat(t *testing.T) {
	yaml := `class_features:
  - id: legacy_blast
    name: Legacy Blast
    aoe_radius: 15
`
	feats, err := ruleset.LoadClassFeaturesFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadClassFeaturesFromBytes: %v", err)
	}
	if feats[0].AoeShape != aoe.AoeShapeNone || feats[0].AoeRadius != 15 {
		t.Fatalf("expected legacy form; got %+v", feats[0])
	}
}

func TestLoadClassFeatures_ConeRequiresLength(t *testing.T) {
	yaml := `class_features:
  - id: cone_feature
    name: Cone Feature
    aoe_shape: cone
`
	_, err := ruleset.LoadClassFeaturesFromBytes([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error; got %v", err)
	}
}

func TestLoadClassFeatures_LineDefaultWidth(t *testing.T) {
	yaml := `class_features:
  - id: line_feature
    name: Line Feature
    aoe_shape: line
    aoe_length: 30
`
	feats, err := ruleset.LoadClassFeaturesFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadClassFeaturesFromBytes: %v", err)
	}
	// AoeWidth left at zero; consumer applies aoe.ResolveLineWidth at use time.
	if feats[0].AoeShape != aoe.AoeShapeLine || feats[0].AoeWidth != 0 {
		t.Fatalf("expected line shape with zero width (defaulted at use); got %+v", feats[0])
	}
	if got := aoe.ResolveLineWidth(feats[0].AoeWidth); got != aoe.DefaultLineWidthFt {
		t.Fatalf("ResolveLineWidth = %d; want %d", got, aoe.DefaultLineWidthFt)
	}
}
