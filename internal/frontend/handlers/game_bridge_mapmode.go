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

// mapModeState holds all map modal state for the frontend.
// It is shared between commandLoop (writes) and forwardServerEvents (reads/writes).
// All fields must be accessed while holding mu, except lastMapResponse which uses atomic.Value.
//
// REQ-WM-33..36: mapMode, mapView, mapSelectedZone, lastMapResponse.
// REQ-WM-37: PlayerSession must NOT hold these fields.
type mapModeState struct {
	mu              sync.Mutex
	mapMode         bool
	mapView         string // "" or "zone" or "world"; "" treated as "zone"
	mapSelectedZone string // "" means no zone selected
	lastMapResponse atomic.Value // stores *gamev1.MapResponse
}

// isActive returns true if map mode is active.
func (m *mapModeState) isActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mapMode
}

// enter sets map mode active with the given view ("zone" or "world").
// REQ-WM-38, REQ-WM-39.
func (m *mapModeState) enter(view string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mapMode = true
	m.mapView = view
	m.mapSelectedZone = ""
	m.lastMapResponse.Store((*gamev1.MapResponse)(nil))
}

// exit clears map mode and resets mapSelectedZone.
// REQ-WM-54.
func (m *mapModeState) exit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mapMode = false
	m.mapSelectedZone = ""
}

// setLastResponse stores the most recent MapResponse for re-render on resize and
// for zone resolution by the travel command (REQ-WM-36, REQ-WM-57..59).
func (m *mapModeState) setLastResponse(resp *gamev1.MapResponse) {
	m.lastMapResponse.Store(resp)
}

// snapshot returns a consistent read of all map mode fields.
func (m *mapModeState) snapshot() (active bool, view, selectedZone string, lastResp *gamev1.MapResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resp, _ := m.lastMapResponse.Load().(*gamev1.MapResponse)
	return m.mapMode, m.mapView, m.mapSelectedZone, resp
}

// mapPrompt returns the prompt string for the current map mode state.
// REQ-WM-41 and REQ-WM-47.
func mapPrompt(selectedZone string, selectedZoneName, dangerLevel string) string {
	if selectedZone != "" && selectedZoneName != "" {
		return fmt.Sprintf("[MAP] Selected: %s (%s)  t=travel  q=exit", selectedZoneName, dangerLevel)
	}
	return "[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit"
}

const mapModeGray = "\033[37m"

// renderMapConsole renders the map response into a terminal string for the console region.
func renderMapConsole(resp *gamev1.MapResponse, view string, width int) string {
	if view == "world" {
		return RenderWorldMap(resp, width)
	}
	return RenderMap(resp, width)
}

// resolveZoneSelector resolves a user input string to a world zone ID.
// It first tries numeric legend matching, then case-insensitive prefix matching
// on zone names in lexicographic zone ID order (REQ-WM-46, REQ-WM-27).
// Only zones present in worldTiles are candidates.
// Returns "" if no match found.
func resolveZoneSelector(input string, worldTiles []*gamev1.WorldZoneTile) string {
	if len(worldTiles) == 0 {
		return ""
	}

	// Sort tiles top-to-bottom, left-to-right (same order as legend assignment).
	sorted := make([]*gamev1.WorldZoneTile, len(worldTiles))
	copy(sorted, worldTiles)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].WorldY != sorted[j].WorldY {
			return sorted[i].WorldY < sorted[j].WorldY
		}
		return sorted[i].WorldX < sorted[j].WorldX
	})

	// Try numeric legend number.
	var legendNum int
	if _, err := fmt.Sscanf(input, "%d", &legendNum); err == nil {
		if legendNum >= 1 && legendNum <= len(sorted) {
			return sorted[legendNum-1].ZoneId
		}
	}

	// Try case-insensitive prefix match on zone name, tiebreak by lexicographic zone ID.
	lower := strings.ToLower(input)
	// Build list of candidates sorted by zone ID for tiebreaking.
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

// resolveTravelZone resolves a zone name fragment for the `travel` command (REQ-WM-58).
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
	// Tiebreak by lexicographic zone ID order (REQ-WM-58).
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

// handleMapModeInput processes a single line of input while map mode is active.
// REQ-WM-43..51.
func (h *AuthHandler) handleMapModeInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, mapState *mapModeState, buildPrompt func() string, requestID *int) {
	line = strings.TrimSpace(line)
	width, _ := conn.Dimensions()

	switch strings.ToLower(line) {
	case "q", "quit", "\x1b": // q or ESC — exit map mode (REQ-WM-50)
		mapState.exit()
		if conn.IsSplitScreen() {
			_ = conn.RedrawConsole()
			_ = conn.WritePromptSplit(buildPrompt())
		} else {
			_ = conn.WritePrompt(buildPrompt())
		}
		return

	case "z", "zone": // switch to zone view (REQ-WM-43)
		mapState.mu.Lock()
		mapState.mapView = "zone"
		mapState.mapSelectedZone = ""
		mapState.mu.Unlock()
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
		mapState.mu.Lock()
		mapState.mapView = "world"
		mapState.mapSelectedZone = ""
		mapState.mu.Unlock()
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
