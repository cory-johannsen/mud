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

// TestCrossValidate_OnboardingQuestNoNPC verifies onboarding quests pass CrossValidate without a matching NPC.
func TestCrossValidate_OnboardingQuestNoNPC(t *testing.T) {
	reg := quest.QuestRegistry{
		"onboarding_find_zone_map": &quest.QuestDef{
			ID:    "onboarding_find_zone_map",
			Title: "Find Your Bearings",
			Type:  "onboarding",
			Objectives: []quest.QuestObjective{
				{ID: "explore_map_room", Type: "explore", Description: "Locate the terminal", TargetID: "flats_82nd_ave", Quantity: 1},
				{ID: "use_zone_map", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
			},
			Rewards: quest.QuestRewards{XP: 50},
		},
	}
	npcIDs := map[string]bool{}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{"flats_82nd_ave": true}
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err != nil {
		t.Fatalf("unexpected error for onboarding CrossValidate: %v", err)
	}
}

// TestCrossValidate_UseZoneMapTargetNotValidated verifies use_zone_map target_id is not validated against roomIDs.
func TestCrossValidate_UseZoneMapTargetNotValidated(t *testing.T) {
	reg := quest.QuestRegistry{
		"onboarding_find_zone_map": &quest.QuestDef{
			ID:    "onboarding_find_zone_map",
			Title: "Find Your Bearings",
			Type:  "onboarding",
			Objectives: []quest.QuestObjective{
				{ID: "explore_map_room", Type: "explore", Description: "Locate the terminal", TargetID: "flats_82nd_ave", Quantity: 1},
				{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
			},
			Rewards: quest.QuestRewards{XP: 50},
		},
	}
	npcIDs := map[string]bool{}
	itemIDs := map[string]bool{}
	roomIDs := map[string]bool{"flats_82nd_ave": true} // "felony_flats" intentionally absent
	if err := reg.CrossValidate(npcIDs, itemIDs, roomIDs); err != nil {
		t.Fatalf("unexpected CrossValidate error: %v", err)
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
		"onboarding_find_zone_map",
	}
	for _, id := range want {
		if reg[id] == nil {
			t.Errorf("missing quest %q", id)
		}
	}
}
