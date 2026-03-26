# Exploration Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `explore` command and its seven exploration modes (Lay Low, Hold Ground, Active Sensors, Case It, Run Point, Shadow, Poke Around) per the spec at `docs/superpowers/specs/2026-03-20-exploration-mode-design.md`.

**Architecture:** `ExploreMode string` already exists on `PlayerSession`. This plan adds `ExploreShadowTarget int64` and `LayLowBlockedRoom string`, a new proto message, the `explore` command end-to-end, a dedicated `grpc_service_explore.go` hook file for room-entry dispatch, and combat-start hook wiring in `combat_handler.go`.

**Tech Stack:** Go, protobuf (`make proto`), `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify/require`.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `content/conditions/undetected.yaml` | **Create** | Undetected condition (REQ-EXP-8) |
| `internal/game/session/manager.go` | **Modify** | Add `ExploreShadowTarget int64`, `LayLowBlockedRoom string`, 7 mode constants |
| `api/proto/game/v1/game.proto` | **Modify** | Add `ExploreRequest` message + field 125 in `ClientMessage` oneof |
| `internal/gameserver/gamev1/game.pb.go` | **Generate** | Run `make proto` |
| `internal/game/command/explore.go` | **Create** | `HandleExplore` parser |
| `internal/game/command/explore_test.go` | **Create** | Parser unit tests |
| `internal/game/command/commands.go` | **Modify** | Add `HandlerExplore` constant + `BuiltinCommands()` entry |
| `internal/frontend/handlers/bridge_handlers.go` | **Modify** | Add `bridgeExplore` + register in handler map |
| `internal/gameserver/grpc_service.go` | **Modify** | Add switch case + `handleExplore`, patch `applyRoomSkillChecks` for Shadow, clear `LayLowBlockedRoom` on move |
| `internal/gameserver/grpc_service_explore.go` | **Create** | `applyExploreModeOnEntry` + `applyExploreModeOnCombatStart` |
| `internal/gameserver/grpc_service_explore_test.go` | **Create** | Mode hook unit tests |
| `internal/gameserver/combat_handler.go` | **Modify** | Wire `applyExploreModeOnCombatStart` after `RollInitiative` |
| `docs/features/index.yaml` | **Modify** | Set `exploration` status to `in_progress`, then `done` |

---

## Task 1: `undetected` Condition + Session Fields

**Files:**
- Create: `content/conditions/undetected.yaml`
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Create `undetected` condition YAML**

```yaml
# content/conditions/undetected.yaml
id: undetected
name: Undetected
description: >
  You are completely hidden from NPCs in this room. Attackers cannot target you
  unless they succeed on a DC 11 flat check to locate you first.
duration_type: encounter
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
```

- [ ] **Step 2: Add mode constants and new fields to `PlayerSession`**

In `internal/game/session/manager.go`, after the existing `ExploreModeCaseIt` constant (line 18–19), add:

```go
// Exploration mode ID constants.
const (
	ExploreModeCaseIt      = "case_it"      // Search mode — enables trap detection (REQ-EXP-24)
	ExploreModeLayLow      = "lay_low"      // Stealth mode — secret Ghosting check on entry
	ExploreModeHoldGround  = "hold_ground"  // Shield mode — auto raise shield at combat start
	ExploreModeActiveSensors = "active_sensors" // Tech scan — secret Tech Lore check on entry
	ExploreModeRunPoint    = "run_point"    // Scout mode — +1 Initiative bonus to co-located players
	ExploreModeShadow     = "shadow"        // Follow mode — borrow ally's skill rank
	ExploreModePokeAround  = "poke_around"  // Lore mode — secret Recall Knowledge check on entry
)
```

In `PlayerSession`, after the `ExploreMode string` field (line 140), add:

```go
// ExploreShadowTarget is the CharacterID of the ally being shadowed.
// Only meaningful when ExploreMode == ExploreModeShadow.
// 0 means no target set.
ExploreShadowTarget int64
// LayLowBlockedRoom is the RoomID where a Lay Low critical failure occurred.
// While in this room, the player cannot gain hidden or undetected.
// Cleared on room exit by handleMove.
LayLowBlockedRoom string
```

- [ ] **Step 3: Verify the session package compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/game/session/...
```

Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add content/conditions/undetected.yaml internal/game/session/manager.go
git commit -m "feat(explore): add undetected condition and session fields for exploration modes"
```

---

## Task 2: Proto Message + `make proto`

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Generate: `internal/gameserver/gamev1/game.pb.go`

- [ ] **Step 1: Add `ExploreRequest` message to game.proto**

After the `AffixRequest` message definition (search for `message AffixRequest`), add:

```proto
// ExploreRequest sets or clears the player's exploration mode.
// mode: one of "lay_low", "hold_ground", "active_sensors", "case_it",
//              "run_point", "shadow", "poke_around", or "off" to clear.
// shadow_target: player name to shadow (only used when mode == "shadow").
message ExploreRequest {
  string mode = 1;
  string shadow_target = 2;
}
```

In the `ClientMessage` oneof (after `affix_request = 124;`), add:

```proto
    ExploreRequest explore_request = 125;
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: exits 0, `internal/gameserver/gamev1/game.pb.go` updated (check `git diff --stat`).

- [ ] **Step 3: Verify build**

```bash
mise exec -- go build ./...
```

Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(explore): add ExploreRequest proto message"
```

---

## Task 3: Command Parser + BuiltinCommands + Bridge Handler

**Files:**
- Create: `internal/game/command/explore.go`
- Create: `internal/game/command/explore_test.go`
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/game/command/explore_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/require"
)

func TestHandleExplore_NoArgs(t *testing.T) {
	req, err := command.HandleExplore(nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "", req.Mode)
	require.Equal(t, "", req.ShadowTarget)
}

func TestHandleExplore_Off(t *testing.T) {
	req, err := command.HandleExplore([]string{"off"})
	require.NoError(t, err)
	require.Equal(t, "off", req.Mode)
}

func TestHandleExplore_SimpleMode(t *testing.T) {
	modes := []string{"lay_low", "hold_ground", "active_sensors", "case_it", "run_point", "poke_around"}
	for _, m := range modes {
		req, err := command.HandleExplore([]string{m})
		require.NoError(t, err, "mode: %s", m)
		require.Equal(t, m, req.Mode, "mode: %s", m)
		require.Equal(t, "", req.ShadowTarget)
	}
}

func TestHandleExplore_Shadow_WithTarget(t *testing.T) {
	req, err := command.HandleExplore([]string{"shadow", "Alice"})
	require.NoError(t, err)
	require.Equal(t, "shadow", req.Mode)
	require.Equal(t, "Alice", req.ShadowTarget)
}

func TestHandleExplore_Shadow_NoTarget(t *testing.T) {
	req, err := command.HandleExplore([]string{"shadow"})
	require.NoError(t, err)
	require.Equal(t, "shadow", req.Mode)
	require.Equal(t, "", req.ShadowTarget)
}

func TestHandleExplore_CaseInsensitive(t *testing.T) {
	req, err := command.HandleExplore([]string{"LAY_LOW"})
	require.NoError(t, err)
	require.Equal(t, "lay_low", req.Mode)
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/command/... -run TestHandleExplore -v 2>&1 | head -20
```

Expected: compile error or `FAIL` — `HandleExplore` does not exist yet.

- [ ] **Step 3: Create `internal/game/command/explore.go`**

```go
package command

import "strings"

// ExploreRequest is the parsed form of the explore command.
//
// Precondition: none.
// Postcondition: Mode is lower-cased; ShadowTarget is the raw second token when Mode == "shadow".
type ExploreRequest struct {
	// Mode is the requested exploration mode ID, "off" to clear, or "" to query.
	Mode string
	// ShadowTarget is the ally player name when Mode == "shadow". Empty otherwise.
	ShadowTarget string
}

// HandleExplore parses the arguments for the "explore" command.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *ExploreRequest; Mode is lower-cased.
func HandleExplore(args []string) (*ExploreRequest, error) {
	if len(args) == 0 {
		return &ExploreRequest{}, nil
	}
	mode := strings.ToLower(args[0])
	req := &ExploreRequest{Mode: mode}
	if mode == "shadow" && len(args) >= 2 {
		req.ShadowTarget = args[1]
	}
	return req, nil
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
mise exec -- go test ./internal/game/command/... -run TestHandleExplore -v
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Add `HandlerExplore` constant to `commands.go`**

In `internal/game/command/commands.go`, after `HandlerAffix = "affix"` (last constant), add:

```go
HandlerExplore = "explore"
```

- [ ] **Step 6: Add `explore` entry to `BuiltinCommands()`**

In `BuiltinCommands()` (in `commands.go`), add — place it near other stance/mode commands:

```go
{Name: "explore", Aliases: []string{"ex"}, Help: "Set or query your exploration mode. Usage: explore [mode|off]. Modes: lay_low, hold_ground, active_sensors, case_it, run_point, shadow <ally>, poke_around.", Category: CategoryExploration, Handler: HandlerExplore},
```

If `CategoryExploration` does not exist, add it alongside the other category constants:

```go
CategoryExploration = "Exploration"
```

- [ ] **Step 7: Add `bridgeExplore` to `bridge_handlers.go`**

In `internal/frontend/handlers/bridge_handlers.go`, add the handler function (near other bridge functions for combat/exploration commands):

```go
// bridgeExplore builds an ExploreRequest.
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: returns a non-nil msg containing an ExploreRequest.
func bridgeExplore(bctx *bridgeContext) (bridgeResult, error) {
	req, err := command.HandleExplore(bctx.args)
	if err != nil {
		return bridgeResult{}, err
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_ExploreRequest{
			ExploreRequest: &gamev1.ExploreRequest{
				Mode:         req.Mode,
				ShadowTarget: req.ShadowTarget,
			},
		},
	}}, nil
}
```

In the handler map (the `var bridgeHandlerMap` or equivalent map initializer), add:

```go
command.HandlerExplore: bridgeExplore,
```

- [ ] **Step 8: Build and verify**

```bash
mise exec -- go build ./...
```

Expected: clean build.

- [ ] **Step 9: Commit**

```bash
git add internal/game/command/explore.go internal/game/command/explore_test.go \
        internal/game/command/commands.go internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(explore): add explore command parser, handler constant, and bridge handler"
```

---

## Task 4: `handleExplore` in `grpc_service.go`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Add switch case for `ExploreRequest`**

In `grpc_service.go`, in the large `switch p := msg.Payload.(type)` block, after the `case *gamev1.ClientMessage_Affix:` case, add:

```go
case *gamev1.ClientMessage_ExploreRequest:
    return s.handleExplore(uid, p.ExploreRequest)
```

- [ ] **Step 2: Implement `handleExplore`**

Add the following function near the end of `grpc_service.go` (or near `handleRaiseShield` for thematic grouping):

```go
// handleExplore sets, clears, or queries the player's exploration mode.
//
// Precondition: uid is a connected player UID; req is non-nil.
// Postcondition: sess.ExploreMode is updated; immediate hooks fire for active_sensors and case_it.
func (s *GameServiceServer) handleExplore(uid string, req *gamev1.ExploreRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Blocked in combat (REQ-EXP-4).
	if sess.Status == statusInCombat {
		return errorEvent("You cannot change exploration mode while in combat."), nil
	}

	// Query mode: no argument.
	if req.Mode == "" {
		if sess.ExploreMode == "" {
			return messageEvent("No active exploration mode."), nil
		}
		msg := "Exploration mode: " + exploreDisplayName(sess.ExploreMode)
		if sess.ExploreMode == session.ExploreModeShadow {
			ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget)
			if ally != nil && ally.RoomID == sess.RoomID {
				msg += " [shadowing " + ally.CharName + "]"
			} else if ally != nil {
				msg += " [shadowing " + ally.CharName + " — not present]"
			}
		}
		return messageEvent(msg), nil
	}

	// Clear mode (REQ-EXP-2).
	if req.Mode == "off" {
		sess.ExploreMode = ""
		sess.ExploreShadowTarget = 0
		return messageEvent("Exploration mode cleared."), nil
	}

	// Validate shadow target (REQ-EXP-6).
	if req.Mode == session.ExploreModeShadow {
		if req.ShadowTarget == "" {
			return errorEvent("Usage: explore shadow <player name>"), nil
		}
		target := s.sessions.GetPlayerByCharNameCI(req.ShadowTarget)
		if target == nil || target.RoomID != sess.RoomID {
			return errorEvent(fmt.Sprintf("No player named %q is in this room.", req.ShadowTarget)), nil
		}
		sess.ExploreMode = session.ExploreModeShadow
		sess.ExploreShadowTarget = target.CharacterID
		return messageEvent(fmt.Sprintf("Exploration mode: Shadow [shadowing %s]", target.CharName)), nil
	}

	// Validate mode ID.
	validModes := map[string]string{
		session.ExploreModeLayLow:        "Lay Low",
		session.ExploreModeHoldGround:    "Hold Ground",
		session.ExploreModeActiveSensors: "Active Sensors",
		session.ExploreModeCaseIt:        "Case It",
		session.ExploreModeRunPoint:      "Run Point",
		session.ExploreModePokeAround:    "Poke Around",
	}
	displayName, valid := validModes[req.Mode]
	if !valid {
		return errorEvent(fmt.Sprintf("Unknown exploration mode %q. Valid modes: lay_low, hold_ground, active_sensors, case_it, run_point, shadow <ally>, poke_around.", req.Mode)), nil
	}

	// Set mode — replaces any existing mode (REQ-EXP-3).
	sess.ExploreMode = req.Mode
	sess.ExploreShadowTarget = 0

	// Fire immediate room-entry hooks for active_sensors and case_it (REQ-EXP-5).
	if req.Mode == session.ExploreModeActiveSensors || req.Mode == session.ExploreModeCaseIt {
		if room, ok := s.world.GetRoom(sess.RoomID); ok {
			msgs := s.applyExploreModeOnEntry(uid, sess, room)
			for _, msg := range msgs {
				s.pushMessageToUID(uid, msg)
			}
		}
	}

	return messageEvent(fmt.Sprintf("Exploration mode set: %s.", displayName)), nil
}

// exploreDisplayName returns the human-readable display name for a mode ID.
func exploreDisplayName(mode string) string {
	names := map[string]string{
		session.ExploreModeLayLow:        "Lay Low",
		session.ExploreModeHoldGround:    "Hold Ground",
		session.ExploreModeActiveSensors: "Active Sensors",
		session.ExploreModeCaseIt:        "Case It",
		session.ExploreModeRunPoint:      "Run Point",
		session.ExploreModeShadow:         "Shadow",
		session.ExploreModePokeAround:    "Poke Around",
	}
	if n, ok := names[mode]; ok {
		return n
	}
	return mode
}
```

**Note on `pushMessageToUID`**: This is a helper that pushes a console message to a player's entity channel. Add it if it does not already exist, following the inline push pattern used throughout `handleMove`:

```go
// pushMessageToUID pushes a console message to the given player's entity stream.
// Silently drops the message if the player session or entity is not found.
func (s *GameServiceServer) pushMessageToUID(uid, content string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || sess.Entity == nil {
		return
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: content,
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = sess.Entity.Push(data)
	}
}
```

**Note on `GetPlayerByCharID`**: If this method does not exist on `session.Manager`, add it:

```go
// GetPlayerByCharID returns the PlayerSession whose CharacterID matches id, or nil.
func (m *Manager) GetPlayerByCharID(id int64) *PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.players {
		if sess.CharacterID == id {
			return sess
		}
	}
	return nil
}
```

The Shadow mode constant is `ExploreModeShadow = "shadow"` (added in Task 1).

- [ ] **Step 3: Build and verify**

```bash
mise exec -- go build ./...
```

Expected: clean build.

- [ ] **Step 4: Run existing tests**

```bash
mise exec -- go test ./internal/gameserver/... -count=1 -timeout 120s 2>&1 | tail -5
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/game/session/manager.go
git commit -m "feat(explore): implement handleExplore command handler"
```

---

## Task 5: Room-Entry Exploration Hooks (`grpc_service_explore.go`)

**Files:**
- Create: `internal/gameserver/grpc_service_explore.go`
- Create: `internal/gameserver/grpc_service_explore_test.go`

These hooks handle: **Lay Low**, **Active Sensors**, **Case It**, **Poke Around**, and **Shadow** (room-entry effects only). Run Point has no room-entry effect — it fires at combat start.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_explore_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestExploreMode_LayLow_NoNPCs_NoCheck verifies that Lay Low fires no check when
// no NPCs are present (REQ-EXP-10).
func TestExploreMode_LayLow_NoNPCs_NoCheck(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := newTestGameServiceServer(t)
		uid, sess := addTestPlayer(t, s, "TestPlayer")
		sess.ExploreMode = session.ExploreModeLayLow

		room := &world.Room{ID: "r1", ZoneID: "z1"}
		// No NPCs in room — s.npcMgr returns empty slice for unknown rooms.
		msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
		require.Empty(t, msgs, "no messages expected when room has no NPCs")
		require.Empty(t, sess.Conditions.All(), "no conditions expected")
	})
}

// TestExploreMode_LayLow_CritSuccess_AppliesHiddenAndUndetected verifies REQ-EXP-8.
func TestExploreMode_LayLow_CritSuccess_AppliesHiddenAndUndetected(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeLayLow
	// Force a roll result that guarantees crit success: set NPC Awareness=0 (DC=10), player
	// ghosting rank=legendary (+8 proficiency), and fix dice to roll 20.
	// Total = 20 + abilityMod + 8 >= 10+10 = 20 => crit success.
	s.SetDiceFixed(20)
	sess.Skills["ghosting"] = "legendary"

	inst := &npc.Instance{ID: "npc1", Awareness: 0}
	s.npcMgr.PlaceInRoom("r1", inst)

	room := &world.Room{ID: "r1", ZoneID: "z1"}
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.NotEmpty(t, msgs)
	require.True(t, sess.Conditions.Has("hidden"), "expected hidden condition")
	require.True(t, sess.Conditions.Has("undetected"), "expected undetected condition")
}

// TestExploreMode_LayLow_CritFail_BlocksHidden verifies REQ-EXP-8a.
func TestExploreMode_LayLow_CritFail_BlocksHidden(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeLayLow
	// Force crit failure: roll 1, no proficiency, NPC Awareness=15 (DC=25). 1 < 25-10=15 => crit fail.
	s.SetDiceFixed(1)
	sess.Skills["ghosting"] = ""

	inst := &npc.Instance{ID: "npc1", Awareness: 15}
	s.npcMgr.PlaceInRoom("r1", inst)

	room := &world.Room{ID: "r1", ZoneID: "z1"}
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.NotEmpty(t, msgs)
	require.Equal(t, "r1", sess.LayLowBlockedRoom, "LayLowBlockedRoom should be set on crit fail")
	require.False(t, sess.Conditions.Has("hidden"))
}

// TestExploreMode_ActiveSensors_Success_SendsMessage verifies REQ-EXP-15.
func TestExploreMode_ActiveSensors_Success_SendsMessage(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeActiveSensors
	s.SetDiceFixed(15)
	sess.Skills["tech_lore"] = "trained"

	room := &world.Room{ID: "r1", ZoneID: "z1", DangerLevel: "safe"} // DC=12
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.NotEmpty(t, msgs, "success should produce a message")
}

// TestExploreMode_ActiveSensors_Failure_NoMessage verifies REQ-EXP-17.
func TestExploreMode_ActiveSensors_Failure_NoMessage(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeActiveSensors
	s.SetDiceFixed(1) // guaranteed failure at any DC
	sess.Skills["tech_lore"] = ""

	room := &world.Room{ID: "r1", ZoneID: "z1", DangerLevel: "safe"}
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.Empty(t, msgs)
}

// TestExploreMode_CaseIt_Success_SendsMessage verifies REQ-EXP-20.
func TestExploreMode_CaseIt_Success_SendsMessage(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeCaseIt
	s.SetDiceFixed(15)
	sess.Skills["awareness"] = "trained"

	room := &world.Room{ID: "r1", ZoneID: "z1", DangerLevel: "safe"} // DC=12
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.NotEmpty(t, msgs)
}

// TestExploreMode_PokeAround_ContextFaction_UsesCorrectSkill verifies REQ-EXP-34.
func TestExploreMode_PokeAround_ContextFaction_UsesCorrectSkill(t *testing.T) {
	s := newTestGameServiceServer(t)
	uid, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModePokeAround
	s.SetDiceFixed(20) // guarantee crit success for message check

	room := &world.Room{
		ID:         "r1",
		ZoneID:     "z1",
		Properties: map[string]string{"context": "faction"},
	}
	msgs := s.ApplyExploreModeOnEntry(uid, sess, room)
	require.NotEmpty(t, msgs)
}
```

- [ ] **Step 2: Run tests — expect failure (function not exported yet)**

```bash
mise exec -- go test ./internal/gameserver/... -run TestExploreMode -v 2>&1 | head -20
```

Expected: compile errors or FAIL.

- [ ] **Step 3: Create `internal/gameserver/grpc_service_explore.go`**

```go
package gameserver

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/danger"
)

// applyExploreModeOnEntry fires the room-entry exploration hook for the player's
// active mode. Called from handleMove after the room description has been sent.
//
// Precondition: uid, sess, and room must be non-nil; sess.ExploreMode is set.
// Postcondition: Returns console messages to send to the player; sess state may be mutated.
func (s *GameServiceServer) applyExploreModeOnEntry(uid string, sess *session.PlayerSession, room *world.Room) []string {
	switch sess.ExploreMode {
	case session.ExploreModeLayLow:
		return s.exploreLayLow(uid, sess, room)
	case session.ExploreModeActiveSensors:
		return s.exploreActiveSensors(uid, sess, room)
	case session.ExploreModeCaseIt:
		return s.exploreCaseIt(uid, sess, room)
	case session.ExploreModePokeAround:
		return s.explorePokeAround(uid, sess, room)
	case session.ExploreModeShadow:
		return s.exploreShadow(uid, sess, room)
	}
	// lay_low, hold_ground, run_point have no room-entry effect (or are handled elsewhere).
	return nil
}

// ApplyExploreModeOnEntry is the exported wrapper used by tests.
func (s *GameServiceServer) ApplyExploreModeOnEntry(uid string, sess *session.PlayerSession, room *world.Room) []string {
	return s.applyExploreModeOnEntry(uid, sess, room)
}

// exploreDangerDC returns the skill check DC for the given room based on its effective
// danger level. Unset danger level defaults to 16 (Sketchy) per REQ-EXP-18 and REQ-EXP-23.
func (s *GameServiceServer) exploreDangerDC(room *world.Room) int {
	var zoneDangerLevel string
	if zone, ok := s.world.GetZone(room.ZoneID); ok {
		zoneDangerLevel = zone.DangerLevel
	}
	effective := danger.EffectiveDangerLevel(zoneDangerLevel, room.DangerLevel)
	switch effective {
	case "safe":
		return 12
	case "sketchy":
		return 16
	case "dangerous":
		return 20
	case "all_out_war":
		return 24
	default:
		return 16 // Sketchy fallback
	}
}

// exploreRoll rolls a d20 secret check and resolves the outcome against dc.
func (s *GameServiceServer) exploreRoll(sess *session.PlayerSession, skill string, dc int) skillcheck.CheckOutcome {
	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}
	abilityScore := s.abilityScoreForSkill(sess, skill)
	amod := abilityModFrom(abilityScore)
	rank := ""
	if sess.Skills != nil {
		rank = sess.Skills[skill]
	}
	result := skillcheck.Resolve(roll, amod, rank, dc, skillcheck.TriggerDef{})
	return result.Outcome
}

// exploreLayLow handles the Lay Low mode room-entry hook (REQ-EXP-7 through REQ-EXP-10).
func (s *GameServiceServer) exploreLayLow(uid string, sess *session.PlayerSession, room *world.Room) []string {
	// No NPCs — no check (REQ-EXP-10).
	insts := s.npcMgr.InstancesInRoom(room.ID)
	var liveNPCs []*npc.Instance
	for _, inst := range insts {
		if !inst.IsDead() {
			liveNPCs = append(liveNPCs, inst)
		}
	}
	if len(liveNPCs) == 0 {
		return nil
	}

	// Highest NPC Awareness DC (REQ-EXP-7).
	maxAwareness := 0
	for _, inst := range liveNPCs {
		if inst.Awareness > maxAwareness {
			maxAwareness = inst.Awareness
		}
	}
	dc := 10 + maxAwareness

	outcome := s.exploreRoll(sess, "ghosting", dc)
	switch outcome {
	case skillcheck.CritSuccess:
		s.applyCondition(uid, sess, "hidden")
		s.applyCondition(uid, sess, "undetected")
		return []string{"You slip into the shadows completely unnoticed."}
	case skillcheck.Success:
		s.applyCondition(uid, sess, "hidden")
		return []string{"You blend into the background, staying hidden."}
	case skillcheck.CritFailure:
		sess.LayLowBlockedRoom = room.ID
		return []string{"You fumble your attempt at stealth — NPCs in the area are on alert."}
	default: // Failure
		return nil
	}
}

// exploreActiveSensors handles the Active Sensors mode room-entry hook (REQ-EXP-14 through REQ-EXP-18).
func (s *GameServiceServer) exploreActiveSensors(uid string, sess *session.PlayerSession, room *world.Room) []string {
	dc := s.exploreDangerDC(room)
	outcome := s.exploreRoll(sess, "tech_lore", dc)

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		// Enumerate technology in the room.
		var items []string
		// Room equipment slots.
		for _, slot := range room.Equipment {
			if slot.ItemID != "" {
				items = append(items, slot.ItemID)
			}
		}
		// NPC-carried technology.
		for _, inst := range s.npcMgr.InstancesInRoom(room.ID) {
			if !inst.IsDead() && inst.Type == "robot" || inst.Type == "machine" {
				items = append(items, inst.Name())
			}
		}
		if outcome == skillcheck.CritSuccess {
			items = append(items, s.concealedTechInRoom(room)...)
		}
		if len(items) == 0 {
			return []string{"Your sensors detect no active technology in this area."}
		}
		return []string{fmt.Sprintf("Your active sensors detect: %s.", strings.Join(items, ", "))}
	default:
		return nil
	}
}

// concealedTechInRoom returns descriptions of concealed technology in the room.
// Uses RoomTrapConfig.TemplateID (the only available field on that struct).
func (s *GameServiceServer) concealedTechInRoom(room *world.Room) []string {
	var out []string
	for _, trap := range room.Traps {
		if trap.TemplateID != "" {
			out = append(out, fmt.Sprintf("concealed trap (%s)", trap.TemplateID))
		}
	}
	return out
}

// exploreCaseIt handles the Case It mode room-entry hook (REQ-EXP-19 through REQ-EXP-24).
func (s *GameServiceServer) exploreCaseIt(uid string, sess *session.PlayerSession, room *world.Room) []string {
	dc := s.exploreDangerDC(room)
	outcome := s.exploreRoll(sess, "awareness", dc)

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		var findings []string
		// Hidden exits.
		for _, exit := range room.Exits {
			if exit.Hidden {
				findings = append(findings, fmt.Sprintf("hidden exit (%s)", exit.Direction))
			}
		}
		// Traps (REQ-EXP-20). RoomTrapConfig only has TemplateID and Position.
		// Crit success reveals template ID (closest approximation to "trap type"); success shows position.
		for _, trap := range room.Traps {
			entry := fmt.Sprintf("trap at %s", trap.Position)
			if outcome == skillcheck.CritSuccess {
				entry = fmt.Sprintf("trap: %s at %s", trap.TemplateID, trap.Position)
			}
			findings = append(findings, entry)
		}
		if len(findings) == 0 {
			return []string{"You look around carefully but find nothing out of the ordinary."}
		}
		return []string{fmt.Sprintf("You notice: %s.", strings.Join(findings, "; "))}
	default:
		return nil
	}
}

// explorePokeAround handles the Poke Around mode room-entry hook (REQ-EXP-33 through REQ-EXP-37).
func (s *GameServiceServer) explorePokeAround(uid string, sess *session.PlayerSession, room *world.Room) []string {
	skill, dc := pokeAroundSkillAndDC(sess, room)
	outcome := s.exploreRoll(sess, skill, dc)

	facts := s.loreFacts(room, outcome)
	if len(facts) == 0 {
		return nil
	}
	var msgs []string
	for _, f := range facts {
		msgs = append(msgs, f)
	}
	return msgs
}

// pokeAroundSkillAndDC selects the Recall Knowledge skill and DC based on room context (REQ-EXP-34).
func pokeAroundSkillAndDC(sess *session.PlayerSession, room *world.Room) (string, int) {
	ctx := ""
	if room.Properties != nil {
		ctx = room.Properties["context"]
	}
	switch ctx {
	case "history":
		return "intel", 15
	case "faction":
		// Conspiracy vs Factions — use higher rank; ties use Conspiracy (REQ-EXP-37).
		conspRank := skillRankBonus(sess.Skills["conspiracy"])
		factRank := skillRankBonus(sess.Skills["factions"])
		if factRank > conspRank {
			return "factions", 17
		}
		return "conspiracy", 17
	case "technology":
		return "tech_lore", 16
	case "creature":
		return "wasteland", 14
	default:
		return "intel", 15
	}
}

// loreFacts returns lore strings for the room based on outcome.
// Success returns 1 fact; critical success returns 2. Failure returns nil.
// False information is never generated (REQ-EXP-36).
func (s *GameServiceServer) loreFacts(room *world.Room, outcome skillcheck.CheckOutcome) []string {
	if outcome == skillcheck.Failure || outcome == skillcheck.CritFailure {
		return nil
	}
	facts := room.LoreFacts()
	if len(facts) == 0 {
		return nil
	}
	if outcome == skillcheck.CritSuccess && len(facts) >= 2 {
		return facts[:2]
	}
	return facts[:1]
}

// exploreShadow handles the Shadow mode room-entry hook (REQ-EXP-28 through REQ-EXP-32).
// Shadow borrows the ally's skill rank for the first room-entry skill check (via applyRoomSkillChecks).
// This function validates ally presence; rank override occurs in applyRoomSkillChecks.
func (s *GameServiceServer) exploreShadow(uid string, sess *session.PlayerSession, room *world.Room) []string {
	// Suspend silently if ally is not in the same room (REQ-EXP-31).
	ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget)
	if ally == nil || ally.RoomID != sess.RoomID {
		return nil
	}
	// Shadow itself produces no message; the rank override fires inside applyRoomSkillChecks.
	return nil
}

// applyCondition applies a named condition to a player session.
// Silently skips if the condition is not in the registry.
func (s *GameServiceServer) applyCondition(uid string, sess *session.PlayerSession, condID string) {
	if s.condRegistry == nil {
		return
	}
	def, ok := s.condRegistry.Get(condID)
	if !ok {
		return
	}
	if sess.Conditions == nil {
		return
	}
	_ = sess.Conditions.Apply(uid, def, 1, -1)
}
```

**Note on `room.LoreFacts()`**: This method needs to exist on `world.Room`. Check if it already does. If not, add it to `internal/game/world/model.go`:

```go
// LoreFacts returns the room's lore fact strings from Properties["lore_facts"].
// Facts are stored as a newline-separated string. Returns nil if not set.
func (r *Room) LoreFacts() []string {
	if r.Properties == nil {
		return nil
	}
	raw := r.Properties["lore_facts"]
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
```

**`room.Equipment`** is `[]world.RoomEquipmentConfig` with field `ItemID string` — already used correctly above.

**`world.RoomTrapConfig`** has only two fields: `TemplateID string` and `Position string`. There is no `TrapType`, `DisarmDC`, or `TriggerType` field. The code above uses `trap.TemplateID` and `trap.Position` correctly.

- [ ] **Step 4: Build and run tests**

```bash
mise exec -- go build ./...
mise exec -- go test ./internal/gameserver/... -run TestExploreMode -v
```

Expected: all exploration mode tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service_explore.go internal/gameserver/grpc_service_explore_test.go \
        internal/game/world/model.go
git commit -m "feat(explore): implement room-entry exploration mode hooks"
```

---

## Task 6: Wire Room-Entry Hook into `handleMove` + Shadow Rank Override in `applyRoomSkillChecks`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Clear `LayLowBlockedRoom` and wire exploration hook in `handleMove`**

In `grpc_service.go`, inside `handleMove`, after the `checkEntryTraps` block (around line 2130):

```go
// Clear LayLow crit-fail block from previous room (REQ-EXP-8a).
sess.LayLowBlockedRoom = ""

// Fire exploration mode room-entry hook (REQ-EXP-38).
if sess.ExploreMode != "" {
    if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
        msgs := s.applyExploreModeOnEntry(uid, sess, newRoom)
        for _, msg := range msgs {
            s.pushMessageToUID(uid, msg)
        }
    }
}
```

- [ ] **Step 2: Shadow rank override in `applyRoomSkillChecks`**

In `applyRoomSkillChecks` (around line 2336), replace the rank lookup:

```go
rank := ""
if sess.Skills != nil {
    rank = sess.Skills[trigger.Skill]
}
```

with:

```go
rank := ""
if sess.Skills != nil {
    rank = sess.Skills[trigger.Skill]
}
// Shadow mode: use ally's rank if higher (REQ-EXP-29, REQ-EXP-30).
if sess.ExploreMode == session.ExploreModeShadow && sess.ExploreShadowTarget != 0 {
    if ally := s.sessions.GetPlayerByCharID(sess.ExploreShadowTarget); ally != nil && ally.RoomID == sess.RoomID {
        if allyRank := ally.Skills[trigger.Skill]; allyRank != "" {
            if skillRankBonus(allyRank) > skillRankBonus(rank) {
                rank = allyRank
            }
        }
    }
}
```

- [ ] **Step 3: Build and run full test suite**

```bash
mise exec -- go test ./internal/gameserver/... -count=1 -timeout 120s 2>&1 | tail -10
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(explore): wire room-entry exploration hook into handleMove; add Shadow rank override"
```

---

## Task 7: Combat-Start Exploration Hook

**Files:**
- Modify: `internal/gameserver/grpc_service_explore.go`
- Modify: `internal/gameserver/combat_handler.go`

- [ ] **Step 1: Add `applyExploreModeOnCombatStart` to `grpc_service_explore.go`**

Append to `grpc_service_explore.go`:

```go
// applyExploreModeOnCombatStart fires the combat-start exploration hook for the player.
// Called after RollInitiative in combat_handler.go before StartCombat.
//
// Precondition: sess, playerCbt, and h must not be nil.
// Postcondition: Lay Low is cleared; Hold Ground applies shield_raised + ACMod;
//   Run Point applies +1 Initiative to co-located players.
func applyExploreModeOnCombatStart(sess *session.PlayerSession, playerCbt *combat.Combatant, h *CombatHandler) []string {
	var msgs []string

	// Lay Low: clear before other hooks (REQ-EXP-40).
	if sess.ExploreMode == session.ExploreModeLayLow {
		sess.ExploreMode = ""
		// No message — mode clears silently at combat start.
	}

	// Hold Ground: apply shield_raised at no AP cost (REQ-EXP-11, REQ-EXP-12).
	if sess.ExploreMode == session.ExploreModeHoldGround {
		if hasShieldEquipped(sess) {
			// Apply condition.
			if h.condRegistry != nil {
				if def, ok := h.condRegistry.Get("shield_raised"); ok {
					if sess.Conditions != nil {
						_ = sess.Conditions.Apply(sess.UID, def, 1, -1)
					}
				}
			}
			// Apply +2 ACMod to combatant.
			playerCbt.ACMod += 2
			msgs = append(msgs, "Hold Ground: your shield is already raised.")
		}
		// No error if no shield (REQ-EXP-12).
	}

	// Run Point: +1 circumstance bonus to Initiative for all other players in room (REQ-EXP-25, REQ-EXP-26).
	if sess.ExploreMode == session.ExploreModeRunPoint {
		others := h.sessions.PlayersInRoomDetails(sess.RoomID)
		for _, other := range others {
			if other.UID == sess.UID {
				continue // REQ-EXP-26: Run Point player does not receive the bonus.
			}
			// Adjust the combatant's initiative by +1 if they are already enrolled in this combat.
			if h.engine != nil {
				if cbt, ok := h.engine.GetCombatant(sess.RoomID, other.UID); ok {
					cbt.Initiative += 1
				}
			}
		}
		msgs = append(msgs, "Run Point: your allies gain +1 to Initiative.")
	}

	return msgs
}

// hasShieldEquipped returns true if the player has a shield in the off-hand slot.
func hasShieldEquipped(sess *session.PlayerSession) bool {
	if sess.LoadoutSet == nil {
		return false
	}
	preset := sess.LoadoutSet.ActivePreset()
	return preset != nil && preset.OffHand != nil && preset.OffHand.Def.IsShield()
}
```

**Note on `playerCbt.ACMod`**: Check whether `combat.Combatant` has an `ACMod int` field. If not, add it (in `internal/game/combat/combat.go`):

```go
// ACMod is a transient modifier applied during combat (e.g., from Hold Ground, Take Cover).
ACMod int
```

Then verify `combat.Combatant.AC` is computed as `baseAC + ACMod` in the resolver. If the resolver uses `AC` directly and `ACMod` is not yet incorporated, add the adjustment in `StartCombat` or the round resolver where AC is read.

**Note on `h.engine.GetCombatant`**: If this method does not exist, add it to the `Engine` interface and implementation in `internal/game/combat/engine.go`:

```go
// GetCombatant returns the Combatant for uid in the given room, and whether it was found.
func (e *Engine) GetCombatant(roomID, uid string) (*Combatant, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cbt, ok := e.combats[roomID]
	if !ok {
		return nil, false
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			return c, true
		}
	}
	return nil, false
}
```

- [ ] **Step 2: Write failing combat-start hook tests**

Add to `grpc_service_explore_test.go`:

```go
// TestExploreMode_HoldGround_ShieldEquipped_AppliesShieldRaised verifies REQ-EXP-11.
func TestExploreMode_HoldGround_ShieldEquipped_AppliesShieldRaised(t *testing.T) {
	s := newTestGameServiceServer(t)
	_, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeHoldGround
	addTestShield(t, sess) // helper that puts a shield in the off-hand slot

	cbt := &combat.Combatant{ID: sess.UID, Kind: combat.KindPlayer, Initiative: 10}
	msgs := ApplyExploreModeOnCombatStartForTest(sess, cbt, s.combatH)
	require.Contains(t, msgs[0], "Hold Ground")
	require.True(t, sess.Conditions.Has("shield_raised"))
	require.Equal(t, 2, cbt.ACMod)
}

// TestExploreMode_HoldGround_NoShield_Silent verifies REQ-EXP-12.
func TestExploreMode_HoldGround_NoShield_Silent(t *testing.T) {
	s := newTestGameServiceServer(t)
	_, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeHoldGround

	cbt := &combat.Combatant{ID: sess.UID, Kind: combat.KindPlayer}
	msgs := ApplyExploreModeOnCombatStartForTest(sess, cbt, s.combatH)
	require.Empty(t, msgs)
	require.Equal(t, 0, cbt.ACMod)
}

// TestExploreMode_LayLow_ClearedAtCombatStart verifies REQ-EXP-40.
func TestExploreMode_LayLow_ClearedAtCombatStart(t *testing.T) {
	s := newTestGameServiceServer(t)
	_, sess := addTestPlayer(t, s, "TestPlayer")
	sess.ExploreMode = session.ExploreModeLayLow

	cbt := &combat.Combatant{ID: sess.UID, Kind: combat.KindPlayer}
	ApplyExploreModeOnCombatStartForTest(sess, cbt, s.combatH)
	require.Equal(t, "", sess.ExploreMode, "Lay Low must be cleared at combat start")
}
```

Add the exported test shim to `grpc_service_explore.go`:

```go
// ApplyExploreModeOnCombatStartForTest is an exported test shim.
func ApplyExploreModeOnCombatStartForTest(sess *session.PlayerSession, playerCbt *combat.Combatant, h *CombatHandler) []string {
	return applyExploreModeOnCombatStart(sess, playerCbt, h)
}
```

- [ ] **Step 3: Wire into `combat_handler.go`**

In `combat_handler.go`, after `combat.RollInitiative(combatants, h.dice.Src())` at line ~2177, add:

```go
// Fire exploration mode combat-start hook (REQ-EXP-39, REQ-EXP-40).
if combatMsgs := applyExploreModeOnCombatStart(sess, playerCbt, h); len(combatMsgs) > 0 {
    for _, msg := range combatMsgs {
        h.pushMessageToUID(sess.UID, msg)
    }
}
```

**Note on `h.pushMessageToUID`**: If `CombatHandler` does not have this helper, add it (similar pattern to the one defined in Task 4 for `GameServiceServer`):

```go
func (h *CombatHandler) pushMessageToUID(uid, content string) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.Entity == nil {
		return
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: content,
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = sess.Entity.Push(data)
	}
}
```

- [ ] **Step 4: Build and run all tests**

```bash
mise exec -- go build ./...
mise exec -- go test ./... -count=1 -timeout 180s 2>&1 | tail -15
```

Expected: all tests pass, no regressions.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service_explore.go internal/gameserver/grpc_service_explore_test.go \
        internal/gameserver/combat_handler.go internal/game/combat/combat.go \
        internal/game/combat/engine.go
git commit -m "feat(explore): implement combat-start hook (Hold Ground, Run Point, Lay Low clear)"
```

---

## Task 8: Feature Status + Final Verification

**Files:**
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Set feature status to `in_progress`**

In `docs/features/index.yaml`, change:

```yaml
  - slug: exploration
    name: Exploration Mode
    status: planned
```

to:

```yaml
  - slug: exploration
    name: Exploration Mode
    status: in_progress
```

- [ ] **Step 2: Run full test suite**

```bash
mise exec -- go test ./... -count=1 -timeout 180s
```

Expected: 100% pass rate.

- [ ] **Step 3: Set feature status to `done`**

Change `status: in_progress` to `status: done`.

- [ ] **Step 4: Final commit**

```bash
git add docs/features/index.yaml
git commit -m "feat(explore): mark exploration feature done"
```

---

## Spec Coverage Checklist

| Req | Task | Notes |
|-----|------|-------|
| REQ-EXP-1 | Task 4 | `handleExplore` sets mode |
| REQ-EXP-2 | Task 4 | `explore off` clears mode |
| REQ-EXP-3 | Task 4 | Setting new mode replaces old |
| REQ-EXP-4 | Task 4 | Rejected in combat |
| REQ-EXP-5 | Task 4 | Immediate hook for `active_sensors`/`case_it` |
| REQ-EXP-6 | Task 4 | Shadow requires valid ally name |
| REQ-EXP-7–10 | Task 5 | Lay Low hook |
| REQ-EXP-11–13 | Task 7 | Hold Ground hook |
| REQ-EXP-14–18 | Task 5 | Active Sensors hook |
| REQ-EXP-19–24 | Task 5 | Case It hook |
| REQ-EXP-25–27 | Task 7 | Run Point hook |
| REQ-EXP-28–32 | Tasks 5, 6 | Shadow hook + rank override in `applyRoomSkillChecks` |
| REQ-EXP-33–37 | Task 5 | Poke Around hook |
| REQ-EXP-38 | Task 6 | Room-entry hook wired after room description |
| REQ-EXP-39 | Task 7 | Combat-start hook fires before Initiative finalized |
| REQ-EXP-40 | Task 7 | Lay Low cleared at combat start first |
