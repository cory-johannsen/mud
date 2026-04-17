package gameserver

import (
	"testing"

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
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
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
