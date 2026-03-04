package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadFeats_ParsesAllFeats(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	if len(feats) == 0 {
		t.Fatal("expected non-empty feats list")
	}
	var found bool
	for _, f := range feats {
		if f.ID == "toughness" && f.Category == "general" && !f.Active {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'toughness' general feat")
	}
}

func TestLoadFeats_SkillFeatHasSkillField(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Category == "skill" && f.Skill == "" {
			t.Errorf("skill feat %q has empty Skill field", f.ID)
		}
	}
}

func TestLoadFeats_ActiveFeatHasActivateText(t *testing.T) {
	feats, err := ruleset.LoadFeats("../../../content/feats.yaml")
	if err != nil {
		t.Fatalf("LoadFeats: %v", err)
	}
	for _, f := range feats {
		if f.Active && f.ActivateText == "" {
			t.Errorf("active feat %q has empty ActivateText", f.ID)
		}
	}
}

func TestFeatRegistry_LookupByID(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	f, ok := reg.Feat("toughness")
	if !ok {
		t.Fatal("expected to find toughness in registry")
	}
	if f.Name != "Toughness" {
		t.Errorf("expected Name=Toughness got %q", f.Name)
	}
}

func TestFeatRegistry_ByCategory(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	generals := reg.ByCategory("general")
	if len(generals) == 0 {
		t.Error("expected non-empty general feats")
	}
	for _, f := range generals {
		if f.Category != "general" {
			t.Errorf("ByCategory(general) returned feat with Category=%q", f.Category)
		}
	}
}

func TestFeatRegistry_BySkill(t *testing.T) {
	feats, _ := ruleset.LoadFeats("../../../content/feats.yaml")
	reg := ruleset.NewFeatRegistry(feats)
	parkour := reg.BySkill("parkour")
	if len(parkour) == 0 {
		t.Error("expected parkour skill feats")
	}
}
