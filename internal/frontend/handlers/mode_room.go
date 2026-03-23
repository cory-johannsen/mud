// internal/frontend/handlers/mode_room.go
package handlers

import (
	"sync"
	"sync/atomic"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// RoomModeHandler implements ModeHandler for normal room-command input.
// REQ-IMR-12, REQ-IMR-12A, REQ-IMR-13, REQ-IMR-14.
type RoomModeHandler struct {
	// Prompt state (injected at construction). REQ-IMR-12A.
	charName         string
	role             string
	currentHP        *atomic.Int32
	maxHP            *atomic.Int32
	currentRoom      *atomic.Value
	currentTime      *atomic.Value
	condMu           *sync.Mutex
	activeConditions map[string]string

	// test-only fields: exitLog and enterLog record lifecycle calls for assertions.
	exitLog  []string
	enterLog []string
}

// NewRoomModeHandler constructs a RoomModeHandler with all live-state refs.
func NewRoomModeHandler(
	charName, role string,
	currentHP, maxHP *atomic.Int32,
	currentRoom, currentTime *atomic.Value,
	condMu *sync.Mutex,
	activeConditions map[string]string,
) *RoomModeHandler {
	return &RoomModeHandler{
		charName:         charName,
		role:             role,
		currentHP:        currentHP,
		maxHP:            maxHP,
		currentRoom:      currentRoom,
		currentTime:      currentTime,
		condMu:           condMu,
		activeConditions: activeConditions,
	}
}

// NewRoomModeHandlerForTest creates a minimal RoomModeHandler for unit tests.
func NewRoomModeHandlerForTest(charName string, hp, maxHP int32) *RoomModeHandler {
	chp := &atomic.Int32{}
	chp.Store(hp)
	mhp := &atomic.Int32{}
	mhp.Store(maxHP)
	return &RoomModeHandler{
		charName:         charName,
		currentHP:        chp,
		maxHP:            mhp,
		currentRoom:      &atomic.Value{},
		currentTime:      &atomic.Value{},
		condMu:           &sync.Mutex{},
		activeConditions: map[string]string{},
	}
}

// ExitLog returns the list of OnExit call records (test helper).
func (h *RoomModeHandler) ExitLog() []string { return h.exitLog }

// Mode returns ModeRoom. REQ-IMR-3.
func (h *RoomModeHandler) Mode() InputMode { return ModeRoom }

// OnEnter redraws the room prompt on return to room mode. REQ-IMR-4.
func (h *RoomModeHandler) OnEnter(conn *telnet.Conn) {
	h.enterLog = append(h.enterLog, "enter")
	if conn != nil {
		if conn.IsSplitScreen() {
			_ = conn.WritePromptSplit(h.Prompt())
		} else {
			_ = conn.WritePrompt(h.Prompt())
		}
	}
}

// OnExit records the exit (no visual change needed). REQ-IMR-5.
func (h *RoomModeHandler) OnExit(_ *telnet.Conn) {
	h.exitLog = append(h.exitLog, "exit")
}

// Prompt returns the colored room prompt string. REQ-IMR-13.
func (h *RoomModeHandler) Prompt() string {
	var conditions []string
	if h.condMu != nil {
		h.condMu.Lock()
		for _, name := range h.activeConditions {
			conditions = append(conditions, name)
		}
		h.condMu.Unlock()
	}
	hp := int32(10)
	mhp := int32(10)
	if h.currentHP != nil {
		hp = h.currentHP.Load()
	}
	if h.maxHP != nil {
		mhp = h.maxHP.Load()
	}
	return BuildPrompt(h.charName, hp, mhp, conditions)
}

// HandleInput handles empty-line prompt redraw. REQ-IMR-14.
// NOTE: Full command dispatch remains in commandLoop for this refactor.
// ModeRoom input is NOT routed through session.HandleInput — commandLoop
// handles it inline. This HandleInput is only called if a future refactor
// routes ModeRoom input through session.HandleInput.
func (h *RoomModeHandler) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState) {
	// Empty line: just redraw prompt.
	if line == "" {
		if conn != nil {
			if conn.IsSplitScreen() {
				_ = conn.WritePromptSplit(h.Prompt())
			} else {
				_ = conn.WritePrompt(h.Prompt())
			}
		}
		return
	}
	// Non-empty: no-op in this refactor. commandLoop handles room dispatch inline.
	_ = line
	_ = stream
	_ = requestID
	_ = session
}

// buildRoomPromptFn returns a func() string closure for legacy callers
// during the transition. Deprecated once all callers use session.CurrentPrompt().
func (h *RoomModeHandler) buildRoomPromptFn() func() string {
	return h.Prompt
}
