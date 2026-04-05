package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// fixedSrcAmmo is a deterministic Source for ammo consumption tests.
type fixedSrcAmmo struct{ val int }

func (f fixedSrcAmmo) Intn(_ int) int { return f.val }

// makeRangedCombat creates a two-combatant combat where the player has a ranged
// weapon loadout with a full magazine. Player at position 0, NPC at position 15
// (within first range increment of 30).
func makeRangedCombat(t *testing.T, capacity int) (*combat.Combat, *combat.Combatant, *combat.Combatant, *inventory.EquippedWeapon) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	pistolDef := &inventory.WeaponDef{
		ID: "pistol", Name: "Pistol",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 30, ReloadActions: 1, MagazineCapacity: capacity,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(pistolDef))

	player := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2, DexMod: 2,
		Loadout: preset, GridX: 0, GridY: 0,
	}
	npc := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 100, MaxHP: 100, AC: 5, Level: 1,
		GridX: 3, GridY: 0,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)
	return cbt, player, npc, preset.MainHand
}

// TestActionAttack_Ranged_ConsumesAmmo verifies that a successful ActionAttack with a
// ranged weapon decrements the magazine by 1.
//
// Precondition: Player has ranged weapon with MagazineCapacity=10 (10 loaded rounds).
// Postcondition: After ActionAttack resolves, magazine.Loaded == 9.
func TestActionAttack_Ranged_ConsumesAmmo(t *testing.T) {
	cbt, _, _, eq := makeRangedCombat(t, 10)

	require.Equal(t, 10, eq.Magazine.Loaded, "precondition: magazine must be full")

	// val=14 → d20=15; high roll should hit AC 5.
	src := fixedSrcAmmo{val: 14}

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil)

	assert.Equal(t, 9, eq.Magazine.Loaded,
		"ActionAttack with ranged weapon must consume 1 round of ammo")
}

// TestActionAttack_Ranged_ConsumesAmmo_OnMiss verifies that ammo is consumed even
// when the attack misses, since firing expends ammunition regardless of outcome.
//
// Precondition: Player has ranged weapon with MagazineCapacity=10; attack roll set very low.
// Postcondition: After ActionAttack resolves, magazine.Loaded == 9.
func TestActionAttack_Ranged_ConsumesAmmo_OnMiss(t *testing.T) {
	cbt, _, _, eq := makeRangedCombat(t, 10)

	require.Equal(t, 10, eq.Magazine.Loaded, "precondition: magazine must be full")

	// val=0 → d20=1; low roll should miss AC 5 (1 + 2 DexMod = 3 < 5).
	src := fixedSrcAmmo{val: 0}

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil)

	assert.Equal(t, 9, eq.Magazine.Loaded,
		"ActionAttack with ranged weapon must consume 1 round even on miss")
}

// TestActionAttack_Melee_DoesNotConsumeAmmo verifies that ActionAttack with a melee
// (or unarmed) attacker does NOT consume any ammo (there is no magazine to consume).
//
// Precondition: Player has no loadout (unarmed melee); target is at distance 5.
// Postcondition: No panic; attack resolves without error.
func TestActionAttack_Melee_DoesNotConsumeAmmo(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	player := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2,
		GridX: 0, GridY: 0,
	}
	npc := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 100, MaxHP: 100, AC: 5, Level: 1,
		GridX: 1, GridY: 0,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{player, npc}, reg, nil, "")
	require.NoError(t, err)

	src := fixedSrcAmmo{val: 14}
	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// Must not panic.
	events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil)
	assert.NotEmpty(t, events, "melee attack must produce at least one event")
}

// TestProperty_ActionAttack_Ranged_AlwaysConsumesOneRound is a property-based test
// verifying that any valid ActionAttack with a loaded ranged weapon always consumes
// exactly 1 round, regardless of roll outcome.
//
// Postcondition: magazine.Loaded decrements by exactly 1 for any roll value.
func TestProperty_ActionAttack_Ranged_AlwaysConsumesOneRound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(0, 19).Draw(rt, "roll")
		capacity := rapid.IntRange(1, 20).Draw(rt, "capacity")

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		pistolDef := &inventory.WeaponDef{
			ID: "pistol", Name: "Pistol",
			DamageDice: "1d6", DamageType: "piercing",
			RangeIncrement: 30, ReloadActions: 1, MagazineCapacity: capacity,
			FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
			ProficiencyCategory: "simple_ranged",
			Rarity:              "salvage",
		}
		preset := inventory.NewWeaponPreset()
		if err := preset.EquipMainHand(pistolDef); err != nil {
			rt.Fatalf("EquipMainHand: %v", err)
		}

		player := &combat.Combatant{
			ID: "p1", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2, DexMod: 2,
			Loadout: preset, GridX: 0, GridY: 0,
		}
		npc := &combat.Combatant{
			ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
			CurrentHP: 100, MaxHP: 100, AC: 5, Level: 1,
			GridX: 3, GridY: 0,
		}

		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_a", []*combat.Combatant{player, npc}, reg, nil, "")
		if err != nil {
			rt.Fatalf("StartCombat: %v", err)
		}

		eq := preset.MainHand
		loadedBefore := eq.Magazine.Loaded

		src := fixedSrcAmmo{val: roll}
		cbt.StartRound(3)
		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
		combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil)

		expected := loadedBefore - 1
		if eq.Magazine.Loaded != expected {
			rt.Errorf("expected magazine.Loaded=%d after single ranged attack, got %d (capacity=%d, roll=%d)",
				expected, eq.Magazine.Loaded, capacity, roll)
		}
	})
}
