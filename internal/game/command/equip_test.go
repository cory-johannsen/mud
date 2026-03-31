package command_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newTestSessionWithBackpack returns a PlayerSession with an initialised LoadoutSet,
// Equipment, and a Backpack.
//
// Postcondition: all session fields needed by equip/unequip/equipment commands are non-nil.
func newTestSessionWithBackpack() *session.PlayerSession {
	return &session.PlayerSession{
		UID:        "test-uid",
		CharName:   "Tester",
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
		Backpack:   inventory.NewBackpack(20, 100.0),
	}
}

// pistolItemDef returns a valid weapon ItemDef with WeaponRef pointing to pistolWeaponDef.
func pistolItemDef() *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:        "pistol-9mm",
		Name:      "9mm Pistol",
		Kind:      "weapon",
		Weight:    1.0,
		WeaponRef: "pistol-9mm",
		MaxStack:  1,
	}
}

// rifleItemDef returns a valid two-handed weapon ItemDef.
func rifleItemDef() *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:        "rifle-ar",
		Name:      "Assault Rifle",
		Kind:      "weapon",
		Weight:    3.0,
		WeaponRef: "rifle-ar",
		MaxStack:  1,
	}
}

// rifleWeaponDef returns a two-handed weapon definition.
func rifleWeaponDef() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:                  "rifle-ar",
		Name:                "Assault Rifle",
		DamageDice:          "3d6",
		DamageType:          "ballistic",
		RangeIncrement:      60,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle, inventory.FiringModeBurst},
		MagazineCapacity:    30,
		Kind:                inventory.WeaponKindTwoHanded,
		ProficiencyCategory: "martial_ranged",
		Rarity:              "salvage",
	}
}

// newTestRegistry returns a Registry populated with pistol and rifle defs.
func newTestRegistry() *inventory.Registry {
	reg := inventory.NewRegistry()
	_ = reg.RegisterWeapon(pistolWeaponDef())
	_ = reg.RegisterWeapon(rifleWeaponDef())
	_ = reg.RegisterItem(pistolItemDef())
	_ = reg.RegisterItem(rifleItemDef())
	return reg
}

// addPistolToBackpack adds a pistol ItemInstance to the session backpack.
//
// Precondition: reg must have "pistol-9mm" registered.
func addPistolToBackpack(t *testing.T, sess *session.PlayerSession, reg *inventory.Registry) {
	t.Helper()
	if _, err := sess.Backpack.Add("pistol-9mm", 1, reg); err != nil {
		t.Fatalf("failed to add pistol to backpack: %v", err)
	}
}

// addRifleToBackpack adds a rifle ItemInstance to the session backpack.
func addRifleToBackpack(t *testing.T, sess *session.PlayerSession, reg *inventory.Registry) {
	t.Helper()
	if _, err := sess.Backpack.Add("rifle-ar", 1, reg); err != nil {
		t.Fatalf("failed to add rifle to backpack: %v", err)
	}
}

// TestHandleEquip_MainHand_Success verifies that equipping a weapon to the main hand
// removes it from the backpack and equips it in the active preset.
func TestHandleEquip_MainHand_Success(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	addPistolToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "pistol-9mm main")

	if !strings.Contains(result, "Equipped") {
		t.Errorf("expected success message, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "main") {
		t.Errorf("expected 'main' in result, got: %q", result)
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset.MainHand == nil {
		t.Fatal("expected MainHand to be set after equip")
	}
	if preset.MainHand.Def.ID != "pistol-9mm" {
		t.Errorf("expected pistol-9mm in MainHand, got %q", preset.MainHand.Def.ID)
	}
	if sess.Backpack.UsedSlots() != 0 {
		t.Errorf("expected backpack to be empty after equip, got %d items", sess.Backpack.UsedSlots())
	}
}

// TestHandleEquip_OffHand_Success verifies that equipping a one-handed weapon to the off
// hand removes it from the backpack and equips it in the active preset.
func TestHandleEquip_OffHand_Success(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	addPistolToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "pistol-9mm off")

	if !strings.Contains(result, "Equipped") {
		t.Errorf("expected success message, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "off") {
		t.Errorf("expected 'off' in result, got: %q", result)
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset.OffHand == nil {
		t.Fatal("expected OffHand to be set after equip")
	}
	if preset.OffHand.Def.ID != "pistol-9mm" {
		t.Errorf("expected pistol-9mm in OffHand, got %q", preset.OffHand.Def.ID)
	}
	if sess.Backpack.UsedSlots() != 0 {
		t.Errorf("expected backpack to be empty after equip, got %d items", sess.Backpack.UsedSlots())
	}
}

// TestHandleEquip_NoSlot_Weapon_DefaultsToMain verifies that omitting the slot argument
// for a weapon defaults to equipping in the main hand.
func TestHandleEquip_NoSlot_Weapon_DefaultsToMain(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	addPistolToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "pistol-9mm")

	if !strings.Contains(strings.ToLower(result), "main") {
		t.Errorf("expected main hand equip confirmation, got: %q", result)
	}
}

// TestHandleEquip_NotInBackpack_ReturnsError verifies that equipping an item not in the
// backpack returns an appropriate error.
func TestHandleEquip_NotInBackpack_ReturnsError(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	// Do not add anything to backpack.

	result := command.HandleEquip(sess, reg, "pistol-9mm main")

	if !strings.Contains(strings.ToLower(result), "not found in your pack") {
		t.Errorf("expected 'not found in your pack', got: %q", result)
	}
}

// TestHandleEquip_UnknownItemID_ReturnsError verifies that an item ID not in the
// registry returns an appropriate error.
func TestHandleEquip_UnknownItemID_ReturnsError(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()

	result := command.HandleEquip(sess, reg, "no-such-item main")

	if !strings.Contains(strings.ToLower(result), "not found in your pack") {
		t.Errorf("expected 'not found in your pack', got: %q", result)
	}
}

// TestHandleEquip_UnknownSlot_ReturnsError verifies that an unknown slot name returns
// an error rather than silently accepting it.
func TestHandleEquip_UnknownSlot_ReturnsError(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	addPistolToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "pistol-9mm torso")

	if !strings.Contains(strings.ToLower(result), "specify main or off") {
		t.Errorf("expected 'specify main or off' for unknown slot, got: %q", result)
	}
}

// TestHandleEquip_TwoHandedBlocksOffHand verifies that equipping a two-handed weapon
// when the off-hand is occupied clears OffHand (weapon rule from EquipMainHand).
func TestHandleEquip_TwoHandedClearsOffHand(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	// First equip a pistol in OffHand directly.
	_ = sess.LoadoutSet.ActivePreset().EquipOffHand(pistolWeaponDef())
	// Add rifle to backpack.
	addRifleToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "rifle-ar main")

	if !strings.Contains(result, "Equipped") {
		t.Errorf("expected success message for two-handed equip, got: %q", result)
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset.MainHand == nil || preset.MainHand.Def.ID != "rifle-ar" {
		t.Errorf("expected rifle-ar in MainHand, got: %v", preset.MainHand)
	}
	if preset.OffHand != nil {
		t.Error("expected OffHand to be nil after equipping two-handed weapon")
	}
}

// TestHandleEquip_OffHandBlockedByTwoHanded verifies that equipping a one-handed weapon
// in the off-hand while a two-handed weapon occupies main hand returns an error.
func TestHandleEquip_OffHandBlockedByTwoHanded(t *testing.T) {
	sess := newTestSessionWithBackpack()
	reg := newTestRegistry()
	// Equip rifle in MainHand.
	_ = sess.LoadoutSet.ActivePreset().EquipMainHand(rifleWeaponDef())
	// Add pistol to backpack.
	addPistolToBackpack(t, sess, reg)

	result := command.HandleEquip(sess, reg, "pistol-9mm off")

	if !strings.Contains(strings.ToLower(result), "cannot equip off-hand") {
		t.Errorf("expected off-hand blocked error, got: %q", result)
	}
	// Pistol must remain in backpack (equip failed).
	if sess.Backpack.UsedSlots() != 1 {
		t.Errorf("expected pistol to remain in backpack, got %d items", sess.Backpack.UsedSlots())
	}
}

// TestProperty_HandleEquip_BackpackCountDecreases is a property-based test verifying
// that a successful equip always removes exactly one item from the backpack.
func TestProperty_HandleEquip_BackpackCountDecreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := newTestSessionWithBackpack()
		reg := newTestRegistry()

		slot := rapid.SampledFrom([]string{"main", "off"}).Draw(rt, "slot")

		// Add a pistol (one-handed, valid for both slots).
		if _, err := sess.Backpack.Add("pistol-9mm", 1, reg); err != nil {
			rt.Skip()
		}
		before := sess.Backpack.UsedSlots()

		result := command.HandleEquip(sess, reg, "pistol-9mm "+slot)

		if strings.Contains(result, "Equipped") {
			after := sess.Backpack.UsedSlots()
			if after != before-1 {
				rt.Fatalf("expected backpack to shrink by 1; before=%d after=%d", before, after)
			}
		}
	})
}

// TestHandleEquip_MinLevelCheck verifies that equipping a higher-rarity weapon
// when the player's level is too low returns the correct error message (REQ-EM-3).
func TestHandleEquip_MinLevelCheck_WeaponTooHighLevel(t *testing.T) {
	sess := newTestSessionWithBackpack()
	sess.Level = 3 // below ghost (min 15)
	reg := newTestRegistry()

	// Register a ghost-rarity weapon.
	ghostWeaponDef := &inventory.WeaponDef{
		ID: "ghost_blade", Name: "Ghost Blade",
		DamageDice: "2d6", DamageType: "slashing",
		Kind: inventory.WeaponKindOneHanded, Group: "blade",
		ProficiencyCategory: "martial_melee",
		Rarity: "ghost",
	}
	ghostItemDef := &inventory.ItemDef{
		ID: "ghost_blade", Name: "Ghost Blade",
		Kind: inventory.KindWeapon, WeaponRef: "ghost_blade",
		Weight: 2.0, MaxStack: 1,
	}
	_ = reg.RegisterWeapon(ghostWeaponDef)
	_ = reg.RegisterItem(ghostItemDef)
	if _, err := sess.Backpack.Add("ghost_blade", 1, reg); err != nil {
		t.Fatalf("failed to add ghost blade: %v", err)
	}

	result := command.HandleEquip(sess, reg, "ghost_blade main")

	if !strings.Contains(result, "level 15") {
		t.Errorf("expected level 15 in error message, got: %q", result)
	}
	if !strings.Contains(strings.ToLower(result), "ghost blade") {
		t.Errorf("expected item name in error message, got: %q", result)
	}
	// Backpack should be unchanged.
	if sess.Backpack.UsedSlots() != 1 {
		t.Errorf("expected item to remain in backpack, got %d items", sess.Backpack.UsedSlots())
	}
}

// TestHandleEquip_MinLevelCheck_ExactLevel verifies that equipping at exactly the
// required level succeeds (REQ-EM-3 boundary).
func TestHandleEquip_MinLevelCheck_ExactLevel(t *testing.T) {
	sess := newTestSessionWithBackpack()
	sess.Level = 15 // exactly ghost min level
	reg := newTestRegistry()

	ghostWeaponDef := &inventory.WeaponDef{
		ID: "ghost_blade2", Name: "Ghost Blade 2",
		DamageDice: "2d6", DamageType: "slashing",
		Kind: inventory.WeaponKindOneHanded, Group: "blade",
		ProficiencyCategory: "martial_melee",
		Rarity: "ghost",
	}
	ghostItemDef := &inventory.ItemDef{
		ID: "ghost_blade2", Name: "Ghost Blade 2",
		Kind: inventory.KindWeapon, WeaponRef: "ghost_blade2",
		Weight: 2.0, MaxStack: 1,
	}
	_ = reg.RegisterWeapon(ghostWeaponDef)
	_ = reg.RegisterItem(ghostItemDef)
	if _, err := sess.Backpack.Add("ghost_blade2", 1, reg); err != nil {
		t.Fatalf("failed to add ghost blade: %v", err)
	}

	result := command.HandleEquip(sess, reg, "ghost_blade2 main")

	if !strings.Contains(result, "Equipped") {
		t.Errorf("expected success at exact min level, got: %q", result)
	}
}
