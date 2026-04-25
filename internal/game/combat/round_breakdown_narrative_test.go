package combat_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestFormatBreakdownInline_TrivialOnlyBase_NoLine(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{{Label: "dice", Value: 6, Source: "attack"}},
	}
	r := combat.ResolveDamage(in)
	line := combat.FormatBreakdownInline(r.Breakdown)
	assert.Empty(t, line)
}

func TestFormatBreakdownInline_CritIncluded(t *testing.T) {
	in := combat.DamageInput{
		Additives:   []combat.DamageAdditive{{Label: "1d8", Value: 5, Source: "attack"}},
		Multipliers: []combat.DamageMultiplier{{Label: "critical hit", Factor: 2.0, Source: "engine:crit"}},
	}
	r := combat.ResolveDamage(in)
	line := combat.FormatBreakdownInline(r.Breakdown)
	assert.NotEmpty(t, line)
	assert.Contains(t, line, "×2")
}

func TestFormatBreakdownInline_WeaknessAndResistance(t *testing.T) {
	in := combat.DamageInput{
		Additives:  []combat.DamageAdditive{{Label: "dice", Value: 14, Source: "attack"}},
		DamageType: "fire",
		Weakness:   3,
		Resistance: 10,
	}
	r := combat.ResolveDamage(in)
	line := combat.FormatBreakdownInline(r.Breakdown)
	assert.Contains(t, line, "weakness")
	assert.Contains(t, line, "resistance")
}

func TestFormatBreakdownInline_WidthWrapping(t *testing.T) {
	in := combat.DamageInput{
		Additives: []combat.DamageAdditive{
			{Label: "1d8", Value: 5, Source: "attack"},
			{Label: "STR mod", Value: 3, Source: "stat:str"},
			{Label: "passive feat add", Value: 2, Source: "feat:brutal"},
		},
		Multipliers: []combat.DamageMultiplier{
			{Label: "critical hit", Factor: 2.0, Source: "engine:crit"},
			{Label: "vulnerable fire", Factor: 2.0, Source: "cond:vuln"},
		},
		Halvers:    []combat.DamageHalver{{Label: "basic save", Source: "tech:fireball"}},
		DamageType: "fire",
		Weakness:   5,
		Resistance: 3,
	}
	r := combat.ResolveDamage(in)
	line := combat.FormatBreakdownInline(r.Breakdown)
	for _, l := range strings.Split(line, "\n") {
		assert.LessOrEqual(t, len(l), 80, "line too wide: %q", l)
	}
}
