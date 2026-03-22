package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/npc"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)

// TestAnimalDeathDropsOrganicLoot verifies GenerateOrganicLoot is called
// for animal NPC loot tables.
func TestAnimalDeathDropsOrganicLoot(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "dog_meat", Weight: 10, QuantityMin: 1, QuantityMax: 2},
		},
	}
	result := npc.GenerateOrganicLoot(lt)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 organic item, got %d", len(result.Items))
	}
	if result.Items[0].ItemDefID != "dog_meat" {
		t.Errorf("expected dog_meat, got %q", result.Items[0].ItemDefID)
	}
}

// TestRobotDeathDropsSalvageLoot verifies GenerateSalvageLoot is called
// for robot NPC loot tables.
func TestRobotDeathDropsSalvageLoot(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"circuit_board", "power_cell"},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	result := npc.GenerateSalvageLoot(lt)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 salvage item, got %d", len(result.Items))
	}
	itemID := result.Items[0].ItemDefID
	if itemID != "circuit_board" && itemID != "power_cell" {
		t.Errorf("unexpected item ID %q", itemID)
	}
}

// TestAnimalSayFiltering verifies that "say" actions are stripped for animals.
func TestAnimalSayFiltering(t *testing.T) {
	actions := []ai.PlannedAction{
		{Action: "attack", Target: "player"},
		{Action: "say"},
		{Action: "pass"},
	}
	result := gameserver.ExportedFilterAnimalPlanActions(actions, true)
	for _, a := range result {
		if a.Action == "say" {
			t.Error("expected 'say' action to be filtered for animal")
		}
	}
	if len(result) != 2 {
		t.Errorf("expected 2 actions after filtering, got %d", len(result))
	}
}

// TestNonAnimalSayRetained verifies that "say" actions are kept for non-animals.
func TestNonAnimalSayRetained(t *testing.T) {
	actions := []ai.PlannedAction{
		{Action: "attack", Target: "player"},
		{Action: "say"},
	}
	result := gameserver.ExportedFilterAnimalPlanActions(actions, false)
	if len(result) != 2 {
		t.Errorf("expected 2 actions for non-animal, got %d", len(result))
	}
}
