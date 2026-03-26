package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newCampingTickSvc creates a GameServiceServer with a "dangerous" room and exits
// for camping tick tests.
func newCampingTickSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "tick_zone",
		Name:        "Tick Zone",
		Description: "Zone for camping tick tests.",
		StartRoom:   "tick_room",
		DangerLevel: "dangerous",
		Rooms: map[string]*world.Room{
			"tick_room": {
				ID:          "tick_room",
				ZoneID:      "tick_zone",
				Title:       "Tick Room",
				Description: "A room for tick tests.",
				Exits: []world.Exit{
					{Direction: "north", TargetRoom: "tick_room_north"},
				},
				Properties:  map[string]string{},
				DangerLevel: "dangerous",
			},
			"tick_room_north": {
				ID:          "tick_room_north",
				ZoneID:      "tick_zone",
				Title:       "North Room",
				Description: "The room to the north.",
				Exits: []world.Exit{
					{Direction: "south", TargetRoom: "tick_room"},
				},
				Properties:  map[string]string{},
				DangerLevel: "dangerous",
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
		nil, // cmdReg
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
	return svc, sessMgr, npcMgr
}

// addCampingPlayer adds a player in tick_room with CampingActive set and given HP values.
func addCampingPlayer(t *testing.T, sessMgr *session.Manager, uid string, currentHP, maxHP int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      "tick_room",
		CurrentHP:   currentHP,
		MaxHP:       maxHP,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	return sess
}

// TestCheckCampingStatus_CompletesOnTime verifies that checkCampingStatus applies a full
// long rest when the camping duration has elapsed (REQ-REST-18).
//
// Precondition: sess.CampingActive == true; elapsed >= CampingDuration.
// Postcondition: sess.CampingActive == false; sess.CurrentHP == sess.MaxHP.
func TestCheckCampingStatus_CompletesOnTime(t *testing.T) {
	svc, sessMgr, _ := newCampingTickSvc(t)

	uid := "camper-complete"
	sess := addCampingPlayer(t, sessMgr, uid, 5, 20)

	sess.CampingActive = true
	sess.CampingDuration = 5 * time.Minute
	sess.CampingStartTime = time.Now().Add(-6 * time.Minute) // past duration

	svc.checkCampingStatus(uid)

	assert.False(t, sess.CampingActive, "CampingActive must be false after camping completes (REQ-REST-18)")
	assert.Equal(t, sess.MaxHP, sess.CurrentHP, "CurrentHP must equal MaxHP after full long rest (REQ-REST-18)")
}

// TestCheckCampingStatus_EnemyInRoom_Cancels verifies that checkCampingStatus cancels
// camping and applies partial restore when a hostile NPC is in the room (REQ-REST-15).
//
// Precondition: sess.CampingActive == true; hostile NPC in same room.
// Postcondition: sess.CampingActive == false.
func TestCheckCampingStatus_EnemyInRoom_Cancels(t *testing.T) {
	svc, sessMgr, npcMgr := newCampingTickSvc(t)

	uid := "camper-hostile"
	sess := addCampingPlayer(t, sessMgr, uid, 5, 20)

	sess.CampingActive = true
	sess.CampingDuration = 5 * time.Minute
	sess.CampingStartTime = time.Now() // just started

	// Spawn a hostile NPC in the same room.
	tmpl := &npc.Template{
		ID:          "bandit",
		Name:        "Bandit",
		NPCType:     "humanoid",
		MaxHP:       10,
		Level:       1,
		Disposition: "hostile",
	}
	_, err := npcMgr.Spawn(tmpl, "tick_room")
	require.NoError(t, err)

	svc.checkCampingStatus(uid)

	assert.False(t, sess.CampingActive, "CampingActive must be false when hostile NPC is present (REQ-REST-15)")
}

// TestHandleMove_WhileCamping_Cancels verifies that handleMove cancels camping when
// the player moves voluntarily (REQ-REST-16).
//
// Precondition: sess.CampingActive == true; valid move direction available.
// Postcondition: sess.CampingActive == false after move.
func TestHandleMove_WhileCamping_Cancels(t *testing.T) {
	svc, sessMgr, _ := newCampingTickSvc(t)

	uid := "camper-move"
	sess := addCampingPlayer(t, sessMgr, uid, 10, 20)

	sess.CampingActive = true
	sess.CampingDuration = 5 * time.Minute
	sess.CampingStartTime = time.Now().Add(-1 * time.Minute) // 1 minute elapsed

	req := &gamev1.MoveRequest{Direction: "north"}
	_, _ = svc.handleMove(uid, req)

	assert.False(t, sess.CampingActive, "CampingActive must be false after player moves (REQ-REST-16)")
}
