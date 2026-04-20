package ai_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestAIChainsaw_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal ai_chainsaw.yaml: %v", err)
	}
	if def.ID != "ai_chainsaw" {
		t.Errorf("expected id ai_chainsaw, got %q", def.ID)
	}
	if def.CombatDomain != "ai_chainsaw_combat" {
		t.Errorf("expected combat_domain ai_chainsaw_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
}

func TestAIChainsawDomain_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw_combat.yaml: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal ai_chainsaw_combat.yaml: %v", err)
	}
	if df.Domain.ID != "ai_chainsaw_combat" {
		t.Errorf("expected domain id ai_chainsaw_combat, got %q", df.Domain.ID)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Errorf("domain Validate(): %v", err)
	}
}

func TestAIAK47_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/items/ai_ak47.yaml")
	if err != nil {
		t.Fatalf("read ai_ak47.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal ai_ak47.yaml: %v", err)
	}
	if def.ID != "ai_ak47" {
		t.Errorf("expected id ai_ak47, got %q", def.ID)
	}
	if def.CombatDomain != "ai_ak47_combat" {
		t.Errorf("expected combat_domain ai_ak47_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
}

func TestAIAK47Domain_LoadsWithoutError(t *testing.T) {
	data, err := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	if err != nil {
		t.Fatalf("read ai_ak47_combat.yaml: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal ai_ak47_combat.yaml: %v", err)
	}
	if df.Domain.ID != "ai_ak47_combat" {
		t.Errorf("expected domain id ai_ak47_combat, got %q", df.Domain.ID)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Errorf("domain Validate(): %v", err)
	}
}
