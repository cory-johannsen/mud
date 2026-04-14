package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// makeMeleeWeaponWithBonus returns a minimal melee WeaponDef with the given bonus.
func makeMeleeWeaponWithBonus(bonus int) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:                  "vibroblade",
		Name:                "Vibroblade",
		DamageDice:          "1d10",
		DamageType:          "slashing",
		RangeIncrement:      0,
		ProficiencyCategory: "martial_melee",
		Rarity:              "street",
		Bonus:               bonus,
	}
}

func makeRangedWeaponWithBonus(bonus int) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:                  "plasma_pistol",
		Name:                "Plasma Pistol",
		DamageDice:          "1d6",
		DamageType:          "fire",
		RangeIncrement:      30,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		MagazineCapacity:    10,
		ProficiencyCategory: "martial_ranged",
		Rarity:              "mil_spec",
		Bonus:               bonus,
	}
}

// REQ-WPN-BONUS-1: A Combatant with WeaponBonus N MUST have AttackTotal = d20 + StrMod + profBonus + N.
func TestResolveAttack_WeaponBonusAddsToAttackTotal(t *testing.T) {
	attacker := &Combatant{
		ID: "p1", Kind: KindPlayer, Name: "Alice",
		MaxHP: 30, CurrentHP: 30, AC: 14,
		Level: 5, StrMod: 2, DexMod: 1,
		WeaponProficiencyRank: "trained",
		WeaponBonus:           3,
	}
	target := &Combatant{
		ID: "n1", Kind: KindNPC, Name: "Ganger",
		MaxHP: 20, CurrentHP: 20, AC: 12,
		Level: 3, StrMod: 1,
	}
	src := fixedSrc{v: 9} // Intn(20)+1 = 10

	r := ResolveAttack(attacker, target, src)

	// atkMod = StrMod(2) + CombatProficiencyBonus(5, "trained")(7) + WeaponBonus(3) = 12
	// AttackTotal = d20(10) + atkMod(12) = 22
	assert.Equal(t, 10, r.AttackRoll, "raw d20 roll must be 10")
	assert.Equal(t, 22, r.AttackTotal, "AttackTotal must include WeaponBonus")
}

// REQ-WPN-BONUS-2: A Combatant with WeaponBonus N MUST have BaseDamage include +N.
func TestResolveAttack_WeaponBonusAddsToBaseDamage(t *testing.T) {
	attacker := &Combatant{
		ID: "p1", Kind: KindPlayer, Name: "Alice",
		MaxHP: 30, CurrentHP: 30, AC: 14,
		Level: 5, StrMod: 2, DexMod: 1,
		WeaponProficiencyRank: "trained",
		WeaponBonus:           3,
	}
	target := &Combatant{
		ID: "n1", Kind: KindNPC, Name: "Ganger",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 3,
	}
	src := fixedSrc{v: 2} // d6: Intn(6)+1 = 3; d20: Intn(20)+1 = 3

	r := ResolveAttack(attacker, target, src)

	// baseDmg = dmgDie(3) + max(0, StrMod)(2) + WeaponBonus(3) = 8
	assert.Equal(t, 8, r.BaseDamage, "BaseDamage must include WeaponBonus")
}

// REQ-WPN-BONUS-3: WeaponBonus zero MUST leave attack and damage unchanged from before.
func TestResolveAttack_ZeroWeaponBonusIsNoop(t *testing.T) {
	attacker := &Combatant{
		ID: "p1", Kind: KindPlayer, Name: "Alice",
		MaxHP: 30, CurrentHP: 30, AC: 14,
		Level: 5, StrMod: 2,
		WeaponProficiencyRank: "trained",
		WeaponBonus:           0,
	}
	target := &Combatant{
		ID: "n1", Kind: KindNPC, Name: "Ganger",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 3,
	}
	src := fixedSrc{v: 9} // d20: 10; d6: min(9,5)=5 → Intn(6)=5 → dmgDie=6

	r := ResolveAttack(attacker, target, src)

	// atkMod = 2 + (5+2) + 0 = 9; AttackTotal = 10 + 9 = 19
	assert.Equal(t, 19, r.AttackTotal)
	// baseDmg = 6 + 2 + 0 = 8
	assert.Equal(t, 8, r.BaseDamage)
}

// REQ-WPN-BONUS-4: Ranged attacks with WeaponBonus MUST include the bonus in AttackTotal.
func TestResolveFirearmAttack_WeaponBonusAddsToAttackTotal(t *testing.T) {
	attacker := &Combatant{
		ID: "p1", Kind: KindPlayer, Name: "Alice",
		MaxHP: 30, CurrentHP: 30, AC: 14,
		Level: 5, StrMod: 2, DexMod: 1,
		WeaponProficiencyRank: "trained",
		WeaponBonus:           3,
	}
	target := &Combatant{
		ID: "n1", Kind: KindNPC, Name: "Ganger",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 3,
	}
	weapon := makeRangedWeaponWithBonus(3)
	src := fixedSrc{v: 9} // d20: Intn(20)+1 = 10

	r := ResolveFirearmAttack(attacker, target, weapon, 0, src)

	// total = d20(10) + DexMod(1) + profBonus(7) - rangePenalty(0) + WeaponBonus(3) = 21
	assert.Equal(t, 10, r.AttackRoll)
	assert.Equal(t, 21, r.AttackTotal, "ResolveFirearmAttack AttackTotal must include WeaponBonus")
}

// REQ-WPN-BONUS-5: Ranged attacks with WeaponBonus MUST include the bonus in BaseDamage.
func TestResolveFirearmAttack_WeaponBonusAddsToBaseDamage(t *testing.T) {
	attacker := &Combatant{
		ID: "p1", Kind: KindPlayer, Name: "Alice",
		MaxHP: 30, CurrentHP: 30, AC: 14,
		Level: 5, StrMod: 2, DexMod: 1,
		WeaponProficiencyRank: "trained",
		WeaponBonus:           3,
	}
	target := &Combatant{
		ID: "n1", Kind: KindNPC, Name: "Ganger",
		MaxHP: 20, CurrentHP: 20, AC: 12, Level: 3,
	}
	weapon := makeRangedWeaponWithBonus(3)
	// fixedSrc{v:2}: d20=3, d6: Intn(6)=2 → 3
	src := fixedSrc{v: 2}

	r := ResolveFirearmAttack(attacker, target, weapon, 0, src)

	// baseDmg = dice(3) + WeaponBonus(3) = 6
	assert.Equal(t, 6, r.BaseDamage, "ResolveFirearmAttack BaseDamage must include WeaponBonus")
}

// REQ-WPN-BONUS-6: WeaponBonus on WeaponDef MUST be loaded from YAML `bonus` field.
func TestWeaponDef_BonusFieldIsLoadable(t *testing.T) {
	def := &inventory.WeaponDef{Bonus: 3}
	assert.Equal(t, 3, def.Bonus)
}

// REQ-WPN-BONUS-7: For any WeaponBonus >= 0 and any StrMod, AttackTotal always includes the bonus.
func TestProperty_ResolveAttack_WeaponBonusAlwaysApplied(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		bonus := rapid.IntRange(0, 5).Draw(rt, "bonus")
		strMod := rapid.IntRange(-4, 5).Draw(rt, "strMod")
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")

		attacker := &Combatant{
			ID: "p", Kind: KindPlayer, Name: "P",
			MaxHP: 30, CurrentHP: 30, AC: 14,
			Level: level, StrMod: strMod,
			WeaponProficiencyRank: "trained",
			WeaponBonus:           bonus,
		}
		target := &Combatant{
			ID: "n", Kind: KindNPC, Name: "N",
			MaxHP: 20, CurrentHP: 20, AC: 99, // high AC so never a crit
			Level: 1,
		}
		src := fixedSrc{v: diceVal}
		r := ResolveAttack(attacker, target, src)

		expectedAtkMod := strMod + CombatProficiencyBonus(level, "trained") + bonus
		expectedTotal := (diceVal + 1) + expectedAtkMod
		assert.Equal(rt, expectedTotal, r.AttackTotal,
			"AttackTotal must always equal d20 + StrMod + profBonus + WeaponBonus")
	})
}
