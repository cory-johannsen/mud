// internal/frontend/handlers/mode_map.go
package handlers

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// MapModeHandler implements ModeHandler for map navigation.
// REQ-IMR-15..18.
type MapModeHandler struct {
	mu              sync.Mutex
	mapView         string // "zone" or "world"
	mapSelectedZone string
	lastMapResponse atomic.Value // *gamev1.MapResponse
}

// NewMapModeHandler constructs a MapModeHandler.
func NewMapModeHandler() *MapModeHandler {
	return &MapModeHandler{}
}

// Mode returns ModeMap. REQ-IMR-3.
func (h *MapModeHandler) Mode() InputMode { return ModeMap }

// Prompt returns the current map prompt string. REQ-IMR-7.
func (h *MapModeHandler) Prompt() string {
	h.mu.Lock()
	sel := h.mapSelectedZone
	resp, _ := h.lastMapResponse.Load().(*gamev1.MapResponse)
	h.mu.Unlock()
	zoneName, danger := zoneNameAndDanger(sel, resp)
	return mapPrompt(sel, zoneName, danger)
}

// OnEnter clears the console and writes the map prompt. REQ-IMR-16.
func (h *MapModeHandler) OnEnter(conn *telnet.Conn) {
	if conn == nil {
		return
	}
	if conn.IsSplitScreen() {
		_ = conn.WriteConsole("")
		_ = conn.WritePromptSplit(h.Prompt())
	} else {
		_ = conn.WritePrompt(h.Prompt())
	}
}

// OnExit clears map state and redraws the console. REQ-IMR-17.
func (h *MapModeHandler) OnExit(conn *telnet.Conn) {
	h.mu.Lock()
	h.mapSelectedZone = ""
	h.mapView = ""
	h.mu.Unlock()
	if conn != nil && conn.IsSplitScreen() {
		_ = conn.RedrawConsole()
	}
}

// SetView sets the map view ("zone" or "world") and resets selection.
func (h *MapModeHandler) SetView(view string) {
	h.mu.Lock()
	h.mapView = view
	h.mapSelectedZone = ""
	h.mu.Unlock()
}

// SetLastResponse stores the latest MapResponse for re-render on resize.
func (h *MapModeHandler) SetLastResponse(resp *gamev1.MapResponse) {
	h.lastMapResponse.Store(resp)
}

// Snapshot returns a consistent read of all map fields.
// Returns (view, selectedZone, lastResp).
func (h *MapModeHandler) Snapshot() (view, selectedZone string, lastResp *gamev1.MapResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	resp, _ := h.lastMapResponse.Load().(*gamev1.MapResponse)
	return h.mapView, h.mapSelectedZone, resp
}

// HandleInput processes one line of map-mode input.
// REQ-IMR-6, REQ-WM-43..51.
func (h *MapModeHandler) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState) {
	handleMapModeInputFn(line, conn, stream, h, requestID, session)
}

// handleMapModeInputFn processes a single line of map-mode input.
// Migrated from AuthHandler.handleMapModeInput; now operates on *MapModeHandler
// and uses *SessionInputState for mode transitions.
// REQ-WM-43..51.
func handleMapModeInputFn(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, mapState *MapModeHandler, requestID *int, session *SessionInputState) {
	line = strings.TrimSpace(line)
	width, _ := conn.Dimensions()

	switch strings.ToLower(line) {
	case "q", "quit", "\x1b": // q or ESC — exit map mode (REQ-WM-50)
		session.SetMode(conn, session.Room())
		return

	case "z", "zone": // switch to zone view (REQ-WM-43)
		mapState.SetView("zone")
		*requestID++
		reqID := fmt.Sprintf("req-%d", *requestID)
		_ = stream.Send(&gamev1.ClientMessage{
			RequestId: reqID,
			Payload: &gamev1.ClientMessage_Map{
				Map: &gamev1.MapRequest{View: "zone"},
			},
		})
		writeMapPromptToConn(conn, "", "", "")
		return

	case "w", "world": // switch to world view (REQ-WM-44)
		mapState.SetView("world")
		*requestID++
		reqID := fmt.Sprintf("req-%d", *requestID)
		_ = stream.Send(&gamev1.ClientMessage{
			RequestId: reqID,
			Payload: &gamev1.ClientMessage_Map{
				Map: &gamev1.MapRequest{View: "world"},
			},
		})
		writeMapPromptToConn(conn, "", "", "")
		return

	case "t", "travel": // travel to selected zone (REQ-WM-48)
		mapState.mu.Lock()
		selectedZone := mapState.mapSelectedZone
		mapState.mu.Unlock()
		if selectedZone == "" {
			writeFmtConsole(conn, "%sSelect a zone first.%s", mapModeGray, "\033[0m")
			writeMapPromptToConn(conn, "", "", "")
			return
		}
		*requestID++
		reqID := fmt.Sprintf("req-%d", *requestID)
		_ = stream.Send(&gamev1.ClientMessage{
			RequestId: reqID,
			Payload: &gamev1.ClientMessage_Travel{
				Travel: &gamev1.TravelRequest{ZoneId: selectedZone},
			},
		})
		return

	case "": // empty input — redisplay map prompt (REQ-WM-51)
		mapState.mu.Lock()
		sel := mapState.mapSelectedZone
		resp, _ := mapState.lastMapResponse.Load().(*gamev1.MapResponse)
		mapState.mu.Unlock()
		zoneName, danger := zoneNameAndDanger(sel, resp)
		writeMapPromptToConn(conn, sel, zoneName, danger)
		return
	}

	// Non-reserved input: treat as zone selector (REQ-WM-46).
	resp, _ := mapState.lastMapResponse.Load().(*gamev1.MapResponse)
	if resp == nil {
		writeFmtConsole(conn, "%sNo map data. Use 'w' to load the world map first.%s", mapModeGray, "\033[0m")
		writeMapPromptToConn(conn, "", "", "")
		return
	}
	tiles := resp.GetWorldTiles()
	if len(tiles) == 0 {
		// Zone view: selector not applicable.
		writeFmtConsole(conn, "%sUnknown map command. Press q to exit.%s", mapModeGray, "\033[0m")
		writeMapPromptToConn(conn, "", "", "")
		return
	}
	zoneID := resolveZoneSelector(line, tiles)
	if zoneID == "" {
		writeFmtConsole(conn, "%sNo zone matching '%s'.%s", mapModeGray, line, "\033[0m")
		writeMapPromptToConn(conn, "", "", "")
		return
	}
	mapState.mu.Lock()
	mapState.mapSelectedZone = zoneID
	mapState.mu.Unlock()
	zoneName, danger := zoneNameAndDanger(zoneID, resp)
	// Re-render world map with selection highlighted (REQ-WM-47).
	if conn.IsSplitScreen() {
		rendered := RenderWorldMap(resp, width)
		_ = conn.WriteConsole(rendered)
	}
	writeMapPromptToConn(conn, zoneID, zoneName, danger)
}
