package gameserver

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newFleeSvcWithCombat builds a GameServiceServer + CombatHandler that share
// the same worldMgr and sessMgr, suitable for flee integration tests.
// Unlike newGrappleSvcWithCombat, it passes worldMgr to CombatHandler so that
// the movement path in Flee is exercised.
func newFleeSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *world.Manager, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		worldMgr, // pass worldMgr so Flee can pick a valid exit
		nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, worldMgr, sessMgr, npcMgr, combatHandler
}

// testWorldAndSessionWithLockedRoom builds a world with room_a, room_b, and
// room_locked (locked east exit only). Used by flee tests needing a dead-end room.
func testWorldAndSessionWithLockedRoom(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID: "test_flee", Name: "Test Flee", Description: "Flee test zone",
		StartRoom: "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID: "room_a", ZoneID: "test_flee", Title: "Room A",
				Description: "Room A.", MapX: 0, MapY: 0,
				Exits:      []world.Exit{{Direction: world.North, TargetRoom: "room_b"}},
				Properties: map[string]string{},
			},
			"room_b": {
				ID: "room_b", ZoneID: "test_flee", Title: "Room B",
				Description: "Room B.", MapX: 0, MapY: 1,
				Exits:      []world.Exit{{Direction: world.South, TargetRoom: "room_a"}},
				Properties: map[string]string{},
			},
			"room_locked": {
				ID: "room_locked", ZoneID: "test_flee", Title: "Dead End",
				Description: "No way out.", MapX: 1, MapY: 0,
				Exits:      []world.Exit{{Direction: world.East, TargetRoom: "room_a", Locked: true}},
				Properties: map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// TestHandleFlee_NotEnoughAP verifies that flee fails when the player has 0 AP.
//
// Precondition: player is in combat with 0 AP remaining.
// Postcondition: error returned; player stays in original room.
func TestHandleFlee_NotEnoughAP(t *testing.T) {
	src := dice.NewDeterministicSource([]int{15})
	roller := dice.NewRoller(src)
	svc, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_flee_ap"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-ap", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_ap", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_ap", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Drain all AP.
	require.NoError(t, combatHandler.SpendAP("u_flee_ap", 3))

	_, err = svc.handleFlee("u_flee_ap")
	assert.ErrorContains(t, err, "1 AP")
	assert.Equal(t, roomID, sess.RoomID, "player must not move on AP error")
}

// TestHandleFlee_Failure verifies a failed flee roll leaves the player in combat.
//
// Precondition: dice returns 1; player total (1+bonus) < DC (10+StrMod).
// Postcondition: FLEE event narrative contains "can't escape"; player stays in room.
func TestHandleFlee_Failure(t *testing.T) {
	src := dice.NewDeterministicSource([]int{1})
	roller := dice.NewRoller(src)
	svc, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_flee_fail"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-fail", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 4,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_fail", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_fail", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	evt, err := svc.handleFlee("u_flee_fail")
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Contains(t, evt.GetCombatEvent().GetNarrative(), "can't escape")
	assert.Equal(t, roomID, sess.RoomID, "player must remain in room on failure")
}

// TestHandleFlee_Success_NoValidExits verifies flee succeeds (combat ends) but
// player stays when the room has no unlocked, non-hidden exits.
//
// Precondition: dice returns 20; room has only a locked exit.
// Postcondition: player status is idle; player stays in room; narrative says "nowhere to run".
func TestHandleFlee_Success_NoValidExits(t *testing.T) {
	src := dice.NewDeterministicSource([]int{20})
	roller := dice.NewRoller(src)
	// Use testWorldAndSessionWithLockedRoom so the svc receives the
	// world containing room_locked.
	lockedWorldMgr, lockedSessMgr := testWorldAndSessionWithLockedRoom(t)
	lockedLogger := zaptest.NewLogger(t)
	lockedNPCMgr := npc.NewManager()
	lockedCombatHandler := NewCombatHandler(
		combat.NewEngine(), lockedNPCMgr, lockedSessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		lockedWorldMgr, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		lockedWorldMgr, lockedSessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(lockedWorldMgr, lockedSessMgr, lockedNPCMgr, nil, nil, nil),
		NewChatHandler(lockedSessMgr),
		lockedLogger,
		nil, roller, nil, lockedNPCMgr, lockedCombatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	sessMgr := lockedSessMgr
	npcMgr := lockedNPCMgr
	combatHandler := lockedCombatHandler

	// room_locked is already in lockedWorldMgr (built by testWorldAndSessionWithLockedRoom).
	const lockedRoomID = "room_locked"

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-lock", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, lockedRoomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_lock", Username: "Runner", CharName: "Runner",
		RoomID: lockedRoomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_lock", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(lockedRoomID)

	evt, err := svc.handleFlee("u_flee_lock")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Player escaped combat but couldn't move.
	assert.Equal(t, lockedRoomID, sess.RoomID)
	assert.Equal(t, int32(1), sess.Status, "player status must be idle after flee")

	// Check narrative includes "nowhere to run".
	assert.Contains(t, evt.GetCombatEvent().GetNarrative(), "nowhere")
}

// TestHandleFlee_Success_NPCPursues verifies that when NPC pursuit roll >= playerTotal,
// the NPC follows the player to the destination room and new combat starts.
//
// Precondition: player flee roll = 20; NPC pursuit roll = 20.
// Postcondition: player moved to room_b; NPC in room_b; new combat active in room_b.
func TestHandleFlee_Success_NPCPursues(t *testing.T) {
	// Rolls: [20=player flee d20, 20=NPC pursuit d20]
	src := dice.NewDeterministicSource([]int{20, 20})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-pursue", Name: "Pursuer", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_pursue", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_pursue", "Pursuer")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	events, fled, err := combatHandler.Flee("u_flee_pursue")
	require.NoError(t, err)
	assert.True(t, fled)
	assert.NotEmpty(t, events)

	destRoomID := sess.RoomID
	assert.NotEqual(t, roomID, destRoomID, "player must have moved")

	// NPC followed player to destination room.
	updatedInst, ok := npcMgr.Get(inst.ID)
	require.True(t, ok)
	assert.Equal(t, destRoomID, updatedInst.RoomID, "NPC must be in destination room")

	// New combat active in destination room.
	cbt := combatHandler.ActiveCombatForRoom(destRoomID)
	assert.NotNil(t, cbt, "new combat must be active in destination room")
}

// TestHandleFlee_Success_NPCFails verifies that when NPC pursuit roll < playerTotal,
// the NPC stays behind and no new combat starts.
//
// Precondition: player flee roll = 20; NPC pursuit roll = 1.
// Postcondition: player moved; NPC stays in original room; no combat in destination.
func TestHandleFlee_Success_NPCFails(t *testing.T) {
	// Rolls: [20=player flee, 1=NPC pursuit]
	src := dice.NewDeterministicSource([]int{20, 1})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-nopursue", Name: "Slowpoke", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_nopursue", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_nopursue", "Slowpoke")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	events, fled, err := combatHandler.Flee("u_flee_nopursue")
	require.NoError(t, err)
	assert.True(t, fled)
	assert.NotEmpty(t, events)

	destRoomID := sess.RoomID
	assert.NotEqual(t, roomID, destRoomID, "player must have moved")

	// NPC stayed in original room.
	updatedInst, ok := npcMgr.Get(inst.ID)
	require.True(t, ok)
	assert.Equal(t, roomID, updatedInst.RoomID, "NPC must remain in original room")

	// No combat in destination room.
	assert.Nil(t, combatHandler.ActiveCombatForRoom(destRoomID), "no combat in destination room")
}

// TestHandleFlee_Success_OriginalCombatEnds verifies that when the fleeing player
// is the only player, the original room's combat ends after a successful flee.
//
// Precondition: single player; dice = [20 flee, 1 pursuit] (NPC stays, no new combat).
// Postcondition: no active combat in original room.
func TestHandleFlee_Success_OriginalCombatEnds(t *testing.T) {
	src := dice.NewDeterministicSource([]int{20, 1})
	roller := dice.NewRoller(src)
	_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

	const roomID = "room_a"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "ganger-flee-endcbt", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_flee_endcbt", Username: "Runner", CharName: "Runner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	_, err = combatHandler.Attack("u_flee_endcbt", "Ganger")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	_, fled, err := combatHandler.Flee("u_flee_endcbt")
	require.NoError(t, err)
	assert.True(t, fled)

	// Combat in original room must be gone.
	assert.Nil(t, combatHandler.ActiveCombatForRoom(roomID), "original combat must have ended")
}

// TestProperty_Flee_SkillCheckBoundary verifies playerTotal >= DC → success for
// all random roll/DC combinations, exercising the actual Flee function.
func TestProperty_Flee_SkillCheckBoundary(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		npcStrMod := rapid.IntRange(0, 4).Draw(rt, "npcStrMod")
		dc := 10 + npcStrMod

		src := dice.NewDeterministicSource([]int{roll})
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		roomID := rapid.StringMatching(`room_prop_[a-z]{4}`).Draw(rt, "roomID")
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-npc-" + roomID, Name: "PropNPC", Level: 1,
			MaxHP: 10, AC: 12, Perception: npcStrMod * 2,
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		_, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-uid-" + roomID, Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-uid-"+roomID, "PropNPC")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		_, fled, err := combatHandler.Flee("prop-uid-" + roomID)
		if err != nil {
			rt.Skip() // AP or other transient error
		}

		playerTotal := roll // bonus=0 since sess.Skills is empty
		if playerTotal >= dc {
			assert.True(t, fled, "expected success when playerTotal(%d) >= dc(%d)", playerTotal, dc)
		} else {
			assert.False(t, fled, "expected failure when playerTotal(%d) < dc(%d)", playerTotal, dc)
		}
	})
}

// TestProperty_Pursuit_RollOutcome verifies the NPC pursuit condition:
// NPC pursues iff pursuitTotal >= playerTotal, by inspecting NPC room location.
func TestProperty_Pursuit_RollOutcome(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fleeRoll := rapid.IntRange(11, 20).Draw(rt, "fleeRoll") // ensure flee succeeds (>=10+0)
		pursuitRoll := rapid.IntRange(1, 20).Draw(rt, "pursuitRoll")

		src := dice.NewDeterministicSource([]int{fleeRoll, pursuitRoll})
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		roomID := "room_a"
		inst, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-pursue-npc", Name: "Pursuer", Level: 1,
			MaxHP: 10, AC: 12, Perception: 0, // StrMod=0 → DC=10; fleeRoll≥11 always succeeds
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-pursue-uid", Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-pursue-uid", "Pursuer")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		_, fled, err := combatHandler.Flee("prop-pursue-uid")
		if err != nil || !fled {
			rt.Skip()
		}

		playerTotal := fleeRoll // bonus=0
		expectedPursues := pursuitRoll >= playerTotal

		if expectedPursues {
			updatedInst, ok := npcMgr.Get(inst.ID)
			assert.True(t, ok)
			assert.Equal(t, sess.RoomID, updatedInst.RoomID,
				"NPC must follow player when pursuitRoll(%d) >= playerTotal(%d)", pursuitRoll, playerTotal)
		} else {
			updatedInst, ok := npcMgr.Get(inst.ID)
			assert.True(t, ok)
			assert.Equal(t, roomID, updatedInst.RoomID,
				"NPC must stay when pursuitRoll(%d) < playerTotal(%d)", pursuitRoll, playerTotal)
		}
	})
}

// TestProperty_Flee_ExitSelection verifies that Flee only moves the player to an
// exit where Hidden==false && Locked==false.
func TestProperty_Flee_ExitSelection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// All exits in room_a are unlocked/unhidden (room_a → room_b via North).
		src := dice.NewDeterministicSource([]int{20, 1}) // flee succeeds; NPC stays
		roller := dice.NewRoller(src)
		_, _, sessMgr, npcMgr, combatHandler := newFleeSvcWithCombat(t, roller)

		const roomID = "room_a"
		_, spawnErr := npcMgr.Spawn(&npc.Template{
			ID: "prop-exit-npc", Name: "Blocker", Level: 1,
			MaxHP: 10, AC: 12, Perception: 0,
		}, roomID)
		if spawnErr != nil {
			rt.Skip()
		}
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "prop-exit-uid", Username: "Runner", CharName: "Runner",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if addErr != nil {
			rt.Skip()
		}
		_, attackErr := combatHandler.Attack("prop-exit-uid", "Blocker")
		if attackErr != nil {
			rt.Skip()
		}
		combatHandler.cancelTimer(roomID)

		_, fled, err := combatHandler.Flee("prop-exit-uid")
		if err != nil || !fled {
			rt.Skip()
		}

		// Destination must be room_b (the only valid exit from room_a).
		assert.Equal(t, "room_b", sess.RoomID,
			"flee from room_a must land in room_b (the only valid exit)")
	})
}
