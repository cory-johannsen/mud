package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestLoadSkills_LoadsAll17(t *testing.T) {
	skills, err := ruleset.LoadSkills("../../../content/skills.yaml")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(skills) != 17 {
		t.Fatalf("expected 17 skills, got %d", len(skills))
	}
}

func TestLoadSkills_FieldsPopulated(t *testing.T) {
	skills, err := ruleset.LoadSkills("../../../content/skills.yaml")
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	byID := make(map[string]*ruleset.Skill, len(skills))
	for _, s := range skills {
		byID[s.ID] = s
	}
	parkour, ok := byID["parkour"]
	if !ok {
		t.Fatal("parkour skill not found")
	}
	if parkour.Name != "Parkour" {
		t.Errorf("expected Name=Parkour, got %q", parkour.Name)
	}
	if parkour.Ability != "quickness" {
		t.Errorf("expected Ability=quickness, got %q", parkour.Ability)
	}
}
