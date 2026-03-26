package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestInitiationReason_Constants(t *testing.T) {
	assert.Equal(t, combat.InitiationReason("on_sight"), combat.ReasonOnSight)
	assert.Equal(t, combat.InitiationReason("territory"), combat.ReasonTerritory)
	assert.Equal(t, combat.InitiationReason("provoked"), combat.ReasonProvoked)
	assert.Equal(t, combat.InitiationReason("call_for_help"), combat.ReasonCallForHelp)
	assert.Equal(t, combat.InitiationReason("wanted"), combat.ReasonWanted)
	assert.Equal(t, combat.InitiationReason("protecting"), combat.ReasonProtecting)
}

func TestInitiationReason_Message_PlayerInitiated(t *testing.T) {
	msg := combat.FormatPlayerInitiationMsg("Scavenger Boss")
	assert.Equal(t, "You attack Scavenger Boss.", msg)
}

func TestInitiationReason_Message_NPCInitiated_OnSight(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonOnSight, "")
	assert.Equal(t, "Scavenger attacks you — attacked on sight.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Territory(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonTerritory, "")
	assert.Equal(t, "Guard Dog attacks you — defending its territory.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Provoked(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger", combat.ReasonProvoked, "")
	assert.Equal(t, "Scavenger attacks you — provoked by your attack.", msg)
}

func TestInitiationReason_Message_NPCInitiated_CallForHelp(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Scavenger Grunt", combat.ReasonCallForHelp, "")
	assert.Equal(t, "Scavenger Grunt attacks you — responding to a call for help.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Wanted(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Zone Guard", combat.ReasonWanted, "")
	assert.Equal(t, "Zone Guard attacks you — alerted by your wanted status.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Protecting(t *testing.T) {
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "Boss Scavenger")
	assert.Equal(t, "Guard Dog attacks you — protecting Boss Scavenger.", msg)
}

func TestInitiationReason_Message_NPCInitiated_Protecting_NoName(t *testing.T) {
	// When ProtectedNPCName is empty but reason is Protecting, falls back to CallForHelp phrasing.
	msg := combat.FormatNPCInitiationMsg("Guard Dog", combat.ReasonProtecting, "")
	assert.Equal(t, "Guard Dog attacks you — responding to a call for help.", msg)
}

func TestProperty_FormatNPCInitiationMsg_NeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reason := rapid.SampledFrom([]combat.InitiationReason{
			combat.ReasonOnSight, combat.ReasonTerritory, combat.ReasonProvoked,
			combat.ReasonCallForHelp, combat.ReasonWanted, combat.ReasonProtecting,
		}).Draw(rt, "reason")
		npcName := rapid.StringN(1, 20, -1).Draw(rt, "npcName")
		msg := combat.FormatNPCInitiationMsg(npcName, reason, "")
		assert.NotEmpty(rt, msg)
	})
}
