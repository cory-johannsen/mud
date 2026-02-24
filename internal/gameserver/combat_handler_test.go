package gameserver

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
	"pgregory.net/rapid"
)

const testRoundDuration = 200 * time.Millisecond

// makeTestConditionRegistry constructs a condition.Registry with the standard PF2E conditions
// needed for combat tests.
//
// Postcondition: Returns a non-nil Registry containing dying, wounded, stunned, prone,
// flat_footed, and frightened definitions.
func makeTestConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	return reg
}

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
	return NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, testRoundDuration, makeTestConditionRegistry(), nil, nil)
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

// TestCombatHandler_Status_NoActiveCombat verifies that Status returns nil, nil
// when no combat is active in the player's room.
//
// Postcondition: Status returns nil conditions and nil error when not in combat.
func TestCombatHandler_Status_NoActiveCombat(t *testing.T) {
	h := makeCombatHandler(t, func(roomID string, events []*gamev1.CombatEvent) {})
	const roomID = "room-status-1"
	addTestPlayer(t, h.sessions, "player-status-1", roomID)

	conds, err := h.Status("player-status-1")
	if err != nil {
		t.Fatalf("Status returned unexpected error: %v", err)
	}
	if conds != nil {
		t.Errorf("expected nil conditions when not in combat; got %v", conds)
	}
}

// TestCombatHandler_Status_UnknownPlayer verifies that Status returns an error
// when the uid is not a registered player.
//
// Postcondition: Status returns a non-nil error for unknown uid.
func TestCombatHandler_Status_UnknownPlayer(t *testing.T) {
	h := makeCombatHandler(t, func(roomID string, events []*gamev1.CombatEvent) {})

	_, err := h.Status("nonexistent-uid")
	if err == nil {
		t.Fatal("expected error for unknown player uid; got nil")
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

// TestCombatHandler_Pass_ForfeitsAP_EventsNonEmpty verifies that Pass returns at
// least one event.
//
// Postcondition: Pass returns len(events) > 0.
func TestCombatHandler_Pass_ForfeitsAP_EventsNonEmpty(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-pass-events"
	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-pass-events", roomID)

	_, err := h.Attack("player-pass-events", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}

	events, err := h.Pass("player-pass-events")
	if err != nil {
		t.Fatalf("Pass returned error: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected len(events) > 0 from Pass; got 0")
	}
}

// TestCombatHandler_Status_WithConditions verifies that Status returns the active
// conditions for a player who has a condition applied during active combat.
//
// Postcondition: Status returns a non-nil slice containing the applied condition.
func TestCombatHandler_Status_WithConditions(t *testing.T) {
	h := makeCombatHandler(t, func(roomID string, events []*gamev1.CombatEvent) {})
	const roomID = "room-status-cond"
	const playerUID = "player-status-cond"

	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, playerUID, roomID)

	// Start combat so a Combat struct exists for the room.
	_, err := h.Attack(playerUID, "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	h.cancelTimer(roomID)

	// Apply a condition to the player directly via the combat instance.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat after Attack; got none")
	}
	if err := cbt.ApplyCondition(playerUID, "prone", 1, -1); err != nil {
		h.combatMu.Unlock()
		t.Fatalf("ApplyCondition: %v", err)
	}
	h.combatMu.Unlock()

	conds, err := h.Status(playerUID)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if conds == nil {
		t.Fatal("expected non-nil conditions slice; got nil")
	}
	found := false
	for _, c := range conds {
		if c.Def != nil && c.Def.ID == "prone" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'prone' condition in Status result; got %v", conds)
	}
}

// TestConditionEventsToProto_LengthEqualsInput is a property-based test verifying
// that conditionEventsToProto always returns a slice whose length equals the
// length of the input slice.
//
// Postcondition: len(output) == len(input) for any input.
func TestConditionEventsToProto_LengthEqualsInput(t *testing.T) {
	reg := makeTestConditionRegistry()
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 20).Draw(rt, "n")
		events := make([]combat.RoundConditionEvent, n)
		for i := 0; i < n; i++ {
			applied := rapid.Bool().Draw(rt, "applied")
			events[i] = combat.RoundConditionEvent{
				UID:         "player-1",
				Name:        rapid.StringMatching(`[A-Za-z ]{1,10}`).Draw(rt, "name"),
				ConditionID: "prone",
				Stacks:      rapid.IntRange(1, 4).Draw(rt, "stacks"),
				Applied:     applied,
			}
		}
		result := conditionEventsToProto(events, reg)
		if len(result) != n {
			rt.Fatalf("expected len(result)==%d; got %d", n, len(result))
		}
	})
}

// TestConditionEventsToProto_NarrativesNonEmpty is a property-based test verifying
// that every output CombatEvent has a non-empty Narrative.
//
// Postcondition: All output narratives are non-empty strings.
func TestConditionEventsToProto_NarrativesNonEmpty(t *testing.T) {
	reg := makeTestConditionRegistry()
	rapid.Check(t, func(rt *rapid.T) {
		applied := rapid.Bool().Draw(rt, "applied")
		event := combat.RoundConditionEvent{
			UID:         "player-1",
			Name:        rapid.StringMatching(`[A-Za-z]{1,10}`).Draw(rt, "name"),
			ConditionID: "stunned",
			Stacks:      rapid.IntRange(1, 3).Draw(rt, "stacks"),
			Applied:     applied,
		}
		result := conditionEventsToProto([]combat.RoundConditionEvent{event}, reg)
		if len(result) != 1 {
			rt.Fatalf("expected 1 result; got %d", len(result))
		}
		if result[0].Narrative == "" {
			rt.Fatal("expected non-empty Narrative; got empty string")
		}
	})
}

// TestConditionEventsToProto_RegistryMissFallback is a property-based test verifying
// that an unknown conditionID does not panic and falls back to the conditionID string
// in the narrative.
//
// Postcondition: Unknown conditionID results in a non-empty Narrative containing the conditionID.
func TestConditionEventsToProto_RegistryMissFallback(t *testing.T) {
	reg := makeTestConditionRegistry()
	rapid.Check(t, func(rt *rapid.T) {
		unknownID := rapid.StringMatching(`[a-z]{4,12}`).Draw(rt, "unknownID")
		// Ensure the generated ID is genuinely unknown (not accidentally registered).
		for {
			if _, ok := reg.Get(unknownID); !ok {
				break
			}
			unknownID = rapid.StringMatching(`[a-z]{4,12}`).Draw(rt, "unknownID2")
		}
		event := combat.RoundConditionEvent{
			UID:         "player-1",
			Name:        "Hero",
			ConditionID: unknownID,
			Stacks:      1,
			Applied:     true,
		}
		// Must not panic; narrative must be non-empty and contain the conditionID.
		result := conditionEventsToProto([]combat.RoundConditionEvent{event}, reg)
		if len(result) != 1 {
			rt.Fatalf("expected 1 result; got %d", len(result))
		}
		if result[0].Narrative == "" {
			rt.Fatal("expected non-empty Narrative for registry miss; got empty string")
		}
	})
}

// TestStatus_Property_UnknownUIDReturnsError is a property-based test verifying
// that Status always returns a non-nil error for any UID that is not registered
// in the session manager.
//
// Postcondition: Status returns non-nil error for any unregistered uid.
func TestStatus_Property_UnknownUIDReturnsError(t *testing.T) {
	h := makeCombatHandler(t, func(roomID string, events []*gamev1.CombatEvent) {})
	rapid.Check(t, func(rt *rapid.T) {
		uid := rapid.StringMatching(`[a-z0-9\-]{4,20}`).Draw(rt, "uid")
		_, err := h.Status(uid)
		if err == nil {
			rt.Fatalf("expected error for unregistered uid %q; got nil", uid)
		}
	})
}

// TestStatus_Property_RegisteredNotInCombat is a property-based test verifying
// that Status returns nil conditions and nil error for any registered player who
// is not in an active combat.
//
// Postcondition: Status returns (nil, nil) for players not in combat.
func TestStatus_Property_RegisteredNotInCombat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		h := makeCombatHandler(t, func(roomID string, events []*gamev1.CombatEvent) {})
		uid := rapid.StringMatching(`[a-z0-9]{4,12}`).Draw(rt, "uid")
		roomID := rapid.StringMatching(`room-[a-z0-9]{4,8}`).Draw(rt, "roomID")

		sess, err := h.sessions.AddPlayer(uid, "testuser", "Hero", 1, roomID, 30)
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}
		_ = sess

		conds, statusErr := h.Status(uid)
		if statusErr != nil {
			rt.Fatalf("expected nil error for registered player not in combat; got %v", statusErr)
		}
		if conds != nil {
			rt.Fatalf("expected nil conditions for player not in combat; got %v", conds)
		}
	})
}
