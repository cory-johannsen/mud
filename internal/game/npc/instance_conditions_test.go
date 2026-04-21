package npc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestNewInstance_ConditionsInitialized verifies that NewInstance always
// initialises the Conditions field to a non-nil ActiveSet.
//
// Precondition: A valid non-nil Template is provided.
// Postcondition: Conditions must be non-nil after construction.
func TestNewInstance_ConditionsInitialized(t *testing.T) {
	t.Parallel()
	tmpl := &npc.Template{ID: "goblin", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13}
	inst := npc.NewInstance("inst-1", tmpl, "room-1")

	require.NotNil(t, inst.Conditions, "Conditions must be non-nil after NewInstance")
}

// TestNewInstanceWithResolver_ConditionsInitialized verifies that the full
// constructor also initialises Conditions.
//
// Precondition: A valid non-nil Template is provided with all optional args nil.
// Postcondition: Conditions must be non-nil after construction.
func TestNewInstanceWithResolver_ConditionsInitialized(t *testing.T) {
	t.Parallel()
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 2, MaxHP: 30, AC: 15}
	inst := npc.NewInstanceWithResolver("inst-2", tmpl, "room-2", nil, nil, nil)

	require.NotNil(t, inst.Conditions, "Conditions must be non-nil after NewInstanceWithResolver")
}

// TestNPCInstance_ConditionApplyAndHas verifies that a condition applied to
// an NPC instance's Conditions ActiveSet is recorded correctly.
//
// Precondition: Conditions is non-nil; ConditionDef is valid.
// Postcondition: Conditions.Has(condID) is true after Apply.
func TestNPCInstance_ConditionApplyAndHas(t *testing.T) {
	t.Parallel()
	tmpl := &npc.Template{ID: "goblin", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13}
	inst := npc.NewInstance("inst-3", tmpl, "room-3")

	def := &condition.ConditionDef{
		ID:            "frightened",
		Name:          "Frightened",
		DurationType:  "rounds",
		MaxStacks:     4,
		AttackPenalty: 1,
		ACPenalty:     1,
	}

	err := inst.Conditions.Apply(inst.ID, def, 1, 3)
	require.NoError(t, err)
	assert.True(t, inst.Conditions.Has("frightened"), "frightened must be active after Apply")
	assert.Equal(t, 1, inst.Conditions.Stacks("frightened"))
}

// TestProperty_NPCInstance_ConditionsAlwaysNonNil verifies that Conditions is
// always non-nil regardless of the template values used at spawn time.
//
// Precondition: Template has valid non-zero Level and MaxHP.
// Postcondition: Conditions != nil for every spawned instance.
func TestProperty_NPCInstance_ConditionsAlwaysNonNil(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		hp := rapid.IntRange(1, 500).Draw(rt, "hp")
		ac := rapid.IntRange(8, 30).Draw(rt, "ac")

		tmpl := &npc.Template{
			ID:    "prop-npc",
			Name:  "PropNPC",
			Level: level,
			MaxHP: hp,
			AC:    ac,
		}
		inst := npc.NewInstance("prop-inst", tmpl, "room-prop")
		assert.NotNil(rt, inst.Conditions, "Conditions must never be nil after spawn")
	})
}
