package gameserver

import (
	"sync"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestCombatEnd_PlayerStatusResetToIdle verifies that after combat ends
// (all NPCs dead), every player session in the room has Status reset from
// statusInCombat (2) back to idle (1).
//
// This is the root cause of BUG-32: post-combat movement is blocked because
// the player session remains stuck in combat status.
//
// Precondition: Player is in combat (Status == 2); NPC is killed.
// Postcondition: After round resolution ends combat, player Status == 1.
func TestCombatEnd_PlayerStatusResetToIdle(t *testing.T) {
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-end-status"

	// Wire onCombatEndFn exactly as grpc_service.go does.
	// Use a channel to synchronise with the goroutine that fires the callback.
	combatEndDone := make(chan struct{})
	h.SetOnCombatEnd(func(rid string) {
		sessions := h.sessions.PlayersInRoomDetails(rid)
		for _, sess := range sessions {
			sess.Status = int32(1) // idle
		}
		close(combatEndDone)
	})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-end", roomID)

	// Start combat via Attack.
	if _, err := h.Attack("player-end", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Verify player is in combat.
	if sess.Status != statusInCombat {
		t.Fatalf("expected Status == %d (in combat); got %d", statusInCombat, sess.Status)
	}

	// Kill the NPC to trigger end-of-combat on next resolve.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
		}
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	<-combatEndDone // wait for the post-unlock goroutine

	// BUG-32: player status must be reset to idle after combat ends.
	if sess.Status != int32(1) {
		t.Errorf("BUG-32: expected Status == 1 (idle) after combat end; got %d", sess.Status)
	}
}

// TestCombatEnd_DisconnectCleansUpSession verifies that when a player
// disconnects, RemovePlayer succeeds even if the player was in combat.
// A subsequent AddPlayer with the same UID must succeed (not "already connected").
//
// This reproduces the second symptom of BUG-32: reconnecting shows
// "That character is already logged in."
//
// Precondition: Player is in combat (Status == 2); player disconnects.
// Postcondition: RemovePlayer succeeds; re-AddPlayer succeeds.
func TestCombatEnd_DisconnectCleansUpSession(t *testing.T) {
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-disconnect"

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	sess := addTestPlayer(t, h.sessions, "player-dc", roomID)

	// Start combat.
	if _, err := h.Attack("player-dc", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	if sess.Status != statusInCombat {
		t.Fatalf("expected in-combat status; got %d", sess.Status)
	}

	// Simulate disconnect: remove player from session manager.
	if err := h.sessions.RemovePlayer("player-dc"); err != nil {
		t.Fatalf("RemovePlayer: %v", err)
	}

	// Re-add with same UID must succeed (no "already connected" error).
	_, err := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID:      "player-dc",
		Username: "testuser",
		CharName: "Hero",
		RoomID:   roomID,
		Role:     "player",
	})
	if err != nil {
		t.Errorf("BUG-32: re-AddPlayer after disconnect failed: %v", err)
	}
}

// TestCombatEnd_TimerCleanedUp verifies that when combat ends via
// resolveAndAdvanceLocked, the room timer is cleaned up from the timers map.
//
// Precondition: Combat is active with a timer; NPC is killed; round resolves.
// Postcondition: IsRoomInCombat returns false.
func TestCombatEnd_TimerCleanedUp(t *testing.T) {
	var mu sync.Mutex
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-timer-cleanup"

	h.SetOnCombatEnd(func(rid string) {})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-timer", roomID)

	if _, err := h.Attack("player-timer", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	// Don't cancel the timer — we want to verify it's cleaned up by combat end.
	// But we do need to stop it from auto-resolving while we modify NPC HP.
	h.cancelTimer(roomID)

	// Re-start timer to simulate the real flow where a timer exists.
	h.combatMu.Lock()
	h.startTimerLocked(roomID)
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}

	// Kill NPC.
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
		}
	}

	// Resolve — should end combat and clean up timer.
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()

	if h.IsRoomInCombat(roomID) {
		t.Errorf("expected IsRoomInCombat == false after combat end; timer not cleaned up")
	}
}

// TestCombatEnd_DisconnectedPlayerDoesNotBlockCombatEnd verifies that when a
// player disconnects mid-combat, the combat engine still ends combat properly
// once all NPCs are dead — a ghost combatant (player removed from session
// manager but still in the combat engine) must not prevent combat from ending.
//
// This reproduces the core pathology of BUG-32: combat never ends because
// the disconnected player's combatant has Dead==false (only settable via
// session lookup), so HasLivingPlayers() returns true forever.
//
// Precondition: Player is in combat; player disconnects (removed from session
//   manager but not from combat engine); NPC is killed.
// Postcondition: Combat ends; the combat engine no longer has an active combat
//   for the room.
func TestCombatEnd_DisconnectedPlayerDoesNotBlockCombatEnd(t *testing.T) {
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-ghost"

	var combatEnded bool
	combatEndDone2 := make(chan struct{})
	h.SetOnCombatEnd(func(rid string) {
		combatEnded = true
		close(combatEndDone2)
	})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-ghost", roomID)

	// Start combat.
	if _, err := h.Attack("player-ghost", inst.Name()); err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Simulate disconnect: remove player from session manager but NOT from combat engine.
	if err := h.sessions.RemovePlayer("player-ghost"); err != nil {
		t.Fatalf("RemovePlayer: %v", err)
	}

	// Kill the NPC.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat")
	}
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
		}
	}

	// Resolve — combat should end even though the ghost player's Dead flag is false.
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	<-combatEndDone2 // wait for the post-unlock goroutine

	if !combatEnded {
		t.Errorf("BUG-32: combat did not end — ghost player combatant blocked HasLivingPlayers()")
	}

	// Engine should have no active combat for this room.
	if _, stillActive := h.engine.GetCombat(roomID); stillActive {
		t.Errorf("BUG-32: combat engine still has active combat for room after all NPCs dead")
	}
}

// TestCombatEnd_StaleSessionReplacedOnReconnect verifies that when a player
// reconnects while the old session is still in the session manager (race
// between cleanup and reconnect), the new AddPlayer call succeeds by
// evicting the stale session.
//
// This reproduces the "already logged in" symptom of BUG-32.
//
// Precondition: Player session exists in the session manager.
// Postcondition: A second AddPlayer with the same UID succeeds (evicts stale).
func TestCombatEnd_StaleSessionReplacedOnReconnect(t *testing.T) {
	sessMgr := session.NewManager()

	// First login.
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "player-stale",
		Username: "testuser",
		CharName: "Hero",
		RoomID:   "room-stale",
		Role:     "player",
	})
	if err != nil {
		t.Fatalf("first AddPlayer: %v", err)
	}

	// Second login with same UID (simulates reconnect before cleanup).
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "player-stale",
		Username: "testuser",
		CharName: "Hero",
		RoomID:   "room-stale",
		Role:     "player",
	})
	if err != nil {
		t.Errorf("BUG-32: reconnect AddPlayer failed with stale session: %v", err)
	}
}
