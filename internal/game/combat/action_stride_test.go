package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionStride_Cost_IsOneAP(t *testing.T) {
	assert.Equal(t, 1, combat.ActionStride.Cost())
}

func TestActionStride_String(t *testing.T) {
	assert.Equal(t, "stride", combat.ActionStride.String())
}

// makeStrideCombat creates a minimal two-combatant combat with positions set so that
// combatantDist(player, npc) == distanceFt.
// Player starts at Position=0; NPC at Position=distanceFt.
func makeStrideCombat(t *testing.T, distanceFt int) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	actor := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		Position: 0,
	}
	other := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		CurrentHP: 20, MaxHP: 20, AC: 10, Level: 1,
		Position: distanceFt,
	}

	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room_a", []*combat.Combatant{actor, other}, reg, nil, "")
	require.NoError(t, err)
	return cbt
}

// TestResolveRound_ActionStride_Toward_IncreasesPosition verifies that striding toward
// increases the player's Position by 25.
func TestResolveRound_ActionStride_Toward_IncreasesPosition(t *testing.T) {
	src := fixedSrcDist{val: 1}
	// Player at 0, NPC at 50; distance=50.
	cbt := makeStrideCombat(t, 50)
	p1 := cbt.GetCombatant("p1")
	require.NotNil(t, p1)

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	_ = combat.ResolveRound(cbt, src, func(id string, hp int) {})
	assert.Equal(t, 25, p1.Position, "player.Position should be 25 after striding toward from 0")
}

// TestResolveRound_ActionStride_Away_DecreasesPosition verifies that striding away
// decreases the player's Position by 25.
func TestResolveRound_ActionStride_Away_DecreasesPosition(t *testing.T) {
	src := fixedSrcDist{val: 1}
	// Player at 0, NPC at 25.
	cbt := makeStrideCombat(t, 25)
	p1 := cbt.GetCombatant("p1")
	require.NotNil(t, p1)
	p1.Position = 25 // start player at 25 so away stride goes to 0

	_ = cbt.StartRound(3)
	_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "away"})
	_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	_ = combat.ResolveRound(cbt, src, func(id string, hp int) {})
	assert.Equal(t, 0, p1.Position, "player.Position should be 0 after striding away from 25")
}
