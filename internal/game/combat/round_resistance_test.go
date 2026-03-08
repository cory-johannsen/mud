package combat_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeResistanceCombat(t *testing.T, playerDmgType string, npcResistances map[string]int, npcWeaknesses map[string]int) (*combat.Combat, string, string) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{
			ID: "p", Kind: combat.KindPlayer, Name: "Player",
			MaxHP: 30, CurrentHP: 30, AC: 10, Level: 1, StrMod: 3,
			WeaponDamageType: playerDmgType,
		},
		{
			ID: "n", Kind: combat.KindNPC, Name: "NPC",
			MaxHP: 50, CurrentHP: 50, AC: 10,
			Resistances: npcResistances,
			Weaknesses:  npcWeaknesses,
		},
	}
	cbt, err := eng.StartCombat("zone1", combatants, reg, nil, "")
	require.NoError(t, err)
	_ = cbt.StartRound(3)
	return cbt, "p", "n"
}

func findCombatantInCbt(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// TestResolveRound_Resistance_NarrativeContainsResisted verifies the narrative mentions resistance.
func TestResolveRound_Resistance_NarrativeContainsResisted(t *testing.T) {
	src := fixedSrc{val: 18} // high roll → guaranteed hit
	cbt, playerID, npcID := makeResistanceCombat(t, "fire", map[string]int{"fire": 5}, nil)
	_ = npcID
	err := cbt.QueueAction(playerID, combat.QueuedAction{Type: combat.ActionAttack, Target: "NPC"})
	require.NoError(t, err)
	events := combat.ResolveRound(cbt, src, nil)
	require.NotEmpty(t, events)
	found := false
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Narrative), "resist") {
			found = true
		}
	}
	assert.True(t, found, "expected 'resist' in narrative, got: %v", events)
}

// TestResolveRound_Weakness_NarrativeContainsWeak verifies the narrative mentions weakness.
func TestResolveRound_Weakness_NarrativeContainsWeak(t *testing.T) {
	src := fixedSrc{val: 18}
	cbt, playerID, npcID := makeResistanceCombat(t, "electricity", nil, map[string]int{"electricity": 4})
	_ = npcID
	err := cbt.QueueAction(playerID, combat.QueuedAction{Type: combat.ActionAttack, Target: "NPC"})
	require.NoError(t, err)
	events := combat.ResolveRound(cbt, src, nil)
	require.NotEmpty(t, events)
	found := false
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Narrative), "weak") {
			found = true
		}
	}
	assert.True(t, found, "expected 'weak' in narrative, got: %v", events)
}

// TestResolveRound_Resistance_MinZero verifies resistance cannot reduce HP below zero unexpectedly.
func TestResolveRound_Resistance_MinZero(t *testing.T) {
	src := fixedSrc{val: 18}
	cbt, playerID, npcID := makeResistanceCombat(t, "piercing", map[string]int{"piercing": 1000}, nil)
	err := cbt.QueueAction(playerID, combat.QueuedAction{Type: combat.ActionAttack, Target: "NPC"})
	require.NoError(t, err)
	combat.ResolveRound(cbt, src, nil)
	npc := findCombatantInCbt(cbt, npcID)
	require.NotNil(t, npc)
	assert.GreaterOrEqual(t, npc.CurrentHP, 0, "HP must not go below 0")
	assert.LessOrEqual(t, npc.CurrentHP, 50, "HP should not exceed max")
}

// TestResolveRound_NoResistance_UnchangedDamage verifies no adjustment when types don't match.
func TestResolveRound_NoResistance_UnchangedDamage(t *testing.T) {
	src := fixedSrc{val: 18}
	cbt, playerID, npcID := makeResistanceCombat(t, "fire", map[string]int{"piercing": 5}, nil)
	err := cbt.QueueAction(playerID, combat.QueuedAction{Type: combat.ActionAttack, Target: "NPC"})
	require.NoError(t, err)
	combat.ResolveRound(cbt, src, nil)
	npc := findCombatantInCbt(cbt, npcID)
	require.NotNil(t, npc)
	// NPC should have taken normal damage (no resistance to fire)
	assert.Less(t, npc.CurrentHP, 50)
}
