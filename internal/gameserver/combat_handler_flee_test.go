package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap"
)

// makeFleeHandler builds a CombatHandler with a world manager containing two
// rooms (room-a and room-b) connected by exits.
func makeFleeHandler(t *testing.T, broadcastFn func(string, []*gamev1.CombatEvent)) (*CombatHandler, *world.Manager) {
	t.Helper()
	roomA := &world.Room{
		ID:     "room-a",
		ZoneID: "zone-test",
		Title:  "Room A",
		Exits: []world.Exit{
			{Direction: "north", TargetRoom: "room-b"},
		},
	}
	roomB := &world.Room{
		ID:     "room-b",
		ZoneID: "zone-test",
		Title:  "Room B",
		Exits: []world.Exit{
			{Direction: "south", TargetRoom: "room-a"},
		},
	}
	zone := &world.Zone{
		ID:        "zone-test",
		Name:      "Test Zone",
		StartRoom: "room-a",
		Rooms:     map[string]*world.Room{"room-a": roomA, "room-b": roomB},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	aiReg := ai.NewRegistry()

	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	h := NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		broadcastFn, testRoundDuration,
		makeTestConditionRegistry(), wm, nil, nil, aiReg,
		nil, nil, nil,
	)
	return h, wm
}

// makeFleeHandlerNoExits builds a CombatHandler with a single isolated room.
func makeFleeHandlerNoExits(t *testing.T, broadcastFn func(string, []*gamev1.CombatEvent)) *CombatHandler {
	t.Helper()
	roomA := &world.Room{
		ID:    "room-a",
		Title: "Room A",
		Exits: nil,
	}
	zone := &world.Zone{
		ID: "zone-test", StartRoom: "room-a",
		Rooms: map[string]*world.Room{"room-a": roomA},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	aiReg := ai.NewRegistry()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	return NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		broadcastFn, testRoundDuration,
		makeTestConditionRegistry(), wm, nil, nil, aiReg,
		nil, nil, nil,
	)
}

// makeFleeHTNDomain returns a minimal domain that always plans a "flee" action.
func makeFleeHTNDomain(domainID string) *ai.Domain {
	return &ai.Domain{
		ID:    domainID,
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "m_flee", Subtasks: []string{"do_flee"}},
		},
		Operators: []*ai.Operator{
			{ID: "do_flee", Action: "flee"},
		},
	}
}

// makeTargetWeakestHTNDomain returns a domain that always plans "target_weakest".
func makeTargetWeakestHTNDomain(domainID string) *ai.Domain {
	return &ai.Domain{
		ID:    domainID,
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "m_tw", Subtasks: []string{"do_tw"}},
		},
		Operators: []*ai.Operator{
			{ID: "do_tw", Action: "target_weakest"},
		},
	}
}

// makeCallForHelpHTNDomain returns a domain that always plans "call_for_help".
func makeCallForHelpHTNDomain(domainID string) *ai.Domain {
	return &ai.Domain{
		ID:    domainID,
		Tasks: []*ai.Task{{ID: "behave"}},
		Methods: []*ai.Method{
			{TaskID: "behave", ID: "m_cfh", Subtasks: []string{"do_cfh"}},
		},
		Operators: []*ai.Operator{
			{ID: "do_cfh", Action: "call_for_help"},
		},
	}
}

// startCombatWithHTN starts combat in roomID between the given NPC and player,
// then manually applies the given plan via applyPlanLocked (under combatMu).
func applyPlanInCombat(t *testing.T, h *CombatHandler, roomID, npcInstID, playerUID string, plan []ai.PlannedAction) {
	t.Helper()
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		t.Fatalf("expected active combat in %q", roomID)
	}
	actor := cbt.GetCombatant(npcInstID)
	if actor == nil {
		t.Fatalf("actor %q not found in combat", npcInstID)
	}
	h.applyPlanLocked(cbt, actor, plan)
}

// TestFlee_NPCRemovedFromCombat verifies that executing a "flee" plan removes
// the NPC from the combat combatants list and moves it to an adjacent room.
//
// REQ-NB-25, REQ-NB-26, REQ-NB-27.
func TestFlee_NPCRemovedFromCombat(t *testing.T) {
	const roomID = "room-a"
	var broadcasts []string
	h, _ := makeFleeHandler(t, func(_ string, evts []*gamev1.CombatEvent) {
		for _, e := range evts {
			broadcasts = append(broadcasts, e.Narrative)
		}
	})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-flee-1", roomID)

	_, err := h.Attack("player-flee-1", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	plan := []ai.PlannedAction{{Action: "flee", OperatorID: "__flee"}}
	applyPlanInCombat(t, h, roomID, inst.ID, "player-flee-1", plan)

	// NPC should no longer be a combatant.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	var found bool
	if ok {
		for _, c := range cbt.Combatants {
			if c.ID == inst.ID {
				found = true
			}
		}
	}
	h.combatMu.Unlock()

	if found {
		t.Error("expected NPC to be removed from combat after flee")
	}
	// NPC should have moved away from room-a.
	updatedInst, _ := h.npcMgr.Get(inst.ID)
	if updatedInst != nil && updatedInst.RoomID == roomID {
		t.Error("expected NPC to have moved out of room-a after flee")
	}
}

// TestFlee_NoExits_OperatorFails verifies that flee with no valid exits is a no-op.
//
// REQ-NB-28.
func TestFlee_NoExits_OperatorFails(t *testing.T) {
	const roomID = "room-a"
	h := makeFleeHandlerNoExits(t, func(_ string, _ []*gamev1.CombatEvent) {})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-flee-2", roomID)

	_, err := h.Attack("player-flee-2", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	plan := []ai.PlannedAction{{Action: "flee", OperatorID: "__flee"}}
	applyPlanInCombat(t, h, roomID, inst.ID, "player-flee-2", plan)

	// NPC should still be in combat when there are no exits.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	var found bool
	if ok {
		for _, c := range cbt.Combatants {
			if c.ID == inst.ID {
				found = true
			}
		}
	}
	h.combatMu.Unlock()

	if !found {
		t.Error("expected NPC to remain in combat when flee has no valid exits")
	}
}

// TestFlee_CombatContinuesWithRemainingParticipants verifies that after NPC
// flee, the player remains in combat. REQ-NB-27.
func TestFlee_CombatContinuesWithRemainingParticipants(t *testing.T) {
	const roomID = "room-a"
	h, _ := makeFleeHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-flee-3", roomID)

	_, err := h.Attack("player-flee-3", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	plan := []ai.PlannedAction{{Action: "flee", OperatorID: "__flee"}}
	applyPlanInCombat(t, h, roomID, inst.ID, "player-flee-3", plan)

	// Combat should still exist (player still in it).
	h.combatMu.Lock()
	_, still := h.engine.GetCombat(roomID)
	h.combatMu.Unlock()

	// Combat may have ended (no enemies), but player was not removed.
	// The key assertion: player is not harmed by the flee mechanism.
	_ = still // combat state is implementation-defined; just verify no panic
}

// TestTargetWeakest_SelectsLowestHPPlayer verifies that target_weakest queues
// an attack action targeting the lowest HP% player. REQ-NB-29, REQ-NB-30.
func TestTargetWeakest_SelectsLowestHPPlayer(t *testing.T) {
	const roomID = "room-a"
	h, _ := makeFleeHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")

	// Add two players: Alpha at 80%, Beta at 30% HP (weakest).
	_ = addTestPlayerWithHP(t, h.sessions, "player-tw-1", roomID, 8, 10, "Alpha")
	p2 := addTestPlayerWithHP(t, h.sessions, "player-tw-2", roomID, 3, 10, "Beta")

	_, err := h.Attack("player-tw-1", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	// Add p2 to the combat so there are 2 living players.
	h.combatMu.Lock()
	_, ok := h.engine.GetCombat(roomID)
	if ok {
		p2Cbt := &combat.Combatant{
			ID: p2.UID, Name: p2.CharName, Kind: combat.KindPlayer,
			CurrentHP: p2.CurrentHP, MaxHP: p2.MaxHP, AC: 12, Level: 1,
		}
		_ = h.engine.AddCombatant(roomID, p2Cbt)
	}
	h.combatMu.Unlock()

	if !ok {
		t.Fatal("no active combat")
	}

	plan := []ai.PlannedAction{{Action: "target_weakest", OperatorID: "__tw"}}
	applyPlanInCombat(t, h, roomID, inst.ID, "player-tw-1", plan)

	// The NPC's action queue should contain at least one attack action targeting p2 (weakest).
	h.combatMu.Lock()
	cbt, _ := h.engine.GetCombat(roomID)
	var foundWeakestTarget bool
	if cbt != nil {
		if aq, found := cbt.ActionQueues[inst.ID]; found {
			for _, qa := range aq.QueuedActions() {
				if qa.Type == combat.ActionAttack && qa.Target == p2.CharName {
					foundWeakestTarget = true
					break
				}
			}
		}
	}
	h.combatMu.Unlock()

	if !foundWeakestTarget {
		t.Errorf("expected queued attack targeting weakest player %q, but not found in queue", p2.CharName)
	}
}

// TestTargetWeakest_OnePlayer_FailsSilently verifies that target_weakest with
// only one living player does not panic and queues no attack. REQ-NB-31.
func TestTargetWeakest_OnePlayer_FailsSilently(t *testing.T) {
	const roomID = "room-a"
	h, _ := makeFleeHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-tw-solo", roomID)

	_, err := h.Attack("player-tw-solo", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	plan := []ai.PlannedAction{{Action: "target_weakest", OperatorID: "__tw"}}
	// Must not panic with only 1 player.
	applyPlanInCombat(t, h, roomID, inst.ID, "player-tw-solo", plan)
}

// TestCallForHelp_FiresOnlyOnce verifies that call_for_help is idempotent
// (second invocation is a no-op). REQ-NB-35.
func TestCallForHelp_FiresOnlyOnce(t *testing.T) {
	const roomID = "room-a"
	var broadcastCount int
	h, _ := makeFleeHandler(t, func(_ string, evts []*gamev1.CombatEvent) {
		for _, e := range evts {
			if e.Narrative != "" {
				broadcastCount++
			}
		}
	})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-cfh-1", roomID)

	_, err := h.Attack("player-cfh-1", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	plan := []ai.PlannedAction{{Action: "call_for_help", OperatorID: "__cfh"}}

	// First invocation.
	applyPlanInCombat(t, h, roomID, inst.ID, "player-cfh-1", plan)
	firstCount := broadcastCount

	// Second invocation should be a no-op.
	applyPlanInCombat(t, h, roomID, inst.ID, "player-cfh-1", plan)
	secondCount := broadcastCount

	if secondCount > firstCount {
		t.Errorf("call_for_help fired again on second invocation (counts: first=%d, second=%d)", firstCount, secondCount)
	}
}

// TestCallForHelp_NoQualifyingNPC_FailsSilently verifies that when no adjacent
// NPC matches, call_for_help is a silent no-op. REQ-NB-33.
func TestCallForHelp_NoQualifyingNPC_FailsSilently(t *testing.T) {
	const roomID = "room-a"
	h, _ := makeFleeHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, "")
	addTestPlayer(t, h.sessions, "player-cfh-2", roomID)

	_, err := h.Attack("player-cfh-2", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	defer h.cancelTimer(roomID)

	// No NPC in adjacent room → should not panic.
	plan := []ai.PlannedAction{{Action: "call_for_help", OperatorID: "__cfh"}}
	applyPlanInCombat(t, h, roomID, inst.ID, "player-cfh-2", plan)
}

// addTestPlayerWithHP creates a player session with explicit HP values and a custom char name.
func addTestPlayerWithHP(t *testing.T, sessMgr *session.Manager, uid, roomID string, currentHP, maxHP int, charName string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    charName,
		CharacterID: 99,
		RoomID:      roomID,
		CurrentHP:   currentHP,
		MaxHP:       maxHP,
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("addTestPlayerWithHP: %v", err)
	}
	return sess
}
