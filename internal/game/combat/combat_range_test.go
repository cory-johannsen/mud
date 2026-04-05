package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// TestCombatRange_MeleeAdjacent verifies that adjacent combatants (1 square apart) are 5 ft.
func TestCombatRange_MeleeAdjacent(t *testing.T) {
	a := combat.Combatant{GridX: 0, GridY: 0}
	b := combat.Combatant{GridX: 1, GridY: 0}
	assert.Equal(t, 5, combat.CombatRange(a, b))
}

// TestCombatRange_Diagonal verifies Chebyshev: diagonal 1 square = 5 ft.
func TestCombatRange_Diagonal(t *testing.T) {
	a := combat.Combatant{GridX: 0, GridY: 0}
	b := combat.Combatant{GridX: 1, GridY: 1}
	assert.Equal(t, 5, combat.CombatRange(a, b))
}

// TestCombatRange_Symmetric verifies CombatRange(a,b) == CombatRange(b,a).
func TestProperty_CombatRange_Symmetric(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ax := rapid.IntRange(0, 9).Draw(rt, "ax")
		ay := rapid.IntRange(0, 9).Draw(rt, "ay")
		bx := rapid.IntRange(0, 9).Draw(rt, "bx")
		by := rapid.IntRange(0, 9).Draw(rt, "by")
		a := combat.Combatant{GridX: ax, GridY: ay}
		b := combat.Combatant{GridX: bx, GridY: by}
		if combat.CombatRange(a, b) != combat.CombatRange(b, a) {
			rt.Fatalf("CombatRange not symmetric: (%d,%d)->(%d,%d)", ax, ay, bx, by)
		}
	})
}

// TestProperty_CombatRange_NonNegative verifies CombatRange always >= 0.
func TestProperty_CombatRange_NonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ax := rapid.IntRange(0, 9).Draw(rt, "ax")
		ay := rapid.IntRange(0, 9).Draw(rt, "ay")
		bx := rapid.IntRange(0, 9).Draw(rt, "bx")
		by := rapid.IntRange(0, 9).Draw(rt, "by")
		a := combat.Combatant{GridX: ax, GridY: ay}
		b := combat.Combatant{GridX: bx, GridY: by}
		if combat.CombatRange(a, b) < 0 {
			rt.Fatal("CombatRange returned negative value")
		}
	})
}

// TestIsFlanked_TwoOpponentsOppositeQuadrants verifies flanked when two enemies span opposite sides.
func TestIsFlanked_TwoOpponentsOppositeQuadrants(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 3, GridY: 3}, // northwest of target
		{GridX: 7, GridY: 7}, // southeast of target
	}
	assert.True(t, combat.IsFlanked(target, attackers))
}

// TestIsFlanked_SingleAttacker verifies not flanked with only one attacker.
func TestIsFlanked_SingleAttacker(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 3, GridY: 3},
	}
	assert.False(t, combat.IsFlanked(target, attackers))
}

// TestIsFlanked_SameQuadrant verifies not flanked when both attackers are on same side.
func TestIsFlanked_SameQuadrant(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 3, GridY: 3},
		{GridX: 4, GridY: 4},
	}
	assert.False(t, combat.IsFlanked(target, attackers))
}

// TestIsFlanked_CardinalNS verifies that directly-north and directly-south attackers
// do NOT trigger flanking per REQ-2D-4b (row and column BOTH must differ by ≥1).
func TestIsFlanked_CardinalNS(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 5, GridY: 3}, // directly north (same column, different row)
		{GridX: 5, GridY: 7}, // directly south (same column, different row)
	}
	assert.False(t, combat.IsFlanked(target, attackers), "cardinal N/S does not meet diagonal quadrant requirement")
}

// TestIsFlanked_CardinalEW verifies that directly-east and directly-west attackers
// do NOT trigger flanking per REQ-2D-4b.
func TestIsFlanked_CardinalEW(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 3, GridY: 5}, // directly west (different column, same row)
		{GridX: 7, GridY: 5}, // directly east (different column, same row)
	}
	assert.False(t, combat.IsFlanked(target, attackers), "cardinal E/W does not meet diagonal quadrant requirement")
}

// TestIsFlanked_MixedCardinalDiagonal verifies that one cardinal + one diagonal attacker
// do NOT trigger flanking (neither forms an opposite-quadrant pair with a cardinal attacker).
func TestIsFlanked_MixedCardinalDiagonal(t *testing.T) {
	target := combat.Combatant{GridX: 5, GridY: 5}
	attackers := []combat.Combatant{
		{GridX: 5, GridY: 3}, // directly north (cardinal)
		{GridX: 7, GridY: 7}, // southeast diagonal
	}
	// NW attacker (5,3) has dx=0, so guard requires both dx and dy non-zero → skipped.
	assert.False(t, combat.IsFlanked(target, attackers), "cardinal + diagonal does not meet strict quadrant requirement")
}
