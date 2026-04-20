package ai_test

import (
	"os"
	"testing"

	"pgregory.net/rapid"
	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func loadItemDef(t *testing.T, path string) inventory.ItemDef {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var def inventory.ItemDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return def
}

func loadDomain(t *testing.T, path string) ai.Domain {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	type domainFile struct {
		Domain ai.Domain `yaml:"domain"`
	}
	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if err := df.Domain.Validate(); err != nil {
		t.Fatalf("domain Validate: %v", err)
	}
	return df.Domain
}

func TestAISawnOff_Loads(t *testing.T) {
	def := loadItemDef(t, "../../../content/items/ai_sawn_off.yaml")
	if def.ID != "ai_sawn_off" {
		t.Errorf("expected id ai_sawn_off, got %q", def.ID)
	}
	if def.CombatDomain != "ai_sawn_off_combat" {
		t.Errorf("expected combat_domain ai_sawn_off_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
	loadDomain(t, "../../../content/ai/ai_sawn_off_combat.yaml")
}

func TestAICombatKnife_Loads(t *testing.T) {
	def := loadItemDef(t, "../../../content/items/ai_combat_knife.yaml")
	if def.ID != "ai_combat_knife" {
		t.Errorf("expected id ai_combat_knife, got %q", def.ID)
	}
	if def.CombatDomain != "ai_combat_knife_combat" {
		t.Errorf("expected combat_domain ai_combat_knife_combat, got %q", def.CombatDomain)
	}
	if def.CombatScript == "" {
		t.Error("combat_script must not be empty")
	}
	loadDomain(t, "../../../content/ai/ai_combat_knife_combat.yaml")
}

func TestArmorItems_AllLoad(t *testing.T) {
	items := []struct {
		itemPath   string
		domainPath string
		itemID     string
		domainID   string
	}{
		{"../../../content/items/ai_machete_armor_light.yaml", "../../../content/ai/ai_machete_armor_light_combat.yaml", "ai_machete_armor_light", "ai_machete_armor_light_combat"},
		{"../../../content/items/ai_machete_armor_medium.yaml", "../../../content/ai/ai_machete_armor_medium_combat.yaml", "ai_machete_armor_medium", "ai_machete_armor_medium_combat"},
		{"../../../content/items/ai_machete_armor_heavy.yaml", "../../../content/ai/ai_machete_armor_heavy_combat.yaml", "ai_machete_armor_heavy", "ai_machete_armor_heavy_combat"},
		{"../../../content/items/ai_gun_armor_light.yaml", "../../../content/ai/ai_gun_armor_light_combat.yaml", "ai_gun_armor_light", "ai_gun_armor_light_combat"},
		{"../../../content/items/ai_gun_armor_medium.yaml", "../../../content/ai/ai_gun_armor_medium_combat.yaml", "ai_gun_armor_medium", "ai_gun_armor_medium_combat"},
		{"../../../content/items/ai_gun_armor_heavy.yaml", "../../../content/ai/ai_gun_armor_heavy_combat.yaml", "ai_gun_armor_heavy", "ai_gun_armor_heavy_combat"},
	}
	for _, tc := range items {
		t.Run(tc.itemID, func(t *testing.T) {
			def := loadItemDef(t, tc.itemPath)
			if def.ID != tc.itemID {
				t.Errorf("expected id %q, got %q", tc.itemID, def.ID)
			}
			if def.CombatDomain != tc.domainID {
				t.Errorf("expected combat_domain %q, got %q", tc.domainID, def.CombatDomain)
			}
			if def.CombatScript == "" {
				t.Error("combat_script must not be empty")
			}
			domain := loadDomain(t, tc.domainPath)
			if domain.ID != tc.domainID {
				t.Errorf("expected domain id %q, got %q", tc.domainID, domain.ID)
			}
		})
	}
}

func TestShieldItems_AllLoad(t *testing.T) {
	items := []struct {
		itemPath   string
		domainPath string
		itemID     string
		domainID   string
	}{
		{"../../../content/items/ai_machete_shield.yaml", "../../../content/ai/ai_machete_shield_combat.yaml", "ai_machete_shield", "ai_machete_shield_combat"},
		{"../../../content/items/ai_gun_shield.yaml", "../../../content/ai/ai_gun_shield_combat.yaml", "ai_gun_shield", "ai_gun_shield_combat"},
	}
	for _, tc := range items {
		t.Run(tc.itemID, func(t *testing.T) {
			def := loadItemDef(t, tc.itemPath)
			if def.ID != tc.itemID {
				t.Errorf("expected id %q, got %q", tc.itemID, def.ID)
			}
			if def.CombatDomain != tc.domainID {
				t.Errorf("expected combat_domain %q, got %q", tc.domainID, def.CombatDomain)
			}
			if def.CombatScript == "" {
				t.Error("combat_script must not be empty")
			}
			domain := loadDomain(t, tc.domainPath)
			if domain.ID != tc.domainID {
				t.Errorf("expected domain id %q, got %q", tc.domainID, domain.ID)
			}
		})
	}
}

func TestExpansionQuests_AllLoad(t *testing.T) {
	questFiles := []string{
		"../../../content/quests/machete_ranged_field_test.yaml",
		"../../../content/quests/gun_melee_field_test.yaml",
		"../../../content/quests/machete_armor_light_quest.yaml",
		"../../../content/quests/machete_armor_medium_quest.yaml",
		"../../../content/quests/machete_armor_heavy_quest.yaml",
		"../../../content/quests/machete_shield_quest.yaml",
		"../../../content/quests/gun_armor_light_quest.yaml",
		"../../../content/quests/gun_armor_medium_quest.yaml",
		"../../../content/quests/gun_armor_heavy_quest.yaml",
		"../../../content/quests/gun_shield_quest.yaml",
	}
	for _, path := range questFiles {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if raw["id"] == nil {
				t.Error("quest missing id field")
			}
			if raw["giver_npc_id"] == nil {
				t.Error("quest missing giver_npc_id field")
			}
		})
	}
}

func TestCipher_HasAllExpansionQuests(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/cipher.yaml")
	if err != nil {
		t.Fatalf("read cipher.yaml: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)

	wantIDs := []string{
		"machete_field_test",
		"gun_field_test",
		"machete_ranged_field_test",
		"gun_melee_field_test",
		"machete_armor_light_quest",
		"machete_armor_medium_quest",
		"machete_armor_heavy_quest",
		"machete_shield_quest",
		"gun_armor_light_quest",
		"gun_armor_medium_quest",
		"gun_armor_heavy_quest",
		"gun_shield_quest",
	}
	idSet := make(map[string]bool, len(npc.QuestGiver.QuestIDs))
	for _, id := range npc.QuestGiver.QuestIDs {
		idSet[id] = true
	}
	for _, want := range wantIDs {
		if !idSet[want] {
			t.Errorf("cipher missing quest_id %q", want)
		}
	}
}

type npcLootCheck struct {
	Loot struct {
		Items []struct {
			ItemID string  `yaml:"item"`
			Chance float64 `yaml:"chance"`
		} `yaml:"items"`
	} `yaml:"loot"`
}

func checkDrops(t *testing.T, npcPath string, wantItemIDs []string) {
	t.Helper()
	data, err := os.ReadFile(npcPath)
	if err != nil {
		t.Fatalf("read %s: %v", npcPath, err)
	}
	var npc npcLootCheck
	yaml.Unmarshal(data, &npc)
	found := make(map[string]bool)
	for _, item := range npc.Loot.Items {
		found[item.ItemID] = true
		if item.Chance != 0.05 {
			for _, want := range wantItemIDs {
				if item.ItemID == want {
					t.Errorf("%s: item %q chance should be 0.05, got %f", npcPath, item.ItemID, item.Chance)
				}
			}
		}
	}
	for _, want := range wantItemIDs {
		if !found[want] {
			t.Errorf("%s: missing loot item %q", npcPath, want)
		}
	}
}

func TestGangbang_HasExpansionDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/gangbang.yaml", []string{
		"ai_sawn_off",
		"ai_combat_knife",
	})
}

func TestTheBig3_HasArmorDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/the_big_3.yaml", []string{
		"ai_machete_armor_light",
		"ai_machete_armor_medium",
		"ai_machete_armor_heavy",
		"ai_gun_armor_light",
		"ai_gun_armor_medium",
		"ai_gun_armor_heavy",
	})
}

func TestPapaWook_HasShieldDrops(t *testing.T) {
	checkDrops(t, "../../../content/npcs/papa_wook.yaml", []string{
		"ai_machete_shield",
		"ai_gun_shield",
	})
}

func TestProperty_AIItemPhase_APNeverGoesNegative(t *testing.T) {
	domainPaths := []string{
		"../../../content/ai/ai_sawn_off_combat.yaml",
		"../../../content/ai/ai_combat_knife_combat.yaml",
		"../../../content/ai/ai_machete_armor_light_combat.yaml",
		"../../../content/ai/ai_machete_armor_medium_combat.yaml",
		"../../../content/ai/ai_machete_armor_heavy_combat.yaml",
		"../../../content/ai/ai_machete_shield_combat.yaml",
		"../../../content/ai/ai_gun_armor_light_combat.yaml",
		"../../../content/ai/ai_gun_armor_medium_combat.yaml",
		"../../../content/ai/ai_gun_armor_heavy_combat.yaml",
		"../../../content/ai/ai_gun_shield_combat.yaml",
	}

	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, len(domainPaths)).Draw(t, "n")
		indices := rapid.SliceOfDistinct(rapid.IntRange(0, len(domainPaths)-1), func(v int) int { return v }).Draw(t, "indices")
		if len(indices) > n {
			indices = indices[:n]
		}

		ap := rapid.IntRange(3, 12).Draw(t, "initial_ap")

		for _, idx := range indices {
			domData, err := os.ReadFile(domainPaths[idx])
			if err != nil {
				t.Skip("domain file not found")
			}
			type domainFile struct {
				Domain ai.Domain `yaml:"domain"`
			}
			var df domainFile
			if err := yaml.Unmarshal(domData, &df); err != nil {
				t.Fatalf("unmarshal domain: %v", err)
			}

			maxCost := 0
			for _, op := range df.Domain.Operators {
				if op.APCost > maxCost {
					maxCost = op.APCost
				}
			}

			if ap >= maxCost && maxCost > 0 {
				ap -= maxCost
			}
			if ap < 0 {
				t.Errorf("AP went negative (%d) after domain %s", ap, df.Domain.ID)
			}
		}
	})
}
