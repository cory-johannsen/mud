package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

// mockScriptCaller is a ScriptCaller whose CallHook always returns lua.LTrue,
// making every Lua precondition pass unconditionally.
//
// Postcondition: CallHook always returns (lua.LTrue, nil).
type mockScriptCaller struct{}

func (m *mockScriptCaller) CallHook(_ string, _ string, _ ...lua.LValue) (lua.LValue, error) {
	return lua.LTrue, nil
}

// makeHTNDomain constructs a minimal ai.Domain whose single "behave" method has
// an empty precondition and decomposes to the "do_pass" operator.
//
// Postcondition: Returns a non-nil Domain that always plans exactly one "pass"
// action; domain.Validate() returns nil.
func makeHTNDomain(domainID string) *ai.Domain {
	return &ai.Domain{
		ID:          domainID,
		Description: "test domain",
		Tasks: []*ai.Task{
			{ID: "behave", Description: "root task"},
		},
		Methods: []*ai.Method{
			{
				TaskID:       "behave",
				ID:           "m_pass",
				Precondition: "", // empty precondition = always applicable
				Subtasks:     []string{"do_pass"},
			},
		},
		Operators: []*ai.Operator{
			{ID: "do_pass", Action: "pass", Target: ""},
		},
	}
}

// makeHTNCombatHandler constructs a CombatHandler with a non-nil ai.Registry
// that contains the provided domain.  All other optional dependencies are nil.
//
// Postcondition: Returns a non-nil CombatHandler whose aiRegistry is non-nil.
func makeHTNCombatHandler(t *testing.T, broadcastFn func(string, []*gamev1.CombatEvent), aiReg *ai.Registry) *CombatHandler {
	t.Helper()
	_ = zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	return NewCombatHandler(
		engine,
		npcMgr,
		sessMgr,
		roller,
		broadcastFn,
		testRoundDuration,
		makeTestConditionRegistry(),
		nil, // worldMgr — not required for HTN path (zoneID will be "")
		nil, // scriptMgr
		nil, // invRegistry
		aiReg,
		nil, // respawnMgr
		nil, // floorMgr
	)
}

// spawnHTNTestNPC creates and registers a live NPC instance with the given
// AIDomain in roomID.
//
// Postcondition: Returns a non-nil Instance with AIDomain == domainID.
func spawnHTNTestNPC(t *testing.T, npcMgr *npc.Manager, roomID, domainID string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:         "htn-goblin",
		Name:       "HTNGoblin",
		Level:      1,
		MaxHP:      20,
		AC:         13,
		Perception: 2,
		AIDomain:   domainID,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnHTNTestNPC: %v", err)
	}
	return inst
}

// TestCombatHandler_HTN_AutoQueueNPCsLocked_UsesHTNPath verifies that
// autoQueueNPCsLocked routes through applyPlanLocked (the HTN path) rather than
// legacyAutoQueueLocked when the NPC has a registered AI domain.
//
// The always-pass HTN domain produces ActionPass; the legacy fallback produces
// ActionAttack.  The test confirms that the NPC's ActionQueue contains
// ActionPass and no ActionAttack entries.
//
// Precondition: CombatHandler constructed with a non-nil ai.Registry containing
// a domain that always plans "pass".
// Postcondition: NPC ActionQueue contains ActionPass and no ActionAttack.
func TestCombatHandler_HTN_AutoQueueNPCsLocked_UsesHTNPath(t *testing.T) {
	const domainID = "test-htn-domain"
	const roomID = "room-htn-1"

	// Build the always-pass HTN registry.
	domain := makeHTNDomain(domainID)
	if err := domain.Validate(); err != nil {
		t.Fatalf("makeHTNDomain produced invalid domain: %v", err)
	}
	aiReg := ai.NewRegistry()
	if err := aiReg.Register(domain, &mockScriptCaller{}, ""); err != nil {
		t.Fatalf("ai.Registry.Register: %v", err)
	}

	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeHTNCombatHandler(t, broadcastFn, aiReg)

	// Spawn an NPC whose AIDomain matches the registered domain.
	inst := spawnHTNTestNPC(t, h.npcMgr, roomID, domainID)
	addTestPlayer(t, h.sessions, "player-htn-1", roomID)

	// Attack triggers startCombatLocked then autoQueueNPCsLocked.
	_, err := h.Attack("player-htn-1", inst.Name())
	if err != nil {
		t.Fatalf("Attack to start combat: %v", err)
	}
	defer h.cancelTimer(roomID)

	// Inspect the NPC's ActionQueue while holding combatMu.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		h.combatMu.Unlock()
		t.Fatal("expected active combat after Attack; got none")
	}
	aq, queueFound := cbt.ActionQueues[inst.ID]
	h.combatMu.Unlock()

	if !queueFound {
		t.Fatalf("no ActionQueue found for NPC %q; expected HTN path to queue an action", inst.ID)
	}

	queued := aq.QueuedActions()
	if len(queued) == 0 {
		t.Fatal("expected at least one queued action for the NPC; got none")
	}

	// Confirm the legacy fallback was NOT used: no ActionAttack should be present.
	for i, qa := range queued {
		if qa.Type == combat.ActionAttack {
			t.Errorf("queued action[%d] is ActionAttack; expected ActionPass (HTN path not taken)", i)
		}
	}

	// Confirm the HTN "pass" operator was applied.
	foundPass := false
	for _, qa := range queued {
		if qa.Type == combat.ActionPass {
			foundPass = true
			break
		}
	}
	if !foundPass {
		t.Errorf("expected ActionPass in NPC queue after HTN planning; got %v", queued)
	}
}
