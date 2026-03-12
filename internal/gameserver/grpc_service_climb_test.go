package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// initSessionConditions initialises the Conditions field of the session to an empty ActiveSet.
// Required for tests that exercise condition-apply paths, since the field is normally
// populated by the login flow rather than AddPlayer.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.Conditions is a non-nil empty ActiveSet.
func initSessionConditions(sess *session.PlayerSession) {
	sess.Conditions = condition.NewActiveSet()
}

// makeTestConditionRegistryWithProne returns a condition registry containing
// the standard test conditions plus "submerged" and "prone" (prone is already in
// makeTestConditionRegistry, but we explicitly confirm its presence here).
//
// Postcondition: Registry contains at least prone and submerged definitions.
func makeTestConditionRegistryWithProneAndSubmerged() *condition.Registry {
	reg := makeTestConditionRegistry()
	reg.Register(&condition.ConditionDef{ID: "submerged", Name: "Submerged", DurationType: "permanent", MaxStacks: 1})
	return reg
}

// newClimbWorld creates a world suitable for climb tests.
// Room "room_climb" is climbable with an "up" exit to "room_climb_top".
// Room "room_noclimb" is a plain room with no climbable property.
// Room "room_climbable_noexit" has climbable=true but no vertical exits.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a world.Manager and session.Manager containing the above rooms.
func newClimbWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone",
		StartRoom:   "room_climb",
		Rooms: map[string]*world.Room{
			"room_climb": {
				ID:          "room_climb",
				ZoneID:      "test",
				Title:       "Climbable Room",
				Description: "A room with something to climb.",
				Exits: []world.Exit{
					{Direction: world.Up, TargetRoom: "room_climb_top"},
				},
				Properties: map[string]string{
					"climbable": "true",
				},
			},
			"room_climb_top": {
				ID:          "room_climb_top",
				ZoneID:      "test",
				Title:       "The Top",
				Description: "You made it to the top.",
				Exits: []world.Exit{
					{Direction: world.Down, TargetRoom: "room_climb"},
				},
				Properties: map[string]string{},
			},
			"room_noclimb": {
				ID:          "room_noclimb",
				ZoneID:      "test",
				Title:       "Plain Room",
				Description: "Nothing to climb here.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
			"room_climbable_noexit": {
				ID:          "room_climbable_noexit",
				ZoneID:      "test",
				Title:       "Climbable No Exit",
				Description: "Climbable but no vertical exits.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{"climbable": "true"},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// newClimbSvc builds a minimal GameServiceServer using the climb world.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr sharing the climb world.
func newClimbSvc(t *testing.T, roller *dice.Roller, condReg *condition.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := newClimbWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newClimbSvcWithCombat builds a GameServiceServer with a real CombatHandler
// and condRegistry for climb tests that require in-combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler all sharing the same sessMgr.
func newClimbSvcWithCombat(t *testing.T, roller *dice.Roller, condReg *condition.Registry) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := newClimbWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleClimb_RoomNotClimbable verifies that handleClimb returns a "nothing to climb"
// message when the player's room has no climbable property.
//
// Precondition: Player is in "room_noclimb" with no climbable property.
// Postcondition: Message event contains "nothing to climb".
func TestHandleClimb_RoomNotClimbable(t *testing.T) {
	svc, sessMgr := newClimbSvc(t, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_climb_nc",
		Username: "Climber",
		CharName: "Climber",
		RoomID:   "room_noclimb",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleClimb("u_climb_nc", &gamev1.ClimbRequest{})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "nothing to climb")
}

// TestHandleClimb_NoVerticalExit verifies that handleClimb returns an appropriate message
// when the room is climbable but has no up or down exits.
//
// Precondition: Player is in "room_climbable_noexit" which has climbable=true but no vertical exits.
// Postcondition: Message event contains "no clear route".
func TestHandleClimb_NoVerticalExit(t *testing.T) {
	svc, sessMgr := newClimbSvc(t, nil, nil)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_climb_nve",
		Username: "Climber",
		CharName: "Climber",
		RoomID:   "room_climbable_noexit",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleClimb("u_climb_nve", &gamev1.ClimbRequest{})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "no clear route")
}

// TestHandleClimb_CritFailure_InCombat verifies that on a critical failure, the player
// takes falling damage and the prone condition is applied when in combat.
//
// Precondition: Player is in "room_climb" (climbable, up exit) and in combat.
// Dice fixed at val=0: roll=1, bonus=0, total=1; DC=15; 1 < 5 → CritFailure.
// Postcondition: Player HP is decreased; prone condition is set; message contains "fall".
func TestHandleClimb_CritFailure_InCombat(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // Intn(20)=0 → roll=1; total=1 < DC-10=5 → CritFailure
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithProneAndSubmerged()

	svc, sessMgr, npcMgr, combatHandler := newClimbSvcWithCombat(t, roller, condReg)

	const roomID = "room_climb"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-climb-cf-ic", Name: "Bandit", Level: 1, MaxHP: 20, AC: 13, Perception: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_climb_cf_ic",
		Username:  "Climber",
		CharName:  "Climber",
		RoomID:    roomID,
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat
	initSessionConditions(sess)

	_, err = combatHandler.Attack("u_climb_cf_ic", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	hpBefore := sess.CurrentHP

	event, err := svc.handleClimb("u_climb_cf_ic", &gamev1.ClimbRequest{})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit failure")
	assert.Contains(t, msgEvt.Content, "fall")

	// HP must have decreased.
	assert.Less(t, sess.CurrentHP, hpBefore, "player HP must decrease on crit failure")

	// Prone condition must be applied when in combat.
	require.NotNil(t, sess.Conditions, "conditions must be initialised")
	assert.True(t, sess.Conditions.Has("prone"), "player must have prone condition in combat crit failure")
}

// TestHandleClimb_CritFailure_OutOfCombat verifies that on a critical failure out of combat,
// the player takes falling damage but the prone condition is NOT applied.
//
// Precondition: Player is in "room_climb" (climbable, up exit) and NOT in combat.
// Dice fixed at val=0: roll=1, bonus=0, total=1; DC=15; 1 < 5 → CritFailure.
// Postcondition: Player HP is decreased; prone condition is NOT set.
func TestHandleClimb_CritFailure_OutOfCombat(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithProneAndSubmerged()

	svc, sessMgr := newClimbSvc(t, roller, condReg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_climb_cf_ooc",
		Username:  "Climber",
		CharName:  "Climber",
		RoomID:    "room_climb",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	// sess.Status is not statusInCombat (default value).

	hpBefore := sess.CurrentHP

	event, err := svc.handleClimb("u_climb_cf_ooc", &gamev1.ClimbRequest{})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit failure out of combat")
	assert.Contains(t, msgEvt.Content, "fall")

	// HP must have decreased.
	assert.Less(t, sess.CurrentHP, hpBefore, "player HP must decrease on crit failure")

	// Prone condition must NOT be applied when out of combat.
	if sess.Conditions != nil {
		assert.False(t, sess.Conditions.Has("prone"), "player must NOT have prone condition out of combat")
	}
}
