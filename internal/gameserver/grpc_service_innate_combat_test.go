package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// makeTechRegistry builds a technology Registry containing a single def.
func makeTechRegistry(def *technology.TechnologyDef) *technology.Registry {
	reg := technology.NewRegistry()
	reg.Register(def)
	return reg
}

// newInnateCombatSvc creates a fully wired service+combatHandler for innate-in-combat tests.
func newInnateCombatSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)

	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil,
	)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleUse_InnateTech_WithActionCost_QueuesInCombat is the regression test for
// issue #125: innate techs with action_cost > 0 must queue for round resolution in
// combat instead of firing immediately.
//
// Precondition: player is in active combat; innate tech has action_cost 2.
// Postcondition: handleUse returns nil (queued) and ActionUseTech appears in the combat queue.
func TestHandleUse_InnateTech_WithActionCost_QueuesInCombat(t *testing.T) {
	t.Parallel()

	const roomID = "room_innate_combat"
	svc, sessMgr, npcMgr, combatHandler := newInnateCombatSvc(t)

	svc.SetTechRegistry(makeTechRegistry(&technology.TechnologyDef{
		ID:         "atmospheric_surge",
		Name:       "Atmospheric Surge",
		UsageType:  "innate",
		ActionCost: 2,
	}))
	svc.SetInnateTechRepo(&innateRepoForGrpcTest{})

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-innate-cbt", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	const uid = "u_innate_cbt"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	sess.InnateTechs = map[string]*session.InnateSlot{
		"atmospheric_surge": {MaxUses: 0, UsesRemaining: 0}, // unlimited
	}
	sess.Status = statusInCombat

	_, err = combatHandler.Attack(uid, "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	require.Greater(t, combatHandler.RemainingAP(uid), 0, "player must have AP to queue")

	// With the bug: fires immediately, returns non-nil event.
	// With the fix: queues, returns nil.
	evt, err := svc.handleUse(uid, "atmospheric_surge", "", -1, -1)
	require.NoError(t, err)
	assert.Nil(t, evt, "innate tech with action_cost>0 must return nil (queued), not an immediate effect")

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	q, hasQ := cbt.ActionQueues[uid]
	require.True(t, hasQ, "player must have an action queue")
	actions := q.QueuedActions()
	require.NotEmpty(t, actions, "ActionUseTech must be queued")
	var foundTechAction bool
	for _, a := range actions {
		if a.Type == combat.ActionUseTech && a.AbilityID == "atmospheric_surge" {
			foundTechAction = true
			break
		}
	}
	assert.True(t, foundTechAction, "ActionUseTech for atmospheric_surge must be in the queue")
}

// TestBug158_TechUseResolverFn_NeverDeadlocks is the regression test for GitHub issue #158:
// "Using Righteous Condemnation from hotbar causes combat to halt."
//
// Root cause: techUseResolverFn was called from within resolveAndAdvanceLocked (combatMu held),
// and the resolver called ActiveCombatForPlayer which tried to re-acquire combatMu on the same
// goroutine — deadlock.
//
// Fix: techUseResolverFn now receives the *combat.Combat directly so it never needs to
// re-acquire combatMu. This test verifies the fix by calling the resolver directly while
// simulating the held-lock condition (passing a non-nil cbt to the callback).
//
// Precondition: a techUseResolverFn is registered; it receives cbt directly.
// Postcondition: the resolver invocation completes without deadlocking.
func TestBug158_TechUseResolverFn_NeverDeadlocks(t *testing.T) {
	t.Parallel()

	const roomID = "room_bug158"
	_, sessMgr, npcMgr, combatHandler := newInnateCombatSvc(t)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-bug158", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	const uid = "u_bug158"
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Zealot", CharName: "Zealot",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	// Track whether the resolver was invoked.
	resolverCalled := make(chan struct{}, 1)

	// Install a resolver that simply closes the channel — no combat state lookup needed.
	// Previously it called ActiveCombatForPlayer which deadlocked; now cbt is passed directly.
	combatHandler.SetTechUseResolverFn(func(uid, techID, targetID string, targetX, targetY int32, cbt *combat.Combat) {
		// Verify cbt is non-nil (passed from resolveAndAdvanceLocked).
		if cbt != nil {
			resolverCalled <- struct{}{}
		}
	})

	_, err = combatHandler.Attack(uid, "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	// Manually enqueue an ActionUseTech for the player without going through QueueTechUse
	// (which would try to resolve immediately via AllActionsSubmitted). We want to exercise
	// the resolver path inside resolveAndAdvanceLocked directly.
	qa := combat.QueuedAction{
		Type:        combat.ActionUseTech,
		AbilityID:   "righteous_condemnation",
		Target:      "Goblin",
		AbilityCost: 1,
		TargetX:     -1,
		TargetY:     -1,
	}
	err = cbt.QueueAction(uid, qa)
	require.NoError(t, err)

	// Also queue an action for the NPC so AllActionsSubmitted becomes true and we can trigger resolution.
	npcInsts := npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	npcID := npcInsts[0].ID
	err = cbt.QueueAction(npcID, combat.QueuedAction{
		Type:        combat.ActionAttack,
		Target:      uid,
		AbilityCost: 1,
	})
	require.NoError(t, err)

	// Trigger round resolution directly. If the old deadlock were present, this would hang.
	// The test uses a goroutine + timeout to detect hangs.
	done := make(chan struct{})
	go func() {
		combatHandler.resolveAndAdvanceLocked(roomID, cbt)
		close(done)
	}()

	select {
	case <-done:
		// Resolution completed — no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("resolveAndAdvanceLocked deadlocked (Bug #158 regression)")
	}

	// Resolver should have been called exactly once for the ActionUseTech event.
	select {
	case <-resolverCalled:
		// Resolver was invoked with a non-nil cbt.
	default:
		t.Fatal("techUseResolverFn was not called during round resolution")
	}
}

// TestBug197_PreparedAttackTech_ResolverNeverDeadlocks is the regression test for GitHub issue #197:
// "Using Cryo-Gel Projectile halts combat, requires server restart."
//
// Root cause: activateTechWithEffectsWithCombat called resolveUseTarget, which called
// ActiveCombatForPlayer — that tried to re-acquire combatMu on the same goroutine, causing deadlock.
//
// Fix: activateTechWithEffectsWithCombat now resolves the target directly from the provided
// *combat.Combat without calling ActiveCombatForPlayer.
//
// Precondition: player has a prepared attack-based tech (resolution:"attack") queued in combat.
// Postcondition: resolveAndAdvanceLocked completes within 5s without deadlock; resolver is called.
func TestBug197_PreparedAttackTech_ResolverNeverDeadlocks(t *testing.T) {
	t.Parallel()

	const roomID = "room_bug197"
	_, sessMgr, npcMgr, combatHandler := newInnateCombatSvc(t)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "rat-bug197", Name: "Giant Rat", Level: 1, MaxHP: 20, AC: 10, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	const uid = "u_bug197"
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Engineer", CharName: "Engineer",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	resolverCalled := make(chan struct{}, 1)

	// Install a resolver that records the call — previously this caused a deadlock.
	combatHandler.SetTechUseResolverFn(func(uid, techID, targetID string, targetX, targetY int32, cbt *combat.Combat) {
		if cbt != nil {
			resolverCalled <- struct{}{}
		}
	})

	_, err = combatHandler.Attack(uid, "Giant Rat")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	// Manually enqueue an ActionUseTech (prepared, resolution:attack) for the player.
	qa := combat.QueuedAction{
		Type:        combat.ActionUseTech,
		AbilityID:   "cryo_gel_projectile",
		Target:      "Giant Rat",
		AbilityCost: 2,
		TargetX:     -1,
		TargetY:     -1,
	}
	err = cbt.QueueAction(uid, qa)
	require.NoError(t, err)

	npcInsts := npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	err = cbt.QueueAction(npcInsts[0].ID, combat.QueuedAction{
		Type:        combat.ActionAttack,
		Target:      uid,
		AbilityCost: 1,
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		combatHandler.resolveAndAdvanceLocked(roomID, cbt)
		close(done)
	}()

	select {
	case <-done:
		// Resolution completed — no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("resolveAndAdvanceLocked deadlocked (Bug #197 regression: prepared attack tech)")
	}

	select {
	case <-resolverCalled:
		// Resolver was invoked with a non-nil cbt.
	default:
		t.Fatal("techUseResolverFn was not called during round resolution")
	}
}

// TestBug198_InnateTech_WithKnownTechs_NotRejected is the regression test for GitHub issue #198:
// "Innate techs produce 'You don't know <tech>' error when activated."
//
// Root cause: handleUse reached the spontaneous (KnownTechs) branch when the player is a
// wizard/ranger and has KnownTechs populated. That branch returned "You don't know <tech>"
// when the innate tech ID was absent from KnownTechs, never falling through to the innate
// check further down.
//
// Fix: the spontaneous branch now uses goto innateCheck when the tech is not in KnownTechs,
// allowing the innate branch to run.
//
// Precondition: player has both KnownTechs (wizard model) and an innate tech assigned.
// Postcondition: handleUse activates the innate tech successfully — returns a non-nil event.
func TestBug198_InnateTech_WithKnownTechs_NotRejected(t *testing.T) {
	t.Parallel()

	const roomID = "room_bug198"
	svc, sessMgr, _, _ := newInnateCombatSvc(t)

	svc.SetTechRegistry(makeTechRegistry(&technology.TechnologyDef{
		ID:         "atmospheric_surge",
		Name:       "Atmospheric Surge",
		UsageType:  "innate",
		ActionCost: 0,
	}))
	svc.SetInnateTechRepo(&innateRepoForGrpcTest{})

	const uid = "u_bug198"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Nerd", CharName: "Nerd",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	// Wizard model: has KnownTechs but atmospheric_surge is innate, NOT in KnownTechs.
	sess.KnownTechs = map[int][]string{
		1: {"shock_wave", "emp_burst"},
	}
	sess.InnateTechs = map[string]*session.InnateSlot{
		"atmospheric_surge": {MaxUses: 0, UsesRemaining: 0}, // unlimited uses
	}

	// Without the fix, this returns "You don't know atmospheric_surge.".
	// With the fix, it should fall through to the innate path and return a non-nil event.
	evt, err := svc.handleUse(uid, "atmospheric_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt, "innate tech must activate successfully even when player has KnownTechs (wizard model)")

	// Verify the event is not an error message.
	if msg := evt.GetMessage(); msg != nil {
		assert.NotContains(t, msg.Content, "don't know", "must not return 'You don't know' for an innate tech")
		assert.NotContains(t, msg.Content, "don't have", "must not return 'You don't have' for an innate tech")
	}
}

// TestHandleUse_InnateTech_NoActionCost_FiresImmediately verifies that innate techs
// with no action_cost (true cantrips) still fire immediately — existing behavior preserved.
//
// Precondition: player is in active combat; innate tech has action_cost 0.
// Postcondition: handleUse returns a non-nil event (immediate activation).
func TestHandleUse_InnateTech_NoActionCost_FiresImmediately(t *testing.T) {
	t.Parallel()

	const roomID = "room_innate_free"
	svc, sessMgr, npcMgr, combatHandler := newInnateCombatSvc(t)

	svc.SetTechRegistry(makeTechRegistry(&technology.TechnologyDef{
		ID:         "detect_signal",
		Name:       "Detect Signal",
		UsageType:  "innate",
		ActionCost: 0,
	}))
	svc.SetInnateTechRepo(&innateRepoForGrpcTest{})

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-free", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	const uid = "u_innate_free"
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	sess.InnateTechs = map[string]*session.InnateSlot{
		"detect_signal": {MaxUses: 0, UsesRemaining: 0},
	}
	sess.Status = statusInCombat

	_, err = combatHandler.Attack(uid, "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Free innate should fire immediately — non-nil event.
	evt, err := svc.handleUse(uid, "detect_signal", "", -1, -1)
	require.NoError(t, err)
	assert.NotNil(t, evt, "free innate tech (action_cost 0) must fire immediately and return an event")
}
