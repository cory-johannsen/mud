package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// REQ-ILT6: handleSelectTech with no pending grants sends "no pending technology selections."
func TestHandleSelectTech_NoPending_SendsNoPending(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-selecttech-empty"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	stream := &fakeSessionStream{}
	err = svc.handleSelectTech(uid, "req1", stream)
	require.NoError(t, err)

	require.NotEmpty(t, stream.sent)
	last := stream.sent[len(stream.sent)-1]
	msg := last.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "no pending")
}

// REQ-ILT9: CharacterSheetView reports pending tech selections count.
func TestBuildCharacterSheetView_PendingTechSelections(t *testing.T) {
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	uid := "player-pending-tech-sheet"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	target, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	target.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
		2: {Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "pending_choice", Level: 1}},
		}},
	}

	evt, err := svc.handleChar(uid)
	require.NoError(t, err)
	require.NotNil(t, evt)

	var sheetView *gamev1.CharacterSheetView
	if cs := evt.GetCharacterSheet(); cs != nil {
		sheetView = cs
	}
	require.NotNil(t, sheetView, "CharacterSheetView must be returned by handleChar")
	assert.Equal(t, int32(1), sheetView.PendingTechSelections)
}
