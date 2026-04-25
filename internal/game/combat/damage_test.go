package combat_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestResolveDamage_BaseOnly(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 8, Source: "attack"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 8, r.Final)
	require.Len(t, r.Breakdown, 1)
	assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage)
}

func TestResolveDamage_NegativeAdditivesClamped(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{
			{Label: "dice", Value: 5, Source: "attack"},
			{Label: "penalty", Value: -10, Source: "condition:weakened"},
		},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 0, r.Final)
}

func TestResolveDamage_CritMultiplier(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 7, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 14, r.Final)
}

func TestResolveDamage_TwoMultipliers_NotChained(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{
			{Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
			{Label: "vulnerable", Factor: 2.0, Source: "condition:vulnerable"},
		},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 30, r.Final) // 1 + (2-1) + (2-1) = ×3
}

func TestResolveDamage_ThreeMultipliers(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{
			{Label: "crit", Factor: 2.0, Source: "engine:crit"},
			{Label: "vuln", Factor: 2.0, Source: "cond:vuln"},
			{Label: "extra", Factor: 2.0, Source: "tech:amp"},
		},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 40, r.Final) // ×4
}

func TestResolveDamage_Halver(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Halvers:   []combat.DamageHalver{{Label: "basic save success", Source: "tech:fireball"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 5, r.Final)
}

func TestResolveDamage_HalverIdempotent(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Halvers: []combat.DamageHalver{
			{Label: "save", Source: "tech:a"},
			{Label: "evasion", Source: "feat:b"},
		},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 5, r.Final)
}

func TestResolveDamage_CritPlusHalver_NetOne(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
		Halvers:     []combat.DamageHalver{{Label: "save", Source: "tech:fireball"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 10, r.Final)
}

func TestResolveDamage_Weakness(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
		DamageType: "fire",
		Weakness:   3,
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 15, r.Final)
}

func TestResolveDamage_Resistance(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
		DamageType: "physical",
		Resistance: 5,
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 7, r.Final)
}

func TestResolveDamage_ResistanceGreaterThanDamage_FloorsToZero(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 3, Source: "attack"}},
		DamageType: "physical",
		Resistance: 10,
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 0, r.Final)
}

func TestResolveDamage_FinalAlwaysNonNegative(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{
			{Label: "dice", Value: 1, Source: "attack"},
			{Label: "penalty", Value: -100, Source: "debuff"},
		},
	}
	r := combat.ResolveDamage(in)
	assert.GreaterOrEqual(t, r.Final, 0)
}

func TestResolveDamage_Pure(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 7, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
		Weakness:    2,
		Resistance:  1,
	}
	r1 := combat.ResolveDamage(in)
	r2 := combat.ResolveDamage(in)
	assert.Equal(t, r1.Final, r2.Final)
}

func TestResolveDamage_BreakdownContainsBase(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 5, Source: "attack"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage)
}

func TestResolveDamage_BreakdownOrder_MultiplierBeforeHalver(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 10, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
		Halvers:     []combat.DamageHalver{{Label: "save", Source: "tech:a"}},
	}
	r := combat.ResolveDamage(in)
	multIdx, halvIdx := -1, -1
	for i, s := range r.Breakdown {
		if s.Stage == combat.StageMultiplier {
			multIdx = i
		}
		if s.Stage == combat.StageHalver {
			halvIdx = i
		}
	}
	require.Greater(t, multIdx, -1)
	require.Greater(t, halvIdx, -1)
	assert.Less(t, multIdx, halvIdx)
}

func TestProperty_ResolveDamage_MultiplierCombination(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.IntRange(1, 100).Draw(rt, "base")
		n := rapid.IntRange(1, 4).Draw(rt, "n")
		var mults []combat.DamageMultiplier
		sumExtra := 0.0
		for i := 0; i < n; i++ {
			factor := rapid.Float64Range(1.01, 4.0).Draw(rt, fmt.Sprintf("factor%d", i))
			mults = append(mults, combat.DamageMultiplier{Label: "m", Factor: factor, Source: "test"})
			sumExtra += factor - 1.0
		}
		in := combat.DamageInput{
			Additives:   []combat.DamageAdditive{{Label: "dice", Value: base, Source: "test"}},
			Multipliers: mults,
		}
		r := combat.ResolveDamage(in)
		effective := 1.0 + sumExtra
		expected := int(math.Floor(float64(base) * effective))
		assert.Equal(rt, expected, r.Final)
	})
}

func TestProperty_ResolveDamage_FinalNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.Int().Draw(rt, "base")
		resist := rapid.IntRange(0, 200).Draw(rt, "resist")
		in := combat.DamageInput{
			Additives:  []combat.DamageAdditive{{Label: "dice", Value: base, Source: "test"}},
			Resistance: resist,
		}
		r := combat.ResolveDamage(in)
		assert.GreaterOrEqual(rt, r.Final, 0)
	})
}
