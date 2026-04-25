package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/require"
)

// fixedSrcMod is a deterministic dice source for weapon modifier tests.
type fixedSrcMod struct{ val int }

func (f fixedSrcMod) Intn(n int) int {
	v := f.val % n
	if v < 0 {
		v = 0
	}
	return v
}

// makeModifierCombat creates a combat with a player equipped with a melee weapon
// using the given modifier, vs. a low-AC NPC with no resistances.
// Returns the combat, playerID, npcID.
func makeModifierCombat(t *testing.T, modifier string) (*combat.Combat, string, string) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})

	// Use a 1d4 weapon so dice always roll at least 1; with fixedSrcMod{val:18},
	// Intn(4)=18%4=2 → base damage = 3.  Tuned=4, Defective=2, Cursed=1 — all >0.
	swordDef := &inventory.WeaponDef{
		ID: "sword", Name: "Sword",
		DamageDice: "1d4", DamageType: "slashing",
		ProficiencyCategory: "martial_melee",
		Rarity:              "street",
	}
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(swordDef))
	// Set modifier on the EquippedWeapon.
	preset.MainHand.Modifier = modifier

	// StrMod=5 ensures base damage is high enough that cursed (-2) still hits >0.
	player := &combat.Combatant{
		ID: "p", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 50, CurrentHP: 50, AC: 10, Level: 1, StrMod: 5, DexMod: 0, Initiative: 20,
		Loadout: preset,
	}
	npc := &combat.Combatant{
		ID: "n", Kind: combat.KindNPC, Name: "Goblin",
		MaxHP: 100, CurrentHP: 100, AC: 10, Level: 1, StrMod: 0, DexMod: 0, Initiative: 5,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("zone1", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)
	_ = cbt.StartRound(3)
	return cbt, "p", "n"
}

// TestResolveRound_WeaponModifier_Tuned_DealsOneDamageMore verifies that a tuned
// weapon adds +1 damage compared to an unmodified weapon (REQ-EM-23).
func TestResolveRound_WeaponModifier_Tuned_DealsOneDamageMore(t *testing.T) {
	// Use val=3 so damage die (1d6 → Intn(6)=3 → result=4) is deterministic.
	// High attack roll ensures hit.
	src := fixedSrcMod{val: 18}

	// Baseline: no modifier.
	cbt1, pid1, nid1 := makeModifierCombat(t, "")
	require.NoError(t, cbt1.QueueAction(pid1, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	events1 := combat.ResolveRound(cbt1, src, nil, nil, 0)
	_ = events1
	npc1 := findCombatantInCbt(cbt1, nid1)
	require.NotNil(t, npc1)
	dmgNoModifier := 100 - npc1.CurrentHP

	// Tuned: +1 damage.
	cbt2, pid2, nid2 := makeModifierCombat(t, "tuned")
	require.NoError(t, cbt2.QueueAction(pid2, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	events2 := combat.ResolveRound(cbt2, src, nil, nil, 0)
	_ = events2
	npc2 := findCombatantInCbt(cbt2, nid2)
	require.NotNil(t, npc2)
	dmgTuned := 100 - npc2.CurrentHP

	require.Equal(t, dmgNoModifier+1, dmgTuned,
		"tuned modifier should add +1 damage (no-modifier=%d, tuned=%d)", dmgNoModifier, dmgTuned)
}

// TestResolveRound_WeaponModifier_Defective_DealsOneDamageLess verifies that a defective
// weapon deals -1 damage compared to an unmodified weapon (REQ-EM-23).
func TestResolveRound_WeaponModifier_Defective_DealsOneDamageLess(t *testing.T) {
	src := fixedSrcMod{val: 18}

	cbt1, pid1, nid1 := makeModifierCombat(t, "")
	require.NoError(t, cbt1.QueueAction(pid1, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	combat.ResolveRound(cbt1, src, nil, nil, 0)
	npc1 := findCombatantInCbt(cbt1, nid1)
	require.NotNil(t, npc1)
	dmgNoModifier := 100 - npc1.CurrentHP

	cbt2, pid2, nid2 := makeModifierCombat(t, "defective")
	require.NoError(t, cbt2.QueueAction(pid2, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	combat.ResolveRound(cbt2, src, nil, nil, 0)
	npc2 := findCombatantInCbt(cbt2, nid2)
	require.NotNil(t, npc2)
	dmgDefective := 100 - npc2.CurrentHP

	require.Equal(t, dmgNoModifier-1, dmgDefective,
		"defective modifier should reduce damage by 1 (no-modifier=%d, defective=%d)", dmgNoModifier, dmgDefective)
}

// TestResolveRound_WeaponModifier_Cursed_DealsTwoDamageLess verifies that a cursed
// weapon deals -2 damage compared to an unmodified weapon (REQ-EM-23).
func TestResolveRound_WeaponModifier_Cursed_DealsTwoDamageLess(t *testing.T) {
	src := fixedSrcMod{val: 18}

	cbt1, pid1, nid1 := makeModifierCombat(t, "")
	require.NoError(t, cbt1.QueueAction(pid1, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	combat.ResolveRound(cbt1, src, nil, nil, 0)
	npc1 := findCombatantInCbt(cbt1, nid1)
	require.NotNil(t, npc1)
	dmgNoModifier := 100 - npc1.CurrentHP

	cbt2, pid2, nid2 := makeModifierCombat(t, "cursed")
	require.NoError(t, cbt2.QueueAction(pid2, combat.QueuedAction{Type: combat.ActionAttack, Target: "Goblin"}))
	combat.ResolveRound(cbt2, src, nil, nil, 0)
	npc2 := findCombatantInCbt(cbt2, nid2)
	require.NotNil(t, npc2)
	dmgCursed := 100 - npc2.CurrentHP

	require.Equal(t, dmgNoModifier-2, dmgCursed,
		"cursed modifier should reduce damage by 2 (no-modifier=%d, cursed=%d)", dmgNoModifier, dmgCursed)
}
