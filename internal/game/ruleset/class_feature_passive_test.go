package ruleset

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestClassFeature_PassiveBonuses_ParsedFromYAML confirms that passive_bonuses
// in YAML are unmarshaled into ClassFeature.PassiveBonuses with proper types.
func TestClassFeature_PassiveBonuses_ParsedFromYAML(t *testing.T) {
	yaml := []byte(`
class_features:
  - id: test_passive
    name: Test Passive Feature
    passive_bonuses:
      - stat: grit
        value: 1
        type: status
`)

	features, err := LoadClassFeaturesFromBytes(yaml)
	if err != nil {
		t.Fatalf("LoadClassFeaturesFromBytes: %v", err)
	}

	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	f := features[0]
	if f.ID != "test_passive" {
		t.Errorf("feature ID: got %q, want %q", f.ID, "test_passive")
	}

	if len(f.PassiveBonuses) != 1 {
		t.Fatalf("PassiveBonuses: got %d entries, want 1", len(f.PassiveBonuses))
	}

	b := f.PassiveBonuses[0]
	if b.Stat != effect.StatGrit {
		t.Errorf("bonus.Stat: got %q, want %q", b.Stat, effect.StatGrit)
	}
	if b.Value != 1 {
		t.Errorf("bonus.Value: got %d, want 1", b.Value)
	}
	if b.Type != effect.BonusTypeStatus {
		t.Errorf("bonus.Type: got %q, want %q", b.Type, effect.BonusTypeStatus)
	}
}

// TestClassFeature_PassiveBonuses_EmptyByDefault confirms that when
// passive_bonuses is absent from YAML, the field defaults to an empty slice.
func TestClassFeature_PassiveBonuses_EmptyByDefault(t *testing.T) {
	yaml := []byte(`
class_features:
  - id: test_passive_no_bonuses
    name: Test Feature Without Bonuses
`)

	features, err := LoadClassFeaturesFromBytes(yaml)
	if err != nil {
		t.Fatalf("LoadClassFeaturesFromBytes: %v", err)
	}

	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	f := features[0]
	if len(f.PassiveBonuses) != 0 {
		t.Errorf("PassiveBonuses: got %d entries, want 0", len(f.PassiveBonuses))
	}
}
