// internal/frontend/handlers/mode_stubs.go
package handlers

import (
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// stubModeHandler is a reusable base for stub mode implementations.
// REQ-IMR-19, REQ-IMR-20.
type stubModeHandler struct {
	mode         InputMode
	enterMessage string
	prompt       string
}

func (h *stubModeHandler) Mode() InputMode { return h.mode }

func (h *stubModeHandler) Prompt() string { return h.prompt }

func (h *stubModeHandler) OnEnter(conn *telnet.Conn) {
	if conn == nil {
		return
	}
	if conn.IsSplitScreen() {
		_ = conn.WriteConsole(h.enterMessage)
		_ = conn.WritePromptSplit(h.prompt)
	} else {
		_ = conn.WriteLine(h.enterMessage)
		_ = conn.WritePrompt(h.prompt)
	}
}

func (h *stubModeHandler) OnExit(conn *telnet.Conn) {
	if conn != nil && conn.IsSplitScreen() {
		_ = conn.RedrawConsole()
	}
}

func (h *stubModeHandler) HandleInput(line string, conn *telnet.Conn, _ gamev1.GameService_SessionClient, _ *int, session *SessionInputState) {
	if line == "q" || line == "\x1b" {
		session.SetMode(conn, session.Room())
		return
	}
	if conn != nil {
		msg := "Press 'q' to exit."
		if conn.IsSplitScreen() {
			_ = conn.WriteConsole(msg)
			_ = conn.WritePromptSplit(h.prompt)
		} else {
			_ = conn.WriteLine(msg)
			_ = conn.WritePrompt(h.prompt)
		}
	}
}

// InventoryModeHandler is a stub for the inventory/loot screen. REQ-IMR-19.
type InventoryModeHandler struct{ stubModeHandler }

func NewInventoryModeHandler() *InventoryModeHandler {
	return &InventoryModeHandler{stubModeHandler{
		mode:         ModeInventory,
		enterMessage: "[INVENTORY] (coming soon)  Press 'q' to exit.",
		prompt:       "[INV] q=exit",
	}}
}

// CharSheetModeHandler is a stub for the character sheet viewer. REQ-IMR-19.
type CharSheetModeHandler struct{ stubModeHandler }

func NewCharSheetModeHandler() *CharSheetModeHandler {
	return &CharSheetModeHandler{stubModeHandler{
		mode:         ModeCharSheet,
		enterMessage: "[CHARACTER SHEET] (coming soon)  Press 'q' to exit.",
		prompt:       "[CHAR] q=exit",
	}}
}

// EditorModeHandler is a stub for the world editor. REQ-IMR-19.
type EditorModeHandler struct{ stubModeHandler }

func NewEditorModeHandler() *EditorModeHandler {
	return &EditorModeHandler{stubModeHandler{
		mode:         ModeEditor,
		enterMessage: "[EDITOR] (coming soon)  Press 'q' to exit.",
		prompt:       "[EDIT] q=exit",
	}}
}

// CombatModeHandler is a stub for the combat display. REQ-IMR-19.
type CombatModeHandler struct{ stubModeHandler }

func NewCombatModeHandler() *CombatModeHandler {
	return &CombatModeHandler{stubModeHandler{
		mode:         ModeCombat,
		enterMessage: "[COMBAT] (coming soon)  Press 'q' to exit.",
		prompt:       "[COMBAT] q=exit",
	}}
}
