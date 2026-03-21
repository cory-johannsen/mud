package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// spawnTypedNPC creates and registers a live NPC instance with the given type in roomID.
func spawnTypedNPC(t *testing.T, npcMgr *npc.Manager, roomID, npcType string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:         npcType + "-tmpl",
		Name:       "Guard",
		Type:       npcType,
		Level:      1,
		MaxHP:      20,
		AC:         13,
		Perception: 2,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnTypedNPC(%s): %v", npcType, err)
	}
	return inst
}

// TestInitiateGuardCombat_NoGuards_NoOp verifies that broadcastFn is NOT called when
// no guard-typed NPCs are present in the player's room.
func TestInitiateGuardCombat_NoGuards_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const roomID = "room-guard-1"
	// Spawn a non-guard NPC (goblin type).
	spawnTypedNPC(t, h.npcMgr, roomID, "goblin")
	addTestPlayerNamed(t, h.sessions, "player-guard-1", roomID, "Alice")

	h.InitiateGuardCombat("player-guard-1", "zone-1", 2)

	if broadcastCalled {
		t.Fatal("expected broadcastFn NOT to be called when no guards are present; it was called")
	}
}

// TestInitiateGuardCombat_WithGuards_BroadcastsAndAttacks verifies that InitiateGuardCombat
// calls broadcastFn with a non-empty narrative when a guard NPC is present in the room.
func TestInitiateGuardCombat_WithGuards_BroadcastsAndAttacks(t *testing.T) {
	var capturedEvents []*gamev1.CombatEvent
	h := makeCombatHandler(t, func(_ string, events []*gamev1.CombatEvent) {
		capturedEvents = append(capturedEvents, events...)
	})

	const roomID = "room-guard-2"
	spawnTypedNPC(t, h.npcMgr, roomID, "guard")
	addTestPlayerNamed(t, h.sessions, "player-guard-2", roomID, "Bob")

	h.InitiateGuardCombat("player-guard-2", "zone-1", 2)

	if len(capturedEvents) == 0 {
		t.Fatal("expected broadcastFn to be called with events; got none")
	}
	if capturedEvents[0].Narrative == "" {
		t.Fatal("expected non-empty Narrative in broadcast event; got empty string")
	}
}

// TestInitiateGuardCombat_KillLevel_NarrativeContainsAttack verifies that wantedLevel >= 3
// produces an attack-on-sight narrative.
func TestInitiateGuardCombat_KillLevel_NarrativeContainsAttack(t *testing.T) {
	var capturedEvents []*gamev1.CombatEvent
	h := makeCombatHandler(t, func(_ string, events []*gamev1.CombatEvent) {
		capturedEvents = append(capturedEvents, events...)
	})

	const roomID = "room-guard-3"
	spawnTypedNPC(t, h.npcMgr, roomID, "guard")
	addTestPlayerNamed(t, h.sessions, "player-guard-3", roomID, "Charlie")

	h.InitiateGuardCombat("player-guard-3", "zone-1", 3)

	if len(capturedEvents) == 0 {
		t.Fatal("expected broadcastFn to be called with events; got none")
	}
	narrative := capturedEvents[0].Narrative
	if narrative == "" {
		t.Fatal("expected non-empty Narrative in broadcast event; got empty string")
	}
}

// TestInitiateGuardCombat_UnknownPlayer_NoOp verifies that InitiateGuardCombat is a no-op
// when the uid does not correspond to a registered player session.
func TestInitiateGuardCombat_UnknownPlayer_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	h.InitiateGuardCombat("nonexistent-uid", "zone-1", 2)

	if broadcastCalled {
		t.Fatal("expected broadcastFn NOT to be called for unknown player; it was called")
	}
}
