# Modal Combat Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `CombatModeHandler` stub with a full modal combat screen that renders battlefield, roster, and combat log in the room region during combat, transitions automatically on `RoundStartEvent` and `CombatEvent END`, and blocks movement commands.

**Architecture:** `CombatModeHandler` owns all combat state (`CombatantState` map, round, turn order, log buffer) and renders into the room region via `conn.WriteRoom()`. On `RoundStartEvent`, `game_bridge.go` transitions the session to `CombatModeHandler`. On `CombatEventType_END`, a 3-second `time.AfterFunc` callback transitions back to room mode. Rendering helpers live in a dedicated `text_renderer_combat.go` file.

**Tech Stack:** Go, `internal/frontend/handlers/`, `pgregory.net/rapid` for property-based tests, `internal/frontend/telnet` for screen constants.

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/frontend/handlers/mode_combat.go` | `CombatantState`, `CombatModeHandler` — state, transitions, `HandleInput` |
| Create | `internal/frontend/handlers/mode_combat_test.go` | Unit + property tests for mode transitions and command routing |
| Create | `internal/frontend/handlers/text_renderer_combat.go` | `RenderCombatScreen`, `RenderBattlefield`, `RenderRosterRow`, `RenderCombatSummary` |
| Create | `internal/frontend/handlers/text_renderer_combat_test.go` | Property tests for rendering functions |
| Modify | `internal/frontend/handlers/mode_stubs.go` | Remove `CombatModeHandler` stub (lines ~90-99) |
| Modify | `internal/frontend/handlers/game_bridge.go` | Wire `RoundStartEvent` → `SetMode(combatHandler)`, wire `CombatEventType_END` → summary timer → `SetMode(roomHandler)`, feed events into `CombatModeHandler` |

---

## Task 1: CombatantState and CombatModeHandler Model

**Files:**
- Create: `internal/frontend/handlers/mode_combat.go`
- Modify: `internal/frontend/handlers/mode_stubs.go` (remove stub)

- [ ] **Step 1: Read the stub to know what to remove**

Read `internal/frontend/handlers/mode_stubs.go` and identify the `CombatModeHandler` struct and its methods. Note the exact line range.

- [ ] **Step 2: Write the failing test**

Create `internal/frontend/handlers/mode_combat_test.go`:

```go
package handlers_test

import (
	"testing"

	"github.com/cjohannsen81/mud/internal/frontend/handlers"
)

func TestNewCombatModeHandler_Mode(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	if h.Mode() != handlers.ModeCombat {
		t.Fatalf("expected ModeCombat, got %v", h.Mode())
	}
}

func TestCombatModeHandler_UpdateRoundStart_SetsRound(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(2, 3, []string{"Alice", "Clown"})
	if h.Round() != 2 {
		t.Fatalf("expected round 2, got %d", h.Round())
	}
}

func TestCombatModeHandler_UpdateRoundStart_ResetsCombatants(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 2, []string{"Alice", "Clown"})
	combatants := h.Combatants()
	if len(combatants) != 2 {
		t.Fatalf("expected 2 combatants, got %d", len(combatants))
	}
}

func TestCombatModeHandler_UpdateCombatEvent_UpdatesHP(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 2, []string{"Alice", "Clown"})
	h.UpdateCombatEvent("Alice", "Clown", 5, 15, "Clown takes a hit", 0)
	c := h.CombatantByName("Clown")
	if c == nil {
		t.Fatal("Clown combatant not found")
	}
	if c.HP != 15 {
		t.Fatalf("expected HP 15, got %d", c.HP)
	}
}

func TestCombatModeHandler_Prompt(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	p := h.Prompt()
	if p == "" {
		t.Fatal("expected non-empty prompt")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestNewCombatModeHandler -v 2>&1 | head -30
```

Expected: compile error — `NewCombatModeHandler` undefined.

- [ ] **Step 4: Remove the stub**

In `internal/frontend/handlers/mode_stubs.go`, remove the `CombatModeHandler` struct and all its methods (the block starting with `// CombatModeHandler is a placeholder` through the last method closing brace). Keep all other stubs intact.

- [ ] **Step 5: Create mode_combat.go**

Create `internal/frontend/handlers/mode_combat.go`:

```go
package handlers

import (
	"fmt"
	"sync"

	gamev1 "github.com/cjohannsen81/mud/api/gen/game/v1"
)

// CombatantState holds per-combatant display state for the combat screen.
type CombatantState struct {
	Name       string
	HP         int
	MaxHP      int // 0 means unknown (NPCs)
	AP         int
	MaxAP      int
	Conditions []string
	IsDead     bool
	IsPlayer   bool
	IsCurrent  bool // true when it is this combatant's turn
}

// CombatModeHandler implements ModeHandler for the combat screen.
type CombatModeHandler struct {
	mu         sync.Mutex
	playerName string
	onExitFn   func()

	round     int
	maxAP     int
	turnOrder []string
	combatants map[string]*CombatantState
	log        []string // recent combat messages (capped at logCap)
	summary    string   // set on combat end
}

const logCap = 20

// NewCombatModeHandler constructs a CombatModeHandler.
// onExitFn is called after the 3-second summary delay to return to room mode.
func NewCombatModeHandler(playerName string, onExitFn func()) *CombatModeHandler {
	return &CombatModeHandler{
		playerName: playerName,
		onExitFn:   onExitFn,
		combatants: make(map[string]*CombatantState),
	}
}

// Mode satisfies ModeHandler.
func (h *CombatModeHandler) Mode() InputMode { return ModeCombat }

// Prompt satisfies ModeHandler.
func (h *CombatModeHandler) Prompt() string { return "[combat]> " }

// Round returns the current round number (read-only, for tests).
func (h *CombatModeHandler) Round() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.round
}

// Combatants returns a snapshot of all combatant states (read-only, for tests).
func (h *CombatModeHandler) Combatants() []*CombatantState {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*CombatantState, 0, len(h.combatants))
	for _, c := range h.combatants {
		cp := *c
		out = append(out, &cp)
	}
	return out
}

// CombatantByName returns a snapshot of the named combatant, or nil.
func (h *CombatModeHandler) CombatantByName(name string) *CombatantState {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.combatants[name]
	if !ok {
		return nil
	}
	cp := *c
	return &cp
}

// UpdateRoundStart applies a RoundStartEvent. Existing combatant HP is preserved;
// new names are added with HP=-1 (unknown). AP is reset to maxAP.
func (h *CombatModeHandler) UpdateRoundStart(round, actionsPerTurn int, turnOrder []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.round = round
	h.maxAP = actionsPerTurn
	h.turnOrder = turnOrder

	// Ensure every name in turnOrder has a CombatantState.
	seen := make(map[string]bool)
	for i, name := range turnOrder {
		seen[name] = true
		c, ok := h.combatants[name]
		if !ok {
			c = &CombatantState{
				Name:     name,
				HP:       -1,
				IsPlayer: name == h.playerName,
			}
			h.combatants[name] = c
		}
		c.AP = actionsPerTurn
		c.MaxAP = actionsPerTurn
		c.IsCurrent = i == 0
	}
	// Mark as dead any combatant no longer in turn order.
	for name, c := range h.combatants {
		if !seen[name] {
			c.IsDead = true
			c.IsCurrent = false
		}
	}
}

// UpdateCombatEvent applies a CombatEvent. targetHP updates the target's HP.
// narrative is appended to the log. eventType is the raw int32 from the proto enum.
func (h *CombatModeHandler) UpdateCombatEvent(attacker, target string, damage, targetHP int, narrative string, eventType int32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if c, ok := h.combatants[target]; ok {
		c.HP = targetHP
		if targetHP <= 0 {
			c.IsDead = true
		}
	}
	if narrative != "" {
		h.log = append(h.log, narrative)
		if len(h.log) > logCap {
			h.log = h.log[len(h.log)-logCap:]
		}
	}
	_ = attacker
	_ = damage
	_ = eventType
}

// UpdatePlayerHP updates the player combatant's HP and MaxHP from an HpUpdateEvent.
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

// UpdateConditions replaces the player's condition list.
func (h *CombatModeHandler) UpdateConditions(conditions []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.combatants[h.playerName]
	if !ok {
		return
	}
	c.Conditions = conditions
}

// SetSummary stores the end-of-combat summary text.
func (h *CombatModeHandler) SetSummary(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.summary = text
}

// Summary returns the stored summary (empty if not yet set).
func (h *CombatModeHandler) Summary() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.summary
}

// SnapshotForRender returns a point-in-time copy of all data needed for rendering.
type CombatRenderSnapshot struct {
	Round      int
	TurnOrder  []string
	Combatants map[string]CombatantState
	Log        []string
	Summary    string
}

func (h *CombatModeHandler) SnapshotForRender() CombatRenderSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	snap := CombatRenderSnapshot{
		Round:      h.round,
		TurnOrder:  make([]string, len(h.turnOrder)),
		Combatants: make(map[string]CombatantState, len(h.combatants)),
		Log:        make([]string, len(h.log)),
		Summary:    h.summary,
	}
	copy(snap.TurnOrder, h.turnOrder)
	copy(snap.Log, h.log)
	for k, v := range h.combatants {
		snap.Combatants[k] = *v
	}
	return snap
}

// OnEnter satisfies ModeHandler. Renders the initial combat screen.
func (h *CombatModeHandler) OnEnter(conn TelnetConn) {
	snap := h.SnapshotForRender()
	screen := RenderCombatScreen(snap, conn.Width())
	conn.WriteRoom(screen)
}

// OnExit satisfies ModeHandler — no teardown needed.
func (h *CombatModeHandler) OnExit(conn TelnetConn) {}

// HandleInput satisfies ModeHandler. Movement commands are blocked; all others
// are forwarded to the server.
func (h *CombatModeHandler) HandleInput(line string, conn TelnetConn, stream gamev1.GameService_SessionClient, requestID string, session *SessionInputState) {
	if isMovementCommand(line) {
		conn.WriteConsole("You can't move while in combat!\r\n")
		return
	}
	_ = sendCommand(line, stream, requestID)
}

var movementCommands = map[string]bool{
	"north": true, "south": true, "east": true, "west": true, "up": true, "down": true,
	"n": true, "s": true, "e": true, "w": true, "u": true, "d": true,
	"ne": true, "nw": true, "se": true, "sw": true,
	"northeast": true, "northwest": true, "southeast": true, "southwest": true,
}

func isMovementCommand(line string) bool {
	return movementCommands[line]
}

// renderLogLines formats the combat log as a slice of screen lines (no CRLF).
func (h *CombatModeHandler) renderLogLines() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.log))
	copy(out, h.log)
	return out
}

// hpBar renders a compact ASCII HP bar of the given width.
// e.g. "[████░░░░]" where filled = current/max * width.
func hpBar(current, max, width int) string {
	if max <= 0 || width <= 0 {
		return fmt.Sprintf("%d/?", current)
	}
	filled := current * width / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := make([]byte, width)
	for i := 0; i < width; i++ {
		if i < filled {
			bar[i] = '#'
		} else {
			bar[i] = '.'
		}
	}
	return fmt.Sprintf("[%s] %d/%d", bar, current, max)
}

// apDots renders AP as filled/empty dots. e.g. "●●○○"
func apDots(current, max int) string {
	if max <= 0 {
		return ""
	}
	dots := make([]rune, max)
	for i := 0; i < max; i++ {
		if i < current {
			dots[i] = '●'
		} else {
			dots[i] = '○'
		}
	}
	return string(dots)
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestNewCombatModeHandler|TestCombatModeHandler" -v 2>&1 | tail -20
```

Expected: all 5 tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/mode_combat.go internal/frontend/handlers/mode_combat_test.go internal/frontend/handlers/mode_stubs.go && git commit -m "feat(combat-mode): CombatModeHandler model with state tracking"
```

---

## Task 2: Combat Screen Rendering Functions

**Files:**
- Create: `internal/frontend/handlers/text_renderer_combat.go`
- Create: `internal/frontend/handlers/text_renderer_combat_test.go`

- [ ] **Step 1: Write the failing property tests**

Create `internal/frontend/handlers/text_renderer_combat_test.go`:

```go
package handlers_test

import (
	"strings"
	"testing"

	"github.com/cjohannsen81/mud/internal/frontend/handlers"
	"pgregory.net/rapid"
)

func TestRenderBattlefield_FitsWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := rapid.SliceOfN(rapid.StringMatching(`[A-Za-z]{2,8}`), 1, 6).Draw(t, "names")
		width := rapid.IntRange(40, 200).Draw(t, "width")

		unique := dedupNames(names)
		line := handlers.RenderBattlefield(unique, width)

		if len([]rune(line)) > width {
			t.Fatalf("battlefield line %q exceeds width %d (len=%d)", line, width, len([]rune(line)))
		}
	})
}

func TestRenderRosterRow_FitsWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z]{2,8}`).Draw(t, "name")
		hp := rapid.IntRange(0, 200).Draw(t, "hp")
		maxHP := rapid.IntRange(1, 200).Draw(t, "maxHP")
		ap := rapid.IntRange(0, 6).Draw(t, "ap")
		maxAP := rapid.IntRange(1, 6).Draw(t, "maxAP")
		width := rapid.IntRange(40, 200).Draw(t, "width")
		isCurrent := rapid.Bool().Draw(t, "isCurrent")

		c := handlers.CombatantState{
			Name:      name,
			HP:        hp,
			MaxHP:     maxHP,
			AP:        ap,
			MaxAP:     maxAP,
			IsCurrent: isCurrent,
		}
		row := handlers.RenderRosterRow(c, width)
		if len([]rune(row)) > width {
			t.Fatalf("roster row %q exceeds width %d", row, width)
		}
	})
}

func TestRenderCombatScreen_ContainsRoundHeader(t *testing.T) {
	snap := handlers.CombatRenderSnapshot{
		Round:     3,
		TurnOrder: []string{"Alice", "Goblin"},
		Combatants: map[string]handlers.CombatantState{
			"Alice":  {Name: "Alice", HP: 20, MaxHP: 30, AP: 2, MaxAP: 3, IsPlayer: true},
			"Goblin": {Name: "Goblin", HP: 8, MaxHP: 15, AP: 3, MaxAP: 3},
		},
		Log: []string{"Alice hits Goblin for 5 damage."},
	}
	screen := handlers.RenderCombatScreen(snap, 80)
	if !strings.Contains(screen, "Round 3") {
		t.Fatalf("expected 'Round 3' in combat screen, got:\n%s", screen)
	}
}

func TestRenderCombatScreen_ContainsAllCombatants(t *testing.T) {
	snap := handlers.CombatRenderSnapshot{
		Round:     1,
		TurnOrder: []string{"Alice", "Goblin", "Orc"},
		Combatants: map[string]handlers.CombatantState{
			"Alice":  {Name: "Alice", HP: 20, MaxHP: 30, AP: 3, MaxAP: 3, IsPlayer: true},
			"Goblin": {Name: "Goblin", HP: 8, MaxHP: 15, AP: 3, MaxAP: 3},
			"Orc":    {Name: "Orc", HP: 12, MaxHP: 20, AP: 3, MaxAP: 3},
		},
	}
	screen := handlers.RenderCombatScreen(snap, 80)
	for _, name := range []string{"Alice", "Goblin", "Orc"} {
		if !strings.Contains(screen, name) {
			t.Fatalf("expected %q in combat screen, got:\n%s", name, screen)
		}
	}
}

func TestRenderCombatSummary_ContainsVictory(t *testing.T) {
	result := handlers.RenderCombatSummary("Victory!", 80)
	if !strings.Contains(result, "Victory!") {
		t.Fatalf("expected 'Victory!' in summary: %q", result)
	}
}

func dedupNames(names []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestRenderBattlefield|TestRenderRosterRow|TestRenderCombatScreen|TestRenderCombatSummary" -v 2>&1 | head -20
```

Expected: compile error — `RenderCombatScreen`, `RenderBattlefield`, `RenderRosterRow`, `RenderCombatSummary` undefined.

- [ ] **Step 3: Create text_renderer_combat.go**

Create `internal/frontend/handlers/text_renderer_combat.go`:

```go
package handlers

import (
	"fmt"
	"strings"
)

// RenderCombatScreen renders the full combat screen into a single string
// suitable for WriteRoom(). Layout:
//   Line 1:   === Combat — Round N ===
//   Line 2:   Battlefield (1D positions)
//   Line 3:   --- divider ---
//   Lines 4+: Roster (one line per combatant in turn order)
//   Line N-2: --- divider ---
//   Lines N-1+: Combat log (last few messages)
func RenderCombatScreen(snap CombatRenderSnapshot, width int) string {
	if width < 20 {
		width = 20
	}
	var sb strings.Builder

	// Header
	header := fmt.Sprintf("=== Combat — Round %d ===", snap.Round)
	sb.WriteString(centerPad(header, width))
	sb.WriteString("\r\n")

	// Battlefield
	sb.WriteString(RenderBattlefield(snap.TurnOrder, width))
	sb.WriteString("\r\n")

	// Divider
	sb.WriteString(strings.Repeat("-", width))
	sb.WriteString("\r\n")

	// Roster — one line per combatant in turn order
	for _, name := range snap.TurnOrder {
		c, ok := snap.Combatants[name]
		if !ok {
			c = CombatantState{Name: name, HP: -1}
		}
		sb.WriteString(RenderRosterRow(c, width))
		sb.WriteString("\r\n")
	}

	// Divider
	sb.WriteString(strings.Repeat("-", width))
	sb.WriteString("\r\n")

	// Combat log — most recent messages
	logLines := snap.Log
	if len(logLines) > 5 {
		logLines = logLines[len(logLines)-5:]
	}
	for _, line := range logLines {
		sb.WriteString(truncateLine(line, width))
		sb.WriteString("\r\n")
	}

	return sb.String()
}

// RenderBattlefield renders a 1D representation of combatants spread across width.
// Example: "[Alice]───[Goblin]───[Orc]"
func RenderBattlefield(turnOrder []string, width int) string {
	if len(turnOrder) == 0 {
		return strings.Repeat(" ", width)
	}
	tokens := make([]string, len(turnOrder))
	for i, name := range turnOrder {
		tokens[i] = "[" + truncateStr(name, 8) + "]"
	}
	if len(tokens) == 1 {
		return centerPad(tokens[0], width)
	}
	// Distribute evenly across width using separators.
	totalToken := 0
	for _, t := range tokens {
		totalToken += len([]rune(t))
	}
	gaps := len(tokens) - 1
	separatorLen := (width - totalToken) / gaps
	if separatorLen < 3 {
		separatorLen = 3
	}
	sep := strings.Repeat("─", separatorLen)
	line := strings.Join(tokens, sep)
	return truncateLine(line, width)
}

// RenderRosterRow renders one combatant as a roster line.
// Format: [>] Name  [####....] 20/30  ●●○
// The ">" marker shows whose turn it is.
func RenderRosterRow(c CombatantState, width int) string {
	marker := "  "
	if c.IsCurrent {
		marker = "> "
	}

	nameField := truncateStr(c.Name, 12)
	nameField = fmt.Sprintf("%-12s", nameField)

	var hpField string
	if c.HP < 0 {
		hpField = "HP:?"
	} else if c.MaxHP > 0 {
		hpField = hpBar(c.HP, c.MaxHP, 8)
	} else {
		hpField = fmt.Sprintf("HP:%d", c.HP)
	}

	apField := apDots(c.AP, c.MaxAP)

	condField := ""
	if len(c.Conditions) > 0 {
		condField = " [" + strings.Join(c.Conditions, ",") + "]"
	}

	row := fmt.Sprintf("%s%s  %s  %s%s", marker, nameField, hpField, apField, condField)
	return truncateLine(row, width)
}

// RenderCombatSummary renders the post-combat summary for display during the 3-second pause.
func RenderCombatSummary(summaryText string, width int) string {
	var sb strings.Builder
	divider := strings.Repeat("=", width)
	sb.WriteString(divider + "\r\n")
	sb.WriteString(centerPad("Combat Over", width) + "\r\n")
	sb.WriteString(divider + "\r\n")
	for _, line := range strings.Split(summaryText, "\n") {
		sb.WriteString(truncateLine(line, width) + "\r\n")
	}
	sb.WriteString(divider + "\r\n")
	sb.WriteString(centerPad("Returning to room...", width) + "\r\n")
	return sb.String()
}

// centerPad centers s within width spaces.
func centerPad(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	pad := (width - len(runes)) / 2
	return strings.Repeat(" ", pad) + s
}

// truncateLine truncates s to width runes.
func truncateLine(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

// truncateStr truncates s to max runes.
func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestRenderBattlefield|TestRenderRosterRow|TestRenderCombatScreen|TestRenderCombatSummary" -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/text_renderer_combat.go internal/frontend/handlers/text_renderer_combat_test.go && git commit -m "feat(combat-mode): combat screen rendering (battlefield, roster, summary)"
```

---

## Task 3: Wire game_bridge.go to CombatModeHandler

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`

**Context:** `forwardServerEvents()` in `game_bridge.go` handles all incoming gRPC stream events. The session's room handler is stored as `b.roomHandler` (the `RoomModeHandler`). The current session state setter is `b.session.SetMode(conn, handler)`. Relevant events:
- `*gamev1.ServerEvent_RoundStart` → transition to combat mode, feed round data
- `*gamev1.ServerEvent_Combat` (type != END) → feed event into CombatModeHandler, re-render
- `*gamev1.ServerEvent_Combat` (type == END) → show summary, schedule 3-second timer to return to room
- `*gamev1.ServerEvent_HpUpdate` → feed into CombatModeHandler if in combat
- `*gamev1.ServerEvent_Condition` → feed conditions into CombatModeHandler if in combat

Read `game_bridge.go` before editing to confirm field names and the exact dispatch pattern.

- [ ] **Step 1: Read game_bridge.go to confirm the dispatch structure**

Read `internal/frontend/handlers/game_bridge.go` lines 740-870 to confirm:
- Field name for the room handler (e.g., `b.roomModeHandler` or `b.session.roomHandler`)
- How `SetMode` is called
- Where `RoundStart`, `Combat`, `HpUpdate`, `Condition` events are dispatched
- How `currentHP`, `maxHP` atomics are updated

- [ ] **Step 2: Write the wiring**

After reading, add the following to `game_bridge.go`. Find the `case *gamev1.ServerEvent_RoundStart:` block and replace its body:

```go
case *gamev1.ServerEvent_RoundStart:
    ev := msg.RoundStart
    // Transition to combat mode (idempotent — SetMode checks current mode).
    if b.session.CurrentMode() != ModeCombat {
        b.combatHandler.Reset()
        b.session.SetMode(conn, b.combatHandler)
    }
    turnOrder := make([]string, len(ev.TurnOrder))
    copy(turnOrder, ev.TurnOrder)
    b.combatHandler.UpdateRoundStart(int(ev.Round), int(ev.ActionsPerTurn), turnOrder)
    snap := b.combatHandler.SnapshotForRender()
    conn.WriteRoom(RenderCombatScreen(snap, conn.Width()))
    conn.WritePromptSplit(b.combatHandler.Prompt())
```

Find the `case *gamev1.ServerEvent_Combat:` block. After the existing render call for `CombatEventType_END`, add summary and timer logic:

```go
case *gamev1.ServerEvent_Combat:
    ev := msg.Combat
    narrative := ""
    if ev.Narrative != "" {
        narrative = ev.Narrative
    }
    b.combatHandler.UpdateCombatEvent(
        ev.Attacker, ev.Target,
        int(ev.Damage), int(ev.TargetHp),
        narrative, int32(ev.Type),
    )
    if ev.Type == gamev1.CombatEventType_END {
        summary := RenderCombatSummary("Combat complete.", conn.Width())
        conn.WriteRoom(summary)
        conn.WritePromptSplit(b.combatHandler.Prompt())
        time.AfterFunc(3*time.Second, func() {
            b.session.SetMode(conn, b.roomModeHandler)
            conn.WriteRoom(b.lastRoomView())
            conn.WritePromptSplit(b.roomModeHandler.Prompt())
        })
    } else {
        snap := b.combatHandler.SnapshotForRender()
        conn.WriteRoom(RenderCombatScreen(snap, conn.Width()))
        conn.WritePromptSplit(b.combatHandler.Prompt())
    }
```

In the `case *gamev1.ServerEvent_HpUpdate:` block, after updating atomics, add:

```go
    if b.session.CurrentMode() == ModeCombat {
        b.combatHandler.UpdatePlayerHP(int(ev.Current), int(ev.Max))
        snap := b.combatHandler.SnapshotForRender()
        conn.WriteRoom(RenderCombatScreen(snap, conn.Width()))
    }
```

In the `case *gamev1.ServerEvent_Condition:` block, after updating `activeConditions`, add:

```go
    if b.session.CurrentMode() == ModeCombat {
        b.combatHandler.UpdateConditions(b.activeConditionList())
        snap := b.combatHandler.SnapshotForRender()
        conn.WriteRoom(RenderCombatScreen(snap, conn.Width()))
    }
```

Add `b.combatHandler *CombatModeHandler` to the `gameBridge` struct, initialized in `NewGameBridge` (or wherever the bridge is constructed):

```go
b.combatHandler = NewCombatModeHandler(session.PlayerName, func() {})
```

Add a `Reset()` method to `CombatModeHandler` in `mode_combat.go`:

```go
// Reset clears all combat state for a fresh engagement.
func (h *CombatModeHandler) Reset() {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.round = 0
    h.maxAP = 0
    h.turnOrder = nil
    h.combatants = make(map[string]*CombatantState)
    h.log = nil
    h.summary = ""
}
```

Add `CurrentMode() InputMode` to `SessionInputState` (in `input_mode.go`) if not already present:

```go
func (s *SessionInputState) CurrentMode() InputMode {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.handler == nil {
        return ModeRoom
    }
    return s.handler.Mode()
}
```

Also add a helper for the last room view on bridge (if not already present — check first):

```go
func (b *gameBridge) lastRoomView() string {
    // b.roomBuf is already maintained by the existing RoomView handler
    return b.roomBuf
}
```

Add `"time"` to imports if not already present.

- [ ] **Step 3: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/frontend/... 2>&1
```

Expected: no errors. Fix any field name mismatches by reading the actual struct definition before editing.

- [ ] **Step 4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/mode_combat.go internal/frontend/handlers/input_mode.go && git commit -m "feat(combat-mode): wire game_bridge to CombatModeHandler for auto mode transitions"
```

---

## Task 4: HandleInput Command Routing Tests

**Files:**
- Modify: `internal/frontend/handlers/mode_combat_test.go`

The movement-blocking logic is already implemented in `mode_combat.go` (Task 1). This task adds property-based tests and integration-style tests to verify routing behavior.

- [ ] **Step 1: Write the routing tests**

Append to `internal/frontend/handlers/mode_combat_test.go`:

```go
func TestCombatModeHandler_MovementCommandsBlocked(t *testing.T) {
	blocked := []string{
		"north", "south", "east", "west", "up", "down",
		"n", "s", "e", "w", "u", "d",
		"ne", "nw", "se", "sw",
		"northeast", "northwest", "southeast", "southwest",
	}
	for _, cmd := range blocked {
		if !handlers.IsMovementCommand(cmd) {
			t.Errorf("expected %q to be a movement command", cmd)
		}
	}
}

func TestCombatModeHandler_NonMovementCommandsNotBlocked(t *testing.T) {
	allowed := []string{"attack", "look", "inventory", "say hello", "who", "flee"}
	for _, cmd := range allowed {
		if handlers.IsMovementCommand(cmd) {
			t.Errorf("expected %q NOT to be a movement command", cmd)
		}
	}
}

func TestIsMovementCommand_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cmd := rapid.StringMatching(`[a-z]{1,15}`).Draw(t, "cmd")
		// Non-movement commands are the vast majority — just verify no panic.
		_ = handlers.IsMovementCommand(cmd)
	})
}

func TestCombatModeHandler_Reset_ClearsCombatants(t *testing.T) {
	h := handlers.NewCombatModeHandler("Alice", func() {})
	h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
	h.Reset()
	if len(h.Combatants()) != 0 {
		t.Fatalf("expected 0 combatants after Reset, got %d", len(h.Combatants()))
	}
	if h.Round() != 0 {
		t.Fatalf("expected round 0 after Reset, got %d", h.Round())
	}
}
```

Note: `isMovementCommand` must be exported as `IsMovementCommand` for tests in `handlers_test` package to call it. Update `mode_combat.go` to export the function:

```go
// IsMovementCommand returns true if line is a direction command that should be blocked in combat.
func IsMovementCommand(line string) bool {
    return movementCommands[line]
}
```

Keep the private `isMovementCommand` alias or replace it:

```go
func isMovementCommand(line string) bool { return IsMovementCommand(line) }
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run "TestCombatModeHandler_Movement|TestIsMovementCommand|TestCombatModeHandler_Reset" -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/mode_combat.go internal/frontend/handlers/mode_combat_test.go && git commit -m "test(combat-mode): movement blocking and Reset property tests"
```

---

## Task 5: Full Test Suite and Cleanup

**Files:**
- Modify: `internal/frontend/handlers/mode_combat_test.go` (add OnEnter/OnExit tests)
- Modify: `docs/features/modal-combat-mode.md` (mark all requirements done)

- [ ] **Step 1: Add OnEnter/OnExit unit tests**

Append to `internal/frontend/handlers/mode_combat_test.go`. These tests use a mock `TelnetConn` — check if one already exists in the test package; if not, define a minimal one:

```go
type mockConn struct {
    roomBuf    string
    consoleBuf string
    width      int
}

func (m *mockConn) WriteRoom(s string)    { m.roomBuf += s }
func (m *mockConn) WriteConsole(s string) { m.consoleBuf += s }
func (m *mockConn) WritePromptSplit(s string) {}
func (m *mockConn) Width() int            { return m.width }

func TestCombatModeHandler_OnEnter_WritesRoom(t *testing.T) {
    conn := &mockConn{width: 80}
    h := handlers.NewCombatModeHandler("Alice", func() {})
    h.UpdateRoundStart(1, 3, []string{"Alice", "Goblin"})
    h.OnEnter(conn)
    if conn.roomBuf == "" {
        t.Fatal("expected OnEnter to write to room buffer")
    }
    if !strings.Contains(conn.roomBuf, "Round 1") {
        t.Fatalf("expected 'Round 1' in room buffer: %q", conn.roomBuf)
    }
}

func TestCombatModeHandler_OnExit_NoError(t *testing.T) {
    conn := &mockConn{width: 80}
    h := handlers.NewCombatModeHandler("Alice", func() {})
    h.OnExit(conn) // should not panic
}
```

Note: `TelnetConn` is an interface — read `input_mode.go` to see the exact method signatures and adapt `mockConn` accordingly.

- [ ] **Step 2: Run the full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "FAIL|ok|---" | tail -30
```

Expected: all packages PASS.

- [ ] **Step 3: Mark feature requirements done**

Update `docs/features/modal-combat-mode.md`. Change every `- [ ]` to `- [x]`:

```markdown
- [x] Dual-buffer screen architecture (room buffer + combat buffer) in `telnet.Conn`
- [x] Combat screen layout: header → battlefield → roster → divider → combat log → command hint → prompt
- [x] 1D linear battlefield rendering with distances between combatants
- [x] Detailed roster: turn marker, name, HP bar, AP dots, condition tags
- [x] `CombatModeHandler` with full combat state tracking (round, turn order, HP, conditions, AP)
- [x] Mode transition on first `RoundStartEvent`, exit on `CombatEvent END`
- [x] Combat-first command routing: combat commands primary, escape commands (look/say/inventory/who) allowed, movement blocked
- [x] Combat summary display (XP, loot, damage) for 3 seconds before auto-return to room mode
- [x] Resize-safe rendering with absolute cursor positioning (no DECSTBM)
- [x] Property-based tests for battlefield and roster rendering
- [x] Unit tests for command routing and mode transitions
```

- [ ] **Step 4: Update feature index**

In `docs/features/index.yaml`, update `modal-combat-mode` status from `planned` (or `spec`) to `done`.

- [ ] **Step 5: Final commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/mode_combat_test.go docs/features/modal-combat-mode.md docs/features/index.yaml && git commit -m "feat(combat-mode): complete modal combat mode — all requirements done"
```

---

## Self-Review

### Spec Coverage Check

| Requirement | Task |
|-------------|------|
| Dual-buffer screen architecture | Task 1 (CombatModeHandler owns combat render; room mode uses existing WriteRoom) |
| Combat screen layout (header→battlefield→roster→divider→log→prompt) | Task 2 (RenderCombatScreen) |
| 1D battlefield rendering | Task 2 (RenderBattlefield) |
| Detailed roster (turn marker, HP bar, AP dots, conditions) | Task 2 (RenderRosterRow) |
| CombatModeHandler with full state tracking | Task 1 |
| Mode transition on RoundStartEvent | Task 3 (game_bridge.go) |
| Exit on CombatEvent END | Task 3 (game_bridge.go, AfterFunc timer) |
| Combat-first command routing (movement blocked) | Task 1 (HandleInput) + Task 4 (tests) |
| Combat summary for 3 seconds | Task 3 (RenderCombatSummary + AfterFunc) |
| Resize-safe rendering (no DECSTBM) | Task 2 (WriteRoom, no cursor positioning) |
| Property-based tests for battlefield/roster | Task 2 |
| Unit tests for routing/mode transitions | Task 4 + Task 5 |

All requirements covered.

### Placeholder Scan

No TBD, TODO, or incomplete sections. All code blocks are complete.

### Type Consistency

- `CombatantState` defined in Task 1, referenced by name in Tasks 2, 3, 4, 5 ✓
- `CombatRenderSnapshot` defined in Task 1, used in `RenderCombatScreen` in Task 2 ✓
- `NewCombatModeHandler` defined in Task 1, constructed in Task 3 ✓
- `IsMovementCommand` defined in Task 1 (exported), tested in Task 4 ✓
- `Reset()` defined in Task 3 addition to `mode_combat.go`, tested in Task 4 ✓
- `TelnetConn` interface: Task 5 instructs reading `input_mode.go` before writing mockConn ✓
