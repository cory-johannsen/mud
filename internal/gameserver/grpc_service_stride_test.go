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

// TestHandleStride_TowardIncreasesPosition verifies that striding toward increases the
// player's Position by 25, moving them closer to the NPC (which starts at Position=25).
//
// Precondition: player in combat at Position=0 (distance=25 from NPC at 25); direction=="toward".
// Postcondition: player.Position becomes 25; distance to NPC becomes 0.
func TestHandleStride_TowardIncreasesPosition(t *testing.T) {
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

	// Player starts at Position=0, NPC at Position=25.
	playerCbt := cbt.GetCombatant("u_str_tr")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0

	event, err := svc.handleStride("u_str_tr", &gamev1.StrideRequest{Direction: "toward"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "toward")

	assert.Equal(t, 25, playerCbt.Position, "player.Position should be 25 after striding toward")
}

// TestHandleStride_AwayDecreasesPosition verifies that striding away decreases the
// player's Position by 25, floored at 0.
//
// Precondition: player at Position=25; direction=="away".
// Postcondition: player.Position becomes 0.
func TestHandleStride_AwayDecreasesPosition(t *testing.T) {
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
	playerCbt.Position = 25

	event, err := svc.handleStride("u_str_ai", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "away")

	assert.Equal(t, 0, playerCbt.Position, "player.Position should be 0 after striding away from 25")
}

// TestHandleStride_AwayFlooredAtZero verifies that striding away from Position=0
// does not go below zero.
//
// Precondition: player at Position=0; direction=="away".
// Postcondition: player.Position remains 0.
func TestHandleStride_AwayFlooredAtZero(t *testing.T) {
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
	playerCbt.Position = 0

	event, err := svc.handleStride("u_str_cm", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 0, playerCbt.Position, "player.Position should remain 0 when already at floor")
}

// TestHandleStride_DefaultDirectionIsToward verifies that an empty direction defaults to toward.
//
// Precondition: player at Position=0; direction=="".
// Postcondition: player.Position becomes 25.
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
	playerCbt.Position = 0

	event, err := svc.handleStride("u_str_cx", &gamev1.StrideRequest{Direction: ""})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")

	assert.Equal(t, 25, playerCbt.Position, "empty direction should default to toward, increasing Position by 25")
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

// TestNPCAutoStride_MeleeNPC_ClosesDistance verifies that a melee NPC at distance > 5
// has ActionStride{Direction:"toward"} prepended before ActionAttack in its queue.
//
// Precondition: NPC has no WeaponID (unarmed/melee); player at Position=0, NPC at Position=25.
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
	// Player at Position=0, NPC at Position=25 → distance=25 > 5 → NPC should stride.
	playerCbt := cbt.GetCombatant("u_npc_melee_stride")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	npcCbt.Position = 25
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
// ranged weapon does NOT queue ActionStride even when distance > 5.
//
// Precondition: NPC WeaponID references a WeaponDef with RangeIncrement > 0; distance == 25.
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
	playerCbt := cbt.GetCombatant("u_npc_ranged_stride")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 0
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	npcCbt.Position = 25
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
// adjacent NPC (within 5ft before the stride), the response message includes a
// reactive strike narrative from that NPC.
//
// Precondition: player in combat at Position=5 (adjacent to NPC at Position=0); direction=="away".
// Postcondition: response message contains "reactive strike".
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

	// Place player at Position=30, NPC at Position=25 (dist=5 ≤ 5 → adjacent).
	// handleStride "away" decreases Position by 25: 30-25=5. New dist = |5-25| = 20 > 5 → RS fires.
	playerCbt := cbt.GetCombatant("u_str_rs")
	require.NotNil(t, playerCbt)
	playerCbt.Position = 30

	// Place NPC at Position=25 (adjacent: dist = |30-25| = 5 ≤ 5).
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.Position = 25
			break
		}
	}

	// Stride away: player moves from 30 to 5. Old dist=5, new dist=20 → RS fires.
	event, err := svc.handleStride("u_str_rs", &gamev1.StrideRequest{Direction: "away"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "reactive strike", "response must contain reactive strike narrative")
}
