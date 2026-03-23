// internal/frontend/handlers/input_mode_whitebox_test.go
package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// nilTestHandler is a minimal ModeHandler for nil-outgoing-handler test.
type nilTestHandler struct{}

func (h *nilTestHandler) Mode() InputMode { return ModeMap }
func (h *nilTestHandler) Prompt() string  { return "[MAP]" }
func (h *nilTestHandler) OnEnter(_ *telnet.Conn) {}
func (h *nilTestHandler) OnExit(_ *telnet.Conn)  {}
func (h *nilTestHandler) HandleInput(_ string, _ *telnet.Conn, _ gamev1.GameService_SessionClient, _ *int, _ *SessionInputState) {
}

// TestSessionInputState_SetMode_NilOutgoingNoPanic_Whitebox exercises the
// nil-outgoing-handler guard in SetMode by directly constructing an uninitialized
// SessionInputState (bypassing NewSessionInputState). REQ-IMR-29.
func TestSessionInputState_SetMode_NilOutgoingNoPanic_Whitebox(t *testing.T) {
	s := &SessionInputState{} // current == nil
	h := &nilTestHandler{}
	assert.NotPanics(t, func() {
		s.SetMode(nil, h)
	})
	assert.Equal(t, ModeMap, s.Mode())
}
