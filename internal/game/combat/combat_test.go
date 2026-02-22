package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestCombatant_IsPlayer(t *testing.T) {
	p := combat.Combatant{Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20}
	n := combat.Combatant{Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18}
	assert.True(t, p.IsPlayer())
	assert.False(t, n.IsPlayer())
}

func TestCombatant_IsDead(t *testing.T) {
	c := combat.Combatant{Kind: combat.KindPlayer, Name: "X", MaxHP: 10, CurrentHP: 0}
	assert.True(t, c.IsDead())
	c.CurrentHP = 1
	assert.False(t, c.IsDead())
}

func TestCombatant_ApplyDamage(t *testing.T) {
	c := combat.Combatant{Kind: combat.KindNPC, Name: "G", MaxHP: 18, CurrentHP: 18}
	c.ApplyDamage(5)
	assert.Equal(t, 13, c.CurrentHP)
	c.ApplyDamage(20)
	assert.Equal(t, 0, c.CurrentHP) // floors at 0
}

func TestCombatant_Property_DamageNeverBelowZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(1, 200).Draw(rt, "max_hp")
		dmg := rapid.IntRange(0, 500).Draw(rt, "dmg")
		c := combat.Combatant{Kind: combat.KindNPC, Name: "X", MaxHP: maxHP, CurrentHP: maxHP}
		c.ApplyDamage(dmg)
		assert.GreaterOrEqual(rt, c.CurrentHP, 0)
	})
}

func TestOutcomeFor(t *testing.T) {
	tests := []struct {
		roll int
		ac   int
		want combat.Outcome
	}{
		{30, 15, combat.CritSuccess},  // >= AC+10 (25)
		{25, 15, combat.CritSuccess},  // exactly AC+10
		{20, 15, combat.Success},      // >= AC
		{15, 15, combat.Success},      // exactly AC
		{10, 15, combat.Failure},      // >= AC-10 (5)
		{5, 15, combat.Failure},       // exactly AC-10
		{4, 15, combat.CritFailure},   // < AC-10
		{1, 15, combat.CritFailure},
	}
	for _, tc := range tests {
		got := combat.OutcomeFor(tc.roll, tc.ac)
		assert.Equal(t, tc.want, got, "roll=%d ac=%d", tc.roll, tc.ac)
	}
}

func TestOutcomeFor_Property_AllRollsMapToAnOutcome(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 40).Draw(rt, "roll")
		ac := rapid.IntRange(10, 30).Draw(rt, "ac")
		out := combat.OutcomeFor(roll, ac)
		assert.Contains(rt, []combat.Outcome{
			combat.CritSuccess, combat.Success, combat.Failure, combat.CritFailure,
		}, out)
	})
}

func TestProficiencyBonus(t *testing.T) {
	tests := []struct{ level, want int }{
		{1, 2}, {2, 2}, {3, 2}, {4, 2},
		{5, 3}, {6, 3}, {7, 3}, {8, 3},
		{9, 4}, {17, 6}, {20, 6},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, combat.ProficiencyBonus(tc.level), "level=%d", tc.level)
	}
}

func TestProficiencyBonus_Property_AlwaysAtLeastTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		assert.GreaterOrEqual(rt, combat.ProficiencyBonus(level), 2)
	})
}
