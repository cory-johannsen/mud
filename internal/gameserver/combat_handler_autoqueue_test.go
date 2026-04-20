package gameserver

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

// makeAutoQueueHandler builds a CombatHandler for autoQueuePlayersLocked tests.
//
// Postcondition: Returns a non-nil CombatHandler, *npc.Manager, and *session.Manager.
func makeAutoQueueHandler(t *testing.T) (*CombatHandler, *npc.Manager, *session.Manager) {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, 10*time.Second, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil)
	return h, npcMgr, sessMgr
}

// addAutoQueuePlayer registers a player with a specific DefaultCombatAction and LastCombatTarget.
//
// Precondition: sessMgr must be non-nil; uid, roomID, and charName must be non-empty.
// Postcondition: Returns the registered *session.PlayerSession.
func addAutoQueuePlayer(t *testing.T, sessMgr *session.Manager, uid, charName, roomID, defaultAction, lastTarget string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:                 uid,
		Username:            "testuser",
		CharName:            charName,
		CharacterID:         1,
		RoomID:              roomID,
		CurrentHP:           10,
		MaxHP:               10,
		Abilities:           character.AbilityScores{},
		Role:                "player",
		DefaultCombatAction: defaultAction,
	})
	if err != nil {
		t.Fatalf("addAutoQueuePlayer(%q): %v", uid, err)
	}
	sess.LastCombatTarget = lastTarget
	return sess
}

// spawnAutoQueueNPC creates a named NPC in the given room.
//
// Postcondition: Returns the spawned *npc.Instance.
func spawnAutoQueueNPC(t *testing.T, npcMgr *npc.Manager, roomID, name string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:         name,
		Name:       name,
		Level:      1,
		MaxHP:      20,
		AC:         13,
		Awareness: 2,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnAutoQueueNPC(%q): %v", name, err)
	}
	return inst
}

// startTestCombat starts combat with the given player and NPC sessions.
// Caller MUST NOT hold combatMu. Returns the active *combat.Combat after starting.
//
// Postcondition: Combat started; timer not started (no timer goroutine started).
func startTestCombat(t *testing.T, h *CombatHandler, playerUID, roomID, npcName string) *combat.Combat {
	t.Helper()
	_, err := h.Attack(playerUID, npcName)
	if err != nil {
		t.Fatalf("startTestCombat Attack(%q, %q): %v", playerUID, npcName, err)
	}
	h.cancelTimer(roomID)
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	h.combatMu.Unlock()
	if !ok {
		t.Fatalf("startTestCombat: no active combat in room %q", roomID)
	}
	return cbt
}

// TestAutoQueuePlayers_UnsubmittedPlayerGetsQueued verifies that a player with no
// queued action for the round is auto-queued with their DefaultCombatAction.
func TestAutoQueuePlayers_UnsubmittedPlayerGetsQueued(t *testing.T) {
	const roomID = "aq-room-1"
	h, npcMgr, sessMgr := makeAutoQueueHandler(t)

	spawnAutoQueueNPC(t, npcMgr, roomID, "Goblin")
	addAutoQueuePlayer(t, sessMgr, "player-aq-1", "Hero", roomID, "attack", "")

	cbt := startTestCombat(t, h, "player-aq-1", roomID, "Goblin")

	// Reset the round so Hero has no actions queued.
	_ = cbt.StartRound(3)

	h.combatMu.Lock()
	h.autoQueuePlayersLocked(cbt)
	h.combatMu.Unlock()

	q, ok := cbt.ActionQueues["player-aq-1"]
	if !ok {
		t.Fatal("expected ActionQueue for player-aq-1")
	}
	actions := q.QueuedActions()
	if len(actions) == 0 {
		t.Fatal("expected at least one queued action after auto-queue")
	}
	if actions[0].Type != combat.ActionAttack {
		t.Errorf("expected ActionAttack, got %v", actions[0].Type)
	}
}

// TestAutoQueuePlayers_AlreadySubmittedIsSkipped verifies that a player who already
// has a submitted action queue is not re-queued.
func TestAutoQueuePlayers_AlreadySubmittedIsSkipped(t *testing.T) {
	const roomID = "aq-room-2"
	h, npcMgr, sessMgr := makeAutoQueueHandler(t)

	spawnAutoQueueNPC(t, npcMgr, roomID, "Goblin")
	addAutoQueuePlayer(t, sessMgr, "player-aq-2", "Hero", roomID, "attack", "")

	// Attack to start combat and queue an action.
	_, err := h.Attack("player-aq-2", "Goblin")
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("no active combat")
	}
	q := cbt.ActionQueues["player-aq-2"]
	// Player already has one action queued from Attack; auto-queue should not add more.
	if len(q.QueuedActions()) == 0 {
		h.combatMu.Unlock()
		t.Fatal("expected player-aq-2 to have a queued action before auto-queue")
	}
	beforeActions := len(q.QueuedActions())

	h.autoQueuePlayersLocked(cbt)
	afterActions := len(q.QueuedActions())
	h.combatMu.Unlock()

	if afterActions != beforeActions {
		t.Errorf("expected action count to remain %d after auto-queue (already has actions), got %d", beforeActions, afterActions)
	}
}

// TestAutoQueuePlayers_LastCombatTargetUsedWhenAlive verifies that LastCombatTarget
// is used as the attack target when the named NPC is still alive in the combat.
// The player's LastCombatTarget is set to "Goblin" (the NPC already in combat).
// After a round reset, autoQueuePlayersLocked should queue an attack against "Goblin".
func TestAutoQueuePlayers_LastCombatTargetUsedWhenAlive(t *testing.T) {
	const roomID = "aq-room-3"
	h, npcMgr, sessMgr := makeAutoQueueHandler(t)

	spawnAutoQueueNPC(t, npcMgr, roomID, "Goblin")
	// Player's LastCombatTarget is set to "Goblin" — the NPC in combat.
	sess := addAutoQueuePlayer(t, sessMgr, "player-aq-3", "Hero", roomID, "attack", "Goblin")
	_ = sess

	cbt := startTestCombat(t, h, "player-aq-3", roomID, "Goblin")

	// Verify Goblin is in the combatants list.
	goblinInCombat := false
	for _, c := range cbt.Combatants {
		if c.Name == "Goblin" && !c.IsDead() {
			goblinInCombat = true
			break
		}
	}
	if !goblinInCombat {
		t.Fatal("Goblin should be a living combatant")
	}

	// Resolve a round so the system advances to round 2, which calls autoQueuePlayersLocked.
	// But we need to ensure the player has NO queued action after the round resets.
	// We use resolveAndAdvanceLocked directly after stopping the timer.
	h.combatMu.Lock()
	_ = cbt.StartRound(3) // reset queues so player has no actions
	// Verify the reset cleared player's action queue.
	q := cbt.ActionQueues["player-aq-3"]
	if q == nil {
		h.combatMu.Unlock()
		t.Fatal("no action queue for player-aq-3 after StartRound")
	}
	if len(q.QueuedActions()) != 0 {
		h.combatMu.Unlock()
		t.Fatalf("expected empty action queue after StartRound, got %d actions", len(q.QueuedActions()))
	}
	h.autoQueuePlayersLocked(cbt)
	actions := q.QueuedActions()
	h.combatMu.Unlock()

	if len(actions) == 0 {
		t.Fatal("expected at least one queued action after auto-queue")
	}
	if actions[0].Target != "Goblin" {
		t.Errorf("expected target 'Goblin' (from LastCombatTarget), got %q", actions[0].Target)
	}
}

// TestAutoQueuePlayers_FallbackWhenLastTargetDead verifies that when LastCombatTarget
// is not present in the living combatants list (e.g. already dead or never in this combat),
// the auto-queue falls back to the first living NPC in combat.
func TestAutoQueuePlayers_FallbackWhenLastTargetDead(t *testing.T) {
	const roomID = "aq-room-4"
	h, npcMgr, sessMgr := makeAutoQueueHandler(t)

	spawnAutoQueueNPC(t, npcMgr, roomID, "Goblin")
	// LastCombatTarget is "NonExistentOrc" which is not in the combat at all.
	addAutoQueuePlayer(t, sessMgr, "player-aq-4", "Hero", roomID, "attack", "NonExistentOrc")

	cbt := startTestCombat(t, h, "player-aq-4", roomID, "Goblin")

	h.combatMu.Lock()
	_ = cbt.StartRound(3)
	q := cbt.ActionQueues["player-aq-4"]
	if q == nil {
		h.combatMu.Unlock()
		t.Fatal("no action queue for player-aq-4")
	}
	h.autoQueuePlayersLocked(cbt)
	actions := q.QueuedActions()
	h.combatMu.Unlock()

	if len(actions) == 0 {
		t.Fatal("expected at least one queued action")
	}
	// Should fall back to Goblin (the living NPC in combat).
	if actions[0].Target != "Goblin" {
		t.Errorf("expected fallback target 'Goblin', got %q", actions[0].Target)
	}
}

// TestAutoQueuePlayers_PassAction verifies that a player with DefaultCombatAction="pass"
// gets a pass action queued (no target required).
func TestAutoQueuePlayers_PassAction(t *testing.T) {
	const roomID = "aq-room-5"
	h, npcMgr, sessMgr := makeAutoQueueHandler(t)

	spawnAutoQueueNPC(t, npcMgr, roomID, "Goblin")
	addAutoQueuePlayer(t, sessMgr, "player-aq-5", "Hero", roomID, "pass", "")

	cbt := startTestCombat(t, h, "player-aq-5", roomID, "Goblin")

	// Reset round.
	h.combatMu.Lock()
	_ = cbt.StartRound(3)
	q := cbt.ActionQueues["player-aq-5"]
	if q == nil {
		h.combatMu.Unlock()
		t.Fatal("no action queue for player-aq-5")
	}
	h.autoQueuePlayersLocked(cbt)
	actions := q.QueuedActions()
	h.combatMu.Unlock()

	if len(actions) == 0 {
		t.Fatal("expected at least one queued action")
	}
	if actions[0].Type != combat.ActionPass {
		t.Errorf("expected ActionPass, got %v", actions[0].Type)
	}
	// ActionPass sets remaining=0, so queue should be submitted.
	if !q.IsSubmitted() {
		t.Error("expected pass action to submit the queue")
	}
}

// TestAutoQueuePlayers_Property is a property-based test verifying that
// autoQueuePlayersLocked never produces a double-queue for an already-submitted player.
func TestAutoQueuePlayers_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		const roomID = "aq-prop-room"
		logger := zap.NewNop()
		src := dice.NewCryptoSource()
		roller := dice.NewLoggedRoller(src, logger)
		engine := combat.NewEngine()
		npcMgr := npc.NewManager()
		sessMgr := session.NewManager()
		broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
		h := NewCombatHandler(engine, npcMgr, sessMgr, roller, broadcastFn, 10*time.Second, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil)

		tmpl := &npc.Template{
			ID: "prop-goblin", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
		}
		_, err := npcMgr.Spawn(tmpl, roomID)
		if err != nil {
			rt.Fatalf("spawn: %v", err)
		}

		defaultAction := rapid.SampledFrom([]string{"attack", "pass", "strike"}).Draw(rt, "defaultAction")
		alreadyQueued := rapid.Bool().Draw(rt, "alreadyQueued")

		_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-player", Username: "u", CharName: "Hero",
			CharacterID: 1, RoomID: roomID, CurrentHP: 10, MaxHP: 10,
			Abilities: character.AbilityScores{}, Role: "player",
			DefaultCombatAction: defaultAction,
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}

		_, err = h.Attack("prop-player", "Goblin")
		if err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		h.cancelTimer(roomID)

		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		if !ok {
			h.combatMu.Unlock()
			rt.Fatal("no combat")
		}

		if !alreadyQueued {
			_ = cbt.StartRound(3) // reset so player has no actions
		}

		q := cbt.ActionQueues["prop-player"]
		beforeCount := len(q.QueuedActions())

		h.autoQueuePlayersLocked(cbt)

		afterCount := len(q.QueuedActions())
		h.combatMu.Unlock()

		if alreadyQueued {
			// Player already had a queued action: auto-queue must not add more.
			if afterCount != beforeCount {
				rt.Errorf("player with existing actions: count changed: before=%d after=%d", beforeCount, afterCount)
			}
		} else {
			// Player had no actions: auto-queue must add exactly one action.
			if afterCount != 1 {
				rt.Errorf("unsubmitted player should have exactly 1 action after autoQueuePlayersLocked, got %d", afterCount)
			}
		}
	})
}
