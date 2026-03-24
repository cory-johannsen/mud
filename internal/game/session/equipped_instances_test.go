package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestEquippedInstances_Empty(t *testing.T) {
	sess := &PlayerSession{
		Backpack:   inventory.NewBackpack(10, 100),
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
	}
	insts := sess.EquippedInstances()
	assert.NotNil(t, insts)
	assert.Empty(t, insts)
}

func TestEquippedInstances_NilFields(t *testing.T) {
	sess := &PlayerSession{}
	insts := sess.EquippedInstances()
	assert.NotNil(t, insts)
	assert.Empty(t, insts)
}

func TestEquippedInstances_ActivePresetOnly(t *testing.T) {
	reg := inventory.NewRegistry()
	weaponDef := &inventory.WeaponDef{
		ID:                  "knife",
		Name:                "Knife",
		DamageDice:          "1d4",
		DamageType:          "slashing",
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	require.NoError(t, reg.RegisterWeapon(weaponDef))
	itemDef := &inventory.ItemDef{
		ID:        "knife",
		Name:      "Knife",
		Kind:      inventory.KindWeapon,
		MaxStack:  1,
		Weight:    0.5,
		WeaponRef: "knife",
	}
	require.NoError(t, reg.RegisterItem(itemDef))

	bp := inventory.NewBackpack(10, 100)
	inst, err := bp.Add("knife", 1, reg)
	require.NoError(t, err)
	inst.InstanceID = "knife-inst-1"

	ls := inventory.NewLoadoutSet()
	// Active preset 0 has main hand.
	ls.Presets[0].MainHand = &inventory.EquippedWeapon{Def: weaponDef, InstanceID: "knife-inst-1"}
	// Preset 1 has a different instance ID that should NOT appear in results.
	ls.Presets[1].MainHand = &inventory.EquippedWeapon{Def: weaponDef, InstanceID: "other-inst"}

	sess := &PlayerSession{
		Backpack:   bp,
		LoadoutSet: ls,
		Equipment:  inventory.NewEquipment(),
	}
	insts := sess.EquippedInstances()
	require.Len(t, insts, 1)
	assert.Equal(t, "knife-inst-1", insts[0].InstanceID)
}

func TestEquippedInstances_ArmorSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{
		ID:      "vest",
		Name:    "Vest",
		Slot:    inventory.SlotTorso,
		ACBonus: 2,
		Rarity:  "street",
	}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{
		ID:       "vest",
		Name:     "Vest",
		Kind:     inventory.KindArmor,
		MaxStack: 1,
		Weight:   3.0,
		ArmorRef: "vest",
	}
	require.NoError(t, reg.RegisterItem(itemDef))

	bp := inventory.NewBackpack(10, 100)
	inst, err := bp.Add("vest", 1, reg)
	require.NoError(t, err)
	inst.InstanceID = "vest-inst-1"

	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "vest",
		Name:       "Vest",
		InstanceID: "vest-inst-1",
	}

	sess := &PlayerSession{
		Backpack:   bp,
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  eq,
	}
	insts := sess.EquippedInstances()
	require.Len(t, insts, 1)
	assert.Equal(t, "vest-inst-1", insts[0].InstanceID)
}
