package combat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// TestHazard_OnEnter_DealsDamage verifies that entering a hazardous cell
// applies damage to the combatant.
func TestHazard_OnEnter_DealsDamage(t *testing.T) {
	src := fixedSrc{val: 1}
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		CurrentHP: 20, MaxHP: 20, AC: 12,
		GridX: 0, GridY: 5,
	}
	hazDef := &world.HazardDef{
		ID: "fire_vent", Trigger: "on_enter",
		DamageExpr: "1d4", DamageType: "fire",
		Message: "flames lick at you",
	}
	tc := combat.TerrainCell{X: 1, Y: 5, Type: combat.TerrainHazardous,
		Hazard: &combat.CellHazard{Def: hazDef},
	}
	hpBefore := actor.CurrentHP
	events := combat.ApplyCellHazardForTest(nil, actor, tc, "on_enter", src)
	if actor.CurrentHP >= hpBefore {
		t.Errorf("on_enter hazard must deal damage; HP before=%d after=%d", hpBefore, actor.CurrentHP)
	}
	if len(events) == 0 {
		t.Error("applyCellHazard must emit at least one RoundEvent")
	}
}

// TestHazard_RoundStart_FiresRoundStart verifies that round_start hazards
// fire on combatants occupying hazardous cells.
func TestHazard_RoundStart_FiresRoundStart(t *testing.T) {
	src := &fixedSrc{val: 2}
	reg := condition.NewRegistry()
	eng := combat.NewEngine()
	actor := &combat.Combatant{
		ID: "actor", Name: "actor", Kind: combat.KindPlayer,
		CurrentHP: 30, MaxHP: 30, AC: 12, Level: 1,
		GridX: 3, GridY: 3, Initiative: 10,
	}
	cbt, err := eng.StartCombat("room1", []*combat.Combatant{actor}, reg, nil, "")
	require.NoError(t, err)
	cbt.GridWidth = 10
	cbt.GridHeight = 10
	hazDef := &world.HazardDef{
		ID: "acid_pool", Trigger: "round_start",
		DamageExpr: "1d4", DamageType: "acid",
	}
	cbt.Terrain = map[combat.GridCell]*combat.TerrainCell{
		{X: 3, Y: 3}: {X: 3, Y: 3, Type: combat.TerrainHazardous,
			Hazard: &combat.CellHazard{Def: hazDef},
		},
	}
	hpBefore := actor.CurrentHP
	cbt.StartRoundWithSrc(2, src)
	if actor.CurrentHP >= hpBefore {
		t.Errorf("round_start hazard must deal damage; HP before=%d after=%d", hpBefore, actor.CurrentHP)
	}
}

// TestHazard_CombatStart_NoDoubleFireRound1 verifies that
// a combatant placed on a hazardous cell at combat start fires on_enter once
// and round_start does NOT double-fire in round 1.
func TestHazard_CombatStart_NoDoubleFireRound1(t *testing.T) {
	t.Skip("implement after gameserver combat-start placement hazard wiring lands")
}
