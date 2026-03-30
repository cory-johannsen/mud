package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// fixedSrcFirearm is a deterministic Source for firearm field tests.
type fixedSrcFirearm struct{ val int }

func (f fixedSrcFirearm) Intn(_ int) int { return f.val }

func makeFirearmWeaponDef(name, damageType string) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:                  "test-gun",
		Name:                name,
		DamageDice:          "1d6",
		DamageType:          damageType,
		RangeIncrement:      30,
		ReloadActions:       1,
		MagazineCapacity:    10,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
}

// TestResolveFirearmAttack_PopulatesDamageType verifies that ResolveFirearmAttack
// sets AttackResult.DamageType from weapon.DamageType.
//
// Precondition: weapon.DamageType is non-empty.
// Postcondition: result.DamageType equals weapon.DamageType.
func TestResolveFirearmAttack_PopulatesDamageType(t *testing.T) {
	src := fixedSrcFirearm{val: 14}
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, DexMod: 2,
		WeaponProficiencyRank: "trained",
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	weapon := makeFirearmWeaponDef("Pistol", "piercing")

	result := combat.ResolveFirearmAttack(attacker, target, weapon, 0, src)

	assert.Equal(t, "piercing", result.DamageType,
		"DamageType must be propagated from weapon.DamageType")
}

// TestResolveFirearmAttack_PopulatesWeaponName verifies that ResolveFirearmAttack
// sets AttackResult.WeaponName from weapon.Name.
//
// Precondition: weapon.Name is non-empty.
// Postcondition: result.WeaponName equals weapon.Name.
func TestResolveFirearmAttack_PopulatesWeaponName(t *testing.T) {
	src := fixedSrcFirearm{val: 14}
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, DexMod: 2,
		WeaponProficiencyRank: "trained",
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	weapon := makeFirearmWeaponDef("Custom Revolver", "piercing")

	result := combat.ResolveFirearmAttack(attacker, target, weapon, 0, src)

	assert.Equal(t, "Custom Revolver", result.WeaponName,
		"WeaponName must be propagated from weapon.Name")
}

// TestResolveFirearmAttack_EmptyDamageType_WhenWeaponHasNoType verifies that
// result.DamageType is empty when weapon.DamageType is empty.
//
// Precondition: weapon.DamageType is empty string.
// Postcondition: result.DamageType is empty string.
func TestResolveFirearmAttack_EmptyDamageType_WhenWeaponHasNoType(t *testing.T) {
	src := fixedSrcFirearm{val: 10}
	attacker := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 1, DexMod: 1,
	}
	target := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "NPC",
		MaxHP: 10, CurrentHP: 10, AC: 10,
	}
	weapon := &inventory.WeaponDef{
		ID:         "bare-gun",
		Name:       "",
		DamageDice: "1d4",
		DamageType: "",
	}

	result := combat.ResolveFirearmAttack(attacker, target, weapon, 0, src)

	assert.Equal(t, "", result.DamageType)
	assert.Equal(t, "", result.WeaponName)
}

// TestProperty_ResolveFirearmAttack_DamageTypeAlwaysMatchesWeapon is a property-based
// test verifying that result.DamageType always matches weapon.DamageType.
//
// Postcondition: result.DamageType == weapon.DamageType for all valid inputs.
func TestProperty_ResolveFirearmAttack_DamageTypeAlwaysMatchesWeapon(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(0, 19).Draw(rt, "roll")
		damageType := rapid.SampledFrom([]string{"", "piercing", "slashing", "fire", "cold"}).Draw(rt, "damageType")
		weaponName := rapid.StringN(0, 30, -1).Draw(rt, "weaponName")

		src := fixedSrcFirearm{val: roll}
		attacker := &combat.Combatant{
			ID: "p", Kind: combat.KindPlayer, Name: "Player",
			MaxHP: 20, CurrentHP: 20, AC: 10, Level: 1, DexMod: 1,
		}
		target := &combat.Combatant{
			ID: "n", Kind: combat.KindNPC, Name: "NPC",
			MaxHP: 10, CurrentHP: 10, AC: 10,
		}
		weapon := &inventory.WeaponDef{
			ID:         "gun",
			Name:       weaponName,
			DamageDice: "1d6",
			DamageType: damageType,
		}

		result := combat.ResolveFirearmAttack(attacker, target, weapon, 0, src)

		if result.DamageType != damageType {
			rt.Errorf("expected DamageType=%q, got %q", damageType, result.DamageType)
		}
		if result.WeaponName != weaponName {
			rt.Errorf("expected WeaponName=%q, got %q", weaponName, result.WeaponName)
		}
	})
}
