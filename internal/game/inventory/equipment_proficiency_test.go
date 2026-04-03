package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// TestComputedDefensesWithProficiencies_TrainedCategory_AppliesBonusNoPenalty verifies
// that a player trained in the armor's proficiency category receives the proficiency
// bonus added to ACBonus and does NOT incur the check penalty.
func TestComputedDefensesWithProficiencies_TrainedCategory_AppliesBonusNoPenalty(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:                  "medium_chest",
		Name:                "Medium Chest",
		Slot:                inventory.SlotTorso,
		Group:               "composite",
		ACBonus:             4,
		DexCap:              2,
		CheckPenalty:        -2,
		SpeedPenalty:        -5,
		StrengthReq:         14,
		ProficiencyCategory: "medium_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(def))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "medium_chest",
		Name:       "Medium Chest",
		Durability: 10,
	}

	profs := map[string]string{
		"medium_armor": "trained",
	}
	level := 3

	stats := eq.ComputedDefensesWithProficiencies(reg, 4, profs, level)

	// Proficiency bonus for trained at level 3 = level + 2 = 5.
	// Base ACBonus = 4; total = 4 + 5 = 9.
	assert.Equal(t, 9, stats.ACBonus, "ACBonus must include proficiency bonus of level+2=5")
	// Trained: check penalty must NOT be applied.
	assert.Equal(t, 0, stats.CheckPenalty, "trained armor must not incur check penalty")
	// Trained: speed penalty must NOT be applied.
	assert.Equal(t, 0, stats.SpeedPenalty, "trained armor must not incur speed penalty")
}

// TestComputedDefensesWithProficiencies_UntrainedCategory_AppliesPenaltyNoBonus verifies
// that a player with no proficiency in the armor's category suffers check and speed
// penalties and receives no proficiency bonus.
func TestComputedDefensesWithProficiencies_UntrainedCategory_AppliesPenaltyNoBonus(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:                  "medium_chest2",
		Name:                "Medium Chest 2",
		Slot:                inventory.SlotTorso,
		Group:               "composite",
		ACBonus:             4,
		DexCap:              2,
		CheckPenalty:        -2,
		SpeedPenalty:        -5,
		StrengthReq:         14,
		ProficiencyCategory: "medium_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(def))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "medium_chest2",
		Name:       "Medium Chest 2",
		Durability: 10,
	}

	// No medium_armor proficiency.
	profs := map[string]string{}
	level := 3

	stats := eq.ComputedDefensesWithProficiencies(reg, 4, profs, level)

	// No proficiency bonus: ACBonus = base 4 only.
	assert.Equal(t, 4, stats.ACBonus, "untrained armor must not add proficiency bonus")
	// Untrained: check penalty must be applied.
	assert.Equal(t, -2, stats.CheckPenalty, "untrained armor must apply check penalty")
	// Untrained: speed penalty must be applied.
	assert.Equal(t, -5, stats.SpeedPenalty, "untrained armor must apply speed penalty")
}
