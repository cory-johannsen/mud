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

// fixedSrcDist is a deterministic Source for distance tests; always returns val.
type fixedSrcDist struct{ val int }

func (f fixedSrcDist) Intn(_ int) int { return f.val }

// makeDistanceCombat creates a minimal two-combatant combat with CombatRange == distanceFt.
// Player starts at GridX=0, GridY=0; NPC at GridX=distanceFt/5, GridY=0 (1D along X axis).
// distanceFt must be a multiple of 5. The player combatant has no loadout (unarmed / melee).
func makeDistanceCombat(t *testing.T, distanceFt int) (*combat.Combat, *combat.Combatant, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	attacker := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2,
		GridX: 0, GridY: 0,
	}
	target := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		GridX: distanceFt / 5, GridY: 0,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{attacker, target}, reg, nil, "")
	require.NoError(t, err)
	return cbt, attacker, target
}

// TestRangeEnforcement_MeleeAttack_BeyondMeleeRange_Misses verifies that an unarmed melee
// attacker at distance > 5 produces an out-of-range miss event without damaging the target.
func TestRangeEnforcement_MeleeAttack_BeyondMeleeRange_Misses(t *testing.T) {
	cbt, _, target := makeDistanceCombat(t, 30)

	// val=18 → Intn(20)=18 → d20=19; would normally hit AC 10 easily.
	src := fixedSrcDist{val: 18}

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil, 0)

	var found bool
	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionAttack {
			assert.Contains(t, e.Narrative, "out of melee range")
			found = true
		}
	}
	assert.True(t, found, "expected an attack event from p1")
	assert.Equal(t, 20, target.CurrentHP, "target should be unharmed when melee attacker is beyond range")
}

// TestRangeEnforcement_MeleeAttack_AtMeleeRange_CanResolve verifies that a melee attacker
// at distance == 5 does NOT get the out-of-range miss.
func TestRangeEnforcement_MeleeAttack_AtMeleeRange_CanResolve(t *testing.T) {
	cbt, _, _ := makeDistanceCombat(t, 5)

	// val=18 → d20=19; should hit.
	src := fixedSrcDist{val: 18}

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil, 0)

	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionAttack {
			assert.NotContains(t, e.Narrative, "out of melee range",
				"melee attacker at distance 5 should not get a range-miss event")
		}
	}
}

// TestRangeEnforcement_Property_MeleeAlwaysMissesIfDistanceOver5 is a property-based test
// asserting that any unarmed melee attacker beyond 5ft (>1 grid square) always produces
// the range-miss event.
func TestRangeEnforcement_Property_MeleeAlwaysMissesIfDistanceOver5(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Draw grid squares from 2..9 (distance = squares*5 = 10..45 ft, all > 5 ft).
		squares := rapid.IntRange(2, 9).Draw(rt, "squares")

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

		attacker := &combat.Combatant{
			ID: "p1", Kind: combat.KindPlayer, Name: "Player",
			CurrentHP: 20, MaxHP: 20, AC: 5, Level: 1, StrMod: 5,
			GridX: 0, GridY: 0,
		}
		target := &combat.Combatant{
			ID: "n1", Kind: combat.KindNPC, Name: "T",
			CurrentHP: 20, MaxHP: 20, AC: 5, Level: 1,
			GridX: squares, GridY: 0,
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room_a", []*combat.Combatant{attacker, target}, reg, nil, "")
		require.NoError(rt, err)

		cbt.StartRound(3)
		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "T"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		// val=18 → would hit any AC if not blocked by range.
		src := fixedSrcDist{val: 18}
		events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil, 0)

		distFt := squares * 5
		for _, e := range events {
			if e.ActorID == "p1" && e.ActionType == combat.ActionAttack {
				assert.Contains(rt, e.Narrative, "out of melee range",
					"distanceFt=%d: melee attack must miss beyond 5ft", distFt)
			}
		}
		assert.Equal(rt, 20, target.CurrentHP, "distanceFt=%d: target must be unharmed", distFt)
	})
}

// TestRangeEnforcement_RangedWeapon_ExtremeRange_Misses verifies that a ranged attacker
// beyond 4x the weapon's RangeIncrement gets an extreme-range miss.
func TestRangeEnforcement_RangedWeapon_ExtremeRange_Misses(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	// weapon RangeIncrement=20; extreme range is > 80ft.
	pistolDef := &inventory.WeaponDef{
		ID: "pistol", Name: "Pistol",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 20, ReloadActions: 1, MagazineCapacity: 15,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(pistolDef))

	attacker := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2,
		Loadout: preset, GridX: 0, GridY: 0,
	}
	target := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		// 4 * 20 = 80; GridX=17 means distance=17*5=85ft (extreme range).
		GridX: 17, GridY: 0,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{attacker, target}, reg, nil, "")
	require.NoError(t, err)

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	src := fixedSrcDist{val: 18}
	events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil, 0)

	var found bool
	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionAttack {
			assert.Contains(t, e.Narrative, "extreme range")
			found = true
		}
	}
	assert.True(t, found, "expected an attack event from p1")
	assert.Equal(t, 20, target.CurrentHP, "target should be unharmed at extreme range")
}

// TestRangeEnforcement_RangedWeapon_WithinRange_CanResolve verifies that a ranged attacker
// within normal range resolves normally (no extreme-range or melee-range miss).
func TestRangeEnforcement_RangedWeapon_WithinRange_CanResolve(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	pistolDef := &inventory.WeaponDef{
		ID: "pistol", Name: "Pistol",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 20, ReloadActions: 1, MagazineCapacity: 15,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
	preset := inventory.NewWeaponPreset()
	require.NoError(t, preset.EquipMainHand(pistolDef))

	attacker := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1, StrMod: 2,
		Loadout: preset, GridX: 0, GridY: 0,
	}
	target := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		// GridX=3 means distance=3*5=15ft (within first range increment of 20).
		GridX: 3, GridY: 0,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{attacker, target}, reg, nil, "")
	require.NoError(t, err)

	cbt.StartRound(3)
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	src := fixedSrcDist{val: 18}
	events := combat.ResolveRound(cbt, src, func(_ string, _ int) {}, nil, 0)

	for _, e := range events {
		if e.ActorID == "p1" && e.ActionType == combat.ActionAttack {
			assert.NotContains(t, e.Narrative, "extreme range")
			assert.NotContains(t, e.Narrative, "out of melee range")
		}
	}
}
