package gameserver

import (
	"sync"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// addTestPlayerNamed registers a player session with a custom CharName and returns it.
func addTestPlayerNamed(t *testing.T, sessMgr *session.Manager, uid, roomID, charName string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    charName,
		CharName:    charName,
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("addTestPlayerNamed: %v", err)
	}
	return sess
}

// TestCombatHandler_Aid_EmptyAllyName verifies that Aid returns an error when allyName is empty.
func TestCombatHandler_Aid_EmptyAllyName(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-aid-1"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayerNamed(t, h.sessions, "player-aid-1", roomID, "Alice")

	_, err := h.Attack("player-aid-1", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	_, err = h.Aid("player-aid-1", "")
	if err == nil {
		t.Fatal("expected error for empty allyName; got nil")
	}
}

// TestCombatHandler_Aid_SelfTargeting verifies that Aid returns an error when allyName matches
// the actor's own CharName (case-insensitive).
func TestCombatHandler_Aid_SelfTargeting(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-aid-2"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayerNamed(t, h.sessions, "player-aid-2", roomID, "Alice")

	_, err := h.Attack("player-aid-2", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	_, err = h.Aid("player-aid-2", "Alice")
	if err == nil {
		t.Fatal("expected error for self-targeting; got nil")
	}
}

// TestCombatHandler_Aid_AllyNotInCombat verifies that Aid returns an error when the named ally
// is not a combatant in the actor's active combat.
func TestCombatHandler_Aid_AllyNotInCombat(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(_ string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-aid-3"
	const otherRoomID = "room-aid-3b"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayerNamed(t, h.sessions, "player-aid-3a", roomID, "Alice")
	// Bob is in a different room and not part of Alice's combat.
	addTestPlayerNamed(t, h.sessions, "player-aid-3b", otherRoomID, "Bob")

	_, err := h.Attack("player-aid-3a", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	_, err = h.Aid("player-aid-3a", "Bob")
	if err == nil {
		t.Fatal("expected error when ally is not in combat; got nil")
	}
}

// TestCombatHandler_Aid_InsufficientAP verifies that Aid returns an error when the actor
// has no remaining AP (e.g., after Attack + Strike spending all 3 AP).
func TestCombatHandler_Aid_InsufficientAP(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-aid-4"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayerNamed(t, h.sessions, "player-aid-4a", roomID, "Alice")
	addTestPlayerNamed(t, h.sessions, "player-aid-4b", roomID, "Bob")

	// Start combat (costs 1 AP, 2 remain).
	_, err := h.Attack("player-aid-4a", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	// Strike costs 2 AP, leaving 0 AP.
	_, err = h.Strike("player-aid-4a", "Goblin")
	if err != nil {
		t.Fatalf("Strike: %v", err)
	}

	// Now 0 AP remaining; Aid (cost 2) should fail.
	_, err = h.Aid("player-aid-4a", "Bob")
	if err == nil {
		t.Fatal("expected error for insufficient AP; got nil")
	}
}

// TestCombatHandler_Aid_Success verifies that Aid succeeds when the actor has 2+ AP and
// allyName refers to a living player combatant in the same combat.
func TestCombatHandler_Aid_Success(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-aid-5"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayerNamed(t, h.sessions, "player-aid-5a", roomID, "Alice")
	addTestPlayerNamed(t, h.sessions, "player-aid-5b", roomID, "Bob")

	// Start combat (costs 1 AP, 2 remain).
	_, err := h.Attack("player-aid-5a", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	// Manually add Bob as a player combatant in the same combat so Aid can target him.
	bobCbt := &combat.Combatant{
		ID:        "player-aid-5b",
		Kind:      combat.KindPlayer,
		Name:      "Bob",
		MaxHP:     10,
		CurrentHP: 10,
		AC:        10,
		Level:     1,
	}
	if addErr := h.engine.AddCombatant(roomID, bobCbt); addErr != nil {
		t.Fatalf("AddCombatant Bob: %v", addErr)
	}

	// Aid costs 2 AP; Alice has 2 remaining.
	events, err := h.Aid("player-aid-5a", "Bob")
	if err != nil {
		t.Fatalf("Aid returned unexpected error: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least one event from Aid; got none")
	}
}
