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
