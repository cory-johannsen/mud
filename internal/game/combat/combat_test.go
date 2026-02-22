package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAbilityMod(t *testing.T) {
	tests := []struct{ score, want int }{
		{10, 0},
		{12, 1},
		{8, -1},
		{9, -1},  // floor division: (9-10)/2 floors to -1
		{20, 5},
		{1, -5},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, combat.AbilityMod(tc.score), "score=%d", tc.score)
	}
}

func TestAbilityMod_Property_EvenScoresSymmetric(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 10).Draw(rt, "n")
		// AbilityMod(10+2n) == n and AbilityMod(10-2n) == -n
		assert.Equal(rt, n, combat.AbilityMod(10+2*n))
		assert.Equal(rt, -n, combat.AbilityMod(10-2*n))
	})
}

// --- Engine ---

func makeCombatants() []*combat.Combatant {
	return []*combat.Combatant{
		{ID: "player-1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "npc-ganger-1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 13, Level: 1, StrMod: 2, DexMod: 1},
	}
}

func TestEngine_StartCombat(t *testing.T) {
	eng := combat.NewEngine()
	cbt, err := eng.StartCombat("room-alley", makeCombatants())
	require.NoError(t, err)
	assert.Equal(t, "room-alley", cbt.RoomID)
	assert.Len(t, cbt.Combatants, 2)
	assert.False(t, cbt.Over)
}

func TestEngine_StartCombat_DuplicateRoom(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)
	_, err = eng.StartCombat("room-1", makeCombatants())
	assert.Error(t, err)
}

func TestEngine_GetCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)

	cbt, ok := eng.GetCombat("room-1")
	assert.True(t, ok)
	assert.Equal(t, "room-1", cbt.RoomID)

	_, ok = eng.GetCombat("room-missing")
	assert.False(t, ok)
}

func TestEngine_EndCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants())
	require.NoError(t, err)

	eng.EndCombat("room-1")
	_, ok := eng.GetCombat("room-1")
	assert.False(t, ok)
}

func TestCombat_CurrentTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants())
	current := cbt.CurrentTurn()
	require.NotNil(t, current)
	assert.NotEmpty(t, current.ID)
}

func TestCombat_AdvanceTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants())
	first := cbt.CurrentTurn()
	cbt.AdvanceTurn()
	second := cbt.CurrentTurn()
	assert.NotEqual(t, first.ID, second.ID)
}

func TestCombat_AdvanceTurn_SkipsDead(t *testing.T) {
	eng := combat.NewEngine()
	c := makeCombatants()
	c[1].CurrentHP = 0 // NPC is dead
	cbt, _ := eng.StartCombat("room-1", c)
	for i := 0; i < 5; i++ {
		current := cbt.CurrentTurn()
		assert.Equal(t, combat.KindPlayer, current.Kind)
		cbt.AdvanceTurn()
	}
}

// --- AttackResult / ResolveAttack ---

func TestAttackResult_DamageByOutcome(t *testing.T) {
	tests := []struct {
		outcome combat.Outcome
		base    int
		want    int
	}{
		{combat.CritSuccess, 6, 12},
		{combat.Success, 6, 6},
		{combat.Failure, 6, 0},
		{combat.CritFailure, 6, 0},
	}
	for _, tc := range tests {
		ar := combat.AttackResult{Outcome: tc.outcome, BaseDamage: tc.base}
		assert.Equal(t, tc.want, ar.EffectiveDamage(), "outcome=%s base=%d", tc.outcome, tc.base)
	}
}

func TestAttackResult_Property_EffectiveDamageNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		outcome := combat.Outcome(rapid.IntRange(0, 3).Draw(rt, "outcome"))
		base := rapid.IntRange(0, 50).Draw(rt, "base")
		ar := combat.AttackResult{Outcome: outcome, BaseDamage: base}
		assert.GreaterOrEqual(rt, ar.EffectiveDamage(), 0)
	})
}

func TestResolveAttack_HitDealsPositiveDamage(t *testing.T) {
	attacker := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "A",
		MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 3, DexMod: 1}
	target := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "G",
		MaxHP: 18, CurrentHP: 18, AC: 10, Level: 1, StrMod: 2, DexMod: 1}

	// Roll d20=20 → total = 20 + 3 (str) + 2 (prof) = 25 vs AC 10 → crit success
	src := &fixedSource{val: 19} // Intn(20)→19 +1=20; Intn(6)→5 +1=6
	result := combat.ResolveAttack(attacker, target, src)
	assert.Equal(t, combat.CritSuccess, result.Outcome)
	assert.Greater(t, result.EffectiveDamage(), 0)
}

// fixedSource always returns val for any Intn call.
type fixedSource struct{ val int }

func (f *fixedSource) Intn(n int) int {
	if f.val >= n {
		return n - 1
	}
	return f.val
}

// --- RollInitiative ---

func TestRollInitiative_SetsInitiativeField(t *testing.T) {
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "A", DexMod: 2},
		{ID: "n1", Kind: combat.KindNPC, Name: "G", DexMod: 1},
	}
	src := &fixedSource{val: 9} // Intn(20)→9, +1=10
	combat.RollInitiative(combatants, src)

	// player: 10 + 2 = 12; npc: 10 + 1 = 11
	assert.Equal(t, 12, combatants[0].Initiative)
	assert.Equal(t, 11, combatants[1].Initiative)
}

func TestRollInitiative_Property_InitiativeAtLeastOnePlusMod(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dexMod := rapid.IntRange(-3, 5).Draw(rt, "dex_mod")
		c := &combat.Combatant{ID: "x", Kind: combat.KindNPC, Name: "X", DexMod: dexMod}
		src := &fixedSource{val: 0} // Intn(20)→0, +1=1
		combat.RollInitiative([]*combat.Combatant{c}, src)
		// minimum roll is 1, so initiative == 1 + dexMod
		assert.Equal(rt, 1+dexMod, c.Initiative)
	})
}
