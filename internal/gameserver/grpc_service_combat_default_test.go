package gameserver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// mockCharSaverCombatDefault is a CharacterSaver test double for handleCombatDefault tests.
//
// Precondition: none.
// Postcondition: SaveDefaultCombatAction records calls and returns saveErr if set.
type mockCharSaverCombatDefault struct {
	mockCharSaverFull
	saveErr   error
	saveCalls atomic.Int32
	savedAction string
}

// SaveDefaultCombatAction records the call and returns saveErr if set.
func (m *mockCharSaverCombatDefault) SaveDefaultCombatAction(_ context.Context, _ int64, action string) error {
	m.saveCalls.Add(1)
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedAction = action
	return nil
}

// SaveAbilities satisfies CharacterSaver; always succeeds.
func (m *mockCharSaverCombatDefault) SaveAbilities(_ context.Context, _ int64, _ character.AbilityScores) error {
	return nil
}

func (m *mockCharSaverCombatDefault) SaveCurrency(_ context.Context, _ int64, _ int) error { return nil }
func (m *mockCharSaverCombatDefault) LoadCurrency(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockCharSaverCombatDefault) SaveGender(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *mockCharSaverCombatDefault) SaveHeroPoints(_ context.Context, _ int64, _ int) error {
	return nil
}
func (m *mockCharSaverCombatDefault) LoadHeroPoints(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockCharSaverCombatDefault) SaveJobs(_ context.Context, _ int64, _ map[string]int, _ string) error {
	return nil
}
func (m *mockCharSaverCombatDefault) SaveInstanceCharges(_ context.Context, _ int64, _, _ string, _ int, _ bool) error {
	return nil
}
func (m *mockCharSaverCombatDefault) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	return map[string]int{}, "", nil
}
func (m *mockCharSaverCombatDefault) LoadFocusPoints(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockCharSaverCombatDefault) SaveFocusPoints(_ context.Context, _ int64, _ int) error {
	return nil
}
func (m *mockCharSaverCombatDefault) SaveHotbar(_ context.Context, _ int64, _ [10]string) error {
	return nil
}
func (m *mockCharSaverCombatDefault) LoadHotbar(_ context.Context, _ int64) ([10]string, error) {
	return [10]string{}, nil
}

// testServiceForCombatDefault creates a minimal GameServiceServer suitable for handleCombatDefault tests.
//
// Precondition: t must be non-nil; saver may be nil.
// Postcondition: Returns a configured *GameServiceServer with no world state beyond the session manager.
func testServiceForCombatDefault(t *testing.T, saver CharacterSaver) *GameServiceServer {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	return newTestGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		saver, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
}

// addPlayerForCombatDefault adds a player session with the given uid and characterID.
//
// Precondition: svc must have a valid session manager.
// Postcondition: Player is in the session manager and the session is returned.
func addPlayerForCombatDefault(t *testing.T, svc *GameServiceServer, uid string, characterID int64) *session.PlayerSession {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:               uid,
		Username:          uid,
		CharName:          uid,
		CharacterID:       characterID,
		RoomID:            "room_a",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "",
		Class:             "",
		Level:             1,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	return sess
}

// TestHandleCombatDefault_PlayerNotFound verifies that handleCombatDefault returns an error
// (without panicking) when the uid does not identify an active session.
//
// Precondition: no session for uid exists.
// Postcondition: Returns a non-nil error; SaveDefaultCombatAction is never called.
func TestHandleCombatDefault_PlayerNotFound(t *testing.T) {
	saver := &mockCharSaverCombatDefault{}
	svc := testServiceForCombatDefault(t, saver)

	_, err := svc.handleCombatDefault("nonexistent_uid", "attack")

	assert.Error(t, err, "handleCombatDefault must return error for unknown uid")
	assert.Equal(t, int32(0), saver.saveCalls.Load(), "SaveDefaultCombatAction must not be called for unknown player")
}

// TestHandleCombatDefault_InvalidAction verifies that an invalid action produces an error message event
// and that SaveDefaultCombatAction is never called.
//
// Precondition: uid identifies an active session; action is not in ValidCombatActions.
// Postcondition: Returns a message event with error text; SaveDefaultCombatAction is not called.
func TestHandleCombatDefault_InvalidAction(t *testing.T) {
	saver := &mockCharSaverCombatDefault{}
	svc := testServiceForCombatDefault(t, saver)
	addPlayerForCombatDefault(t, svc, "u_invalid", 1)

	evt, err := svc.handleCombatDefault("u_invalid", "notanaction")

	require.NoError(t, err, "handleCombatDefault must not return a Go error for invalid action")
	require.NotNil(t, evt, "handleCombatDefault must return a non-nil ServerEvent")
	msg := evt.GetMessage()
	require.NotNil(t, msg, "ServerEvent must carry a MessageEvent")
	assert.Contains(t, msg.Content, "Invalid", "message must indicate the action is invalid")
	assert.Equal(t, int32(0), saver.saveCalls.Load(), "SaveDefaultCombatAction must not be called for invalid action")
}

// TestHandleCombatDefault_PersistenceFailure verifies that when SaveDefaultCombatAction returns an error,
// the handler returns an error message event and sess.DefaultCombatAction is NOT updated.
//
// Precondition: uid identifies an active session; action is valid; charSaver.SaveDefaultCombatAction returns an error.
// Postcondition: Returns a message event with failure text; sess.DefaultCombatAction remains unchanged.
func TestHandleCombatDefault_PersistenceFailure(t *testing.T) {
	saver := &mockCharSaverCombatDefault{saveErr: errors.New("db error")}
	svc := testServiceForCombatDefault(t, saver)
	sess := addPlayerForCombatDefault(t, svc, "u_persist_fail", 42)

	evt, err := svc.handleCombatDefault("u_persist_fail", "attack")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Failed", "message must indicate save failure")
	assert.Equal(t, int32(1), saver.saveCalls.Load(), "SaveDefaultCombatAction must have been called once")
	assert.Equal(t, "pass", sess.DefaultCombatAction, "sess.DefaultCombatAction must not be updated on persistence failure (must remain at default)")
}

// TestHandleCombatDefault_HappyPath verifies that a valid action is persisted and the session is updated.
//
// Precondition: uid identifies an active session; action is valid; charSaver succeeds.
// Postcondition: SaveDefaultCombatAction is called once; sess.DefaultCombatAction equals action; confirmation returned.
func TestHandleCombatDefault_HappyPath(t *testing.T) {
	saver := &mockCharSaverCombatDefault{}
	svc := testServiceForCombatDefault(t, saver)
	sess := addPlayerForCombatDefault(t, svc, "u_happy", 99)

	evt, err := svc.handleCombatDefault("u_happy", "parry")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "parry", "confirmation message must include the action")
	assert.Equal(t, int32(1), saver.saveCalls.Load(), "SaveDefaultCombatAction must be called exactly once")
	assert.Equal(t, "parry", sess.DefaultCombatAction, "sess.DefaultCombatAction must be updated to the new action")
}

// TestHandleCombatDefault_AllValidActions_HappyPath is a property-based test verifying that every
// action in ValidCombatActions succeeds without error.
//
// Precondition: saver always succeeds.
// Postcondition: For any valid action, SaveDefaultCombatAction is called and session is updated.
func TestHandleCombatDefault_AllValidActions_HappyPath(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		action := rapid.SampledFrom(command.ValidCombatActions).Draw(rt, "action")

		saver := &mockCharSaverCombatDefault{}
		svc := testServiceForCombatDefault(t, saver)
		sess := addPlayerForCombatDefault(t, svc, "u_prop_valid", 1)

		evt, err := svc.handleCombatDefault("u_prop_valid", action)

		if err != nil {
			rt.Fatalf("expected no error for valid action %q, got %v", action, err)
		}
		if evt == nil || evt.GetMessage() == nil {
			rt.Fatalf("expected non-nil MessageEvent for valid action %q", action)
		}
		if sess.DefaultCombatAction != action {
			rt.Fatalf("expected DefaultCombatAction=%q, got %q", action, sess.DefaultCombatAction)
		}
	})
}
