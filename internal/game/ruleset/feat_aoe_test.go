package ruleset_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/aoe"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadFeats_LegacyAoeRadiusBackCompat(t *testing.T) {
	yaml := `feats:
  - id: legacy_burst
    name: Legacy Burst
    category: general
    aoe_radius: 10
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFeatsFromBytes: %v", err)
	}
	if len(feats) != 1 {
		t.Fatalf("expected 1 feat; got %d", len(feats))
	}
	// Legacy: empty AoeShape with positive AoeRadius is allowed (treated as burst at use time).
	if feats[0].AoeShape != aoe.AoeShapeNone {
		t.Errorf("expected AoeShapeNone for legacy form; got %q", feats[0].AoeShape)
	}
	if feats[0].AoeRadius != 10 {
		t.Errorf("expected AoeRadius 10; got %d", feats[0].AoeRadius)
	}
}

func TestLoadFeats_ConeRequiresLength(t *testing.T) {
	yaml := `feats:
  - id: cone_feat
    name: Cone Feat
    category: general
    aoe_shape: cone
`
	_, err := ruleset.LoadFeatsFromBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for cone with no aoe_length")
	}
	if !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected error to mention aoe_length; got %v", err)
	}
}

func TestLoadFeats_ConeWithLengthOK(t *testing.T) {
	yaml := `feats:
  - id: cone_feat
    name: Cone Feat
    category: general
    aoe_shape: cone
    aoe_length: 30
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFeatsFromBytes: %v", err)
	}
	if feats[0].AoeShape != aoe.AoeShapeCone || feats[0].AoeLength != 30 {
		t.Fatalf("expected cone+30ft; got %+v", feats[0])
	}
}

func TestLoadFeats_BurstWithLengthRejected(t *testing.T) {
	yaml := `feats:
  - id: bad_burst
    name: Bad Burst
    category: general
    aoe_shape: burst
    aoe_radius: 10
    aoe_length: 30
`
	_, err := ruleset.LoadFeatsFromBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for burst with aoe_length")
	}
}
