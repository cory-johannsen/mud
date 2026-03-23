package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestInitiateNPCCombat_PlayerNotFound_NoOp verifies that calling with an unknown player UID
// does not panic and produces no combat.
func TestInitiateNPCCombat_PlayerNotFound_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	tmpl := &npc.Template{
		ID: "bandit0", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10, NPCType: "combat",
	}
	_ = tmpl.Validate()
	inst, err := h.npcMgr.Spawn(tmpl, "room-1")
	if err != nil {
		t.Fatal(err)
	}

	h.InitiateNPCCombat(inst, "unknown-player-uid")

	if broadcastCalled {
		t.Fatal("expected no broadcast when player not found")
	}
}

// TestInitiateNPCCombat_PlayerInDifferentRoom_NoOp verifies that NPCs cannot
// initiate combat with players in a different room.
func TestInitiateNPCCombat_PlayerInDifferentRoom_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const playerRoom = "room-player"
	const npcRoom = "room-npc"
	addTestPlayer(t, h.sessions, "player-1", playerRoom)

	tmpl := &npc.Template{
		ID: "bandit", Name: "Bandit", Level: 1, MaxHP: 10, AC: 10, NPCType: "combat",
	}
	_ = tmpl.Validate()
	inst, err := h.npcMgr.Spawn(tmpl, npcRoom)
	if err != nil {
		t.Fatal(err)
	}

	h.InitiateNPCCombat(inst, "player-1")

	if broadcastCalled {
		t.Fatal("expected no broadcast when NPC and player are in different rooms")
	}
}

// TestInitiateNPCCombat_StartsCombat verifies that when NPC and player are in the same room,
// combat is started and events are broadcast.
func TestInitiateNPCCombat_StartsCombat(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, events []*gamev1.CombatEvent) {
		if len(events) > 0 {
			broadcastCalled = true
		}
	})

	const roomID = "room-combat"
	addTestPlayer(t, h.sessions, "player-2", roomID)

	tmpl := &npc.Template{
		ID: "bandit2", Name: "Bandit", Level: 1, MaxHP: 10, AC: 10, NPCType: "combat",
	}
	_ = tmpl.Validate()
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatal(err)
	}

	h.InitiateNPCCombat(inst, "player-2")

	if !broadcastCalled {
		t.Fatal("expected combat initiation to broadcast events")
	}
	// Verify combat is now active.
	if _, ok := h.engine.GetCombat(roomID); !ok {
		t.Error("expected combat to be active after InitiateNPCCombat")
	}
}
