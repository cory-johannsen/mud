// internal/frontend/handlers/mode_map.go
package handlers

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/world"
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

const mapModeGray = "\033[37m"

// mapPrompt returns the prompt string for the current map mode state.
func mapPrompt(selectedZone string, selectedZoneName, dangerLevel string) string {
	if selectedZone != "" && selectedZoneName != "" {
		return fmt.Sprintf("[MAP] Selected: %s (%s)  t=travel  q=exit", selectedZoneName, dangerLevel)
	}
	return "[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit"
}

// renderMapConsole renders the map response into a terminal string for the console region.
func renderMapConsole(resp *gamev1.MapResponse, view string, width int) string {
	if view == "world" {
		return RenderWorldMap(resp, width)
	}
	return RenderMap(resp, width)
}

// resolveZoneSelector resolves a user input string to a world zone ID.
// It first tries numeric legend matching, then case-insensitive prefix matching
// on zone names in lexicographic zone ID order.
// Only zones present in worldTiles are candidates.
// Returns "" if no match found.
func resolveZoneSelector(input string, worldTiles []*gamev1.WorldZoneTile) string {
	if len(worldTiles) == 0 {
		return ""
	}
	sorted := make([]*gamev1.WorldZoneTile, len(worldTiles))
	copy(sorted, worldTiles)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].WorldY != sorted[j].WorldY {
			return sorted[i].WorldY < sorted[j].WorldY
		}
		return sorted[i].WorldX < sorted[j].WorldX
	})
	var legendNum int
	if _, err := fmt.Sscanf(input, "%d", &legendNum); err == nil {
		if legendNum >= 1 && legendNum <= len(sorted) {
			return sorted[legendNum-1].ZoneId
		}
	}
	lower := strings.ToLower(input)
	type candidate struct {
		zoneID   string
		zoneName string
	}
	var candidates []candidate
	for _, t := range sorted {
		if strings.HasPrefix(strings.ToLower(t.ZoneName), lower) {
			candidates = append(candidates, candidate{t.ZoneId, t.ZoneName})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].zoneID < candidates[j].zoneID
	})
	return candidates[0].zoneID
}

// resolveTravelZone resolves a zone name fragment for the `travel` command.
// Only zones with non-nil WorldX/WorldY are valid targets.
// Returns ("", false) if no match; (zoneID, true) if exactly one prefix match.
func resolveTravelZone(input string, zones []*world.Zone) (string, bool) {
	lower := strings.ToLower(input)
	type candidate struct {
		zoneID string
	}
	var candidates []candidate
	for _, z := range zones {
		if z.WorldX == nil || z.WorldY == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(z.Name), lower) {
			candidates = append(candidates, candidate{z.ID})
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].zoneID < candidates[j].zoneID
	})
	return candidates[0].zoneID, true
}

// writeFmtConsole writes a formatted string to the telnet console or line (based on split-screen mode).
func writeFmtConsole(conn *telnet.Conn, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if conn.IsSplitScreen() {
		_ = conn.WriteConsole(msg)
	} else {
		_ = conn.WriteLine(msg)
	}
}

// sortWorldTiles sorts world tiles by lexicographic zone ID (ascending).
func sortWorldTiles(tiles []*gamev1.WorldZoneTile) {
	sort.Slice(tiles, func(i, j int) bool {
		return tiles[i].ZoneId < tiles[j].ZoneId
	})
}

// zoneNameAndDanger extracts zone name and danger level from a MapResponse for a given zone ID.
func zoneNameAndDanger(zoneID string, resp *gamev1.MapResponse) (name, danger string) {
	if resp == nil || zoneID == "" {
		return "", ""
	}
	for _, t := range resp.GetWorldTiles() {
		if t.GetZoneId() == zoneID {
			return t.GetZoneName(), t.GetDangerLevel()
		}
	}
	return "", ""
}

// writeMapPromptToConn writes the appropriate map prompt to the connection.
func writeMapPromptToConn(conn *telnet.Conn, selectedZone, zoneName, dangerLevel string) {
	prompt := mapPrompt(selectedZone, zoneName, dangerLevel)
	if conn.IsSplitScreen() {
		_ = conn.WritePromptSplit(prompt)
	} else {
		_ = conn.WritePrompt(prompt)
	}
}
