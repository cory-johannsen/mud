package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// Scenario: crit alone = ×2.
func TestScenario_CritAlone(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
	}
	assert.Equal(t, 10, combat.ResolveDamage(in).Final)
}

// Scenario: crit + vulnerable (both ×2) = ×3. PF2E core.
func TestScenario_CritPlusVulnerable(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{
			{Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
			{Label: "vulnerable fire", Factor: 2.0, Source: "condition:vulnerable"},
		},
	}
	assert.Equal(t, 15, combat.ResolveDamage(in).Final) // 5 × 3
}

// Scenario: crit + vulnerable + hypothetical third ×2 = ×4.
func TestScenario_ThreeDoubling(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "1d6", Value: 4, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{
			{Label: "crit", Factor: 2.0, Source: "engine:crit"},
			{Label: "vuln", Factor: 2.0, Source: "cond:vuln"},
			{Label: "amp", Factor: 2.0, Source: "tech:amp"},
		},
	}
	assert.Equal(t, 16, combat.ResolveDamage(in).Final) // 4 × 4
}

// Scenario: crit + basic save success halver = net ×1.
func TestScenario_CritPlusHalver(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 8, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
		Halvers:     []combat.DamageHalver{{Label: "basic save success", Source: "tech:fireball"}},
	}
	assert.Equal(t, 8, combat.ResolveDamage(in).Final) // 8×2=16, 16÷2=8
}

// Scenario: weakness 5 + resistance 10 + base 12 = 7. Per spec §7.
func TestScenario_WeaknessAndResistance(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 12, Source: "attack"}},
		DamageType: "fire",
		Weakness:   5,
		Resistance: 10,
	}
	// 12 + 5 (weakness) - 10 (resistance) = 7
	assert.Equal(t, 7, combat.ResolveDamage(in).Final)
}

// Scenario: resistance greater than total — floor at 0.
func TestScenario_ResistanceGreaterThanDamage(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 3, Source: "attack"}},
		DamageType: "fire",
		Resistance: 10,
	}
	assert.Equal(t, 0, combat.ResolveDamage(in).Final)
}

// Scenario: empty multipliers / halvers — base passthrough.
func TestScenario_EmptyModifiers(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 6, Source: "attack"}},
	}
	r := combat.ResolveDamage(in)
	assert.Equal(t, 6, r.Final)
	assert.Len(t, r.Breakdown, 1) // only StageBase
	assert.Equal(t, combat.StageBase, r.Breakdown[0].Stage)
}

// Scenario: breakdown length for non-trivial inputs — more than 1 stage.
func TestScenario_NonTrivialBreakdownLength(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "dice", Value: 5, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "crit", Factor: 2.0, Source: "engine:crit"}},
		Weakness:    3,
	}
	r := combat.ResolveDamage(in)
	assert.Greater(t, len(r.Breakdown), 1) // MULT-13: non-trivial stages present
}
