package combat_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// makeSeekCombat builds a Combat where the player attacks a hidden NPC.
// The player (p1) attacks the NPC (n1) which has Hidden=true.
func makeSeekCombat(t *testing.T) (*combat.Combat, *combat.Combatant, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "grabbed", Name: "Grabbed", DurationType: "rounds", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	eng := combat.NewEngine()
	// Player has high StrMod so attacks that pass flat check will hit AC=1.
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Player", CurrentHP: 20, MaxHP: 20, AC: 14, Level: 1, StrMod: 5},
		{ID: "n1", Kind: combat.KindNPC, Name: "Lurker", CurrentHP: 10, MaxHP: 10, AC: 1, Level: 1, StrMod: 1},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	_ = cbt.StartRound(3)

	player := cbt.Combatants[0]
	npc := cbt.Combatants[1]
	npc.Hidden = true
	npc.RevealedUntilRound = 0

	return cbt, player, npc
}

// TestHiddenNPC_FlatCheckEnforced verifies that when the flat check fails (roll ≤ 10),
// the player's attack against a hidden NPC is blocked with a "can't locate" narrative.
//
// fixedSrc{val:4} → Intn(20)=4 → roll=5 ≤ 10 → flat check fails.
func TestHiddenNPC_FlatCheckEnforced(t *testing.T) {
	cbt, player, npc := makeSeekCombat(t)
	src := fixedSrc{val: 4}

	require.NoError(t, cbt.QueueAction(player.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: npc.Name}))
	require.NoError(t, cbt.QueueAction(npc.ID, combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, src, noopUpdater, nil, 0)

	assert.Equal(t, npc.MaxHP, npc.CurrentHP, "NPC should be unharmed when flat check fails")

	found := false
	for _, e := range events {
		if e.ActionType == combat.ActionAttack && strings.Contains(e.Narrative, "can't locate") {
			found = true
		}
	}
	assert.True(t, found, "expected a 'can't locate' narrative event")
}

// TestHiddenNPC_RevealedBySeek_FlatCheckBypassed verifies that when RevealedUntilRound > cbt.Round,
// the flat check is skipped and the attack proceeds normally.
//
// fixedSrc{val:4} would fail the flat check — but it must be skipped when NPC is revealed.
func TestHiddenNPC_RevealedBySeek_FlatCheckBypassed(t *testing.T) {
	cbt, player, npc := makeSeekCombat(t)
	npc.RevealedUntilRound = cbt.Round + 1

	src := fixedSrc{val: 4}

	require.NoError(t, cbt.QueueAction(player.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: npc.Name}))
	require.NoError(t, cbt.QueueAction(npc.ID, combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, src, noopUpdater, nil, 0)

	attacked := false
	for _, e := range events {
		if e.ActionType == combat.ActionAttack && e.ActorID == player.ID && e.AttackResult != nil {
			attacked = true
		}
	}
	assert.True(t, attacked, "expected an attack event with AttackResult when NPC is revealed")
}

// TestPropertyHiddenNPC_FlatCheck verifies flat check behavior for all possible roll values.
func TestPropertyHiddenNPC_FlatCheck(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		flatVal := rapid.IntRange(0, 19).Draw(rt, "flatVal")
		roll := flatVal + 1 // Intn(20) returns flatVal, so roll = flatVal+1

		cbt, player, npc := makeSeekCombat(t)
		src := fixedSrc{val: flatVal}

		require.NoError(rt, cbt.QueueAction(player.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: npc.Name}))
		require.NoError(rt, cbt.QueueAction(npc.ID, combat.QueuedAction{Type: combat.ActionPass}))

		origHP := npc.CurrentHP
		events := combat.ResolveRound(cbt, src, noopUpdater, nil, 0)

		if roll <= 10 {
			require.Equal(rt, origHP, npc.CurrentHP, "flat check miss: NPC HP must be unchanged")
			found := false
			for _, e := range events {
				if strings.Contains(e.Narrative, "can't locate") {
					found = true
				}
			}
			require.True(rt, found, "flat check miss: must emit can't-locate narrative")
		} else {
			require.False(rt, npc.Hidden, "flat check pass: NPC Hidden must be cleared")
		}
	})
}
