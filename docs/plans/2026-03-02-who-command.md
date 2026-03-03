# Who Command Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enhance the `who` command to display level, job, descriptive health label, and status for each player in the current room.

**Architecture:** Add `CombatStatus` enum and `PlayerInfo` proto message; replace `repeated string` in `PlayerList` with `repeated PlayerInfo`. Add `Status int32` to `PlayerSession`. Add pure `HealthLabel`/`StatusLabel` helpers in `command/who.go`. Update `ChatHandler.Who` to build `PlayerInfo` entries. Update `RenderPlayerList` to format the richer output.

**Tech Stack:** Go, protobuf/gRPC, `pgregory.net/rapid` for property tests, `testify` for assertions.

---

### Task 1: Add CombatStatus enum + PlayerInfo message to proto; update PlayerList

**Files:**
- Modify: `api/proto/game/v1/game.proto`

**Step 1: Read the proto file**

Read `api/proto/game/v1/game.proto` to find the exact line numbers for `PlayerList` and a good place to add the new enum and message (near other standalone messages, e.g. after `PlayerList`).

**Step 2: Add CombatStatus enum and PlayerInfo message**

After the existing `PlayerList` message, add:

```proto
// CombatStatus describes a player's current combat state.
enum CombatStatus {
    COMBAT_STATUS_UNSPECIFIED = 0;
    COMBAT_STATUS_IDLE        = 1;
    COMBAT_STATUS_IN_COMBAT   = 2;
    COMBAT_STATUS_RESTING     = 3;
    COMBAT_STATUS_UNCONSCIOUS = 4;
}

// PlayerInfo carries per-player detail for the who command.
message PlayerInfo {
    string       name         = 1;
    int32        level        = 2;
    string       job          = 3;
    string       health_label = 4;
    CombatStatus status       = 5;
}
```

**Step 3: Replace `repeated string players` in PlayerList**

Change:
```proto
message PlayerList {
  string room_title = 1;
  repeated string players = 2;
}
```
To:
```proto
// PlayerList contains the players in the current room.
message PlayerList {
    string              room_title = 1;
    repeated PlayerInfo players    = 2;
}
```

**Step 4: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: exits 0, regenerated files in `internal/gameserver/gamev1/`.

**Step 5: Check build errors**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -30
```

Expected: compile errors in `chat_handler.go`, `chat_handler_test.go`, and `text_renderer.go` because they used `repeated string`. This is expected — we will fix them in later tasks.

**Step 6: Commit proto changes only**

```bash
cd /home/cjohannsen/src/mud && git add api/proto/game/v1/game.proto internal/gameserver/gamev1/ && git commit -m "feat: add CombatStatus enum and PlayerInfo proto message"
```

---

### Task 2: Add Status field to PlayerSession

**Files:**
- Modify: `internal/game/session/manager.go`

The `PlayerSession` struct needs a `Status` field that maps to the `CombatStatus` proto enum values. We store it as `int32` to avoid importing the proto package into the session layer (the server layer maps it to the enum when building `PlayerInfo`).

**Step 1: Read manager.go**

Read `internal/game/session/manager.go` to see the current `PlayerSession` struct (lines ~12-47).

**Step 2: Add Status field**

In the `PlayerSession` struct, add after `Entity`:

```go
// Status is the player's current combat state.
// Maps to gamev1.CombatStatus enum values: 0=Unspecified/Idle, 1=Idle, 2=InCombat, 3=Resting, 4=Unconscious.
Status int32
```

**Step 3: Build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/game/session/... 2>&1
```

Expected: exits 0.

**Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/manager.go && git commit -m "feat: add Status field to PlayerSession for combat state"
```

---

### Task 3: Add PlayersInRoomDetails to session.Manager

The `ChatHandler.Who` needs full session data (level, class, HP, status) for each player in the room, not just names. Add a `PlayersInRoomDetails` method that returns `[]*PlayerSession`.

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/game/session/manager_test.go`

**Step 1: Write a failing test**

Read `internal/game/session/manager_test.go` to understand the existing test patterns. Then append:

```go
func TestManager_PlayersInRoomDetails_ReturnsSessions(t *testing.T) {
	m := session.NewManager()
	_, err := m.AddPlayer(session.AddPlayerOptions{
		UID: "u1", Username: "a", CharName: "Alpha",
		CharacterID: 1, RoomID: "room1",
		CurrentHP: 8, MaxHP: 10,
		Abilities: character.AbilityScores{}, Role: "player",
		RegionDisplayName: "", Class: "striker_gun", Level: 3,
	})
	require.NoError(t, err)
	_, err = m.AddPlayer(session.AddPlayerOptions{
		UID: "u2", Username: "b", CharName: "Beta",
		CharacterID: 2, RoomID: "room1",
		CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{}, Role: "player",
		RegionDisplayName: "", Class: "boot_gun", Level: 1,
	})
	require.NoError(t, err)

	details := m.PlayersInRoomDetails("room1")
	require.Len(t, details, 2)
	names := []string{details[0].CharName, details[1].CharName}
	assert.ElementsMatch(t, []string{"Alpha", "Beta"}, names)
	// Verify full fields are present
	for _, d := range details {
		assert.NotEmpty(t, d.Class)
		assert.Greater(t, d.MaxHP, 0)
	}
}

func TestManager_PlayersInRoomDetails_EmptyRoomReturnsEmpty(t *testing.T) {
	m := session.NewManager()
	assert.Empty(t, m.PlayersInRoomDetails("nonexistent"))
}
```

**Step 2: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -run "TestManager_PlayersInRoomDetails" -v 2>&1 | head -10
```

Expected: compile error — `PlayersInRoomDetails` not defined.

**Step 3: Implement PlayersInRoomDetails**

In `internal/game/session/manager.go`, append after `PlayersInRoom`:

```go
// PlayersInRoomDetails returns the full PlayerSession for each player in the given room.
//
// Precondition: roomID may be any string.
// Postcondition: Returns a non-nil slice (may be empty); each element is a non-nil *PlayerSession.
func (m *Manager) PlayersInRoomDetails(roomID string) []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uids, ok := m.roomSets[roomID]
	if !ok {
		return []*PlayerSession{}
	}
	result := make([]*PlayerSession, 0, len(uids))
	for uid := range uids {
		if sess, ok := m.players[uid]; ok {
			result = append(result, sess)
		}
	}
	return result
}
```

**Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -v 2>&1 | tail -15
```

Expected: all PASS.

**Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/manager.go internal/game/session/manager_test.go && git commit -m "feat: add PlayersInRoomDetails to session.Manager"
```

---

### Task 4: Add HandleWho, HealthLabel, StatusLabel in command/who.go

**Files:**
- Create: `internal/game/command/who.go`
- Create: `internal/game/command/who_test.go`

**Step 1: Create the test file first**

Create `internal/game/command/who_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHealthLabel_AllThresholds(t *testing.T) {
	cases := []struct {
		current, max int
		want         string
	}{
		{10, 10, "Uninjured"},
		{9, 10, "Lightly Wounded"},
		{75, 100, "Lightly Wounded"},
		{74, 100, "Wounded"},
		{50, 100, "Wounded"},
		{49, 100, "Badly Wounded"},
		{25, 100, "Badly Wounded"},
		{24, 100, "Near Death"},
		{0, 100, "Near Death"},
		{1, 100, "Near Death"},
	}
	for _, tc := range cases {
		got := command.HealthLabel(tc.current, tc.max)
		assert.Equal(t, tc.want, got, "HealthLabel(%d, %d)", tc.current, tc.max)
	}
}

func TestHealthLabel_ZeroMaxHP_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() { command.HealthLabel(0, 0) })
}

func TestStatusLabel_AllValues(t *testing.T) {
	assert.Equal(t, "Idle", command.StatusLabel(0))
	assert.Equal(t, "Idle", command.StatusLabel(1))
	assert.Equal(t, "In Combat", command.StatusLabel(2))
	assert.Equal(t, "Resting", command.StatusLabel(3))
	assert.Equal(t, "Unconscious", command.StatusLabel(4))
	assert.Equal(t, "Idle", command.StatusLabel(99)) // unknown → Idle
}

func TestHandleWho_EmptyList(t *testing.T) {
	result := command.HandleWho(nil)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Nobody")
}

func TestProperty_HealthLabel_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		current := rapid.Int().Draw(rt, "current")
		max := rapid.Int().Draw(rt, "max")
		_ = command.HealthLabel(current, max)
	})
}

func TestProperty_StatusLabel_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		status := rapid.Int32().Draw(rt, "status")
		_ = command.StatusLabel(status)
	})
}
```

**Step 2: Run to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHealthLabel|TestStatusLabel|TestHandleWho_Empty|TestProperty_HealthLabel|TestProperty_StatusLabel" -v 2>&1 | head -10
```

Expected: compile error.

**Step 3: Create internal/game/command/who.go**

```go
package command

import (
	"fmt"
	"strings"
)

// WhoEntry holds the display fields for one player in the who list.
type WhoEntry struct {
	Name        string
	Level       int
	Job         string
	HealthLabel string
	Status      string
}

// HealthLabel returns a descriptive health label based on current and max HP.
//
// Precondition: max may be zero (treated as 0% HP).
// Postcondition: Returns one of: "Uninjured", "Lightly Wounded", "Wounded", "Badly Wounded", "Near Death".
func HealthLabel(current, max int) string {
	if max <= 0 {
		return "Near Death"
	}
	pct := current * 100 / max
	switch {
	case pct >= 100:
		return "Uninjured"
	case pct >= 75:
		return "Lightly Wounded"
	case pct >= 50:
		return "Wounded"
	case pct >= 25:
		return "Badly Wounded"
	default:
		return "Near Death"
	}
}

// StatusLabel returns a human-readable label for a CombatStatus int32 value.
//
// Precondition: status may be any int32.
// Postcondition: Returns one of: "Idle", "In Combat", "Resting", "Unconscious".
func StatusLabel(status int32) string {
	switch status {
	case 2:
		return "In Combat"
	case 3:
		return "Resting"
	case 4:
		return "Unconscious"
	default:
		return "Idle"
	}
}

// HandleWho returns a plain-text who listing from a slice of WhoEntry.
// Used as a fallback for telnet/dev-server clients.
//
// Precondition: entries may be nil or empty.
// Postcondition: Returns a non-empty human-readable string.
func HandleWho(entries []WhoEntry) string {
	if len(entries) == 0 {
		return "Nobody else is here."
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("  %s — Lvl %d %s — %s — %s\r\n",
			e.Name, e.Level, e.Job, e.HealthLabel, e.Status))
	}
	return sb.String()
}
```

**Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run "TestHealthLabel|TestStatusLabel|TestHandleWho_Empty|TestProperty_HealthLabel|TestProperty_StatusLabel" -v
```

Expected: all PASS.

**Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/who.go internal/game/command/who_test.go && git commit -m "feat: add HandleWho, HealthLabel, StatusLabel to command package"
```

---

### Task 5: Update ChatHandler.Who and its tests

`ChatHandler.Who` currently returns `Players: players` where `players` is `[]string`. It must now return `[]*gamev1.PlayerInfo`. The `chat_handler_test.go` assertions on `list.Players` must be updated.

**Files:**
- Modify: `internal/gameserver/chat_handler.go`
- Modify: `internal/gameserver/chat_handler_test.go`

**Step 1: Update TestChatHandler_Who in chat_handler_test.go**

Read the file, then replace the test assertions (lines 111-115) from:
```go
assert.Len(t, list.Players, 2)
assert.Contains(t, list.Players, "Alice")
assert.Contains(t, list.Players, "Bob")
```
To:
```go
require.Len(t, list.Players, 2)
names := []string{list.Players[0].Name, list.Players[1].Name}
assert.ElementsMatch(t, []string{"Alice", "Bob"}, names)
```

Also add a test for richer fields. Append a new test:

```go
func TestChatHandler_Who_PopulatesPlayerInfo(t *testing.T) {
	sessMgr := session.NewManager()
	h := NewChatHandler(sessMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u1", Username: "Alice", CharName: "Alice",
		CharacterID: 1, RoomID: "room_a",
		CurrentHP:   8, MaxHP: 10,
		Abilities:   character.AbilityScores{}, Role: "player",
		Class:       "striker_gun", Level: 3,
	})
	require.NoError(t, err)

	list, err := h.Who("u1")
	require.NoError(t, err)
	require.Len(t, list.Players, 1)
	p := list.Players[0]
	assert.Equal(t, "Alice", p.Name)
	assert.Equal(t, int32(3), p.Level)
	assert.Equal(t, "striker_gun", p.Job)
	assert.Equal(t, "Lightly Wounded", p.HealthLabel)
	assert.Equal(t, gamev1.CombatStatus_COMBAT_STATUS_IDLE, p.Status)
}
```

**Step 2: Run tests to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestChatHandler_Who" -v 2>&1 | head -20
```

Expected: compile errors (Players is now `[]*PlayerInfo` not `[]string`).

**Step 3: Update ChatHandler.Who in chat_handler.go**

The `Who` method must:
1. Call `h.sessions.PlayersInRoomDetails(sess.RoomID)` instead of `PlayersInRoom`
2. Build `[]*gamev1.PlayerInfo` from the returned sessions
3. Use `command.HealthLabel` and `command.StatusLabel` for the descriptive fields

Replace the entire `Who` method:

```go
// Who returns the list of players in the sender's room with full detail.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a PlayerList with PlayerInfo entries, or an error.
func (h *ChatHandler) Who(uid string) (*gamev1.PlayerList, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	sessions := h.sessions.PlayersInRoomDetails(sess.RoomID)
	players := make([]*gamev1.PlayerInfo, 0, len(sessions))
	for _, s := range sessions {
		players = append(players, &gamev1.PlayerInfo{
			Name:        s.CharName,
			Level:       int32(s.Level),
			Job:         s.Class,
			HealthLabel: command.HealthLabel(s.CurrentHP, s.MaxHP),
			Status:      gamev1.CombatStatus(s.Status),
		})
	}
	return &gamev1.PlayerList{
		RoomTitle: sess.RoomID,
		Players:   players,
	}, nil
}
```

Add the import for `command` package:
```go
"github.com/cory-johannsen/mud/internal/game/command"
```

**Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestChatHandler_Who" -v
```

Expected: all PASS.

**Step 5: Build to check for other breakage**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -20
```

Expected: only `text_renderer.go` errors (because `RenderPlayerList` still uses `strings.Join(pl.Players, ", ")`). We fix that in Task 6.

**Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/chat_handler.go internal/gameserver/chat_handler_test.go && git commit -m "feat: update ChatHandler.Who to return PlayerInfo entries"
```

---

### Task 6: Update RenderPlayerList in text_renderer.go

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`

The current renderer calls `strings.Join(pl.Players, ", ")` which treats `Players` as `[]string`. It must be updated to format `[]*gamev1.PlayerInfo` entries as:
```
  Name — Lvl N Job — Health Label — Status
```

**Step 1: Write a failing test**

Check if `internal/frontend/handlers/text_renderer_test.go` exists:
```bash
ls /home/cjohannsen/src/mud/internal/frontend/handlers/text_renderer_test.go 2>/dev/null && echo EXISTS || echo MISSING
```

If MISSING, create it. If EXISTS, append to it.

Add tests:

```go
package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
)

func TestRenderPlayerList_EmptyList(t *testing.T) {
	pl := &gamev1.PlayerList{RoomTitle: "room1", Players: nil}
	result := handlers.RenderPlayerList(pl)
	assert.Contains(t, result, "Nobody")
}

func TestRenderPlayerList_ShowsLevelAndJob(t *testing.T) {
	pl := &gamev1.PlayerList{
		RoomTitle: "room1",
		Players: []*gamev1.PlayerInfo{
			{Name: "Raze", Level: 3, Job: "Striker (Gun)", HealthLabel: "Wounded", Status: gamev1.CombatStatus_COMBAT_STATUS_IDLE},
		},
	}
	result := handlers.RenderPlayerList(pl)
	assert.Contains(t, result, "Raze")
	assert.Contains(t, result, "Lvl 3")
	assert.Contains(t, result, "Striker (Gun)")
}

func TestRenderPlayerList_ShowsHealthLabel(t *testing.T) {
	pl := &gamev1.PlayerList{
		RoomTitle: "room1",
		Players: []*gamev1.PlayerInfo{
			{Name: "Ash", Level: 1, Job: "Boot (Gun)", HealthLabel: "Near Death", Status: gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT},
		},
	}
	result := handlers.RenderPlayerList(pl)
	assert.Contains(t, result, "Near Death")
	assert.Contains(t, result, "In Combat")
}
```

**Step 2: Note — RenderPlayerList is currently unexported?**

Check: in `text_renderer.go` is `RenderPlayerList` already exported (capital R)? If yes, tests can reference it as `handlers.RenderPlayerList`. Read the file to confirm before writing tests.

**Step 3: Run tests to confirm failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run "TestRenderPlayerList" -v 2>&1 | head -15
```

Expected: compile error (Players type mismatch or test file not yet accepted).

**Step 4: Update RenderPlayerList in text_renderer.go**

Read text_renderer.go to find the exact current implementation. Then replace `RenderPlayerList` with:

```go
// RenderPlayerList formats a PlayerList for telnet display.
//
// Precondition: pl must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func RenderPlayerList(pl *gamev1.PlayerList) string {
	if len(pl.Players) == 0 {
		return telnet.Colorize(telnet.Dim, "Nobody else is here.")
	}
	var sb strings.Builder
	sb.WriteString(telnet.Colorize(telnet.BrightWhite, "Players here:\r\n"))
	for _, p := range pl.Players {
		status := statusLabel(p.Status)
		sb.WriteString(fmt.Sprintf("  %s%s%s — Lvl %d %s — %s — %s\r\n",
			telnet.Green, p.Name, telnet.Reset,
			p.Level, p.Job,
			p.HealthLabel,
			status))
	}
	return sb.String()
}

// statusLabel converts a CombatStatus to a display string.
func statusLabel(s gamev1.CombatStatus) string {
	switch s {
	case gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT:
		return "In Combat"
	case gamev1.CombatStatus_COMBAT_STATUS_RESTING:
		return "Resting"
	case gamev1.CombatStatus_COMBAT_STATUS_UNCONSCIOUS:
		return "Unconscious"
	default:
		return "Idle"
	}
}
```

Make sure `"fmt"` and `"strings"` are in the imports (check existing imports first).

**Step 5: Build everything**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

Expected: exits 0 with no errors.

**Step 6: Run all tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1 | tail -15
```

Expected: all PASS.

**Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go && git commit -m "feat: update RenderPlayerList to format PlayerInfo with level, job, health, status"
```

---

### Task 7: Final verification

**Step 1: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test $(mise exec -- go list ./... | grep -v postgres) 2>&1
```

Expected: all `ok`.

**Step 2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Expected: exits 0.
