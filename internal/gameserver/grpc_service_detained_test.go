package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap/zaptest"
)

// makeDetainedConditionRegistry returns a condition registry that includes the
// detained condition with all three enforcement flags set.
func makeDetainedConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "detained", Name: "Detained",
		Description:      "You are restrained and cannot act.",
		DurationType:     "permanent",
		PreventMovement:  true,
		PreventCommands:  true,
		PreventTargeting: true,
	})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	return reg
}

// newDetainedSvc builds a GameServiceServer with a condition registry that
// includes the detained condition.
func newDetainedSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeDetainedConditionRegistry()
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
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

// applyDetainedCondition applies the detained condition to the given session.
func applyDetainedCondition(t *testing.T, sess *session.PlayerSession, reg *condition.Registry) {
	t.Helper()
	def, ok := reg.Get("detained")
	require.True(t, ok, "detained condition must be registered")
	sess.Conditions = condition.NewActiveSet()
	require.NoError(t, sess.Conditions.Apply(sess.UID, def, 1, -1))
}

// TestDetained_BlocksMovement verifies that a player with the detained condition
// cannot move to another room.
func TestDetained_BlocksMovement(t *testing.T) {
	svc, sessMgr := newDetainedSvc(t)
	condReg := makeDetainedConditionRegistry()

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_detained_move",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	applyDetainedCondition(t, sess, condReg)

	evt, err := svc.handleMove("u_detained_move", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected ErrorEvent for detained player, got: %T", evt.Payload)
	assert.Contains(t, errEvt.Message, "detained")

	// Player must still be in room_a.
	sess2, ok := sessMgr.GetPlayer("u_detained_move")
	require.True(t, ok)
	assert.Equal(t, "room_a", sess2.RoomID, "detained player must not have moved")
}

// TestDetained_BlocksCommands verifies that a player with the detained condition
// cannot issue action commands (e.g. Attack).
func TestDetained_BlocksCommands(t *testing.T) {
	svc, sessMgr := newDetainedSvc(t)
	condReg := makeDetainedConditionRegistry()

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_detained_cmd",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	applyDetainedCondition(t, sess, condReg)

	// Use dispatch with an Attack command — a command that a free player could
	// normally issue. Detained should block it before combat logic runs.
	msg := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Attack{
			Attack: &gamev1.AttackRequest{Target: "Goblin"},
		},
	}
	resp, err := svc.dispatch("u_detained_cmd", msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The response must be a message event containing "detained".
	msgEvt := resp.GetMessage()
	require.NotNil(t, msgEvt, "expected MessageEvent for detained command block, got: %T", resp.Payload)
	assert.Contains(t, msgEvt.Content, "detained")
}

// TestDetained_LookIsNotBlocked verifies that look is exempt from the
// PreventCommands block so a detained player can still inspect their surroundings.
func TestDetained_LookIsNotBlocked(t *testing.T) {
	svc, sessMgr := newDetainedSvc(t)
	condReg := makeDetainedConditionRegistry()

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_detained_look",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	applyDetainedCondition(t, sess, condReg)

	msg := &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}},
	}
	resp, err := svc.dispatch("u_detained_look", msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Must return a RoomView, not an error.
	assert.NotNil(t, resp.GetRoomView(), "look must succeed for detained player")
}

// TestDetained_VisibleInRoomLook verifies that a detained player appears annotated
// in the room view of a second player looking at the same room (REQ-WC-11).
func TestDetained_VisibleInRoomLook(t *testing.T) {
	svc, sessMgr := newDetainedSvc(t)
	condReg := makeDetainedConditionRegistry()

	// Add the detained prisoner.
	prisoner, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_detained_vis_prisoner",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	applyDetainedCondition(t, prisoner, condReg)

	// Add the observer.
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_detained_vis_observer",
		Username:  "Observer",
		CharName:  "Observer",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Observer looks at the room.
	resp, err := svc.handleLook("u_detained_vis_observer")
	require.NoError(t, err)
	require.NotNil(t, resp)

	rv := resp.GetRoomView()
	require.NotNil(t, rv)

	// The detained player must appear annotated in the players list.
	found := false
	for _, p := range rv.Players {
		if p == "Prisoner (detained)" {
			found = true
			break
		}
	}
	assert.True(t, found, "detained player must appear as 'Prisoner (detained)' in room view; got players: %v", rv.Players)
}

// TestProperty_DetainedAlwaysBlocksMove verifies that whenever the detained
// condition is active, handleMove always returns an error event and the player
// never changes rooms.
func TestProperty_DetainedAlwaysBlocksMove(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newDetainedSvc(t)
		uid := rapid.StringMatching(`u_dprop_[a-z]{6}`).Draw(rt, "uid")
		condReg := makeDetainedConditionRegistry()

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "Prop",
			CharName:  "Prop",
			RoomID:    "room_a",
			CurrentHP: 10,
			MaxHP:     10,
			Abilities: character.AbilityScores{},
			Role:      "player",
		})
		require.NoError(rt, err)
		applyDetainedCondition(t, sess, condReg)

		evt, err := svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		errEvt := evt.GetError()
		assert.NotNil(rt, errEvt, "detained player must always receive an error event")

		sess2, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		assert.Equal(rt, "room_a", sess2.RoomID, "detained player must never move rooms")
	})
}
