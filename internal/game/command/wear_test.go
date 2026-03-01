package command_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// armorItemDef returns a valid armor ItemDef referencing chestArmorDef.
func armorItemDef() *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:       "chest-plate",
		Name:     "Steel Chest Plate",
		Kind:     inventory.KindArmor,
		Weight:   5.0,
		ArmorRef: "chest-plate-def",
		MaxStack: 1,
	}
}

// chestArmorDef returns an ArmorDef for the torso slot.
func chestArmorDef() *inventory.ArmorDef {
	return &inventory.ArmorDef{
		ID:           "chest-plate-def",
		Name:         "Steel Chest Plate",
		Description:  "Heavy steel chest protection.",
		Slot:         inventory.SlotTorso,
		ACBonus:      4,
		DexCap:       2,
		CheckPenalty: 0,
		SpeedPenalty: 0,
		StrengthReq:  12,
		Bulk:         3,
		Group:        "plate",
	}
}

// helmetItemDef returns an armor ItemDef for the head slot.
func helmetItemDef() *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:       "iron-helmet",
		Name:     "Iron Helmet",
		Kind:     inventory.KindArmor,
		Weight:   2.0,
		ArmorRef: "iron-helmet-def",
		MaxStack: 1,
	}
}

// helmetArmorDef returns an ArmorDef for the head slot.
func helmetArmorDef() *inventory.ArmorDef {
	return &inventory.ArmorDef{
		ID:           "iron-helmet-def",
		Name:         "Iron Helmet",
		Description:  "Basic head protection.",
		Slot:         inventory.SlotHead,
		ACBonus:      1,
		DexCap:       10,
		CheckPenalty: 0,
		SpeedPenalty: 0,
		StrengthReq:  8,
		Bulk:         1,
		Group:        "plate",
	}
}

// newTestRegistryWithArmor returns a Registry with chest plate and helmet defs registered.
func newTestRegistryWithArmor() *inventory.Registry {
	reg := inventory.NewRegistry()
	_ = reg.RegisterItem(armorItemDef())
	_ = reg.RegisterArmor(chestArmorDef())
	_ = reg.RegisterItem(helmetItemDef())
	_ = reg.RegisterArmor(helmetArmorDef())
	return reg
}

// addChestToBackpack adds a chest-plate item to the session backpack.
//
// Precondition: reg must have "chest-plate" registered.
func addChestToBackpack(t *testing.T, sess *session.PlayerSession, reg *inventory.Registry) {
	t.Helper()
	if _, err := sess.Backpack.Add("chest-plate", 1, reg); err != nil {
		t.Fatalf("failed to add chest plate to backpack: %v", err)
	}
}

// TestHandleWear_EquipsArmorFromBackpack verifies that wearing valid armor:
//   - returns a confirmation message containing "Wore"
//   - removes the item from the backpack
//   - sets the correct equipment slot
func TestHandleWear_EquipsArmorFromBackpack(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()
	addChestToBackpack(t, sess, reg)

	result := command.HandleWear(sess, reg, "chest-plate torso")

	if !strings.Contains(result, "Wore") {
		t.Errorf("expected success message containing 'Wore', got: %q", result)
	}
	if sess.Backpack.UsedSlots() != 0 {
		t.Errorf("expected backpack to be empty after wear, got %d items", sess.Backpack.UsedSlots())
	}
	slotted := sess.Equipment.Armor[inventory.SlotTorso]
	if slotted == nil {
		t.Fatal("expected torso slot to be filled after wear")
	}
	if slotted.ItemDefID != "chest-plate-def" {
		t.Errorf("expected ItemDefID 'chest-plate-def', got %q", slotted.ItemDefID)
	}
}

// TestHandleWear_ItemNotInBackpack verifies that attempting to wear an item not in the
// backpack returns an appropriate error and leaves state unchanged.
func TestHandleWear_ItemNotInBackpack(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()
	// No items added to backpack.

	result := command.HandleWear(sess, reg, "chest-plate torso")

	if !strings.Contains(strings.ToLower(result), "not found in inventory") {
		t.Errorf("expected 'not found in inventory', got: %q", result)
	}
	if sess.Equipment.Armor[inventory.SlotTorso] != nil {
		t.Error("expected torso slot to remain empty")
	}
}

// TestHandleWear_ItemNotArmor verifies that attempting to wear a non-armor item
// returns an appropriate error.
func TestHandleWear_ItemNotArmor(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()
	// Register and add a weapon item.
	_ = reg.RegisterItem(pistolItemDef())
	_ = reg.RegisterWeapon(pistolWeaponDef())
	if _, err := sess.Backpack.Add("pistol-9mm", 1, reg); err != nil {
		t.Fatalf("failed to add pistol to backpack: %v", err)
	}

	result := command.HandleWear(sess, reg, "pistol-9mm torso")

	if !strings.Contains(strings.ToLower(result), "is not armor") {
		t.Errorf("expected 'is not armor', got: %q", result)
	}
}

// TestHandleWear_WrongSlot verifies that attempting to equip armor in the wrong body slot
// returns a descriptive error naming the correct slot, and leaves all equipment slots unchanged.
func TestHandleWear_WrongSlot(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()
	addChestToBackpack(t, sess, reg)

	// chest-plate belongs in torso, not head.
	result := command.HandleWear(sess, reg, "chest-plate head")

	if !strings.Contains(strings.ToLower(result), "torso") {
		t.Errorf("expected message mentioning correct slot 'torso', got: %q", result)
	}
	// Item must remain in backpack after failed attempt.
	if sess.Backpack.UsedSlots() != 1 {
		t.Errorf("expected backpack to still have item after wrong-slot attempt, got %d items", sess.Backpack.UsedSlots())
	}
	// Equipment slots must remain unoccupied after the failed wear.
	assert.Nil(t, sess.Equipment.Armor[inventory.SlotHead], "head slot must remain nil after wrong-slot attempt")
	assert.Nil(t, sess.Equipment.Armor[inventory.SlotTorso], "torso slot must remain nil after wrong-slot attempt")
}

// TestHandleWear_InvalidSlot verifies that an unrecognised slot name returns a descriptive error.
func TestHandleWear_InvalidSlot(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()
	addChestToBackpack(t, sess, reg)

	result := command.HandleWear(sess, reg, "chest-plate finger")

	if !strings.Contains(strings.ToLower(result), "unknown slot") {
		t.Errorf("expected 'Unknown slot' in result, got: %q", result)
	}
}

// TestHandleWear_ReplacesExistingSlot verifies that wearing new armor when a slot is
// already occupied returns the previous item to the backpack.
func TestHandleWear_ReplacesExistingSlot(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistryWithArmor()

	// Pre-equip a chest piece directly without going through the backpack.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "chest-plate-def",
		Name:      "Steel Chest Plate",
	}
	// Add a new chest plate to the backpack.
	addChestToBackpack(t, sess, reg)
	beforeSlots := sess.Backpack.UsedSlots()

	result := command.HandleWear(sess, reg, "chest-plate torso")

	if !strings.Contains(result, "Wore") {
		t.Errorf("expected success message containing 'Wore', got: %q", result)
	}
	// Backpack slot count should stay the same: one removed (new item), one added (old item returned).
	afterSlots := sess.Backpack.UsedSlots()
	if afterSlots != beforeSlots {
		t.Errorf("expected backpack slot count to remain %d (old item returned), got %d", beforeSlots, afterSlots)
	}
}

// TestProperty_HandleWear_NeverPanics is a property-based test verifying that
// HandleWear never panics regardless of arbitrary input strings.
func TestProperty_HandleWear_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := newTestSessionWithBackpack()
		reg := newTestRegistryWithArmor()

		itemID := rapid.StringMatching(`[a-z0-9\-]{1,20}`).Draw(rt, "itemID")
		slot := rapid.StringMatching(`[a-z_]{1,20}`).Draw(rt, "slot")
		arg := itemID + " " + slot

		// Must not panic regardless of input.
		_ = command.HandleWear(sess, reg, arg)
	})
}

// TestProperty_HandleWear_BackpackDecreasesByOne is a property-based test verifying that
// a successful wear always removes exactly one item from the backpack (when slot was empty).
func TestProperty_HandleWear_BackpackDecreasesByOne(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := newTestSessionWithBackpack()
		reg := newTestRegistryWithArmor()

		// Add a chest plate.
		if _, err := sess.Backpack.Add("chest-plate", 1, reg); err != nil {
			rt.Skip()
		}
		before := sess.Backpack.UsedSlots()

		result := command.HandleWear(sess, reg, "chest-plate torso")

		if strings.Contains(result, "Wore") {
			after := sess.Backpack.UsedSlots()
			// Slot was empty so net change must be exactly -1.
			if after != before-1 {
				rt.Fatalf("expected backpack to shrink by 1; before=%d after=%d", before, after)
			}
		}
	})
}

// TestProperty_HandleWear_SlotUnchangedOnFailure is a property-based test verifying that
// when HandleWear fails, all equipment armor slots remain unchanged from their pre-call state.
func TestProperty_HandleWear_SlotUnchangedOnFailure(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		armorDef := &inventory.ArmorDef{
			ID:    "test_helm",
			Name:  "Test Helm",
			Slot:  inventory.SlotHead,
			Group: "composite",
		}
		_ = reg.RegisterArmor(armorDef)
		itemDef := &inventory.ItemDef{
			ID:       "test_helm_item",
			Name:     "Test Helm",
			Kind:     inventory.KindArmor,
			ArmorRef: "test_helm",
			Weight:   1,
			MaxStack: 1,
		}
		_ = reg.RegisterItem(itemDef)

		bp := inventory.NewBackpack(20, 100)
		eq := inventory.NewEquipment()
		sess := &session.PlayerSession{
			UID:        "uid",
			CharName:   "Test",
			Backpack:   bp,
			Equipment:  eq,
			LoadoutSet: inventory.NewLoadoutSet(),
		}

		// Generate random args that will mostly fail (unknown item IDs or invalid slots).
		itemID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "itemID")
		slotStr := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "slot")
		arg := itemID + " " + slotStr

		// Capture equipment state before the call.
		slotsBefore := make(map[inventory.ArmorSlot]*inventory.SlottedItem)
		for k, v := range sess.Equipment.Armor {
			slotsBefore[k] = v
		}

		result := command.HandleWear(sess, reg, arg)

		// When the wear fails, all equipment slots must be unchanged.
		if !strings.HasPrefix(result, "Wore ") {
			for k, v := range slotsBefore {
				assert.Equal(rt, v, sess.Equipment.Armor[k], "slot %s changed on failed wear", k)
			}
			// Also verify no new slot was populated.
			for k, v := range sess.Equipment.Armor {
				if _, existed := slotsBefore[k]; !existed {
					assert.Nil(rt, v, "slot %s was populated on failed wear but was absent before", k)
				}
			}
		}
	})
}
