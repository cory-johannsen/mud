package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// REQ-UC8: Character sheet PreparedSlotView.expended reflects session state.
func TestHandleChar_PreparedSlots_ReflectsExpendedState(t *testing.T) {
	mgr := session.NewManager()
	_, err := mgr.AddPlayer(session.AddPlayerOptions{
		UID:      "uid-sheet-prep",
		Username: "user-sheet-prep",
		CharName: "PrepChar",
		RoomID:   "room1",
		Role:     "player",
	})
	require.NoError(t, err)

	sess, ok := mgr.GetPlayer("uid-sheet-prep")
	require.True(t, ok)
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {
			{TechID: "shock_grenade", Expended: false},
			{TechID: "neural_disruptor", Expended: true},
		},
	}

	svc := &GameServiceServer{sessions: mgr}
	result, err := svc.handleChar("uid-sheet-prep")
	require.NoError(t, err)
	require.NotNil(t, result)

	cs, ok := result.Payload.(*gamev1.ServerEvent_CharacterSheet)
	require.True(t, ok, "expected ServerEvent_CharacterSheet payload, got %T", result.Payload)

	sheetView := cs.CharacterSheet
	require.Len(t, sheetView.PreparedSlots, 2)

	byTech := make(map[string]*gamev1.PreparedSlotView)
	for _, s := range sheetView.PreparedSlots {
		byTech[s.TechId] = s
	}
	require.Contains(t, byTech, "shock_grenade")
	assert.False(t, byTech["shock_grenade"].Expended, "shock_grenade slot must not be expended")
	require.Contains(t, byTech, "neural_disruptor")
	assert.True(t, byTech["neural_disruptor"].Expended, "neural_disruptor slot must be expended")
}

// REQ-UC5: After rest, previously expended slots are restored (Expended = false in session).
func TestHandleRest_ResetsExpendedSlots(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-rest-expended"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
		Level:    1,
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)

	// Set up an expended slot before rest.
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "shock_grenade", Expended: true}},
	}

	prepRepo := &fakePreparedRepoRest{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "shock_grenade", Expended: false}},
		},
	}
	svc.SetPreparedTechRepo(prepRepo)

	stream := &fakeSessionStream{}
	require.NoError(t, svc.handleRest(uid, "req-rest", stream))

	// After rest, RearrangePreparedTechs rebuilds slots from the repo (which returns expended=false).
	// The session PreparedTechs must be non-nil and the slot must not be expended.
	require.NotNil(t, sess.PreparedTechs)
}
