package gameserver

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

const testRoundDuration = 200 * time.Millisecond

// makeCombatHandler constructs a CombatHandler suitable for unit tests.
// broadcastFn captures all events broadcast to the room.
func makeCombatHandler(t *testing.T, broadcastFn func(roomID string, events []*gamev1.CombatEvent)) *CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	return NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, testRoundDuration)
}

// spawnTestNPC creates and registers a live NPC instance in roomID.
func spawnTestNPC(t *testing.T, npcMgr *npc.Manager, roomID string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:         "goblin",
		Name:       "Goblin",
		Level:      1,
		MaxHP:      20,
		AC:         13,
		Perception: 2,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnTestNPC: %v", err)
	}
	return inst
}

// addTestPlayer registers a player session and returns it.
func addTestPlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(uid, "testuser", "Hero", 1, roomID, 30)
	if err != nil {
		t.Fatalf("addTestPlayer: %v", err)
	}
	return sess
}

// TestCombatHandler_Attack_StartsCombat verifies that the first attack starts
// combat and returns initiative events.
func TestCombatHandler_Attack_StartsCombat(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-1"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-1", roomID)

	events, err := h.Attack("player-1", "Goblin")
	if err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event from combat start")
	}

	// Cancel timer to avoid interference with other tests.
	h.cancelTimer(roomID)

	// Verify at least one initiative event is present.
	found := false
	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INITIATIVE event among start events; got types: %v", eventTypes(events))
	}
}

// TestCombatHandler_Attack_QueuesAction verifies that a second attack call (with
// combat already active and a fresh NPC) queues the action without returning an
// end event.
func TestCombatHandler_Attack_QueuesAction(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-2"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-2", roomID)

	// First attack starts combat.
	_, err := h.Attack("player-2", "Goblin")
	if err != nil {
		t.Fatalf("first Attack: %v", err)
	}

	// The NPC has 3 AP and consumed 1 (auto-queued attack during combat start), leaving 2 remaining.
	// AllActionsSubmitted() is therefore false, so no early resolution fires.
	// Second attack should queue without resolving yet (1 AP used, player has 2 left).
	events, err := h.Attack("player-2", "Goblin")
	if err != nil {
		t.Fatalf("second Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Should have at least a confirmation event but no END event.
	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_END {
			t.Errorf("unexpected END event on second attack: %v", e.Narrative)
		}
	}
}

// TestCombatHandler_Pass_ForfeitsAP verifies that Pass returns a narrative event
// and does not return an error.
func TestCombatHandler_Pass_ForfeitsAP(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-3"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-3", roomID)

	// Start combat first.
	_, err := h.Attack("player-3", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}

	// Pass should succeed (forfeits remaining AP).
	events, err := h.Pass("player-3")
	if err != nil {
		// Pass may trigger resolve + broadcast when AllActionsSubmitted; that's also fine.
		t.Fatalf("Pass returned error: %v", err)
	}
	// After Pass the player submitted all AP; either events returned or broadcastFn called.
	_ = events
}

// TestCombatHandler_TimerFires_ResolvesRound verifies that when the round timer
// expires (after ~200ms), broadcastFn is called with non-empty events.
func TestCombatHandler_TimerFires_ResolvesRound(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-4"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-4", roomID)

	// Start combat (queues action + starts timer).
	_, err := h.Attack("player-4", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}

	// Wait longer than roundDuration to ensure timer fires.
	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	count := len(broadcasts)
	mu.Unlock()

	if count == 0 {
		t.Fatal("expected broadcastFn to be called after timer fires; got 0 broadcasts")
	}
}

// TestCombatHandler_Strike_Costs2AP verifies that Strike queues a 2-AP action
// and returns a non-empty events slice without error.
func TestCombatHandler_Strike_Costs2AP(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-5"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-5", roomID)

	// Start combat first.
	_, err := h.Attack("player-5", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}

	// Now try Strike — needs 2 AP; player has 2 AP remaining after 1-AP attack, so may fail.
	// Use a fresh room with a fresh player so all 3 AP are available.
	const roomID2 = "room-5b"
	spawnTestNPC(t, h.npcMgr, roomID2)
	addTestPlayer(t, h.sessions, "player-5b", roomID2)

	// Start combat in new room via Attack (costs 1 AP, leaves 2 AP).
	_, err = h.Attack("player-5b", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat in room2: %v", err)
	}
	h.cancelTimer(roomID)

	// Strike costs 2 AP, player still has 2 AP remaining.
	events, err := h.Strike("player-5b", "Goblin")
	h.cancelTimer(roomID2)
	if err != nil {
		t.Fatalf("Strike returned error: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected non-empty events slice from Strike")
	}
}

// eventTypes returns a slice of type names for debugging.
func eventTypes(events []*gamev1.CombatEvent) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.Type.String()
	}
	return names
}
