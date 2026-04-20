package quest_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/quest"
	"pgregory.net/rapid"
)

func TestQuestDef_Validate_EmptyID(t *testing.T) {
	d := quest.QuestDef{Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestQuestDef_Validate_EmptyTitle(t *testing.T) {
	d := quest.QuestDef{ID: "q1", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty Title")
	}
}

func TestQuestDef_Validate_EmptyGiverNPCID(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty GiverNPCID")
	}
}

func TestQuestDef_Validate_NoObjectives(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1"}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty Objectives")
	}
}

func TestQuestDef_Validate_InvalidObjectiveType(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "dance", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for invalid objective type")
	}
}

func TestQuestDef_Validate_DeliverWithoutItemID(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "deliver", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for deliver without ItemID")
	}
}

func TestQuestDef_Validate_QuantityZero(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 0}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for Quantity < 1")
	}
}

func TestQuestDef_Validate_CooldownOnNonRepeatable(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Repeatable: false, Cooldown: "1h", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for cooldown on non-repeatable quest")
	}
}

func TestQuestDef_Validate_InvalidCooldownDuration(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Repeatable: true, Cooldown: "notaduration", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for invalid cooldown duration")
	}
}

func TestQuestDef_Validate_ValidKill(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", Description: "d", TargetID: "t", Quantity: 1}}}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuestDef_Validate_ValidDeliver(t *testing.T) {
	d := quest.QuestDef{ID: "q1", Title: "T", GiverNPCID: "npc1", Objectives: []quest.QuestObjective{{ID: "o1", Type: "deliver", Description: "d", TargetID: "npc1", ItemID: "item1", Quantity: 2}}}
	if err := d.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestQuestDef_FindTrainer_ValidatesWithoutGiverNPCOrObjectives verifies that a
// quest of type "find_trainer" passes Validate() with no GiverNPCID and no objectives.
//
// Precondition: QuestDef with Type "find_trainer", no GiverNPCID, no Objectives.
// Postcondition: Validate returns nil.
func TestQuestDef_FindTrainer_ValidatesWithoutGiverNPCOrObjectives(t *testing.T) {
	def := quest.QuestDef{
		ID:           "find_neural_trainer_vantucky",
		Title:        "Neural Training Available",
		Description:  "Find a trainer in Vantucky.",
		Type:         "find_trainer",
		AutoComplete: true,
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error for find_trainer quest: %v", err)
	}
}

// TestQuestDef_FindTrainer_CrossValidate_SkipsNPCCheck verifies that CrossValidate
// does not require GiverNPCID to exist in npcIDs for find_trainer quests.
//
// Precondition: QuestRegistry with a find_trainer quest; empty npcIDs map.
// Postcondition: CrossValidate returns nil.
func TestQuestDef_FindTrainer_CrossValidate_SkipsNPCCheck(t *testing.T) {
	reg := quest.QuestRegistry{
		"find_neural_trainer_vantucky": &quest.QuestDef{
			ID:           "find_neural_trainer_vantucky",
			Title:        "Neural Training Available",
			Description:  "Find a trainer.",
			Type:         "find_trainer",
			AutoComplete: true,
		},
	}
	err := reg.CrossValidate(map[string]bool{}, map[string]bool{}, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error for find_trainer CrossValidate: %v", err)
	}
}

// TestValidate_UseZoneMapObjective verifies use_zone_map objective type passes Validate.
func TestValidate_UseZoneMapObjective(t *testing.T) {
	def := quest.QuestDef{
		ID:         "test_use_zone_map",
		Title:      "Test Quest",
		GiverNPCID: "some_npc",
		Repeatable: false,
		Objectives: []quest.QuestObjective{
			{
				ID:          "use_map_obj",
				Type:        "use_zone_map",
				Description: "Use the zone map",
				TargetID:    "felony_flats",
				Quantity:    1,
			},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error for use_zone_map objective: %v", err)
	}
}

// TestValidate_OnboardingQuestNoGiver verifies onboarding quests pass Validate without GiverNPCID.
func TestValidate_OnboardingQuestNoGiver(t *testing.T) {
	def := quest.QuestDef{
		ID:    "onboarding_test",
		Title: "Onboarding Quest",
		Type:  "onboarding",
		Objectives: []quest.QuestObjective{
			{
				ID:          "obj1",
				Type:        "use_zone_map",
				Description: "Use the zone map",
				TargetID:    "felony_flats",
				Quantity:    1,
			},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("unexpected error for onboarding quest with no giver: %v", err)
	}
}

func TestQuestDef_Validate_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		qty := rapid.IntRange(1, 100).Draw(rt, "qty")
		d := quest.QuestDef{
			ID:         rapid.StringN(1, 20, -1).Draw(rt, "id"),
			Title:      rapid.StringN(1, 20, -1).Draw(rt, "title"),
			GiverNPCID: rapid.StringN(1, 20, -1).Draw(rt, "giver"),
			Objectives: []quest.QuestObjective{{
				ID:          rapid.StringN(1, 20, -1).Draw(rt, "oid"),
				Type:        "kill",
				Description: rapid.StringN(1, 20, -1).Draw(rt, "desc"),
				TargetID:    rapid.StringN(1, 20, -1).Draw(rt, "target"),
				Quantity:    qty,
			}},
		}
		if err := d.Validate(); err != nil {
			rt.Fatalf("valid QuestDef failed Validate: %v", err)
		}
	})
}
