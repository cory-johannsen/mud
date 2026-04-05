package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"go.uber.org/zap/zaptest"
)

// buildUseFallthroughService constructs a GameServiceServer for use-fallthrough tests.
// When feat is non-nil, the feat registry and repo are populated with that feat.
// When equipMgr is non-nil, it is injected as the roomEquipMgr.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil *GameServiceServer with a feat repo set (never nil)
// so that handleUse does not exit early with "Ability data is not available."
func buildUseFallthroughService(
	t *testing.T,
	sessMgr *session.Manager,
	feat *ruleset.Feat,
	equipMgr *inventory.RoomEquipmentManager,
) *GameServiceServer {
	t.Helper()
	worldMgr, _ := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	var allFeats []*ruleset.Feat
	var featRegistry *ruleset.FeatRegistry
	featsRepo := &stubFeatsRepo{data: map[int64][]string{}}
	if feat != nil {
		allFeats = []*ruleset.Feat{feat}
		featRegistry = ruleset.NewFeatRegistry(allFeats)
		featsRepo = &stubFeatsRepo{
			data: map[int64][]string{0: {feat.ID}},
		}
	} else {
		featRegistry = ruleset.NewFeatRegistry(nil)
	}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		allFeats, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	svc.roomEquipMgr = equipMgr
	return svc
}

// addPlayerForFallthroughTest adds a player with CharacterID=0 in room_a.
//
// Precondition: sessMgr must be non-nil; uid must be unique.
// Postcondition: Returns a non-nil *session.PlayerSession in room_a.
func addPlayerForFallthroughTest(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	return sess
}

// TestHandleUse_FeatMatchFirst_EquipmentNotCalled verifies that when a feat matches
// the abilityID, handleUse activates the feat and does NOT fall through to equipment.
//
// Precondition: feat "push" is registered; room has equipment with ItemDefID "push".
// Postcondition: UseResponse.Message contains feat's ActivateText, not equipment output.
func TestHandleUse_FeatMatchFirst_EquipmentNotCalled(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "push",
		Name:         "Push",
		Active:       true,
		ActivateText: "You push with force!",
	}

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "push", MaxCount: 1, Immovable: true, Script: ""},
	})

	svc := buildUseFallthroughService(t, sessMgr, feat, mgr)
	addPlayerForFallthroughTest(t, sessMgr, "u_feat_first")

	evt, err := svc.handleUse("u_feat_first", "push", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	useResp := evt.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse, not Message")
	assert.Contains(t, useResp.Message, "You push with force!")
}

// TestHandleUse_NoFeatMatch_FallsThruToEquipment verifies that when no feat matches
// the abilityID but a room equipment instance does, handleUse activates the equipment.
//
// Precondition: no feat named "console"; room_a has equipment with ItemDefID "console" and no script.
// Postcondition: event message contains "Nothing happens" (equipment no-script response).
func TestHandleUse_NoFeatMatch_FallsThruToEquipment(t *testing.T) {
	sessMgr := session.NewManager()

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "console", MaxCount: 1, Immovable: true, Script: ""},
	})

	// Pass nil feat so feat registry has no entries; featsRepo maps char 0 → [].
	svc := buildUseFallthroughService(t, sessMgr, nil, mgr)
	addPlayerForFallthroughTest(t, sessMgr, "u_fallthrough")

	evt, err := svc.handleUse("u_fallthrough", "console", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected Message event from equipment activation")
	assert.Contains(t, msg.Content, "Nothing happens")
}

// TestHandleUse_NeitherMatch_ReturnsError verifies that when neither a feat nor any
// room equipment matches the abilityID, handleUse returns an appropriate error message.
//
// Precondition: no feat named "mystery"; room has no equipment with that ID.
// Postcondition: event message indicates the ability was not found.
func TestHandleUse_NeitherMatch_ReturnsError(t *testing.T) {
	sessMgr := session.NewManager()

	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room_a", []world.RoomEquipmentConfig{
		{ItemID: "other_item", MaxCount: 1, Immovable: true, Script: ""},
	})

	svc := buildUseFallthroughService(t, sessMgr, nil, mgr)
	addPlayerForFallthroughTest(t, sessMgr, "u_neither")

	evt, err := svc.handleUse("u_neither", "mystery", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "mystery")
}
