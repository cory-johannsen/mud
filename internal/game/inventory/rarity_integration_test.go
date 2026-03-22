package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// ── WeaponDef rarity field ────────────────────────────────────────────────────

func TestWeaponDef_Validate_RarityRequired(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "test", Name: "Test", DamageDice: "1d4", DamageType: "piercing",
		Kind: inventory.WeaponKindOneHanded, Group: "blade",
		ProficiencyCategory: "martial_melee",
		// Rarity intentionally absent
	}
	err := w.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rarity")
}

func TestWeaponDef_Validate_ValidRarity(t *testing.T) {
	for _, rarity := range []string{"salvage", "street", "mil_spec", "black_market", "ghost"} {
		w := &inventory.WeaponDef{
			ID: "test", Name: "Test", DamageDice: "1d4", DamageType: "piercing",
			Kind: inventory.WeaponKindOneHanded, Group: "blade",
			ProficiencyCategory: "martial_melee",
			Rarity:              rarity,
		}
		require.NoError(t, w.Validate(), "rarity %q should be valid", rarity)
	}
}

func TestWeaponDef_Validate_InvalidRarity(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "test", Name: "Test", DamageDice: "1d4", DamageType: "piercing",
		Kind: inventory.WeaponKindOneHanded, Group: "blade",
		ProficiencyCategory: "martial_melee",
		Rarity:              "legendary",
	}
	err := w.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legendary")
}

// ── ArmorDef rarity field ─────────────────────────────────────────────────────

func TestArmorDef_Validate_RarityRequired(t *testing.T) {
	a := &inventory.ArmorDef{
		ID: "test", Name: "Test", Slot: inventory.SlotTorso,
		Group: "leather", ProficiencyCategory: "light_armor",
		// Rarity intentionally absent
	}
	err := a.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rarity")
}

func TestArmorDef_Validate_ValidRarity(t *testing.T) {
	for _, rarity := range []string{"salvage", "street", "mil_spec", "black_market", "ghost"} {
		a := &inventory.ArmorDef{
			ID: "test", Name: "Test", Slot: inventory.SlotTorso,
			Group: "leather", ProficiencyCategory: "light_armor",
			Rarity: rarity,
		}
		require.NoError(t, a.Validate(), "rarity %q should be valid", rarity)
	}
}

// ── Stat multiplier applied at load time ──────────────────────────────────────

func TestLoadWeapons_RarityMultiplierApplied(t *testing.T) {
	dir := t.TempDir()
	yaml := `
id: test_weapon
name: Test Weapon
damage_dice: 1d6
damage_type: slashing
kind: one_handed
group: blade
proficiency_category: martial_melee
rarity: street
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "w.yaml"), []byte(yaml), 0644))
	weapons, err := inventory.LoadWeapons(dir)
	require.NoError(t, err)
	require.Len(t, weapons, 1)
	w := weapons[0]
	assert.Equal(t, "street", w.Rarity)
	// Street stat multiplier = 1.2; stored on WeaponDef for consumption by display/audit.
	assert.InDelta(t, 1.2, w.RarityStatMultiplier, 1e-9)
}

func TestLoadWeapons_MissingRarity_Error(t *testing.T) {
	dir := t.TempDir()
	yaml := `
id: test_weapon
name: Test Weapon
damage_dice: 1d6
damage_type: slashing
kind: one_handed
group: blade
proficiency_category: martial_melee
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "w.yaml"), []byte(yaml), 0644))
	_, err := inventory.LoadWeapons(dir)
	require.Error(t, err)
}

func TestLoadArmors_RarityMultiplierApplied(t *testing.T) {
	dir := t.TempDir()
	yaml := `
id: test_armor
name: Test Armor
slot: torso
ac_bonus: 4
dex_cap: 3
group: leather
proficiency_category: light_armor
rarity: mil_spec
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(yaml), 0644))
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	require.Len(t, armors, 1)
	a := armors[0]
	assert.Equal(t, "mil_spec", a.Rarity)
	assert.InDelta(t, 1.5, a.RarityStatMultiplier, 1e-9)
	// ACBonus should be multiplied: floor(4 * 1.5) = 6
	assert.Equal(t, 6, a.ACBonus)
}

func TestLoadArmors_MissingRarity_Error(t *testing.T) {
	dir := t.TempDir()
	yaml := `
id: test_armor
name: Test Armor
slot: torso
ac_bonus: 2
group: leather
proficiency_category: light_armor
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(yaml), 0644))
	_, err := inventory.LoadArmors(dir)
	require.Error(t, err)
}

// ── Min level enforcement display name ───────────────────────────────────────

func TestWeaponDef_RarityMinLevel(t *testing.T) {
	w := &inventory.WeaponDef{
		ID: "ghost_blade", Name: "Ghost Blade", DamageDice: "2d6", DamageType: "slashing",
		Kind: inventory.WeaponKindTwoHanded, Group: "blade",
		ProficiencyCategory: "martial_melee",
		Rarity:              "ghost",
		RarityStatMultiplier: 2.2,
	}
	rarityDef, ok := inventory.LookupRarity(w.Rarity)
	require.True(t, ok)
	assert.Equal(t, 15, rarityDef.MinLevel)
}

// ── Rarity color codes ────────────────────────────────────────────────────────

func TestRarityColorCode_AllTiers(t *testing.T) {
	tests := []struct {
		rarity string
		want   string
	}{
		{"salvage", "\033[90m"},      // dark gray
		{"street", "\033[97m"},       // bright white
		{"mil_spec", "\033[32m"},     // green
		{"black_market", "\033[35m"}, // purple/magenta
		{"ghost", "\033[33m"},        // gold/yellow
	}
	for _, tc := range tests {
		got := inventory.RarityColorCode(tc.rarity)
		assert.Equal(t, tc.want, got, "rarity %q", tc.rarity)
	}
}

func TestRarityColorCode_Unknown_ReturnsReset(t *testing.T) {
	got := inventory.RarityColorCode("unknown")
	assert.Equal(t, "\033[0m", got)
}

func TestRarityDisplayName_WithColorCodes(t *testing.T) {
	name := inventory.RarityColoredName("salvage", "Iron Pipe")
	assert.Contains(t, name, "Iron Pipe")
	assert.Contains(t, name, "\033[90m")
	assert.Contains(t, name, "\033[0m") // reset at end
}

func TestProperty_RarityColoredName_AlwaysContainsItemName(t *testing.T) {
	tiers := []string{"salvage", "street", "mil_spec", "black_market", "ghost"}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(tiers)-1).Draw(rt, "idx")
		name := rapid.StringMatching(`[A-Za-z0-9 ]+`).Draw(rt, "name")
		colored := inventory.RarityColoredName(tiers[idx], name)
		assert.Contains(rt, colored, name)
	})
}
