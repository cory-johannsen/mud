package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestComputedDefenses_Resistances_MaxWins(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
		ID: "vest1", Name: "Vest1", Slot: inventory.SlotTorso, Group: "leather",
		ProficiencyCategory: "light_armor",
		Resistances:         map[string]int{"fire": 3, "piercing": 2},
	}))
	require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
		ID: "vest2", Name: "Vest2", Slot: inventory.SlotLeftArm, Group: "leather",
		ProficiencyCategory: "light_armor",
		Resistances:         map[string]int{"fire": 5},
	}))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "vest1", Name: "Vest1"}
	eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{ItemDefID: "vest2", Name: "Vest2"}
	def := eq.ComputedDefenses(reg, 2)
	assert.Equal(t, 5, def.Resistances["fire"])
	assert.Equal(t, 2, def.Resistances["piercing"])
}

func TestComputedDefenses_Weaknesses_Additive(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
		ID: "a1", Name: "A1", Slot: inventory.SlotTorso, Group: "leather",
		ProficiencyCategory: "light_armor",
		Weaknesses:          map[string]int{"electricity": 2},
	}))
	require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
		ID: "a2", Name: "A2", Slot: inventory.SlotLeftArm, Group: "leather",
		ProficiencyCategory: "light_armor",
		Weaknesses:          map[string]int{"electricity": 3},
	}))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "a1", Name: "A1"}
	eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{ItemDefID: "a2", Name: "A2"}
	def := eq.ComputedDefenses(reg, 2)
	assert.Equal(t, 5, def.Weaknesses["electricity"])
}

func TestComputedDefenses_NoResistances_EmptyMaps(t *testing.T) {
	reg := inventory.NewRegistry()
	eq := inventory.NewEquipment()
	def := eq.ComputedDefenses(reg, 2)
	assert.Empty(t, def.Resistances)
	assert.Empty(t, def.Weaknesses)
}

func TestProperty_ComputedDefenses_ResistancesMaxPerType(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		eq := inventory.NewEquipment()
		r1 := rapid.IntRange(0, 10).Draw(rt, "r1")
		r2 := rapid.IntRange(0, 10).Draw(rt, "r2")
		require.NoError(rt, reg.RegisterArmor(&inventory.ArmorDef{
			ID: "s1", Name: "S1", Slot: inventory.SlotTorso, Group: "leather",
			ProficiencyCategory: "light_armor",
			Resistances:         map[string]int{"cold": r1},
		}))
		require.NoError(rt, reg.RegisterArmor(&inventory.ArmorDef{
			ID: "s2", Name: "S2", Slot: inventory.SlotLeftArm, Group: "leather",
			ProficiencyCategory: "light_armor",
			Resistances:         map[string]int{"cold": r2},
		}))
		eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "s1", Name: "S1"}
		eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{ItemDefID: "s2", Name: "S2"}
		def := eq.ComputedDefenses(reg, 3)
		want := r1
		if r2 > want {
			want = r2
		}
		assert.Equal(rt, want, def.Resistances["cold"])
	})
}

func TestProperty_ComputedDefenses_WeaknessesAdditive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		eq := inventory.NewEquipment()
		w1 := rapid.IntRange(0, 10).Draw(rt, "w1")
		w2 := rapid.IntRange(0, 10).Draw(rt, "w2")
		require.NoError(rt, reg.RegisterArmor(&inventory.ArmorDef{
			ID: "w1a", Name: "W1A", Slot: inventory.SlotTorso, Group: "leather",
			ProficiencyCategory: "light_armor",
			Weaknesses:          map[string]int{"acid": w1},
		}))
		require.NoError(rt, reg.RegisterArmor(&inventory.ArmorDef{
			ID: "w2a", Name: "W2A", Slot: inventory.SlotLeftArm, Group: "leather",
			ProficiencyCategory: "light_armor",
			Weaknesses:          map[string]int{"acid": w2},
		}))
		eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "w1a", Name: "W1A"}
		eq.Armor[inventory.SlotLeftArm] = &inventory.SlottedItem{ItemDefID: "w2a", Name: "W2A"}
		def := eq.ComputedDefenses(reg, 3)
		assert.Equal(rt, w1+w2, def.Weaknesses["acid"])
	})
}
