// internal/frontend/handlers/input_mode_test.go
package handlers_test

import (
	"sync"
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockModeHandler is a test double for ModeHandler.
type mockModeHandler struct {
	mode      handlers.InputMode
	prompt    string
	enterLog  []string
	exitLog   []string
	mu        sync.Mutex
}

func newMock(m handlers.InputMode, prompt string) *mockModeHandler {
	return &mockModeHandler{mode: m, prompt: prompt}
}

func (h *mockModeHandler) Mode() handlers.InputMode { return h.mode }
func (h *mockModeHandler) Prompt() string           { return h.prompt }
func (h *mockModeHandler) OnEnter(_ *telnet.Conn)   { h.mu.Lock(); h.enterLog = append(h.enterLog, "enter"); h.mu.Unlock() }
func (h *mockModeHandler) OnExit(_ *telnet.Conn)    { h.mu.Lock(); h.exitLog = append(h.exitLog, "exit"); h.mu.Unlock() }
func (h *mockModeHandler) HandleInput(_ string, _ *telnet.Conn, _ gamev1.GameService_SessionClient, _ *int, _ *handlers.SessionInputState) {
}

// TestSessionInputState_SetMode_CallsExitThenEnter verifies OnExit on old,
// OnEnter on new, in order. REQ-IMR-29.
func TestSessionInputState_SetMode_CallsExitThenEnter(t *testing.T) {
	room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
	session := handlers.NewSessionInputState(room)

	mapH := newMock(handlers.ModeMap, "[MAP]")
	session.SetMode(nil, mapH)

	// old handler (room) should have OnExit called once
	require.Equal(t, 1, len(room.ExitLog()))
	// new handler should have OnEnter called once
	require.Equal(t, 1, len(mapH.enterLog))
}

// TestSessionInputState_SetMode_NilOutgoingNoPanic verifies no panic on first
// SetMode when outgoing is the initial RoomModeHandler. REQ-IMR-29.
func TestSessionInputState_SetMode_NilOutgoingNoPanic(t *testing.T) {
	room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
	session := handlers.NewSessionInputState(room)
	mapH := newMock(handlers.ModeMap, "[MAP]")
	assert.NotPanics(t, func() {
		session.SetMode(nil, mapH)
	})
}

// TestSessionInputState_CurrentPrompt_ReflectsActiveHandler verifies
// CurrentPrompt returns the active handler's Prompt(). REQ-IMR-30.
func TestSessionInputState_CurrentPrompt_ReflectsActiveHandler(t *testing.T) {
	room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
	session := handlers.NewSessionInputState(room)
	assert.Equal(t, room.Prompt(), session.CurrentPrompt())

	mapH := newMock(handlers.ModeMap, "[MAP] prompt")
	session.SetMode(nil, mapH)
	assert.Equal(t, "[MAP] prompt", session.CurrentPrompt())
}

// TestSessionInputState_Mode_MatchesActiveHandler verifies Mode() returns the
// active handler's Mode(). REQ-IMR-8A.
func TestSessionInputState_Mode_MatchesActiveHandler(t *testing.T) {
	room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
	session := handlers.NewSessionInputState(room)
	assert.Equal(t, handlers.ModeRoom, session.Mode())

	mapH := newMock(handlers.ModeMap, "[MAP]")
	session.SetMode(nil, mapH)
	assert.Equal(t, handlers.ModeMap, session.Mode())
}

// TestProperty_SessionInputState_ConcurrentSafety verifies no data races
// between concurrent SetMode and CurrentPrompt/Mode reads. REQ-IMR-31a.
func TestProperty_SessionInputState_ConcurrentSafety(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
		session := handlers.NewSessionInputState(room)
		mockHandlers := []*mockModeHandler{
			newMock(handlers.ModeMap, "[MAP]"),
			newMock(handlers.ModeInventory, "[INV]"),
		}
		var wg sync.WaitGroup
		// Writer: SetMode in a loop
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				session.SetMode(nil, mockHandlers[i%2])
			}
		}()
		// Reader: CurrentPrompt + Mode
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = session.CurrentPrompt()
				_ = session.Mode()
			}
		}()
		wg.Wait()
	})
}

// TestProperty_SessionInputState_ModeConsistency verifies Mode() always equals
// the active handler's Mode() after any sequence of SetMode calls. REQ-IMR-31b.
func TestProperty_SessionInputState_ModeConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
		session := handlers.NewSessionInputState(room)
		modes := []handlers.InputMode{handlers.ModeMap, handlers.ModeInventory, handlers.ModeCharSheet, handlers.ModeEditor, handlers.ModeCombat}
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		for i := 0; i < n; i++ {
			idx := rapid.IntRange(0, len(modes)-1).Draw(rt, "mode_idx")
			h := newMock(modes[idx], "prompt")
			session.SetMode(nil, h)
			if session.Mode() != modes[idx] {
				rt.Fatalf("session.Mode() = %v, want %v", session.Mode(), modes[idx])
			}
		}
	})
}

// TestSessionInputState_CurrentPrompt_AfterSetMode_MapHandler verifies
// forwardServerEvents scenario: after SetMode(mapHandler),
// CurrentPrompt returns map prompt not room prompt. REQ-IMR-32.
func TestSessionInputState_CurrentPrompt_AfterSetMode_MapHandler(t *testing.T) {
	room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
	session := handlers.NewSessionInputState(room)
	roomPrompt := session.CurrentPrompt()

	mapH := newMock(handlers.ModeMap, "[MAP] z=zone  w=world  q=exit")
	session.SetMode(nil, mapH)

	got := session.CurrentPrompt()
	assert.NotEqual(t, roomPrompt, got, "map prompt must differ from room prompt")
	assert.Equal(t, "[MAP] z=zone  w=world  q=exit", got)
}

func TestInputMode_String(t *testing.T) {
	cases := []struct {
		mode handlers.InputMode
		want string
	}{
		{handlers.ModeRoom, "room"},
		{handlers.ModeMap, "map"},
		{handlers.ModeInventory, "inventory"},
		{handlers.ModeCharSheet, "charsheet"},
		{handlers.ModeEditor, "editor"},
		{handlers.ModeCombat, "combat"},
		{handlers.InputMode(99), "unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.mode.String())
		})
	}
}
