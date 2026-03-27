package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newRefocusTestSvc creates a minimal GameServiceServer for refocus tests,
// following the pattern from grpc_service_camping_tick_test.go.
func newRefocusTestSvc(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	zone := &world.Zone{
		ID:          "refocus_zone",
		Name:        "Refocus Zone",
		Description: "Zone for refocus tests.",
		StartRoom:   "refocus_room",
		DangerLevel: "safe",
		Rooms: map[string]*world.Room{
			"refocus_room": {
				ID:          "refocus_room",
				ZoneID:      "refocus_zone",
				Title:       "Refocus Room",
				Description: "A quiet room for refocusing.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
				DangerLevel: "safe",
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	logger := zaptest.NewLogger(t)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
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

	uid := "refocus-test-uid"
	sess, err2 := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      "refocus_room",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err2)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	return svc, uid
}

func TestHandleRefocus_BlockedInCombat(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = statusInCombat

	err := svc.handleRefocus(uid)
	require.NoError(t, err)
	assert.False(t, sess.RefocusingActive, "should not start refocus in combat")
}

func TestHandleRefocus_BlockedAtMaxFocusPoints(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = 3
	sess.MaxFocusPoints = 3

	err := svc.handleRefocus(uid)
	require.NoError(t, err)
	assert.False(t, sess.RefocusingActive, "should not start refocus when already at max")
}

func TestHandleRefocus_StartsTimer(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = 0
	sess.MaxFocusPoints = 2

	err := svc.handleRefocus(uid)
	require.NoError(t, err)
	assert.True(t, sess.RefocusingActive)
	assert.False(t, sess.RefocusingStartTime.IsZero())
}

func TestHandleRefocus_AlreadyRefocusing(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.RefocusingActive = true
	sess.RefocusingStartTime = time.Now().Add(-30 * time.Second)

	err := svc.handleRefocus(uid)
	require.NoError(t, err)
	// Still active, not reset
	assert.True(t, sess.RefocusingActive)
}

func TestCheckRefocusStatus_CompletesAfterOneMinute(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = 1
	sess.MaxFocusPoints = 3
	sess.RefocusingActive = true
	sess.RefocusingStartTime = time.Now().Add(-90 * time.Second) // past 1-minute threshold

	svc.checkRefocusStatus(uid)

	assert.False(t, sess.RefocusingActive, "refocus should complete")
	assert.Equal(t, 2, sess.FocusPoints, "should restore 1 focus point")
}

func TestCheckRefocusStatus_NotYetComplete(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = 0
	sess.MaxFocusPoints = 2
	sess.RefocusingActive = true
	sess.RefocusingStartTime = time.Now().Add(-20 * time.Second) // only 20s elapsed

	svc.checkRefocusStatus(uid)

	assert.True(t, sess.RefocusingActive, "should still be refocusing")
	assert.Equal(t, 0, sess.FocusPoints, "no FP restored yet")
}

func TestCheckRefocusStatus_CapsAtMaxFocusPoints(t *testing.T) {
	svc, uid := newRefocusTestSvc(t)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.FocusPoints = 2
	sess.MaxFocusPoints = 2 // already at max (shouldn't happen but guard it)
	sess.RefocusingActive = true
	sess.RefocusingStartTime = time.Now().Add(-90 * time.Second)

	svc.checkRefocusStatus(uid)

	assert.False(t, sess.RefocusingActive)
	assert.Equal(t, 2, sess.FocusPoints, "should not exceed MaxFocusPoints")
}
