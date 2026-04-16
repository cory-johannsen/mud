package quest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/quest"
)

func writeQuestYAML(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestQuestRegistry_LoadFromDir_ValidQuest(t *testing.T) {
	dir := t.TempDir()
	writeQuestYAML(t, dir, "q1.yaml", `
id: q1
title: Kill Some Rats
description: There are rats.
giver_npc_id: npc1
objectives:
  - id: kill_rats
    type: kill
    description: Kill 3 rats
    target_id: rat
    quantity: 3
rewards:
  xp: 100
  credits: 50
`)
	reg, err := quest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := reg["q1"]; !ok {
		t.Fatal("expected quest q1 to be loaded")
	}
}

func TestQuestRegistry_LoadFromDir_InvalidQuestErrors(t *testing.T) {
	dir := t.TempDir()
	writeQuestYAML(t, dir, "bad.yaml", `
id: ""
title: ""
giver_npc_id: npc1
objectives: []
`)
	_, err := quest.LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid quest")
	}
}

func TestQuestRegistry_LoadFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	reg, err := quest.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error on empty dir: %v", err)
	}
	if len(reg) != 0 {
		t.Fatalf("expected empty registry, got %d entries", len(reg))
	}
}

func TestQuestRegistry_CrossValidate_UnknownPrerequisite(t *testing.T) {
	reg := quest.QuestRegistry{
		"q1": {ID: "q1", Title: "T", GiverNPCID: "npc1",
			Prerequisites: []string{"unknown_quest"},
			Objectives:    []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}},
		},
	}
	npcIDs := map[string]bool{"npc1": true}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{}
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err == nil {
		t.Fatal("expected error for unknown prerequisite")
	}
}

func TestQuestRegistry_CrossValidate_UnknownKillTargetNPC(t *testing.T) {
	reg := quest.QuestRegistry{
		"q1": {ID: "q1", Title: "T", GiverNPCID: "npc1",
			Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "unknown_npc", Quantity: 1}},
		},
	}
	npcIDs := map[string]bool{"npc1": true}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{}
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err == nil {
		t.Fatal("expected error for unknown kill target NPC")
	}
}

func TestQuestRegistry_CrossValidate_Valid(t *testing.T) {
	reg := quest.QuestRegistry{
		"q1": {ID: "q1", Title: "T", GiverNPCID: "npc1",
			Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "rat", Quantity: 1}},
		},
	}
	npcIDs := map[string]bool{"npc1": true, "rat": true}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{}
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLoadFromDir_AllZoneQuests verifies that all zone quests and find-trainer quests
// load successfully from content/quests.
//
// Precondition: quest YAML files exist in ../../../content/quests.
// Postcondition: registry contains all expected quest IDs.
func TestLoadFromDir_AllZoneQuests(t *testing.T) {
	reg, err := quest.LoadFromDir("../../../content/quests")
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	want := []string{
		"rrq_scavenger_sweep",
		"rrq_rail_gang_bounty",
		"rrq_barrel_house_cleanup",
		"rrq_take_down_big_grizz",
		"vtq_militia_patrol",
		"vtq_scavenger_drive",
		"vtq_bandit_bounty",
		"vtq_gang_enforcer_takedown",
		"find_technical_trainer_vantucky",
		"find_neural_trainer_vantucky",
		"find_biosynthetic_trainer_vantucky",
		"find_fanatic_trainer_vantucky",
		"find_technical_trainer_ne_portland",
		"find_neural_trainer_ne_portland",
		"find_biosynthetic_trainer_ne_portland",
		"find_fanatic_trainer_ne_portland",
	}
	for _, id := range want {
		if reg[id] == nil {
			t.Errorf("missing quest %q", id)
		}
	}
}
