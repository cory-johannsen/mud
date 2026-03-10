package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
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
	// NPCs: dead when HP <= 0
	npc := combat.Combatant{Kind: combat.KindNPC, Name: "G", MaxHP: 10, CurrentHP: 0}
	assert.True(t, npc.IsDead())
	npc.CurrentHP = 1
	assert.False(t, npc.IsDead())

	// Players: dead only when Dead flag is set (HP=0 means dying, not dead)
	player := combat.Combatant{Kind: combat.KindPlayer, Name: "X", MaxHP: 10, CurrentHP: 0}
	assert.False(t, player.IsDead(), "player at HP=0 is dying, not dead")
	player.Dead = true
	assert.True(t, player.IsDead(), "player with Dead=true is dead")
	player.CurrentHP = 1
	player.Dead = false
	assert.False(t, player.IsDead(), "player with Dead=false is alive")
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
		{30, 15, combat.CritSuccess}, // >= AC+10 (25)
		{25, 15, combat.CritSuccess}, // exactly AC+10
		{20, 15, combat.Success},     // >= AC
		{15, 15, combat.Success},     // exactly AC
		{10, 15, combat.Failure},     // >= AC-10 (5)
		{5, 15, combat.Failure},      // exactly AC-10
		{4, 15, combat.CritFailure},  // < AC-10
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
	// ProficiencyBonus is now a shim for CombatProficiencyBonus(level, "trained") = level+2.
	tests := []struct{ level, want int }{
		{1, 3}, {2, 4}, {3, 5}, {4, 6},
		{5, 7}, {10, 12}, {20, 22},
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

func TestCombatProficiencyBonus_UntrainedAlwaysZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		got := combat.CombatProficiencyBonus(level, "untrained")
		assert.Equal(rt, 0, got)
	})
}

func TestCombatProficiencyBonus_EmptyRankAlwaysZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		got := combat.CombatProficiencyBonus(level, "")
		assert.Equal(rt, 0, got)
	})
}

func TestCombatProficiencyBonus_TrainedIsLevelPlusTwo(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		got := combat.CombatProficiencyBonus(level, "trained")
		assert.Equal(rt, level+2, got)
	})
}

func TestCombatProficiencyBonus_ExpertGreaterThanTrained(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		expert := combat.CombatProficiencyBonus(level, "expert")
		trained := combat.CombatProficiencyBonus(level, "trained")
		assert.Greater(rt, expert, trained)
	})
}

func TestCombatProficiencyBonus_MasterGreaterThanExpert(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		master := combat.CombatProficiencyBonus(level, "master")
		expert := combat.CombatProficiencyBonus(level, "expert")
		assert.Greater(rt, master, expert)
	})
}

func TestCombatProficiencyBonus_LegendaryGreaterThanMaster(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		legendary := combat.CombatProficiencyBonus(level, "legendary")
		master := combat.CombatProficiencyBonus(level, "master")
		assert.Greater(rt, legendary, master)
	})
}

func TestCombatProficiencyBonus_TrainedPositiveForPositiveLevel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		got := combat.CombatProficiencyBonus(level, "trained")
		assert.Greater(rt, got, 0)
	})
}

func TestAbilityMod(t *testing.T) {
	tests := []struct{ score, want int }{
		{10, 0},
		{12, 1},
		{8, -1},
		{9, -1}, // floor division: (9-10)/2 floors to -1
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

func TestCombatant_SaveFields(t *testing.T) {
	c := &combat.Combatant{
		GritMod:       2,
		QuicknessMod:  1,
		SavvyMod:      3,
		ToughnessRank: "trained",
		HustleRank:    "expert",
		CoolRank:      "untrained",
	}
	assert.Equal(t, 2, c.GritMod)
	assert.Equal(t, 1, c.QuicknessMod)
	assert.Equal(t, 3, c.SavvyMod)
	assert.Equal(t, "trained", c.ToughnessRank)
	assert.Equal(t, "expert", c.HustleRank)
	assert.Equal(t, "untrained", c.CoolRank)
}

func TestProperty_DefaultSaveRank(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		validRanks := []string{"untrained", "trained", "expert", "master", "legendary"}
		rank := rapid.SampledFrom(validRanks).Draw(rt, "rank")
		got := combat.DefaultSaveRank(rank)
		assert.Equal(rt, rank, got, "non-empty rank should be returned as-is")
	})
}

func TestDefaultSaveRank_EmptyReturnsUntrained(t *testing.T) {
	assert.Equal(t, "untrained", combat.DefaultSaveRank(""))
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
	cbt, err := eng.StartCombat("room-alley", makeCombatants(), condition.NewRegistry(), nil, "")
	require.NoError(t, err)
	assert.Equal(t, "room-alley", cbt.RoomID)
	assert.Len(t, cbt.Combatants, 2)
	assert.False(t, cbt.Over)
}

func TestEngine_StartCombat_DuplicateRoom(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	require.NoError(t, err)
	_, err = eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	assert.Error(t, err)
}

func TestEngine_GetCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	require.NoError(t, err)

	cbt, ok := eng.GetCombat("room-1")
	assert.True(t, ok)
	assert.Equal(t, "room-1", cbt.RoomID)

	_, ok = eng.GetCombat("room-missing")
	assert.False(t, ok)
}

func TestEngine_EndCombat(t *testing.T) {
	eng := combat.NewEngine()
	_, err := eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	require.NoError(t, err)

	eng.EndCombat("room-1")
	_, ok := eng.GetCombat("room-1")
	assert.False(t, ok)
}

func TestCombat_CurrentTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	current := cbt.CurrentTurn()
	require.NotNil(t, current)
	assert.NotEmpty(t, current.ID)
}

func TestCombat_AdvanceTurn(t *testing.T) {
	eng := combat.NewEngine()
	cbt, _ := eng.StartCombat("room-1", makeCombatants(), condition.NewRegistry(), nil, "")
	first := cbt.CurrentTurn()
	cbt.AdvanceTurn()
	second := cbt.CurrentTurn()
	assert.NotEqual(t, first.ID, second.ID)
}

func TestCombat_AdvanceTurn_SkipsDead(t *testing.T) {
	eng := combat.NewEngine()
	c := makeCombatants()
	c[1].CurrentHP = 0 // NPC is dead
	cbt, _ := eng.StartCombat("room-1", c, condition.NewRegistry(), nil, "")
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

// TestPropertyCombatant_Position_ZeroValue verifies that the zero value of Combatant.Position is 0.
func TestPropertyCombatant_Position_ZeroValue(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{}
		if c.Position != 0 {
			rt.Fatal("Combatant.Position zero value must be 0")
		}
	})
}

// TestPropertyCombatant_Hidden_ZeroValue verifies that the zero value of Combatant.Hidden is false.
func TestPropertyCombatant_Hidden_ZeroValue(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{}
		if c.Hidden != false {
			rt.Fatal("Combatant.Hidden zero value must be false")
		}
	})
}

// TestPropertyCombatant_RevealedUntilRound_ZeroValue verifies that the zero value of Combatant.RevealedUntilRound is 0.
func TestPropertyCombatant_RevealedUntilRound_ZeroValue(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{}
		require.Equal(rt, 0, c.RevealedUntilRound)
	})
}

// TestPropertyCombatant_Hidden_Assignment verifies that Combatant.Hidden stores the assigned value.
func TestPropertyCombatant_Hidden_Assignment(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		c := &combat.Combatant{}
		val := rapid.Bool().Draw(rt, "hidden")
		c.Hidden = val
		if c.Hidden != val {
			rt.Fatalf("expected Hidden=%v, got %v", val, c.Hidden)
		}
	})
}
