package combat_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestHasUndowedPlayers_ReturnsTrueWhenAtLeastOnePlayerHasHP verifies that a
// combat with one living player and one downed player returns true.
//
// Precondition: two player combatants; one with HP=10, one with HP=0.
// Postcondition: HasUndowedPlayers returns true.
func TestHasUndowedPlayers_ReturnsTrueWhenAtLeastOnePlayerHasHP(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 10},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 0},
		{ID: "n1", Kind: combat.KindNPC, Name: "Thug", MaxHP: 10, CurrentHP: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	assert.True(t, cbt.HasUndowedPlayers(), "expected true when at least one player has HP > 0")
}

// TestHasUndowedPlayers_ReturnsFalseWhenAllPlayersAtZeroHP verifies that a
// combat where every player combatant is at 0 HP returns false.
//
// Precondition: two player combatants both at HP=0; one living NPC.
// Postcondition: HasUndowedPlayers returns false.
func TestHasUndowedPlayers_ReturnsFalseWhenAllPlayersAtZeroHP(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 0},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 0},
		{ID: "n1", Kind: combat.KindNPC, Name: "Thug", MaxHP: 10, CurrentHP: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	assert.False(t, cbt.HasUndowedPlayers(), "expected false when all players are at 0 HP")
}

// TestHasUndowedPlayers_ReturnsFalseForEmptyPlayerSet verifies that a combat
// with no player combatants returns false.
//
// Precondition: one NPC combatant only.
// Postcondition: HasUndowedPlayers returns false.
func TestHasUndowedPlayers_ReturnsFalseForEmptyPlayerSet(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "n1", Kind: combat.KindNPC, Name: "Thug", MaxHP: 10, CurrentHP: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	assert.False(t, cbt.HasUndowedPlayers(), "expected false when no player combatants")
}

// TestProperty_HasUndowedPlayers_NeverTrueWhenAllAtZeroHP is a property test verifying
// that HasUndowedPlayers always returns false when every player HP is 0.
//
// Precondition: 1–5 player combatants all with CurrentHP=0; 1 NPC.
// Postcondition: HasUndowedPlayers always returns false.
func TestProperty_HasUndowedPlayers_NeverTrueWhenAllAtZeroHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nPlayers := rapid.IntRange(1, 5).Draw(rt, "nPlayers")
		eng := combat.NewEngine()
		combatants := make([]*combat.Combatant, 0, nPlayers+1)
		for i := 0; i < nPlayers; i++ {
			combatants = append(combatants, &combat.Combatant{
				ID:        fmt.Sprintf("p%d", i),
				Kind:      combat.KindPlayer,
				Name:      fmt.Sprintf("P%d", i),
				MaxHP:     20,
				CurrentHP: 0,
			})
		}
		combatants = append(combatants, &combat.Combatant{
			ID: "n1", Kind: combat.KindNPC, Name: "NPC", MaxHP: 10, CurrentHP: 10,
		})
		cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
		if err != nil {
			rt.Skip()
		}
		if cbt.HasUndowedPlayers() {
			rt.Fatalf("HasUndowedPlayers returned true when all players are at 0 HP")
		}
	})
}
