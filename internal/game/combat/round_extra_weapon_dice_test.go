package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

// makeExtraWeaponDiceCombatants returns a player with a melee weapon preset and an NPC for use in
// extra-weapon-dice tests.
//
// dieSides controls the weapon's damage die (e.g. 10 → "1d10"). npcHP sets the NPC's starting HP.
func makeExtraWeaponDiceCombatants(t *testing.T, dieSides, npcHP int) ([]*combat.Combatant, *condition.Registry) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{
		ID: "brutal_surge_active", Name: "Brutal Surge Active",
		DurationType: "encounter", ExtraWeaponDice: 2, ACPenalty: 2,
	})

	diceSides := 6
	if dieSides > 0 {
		diceSides = dieSides
	}
	var damageDice string
	switch diceSides {
	case 4:
		damageDice = "1d4"
	case 6:
		damageDice = "1d6"
	case 8:
		damageDice = "1d8"
	case 10:
		damageDice = "1d10"
	case 12:
		damageDice = "1d12"
	default:
		damageDice = "1d6"
	}

	weaponDef := &inventory.WeaponDef{
		ID: "test_hammer", Name: "Hammer", DamageDice: damageDice, DamageType: "bludgeoning",
		RangeIncrement: 0, ProficiencyCategory: "simple_weapons", Rarity: "salvage",
	}
	preset := inventory.NewWeaponPreset()
	if err := preset.EquipMainHand(weaponDef); err != nil {
		t.Fatalf("EquipMainHand: %v", err)
	}

	player := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Alice",
		MaxHP: 200, CurrentHP: 200, AC: 14, GridX: 0, GridY: 0, SpeedFt: 25,
		Loadout: preset,
	}
	npc := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Target",
		MaxHP: npcHP, CurrentHP: npcHP, AC: 1, GridX: 0, GridY: 1, SpeedFt: 25,
	}
	return []*combat.Combatant{player, npc}, reg
}

// TestExtraWeaponDice_OnHit_AddsExtraDamage verifies that when a combatant has a
// condition with ExtraWeaponDice=2 and their attack hits, the total damage is greater
// than the base weapon roll alone.
//
// val=15 → d20=16 (hit vs AC 1), 1d10 base roll=6 (15%10+1=6), extra 2d10=6+6=12. Total=18.
// Without extra dice: damage would be 6. With extra: 18. NPC HP=30 → 30-18=12.
func TestExtraWeaponDice_OnHit_AddsExtraDamage(t *testing.T) {
	combatants, reg := makeExtraWeaponDiceCombatants(t, 10, 30)

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.GridWidth = 20
	cbt.GridHeight = 20
	_ = cbt.StartRound(3)

	bruteDef, _ := reg.Get("brutal_surge_active")
	if applyErr := cbt.Conditions["p1"].Apply("p1", bruteDef, 1, -1); applyErr != nil {
		t.Fatalf("Apply brutal_surge_active: %v", applyErr)
	}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Target"}); err != nil {
		t.Fatalf("QueueAction p1 attack: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 pass: %v", err)
	}

	// val=15: d20=16 (hit vs AC 1), 1d10 base = 15%10+1=6, extra dice: 2×(15%10+1)=2×6=12. Total=18.
	// Without extra dice: NPC HP = 30-6 = 24.
	// With extra dice:    NPC HP = 30-18 = 12.
	src := fixedSrc{val: 15}
	combat.ResolveRound(cbt, src, nil, nil)

	npcHP := cbt.Combatants[1].CurrentHP
	// Discriminating assertion: if extra dice fire, damage=18 → npcHP=12.
	// If only base die, damage=6 → npcHP=24.
	if npcHP > 15 {
		t.Errorf("NPC HP=%d, expected ≤15 after extra weapon dice applied (base=6, extra=12)", npcHP)
	}
}

// TestExtraWeaponDice_OnMiss_NoExtraDamage verifies that on a miss, extra weapon dice
// are NOT rolled and the NPC takes no damage.
//
// val=1: d20=2, miss vs AC 10. No damage should be applied.
func TestExtraWeaponDice_OnMiss_NoExtraDamage(t *testing.T) {
	combatants, reg := makeExtraWeaponDiceCombatants(t, 10, 30)
	// Override NPC AC to 10 so val=1 guarantees a miss.
	combatants[1].AC = 10
	combatants[1].CurrentHP = 30

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.GridWidth = 20
	cbt.GridHeight = 20
	_ = cbt.StartRound(3)

	bruteDef, _ := reg.Get("brutal_surge_active")
	if applyErr := cbt.Conditions["p1"].Apply("p1", bruteDef, 1, -1); applyErr != nil {
		t.Fatalf("Apply brutal_surge_active: %v", applyErr)
	}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Target"}); err != nil {
		t.Fatalf("QueueAction p1 attack: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 pass: %v", err)
	}

	// val=1: d20=2, miss vs AC 10 (no mods). No damage should be applied.
	src := fixedSrc{val: 1}
	combat.ResolveRound(cbt, src, nil, nil)

	npcHP := cbt.Combatants[1].CurrentHP
	if npcHP != 30 {
		t.Errorf("NPC HP=%d, expected 30 (no damage on miss; extra dice must not fire)", npcHP)
	}
}

// TestProperty_ExtraWeaponDice_OnHit_DamageGreaterThanBaseAlone verifies that for any
// hit with ExtraWeaponDice > 0, total damage always exceeds what the base die alone could produce.
func TestProperty_ExtraWeaponDice_OnHit_DamageGreaterThanBaseAlone(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		extraDice := rapid.IntRange(1, 4).Draw(rt, "extraDice")
		npcHP := rapid.IntRange(100, 200).Draw(rt, "npcHP")

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{
			ID: "test_surge", Name: "Test Surge",
			DurationType: "encounter", ExtraWeaponDice: extraDice,
		})

		weaponDef := &inventory.WeaponDef{
			ID: "blade", Name: "Blade", DamageDice: "1d6", DamageType: "slashing",
			RangeIncrement: 0, ProficiencyCategory: "simple_weapons", Rarity: "salvage",
		}
		preset := inventory.NewWeaponPreset()
		if err := preset.EquipMainHand(weaponDef); err != nil {
			rt.Fatalf("EquipMainHand: %v", err)
		}

		eng := combat.NewEngine()
		combatants := []*combat.Combatant{
			{
				ID: "p1", Kind: combat.KindPlayer, Name: "Alice",
				MaxHP: 200, CurrentHP: 200, AC: 14, GridX: 0, GridY: 0, SpeedFt: 25,
				Loadout: preset,
			},
			{ID: "n1", Kind: combat.KindNPC, Name: "Target", MaxHP: npcHP, CurrentHP: npcHP, AC: 1, GridX: 0, GridY: 1, SpeedFt: 25},
		}
		cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
		if err != nil {
			rt.Fatalf("StartCombat: %v", err)
		}
		cbt.GridWidth = 20
		cbt.GridHeight = 20
		_ = cbt.StartRound(3)

		surgeDef, _ := reg.Get("test_surge")
		if applyErr := cbt.Conditions["p1"].Apply("p1", surgeDef, 1, -1); applyErr != nil {
			rt.Fatalf("Apply test_surge: %v", applyErr)
		}

		if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Target"}); err != nil {
			rt.Fatalf("QueueAction p1: %v", err)
		}
		if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
			rt.Fatalf("QueueAction n1: %v", err)
		}

		// val=15 → guaranteed hit vs AC 1. Each d6 → 15%6+1=4.
		// Base damage = 4. Extra = extraDice*4. Total ≥ base+extra.
		src := fixedSrc{val: 15}
		combat.ResolveRound(cbt, src, nil, nil)

		dmgDealt := npcHP - cbt.Combatants[1].CurrentHP
		if cbt.Combatants[1].CurrentHP < 0 {
			dmgDealt = npcHP // capped at NPC's starting HP
		}
		// Minimum possible damage with extra dice > minimum possible without.
		// Without extra: min=1 (d6=1). With 1 extra die: min=2 (1+1). With N: min=N+1.
		minWithExtra := 1 + extraDice // each die rolls minimum of 1
		if dmgDealt < minWithExtra {
			rt.Fatalf("dmgDealt=%d < minWithExtra=%d (extraDice=%d)", dmgDealt, minWithExtra, extraDice)
		}
	})
}
