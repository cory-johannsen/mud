package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// makeEngineWithNPC creates an Engine with a single combat containing one player and one NPC.
// It returns the engine and the NPC combatant for further manipulation.
func makeEngineWithNPC(t *testing.T) (*combat.Engine, *combat.Combatant) {
	t.Helper()
	eng := combat.NewEngine()
	npcCombatant := &combat.Combatant{ID: "npc1", Kind: combat.KindNPC, Name: "Goblin", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1}
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
		npcCombatant,
	}
	_, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	return eng, npcCombatant
}

// TestEngine_IsNPCInCombat_TrueWhenInCombat verifies that a living NPC combatant
// in an active combat is detected by IsNPCInCombat.
func TestEngine_IsNPCInCombat_TrueWhenInCombat(t *testing.T) {
	eng, _ := makeEngineWithNPC(t)
	if !eng.IsNPCInCombat("npc1") {
		t.Error("IsNPCInCombat returned false for a living NPC in active combat, want true")
	}
}

// TestEngine_IsNPCInCombat_FalseWhenNotInCombat verifies that an NPC ID not
// present in any active combat returns false.
func TestEngine_IsNPCInCombat_FalseWhenNotInCombat(t *testing.T) {
	eng := combat.NewEngine()
	if eng.IsNPCInCombat("npc_absent") {
		t.Error("IsNPCInCombat returned true for an NPC not in any combat, want false")
	}
}

// TestEngine_IsNPCInCombat_FalseWhenDead verifies that an NPC that is dead
// (Dead == true) is not reported as in combat.
func TestEngine_IsNPCInCombat_FalseWhenDead(t *testing.T) {
	eng, npcCombatant := makeEngineWithNPC(t)
	// Mark the NPC as dead.
	npcCombatant.CurrentHP = 0
	npcCombatant.Dead = true
	if eng.IsNPCInCombat("npc1") {
		t.Error("IsNPCInCombat returned true for a dead NPC, want false")
	}
}
