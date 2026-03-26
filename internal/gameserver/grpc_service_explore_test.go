package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// newExploreSvc builds a minimal GameServiceServer for applyExploreModeOnEntry tests.
func newExploreSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, condReg *condition.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
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

// testRoom builds a minimal room for explore tests.
func testRoom(id string) *world.Room {
	return &world.Room{
		ID:         id,
		ZoneID:     "test",
		Title:      "Test Room",
		Properties: map[string]string{},
	}
}

// TestExploreMode_LayLow_NoNPCs verifies that Lay Low with no NPCs produces no messages and no conditions (REQ-EXP-10).
//
// Precondition: room has no live NPCs.
// Postcondition: returns nil; no conditions applied.
func TestExploreMode_LayLow_NoNPCs(t *testing.T) {
	svc, sessMgr := newExploreSvc(t, nil, nil, nil)
	sess := addTestPlayer(t, sessMgr, "u_ll_nonpc", "room_a")
	sess.ExploreMode = session.ExploreModeLayLow
	sess.Conditions = condition.NewActiveSet()

	room := testRoom("room_a")
	msgs := svc.ApplyExploreModeOnEntry("u_ll_nonpc", sess, room)

	assert.Nil(t, msgs)
	assert.False(t, sess.Conditions.Has("hidden"))
}

// TestExploreMode_LayLow_CritSuccess_AppliesHiddenAndUndetected verifies crit success applies both conditions (REQ-EXP-8).
//
// Precondition: dice forced to roll=20 (val=19); NPC with awareness 0 → DC=10; roll+0 >= DC+10 = crit success.
// Postcondition: hidden and undetected conditions applied; message returned.
func TestExploreMode_LayLow_CritSuccess_AppliesHiddenAndUndetected(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20
	roller := dice.NewLoggedRoller(src, logger)

	npcMgr := npc.NewManager()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-ll-cs", Name: "Guard", Level: 1, MaxHP: 10, AC: 10, Awareness: 0,
	}, "room_ll_cs")
	require.NoError(t, err)

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{ID: "hidden", Name: "Hidden", DurationType: "permanent", MaxStacks: 0})
	condReg.Register(&condition.ConditionDef{ID: "undetected", Name: "Undetected", DurationType: "permanent", MaxStacks: 0})

	// Need a worldMgr that has the room.
	zone := &world.Zone{
		ID: "test",
		Rooms: map[string]*world.Room{
			"room_ll_cs": {ID: "room_ll_cs", ZoneID: "test", Title: "R", Properties: map[string]string{}},
		},
		StartRoom: "room_ll_cs",
	}
	worldMgr, err2 := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err2)
	sessMgr := session.NewManager()

	logger2 := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger2,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	sess, err3 := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_ll_cs", Username: "testuser", CharName: "Hero", RoomID: "room_ll_cs", Role: "player",
	})
	require.NoError(t, err3)
	sess.ExploreMode = session.ExploreModeLayLow
	sess.Conditions = condition.NewActiveSet()

	room := zone.Rooms["room_ll_cs"]
	msgs := svc.ApplyExploreModeOnEntry("u_ll_cs", sess, room)

	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "unnoticed")
	assert.True(t, sess.Conditions.Has("hidden"), "hidden condition must be applied on crit success")
	assert.True(t, sess.Conditions.Has("undetected"), "undetected condition must be applied on crit success")
}

// TestExploreMode_LayLow_CritFailure_SetsBlockedRoom verifies crit failure sets LayLowBlockedRoom (REQ-EXP-8a).
//
// Precondition: dice forced to roll=1 (val=0); NPC awareness=20 → DC=30; roll+0 ≤ DC-10 = crit failure.
// Postcondition: LayLowBlockedRoom set to room ID.
func TestExploreMode_LayLow_CritFailure_SetsBlockedRoom(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1
	roller := dice.NewLoggedRoller(src, logger)

	npcMgr := npc.NewManager()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "boss-ll-cf", Name: "Boss", Level: 5, MaxHP: 50, AC: 18, Awareness: 20,
	}, "room_ll_cf")
	require.NoError(t, err)

	zone := &world.Zone{
		ID: "test",
		Rooms: map[string]*world.Room{
			"room_ll_cf": {ID: "room_ll_cf", ZoneID: "test", Title: "R", Properties: map[string]string{}},
		},
		StartRoom: "room_ll_cf",
	}
	worldMgr, err2 := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err2)
	sessMgr := session.NewManager()

	logger2 := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger2,
		nil, roller, nil, npcMgr, nil, nil,
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

	sess, err3 := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_ll_cf", Username: "testuser", CharName: "Hero", RoomID: "room_ll_cf", Role: "player",
	})
	require.NoError(t, err3)
	sess.ExploreMode = session.ExploreModeLayLow
	sess.Conditions = condition.NewActiveSet()

	room := zone.Rooms["room_ll_cf"]
	msgs := svc.ApplyExploreModeOnEntry("u_ll_cf", sess, room)

	require.NotEmpty(t, msgs)
	assert.Equal(t, "room_ll_cf", sess.LayLowBlockedRoom)
}

// TestExploreMode_ActiveSensors_Success_ReturnsMessage verifies active sensors success returns a message (REQ-EXP-15).
//
// Precondition: dice forced roll=20; room has equipment.
// Postcondition: message returned listing detected items.
func TestExploreMode_ActiveSensors_Success_ReturnsMessage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newExploreSvc(t, roller, nil, nil)
	sess := addTestPlayer(t, sessMgr, "u_as_succ", "room_a")
	sess.ExploreMode = session.ExploreModeActiveSensors
	sess.Conditions = condition.NewActiveSet()

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Title:  "Test Room",
		Properties: map[string]string{},
		Equipment: []world.RoomEquipmentConfig{
			{ItemID: "laser_rifle"},
		},
	}

	msgs := svc.ApplyExploreModeOnEntry("u_as_succ", sess, room)
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "laser_rifle")
}

// TestExploreMode_ActiveSensors_Failure_ReturnsNothing verifies active sensors failure returns nil (REQ-EXP-17).
//
// Precondition: dice forced roll=1; DC is 16 (sketchy default).
// Postcondition: nil returned.
func TestExploreMode_ActiveSensors_Failure_ReturnsNothing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // roll=1
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newExploreSvc(t, roller, nil, nil)
	sess := addTestPlayer(t, sessMgr, "u_as_fail", "room_a")
	sess.ExploreMode = session.ExploreModeActiveSensors
	sess.Conditions = condition.NewActiveSet()

	room := testRoom("room_a")
	msgs := svc.ApplyExploreModeOnEntry("u_as_fail", sess, room)
	assert.Nil(t, msgs)
}

// TestExploreMode_CaseIt_Success_ReturnsMessage verifies case it success with hidden exit returns a message (REQ-EXP-20).
//
// Precondition: dice forced roll=20; room has hidden exit.
// Postcondition: message returned mentioning hidden exit.
func TestExploreMode_CaseIt_Success_ReturnsMessage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // roll=20
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newExploreSvc(t, roller, nil, nil)
	sess := addTestPlayer(t, sessMgr, "u_ci_succ", "room_a")
	sess.ExploreMode = session.ExploreModeCaseIt
	sess.Conditions = condition.NewActiveSet()

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Title:  "Test Room",
		Properties: map[string]string{},
		Exits: []world.Exit{
			{Direction: world.North, TargetRoom: "room_b", Hidden: true},
		},
	}

	msgs := svc.ApplyExploreModeOnEntry("u_ci_succ", sess, room)
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "hidden exit")
}

// TestExploreMode_PokeAround_FactionContext_ReturnsFactOrNil verifies poke around selects faction skill and handles result (REQ-EXP-34).
//
// Precondition: room context is "faction"; dice fixed to roll=20.
// Postcondition: returns lore fact when available, nil when not.
func TestExploreMode_PokeAround_FactionContext_ReturnsFactOrNil(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 14} // roll=15, should succeed at DC=17 with no bonuses → failure (15 < 17)
	roller := dice.NewLoggedRoller(src, logger)

	svc, sessMgr := newExploreSvc(t, roller, nil, nil)
	sess := addTestPlayer(t, sessMgr, "u_pa_fact", "room_a")
	sess.ExploreMode = session.ExploreModePokeAround
	sess.Conditions = condition.NewActiveSet()

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Title:  "Test Room",
		Properties: map[string]string{
			"context":    "faction",
			"lore_facts": "The Crimson Hand controls this block.",
		},
	}

	// With roll=15, DC=17, no ability bonus, no rank → total=15 < 17 → failure → nil
	msgs := svc.ApplyExploreModeOnEntry("u_pa_fact", sess, room)
	assert.Nil(t, msgs)

	// Now force a success: roll=20
	src2 := &fixedDiceSource{val: 19}
	roller2 := dice.NewLoggedRoller(src2, logger)
	svc2, sessMgr2 := newExploreSvc(t, roller2, nil, nil)
	sess2 := addTestPlayer(t, sessMgr2, "u_pa_fact2", "room_a")
	sess2.ExploreMode = session.ExploreModePokeAround
	sess2.Conditions = condition.NewActiveSet()

	msgs2 := svc2.ApplyExploreModeOnEntry("u_pa_fact2", sess2, room)
	require.NotEmpty(t, msgs2)
	assert.Contains(t, msgs2[0], "Crimson Hand")
}
