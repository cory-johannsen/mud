package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

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
// penalties, and receives no AC from the item (proficiency required for AC contribution).
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

	// Untrained: no proficiency bonus, no item AC (item AC excluded when untrained).
	assert.Equal(t, 0, stats.ACBonus, "untrained armor must not contribute AC bonus")
	// Untrained: check penalty must be applied.
	assert.Equal(t, -2, stats.CheckPenalty, "untrained armor must apply check penalty")
	// Untrained: speed penalty must be applied.
	assert.Equal(t, -5, stats.SpeedPenalty, "untrained armor must apply speed penalty")
}

// TestComputedDefensesWithProficiencies_MixedLightMedium_ProfBonusAppliedOnce verifies
// that when multiple armor slots are worn and proficiency applies, the proficiency bonus
// is added exactly once (for the heaviest proficient category), not once per slot.
func TestComputedDefensesWithProficiencies_MixedLightMedium_ProfBonusAppliedOnce(t *testing.T) {
	reg := inventory.NewRegistry()
	lightDef := &inventory.ArmorDef{
		ID:                  "light_arms",
		Name:                "Light Arms",
		Slot:                inventory.SlotLeftArm,
		Group:               "leather",
		ACBonus:             1,
		DexCap:              10,
		ProficiencyCategory: "light_armor",
		Rarity:              "street",
	}
	medDef := &inventory.ArmorDef{
		ID:                  "medium_torso",
		Name:                "Medium Torso",
		Slot:                inventory.SlotTorso,
		Group:               "composite",
		ACBonus:             3,
		DexCap:              10,
		ProficiencyCategory: "medium_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(lightDef))
	require.NoError(t, reg.RegisterArmor(medDef))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{
		ItemDefID:  "light_arms",
		Name:       "Light Arms",
		Durability: 10,
	}
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "medium_torso",
		Name:       "Medium Torso",
		Durability: 10,
	}

	profs := map[string]string{
		"light_armor":  "trained",
		"medium_armor": "trained",
	}
	// Level 8: trained bonus = 8 + 2 = 10.
	level := 8

	stats := eq.ComputedDefensesWithProficiencies(reg, 4, profs, level)

	// Item AC: 1 (light) + 3 (medium) = 4.
	// Proficiency bonus: 10 (trained at level 8), applied ONCE for heaviest = medium_armor.
	// Total ACBonus = 4 + 10 = 14.
	assert.Equal(t, 14, stats.ACBonus, "ACBonus must be item sum (4) + proficiency bonus once (10)")
	assert.Equal(t, 10, stats.ProficiencyACBonus, "ProficiencyACBonus must be 10 (level+2 trained)")
	assert.Equal(t, "medium_armor", stats.EffectiveArmorCategory, "EffectiveArmorCategory must be heaviest proficient category")
}

// TestComputedDefensesWithProficiencies_UnproficientSlot_ItemBonusExcluded verifies that
// when the single worn armor slot has no proficiency, item AC is excluded entirely, and
// CheckPenalty and SpeedPenalty are still applied.
func TestComputedDefensesWithProficiencies_UnproficientSlot_ItemBonusExcluded(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:                  "heavy_chest_np",
		Name:                "Heavy Chest",
		Slot:                inventory.SlotTorso,
		Group:               "plate",
		ACBonus:             3,
		DexCap:              1,
		CheckPenalty:        -3,
		SpeedPenalty:        -10,
		StrengthReq:         16,
		ProficiencyCategory: "heavy_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(def))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "heavy_chest_np",
		Name:       "Heavy Chest",
		Durability: 10,
	}

	// Unproficient: rank is empty.
	profs := map[string]string{}
	level := 5

	stats := eq.ComputedDefensesWithProficiencies(reg, 3, profs, level)

	assert.Equal(t, 0, stats.ACBonus, "unproficient slot must contribute no AC")
	assert.Equal(t, 0, stats.ProficiencyACBonus, "ProficiencyACBonus must be 0 when no proficient slots")
	// Penalties still apply even without proficiency.
	assert.Equal(t, -3, stats.CheckPenalty, "check penalty must still be applied for unproficient armor")
	assert.Equal(t, -10, stats.SpeedPenalty, "speed penalty must still be applied for unproficient armor")
}

// TestComputedDefensesWithProficiencies_NoProficientSlots_ZeroACBonus verifies that
// when the worn armor slot has no proficiency rank, ACBonus and ProficiencyACBonus are
// both zero, and EffectiveArmorCategory is "unarmored".
func TestComputedDefensesWithProficiencies_NoProficientSlots_ZeroACBonus(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:                  "light_vest_np",
		Name:                "Light Vest",
		Slot:                inventory.SlotTorso,
		Group:               "leather",
		ACBonus:             2,
		DexCap:              10,
		CheckPenalty:        0,
		SpeedPenalty:        0,
		ProficiencyCategory: "light_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(def))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "light_vest_np",
		Name:       "Light Vest",
		Durability: 10,
	}

	profs := map[string]string{} // no proficiencies
	level := 2

	stats := eq.ComputedDefensesWithProficiencies(reg, 3, profs, level)

	assert.Equal(t, 0, stats.ACBonus, "ACBonus must be 0 with no proficient slots")
	assert.Equal(t, 0, stats.ProficiencyACBonus, "ProficiencyACBonus must be 0 with no proficient slots")
	assert.Equal(t, "unarmored", stats.EffectiveArmorCategory, "EffectiveArmorCategory must be unarmored when no proficient slots")
}

// TestComputedDefensesWithProficiencies_SingleProficientSlot_ProfBonusOnce verifies that
// a single proficient slot produces the correct ACBonus with proficiency bonus applied once.
func TestComputedDefensesWithProficiencies_SingleProficientSlot_ProfBonusOnce(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:                  "light_arm_single",
		Name:                "Light Arm",
		Slot:                inventory.SlotLeftArm,
		Group:               "leather",
		ACBonus:             1,
		DexCap:              10,
		ProficiencyCategory: "light_armor",
		Rarity:              "street",
	}
	require.NoError(t, reg.RegisterArmor(def))

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{
		ItemDefID:  "light_arm_single",
		Name:       "Light Arm",
		Durability: 10,
	}

	profs := map[string]string{
		"light_armor": "trained",
	}
	// Level 4: trained bonus = 4 + 2 = 6.
	level := 4

	stats := eq.ComputedDefensesWithProficiencies(reg, 3, profs, level)

	// Item AC: 1. Proficiency: 6. Total: 7.
	assert.Equal(t, 7, stats.ACBonus, "ACBonus must be item AC (1) + prof bonus (6) = 7")
	assert.Equal(t, 6, stats.ProficiencyACBonus, "ProficiencyACBonus must be 6 (level+2 trained at level 4)")
}

// TestProperty_ComputedDefensesWithProficiencies_ProfBonusAppearsOnce verifies via
// property-based testing that for any configuration of armor slots and per-category
// proficiency ranks, ProficiencyACBonus appears at most once in ACBonus, and
// ACBonus == sum(AC for slots in proficient categories) + ProficiencyACBonus.
//
// Proficiency is modeled correctly as a character-wide, per-category property:
// all slots in a proficient category benefit from it.
func TestProperty_ComputedDefensesWithProficiencies_ProfBonusAppearsOnce(t *testing.T) {
	allSlots := []inventory.ArmorSlot{
		inventory.SlotHead, inventory.SlotLeftArm, inventory.SlotRightArm, inventory.SlotTorso,
		inventory.SlotHands, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet,
	}
	categories := []string{"light_armor", "medium_armor", "heavy_armor"}
	ranks := []string{"", "trained", "expert", "master", "legendary"}

	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		eq := inventory.NewEquipment()

		level := rapid.IntRange(1, 20).Draw(rt, "level")

		// Draw a per-category proficiency rank for the character (the profs map).
		profs := map[string]string{}
		for _, cat := range categories {
			rankIdx := rapid.IntRange(0, len(ranks)-1).Draw(rt, "rank_"+cat)
			if r := ranks[rankIdx]; r != "" {
				profs[cat] = r
			}
		}

		// For each slot independently decide whether to equip armor.
		// Each slot's category is randomly chosen; proficiency is character-wide via profs.
		slotItemAC := map[inventory.ArmorSlot]int{}
		slotCategory := map[inventory.ArmorSlot]string{}
		for i, slot := range allSlots {
			equip := rapid.Bool().Draw(rt, "equip")
			if !equip {
				continue
			}
			catIdx := rapid.IntRange(0, len(categories)-1).Draw(rt, "catIdx")
			cat := categories[catIdx]
			ac := rapid.IntRange(0, 6).Draw(rt, "ac")

			id := string(slot) + "_prop" + itoa(i)
			def := &inventory.ArmorDef{
				ID:                  id,
				Name:                id,
				Slot:                slot,
				Group:               "leather",
				ACBonus:             ac,
				DexCap:              10,
				ProficiencyCategory: cat,
				Rarity:              "street",
			}
			require.NoError(rt, reg.RegisterArmor(def))
			eq.Armor[slot] = &inventory.SlottedItem{
				ItemDefID:  id,
				Name:       id,
				Durability: 10,
			}
			slotItemAC[slot] = ac
			slotCategory[slot] = cat
		}

		// Compute expected proficientItemACSum: sum of item AC for slots whose category
		// has a non-empty proficiency rank in profs.
		proficientItemACSum := 0
		for slot, cat := range slotCategory {
			if profs[cat] != "" {
				proficientItemACSum += slotItemAC[slot]
			}
		}

		stats := eq.ComputedDefensesWithProficiencies(reg, 5, profs, level)

		// Property: ACBonus == proficientItemACSum + ProficiencyACBonus.
		assert.Equal(rt, proficientItemACSum+stats.ProficiencyACBonus, stats.ACBonus,
			"ACBonus must equal sum of proficient item AC bonuses plus ProficiencyACBonus (applied once)")
	})
}

// itoa converts an int to a string suffix for unique IDs.
func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return string(digits[n/10]) + string(digits[n%10])
}

// rankOrder returns a numeric order for proficiency ranks (higher = better).
func rankOrder(rank string) int {
	switch rank {
	case "legendary":
		return 4
	case "master":
		return 3
	case "expert":
		return 2
	case "trained":
		return 1
	}
	return 0
}
