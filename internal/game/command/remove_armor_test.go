package command_test

import (
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
	itemDef := &inventory.ItemDef{ID: "boots_item", Name: "Boots", Kind: "armor", ArmorRef: "boots", Weight: 1}
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
