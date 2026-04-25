package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestBuildDamageInput_CritAddsMultiplier(t *testing.T) {
	actor := &combat.Combatant{ID: "player", Level: 1, StrMod: 2, WeaponBonus: 0}
	target := &combat.Combatant{ID: "npc", Resistances: map[string]int{}, Weaknesses: map[string]int{}}
	r := combat.AttackResult{
		Outcome:    combat.CritSuccess,
		BaseDamage: 5,
		DamageType: "slashing",
	}
	opts := combat.BuildDamageOpts{
		Actor:             actor,
		Target:            target,
		AttackResult:      r,
		ConditionDmgBonus: 0,
		ExtraDiceRolled:   0,
	}
	di := combat.BuildDamageInput(opts)
	require.Len(t, di.Multipliers, 1)
	assert.Equal(t, 2.0, di.Multipliers[0].Factor)
}

func TestBuildDamageInput_MissProducesZeroBase(t *testing.T) {
	actor := &combat.Combatant{ID: "player", Level: 1, StrMod: 1}
	target := &combat.Combatant{ID: "npc", Resistances: map[string]int{}, Weaknesses: map[string]int{}}
	r := combat.AttackResult{Outcome: combat.Failure, BaseDamage: 0}
	opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
	di := combat.BuildDamageInput(opts)
	total := 0
	for _, a := range di.Additives {
		total += a.Value
	}
	assert.LessOrEqual(t, total, 0)
}

func TestBuildDamageInput_WeaknessFromTarget(t *testing.T) {
	actor := &combat.Combatant{ID: "player", Level: 1}
	target := &combat.Combatant{
		ID:          "npc",
		Weaknesses:  map[string]int{"fire": 5},
		Resistances: map[string]int{},
	}
	r := combat.AttackResult{Outcome: combat.Success, BaseDamage: 8, DamageType: "fire"}
	opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
	di := combat.BuildDamageInput(opts)
	assert.Equal(t, 5, di.Weakness)
	assert.Equal(t, 0, di.Resistance)
}

func TestBuildDamageInput_ResistanceFromTarget(t *testing.T) {
	actor := &combat.Combatant{ID: "player", Level: 1}
	target := &combat.Combatant{
		ID:          "npc",
		Weaknesses:  map[string]int{},
		Resistances: map[string]int{"physical": 3},
	}
	r := combat.AttackResult{Outcome: combat.Success, BaseDamage: 10, DamageType: "physical"}
	opts := combat.BuildDamageOpts{Actor: actor, Target: target, AttackResult: r}
	di := combat.BuildDamageInput(opts)
	assert.Equal(t, 3, di.Resistance)
	assert.Equal(t, 0, di.Weakness)
}
