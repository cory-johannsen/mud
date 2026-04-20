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

// TestAIChainsaw_KillCountEscalates verifies that after 2 kills in an encounter,
// the frenzy method is selected over the standard hunt method.
func TestAIChainsaw_KillCountEscalates(t *testing.T) {
	// Load item def to get the combat script
	data, err := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	if err != nil {
		t.Fatalf("read ai_chainsaw.yaml: %v", err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Load domain
	domData, err := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	if err != nil {
		t.Fatalf("read domain: %v", err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(domData, &df); err != nil {
		t.Fatalf("unmarshal domain: %v", err)
	}

	planner := ai.NewItemPlanner(&df.Domain)

	// state with 2 kills already recorded — frenzy should activate
	state := map[string]interface{}{"kills": float64(2)}

	var attackFormula string
	var attackCost int
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(targetID, formula string, cost int) bool {
			attackFormula = formula
			attackCost = cost
			return true
		},
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}

	snapshot := ai.ItemCombatSnapshot{
		Player: ai.ItemPlayerSnapshot{ID: "player1", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{
			{ID: "enemy1", HP: 20, MaxHP: 40},
		},
	}

	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// frenzy_active is true (kills=2), so overkill_strike fires
	if attackFormula != "2d6+4" {
		t.Errorf("expected overkill formula 2d6+4, got %q", attackFormula)
	}
	if attackCost != 2 {
		t.Errorf("expected overkill AP cost 2, got %d", attackCost)
	}
}

// TestAIChainsaw_OverkillCosts2AP verifies frenzy operator passes cost=2 to Attack.
func TestAIChainsaw_OverkillCosts2AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(&df.Domain)

	state := map[string]interface{}{"kills": float64(2)}
	spent := 0
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { spent += cost; return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 10, MaxHP: 20}},
	}
	planner.Execute(def.CombatScript, state, snapshot, cbs)
	if spent != 2 {
		t.Errorf("expected overkill to pass cost 2 to Attack, got %d", spent)
	}
}

// TestAIChainsaw_IdleFallback verifies that with no enemies, say_hungry fires and costs 0 AP.
func TestAIChainsaw_IdleFallback(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_chainsaw.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_chainsaw_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(&df.Domain)

	state := map[string]interface{}{}
	said := false
	spent := 0
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { return false },
		Say:     func(pool []string) { said = true },
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { spent += n; return true },
	}
	// No enemies
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !said {
		t.Error("expected say_hungry to fire with no enemies")
	}
	if spent != 0 {
		t.Errorf("expected idle to spend 0 AP, got %d", spent)
	}
}

// TestAIAK47_FullSequenceAt3AP verifies burst_and_theorize fires when AP >= 3.
func TestAIAK47_FullSequenceAt3AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(&df.Domain)

	state := map[string]interface{}{}
	var attackFormula string
	var attackCost int
	debuffCalled := false
	cbs := ai.ItemPrimitiveCalls{
		Attack: func(targetID, formula string, cost int) bool {
			attackFormula = formula
			attackCost = cost
			return true
		},
		Say:    func(pool []string) {},
		Buff:   func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff: func(targetID, effectID string, rounds, cost int) bool { debuffCalled = true; return true },
		GetAP:  func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30, MaxHP: 60}},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if attackFormula != "2d6" {
		t.Errorf("expected burst formula 2d6, got %q", attackFormula)
	}
	if attackCost != 2 {
		t.Errorf("expected burst attack cost 2, got %d", attackCost)
	}
	if !debuffCalled {
		t.Error("expected fear debuff to be applied")
	}
}

// TestAIAK47_QuickShotBelow3AP verifies quick_shot fires when AP < 3.
func TestAIAK47_QuickShotBelow3AP(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(&df.Domain)

	state := map[string]interface{}{}
	var attackFormula string
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { attackFormula = formula; return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { return true },
		GetAP:   func() int { return 2 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 2},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30, MaxHP: 60}},
	}
	if err := planner.Execute(def.CombatScript, state, snapshot, cbs); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if attackFormula != "1d6+1" {
		t.Errorf("expected quick_shot formula 1d6+1, got %q", attackFormula)
	}
}

// TestAIAK47_FearDebuffDuration verifies fear condition lasts exactly 2 rounds.
func TestAIAK47_FearDebuffDuration(t *testing.T) {
	data, _ := os.ReadFile("../../../content/items/ai_ak47.yaml")
	var def inventory.ItemDef
	yaml.Unmarshal(data, &def)

	domData, _ := os.ReadFile("../../../content/ai/ai_ak47_combat.yaml")
	type domainFile struct{ Domain ai.Domain `yaml:"domain"` }
	var df domainFile
	yaml.Unmarshal(domData, &df)
	planner := ai.NewItemPlanner(&df.Domain)

	state := map[string]interface{}{}
	var debuffRounds int
	cbs := ai.ItemPrimitiveCalls{
		Attack:  func(targetID, formula string, cost int) bool { return true },
		Say:     func(pool []string) {},
		Buff:    func(targetID, effectID string, rounds, cost int) bool { return true },
		Debuff:  func(targetID, effectID string, rounds, cost int) bool { debuffRounds = rounds; return true },
		GetAP:   func() int { return 3 },
		SpendAP: func(n int) bool { return true },
	}
	snapshot := ai.ItemCombatSnapshot{
		Player:  ai.ItemPlayerSnapshot{ID: "p", HP: 50, AP: 3},
		Enemies: []ai.ItemEnemySnapshot{{ID: "e1", HP: 30, MaxHP: 60}},
	}
	planner.Execute(def.CombatScript, state, snapshot, cbs)
	if debuffRounds != 2 {
		t.Errorf("expected fear debuff to last 2 rounds, got %d", debuffRounds)
	}
}
