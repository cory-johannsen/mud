package command_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRepairRoller is a deterministic Roller for repair tests.
type stubRepairRoller struct{ roll int }

func (s *stubRepairRoller) Roll(_ string) int    { return s.roll }
func (s *stubRepairRoller) RollD20() int         { return 10 }
func (s *stubRepairRoller) RollFloat() float64   { return 0.5 }

// makeRepairRegistry returns a registry with a repair_kit item and a sword weapon.
func makeRepairRegistry(t *testing.T) *inventory.Registry {
	t.Helper()
	reg := inventory.NewRegistry()
	// Register repair_kit consumable
	err := reg.RegisterItem(&inventory.ItemDef{
		ID: "repair_kit", Name: "Repair Kit",
		Kind: inventory.KindConsumable, MaxStack: 10, Weight: 0.5,
	})
	require.NoError(t, err)
	// Register sword weapon
	wd := &inventory.WeaponDef{
		ID: "sword", Name: "Sword",
		DamageDice: "1d6", DamageType: "slashing",
		ProficiencyCategory: "martial_melee",
		Rarity:              "street",
	}
	reg.RegisterWeapon(wd)
	err = reg.RegisterItem(&inventory.ItemDef{
		ID: "iron_sword", Name: "Iron Sword",
		Kind: inventory.KindWeapon, WeaponRef: "sword", MaxStack: 1, Weight: 2.0,
	})
	require.NoError(t, err)
	return reg
}

// makeRepairSession creates a session with a weapon in the active loadout and
// an equipment with a damaged chest armor. The weapon is equipped with the given durability.
func makeRepairSession(t *testing.T, reg *inventory.Registry, weaponDurability int, armorDurability int) *command.RepairSession {
	t.Helper()
	sess := makeWearSession(t, reg)

	// Give the session a LoadoutSet with a sword equipped.
	swordDef := reg.Weapon("sword")
	require.NotNil(t, swordDef)
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(swordDef))
	preset.MainHand.Durability = weaponDurability
	preset.MainHand.InstanceID = "inst-sword-1"
	sess.LoadoutSet.Presets[0] = preset

	// Add chest armor to equipment.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID:  "chest_armor",
		Name:       "Chest Armor",
		InstanceID: "inst-armor-1",
		Durability: armorDurability,
		Rarity:     "street",
	}

	return &command.RepairSession{Session: sess}
}

// TestHandleRepair_NoRepairKit verifies that the command fails without a repair_kit (REQ-EM-13).
func TestHandleRepair_NoRepairKit(t *testing.T) {
	reg := makeRepairRegistry(t)
	rs := makeRepairSession(t, reg, 30, 30)
	rng := &stubRepairRoller{roll: 4}

	result := command.HandleRepair(rs, reg, "sword", rng)
	assert.Equal(t, "You need a repair kit to field-repair equipment.", result)
}

// TestHandleRepair_WeaponNotFound verifies that an unknown item returns an error.
func TestHandleRepair_WeaponNotFound(t *testing.T) {
	reg := makeRepairRegistry(t)
	rs := makeRepairSession(t, reg, 30, 30)
	// Add a repair kit so kit check passes.
	_, err := rs.Session.Backpack.Add("repair_kit", 1, reg)
	require.NoError(t, err)
	rng := &stubRepairRoller{roll: 4}

	result := command.HandleRepair(rs, reg, "unknown_item", rng)
	assert.Contains(t, result, "not found")
}

// TestHandleRepair_WeaponAtFullDurability verifies that a fully durable item is rejected.
func TestHandleRepair_WeaponAtFullDurability(t *testing.T) {
	reg := makeRepairRegistry(t)
	rs := makeRepairSession(t, reg, 40, 30) // sword MaxDurability for "street" = 40
	_, err := rs.Session.Backpack.Add("repair_kit", 1, reg)
	require.NoError(t, err)
	rng := &stubRepairRoller{roll: 4}

	result := command.HandleRepair(rs, reg, "sword", rng)
	assert.Contains(t, strings.ToLower(result), "already")
}

// TestHandleRepair_Success_RestoresDurability verifies REQ-EM-14: restores 1d6 durability
// and consumes the repair_kit (REQ-EM-13).
func TestHandleRepair_Success_RestoresDurability(t *testing.T) {
	reg := makeRepairRegistry(t)
	rs := makeRepairSession(t, reg, 20, 30) // damaged sword
	_, err := rs.Session.Backpack.Add("repair_kit", 1, reg)
	require.NoError(t, err)
	rng := &stubRepairRoller{roll: 4} // 1d6 = 4

	result := command.HandleRepair(rs, reg, "sword", rng)
	assert.Contains(t, result, "4")

	// Repair kit should be consumed.
	kits := rs.Session.Backpack.FindByItemDefID("repair_kit")
	assert.Empty(t, kits, "repair_kit should be consumed")

	// Weapon durability should increase by 4 (from 20 to 24).
	preset := rs.Session.LoadoutSet.ActivePreset()
	require.NotNil(t, preset)
	require.NotNil(t, preset.MainHand)
	assert.Equal(t, 24, preset.MainHand.Durability)
}

// TestHandleRepair_Armor_RestoresDurability verifies repair works on an armor slot.
func TestHandleRepair_Armor_RestoresDurability(t *testing.T) {
	reg := makeRepairRegistry(t)
	rs := makeRepairSession(t, reg, 40, 10) // chest armor at 10/40
	_, err := rs.Session.Backpack.Add("repair_kit", 1, reg)
	require.NoError(t, err)
	rng := &stubRepairRoller{roll: 3}

	result := command.HandleRepair(rs, reg, "chest_armor", rng)
	assert.Contains(t, result, "3")

	si := rs.Session.Equipment.Armor[inventory.SlotTorso]
	require.NotNil(t, si)
	assert.Equal(t, 13, si.Durability)
}
