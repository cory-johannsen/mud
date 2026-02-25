package combat

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

// fixedSrc always returns min(v, n-1), enabling deterministic test rolls.
type fixedSrc struct{ v int }

func (f fixedSrc) Intn(n int) int {
	if f.v >= n {
		return n - 1
	}
	return f.v
}

// makeTestWeapon returns a minimal WeaponDef suitable for resolver tests.
func makeTestWeapon(damageDice string) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:         "test-pistol",
		Name:       "Test Pistol",
		DamageDice: damageDice,
		DamageType: "piercing",
		FiringModes: []inventory.FiringMode{
			inventory.FiringModeSingle,
		},
		MagazineCapacity: 10,
	}
}

// makeTestGrenade returns a minimal ExplosiveDef suitable for resolver tests.
func makeTestGrenade(damageDice string, saveDC int) *inventory.ExplosiveDef {
	return &inventory.ExplosiveDef{
		ID:         "test-grenade",
		Name:       "Test Grenade",
		DamageDice: damageDice,
		DamageType: "fire",
		AreaType:   inventory.AreaTypeBurst,
		SaveType:   "reflex",
		SaveDC:     saveDC,
		Fuse:       inventory.FuseImmediate,
	}
}

// makeCombatant returns a Combatant with the given fields set.
func makeCombatant(id string, ac, level, strMod, dexMod int) *Combatant {
	return &Combatant{
		ID:        id,
		Kind:      KindPlayer,
		Name:      id,
		MaxHP:     30,
		CurrentHP: 30,
		AC:        ac,
		Level:     level,
		StrMod:    strMod,
		DexMod:    dexMod,
	}
}

// TestResolveFirearmAttack_HitWithinRange verifies that roll=18 with DexMod=2,
// profBonus=2 (level 1), rangeIncrements=0 yields AttackTotal=22 vs AC 14 → Success.
func TestResolveFirearmAttack_HitWithinRange(t *testing.T) {
	// fixedSrc.v=17 → Intn(20) returns 17 → d20 = 18
	src := fixedSrc{v: 17}
	attacker := makeCombatant("attacker", 10, 1, 0, 2)
	target := makeCombatant("target", 14, 1, 0, 0)
	weapon := makeTestWeapon("1d6")

	result := ResolveFirearmAttack(attacker, target, weapon, 0, src)

	if result.AttackRoll != 18 {
		t.Errorf("expected AttackRoll=18, got %d", result.AttackRoll)
	}
	if result.AttackTotal != 22 {
		t.Errorf("expected AttackTotal=22 (18+2+2-0), got %d", result.AttackTotal)
	}
	if result.Outcome != Success && result.Outcome != CritSuccess {
		t.Errorf("expected hit outcome, got %v", result.Outcome)
	}
}

// TestResolveFirearmAttack_PenaltyReducesTotal verifies that range=2 yields
// a strictly lower AttackTotal than range=0 for the same fixed roll.
func TestResolveFirearmAttack_PenaltyReducesTotal(t *testing.T) {
	src := fixedSrc{v: 10}
	attacker := makeCombatant("attacker", 10, 1, 0, 2)
	target := makeCombatant("target", 14, 1, 0, 0)
	weapon := makeTestWeapon("1d6")

	r0 := ResolveFirearmAttack(attacker, target, weapon, 0, src)
	r2 := ResolveFirearmAttack(attacker, target, weapon, 2, src)

	if r2.AttackTotal >= r0.AttackTotal {
		t.Errorf("expected range=2 total (%d) < range=0 total (%d)", r2.AttackTotal, r0.AttackTotal)
	}
}

// TestResolveExplosive_AllTargetsDamaged verifies that a low save roll causes
// all targets to take damage (Failure outcome → BaseDamage > 0).
func TestResolveExplosive_AllTargetsDamaged(t *testing.T) {
	// fixedSrc.v=0 → d20=1 (low save) and damage dice also roll low but > 0
	// Use a grenade with fixed 4 damage (0d0+4 not valid; use 1d4 with v=3 → 4)
	// With v=3: Intn(20)=3 → save roll=4; Intn(4)=3 → dmg die=4
	src := fixedSrc{v: 3}
	grenade := makeTestGrenade("1d4", 15) // SaveDC=15, save roll=4+DexMod → failure
	targets := []*Combatant{
		makeCombatant("t1", 10, 1, 0, 0),
		makeCombatant("t2", 10, 1, 0, 0),
	}

	results := ResolveExplosive(grenade, targets, src)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.BaseDamage <= 0 {
			t.Errorf("expected BaseDamage>0 for target %s, got %d", r.TargetID, r.BaseDamage)
		}
	}
}

// TestResolveExplosive_CritSuccessSave_ZeroDamage verifies that a high save roll
// (SaveTotal >= SaveDC+10) yields CritSuccess and BaseDamage==0.
func TestResolveExplosive_CritSuccessSave_ZeroDamage(t *testing.T) {
	// fixedSrc.v=19 → Intn(20)=19 → d20=20 for save; damage dice also return 19→max
	// SaveDC=5; save roll=20+0=20 >= 5+10=15 → CritSuccess
	src := fixedSrc{v: 19}
	grenade := makeTestGrenade("1d6", 5)
	targets := []*Combatant{
		makeCombatant("t1", 10, 1, 0, 0),
	}

	results := ResolveExplosive(grenade, targets, src)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.SaveResult != CritSuccess {
		t.Errorf("expected CritSuccess, got %v", r.SaveResult)
	}
	if r.BaseDamage != 0 {
		t.Errorf("expected BaseDamage=0 for CritSuccess, got %d", r.BaseDamage)
	}
}

// TestProperty_ExplosiveDamage_NeverNegative is a property-based test asserting
// that BaseDamage is never negative for any combination of roll and SaveDC.
func TestProperty_ExplosiveDamage_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rollVal := rapid.IntRange(0, 19).Draw(rt, "rollVal")
		saveDC := rapid.IntRange(5, 25).Draw(rt, "saveDC")

		src := fixedSrc{v: rollVal}
		grenade := makeTestGrenade("1d6", saveDC)
		target := makeCombatant("t1", 10, 1, 0, 0)

		results := ResolveExplosive(grenade, []*Combatant{target}, src)
		if len(results) != 1 {
			rt.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].BaseDamage < 0 {
			rt.Errorf("BaseDamage must never be negative, got %d", results[0].BaseDamage)
		}
	})
}

// TestProperty_FirearmAttack_RangePenaltyMonotone is a property-based test asserting
// that a higher rangeIncrements value never yields a higher AttackTotal than a lower value
// for the same fixed roll.
func TestProperty_FirearmAttack_RangePenaltyMonotone(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rollVal := rapid.IntRange(0, 19).Draw(rt, "rollVal")
		rangeA := rapid.IntRange(0, 5).Draw(rt, "rangeA")
		rangeB := rapid.IntRange(0, 5).Draw(rt, "rangeB")

		src := fixedSrc{v: rollVal}
		attacker := makeCombatant("attacker", 10, 1, 0, 2)
		target := makeCombatant("target", 14, 1, 0, 0)
		weapon := makeTestWeapon("1d6")

		rA := ResolveFirearmAttack(attacker, target, weapon, rangeA, src)
		rB := ResolveFirearmAttack(attacker, target, weapon, rangeB, src)

		// If rangeA < rangeB then totalA >= totalB (more range = lower or equal total)
		if rangeA < rangeB && rA.AttackTotal < rB.AttackTotal {
			rt.Errorf("rangeA=%d gave total=%d but rangeB=%d gave total=%d; expected monotone decrease",
				rangeA, rA.AttackTotal, rangeB, rB.AttackTotal)
		}
	})
}
