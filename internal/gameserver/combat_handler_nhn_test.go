package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
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
