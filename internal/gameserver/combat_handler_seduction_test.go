package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// newSeductionTestCombatHandler builds a CombatHandler wired with a condRegistry
// that has the "seduced" condition registered, and an npcMgr with inst already spawned.
//
// Precondition: inst must be non-nil.
// Postcondition: Returns a CombatHandler with condRegistry.Get("seduced") returning a valid def.
func newSeductionTestCombatHandler(t *testing.T, npcMgr *npc.Manager) *CombatHandler {
	t.Helper()
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           "seduced",
		Name:         "Seduced",
		DurationType: "rounds",
		MaxStacks:    1,
	})
	_, sessMgr := testWorldAndSession(t)
	return NewCombatHandler(
		nil, npcMgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		0, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

// TestNPCSeduction_GenderMismatch_NPCBecomesHostile verifies that when the NPC's
// SeductionGender does not match the player's gender, ResolveNPCSeductionGenderCheck
// returns true (blocked) and sets inst.Disposition to "hostile".
//
// Precondition: inst.SeductionGender="male", playerGender="female".
// Postcondition: result==true; inst.Disposition=="hostile".
func TestNPCSeduction_GenderMismatch_NPCBecomesHostile(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID:              "patron",
		Name:            "Patron",
		MaxHP:           20,
		Level:           2,
		SeductionGender: "male",
		Disposition:     "neutral",
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	if err != nil {
		t.Fatal(err)
	}
	inst.Disposition = "neutral"

	h := newSeductionTestCombatHandler(t, mgr)
	result := h.ResolveNPCSeductionGenderCheck(inst, "player1", "female")

	assert.True(t, result, "gender mismatch must block seduction")
	assert.Equal(t, "hostile", inst.Disposition, "NPC must turn hostile on gender mismatch")
}

// TestNPCSeduction_HighFlair_PlayerSeduced verifies that when the NPC has high Flair
// and wins the opposed check, the "seduced" condition is applied to condSet.
//
// Precondition: inst.Flair=20, playerSavvy=8, npcRoll=20, playerRoll=1.
// Postcondition: result==true; condSet.Has("seduced")==true.
func TestNPCSeduction_HighFlair_PlayerSeduced(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID:          "seducer",
		Name:        "Seducer",
		MaxHP:       20,
		Level:       2,
		Disposition: "neutral",
		Abilities: npc.Abilities{
			Flair: 20,
		},
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	if err != nil {
		t.Fatal(err)
	}

	condReg := condition.NewRegistry()
	seducedDef := &condition.ConditionDef{
		ID:           "seduced",
		Name:         "Seduced",
		DurationType: "rounds",
		MaxStacks:    1,
	}
	condReg.Register(seducedDef)

	condSet := condition.NewActiveSet()
	_, sessMgr := testWorldAndSession(t)
	h := NewCombatHandler(
		nil, mgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		0, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	result := h.ResolveNPCSeductionContest(inst, "player1", 8, seducedDef, condSet, 20, 1)

	assert.True(t, result, "high NPC Flair must win the seduction contest")
	assert.True(t, condSet.Has("seduced"), "player must have seduced condition applied")
}

// TestNPCSeduction_LowFlair_NPCBecomesHostile verifies that when the player's Savvy
// beats the NPC's Flair in the opposed check, the NPC turns hostile and marks the
// player in SeductionRejected.
//
// Precondition: inst.Flair=8, playerSavvy=18, npcRoll=1, playerRoll=20.
// Postcondition: result==false; inst.Disposition=="hostile"; inst.SeductionRejected["player1"]==true.
func TestNPCSeduction_LowFlair_NPCBecomesHostile(t *testing.T) {
	mgr := npc.NewManager()
	tmpl := &npc.Template{
		ID:          "weakseducer",
		Name:        "Weak Seducer",
		MaxHP:       20,
		Level:       2,
		Disposition: "neutral",
		Abilities: npc.Abilities{
			Flair: 8,
		},
	}
	inst, err := mgr.Spawn(tmpl, "room1")
	if err != nil {
		t.Fatal(err)
	}
	inst.Disposition = "neutral"

	seducedDef := &condition.ConditionDef{
		ID:           "seduced",
		Name:         "Seduced",
		DurationType: "rounds",
		MaxStacks:    1,
	}
	condSet := condition.NewActiveSet()
	_, sessMgr := testWorldAndSession(t)
	h := NewCombatHandler(
		nil, mgr, sessMgr, nil,
		func(_ string, _ []*gamev1.CombatEvent) {},
		0, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	result := h.ResolveNPCSeductionContest(inst, "player1", 18, seducedDef, condSet, 1, 20)

	assert.False(t, result, "player with higher Savvy must resist seduction")
	assert.Equal(t, "hostile", inst.Disposition, "NPC must turn hostile on failure")
	assert.True(t, inst.SeductionRejected["player1"], "NPC must mark player as rejected")
}

// TestProperty_NPCSeduction_HighFlairAlwaysSeduces verifies that an NPC with
// very high Flair (score=30, mod=10) and a player with low Savvy will always win
// the seduction contest when npcRoll=20 and playerRoll=1.
//
// Precondition: inst.Flair=30; playerSavvy∈[1,5]; npcRoll=20; playerRoll=1.
// Postcondition: ResolveNPCSeductionContest always returns true; condSet.Has("seduced")==true.
func TestProperty_NPCSeduction_HighFlairAlwaysSeduces(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		playerSavvy := rapid.IntRange(1, 5).Draw(rt, "playerSavvy")

		mgr := npc.NewManager()
		tmpl := &npc.Template{
			ID:          "hiflair",
			Name:        "High Flair NPC",
			MaxHP:       20,
			Level:       2,
			Disposition: "neutral",
			Abilities: npc.Abilities{
				Flair: 30,
			},
		}
		inst, err := mgr.Spawn(tmpl, "room1")
		if err != nil {
			rt.Fatal(err)
		}

		seducedDef := &condition.ConditionDef{
			ID:           "seduced",
			Name:         "Seduced",
			DurationType: "rounds",
			MaxStacks:    1,
		}
		condSet := condition.NewActiveSet()
		h := NewCombatHandler(
			nil, mgr, session.NewManager(), nil,
			func(_ string, _ []*gamev1.CombatEvent) {},
			0, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		)

		result := h.ResolveNPCSeductionContest(inst, "player1", playerSavvy, seducedDef, condSet, 20, 1)

		if !result {
			rt.Fatalf("NPC with Flair=30 must always seduce player with Savvy=%d (npcRoll=20, playerRoll=1)", playerSavvy)
		}
		if !condSet.Has("seduced") {
			rt.Fatal("seduced condition must be applied to condSet when result==true")
		}
	})
}
