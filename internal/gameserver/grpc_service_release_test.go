package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap/zaptest"
)

// newReleaseSvc builds a GameServiceServer with a condition registry that includes
// detained, and a world zone with the given dangerLevel.
func newReleaseSvc(t *testing.T, dangerLevel string) (*GameServiceServer, *session.Manager, *condition.Registry) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_release",
		Name:        "Release Test Zone",
		Description: "Zone for release tests.",
		DangerLevel: dangerLevel,
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test_release",
				Title:       "Room A",
				Description: "Test room.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeDetainedConditionRegistry()

	// Use a deterministic dice roller so tests can control outcomes.
	// Value 0: Intn(20) returns 0, so roll = 1 (always low).
	diceRoller := dice.NewRoller(dice.NewDeterministicSource([]int{0, 0, 0, 0, 0, 0, 0, 0}))

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, diceRoller, nil, npcMgr, nil, nil,
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
	return svc, sessMgr, condReg
}

// TestRelease_TargetNotInRoom verifies that releasing a player not in the same room
// returns "<player> is not here."
func TestRelease_TargetNotInRoom(t *testing.T) {
	svc, sessMgr, _ := newReleaseSvc(t, "sketchy")

	// Add the releaser.
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_rel1",
		Username:  "Releaser",
		CharName:  "Releaser",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// No target added — they are not in the room.
	evt, err := svc.handleRelease("u_rel_rel1", &gamev1.ReleaseRequest{PlayerName: "Ghost"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected MessageEvent, got: %T", evt.Payload)
	assert.Contains(t, msg.Content, "not here")
}

// TestRelease_TargetNotDetained verifies that releasing a player who is in the same
// room but does NOT have the detained condition returns "<player> is not detained."
func TestRelease_TargetNotDetained(t *testing.T) {
	svc, sessMgr, _ := newReleaseSvc(t, "sketchy")

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_rel2",
		Username:  "Releaser",
		CharName:  "Releaser",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_target2",
		Username:  "FreeMan",
		CharName:  "FreeMan",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	// No detained condition applied.

	evt, err := svc.handleRelease("u_rel_rel2", &gamev1.ReleaseRequest{PlayerName: "FreeMan"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected MessageEvent, got: %T", evt.Payload)
	assert.Contains(t, msg.Content, "not detained")
}

// TestRelease_SkillCheckSuccess verifies that a high roll removes the detained
// condition from the target but does NOT change their WantedLevel (REQ-WC-15).
func TestRelease_SkillCheckSuccess(t *testing.T) {
	// Use a high fixed value: Intn(20) returns 18, so roll = 19 (always beats DC 12).
	diceRoller := dice.NewRoller(dice.NewDeterministicSource([]int{18, 18, 18, 18}))
	zone := &world.Zone{
		ID:          "test_rel_succ",
		Name:        "Release Success Zone",
		Description: "Zone.",
		DangerLevel: "safe", // DC 12 — low enough that 19 always passes
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test_rel_succ",
				Title:       "Room A",
				Description: "Test.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeDetainedConditionRegistry()

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, diceRoller, nil, npcMgr, nil, nil,
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

	// Add releaser.
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_rel3",
		Username:  "Releaser",
		CharName:  "Releaser",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Add detained target with a known WantedLevel.
	target, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_target3",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	target.WantedLevel["test_rel_succ"] = 2
	applyDetainedCondition(t, target, condReg)

	evt, err := svc.handleRelease("u_rel_rel3", &gamev1.ReleaseRequest{PlayerName: "Prisoner"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected MessageEvent for success, got: %T", evt.Payload)
	assert.Contains(t, msg.Content, "freed")

	// Detained condition must be removed.
	assert.False(t, target.Conditions.Has("detained"), "detained condition must be removed after successful release")

	// REQ-WC-15: WantedLevel must be unchanged.
	assert.Equal(t, 2, target.WantedLevel["test_rel_succ"], "WantedLevel must not change on successful release")
}

// TestRelease_SkillCheckFailure verifies that a low roll leaves the detained
// condition in place and returns a failure message.
func TestRelease_SkillCheckFailure(t *testing.T) {
	// Use value 0: Intn(20) returns 0, so roll = 1; can never beat DC 16.
	diceRoller := dice.NewRoller(dice.NewDeterministicSource([]int{0, 0, 0, 0}))
	zone := &world.Zone{
		ID:          "test_rel_fail",
		Name:        "Release Fail Zone",
		Description: "Zone.",
		DangerLevel: "sketchy", // DC 16 — roll 1+0=1 always fails
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test_rel_fail",
				Title:       "Room A",
				Description: "Test.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeDetainedConditionRegistry()

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, diceRoller, nil, npcMgr, nil, nil,
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

	// Add releaser.
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_rel4",
		Username:  "Releaser",
		CharName:  "Releaser",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Add detained target.
	target, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_rel_target4",
		Username:  "Prisoner",
		CharName:  "Prisoner",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)
	applyDetainedCondition(t, target, condReg)

	evt, err := svc.handleRelease("u_rel_rel4", &gamev1.ReleaseRequest{PlayerName: "Prisoner"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected MessageEvent for failure, got: %T", evt.Payload)
	assert.Contains(t, msg.Content, "fail")

	// Detained condition must still be present.
	assert.True(t, target.Conditions.Has("detained"), "detained condition must remain after failed release")
}
