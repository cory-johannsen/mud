package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
)

// TestInstance_SenseAbilities_CopiedFromTemplate verifies SenseAbilities is copied at spawn.
//
// Precondition: Template has SenseAbilities=["Rage","Poison Spit"].
// Postcondition: Instance.SenseAbilities == ["Rage","Poison Spit"].
func TestInstance_SenseAbilities_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID: "t1", Name: "Brute", MaxHP: 20, AC: 12,
		SenseAbilities: []string{"Rage", "Poison Spit"},
	}
	inst := npc.NewInstanceWithResolver("inst1", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, []string{"Rage", "Poison Spit"}, inst.SenseAbilities)
}

// TestInstance_Disposition_DefaultHostile verifies empty template Disposition defaults to "hostile".
//
// Precondition: Template.Disposition == "".
// Postcondition: Instance.Disposition == "hostile".
func TestInstance_Disposition_DefaultHostile(t *testing.T) {
	tmpl := &npc.Template{ID: "t2", Name: "Guard", MaxHP: 10, AC: 10}
	inst := npc.NewInstanceWithResolver("inst2", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, "hostile", inst.Disposition)
}

// TestInstance_Disposition_CopiedFromTemplate verifies explicit disposition is preserved.
//
// Precondition: Template.Disposition == "neutral".
// Postcondition: Instance.Disposition == "neutral".
func TestInstance_Disposition_CopiedFromTemplate(t *testing.T) {
	tmpl := &npc.Template{ID: "t3", Name: "Merchant", MaxHP: 10, AC: 10, Disposition: "neutral"}
	inst := npc.NewInstanceWithResolver("inst3", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, "neutral", inst.Disposition)
}

// TestInstance_MotiveBonus_ZeroAtSpawn verifies MotiveBonus starts at 0.
//
// Precondition: fresh instance.
// Postcondition: MotiveBonus == 0.
func TestInstance_MotiveBonus_ZeroAtSpawn(t *testing.T) {
	tmpl := &npc.Template{ID: "t4", Name: "Ganger", MaxHP: 10, AC: 10}
	inst := npc.NewInstanceWithResolver("inst4", tmpl, "room1", func(_ string) int { return 0 })
	assert.Equal(t, 0, inst.MotiveBonus)
}
