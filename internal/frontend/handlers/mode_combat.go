// internal/frontend/handlers/mode_combat.go
package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// combatGridCoord holds the 2D grid coordinates for a single combatant.
type combatGridCoord struct {
	X, Y int32
}

// maxCombatLogLines is the maximum number of narrative log lines retained.
const maxCombatLogLines = 20

// movementCommands enumerates direction strings that are blocked during combat.
var movementCommands = map[string]bool{
	"n": true, "north": true,
	"s": true, "south": true,
	"e": true, "east": true,
	"w": true, "west": true,
	"ne": true, "northeast": true,
	"nw": true, "northwest": true,
	"se": true, "southeast": true,
	"sw": true, "southwest": true,
	"u": true, "up": true,
	"d": true, "down": true,
}

// IsMovementCommand returns true if line is a direction command that should be blocked in combat.
func IsMovementCommand(line string) bool {
	return movementCommands[strings.ToLower(strings.TrimSpace(line))]
}

// isMovementCommand is an alias for IsMovementCommand for internal use.
func isMovementCommand(line string) bool {
	return IsMovementCommand(line)
}

// CombatantState holds a snapshot of one combatant's status.
type CombatantState struct {
	Name       string
	HP         int
	MaxHP      int
	AP         int
	MaxAP      int
	Conditions []string
	IsDead     bool
	IsPlayer   bool
	IsCurrent  bool
	Position   int
}

// clone returns a deep copy of the CombatantState.
func (c *CombatantState) clone() *CombatantState {
	cp := *c
	if c.Conditions != nil {
		cp.Conditions = make([]string, len(c.Conditions))
		copy(cp.Conditions, c.Conditions)
	}
	return &cp
}

// CombatRenderSnapshot is a point-in-time copy of combat state for rendering.
type CombatRenderSnapshot struct {
	Round      int
	TurnOrder  []string
	Combatants map[string]*CombatantState
	Log        []string
	Summary    string
	PlayerName string
	// REQ-61-1: DurationMs > 0 causes a countdown bar to be rendered.
	DurationMs int
	// ElapsedMs is milliseconds elapsed since round start, used to fill the bar.
	ElapsedMs int
}

// CombatModeHandler implements ModeHandler for the combat display.
// REQ-IMR-19.
type CombatModeHandler struct {
	mu             sync.Mutex
	playerName     string
	onExitFn       func()
	round          int
	maxAP          int
	durationMs     int
	roundStartedAt time.Time
	turnOrder      []string
	combatants     map[string]*CombatantState
	gridPositions  map[string]combatGridCoord
	log            []string
	summary        string
}

// NewCombatModeHandler constructs a CombatModeHandler.
func NewCombatModeHandler(playerName string, onExitFn func()) *CombatModeHandler {
	return &CombatModeHandler{
		playerName:    playerName,
		onExitFn:      onExitFn,
		combatants:    make(map[string]*CombatantState),
		gridPositions: make(map[string]combatGridCoord),
	}
}

// Mode returns ModeCombat. REQ-IMR-3.
func (h *CombatModeHandler) Mode() InputMode { return ModeCombat }

// Prompt returns the combat prompt string. REQ-IMR-7.
func (h *CombatModeHandler) Prompt() string { return "[combat]> " }

// Round returns the current combat round (thread-safe).
func (h *CombatModeHandler) Round() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.round
}

// Combatants returns snapshot copies of all combatants in turn order.
func (h *CombatModeHandler) Combatants() []*CombatantState {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]*CombatantState, 0, len(h.turnOrder))
	for _, name := range h.turnOrder {
		if c, ok := h.combatants[name]; ok {
			result = append(result, c.clone())
		}
	}
	return result
}

// CombatantByName returns a snapshot copy of the named combatant, or nil.
func (h *CombatModeHandler) CombatantByName(name string) *CombatantState {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.combatants[name]; ok {
		return c.clone()
	}
	return nil
}

// UpdateRoundStart updates the round number, resets AP, adds new combatants,
// and marks missing combatants as dead.
// REQ-61-1: durationMs is stored so SnapshotForRender can compute ElapsedMs.
func (h *CombatModeHandler) UpdateRoundStart(round, actionsPerTurn, durationMs int, turnOrder []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.round = round
	h.maxAP = actionsPerTurn
	h.durationMs = durationMs
	h.roundStartedAt = time.Now()

	// Mark combatants not in the new turn order as dead.
	present := make(map[string]bool, len(turnOrder))
	for _, name := range turnOrder {
		present[name] = true
	}
	for name, c := range h.combatants {
		if !present[name] {
			c.IsDead = true
		}
	}

	// Add new combatants and reset AP for all living combatants.
	for _, name := range turnOrder {
		c, ok := h.combatants[name]
		if !ok {
			c = &CombatantState{
				Name:     name,
				HP:       -1,
				MaxHP:    -1,
				IsPlayer: name == h.playerName,
			}
			h.combatants[name] = c
		}
		c.AP = actionsPerTurn
		c.MaxAP = actionsPerTurn
		c.IsCurrent = false
	}
	// Mark the first combatant in turn order as the current actor.
	if len(turnOrder) > 0 {
		if c, ok := h.combatants[turnOrder[0]]; ok {
			c.IsCurrent = true
		}
	}

	h.turnOrder = make([]string, len(turnOrder))
	copy(h.turnOrder, turnOrder)
}

// UpdateCombatEvent updates the target's HP/MaxHP and appends the narrative to the log.
func (h *CombatModeHandler) UpdateCombatEvent(attacker, target string, damage, targetHP, targetMaxHP int, narrative string, eventType int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.combatants[target]; ok {
		c.HP = targetHP
		if targetMaxHP > 0 {
			c.MaxHP = targetMaxHP
		}
		if targetHP <= 0 {
			c.IsDead = true
		}
	}
	if narrative != "" {
		h.log = append(h.log, narrative)
		if len(h.log) > maxCombatLogLines {
			h.log = h.log[len(h.log)-maxCombatLogLines:]
		}
	}
}

// UpdatePosition updates a combatant's 1D position (thread-safe).
func (h *CombatModeHandler) UpdatePosition(name string, pos int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.combatants[name]; ok {
		c.Position = pos
	}
}

// UpdatePosition2D updates a combatant's 2D grid coordinates (thread-safe).
//
// Precondition: name must be non-empty; x and y must be within [0, 9].
// Postcondition: gridPositions[name] is updated to {X: x, Y: y}.
func (h *CombatModeHandler) UpdatePosition2D(name string, x, y int32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gridPositions[name] = combatGridCoord{X: x, Y: y}
}

// SetInitialPositions seeds the grid position map from initial_positions.
//
// Precondition: positions may be nil or empty.
// Postcondition: gridPositions is replaced with the supplied positions.
func (h *CombatModeHandler) SetInitialPositions(positions []*gamev1.CombatantPosition) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gridPositions = make(map[string]combatGridCoord, len(positions))
	for _, p := range positions {
		h.gridPositions[p.GetName()] = combatGridCoord{X: p.GetX(), Y: p.GetY()}
	}
}

// GridPositions returns a snapshot of all combatant 2D positions as proto messages.
//
// Precondition: none.
// Postcondition: returned slice length equals len(gridPositions).
func (h *CombatModeHandler) GridPositions() []*gamev1.CombatantPosition {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]*gamev1.CombatantPosition, 0, len(h.gridPositions))
	for name, coord := range h.gridPositions {
		result = append(result, &gamev1.CombatantPosition{
			Name: name,
			X:    coord.X,
			Y:    coord.Y,
		})
	}
	return result
}

// UpdatePlayerHP updates the player combatant's HP and MaxHP.
func (h *CombatModeHandler) UpdatePlayerHP(current, max int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.combatants[h.playerName]
	if !ok {
		c = &CombatantState{Name: h.playerName, IsPlayer: true}
		h.combatants[h.playerName] = c
	}
	c.HP = current
	c.MaxHP = max
}

// UpdateConditions replaces the player combatant's condition list.
func (h *CombatModeHandler) UpdateConditions(conditions []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.combatants[h.playerName]; ok {
		c.Conditions = make([]string, len(conditions))
		copy(c.Conditions, conditions)
	}
}

// SetSummary sets the combat summary text.
func (h *CombatModeHandler) SetSummary(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.summary = text
}

// Summary returns the current combat summary text (thread-safe).
func (h *CombatModeHandler) Summary() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.summary
}

// SnapshotForRender returns a point-in-time copy of combat state for rendering.
func (h *CombatModeHandler) SnapshotForRender() CombatRenderSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	elapsedMs := 0
	if h.durationMs > 0 && !h.roundStartedAt.IsZero() {
		elapsedMs = int(time.Since(h.roundStartedAt).Milliseconds())
		if elapsedMs > h.durationMs {
			elapsedMs = h.durationMs
		}
	}
	snap := CombatRenderSnapshot{
		Round:      h.round,
		PlayerName: h.playerName,
		Summary:    h.summary,
		TurnOrder:  make([]string, len(h.turnOrder)),
		Combatants: make(map[string]*CombatantState, len(h.combatants)),
		Log:        make([]string, len(h.log)),
		DurationMs: h.durationMs,
		ElapsedMs:  elapsedMs,
	}
	copy(snap.TurnOrder, h.turnOrder)
	copy(snap.Log, h.log)
	for k, v := range h.combatants {
		snap.Combatants[k] = v.clone()
	}
	return snap
}

// OnEnter renders the combat screen. REQ-IMR-4.
func (h *CombatModeHandler) OnEnter(conn *telnet.Conn) {
	if conn == nil {
		return
	}
	snap := h.SnapshotForRender()
	width, _ := conn.Dimensions()
	screen := RenderCombatScreen(snap, width)
	if conn.IsSplitScreen() {
		_ = conn.WriteRoom(screen)
	} else {
		_ = conn.WriteLine(screen)
	}
}

// OnExit is a no-op for combat mode. REQ-IMR-5.
func (h *CombatModeHandler) OnExit(conn *telnet.Conn) {}

// HandleInput processes one line of combat-mode input.
// Movement commands are blocked; other commands are forwarded by the command loop.
// REQ-IMR-6.
func (h *CombatModeHandler) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState) {
	if isMovementCommand(line) {
		msg := "You can't move while in combat!"
		if conn.IsSplitScreen() {
			_ = conn.WriteConsole(msg)
			_ = conn.WritePromptSplit(h.Prompt())
		} else {
			_ = conn.WriteLine(msg)
			_ = conn.WritePrompt(h.Prompt())
		}
		return
	}
}

// hpBar renders an HP bar: [####....] 20/30
func hpBar(current, max, width int) string {
	if max <= 0 {
		return fmt.Sprintf("[%s] %d/%d", strings.Repeat("?", width), current, max)
	}
	if current < 0 {
		current = 0
	}
	filled := (current * width) / max
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s] %d/%d", strings.Repeat("#", filled), strings.Repeat(".", empty), current, max)
}

// apDots renders AP as filled/empty dots: ●●○○
func apDots(current, max int) string {
	if max <= 0 {
		return ""
	}
	if current < 0 {
		current = 0
	}
	if current > max {
		current = max
	}
	return strings.Repeat("●", current) + strings.Repeat("○", max-current)
}

// Reset clears all combat state for a fresh engagement.
func (h *CombatModeHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.round = 0
	h.maxAP = 0
	h.durationMs = 0
	h.roundStartedAt = time.Time{}
	h.turnOrder = nil
	h.combatants = make(map[string]*CombatantState)
	h.gridPositions = make(map[string]combatGridCoord)
	h.log = nil
	h.summary = ""
}
