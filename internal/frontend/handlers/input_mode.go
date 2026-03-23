// internal/frontend/handlers/input_mode.go
package handlers

import (
	"sync"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// InputMode identifies the current input routing mode for a player session.
// REQ-IMR-1.
type InputMode int

const (
	ModeRoom      InputMode = iota // default: room commands + movement
	ModeMap                        // map navigation
	ModeInventory                  // inventory / loot screen
	ModeCharSheet                  // character sheet viewer
	ModeEditor                     // world editor commands
	ModeCombat                     // combat display
)

// String returns a human-readable name for the mode. REQ-IMR-2.
func (m InputMode) String() string {
	switch m {
	case ModeRoom:
		return "room"
	case ModeMap:
		return "map"
	case ModeInventory:
		return "inventory"
	case ModeCharSheet:
		return "charsheet"
	case ModeEditor:
		return "editor"
	case ModeCombat:
		return "combat"
	default:
		return "unknown"
	}
}

// ModeHandler handles input and prompt rendering for one InputMode.
// REQ-IMR-3.
type ModeHandler interface {
	// Mode returns the InputMode constant for this handler.
	Mode() InputMode
	// OnEnter is called when this mode becomes active.
	// REQ-IMR-4.
	OnEnter(conn *telnet.Conn)
	// OnExit is called when this mode is being replaced.
	// REQ-IMR-5.
	OnExit(conn *telnet.Conn)
	// HandleInput processes one trimmed input line.
	// session is provided so handlers can call session.SetMode for transitions.
	// REQ-IMR-6.
	HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState)
	// Prompt returns the prompt string to display for this mode.
	// REQ-IMR-7.
	Prompt() string
}

// SessionInputState owns the active ModeHandler and serializes transitions.
// It is safe for concurrent use: SetMode from commandLoop and CurrentPrompt/Mode
// from forwardServerEvents run concurrently.
// REQ-IMR-8, REQ-IMR-10.
type SessionInputState struct {
	mu      sync.RWMutex
	current ModeHandler
	room    *RoomModeHandler
}

// NewSessionInputState constructs a SessionInputState with roomHandler as the
// initial active handler. REQ-IMR-9A.
func NewSessionInputState(roomHandler *RoomModeHandler) *SessionInputState {
	return &SessionInputState{
		current: roomHandler,
		room:    roomHandler,
	}
}

// SetMode transitions to handler m. If the current handler is non-nil, its
// OnExit is called first. Then m.OnEnter is called.
// REQ-IMR-9.
func (s *SessionInputState) SetMode(conn *telnet.Conn, m ModeHandler) {
	s.mu.Lock()
	old := s.current
	s.current = m
	s.mu.Unlock()
	if old != nil {
		old.OnExit(conn)
	}
	m.OnEnter(conn)
}

// CurrentPrompt returns the active handler's Prompt() string.
// REQ-IMR-8, REQ-IMR-8A.
func (s *SessionInputState) CurrentPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.Prompt()
}

// Mode returns the active handler's InputMode constant.
// REQ-IMR-8A.
func (s *SessionInputState) Mode() InputMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current.Mode()
}

// Room returns the RoomModeHandler. Used by forwardServerEvents to return to
// room mode after travel or other mode-exiting server events.
// Invariant: s.room is immutable after construction and requires no lock.
// REQ-IMR-11.
func (s *SessionInputState) Room() *RoomModeHandler {
	return s.room
}

// HandleInput delegates the input line to the currently active ModeHandler.
// It acquires a read lock to snapshot the handler, then releases the lock
// before calling HandleInput so handlers may call SetMode without deadlocking.
// REQ-IMR-6, REQ-IMR-8.
func (s *SessionInputState) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int) {
	s.mu.RLock()
	h := s.current
	s.mu.RUnlock()
	h.HandleInput(line, conn, stream, requestID, s)
}
