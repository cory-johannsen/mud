package gameserver

import (
	"strings"
	"sync"
	"testing"
	"time"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"google.golang.org/protobuf/proto"
)

// drainAllEntityEvents reads all pending events from a session entity channel,
// returning both message content strings and combat event narratives.
func drainAllEntityEvents(t *testing.T, ch <-chan []byte, timeout time.Duration) []string {
	t.Helper()
	var msgs []string
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case data := <-ch:
			var evt gamev1.ServerEvent
			if err := proto.Unmarshal(data, &evt); err != nil {
				continue
			}
			switch p := evt.Payload.(type) {
			case *gamev1.ServerEvent_Message:
				msgs = append(msgs, p.Message.Content)
			case *gamev1.ServerEvent_CombatEvent:
				msgs = append(msgs, p.CombatEvent.Narrative)
			}
		case <-timer.C:
			return msgs
		}
	}
}

// TestAPDisplay_RoundStartShowsAP verifies that at the start of each combat round,
// each player receives a per-player message showing their remaining action points.
//
// This is the root cause of BUG-31: AP is never displayed to the player.
//
// Precondition: Player is in combat; round resolves and a new round starts.
// Postcondition: Player's entity channel contains a message matching "AP" and "3".
func TestAPDisplay_RoundStartShowsAP(t *testing.T) {
	var mu sync.Mutex
	var broadcasts [][]*gamev1.CombatEvent
	broadcastFn := func(_ string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		broadcasts = append(broadcasts, events)
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-ap-display"

	h.SetOnCombatEnd(func(rid string) {})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-ap", roomID)

	// Start combat.
	if _, err := h.Attack("player-ap", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Drain any init messages from the entity channel.
	drainAllEntityEvents(t, sess.Entity.Events(), 50*time.Millisecond)

	// Resolve the round to trigger a new round start.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	h.cancelTimer(roomID)

	// Check that the player received an AP message.
	msgs := drainAllEntityEvents(t, sess.Entity.Events(), 100*time.Millisecond)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "AP") && strings.Contains(m, "3") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BUG-31: expected per-player AP message at round start; got messages: %v", msgs)
	}
}

// TestAPDisplay_AfterAttackShowsRemainingAP verifies that after a player
// queues an attack action, they receive a message showing remaining AP.
//
// Precondition: Player is in combat with AP available.
// Postcondition: Attack response or subsequent entity message contains AP info.
func TestAPDisplay_AfterAttackShowsRemainingAP(t *testing.T) {
	var mu sync.Mutex
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-ap-after-attack"

	h.SetOnCombatEnd(func(rid string) {})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-ap-atk", roomID)

	// Start combat.
	if _, err := h.Attack("player-ap-atk", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Drain init messages.
	drainAllEntityEvents(t, sess.Entity.Events(), 50*time.Millisecond)

	// Player attacks again (costs 1 AP, should have 2 remaining).
	events, err := h.Attack("player-ap-atk", inst.Name())
	if err != nil {
		t.Fatalf("second Attack: %v", err)
	}

	// Check broadcast events and entity messages for AP info.
	foundInEvents := false
	for _, evt := range events {
		if strings.Contains(evt.Narrative, "AP") {
			foundInEvents = true
			break
		}
	}
	msgs := drainAllEntityEvents(t, sess.Entity.Events(), 100*time.Millisecond)
	foundInMsgs := false
	for _, m := range msgs {
		if strings.Contains(m, "AP") {
			foundInMsgs = true
			break
		}
	}
	if !foundInEvents && !foundInMsgs {
		t.Errorf("BUG-31: expected AP remaining info after attack; events: %v, entity msgs: %v", events, msgs)
	}
}
