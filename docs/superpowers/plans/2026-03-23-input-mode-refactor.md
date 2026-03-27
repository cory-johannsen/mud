# Input Mode Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `mapMode bool` flag with a `ModeHandler` interface and `SessionInputState` controller so all prompt rendering and input routing are centralized, fixing the map prompt overwrite bug and enabling future interactive modes.

**Architecture:** Define `InputMode int` + `ModeHandler` interface in `input_mode.go`. `SessionInputState` owns the active handler behind a `sync.RWMutex`. `commandLoop` delegates all input to `session.HandleInput`. `forwardServerEvents` always calls `session.CurrentPrompt()` for prompt writes — never `buildPrompt()` directly.

**Tech Stack:** Go 1.22+, `sync.RWMutex`, `pgregory.net/rapid` (property-based tests), `mise exec --` for all Go commands.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/frontend/handlers/input_mode.go` | Create | `InputMode` type, `ModeHandler` interface, `SessionInputState` |
| `internal/frontend/handlers/input_mode_test.go` | Create | Unit + property tests for `SessionInputState` |
| `internal/frontend/handlers/mode_room.go` | Create | `RoomModeHandler` — wraps existing room command dispatch |
| `internal/frontend/handlers/mode_map.go` | Create | `MapModeHandler` — refactored from `game_bridge_mapmode.go` |
| `internal/frontend/handlers/mode_stubs.go` | Create | `InventoryModeHandler`, `CharSheetModeHandler`, `EditorModeHandler`, `CombatModeHandler` stubs |
| `internal/frontend/handlers/game_bridge.go` | Modify | Remove `mapState`/`buildPrompt`; wire `SessionInputState`; fix all prompt writes |
| `internal/frontend/handlers/game_bridge_mapmode.go` | Delete | Replaced by `mode_map.go` |

---

## Task 1: Define `InputMode` and `ModeHandler` interface

**Files:**
- Create: `internal/frontend/handlers/input_mode.go`
- Create: `internal/frontend/handlers/input_mode_test.go`

- [ ] **Step 1: Write the failing test for `InputMode.String()`**

```go
// internal/frontend/handlers/input_mode_test.go
package handlers_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    "github.com/stretchr/testify/assert"
)

func TestInputMode_String(t *testing.T) {
    cases := []struct {
        mode handlers.InputMode
        want string
    }{
        {handlers.ModeRoom, "room"},
        {handlers.ModeMap, "map"},
        {handlers.ModeInventory, "inventory"},
        {handlers.ModeCharSheet, "charsheet"},
        {handlers.ModeEditor, "editor"},
        {handlers.ModeCombat, "combat"},
    }
    for _, tc := range cases {
        assert.Equal(t, tc.want, tc.mode.String())
    }
}
```

- [ ] **Step 2: Run — verify FAIL**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestInputMode_String -v 2>&1 | grep -E "FAIL|PASS|undefined"
```
Expected: compile error — `handlers.InputMode` undefined.

- [ ] **Step 3: Implement `input_mode.go`**

```go
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
// REQ-IMR-11.
func (s *SessionInputState) Room() *RoomModeHandler {
    return s.room
}

// HandleInput delegates the input line to the active handler.
func (s *SessionInputState) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int) {
    s.mu.RLock()
    h := s.current
    s.mu.RUnlock()
    h.HandleInput(line, conn, stream, requestID, s)
}
```

- [ ] **Step 4: Run — verify PASS**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestInputMode_String -v 2>&1 | tail -5
```
Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/handlers/input_mode.go internal/frontend/handlers/input_mode_test.go
git commit -m "feat(input-mode): define InputMode type, ModeHandler interface, SessionInputState (REQ-IMR-1..11)"
```

---

## Task 2: Unit + property tests for `SessionInputState`

**Files:**
- Modify: `internal/frontend/handlers/input_mode_test.go`

> **Note:** These tests reference `handlers.NewRoomModeHandlerForTest` and `room.ExitLog()` which are defined in Task 3 (`mode_room.go`). Write Task 3 first, then return here to run these tests. The "verify FAIL" step below confirms a compile error before Task 3, not a test failure.

- [ ] **Step 1: Write test file (will not compile until Task 3)**

Replace the existing import block in `input_mode_test.go` (currently only has `"testing"`, `handlers`, `assert`) with the fully merged block:

```go
import (
    "sync"
    "testing"

    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    "github.com/cory-johannsen/mud/internal/frontend/telnet"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)
```

Then append the following function declarations after `TestInputMode_String`:

```go
// mockModeHandler is a test double for ModeHandler.
type mockModeHandler struct {
    mode      handlers.InputMode
    prompt    string
    enterLog  []string
    exitLog   []string
    mu        sync.Mutex
}

func newMock(m handlers.InputMode, prompt string) *mockModeHandler {
    return &mockModeHandler{mode: m, prompt: prompt}
}

func (h *mockModeHandler) Mode() handlers.InputMode { return h.mode }
func (h *mockModeHandler) Prompt() string           { return h.prompt }
func (h *mockModeHandler) OnEnter(_ *telnet.Conn)   { h.mu.Lock(); h.enterLog = append(h.enterLog, "enter"); h.mu.Unlock() }
func (h *mockModeHandler) OnExit(_ *telnet.Conn)    { h.mu.Lock(); h.exitLog = append(h.exitLog, "exit"); h.mu.Unlock() }
func (h *mockModeHandler) HandleInput(_ string, _ *telnet.Conn, _ gamev1.GameService_SessionClient, _ *int, _ *handlers.SessionInputState) {}

// TestSessionInputState_SetMode_CallsExitThenEnter verifies OnExit on old,
// OnEnter on new, in order. REQ-IMR-29.
func TestSessionInputState_SetMode_CallsExitThenEnter(t *testing.T) {
    room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
    session := handlers.NewSessionInputState(room)

    mapH := newMock(handlers.ModeMap, "[MAP]")
    session.SetMode(nil, mapH)

    // old handler (room) should have OnExit called once
    require.Equal(t, 1, len(room.ExitLog()))
    // new handler should have OnEnter called once
    require.Equal(t, 1, len(mapH.enterLog))
}

// TestSessionInputState_SetMode_NilOutgoingNoPanic verifies no panic on first
// SetMode when outgoing is the initial RoomModeHandler. REQ-IMR-29.
func TestSessionInputState_SetMode_NilOutgoingNoPanic(t *testing.T) {
    room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
    session := handlers.NewSessionInputState(room)
    mapH := newMock(handlers.ModeMap, "[MAP]")
    assert.NotPanics(t, func() {
        session.SetMode(nil, mapH)
    })
}

// TestSessionInputState_CurrentPrompt_ReflectsActiveHandler verifies
// CurrentPrompt returns the active handler's Prompt(). REQ-IMR-30.
func TestSessionInputState_CurrentPrompt_ReflectsActiveHandler(t *testing.T) {
    room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
    session := handlers.NewSessionInputState(room)
    assert.Equal(t, room.Prompt(), session.CurrentPrompt())

    mapH := newMock(handlers.ModeMap, "[MAP] prompt")
    session.SetMode(nil, mapH)
    assert.Equal(t, "[MAP] prompt", session.CurrentPrompt())
}

// TestSessionInputState_Mode_MatchesActiveHandler verifies Mode() returns the
// active handler's Mode(). REQ-IMR-8A.
func TestSessionInputState_Mode_MatchesActiveHandler(t *testing.T) {
    room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
    session := handlers.NewSessionInputState(room)
    assert.Equal(t, handlers.ModeRoom, session.Mode())

    mapH := newMock(handlers.ModeMap, "[MAP]")
    session.SetMode(nil, mapH)
    assert.Equal(t, handlers.ModeMap, session.Mode())
}

// TestProperty_SessionInputState_ConcurrentSafety verifies no data races
// between concurrent SetMode and CurrentPrompt/Mode reads. REQ-IMR-31a.
func TestProperty_SessionInputState_ConcurrentSafety(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
        session := handlers.NewSessionInputState(room)
        mockHandlers := []*mockModeHandler{
            newMock(handlers.ModeMap, "[MAP]"),
            newMock(handlers.ModeInventory, "[INV]"),
        }
        var wg sync.WaitGroup
        // Writer: SetMode in a loop
        wg.Add(1)
        go func() {
            defer wg.Done()
            for i := 0; i < 20; i++ {
                session.SetMode(nil, mockHandlers[i%2])
            }
        }()
        // Reader: CurrentPrompt + Mode
        wg.Add(1)
        go func() {
            defer wg.Done()
            for i := 0; i < 20; i++ {
                _ = session.CurrentPrompt()
                _ = session.Mode()
            }
        }()
        wg.Wait()
    })
}

// TestProperty_SessionInputState_ModeConsistency verifies Mode() always equals
// the active handler's Mode() after any sequence of SetMode calls. REQ-IMR-31b.
func TestProperty_SessionInputState_ModeConsistency(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
        session := handlers.NewSessionInputState(room)
        modes := []handlers.InputMode{handlers.ModeMap, handlers.ModeInventory, handlers.ModeCharSheet, handlers.ModeEditor, handlers.ModeCombat}
        n := rapid.IntRange(1, 10).Draw(rt, "n")
        for i := 0; i < n; i++ {
            idx := rapid.IntRange(0, len(modes)-1).Draw(rt, "mode_idx")
            h := newMock(modes[idx], "prompt")
            session.SetMode(nil, h)
            if session.Mode() != modes[idx] {
                rt.Fatalf("session.Mode() = %v, want %v", session.Mode(), modes[idx])
            }
        }
    })
}

// TestSessionInputState_CurrentPrompt_AfterSetMode_MapHandler verifies
// forwardServerEvents scenario: after SetMode(mapHandler),
// CurrentPrompt returns map prompt not room prompt. REQ-IMR-32.
func TestSessionInputState_CurrentPrompt_AfterSetMode_MapHandler(t *testing.T) {
    room := handlers.NewRoomModeHandlerForTest("Hero", 10, 10)
    session := handlers.NewSessionInputState(room)
    roomPrompt := session.CurrentPrompt()

    mapH := newMock(handlers.ModeMap, "[MAP] z=zone  w=world  q=exit")
    session.SetMode(nil, mapH)

    got := session.CurrentPrompt()
    assert.NotEqual(t, roomPrompt, got, "map prompt must differ from room prompt")
    assert.Equal(t, "[MAP] z=zone  w=world  q=exit", got)
}
```

Also create `internal/frontend/handlers/input_mode_whitebox_test.go` (uses `package handlers` — internal package access needed to construct nil-current state for REQ-IMR-29):

```go
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
```

- [ ] **Step 2: Run — verify compile error (expected before Task 3)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/frontend/handlers/... 2>&1 | grep "NewRoomModeHandlerForTest\|ExitLog\|undefined"
```
Expected: error mentioning `NewRoomModeHandlerForTest` or `ExitLog` undefined.

> **Stop here and proceed to Task 3. Return to Step 3 after Task 3 is committed.**

- [ ] **Step 3 (run after Task 3): Run tests — verify PASS**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run "TestSessionInputState|TestProperty_Session" -v -count=1 2>&1 | tail -20
```
Expected: all PASS.

---

## Task 3: Implement `RoomModeHandler`

**Files:**
- Create: `internal/frontend/handlers/mode_room.go`

`RoomModeHandler` wraps all the room-command dispatch logic currently inline in `commandLoop`. It is constructed with live-state references for `Prompt()`.

- [ ] **Step 1: Write mode_room.go**

```go
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

    // exitLog is used only by tests.
    exitLog []string
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

// OnEnter is a no-op for room mode (it is the default state). REQ-IMR-4.
func (h *RoomModeHandler) OnEnter(conn *telnet.Conn) {
    h.enterLog = append(h.enterLog, "enter")
    // Redraw prompt on return to room mode.
    if conn != nil {
        if conn.IsSplitScreen() {
            _ = conn.WritePromptSplit(h.Prompt())
        } else {
            _ = conn.WritePrompt(h.Prompt())
        }
    }
}

// OnExit is a no-op for room mode. REQ-IMR-5.
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

// HandleInput dispatches one line to the command registry. REQ-IMR-14.
// Empty lines redraw the prompt.
// The heavy command dispatch (registry lookup, bridgeContext, etc.) is called
// here; the caller (commandLoop) no longer does this directly.
//
// NOTE: HandleInput deliberately does NOT call session.SetMode here.
// Map-mode transitions (result.enterMapMode) are handled after HandleInput
// returns in commandLoop, which then calls session.SetMode(mapHandler).
// This keeps mode transitions visible at the commandLoop level.
func (h *RoomModeHandler) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState) {
    // Empty line: just redraw prompt.
    if line == "" {
        if conn.IsSplitScreen() {
            _ = conn.WritePromptSplit(h.Prompt())
        } else {
            _ = conn.WritePrompt(h.Prompt())
        }
        return
    }

    // NOTE: The full command dispatch (Parse, registry.Resolve, bridgeContext,
    // handlerFn, result handling) remains in commandLoop for this refactor.
    // HandleInput is the dispatch entry point called by commandLoop after
    // sentinel handling; commandLoop itself still builds bridgeContext.
    // This is intentional: bridgeContext captures closures (travelResolver,
    // helpFn) that depend on session state not visible inside RoomModeHandler.
    //
    // In a future refactor, bridgeContext construction can move here.
    // For now, commandLoop calls session.HandleInput(line,...) only when
    // mode != ModeRoom is needed — for room mode it still runs the existing
    // command dispatch inline. See Task 6 for the commandLoop wiring.
    _ = line // handled by commandLoop for ModeRoom
}

// buildRoomPromptFn returns a func() string closure for legacy callers
// during the transition. Deprecated once all callers use session.CurrentPrompt().
func (h *RoomModeHandler) buildRoomPromptFn() func() string {
    return h.Prompt
}
```

- [ ] **Step 2: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/frontend/handlers/... 2>&1
```
Expected: no errors.

- [ ] **Step 3: Run Task 2 tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run "TestInputMode|TestSessionInputState|TestProperty_Session" -v -count=1 2>&1 | tail -20
```
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/frontend/handlers/input_mode.go internal/frontend/handlers/input_mode_test.go internal/frontend/handlers/mode_room.go
git commit -m "feat(input-mode): add RoomModeHandler and SessionInputState tests (REQ-IMR-12..14, REQ-IMR-29..32)"
```

---

## Task 4: Implement `MapModeHandler`

**Files:**
- Create: `internal/frontend/handlers/mode_map.go`

Refactor `mapModeState` + `handleMapModeInput` from `game_bridge_mapmode.go` into `MapModeHandler`. All map-state fields stay on `MapModeHandler`.

- [ ] **Step 1: Create mode_map.go**

```go
// internal/frontend/handlers/mode_map.go
package handlers

import (
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
func (h *MapModeHandler) Snapshot() (view, selectedZone string, lastResp *gamev1.MapResponse) {
    h.mu.Lock()
    defer h.mu.Unlock()
    resp, _ := h.lastMapResponse.Load().(*gamev1.MapResponse)
    return h.mapView, h.mapSelectedZone, resp
}

// HandleInput processes one line of map-mode input.
// REQ-IMR-6, REQ-WM-43..51.
func (h *MapModeHandler) HandleInput(line string, conn *telnet.Conn, stream gamev1.GameService_SessionClient, requestID *int, session *SessionInputState) {
    // Delegate to the existing handleMapModeInputFn — all logic stays the same,
    // only the signatures change.
    handleMapModeInputFn(line, conn, stream, h, requestID, session)
}
```

Add the complete `handleMapModeInputFn` to `mode_map.go` (full body, migrated from `handleMapModeInput` in `game_bridge_mapmode.go`):

```go
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
```

Replace the import block in `mode_map.go` with this merged version (adds `"fmt"`, `"strings"`, `"sort"`, and `"github.com/cory-johannsen/mud/internal/game/world"` needed by `handleMapModeInputFn` and the helpers moved in Task 7):
```go
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
```

- [ ] **Step 2: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/frontend/handlers/... 2>&1
```
Expected: no errors. Both `mode_map.go` and `game_bridge_mapmode.go` coexist in the package; helpers (`resolveZoneSelector`, `writeFmtConsole`, etc.) remain in `game_bridge_mapmode.go` and are callable from `mode_map.go` without duplication. Do NOT move those helpers yet — that happens in Task 7.

> **Note:** Keep `game_bridge_mapmode.go` until Task 7. In this task, do NOT copy over the functions that remain in `game_bridge_mapmode.go` (helpers like `resolveZoneSelector`, `resolveTravelZone`, `writeFmtConsole`, etc.). Only the state struct + HandleInput logic moves.

- [ ] **Step 3: Commit**

```bash
git add internal/frontend/handlers/mode_map.go
git commit -m "feat(input-mode): add MapModeHandler refactoring mapModeState (REQ-IMR-15..18)"
```

---

## Task 5: Implement stub mode handlers

**Files:**
- Create: `internal/frontend/handlers/mode_stubs.go`

- [ ] **Step 1: Write mode_stubs.go**

```go
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
```

- [ ] **Step 2: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/frontend/handlers/... 2>&1
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/frontend/handlers/mode_stubs.go
git commit -m "feat(input-mode): add stub mode handlers for inventory, charsheet, editor, combat (REQ-IMR-19..20)"
```

---

## Task 6: Wire `SessionInputState` into `game_bridge.go`

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`

This is the central wiring task. Replace `mapState *mapModeState` + `buildCurrentPrompt` with `session *SessionInputState` throughout `gameBridge`, `commandLoop`, and `forwardServerEvents`.

- [ ] **Step 1: Replace mapState construction and forwardServerEvents goroutine in `gameBridge`**

In `gameBridge` (around line 327):

```go
// BEFORE:
mapState := &mapModeState{}
// ...
go func() {
    defer wg.Done()
    h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom, &currentTime, &currentDT, &currentHP, &maxHP, &lastRoomView, buildCurrentPrompt, &condMu, activeConditions, mapState)
}()
err = h.commandLoop(streamCtx, stream, conn, char.Name, acct.Role, &lastInput, buildCurrentPrompt, mapState)

// AFTER:
roomHandler := NewRoomModeHandler(
    char.Name, acct.Role,
    &currentHP, &maxHP,
    &currentRoom, &currentTime,
    &condMu, activeConditions,
)
mapHandler := NewMapModeHandler()
session := NewSessionInputState(roomHandler)
// ...
go func() {
    defer wg.Done()
    h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom, &currentTime, &currentDT, &currentHP, &maxHP, &lastRoomView, &condMu, activeConditions, session, mapHandler)
}()
err = h.commandLoop(streamCtx, stream, conn, char.Name, acct.Role, &lastInput, session, mapHandler)
```

Also remove the `buildCurrentPrompt` closure entirely (it becomes `session.CurrentPrompt()`).

- [ ] **Step 2: Update `commandLoop` signature and body**

```go
// BEFORE:
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string, lastInput *atomic.Int64, buildPrompt func() string, mapState *mapModeState) error {

// AFTER:
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string, lastInput *atomic.Int64, session *SessionInputState, mapHandler *MapModeHandler) error {
```

> **Note (REQ-IMR-22 reconciliation):** The spec says `commandLoop` is "replaced with `session *SessionInputState`" (singular). This plan adds `mapHandler *MapModeHandler` alongside `session`. The extra parameter is intentional: `travelResolver` and `result.enterMapMode` need direct access to `MapModeHandler` state that is not exposed through the generic `ModeHandler` interface on `SessionInputState`. This is the correct design for this refactor scope.

> **Note (REQ-IMR-28):** All mode transitions in `commandLoop` and `forwardServerEvents` use `session.SetMode` exclusively (item 6 below; Task 4 HandleInput; Task 5 stubs). Stub handlers also use `session.SetMode(session.Room())` for exit. This satisfies REQ-IMR-28 structurally — no raw handler swaps outside `session.SetMode`.

Inside `commandLoop`:

1. Replace all `buildPrompt()` in sentinel handling with `session.CurrentPrompt()` (REQ-IMR-23):
   - `conn.SetInputLine(buildPrompt(), cmd)` → `conn.SetInputLine(session.CurrentPrompt(), cmd)`
   - `conn.WritePrompt(buildPrompt())` → `conn.WritePrompt(session.CurrentPrompt())`
   - `conn.WritePromptSplit(buildPrompt())` → `conn.WritePromptSplit(session.CurrentPrompt())`

2. Replace map-mode interceptor (REQ-IMR-21):
   ```go
   // BEFORE:
   if mapState.isActive() {
       h.handleMapModeInput(line, conn, stream, mapState, buildPrompt, &requestID)
       continue
   }
   // AFTER:
   if session.Mode() != ModeRoom {
       session.HandleInput(line, conn, stream, &requestID)
       continue
   }
   ```

   > **Note (REQ-IMR-21 reconciliation):** This pattern satisfies REQ-IMR-21. The `mapState.isActive()` interceptor is replaced by `if session.Mode() != ModeRoom`. For `ModeRoom`, the `continue` is NOT taken and the existing command dispatch (registry.Resolve, bridgeContext, handlerFn) runs inline in `commandLoop` as before. `RoomModeHandler.HandleInput` is intentionally a no-op in this refactor — room command dispatch stays in `commandLoop` until a future refactor moves it into `RoomModeHandler`. **Do NOT route `ModeRoom` lines through `session.HandleInput`.**

3. Update `travelResolver` to use `mapHandler.Snapshot()` instead of `mapState.snapshot()`:
   ```go
   travelResolver := func(zoneName string) (string, string) {
       _, _, resp := mapHandler.Snapshot()
       if resp == nil {
           return "", "Open the world map first with 'map'."
       }
       // ... rest unchanged
   }
   ```

4. Replace `promptFn: buildPrompt` in `bridgeContext` with `promptFn: session.CurrentPrompt`:
   ```go
   bctx := &bridgeContext{
       // ...
       promptFn: session.CurrentPrompt,
       // ...
   }
   ```

5. Update `helpFn` closure to use `session.CurrentPrompt()`.

6. Replace `result.enterMapMode` block:
   ```go
   // BEFORE:
   if result.enterMapMode {
       mapState.enter(result.mapView)
       if conn.IsSplitScreen() {
           _ = conn.WriteConsole("")
           _ = conn.WritePromptSplit(mapPrompt("", "", ""))
       }
   }
   // AFTER:
   if result.enterMapMode {
       mapHandler.SetView(result.mapView)
       session.SetMode(conn, mapHandler)
   }
   ```

7. Replace all remaining `buildPrompt()` in `result.done` handling with `session.CurrentPrompt()`.

- [ ] **Step 3: Update `forwardServerEvents` signature and body**

```go
// BEFORE:
func (h *AuthHandler) forwardServerEvents(..., buildPrompt func() string, condMu *sync.Mutex, activeConditions map[string]string, mapState *mapModeState) {
// AFTER:
func (h *AuthHandler) forwardServerEvents(..., condMu *sync.Mutex, activeConditions map[string]string, session *SessionInputState, mapHandler *MapModeHandler) {
```

Replace every `buildPrompt()` call in `forwardServerEvents` with `session.CurrentPrompt()`. (REQ-IMR-24)

Replace the resize handler's map-mode check:
```go
// BEFORE:
inMapMode, mapView, _, mapResp := mapState.snapshot()
if inMapMode {
    ...
    _ = conn.WritePromptSplit(mapPrompt("", "", ""))
    continue
}
_ = conn.WritePromptSplit(buildPrompt())

// AFTER:
if session.Mode() == ModeMap {
    mapView, _, mapResp := mapHandler.Snapshot()
    if mapResp != nil {
        _ = conn.WriteConsole(renderMapConsole(mapResp, mapView, rw))
    }
    _ = conn.WritePromptSplit(session.CurrentPrompt())
    continue
}
_ = conn.WritePromptSplit(session.CurrentPrompt())
```

Replace the RoomView event map-mode exit (REQ-IMR-11A):
```go
// BEFORE:
mapState.mu.Lock()
if mapState.mapMode {
    mapState.mapMode = false
    mapState.mapSelectedZone = ""
}
mapState.mu.Unlock()

// AFTER:
if session.Mode() != ModeRoom {
    session.SetMode(conn, session.Room())
}
```

Replace the Map event handling:
```go
// BEFORE:
mapState.mu.Lock()
inMapMode := mapState.mapMode
mapView := mapState.mapView
mapState.mu.Unlock()
if inMapMode {
    mapState.setLastResponse(p.Map)
    ...
    _ = conn.WritePromptSplit("[MAP] z=zone  w=world  <num/name>=select  t=travel  q=exit")
    ...
    _ = conn.WritePrompt(buildPrompt())
    ...
}

// AFTER:
if session.Mode() == ModeMap {
    mapHandler.SetLastResponse(p.Map)
    mapView, _, _ := mapHandler.Snapshot()
    var rendered string
    if mapView == "world" {
        rendered = RenderWorldMap(p.Map, mw)
    } else {
        rendered = RenderMap(p.Map, mw)
    }
    if conn.IsSplitScreen() {
        _ = conn.WriteConsole(rendered)
        _ = conn.WritePromptSplit(session.CurrentPrompt())
    } else {
        _ = conn.WriteLine(rendered)
        _ = conn.WritePrompt(session.CurrentPrompt())
    }
    continue
}
```

Replace the promptTicker case:
```go
_ = conn.WritePromptSplit(session.CurrentPrompt())
```

- [ ] **Step 4: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```
Expected: no errors. If `game_bridge_mapmode.go` has duplicate symbols, note them — they will be resolved in Task 7.

- [ ] **Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -count=1 2>&1 | grep -v "^ok\|^?" | grep -v postgres | head -20
```
Expected: zero failures.

- [ ] **Step 6: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go
git commit -m "feat(input-mode): wire SessionInputState into gameBridge, commandLoop, forwardServerEvents (REQ-IMR-21..27)"
```

---

## Task 7: Delete `game_bridge_mapmode.go` and clean up

**Files:**
- Delete: `internal/frontend/handlers/game_bridge_mapmode.go`
- Verify: all helpers now live in `mode_map.go`

The helpers `resolveZoneSelector`, `resolveTravelZone`, `writeFmtConsole`, `sortWorldTiles`, `renderMapConsole`, `zoneNameAndDanger`, `writeMapPromptToConn`, `mapPrompt` must all be present in `mode_map.go` before deletion.

- [ ] **Step 1: Move remaining helpers from `game_bridge_mapmode.go` to `mode_map.go`**

Append the following to `mode_map.go` (these are the package-level helpers currently in `game_bridge_mapmode.go`):

```go
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
```

Also ensure `mode_map.go` imports `"sort"` and `"github.com/cory-johannsen/mud/internal/game/world"` (add to the existing import block).

- [ ] **Step 2: Delete `game_bridge_mapmode.go`**

```bash
git rm internal/frontend/handlers/game_bridge_mapmode.go
```

- [ ] **Step 3: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```
Expected: no errors.

- [ ] **Step 4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -v "^ok\|^?" | grep -v postgres | head -20
```
Expected: zero failures.

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/handlers/mode_map.go
git rm internal/frontend/handlers/game_bridge_mapmode.go
git commit -m "feat(input-mode): delete game_bridge_mapmode.go; all logic in mode_map.go (REQ-IMR-15)"
```

---

## Task 8: Final verification

- [ ] **Step 1: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... -count=1 2>&1 | grep -v "^ok\|^?" | grep -v postgres
```
Expected: empty output (zero failures).

- [ ] **Step 2: Race detector check**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test -race ./internal/frontend/handlers/... -count=1 2>&1 | tail -10
```
Expected: no DATA RACE messages.

- [ ] **Step 3: Update feature index status**

In `docs/features/index.yaml`, change `input-mode-refactor` status from `spec` to `done`.

- [ ] **Step 4: Commit**

```bash
git add docs/features/index.yaml
git commit -m "docs: mark input-mode-refactor as done"
```
