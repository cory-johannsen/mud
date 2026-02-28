package ai_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

func TestBuildCombatWorldState_PopulatesCombatants(t *testing.T) {
	cbt := &combat.Combat{
		RoomID: "pioneer_square",
		Combatants: []*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Player", CurrentHP: 20, MaxHP: 20, AC: 12},
			{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", CurrentHP: 15, MaxHP: 18, AC: 14},
		},
	}
	inst := npc.NewInstance("n1", &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 18, AC: 14, Perception: 5}, "pioneer_square")
	inst.CurrentHP = 15
	ws := ai.BuildCombatWorldState(cbt, inst, "downtown")
	if ws.NPC.UID != "n1" {
		t.Fatalf("expected NPC UID n1, got %q", ws.NPC.UID)
	}
	if len(ws.Combatants) != 2 {
		t.Fatalf("expected 2 combatants, got %d", len(ws.Combatants))
	}
}

func TestBuildCombatWorldState_DeadCombatantsMarked(t *testing.T) {
	cbt := &combat.Combat{
		RoomID: "room1",
		Combatants: []*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "P", CurrentHP: 0, MaxHP: 20},
			{ID: "n1", Kind: combat.KindNPC, Name: "G", CurrentHP: 10, MaxHP: 18},
		},
	}
	inst := npc.NewInstance("n1", &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}, "room1")
	ws := ai.BuildCombatWorldState(cbt, inst, "z1")
	var playerState *ai.CombatantState
	for _, c := range ws.Combatants {
		if c.UID == "p1" {
			playerState = c
		}
	}
	if playerState == nil || !playerState.Dead {
		t.Fatal("expected dead player to be marked Dead")
	}
}
