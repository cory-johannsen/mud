package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newStrideSvc builds a minimal GameServiceServer for handleStride tests without combat.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr.
func newStrideSvc(t *testing.T, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
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
	return svc, sessMgr
}

// newStrideSvcWithCombat builds a GameServiceServer, session.Manager, npc.Manager, and
// CombatHandler sharing the same session.Manager, suitable for tests that need real
// in-progress combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newStrideSvcWithCombat(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
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

// TestHandleStride_NotInCombat verifies that handleStride returns an error event
// when the player is not in combat.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: error event containing "only available in combat".
func TestHandleStride_NotInCombat(t *testing.T) {
	svc, sessMgr := newStrideSvc(t, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_str_nc",
		Username: "Fighter",
		CharName: "Fighter",
		RoomID:   "room_str_nc",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleStride("u_str_nc", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	errEvt := event.GetError()
	require.NotNil(t, errEvt, "expected an error event")
	assert.Contains(t, errEvt.Message, "only available in combat")
}

// TestHandleStride_TowardDecreasesDist verifies that striding "toward" moves the player
// their full speed (25 ft = 5 cells) toward the NPC.
//
// Precondition: player at GridX=0, GridY=0; NPC at GridX=5, GridY=9 (default spawn); direction=="toward".
// Postcondition: player.GridX==5 and player.GridY==5 (5 diagonal steps of delta(1,1)).
func TestHandleStride_TowardDecreasesDist(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_tr"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-tr", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_tr", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_tr", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	// Default spawn: player (0,0), NPC (5,9). Chebyshev dist = max(5,9)*5 = 45ft.
	playerCbt := cbt.GetCombatant("u_str_tr")
	require.NotNil(t, playerCbt)

	event, err := svc.handleStride("u_str_tr", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	// towardDelta(0,0,5,9) = (1,1); 5 steps (25 ft) → player at (5,5).
	assert.Equal(t, 5, playerCbt.GridX, "player.GridX should be 5 after striding toward (25 ft)")
	assert.Equal(t, 5, playerCbt.GridY, "player.GridY should be 5 after striding toward (25 ft)")
}

// TestHandleStride_TowardFromAnyPosition_DecreasesDist verifies that stride "toward"
// moves the player their full speed toward the NPC regardless of starting position.
//
// Precondition: player at GridX=5, GridY=5; NPC at GridX=0, GridY=0; direction=="toward".
// Postcondition: player.GridX==0 and player.GridY==0 (5 diagonal steps of delta(-1,-1)).
func TestHandleStride_TowardFromAnyPosition_DecreasesDist(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_ai"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-ai", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_ai", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_ai", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_str_ai")
	require.NotNil(t, playerCbt)
	// Place player at (5,5); override NPC to (0,0).
	playerCbt.GridX = 5
	playerCbt.GridY = 5
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.GridX = 0
			c.GridY = 0
			break
		}
	}

	event, err := svc.handleStride("u_str_ai", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	// towardDelta(5,5,0,0) = (-1,-1); 5 steps (25 ft) → player at (0,0).
	assert.Equal(t, 0, playerCbt.GridX, "player.GridX should be 0 after striding toward enemy (25 ft)")
	assert.Equal(t, 0, playerCbt.GridY, "player.GridY should be 0 after striding toward enemy (25 ft)")
}

// TestHandleStride_AwayAtBoundary_GridClampApplied verifies that stride "away" at the
// grid boundary does not move the player off the grid (coordinates clamped to 0).
//
// Precondition: player at GridX=0, GridY=0 (grid corner); NPC at GridX=5, GridY=9; direction=="away".
// Postcondition: player remains at GridX=0, GridY=0 (clamped from (-1,-1)).
func TestHandleStride_AwayAtBoundary_GridClampApplied(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_cm"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-cm", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_cm", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_cm", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_str_cm")
	require.NotNil(t, playerCbt)
	// Player already at (0,0) by default. NPC at (5,9) by default.
	// away = -towardDelta(0,0,5,9) = -(1,1) = (-1,-1) → clamped to (0,0).

	event, err := svc.handleStride("u_str_cm", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 0, playerCbt.GridX, "player.GridX should be clamped at 0 when striding away from grid boundary")
	assert.Equal(t, 0, playerCbt.GridY, "player.GridY should be clamped at 0 when striding away from grid boundary")
}

// TestHandleStride_DefaultDirectionIsToward verifies that an empty direction defaults to toward.
//
// Precondition: player at GridX=0, GridY=0; NPC at GridX=5, GridY=9; direction=="".
// Postcondition: player.GridX==5 and player.GridY==5 (same as explicit "toward", 25 ft).
func TestHandleStride_DefaultDirectionIsToward(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_cx"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-cx", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_cx", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_cx", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	playerCbt := cbt.GetCombatant("u_str_cx")
	require.NotNil(t, playerCbt)
	// Default spawn: player (0,0), NPC (5,9).

	event, err := svc.handleStride("u_str_cx", &gamev1.StrideRequest{Direction: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	// towardDelta(0,0,5,9) = (1,1); 5 steps (25 ft) → same as explicit "toward".
	assert.Equal(t, 5, playerCbt.GridX, "empty direction should default to toward, GridX becomes 5")
	assert.Equal(t, 5, playerCbt.GridY, "empty direction should default to toward, GridY becomes 5")
}

// newStrideSvcWithCombatAndRegistry builds a GameServiceServer and associated helpers with
// the given inventory.Registry wired into the CombatHandler, so that NPC weapon
// lookups (e.g. ranged vs melee detection) function correctly.
//
// Precondition: t must be non-nil; reg may be nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newStrideSvcWithCombatAndRegistry(t *testing.T, reg *inventory.Registry) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, reg, nil, nil, nil, nil,
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

// TestNPCAutoStride_MeleeNPC_ClosesDistance verifies that a melee NPC at distance > 5ft
// has ActionStride{Direction:"toward"} prepended before ActionAttack in its queue.
//
// Precondition: NPC has no WeaponID (unarmed/melee); player at GridX=0,GridY=0; NPC at GridX=2,GridY=0
// (Chebyshev dist=10ft > 5ft).
// Postcondition: NPC action queue contains ActionStride as first action.
func TestNPCAutoStride_MeleeNPC_ClosesDistance(t *testing.T) {
	_, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombatAndRegistry(t, nil)

	const roomID = "room_npc_melee_stride"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-melee-stride", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_npc_melee_stride", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_npc_melee_stride", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	combatHandler.combatMu.Lock()
	cbt, ok := combatHandler.engine.GetCombat(roomID)
	require.True(t, ok)
	// Player at (0,0), NPC at (2,0): Chebyshev dist = max(2,0)*5 = 10ft > 5ft → NPC should stride.
	playerCbt := cbt.GetCombatant("u_npc_melee_stride")
	require.NotNil(t, playerCbt)
	playerCbt.GridX = 0
	playerCbt.GridY = 0
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	npcCbt.GridX = 2
	npcCbt.GridY = 0
	// Reset queues so autoQueueNPCsLocked populates from a clean state.
	cbt.StartRound(3)
	combatHandler.autoQueueNPCsLocked(cbt)
	q := cbt.ActionQueues[inst.ID]
	combatHandler.combatMu.Unlock()

	require.NotNil(t, q, "expected an action queue for the NPC")
	actions := q.QueuedActions()
	require.GreaterOrEqual(t, len(actions), 2, "expected at least stride + attack in queue")
	assert.Equal(t, combat.ActionStride, actions[0].Type, "first action must be ActionStride")
	assert.Equal(t, "toward", actions[0].Direction, "stride direction must be toward")
	assert.Equal(t, combat.ActionAttack, actions[1].Type, "second action must be ActionAttack")
}

// TestNPCAutoStride_RangedNPC_DoesNotStride verifies that an NPC equipped with a
// ranged weapon does NOT queue ActionStride even when distance > 5ft.
//
// Precondition: NPC WeaponID references a WeaponDef with RangeIncrement > 0; player at (0,0), NPC at (2,0)
// (Chebyshev dist=10ft > 5ft).
// Postcondition: NPC action queue starts directly with ActionAttack (no ActionStride).
func TestNPCAutoStride_RangedNPC_DoesNotStride(t *testing.T) {
	reg := inventory.NewRegistry()
	bowDef := &inventory.WeaponDef{
		ID:             "shortbow",
		Name:           "Shortbow",
		RangeIncrement: 60,
	}
	require.NoError(t, reg.RegisterWeapon(bowDef))

	_, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombatAndRegistry(t, reg)

	const roomID = "room_npc_ranged_stride"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "archer-ranged-stride", Name: "Archer", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)
	// Assign the ranged weapon to the NPC instance directly.
	inst.WeaponID = "shortbow"

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_npc_ranged_stride", Username: "Ranger", CharName: "Ranger",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_npc_ranged_stride", "Archer")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	combatHandler.combatMu.Lock()
	cbt, ok := combatHandler.engine.GetCombat(roomID)
	require.True(t, ok)
	// Player at (0,0), NPC at (2,0): dist=10ft > 5ft. Ranged NPC should NOT stride.
	playerCbt := cbt.GetCombatant("u_npc_ranged_stride")
	require.NotNil(t, playerCbt)
	playerCbt.GridX = 0
	playerCbt.GridY = 0
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	npcCbt.GridX = 2
	npcCbt.GridY = 0
	cbt.StartRound(3)
	combatHandler.autoQueueNPCsLocked(cbt)
	q := cbt.ActionQueues[inst.ID]
	combatHandler.combatMu.Unlock()

	require.NotNil(t, q, "expected an action queue for the NPC")
	actions := q.QueuedActions()
	require.NotEmpty(t, actions, "expected at least one action in queue")
	assert.Equal(t, combat.ActionAttack, actions[0].Type, "first action must be ActionAttack (no stride for ranged NPC)")
}

// TestHandleStride_ReactiveStrike verifies that when a player strides away from an
// adjacent NPC (Chebyshev dist ≤ 5ft before stride, > 5ft after), the response message
// includes a reactive strike narrative from that NPC.
//
// Precondition: player at GridX=1,GridY=0; NPC at GridX=0,GridY=0 (dist=5ft, adjacent); direction=="away".
// Postcondition: player moves to GridX=2; response message contains "reactive strike".
func TestHandleStride_ReactiveStrike(t *testing.T) {
	svc, sessMgr, npcMgr, combatHandler := newStrideSvcWithCombat(t)

	const roomID = "room_str_rs"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-str-rs", Name: "Goblin", Level: 1, MaxHP: 20, AC: 10, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_str_rs", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 30, MaxHP: 30, Role: "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_str_rs", "Goblin")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)

	// Place player at (1,0), NPC at (0,0): Chebyshev dist = max(1,0)*5 = 5ft (adjacent).
	// Stride "away": towardDelta(1,0,0,0) = (-1,0); away = (1,0).
	// Player moves from (1,0) to (2,0). New dist = max(2,0)*5 = 10ft > 5ft → RS fires.
	playerCbt := cbt.GetCombatant("u_str_rs")
	require.NotNil(t, playerCbt)
	playerCbt.GridX = 1
	playerCbt.GridY = 0

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.GridX = 0
			c.GridY = 0
			break
		}
	}

	event, err := svc.handleStride("u_str_rs", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "reactive strike", "response must contain reactive strike narrative")
}
