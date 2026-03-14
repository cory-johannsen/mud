package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestHandleGrant_HeroPoint_Awards verifies that an editor can grant a hero point to a target.
//
// Precondition: editor session with Role="editor"; target session with HeroPoints==0.
// Postcondition: target.HeroPoints==1; SaveHeroPoints called at least once; response is a MessageEvent.
func TestHandleGrant_HeroPoint_Awards(t *testing.T) {
	charSaver := &heroPointCharSaver{}
	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver})

	_ = addEditorForGrant(t, svc, "editor-uid")
	target := addTargetForGrant(t, svc, "target-uid", "TargetChar")

	require.Equal(t, 0, target.HeroPoints, "precondition: target starts with 0 hero points")

	resp, err := svc.handleGrant("editor-uid", &gamev1.GrantRequest{
		GrantType: "heropoint",
		CharName:  "TargetChar",
		Amount:    0, // amount is ignored for heropoint grants
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	ev, ok := resp.Payload.(*gamev1.ServerEvent_Message)
	require.True(t, ok, "expected MessageEvent response")
	assert.Contains(t, ev.Message.Content, "TargetChar")

	assert.Equal(t, 1, target.HeroPoints, "target must have 1 hero point after grant")
	assert.GreaterOrEqual(t, charSaver.saveHeroPointsCalls.Load(), int32(1), "SaveHeroPoints must be called at least once")
}

// TestHandleGrant_HeroPoint_PermissionDenied verifies that a non-editor cannot grant hero points.
//
// Precondition: player session with Role="player".
// Postcondition: response is an ErrorEvent containing "permission denied".
func TestHandleGrant_HeroPoint_PermissionDenied(t *testing.T) {
	svc := testServiceForGrant(t, grantTestOptions{})

	// Add a player (non-editor).
	_ = addTargetForGrant(t, svc, "player-uid", "RegularPlayer")

	resp, err := svc.handleGrant("player-uid", &gamev1.GrantRequest{
		GrantType: "heropoint",
		CharName:  "SomeTarget",
		Amount:    0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	ev, ok := resp.Payload.(*gamev1.ServerEvent_Error)
	require.True(t, ok, "expected ErrorEvent for non-editor")
	assert.Contains(t, ev.Error.Message, "permission denied")
}
