package command_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newTestSessionWithEquipmentAndBackpack returns a session suitable for equipment affix display tests.
func newTestSessionWithEquipmentAndBackpack() *session.PlayerSession {
	return &session.PlayerSession{
		UID:        "test-uid",
		CharName:   "Tester",
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
		Backpack:   inventory.NewBackpack(20, 100.0),
	}
}

// TestEquipment_ShowsSlotCounter verifies that a mil-spec weapon (UpgradeSlots=2)
// with 1 affixed material shows "[1/2 slots]" in the equipment display.
func TestEquipment_ShowsSlotCounter(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}

	result := command.HandleEquipment(sess, 0, reg)

	if !strings.Contains(result, "[1/2 slots]") {
		t.Errorf("expected '[1/2 slots]' in output, got:\n%s", result)
	}
}

// TestEquipment_ShowsAffixedMaterialSubList verifies that a weapon with an affixed
// material shows the material name and grade name in the sub-list.
func TestEquipment_ShowsAffixedMaterialSubList(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}

	result := command.HandleEquipment(sess, 0, reg)

	// MaterialDef registered for scrap_iron:street_grade has Name="Scrap Iron" GradeName=derived from gradeID.
	// The registry registers material from ItemDef where MaterialName="Scrap Iron".
	if !strings.Contains(result, "Scrap Iron") {
		t.Errorf("expected material name 'Scrap Iron' in output, got:\n%s", result)
	}
	if !strings.Contains(result, "↳") {
		t.Errorf("expected sub-list indicator '↳' in output, got:\n%s", result)
	}
}

// TestEquipment_NoSlotCounter_ForZeroUpgradeSlots verifies that a weapon with
// UpgradeSlots=0 does not show a slot counter.
func TestEquipment_NoSlotCounter_ForZeroUpgradeSlots(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	def := &inventory.WeaponDef{
		ID:                  "no-slots-weapon",
		Name:                "Basic Knife",
		DamageDice:          "1d4",
		DamageType:          "slashing",
		ProficiencyCategory: "simple_melee",
		Rarity:              "salvage",
		UpgradeSlots:        0,
	}
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(def)
	reg := buildAffixTestRegistry(t)

	result := command.HandleEquipment(sess, 0, reg)

	if strings.Contains(result, "slots]") {
		t.Errorf("did not expect 'slots]' in output for zero-slot weapon, got:\n%s", result)
	}
}

// TestEquipment_NilRegistry_NoMaterialSubList verifies that when reg is nil,
// the affixed material sub-list (↳ lines) is suppressed even if materials are affixed.
// Note: the slot counter [N/M slots] still appears because UpgradeSlots is on WeaponDef.
func TestEquipment_NilRegistry_NoMaterialSubList(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}

	// Pass nil registry — material sub-list must be suppressed.
	result := command.HandleEquipment(sess, 0, nil)

	if strings.Contains(result, "↳") {
		t.Errorf("did not expect '↳' sub-list when reg is nil, got:\n%s", result)
	}
}

// TestEquipment_SlotCounter_FullyAffixed verifies that a fully-affixed weapon (2/2 slots) shows "[2/2 slots]".
func TestEquipment_SlotCounter_FullyAffixed(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{
		"scrap_iron:street_grade",
		"hollow_point:street_grade",
	}

	result := command.HandleEquipment(sess, 0, reg)

	if !strings.Contains(result, "[2/2 slots]") {
		t.Errorf("expected '[2/2 slots]' in output, got:\n%s", result)
	}
}

// TestLoadout_ShowsSlotCounter verifies that a mil-spec weapon (UpgradeSlots=2)
// with 1 affixed material shows "[1/2 slots]" in the loadout display.
func TestLoadout_ShowsSlotCounter(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}

	result := command.HandleLoadout(sess, "", reg)

	if !strings.Contains(result, "[1/2 slots]") {
		t.Errorf("expected '[1/2 slots]' in loadout output, got:\n%s", result)
	}
}

// TestLoadout_ShowsAffixedMaterialSubList verifies that a weapon with an affixed
// material shows the material name and grade name in the sub-list under loadout.
func TestLoadout_ShowsAffixedMaterialSubList(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	reg := buildAffixTestRegistry(t)
	equipAffixWeapon(t, sess, reg, "test_pistol_mil_spec")
	sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}

	result := command.HandleLoadout(sess, "", reg)

	if !strings.Contains(result, "Scrap Iron") {
		t.Errorf("expected material name 'Scrap Iron' in loadout output, got:\n%s", result)
	}
	if !strings.Contains(result, "↳") {
		t.Errorf("expected sub-list indicator '↳' in loadout output, got:\n%s", result)
	}
}

// TestLoadout_NoSlotCounter_ForZeroUpgradeSlots verifies that a weapon with
// UpgradeSlots=0 does not show a slot counter in the loadout display.
func TestLoadout_NoSlotCounter_ForZeroUpgradeSlots(t *testing.T) {
	sess := newTestSessionWithEquipmentAndBackpack()
	def := &inventory.WeaponDef{
		ID:                  "no-slots-loadout",
		Name:                "Simple Pistol",
		DamageDice:          "1d4",
		DamageType:          "ballistic",
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
		UpgradeSlots:        0,
	}
	_ = sess.LoadoutSet.Presets[0].EquipMainHand(def)
	reg := buildAffixTestRegistry(t)

	result := command.HandleLoadout(sess, "", reg)

	if strings.Contains(result, "slots]") {
		t.Errorf("did not expect 'slots]' in loadout for zero-slot weapon, got:\n%s", result)
	}
}

// TestProperty_Equipment_SlotCounterNeverPanics is a property-based test verifying
// that HandleEquipment never panics regardless of AffixedMaterials content.
func TestProperty_Equipment_SlotCounterNeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{
			UID:        "test-uid",
			CharName:   "Tester",
			LoadoutSet: inventory.NewLoadoutSet(),
			Equipment:  inventory.NewEquipment(),
			Backpack:   inventory.NewBackpack(20, 100.0),
		}
		reg := buildAffixTestRegistryRT(rt)
		wd := reg.Weapon("test_pistol_mil_spec")
		if wd == nil {
			rt.Fatal("test_pistol_mil_spec not in registry")
		}
		_ = sess.LoadoutSet.Presets[0].EquipMainHand(wd)

		// Random number of affixed entries, including malformed ones.
		n := rapid.IntRange(0, 4).Draw(rt, "nAffixed")
		entries := make([]string, n)
		for i := range entries {
			entries[i] = rapid.StringOf(rapid.Rune()).Draw(rt, "entry")
		}
		sess.LoadoutSet.Presets[0].MainHand.AffixedMaterials = entries

		width := rapid.IntRange(0, 120).Draw(rt, "width")
		_ = command.HandleEquipment(sess, width, reg)
	})
}
