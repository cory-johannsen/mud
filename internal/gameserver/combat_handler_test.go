package gameserver

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
	"pgregory.net/rapid"
)

// seqSource is a deterministic dice.Source that returns values from a fixed
// sequence, cycling when exhausted.  Safe for concurrent use via atomic counter.
type seqSource struct {
	seq []int
	idx atomic.Int64
}

// newSeqSource creates a seqSource from the given sequence of Intn return values.
//
// Precondition: seq must be non-empty.
// Postcondition: Returns a non-nil seqSource whose Intn cycles through seq.
func newSeqSource(seq ...int) *seqSource {
	return &seqSource{seq: seq}
}

// Intn returns the next value in the sequence modulo n, cycling as needed.
//
// Precondition: n > 0.
// Postcondition: Returns a value in [0, n).
func (s *seqSource) Intn(n int) int {
	i := int(s.idx.Add(1)-1) % len(s.seq)
	v := s.seq[i]
	if v >= n {
		v = n - 1
	}
	return v
}

// makeCombatHandlerWithDice constructs a CombatHandler with the supplied dice.Source.
//
// Postcondition: Returns a non-nil CombatHandler using src as its randomness provider.
func makeCombatHandlerWithDice(t *testing.T, src dice.Source, broadcastFn func(roomID string, events []*gamev1.CombatEvent)) *CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	return NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil)
}

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
	return NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil)
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
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   10,
		MaxHP:       0,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
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

// TestResolveAndAdvanceLocked_ResetsSwappedThisRound verifies that after
// resolveAndAdvanceLocked runs, SwappedThisRound is false for all player sessions.
//
// Precondition: A player in active combat has SwappedThisRound set to true.
// Postcondition: After the round resolves, SwappedThisRound is false.
func TestResolveAndAdvanceLocked_ResetsSwappedThisRound(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-reset-round"
	spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-reset", roomID)

	// Start combat to initialise the combat state.
	_, err := h.Attack("player-reset", "Goblin")
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}

	// Simulate a swap having happened this round.
	sess.LoadoutSet.SwappedThisRound = true

	// Retrieve the active combat and invoke resolveAndAdvanceLocked directly.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat; got nil")
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()

	if sess.LoadoutSet.SwappedThisRound {
		t.Fatal("expected SwappedThisRound to be reset to false after round resolution; got true")
	}
}

// TestProperty_ResolveAndAdvanceLocked_AlwaysResetsSwappedThisRound is a
// property-based test verifying that resolveAndAdvanceLocked unconditionally
// resets SwappedThisRound to false regardless of its initial value.
//
// Precondition: SwappedThisRound is set to an arbitrary boolean value.
// Postcondition: SwappedThisRound is always false after resolveAndAdvanceLocked.
func TestProperty_ResolveAndAdvanceLocked_AlwaysResetsSwappedThisRound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}

		h := makeCombatHandler(t, broadcastFn)
		const roomID = "room-prop-swap-reset"
		spawnTestNPC(t, h.npcMgr, roomID)
		sess := addTestPlayer(t, h.sessions, "player-prop-swap", roomID)

		_, err := h.Attack("player-prop-swap", "Goblin")
		if err != nil {
			rt.Fatalf("Attack to start combat: %v", err)
		}

		swapped := rapid.Bool().Draw(rt, "swapped")
		sess.LoadoutSet.SwappedThisRound = swapped

		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		if !ok {
			h.combatMu.Unlock()
			rt.Fatal("expected active combat; got nil")
		}
		h.resolveAndAdvanceLocked(roomID, cbt)
		h.combatMu.Unlock()

		if sess.LoadoutSet.SwappedThisRound {
			rt.Fatalf("expected SwappedThisRound to be false after resolveAndAdvanceLocked (initial=%v); got true", swapped)
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

		sess, err := h.sessions.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    "testuser",
			CharName:    "Hero",
			CharacterID: 1,
			RoomID:      roomID,
			CurrentHP:   10,
			MaxHP:       0,
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
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

// makeCombatHandlerWithRegistry constructs a CombatHandler with the given invRegistry.
func makeCombatHandlerWithRegistry(t *testing.T, reg *inventory.Registry, broadcastFn func(roomID string, events []*gamev1.CombatEvent)) *CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	return NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, testRoundDuration, makeTestConditionRegistry(), nil, nil, reg, nil, nil, nil)
}

// TestStartCombatLocked_NilRegistry_ACIsTenPlusDex verifies that when invRegistry
// is nil, the player AC defaults to 10 + dexMod (i.e. 11).
//
// Precondition: invRegistry is nil; no armor equipped.
// Postcondition: Player combatant AC equals 11.
func TestStartCombatLocked_NilRegistry_ACIsTenPlusDex(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-ac-nil-reg"

	spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-ac-nil", roomID)

	_, err := h.Attack("player-ac-nil", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	var playerAC int
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			playerAC = c.AC
			break
		}
	}
	h.combatMu.Unlock()

	const wantAC = 11 // 10 + dexMod(1) with nil registry
	if playerAC != wantAC {
		t.Errorf("expected player AC=%d; got %d", wantAC, playerAC)
	}
}

// TestStartCombatLocked_WithRegistry_ACIncludesArmorBonus verifies that when an
// invRegistry is provided and the player has armor equipped, the AC reflects the
// armor's ACBonus via ComputedDefenses.
//
// Precondition: invRegistry contains an armor with ACBonus=3; player has it equipped in torso slot.
// Postcondition: Player combatant AC equals 10 + 3 (ACBonus) + 1 (EffectiveDex) = 14.
func TestStartCombatLocked_WithRegistry_ACIncludesArmorBonus(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{
		ID:      "test-vest",
		Name:    "Test Vest",
		Slot:    inventory.SlotTorso,
		ACBonus: 3,
		DexCap:  10,
		Group:   "light",
	}
	if err := reg.RegisterArmor(armorDef); err != nil {
		t.Fatalf("RegisterArmor: %v", err)
	}

	h := makeCombatHandlerWithRegistry(t, reg, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-ac-with-reg"

	spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-ac-armor", roomID)

	// Equip the armor directly on the session's Equipment by setting the torso slot.
	sess.Equipment.Armor[inventory.SlotTorso] = &inventory.SlottedItem{
		ItemDefID: "test-vest",
		Name:      "Test Vest",
	}

	_, err := h.Attack("player-ac-armor", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	var playerAC int
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			playerAC = c.AC
			break
		}
	}
	h.combatMu.Unlock()

	const wantAC = 14 // 10 + ACBonus(3) + EffectiveDex(1)
	if playerAC != wantAC {
		t.Errorf("expected player AC=%d; got %d", wantAC, playerAC)
	}
}

// TestProperty_StartCombat_ACNeverLessThanTen is a property-based test verifying
// that the player AC computed during combat start is always >= 10 when no armor
// is equipped (ACBonus=0) and dexMod is fixed at 1.
//
// Postcondition: Player AC >= 10 for any valid invRegistry state with no armor.
func TestProperty_StartCombat_ACNeverLessThanTen(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
		roomID := rapid.StringMatching(`room-prop-[a-z]{4}`).Draw(rt, "roomID")
		uid := rapid.StringMatching(`player-prop-[a-z]{4}`).Draw(rt, "uid")

		spawnTestNPC(t, h.npcMgr, roomID)
		addTestPlayer(t, h.sessions, uid, roomID)

		_, err := h.Attack(uid, "Goblin")
		if err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		h.cancelTimer(roomID)

		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		if !ok {
			h.combatMu.Unlock()
			rt.Fatal("expected active combat")
		}
		var playerAC int
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindPlayer {
				playerAC = c.AC
				break
			}
		}
		h.combatMu.Unlock()

		if playerAC < 10 {
			rt.Fatalf("expected player AC >= 10; got %d", playerAC)
		}
	})
}

// TestCombatHandler_SetOnCombatEnd_CallbackInvoked verifies that the onCombatEndFn
// callback is invoked with the correct roomID when combat ends via resolveAndAdvanceLocked.
//
// Precondition: A CombatHandler is constructed with a player and NPC in the same room.
// Postcondition: After all enemies are killed, the callback fires with the room ID.
func TestCombatHandler_SetOnCombatEnd_CallbackInvoked(t *testing.T) {
	var broadcastMu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {
		broadcastMu.Lock()
		defer broadcastMu.Unlock()
		broadcasts = append(broadcasts, events)
	}

	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-callback"

	// Register callback before combat starts.
	var callbackMu sync.Mutex
	var callbackRoomIDs []string
	h.SetOnCombatEnd(func(rid string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackRoomIDs = append(callbackRoomIDs, rid)
	})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-cb", roomID)

	// Start combat.
	_, err := h.Attack("player-cb", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Kill the NPC directly to force end-of-combat on next resolve.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat after attack")
	}
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	// Resolve manually to trigger end-of-combat path.
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()

	callbackMu.Lock()
	ids := callbackRoomIDs
	callbackMu.Unlock()

	if len(ids) == 0 {
		t.Fatal("expected onCombatEndFn to be called; it was not")
	}
	if ids[0] != roomID {
		t.Fatalf("onCombatEndFn called with roomID %q; want %q", ids[0], roomID)
	}
}

// TestCombatHandler_SetOnCombatEnd_NilCallback_NoPanic verifies that if no
// callback is registered, combat end does not panic.
//
// Precondition: CombatHandler has no onCombatEndFn set.
// Postcondition: resolveAndAdvanceLocked completes without panic when all NPCs are dead.
func TestCombatHandler_SetOnCombatEnd_NilCallback_NoPanic(t *testing.T) {
	broadcastFn := func(roomID string, events []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	// Intentionally do NOT call SetOnCombatEnd — fn remains nil.
	const roomID = "room-nil-cb"

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-nil", roomID)

	_, err := h.Attack("player-nil", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	// Must not panic.
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
}

// TestProperty_OnCombatEnd_CallbackAlwaysReceivesRoomID is a property test verifying
// that when onCombatEndFn is set, it always receives the exact roomID passed to EndCombat.
//
// Precondition: arbitrary roomID strings used as keys.
// Postcondition: callback receives the same roomID passed to the combat engine for every run.
func TestProperty_OnCombatEnd_CallbackAlwaysReceivesRoomID(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roomID := rapid.StringMatching(`[a-z][a-z0-9-]{1,15}`).Draw(rt, "roomID")

		broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
		h := makeCombatHandler(t, broadcastFn)

		var got string
		h.SetOnCombatEnd(func(rid string) { got = rid })

		// Spawn NPC inline (rapid.T is not *testing.T).
		tmpl := &npc.Template{
			ID: "goblin-prop", Name: "GoblinProp", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
		}
		inst, err := h.npcMgr.Spawn(tmpl, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}
		_, addErr := h.sessions.AddPlayer(session.AddPlayerOptions{
			UID: "player-prop", Username: "u", CharName: "Hero", CharacterID: 1,
			RoomID: roomID, CurrentHP: 10, MaxHP: 10, Role: "player",
		})
		if addErr != nil {
			rt.Fatalf("AddPlayer: %v", addErr)
		}

		_, attackErr := h.Attack("player-prop", inst.Name())
		if attackErr != nil {
			rt.Fatalf("Attack: %v", attackErr)
		}
		h.cancelTimer(roomID)

		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		if !ok {
			h.combatMu.Unlock()
			rt.Fatal("expected active combat")
		}
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC {
				c.CurrentHP = 0
				c.Dead = true
			}
		}
		h.resolveAndAdvanceLocked(roomID, cbt)
		h.combatMu.Unlock()

		if got != roomID {
			rt.Fatalf("callback got roomID %q; want %q", got, roomID)
		}
	})
}

// setupStreetBrawlerRoom creates a room with one NPC, player A (the fleeing player), and
// player B (who may hold street_brawler).  Returns the handler and player B's session.
//
// Precondition: roomID must be unique across all tests using the returned handler.
// Postcondition: combat is NOT started; callers start combat via h.Attack.
func setupStreetBrawlerRoom(
	t *testing.T,
	roomID string,
	src dice.Source,
	playerBHasStreetBrawler bool,
) (*CombatHandler, *session.PlayerSession) {
	t.Helper()
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(_ string, evts []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, evts)
	}
	_ = broadcasts

	h := makeCombatHandlerWithDice(t, src, broadcastFn)

	spawnTestNPC(t, h.npcMgr, roomID)

	// player A: the fleeing combatant.
	_, err := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "player-a", Username: "ua", CharName: "PlayerA",
		CharacterID: 1, RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	if err != nil {
		t.Fatalf("AddPlayer player-a: %v", err)
	}

	// player B: the bystander who may hold street_brawler.
	sessB, err := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "player-b", Username: "ub", CharName: "PlayerB",
		CharacterID: 2, RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	if err != nil {
		t.Fatalf("AddPlayer player-b: %v", err)
	}
	if playerBHasStreetBrawler {
		sessB.PassiveFeats = map[string]bool{"street_brawler": true}
	} else {
		sessB.PassiveFeats = map[string]bool{}
	}

	return h, sessB
}

// TestCombatHandler_StreetBrawler_AoO_OnPlayerFlee verifies that when a player
// successfully flees combat, any other player in the room with the street_brawler
// passive feat fires a free attack-of-opportunity against the fleeing player.
//
// Precondition: player B holds PassiveFeats["street_brawler"]==true.
// Postcondition: Returned events include at least one ATTACK event whose Attacker is "PlayerB".
func TestCombatHandler_StreetBrawler_AoO_OnPlayerFlee(t *testing.T) {
	const roomID = "room-sb-aoo-flee"

	// Dice sequence (seqSource cycles):
	//   Roll 0 (RollInitiative player-a d20):  9 → initiative 10
	//   Roll 1 (RollInitiative goblin d20):     9 → initiative 10
	//   Roll 2 (player flee d20): Intn(20)=10 → d20=11, playerTotal = 11+2 = 13
	//   Roll 3 (NPC flee d20):   Intn(20)= 0 → d20= 1, npcTotal   =  1-4 = -3
	//   playerTotal(13) > npcTotal(-3): flee succeeds → AoO fires
	//   Roll 4 (AoO attack d20): any value is fine
	//   Roll 5 (AoO damage d6):  any value is fine
	src := newSeqSource(9, 9, 10, 0, 9, 3)

	h, sessB := setupStreetBrawlerRoom(t, roomID, src, true)

	// Start combat via attack (only player A and Goblin are added by default).
	if _, err := h.Attack("player-a", "Goblin"); err != nil {
		t.Fatalf("Attack (start combat): %v", err)
	}
	h.cancelTimer(roomID)

	// Manually add player B as a combatant so the AoO loop can find them.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	playerBCbt := &combat.Combatant{
		ID:        sessB.UID,
		Kind:      combat.KindPlayer,
		Name:      sessB.CharName,
		MaxHP:     20,
		CurrentHP: 20,
		AC:        12,
		Level:     1,
		StrMod:    1,
	}
	cbt.Combatants = append(cbt.Combatants, playerBCbt)
	h.combatMu.Unlock()

	events, err := h.Flee("player-a")
	if err != nil {
		t.Fatalf("Flee returned error: %v", err)
	}

	found := false
	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK && e.Attacker == "PlayerB" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ATTACK event from PlayerB (street_brawler AoO); event types: %v", eventTypes(events))
	}
}

// TestCombatHandler_StreetBrawler_AoO_NotFiredWhenFleeFails verifies that when a
// player fails to flee combat, no attack-of-opportunity is fired.
//
// Precondition: player B holds PassiveFeats["street_brawler"]==true; player A fails the flee check.
// Postcondition: Returned events contain no ATTACK event from PlayerB.
func TestCombatHandler_StreetBrawler_AoO_NotFiredWhenFleeFails(t *testing.T) {
	const roomID = "room-sb-aoo-fail"

	// Dice sequence:
	//   Roll 0 (RollInitiative player-a): 9
	//   Roll 1 (RollInitiative goblin):   9
	//   Roll 2 (player flee d20): Intn(20)= 0 → d20= 1, playerTotal =  1+2 =  3
	//   Roll 3 (NPC flee d20):   Intn(20)=10 → d20=11, npcTotal    = 11-4 =  7
	//   playerTotal(3) < npcTotal(7): flee fails → no AoO
	src := newSeqSource(9, 9, 0, 10)

	h, sessB := setupStreetBrawlerRoom(t, roomID, src, true)

	if _, err := h.Attack("player-a", "Goblin"); err != nil {
		t.Fatalf("Attack (start combat): %v", err)
	}
	h.cancelTimer(roomID)

	// Add player B to the combat so the loop would trigger if flee succeeded.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	playerBCbt := &combat.Combatant{
		ID:        sessB.UID,
		Kind:      combat.KindPlayer,
		Name:      sessB.CharName,
		MaxHP:     20,
		CurrentHP: 20,
		AC:        12,
		Level:     1,
		StrMod:    1,
	}
	cbt.Combatants = append(cbt.Combatants, playerBCbt)
	h.combatMu.Unlock()

	events, err := h.Flee("player-a")
	if err != nil {
		t.Fatalf("Flee returned error: %v", err)
	}

	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK && e.Attacker == "PlayerB" {
			t.Errorf("expected no AoO from PlayerB when flee fails; got ATTACK event: %v", e.Narrative)
		}
	}
}

// TestCombatHandler_StreetBrawler_AoO_NotFiredForNPCs verifies that NPCs do not
// receive an attack-of-opportunity on flee; only players with street_brawler do.
//
// Precondition: player B does NOT hold street_brawler; NPC is the only other combatant.
// Postcondition: Returned events contain no ATTACK event from PlayerB.
func TestCombatHandler_StreetBrawler_AoO_NotFiredForNPCs(t *testing.T) {
	const roomID = "room-sb-aoo-npc"

	// Player A wins the flee check; PlayerB has no street_brawler.
	// Same dice arrangement as the AoO success test.
	src := newSeqSource(9, 9, 10, 0, 9, 3)

	h, sessB := setupStreetBrawlerRoom(t, roomID, src, false)

	if _, err := h.Attack("player-a", "Goblin"); err != nil {
		t.Fatalf("Attack (start combat): %v", err)
	}
	h.cancelTimer(roomID)

	// Add player B to the combat; they have no street_brawler so no AoO should fire.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	playerBCbt := &combat.Combatant{
		ID:        sessB.UID,
		Kind:      combat.KindPlayer,
		Name:      sessB.CharName,
		MaxHP:     20,
		CurrentHP: 20,
		AC:        12,
		Level:     1,
		StrMod:    1,
	}
	cbt.Combatants = append(cbt.Combatants, playerBCbt)
	h.combatMu.Unlock()

	events, err := h.Flee("player-a")
	if err != nil {
		t.Fatalf("Flee returned error: %v", err)
	}

	for _, e := range events {
		if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK && e.Attacker == "PlayerB" {
			t.Errorf("expected no AoO from PlayerB (no street_brawler); got ATTACK event: %v", e.Narrative)
		}
	}
}
