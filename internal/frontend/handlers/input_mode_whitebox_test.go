// internal/frontend/handlers/input_mode_whitebox_test.go
package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSessionInputState_SetMode_NilOutgoingNoPanic_Whitebox exercises the
// nil-outgoing-handler guard in SetMode by directly constructing an uninitialized
// SessionInputState (bypassing NewSessionInputState). REQ-IMR-29.
func TestSessionInputState_SetMode_NilOutgoingNoPanic_Whitebox(t *testing.T) {
	s := &SessionInputState{} // current == nil
	mapH := &stubModeHandler{mode: ModeMap, prompt: "[MAP]", enterMessage: "map"}
	assert.NotPanics(t, func() {
		s.SetMode(nil, mapH)
	})
	assert.Equal(t, ModeMap, s.Mode())
}
