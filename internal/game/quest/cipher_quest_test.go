package quest_test

import (
	"os"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/quest"
	"gopkg.in/yaml.v3"
)

type cipherNPC struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	NPCType string `yaml:"npc_type"`
	NPCRole string `yaml:"npc_role"`
	QuestGiver struct {
		QuestIDs []string `yaml:"quest_ids"`
	} `yaml:"quest_giver"`
}

func TestCipherNPC_LoadsWithExpectedFields(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/cipher.yaml")
	if err != nil {
		t.Fatalf("read cipher.yaml: %v", err)
	}
	var npc cipherNPC
	if err := yaml.Unmarshal(data, &npc); err != nil {
		t.Fatalf("unmarshal cipher.yaml: %v", err)
	}
	if npc.ID != "cipher" {
		t.Errorf("expected id cipher, got %q", npc.ID)
	}
	if npc.NPCType != "quest_giver" {
		t.Errorf("expected npc_type quest_giver, got %q", npc.NPCType)
	}
	if len(npc.QuestGiver.QuestIDs) < 2 {
		t.Errorf("expected at least 2 quest_ids, got %d", len(npc.QuestGiver.QuestIDs))
	}
}

type zoneRoom struct {
	ID     string `yaml:"id"`
	Spawns []struct {
		Template string `yaml:"template"`
	} `yaml:"spawns"`
}

type zoneFile struct {
	Zone struct {
		Rooms []zoneRoom `yaml:"rooms"`
	} `yaml:"zone"`
}

func TestVelvetRopeBrothel_HasCipherSpawn(t *testing.T) {
	data, err := os.ReadFile("../../../content/zones/the_velvet_rope.yaml")
	if err != nil {
		t.Fatalf("read the_velvet_rope.yaml: %v", err)
	}
	var zf zoneFile
	if err := yaml.Unmarshal(data, &zf); err != nil {
		t.Fatalf("unmarshal zone: %v", err)
	}
	for _, room := range zf.Zone.Rooms {
		if room.ID != "the_velvet_rope_brothel" {
			continue
		}
		for _, spawn := range room.Spawns {
			if spawn.Template == "cipher" {
				return // found
			}
		}
		t.Fatal("the_velvet_rope_brothel has no cipher spawn")
	}
	t.Fatal("room the_velvet_rope_brothel not found in zone")
}

type npcLoot struct {
	Loot struct {
		Items []struct {
			ItemID string  `yaml:"item"`
			Chance float64 `yaml:"chance"`
		} `yaml:"items"`
	} `yaml:"loot"`
}

func TestGangbang_HasAIItemDrops(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/gangbang.yaml")
	if err != nil {
		t.Fatalf("read gangbang.yaml: %v", err)
	}
	var npc npcLoot
	if err := yaml.Unmarshal(data, &npc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantItems := map[string]bool{
		"ai_chainsaw": false,
		"ai_ak47":     false,
	}
	for _, item := range npc.Loot.Items {
		if _, ok := wantItems[item.ItemID]; ok {
			wantItems[item.ItemID] = true
			if item.Chance != 0.05 {
				t.Errorf("item %q: expected chance 0.05, got %f", item.ItemID, item.Chance)
			}
		}
	}
	for itemID, found := range wantItems {
		if !found {
			t.Errorf("gangbang loot missing item %q", itemID)
		}
	}
}

func TestQuestRegistry_CipherQuestsValid(t *testing.T) {
	questFiles := []string{
		"../../../content/quests/machete_signal_in_static.yaml",
		"../../../content/quests/gun_signal_in_static.yaml",
		"../../../content/quests/machete_field_test.yaml",
		"../../../content/quests/gun_field_test.yaml",
	}
	for _, path := range questFiles {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var def quest.QuestDef
			if err := yaml.Unmarshal(data, &def); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}
			if err := def.Validate(); err != nil {
				t.Errorf("Validate %s: %v", path, err)
			}
		})
	}
}

func TestRustbucketRidgeQuestGiver_HasMacheteSignal(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/rustbucket_ridge_quest_giver.yaml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)
	for _, id := range npc.QuestGiver.QuestIDs {
		if id == "machete_signal_in_static" {
			return
		}
	}
	t.Error("rustbucket_ridge_quest_giver missing machete_signal_in_static quest_id")
}

func TestVantuckyQuestGiver_HasGunSignal(t *testing.T) {
	data, err := os.ReadFile("../../../content/npcs/vantucky_quest_giver.yaml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var npc struct {
		QuestGiver struct {
			QuestIDs []string `yaml:"quest_ids"`
		} `yaml:"quest_giver"`
	}
	yaml.Unmarshal(data, &npc)
	for _, id := range npc.QuestGiver.QuestIDs {
		if id == "gun_signal_in_static" {
			return
		}
	}
	t.Error("vantucky_quest_giver missing gun_signal_in_static quest_id")
}
