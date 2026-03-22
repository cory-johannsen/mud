package command_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHandleRemoveArmor_RemovesEquippedSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{ID: "boots", Name: "Boots", Slot: inventory.SlotFeet, Group: "leather"}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{ID: "boots_item", Name: "Boots", Kind: "armor", ArmorRef: "boots", Weight: 1, MaxStack: 1}
	require.NoError(t, reg.RegisterItem(itemDef))

	sess := makeWearSession(t, reg)
	sess.Equipment.Armor[inventory.SlotFeet] = &inventory.SlottedItem{ItemDefID: "boots", Name: "Boots"}

	result := command.HandleRemoveArmor(sess, reg, "feet")
	assert.Contains(t, result, "Removed")
	assert.Nil(t, sess.Equipment.Armor[inventory.SlotFeet])
}

func TestHandleRemoveArmor_EmptySlotReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	sess := makeWearSession(t, reg)
	result := command.HandleRemoveArmor(sess, reg, "head")
	assert.Contains(t, result, "nothing")
}

func TestHandleRemoveArmor_InvalidSlotReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	sess := makeWearSession(t, reg)
	result := command.HandleRemoveArmor(sess, reg, "invalid_slot")
	assert.Contains(t, result, "slot")
}

func TestProperty_HandleRemoveArmor_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		sess := &session.PlayerSession{
			UID:        "test-uid",
			CharName:   "Tester",
			LoadoutSet: inventory.NewLoadoutSet(),
			Equipment:  inventory.NewEquipment(),
			Backpack:   inventory.NewBackpack(20, 100.0),
		}
		slot := rapid.String().Draw(rt, "slot")
		assert.NotPanics(rt, func() { command.HandleRemoveArmor(sess, reg, slot) })
	})
}

// TestHandleRemoveArmor_CursedArmorBlockedWhenRevealed verifies REQ-EM-24:
// a cursed armor piece with CurseRevealed=true cannot be removed.
func TestHandleRemoveArmor_CursedArmorBlockedWhenRevealed(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{ID: "cursed_plate", Name: "Cursed Plate", Slot: inventory.SlotTorso, Group: "plate"}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{ID: "cursed_plate_item", Name: "Cursed Plate", Kind: "armor", ArmorRef: "cursed_plate", Weight: 5, MaxStack: 1}
	require.NoError(t, reg.RegisterItem(itemDef))

	sess := &session.PlayerSession{
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
		Backpack:   inventory.NewBackpack(20, 100.0),
	}
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:     "cursed_plate",
		Name:          "Cursed Plate",
		Modifier:      "cursed",
		CurseRevealed: true,
	}

	result := command.HandleRemoveArmor(sess, reg, "torso")
	assert.Contains(t, strings.ToLower(result), "curse", "expected curse-related refusal")
	assert.NotNil(t, sess.Equipment.Armor[inventory.SlotTorso], "cursed armor should still be equipped")
}
