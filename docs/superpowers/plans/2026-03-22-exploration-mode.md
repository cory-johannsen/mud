# Exploration Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the `explore` command with seven persistent stances that fire effects on room entry and combat start.

**Architecture:** `ExploreMode string` already exists on `PlayerSession`; `case_it` is already checked by the traps system. This plan expands the stub into a full command with all seven modes. Mode logic lives in `internal/game/explore/` (pure functions). Hook dispatch lives in `grpc_service.go` (`onRoomEntry`, `onCombatStart`). No DB persistence — session-only state.

**Tech Stack:** Go, protobuf (buf), testify, rapid

**Dependencies:** `actions` (technology activation), `npc-awareness` (Instance.Awareness field — already present in codebase)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/game/session/manager.go` | Add `ExploreShadowTarget int64`, `ExploreLayLowCritFailRoom string`; add mode ID constants |
| Add | `internal/game/explore/explore.go` | Pure mode logic: mode validation, lay_low check, active_sensors, case_it, run_point, shadow, poke_around |
| Add | `internal/game/explore/explore_test.go` | Unit + property-based tests for all pure functions |
| Modify | `internal/game/command/commands.go` | Add `HandlerExplore` constant and `BuiltinCommands()` entry |
| Modify | `api/proto/game/v1/game.proto` | Add `ExploreRequest` message and oneof entry in `ClientMessage` |
| Regenerate | `api/proto/game/v1/game.pb.go` | `buf generate` |
| Modify | `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeExplore` handler and map entry |
| Modify | `internal/gameserver/grpc_service.go` | Add `handleExplore` case; add `onRoomEntry` call in `handleMove`; expand `onCombatStart` in `startCombatLocked` |

---

### Task 1: Session fields and mode ID constants

**Files:**
- Modify: `internal/game/session/manager.go`

The field `ExploreMode string` (line ~126) and constant `ExploreModeCaseIt = "case_it"` (line ~16) already exist. This task adds the remaining constants and two new fields.

- [ ] **Step 1: Add mode ID constants**

In `internal/game/session/manager.go`, after `ExploreModeCaseIt`, add:

```go
const (
    ExploreModeCaseIt      = "case_it"       // already exists — do not duplicate
    ExploreModeLayLow      = "lay_low"
    ExploreModeHoldGround  = "hold_ground"
    ExploreModeActiveSensors = "active_sensors"
    ExploreModeRunPoint    = "run_point"
    ExploreModeShadow      = "shadow"
    ExploreModePokeAround  = "poke_around"
)
```

Note: `ExploreModeCaseIt` already exists as a standalone const — replace the single-const declaration with a `const (...)` block that includes all seven.

- [ ] **Step 2: Add new fields to `PlayerSession`**

After the existing `ExploreMode string` field, add:

```go
// ExploreShadowTarget is the UID of the ally being shadowed.
// Only meaningful when ExploreMode == ExploreModeShadow.
ExploreShadowTarget string

// ExploreLayLowCritFailRoom is the room ID where a Lay Low critical failure
// occurred. Player cannot gain hidden/undetected in this room until they leave.
// Cleared when player exits the room.
ExploreLayLowCritFailRoom string

// HoldGroundPending is true when Hold Ground mode is active and the player
// has not yet received their first AP grant in this combat.
HoldGroundPending bool
```

- [ ] **Step 3: Add `LoreFacts []string` to `Room` struct**

In `internal/game/world/model.go`, in the `Room` struct, add:

```go
// LoreFacts is an optional list of contextual lore strings revealed by Poke Around.
// Empty slice means nothing to reveal.
LoreFacts []string `yaml:"lore_facts,omitempty"`
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/game/session/manager.go internal/game/world/model.go
git commit -m "feat(exploration): add ExploreMode constants, shadow/lay-low/hold-ground session fields, Room.LoreFacts"
```

---

### Task 2: Pure exploration mode logic package

**Files:**
- Create: `internal/game/explore/explore.go`
- Create: `internal/game/explore/explore_test.go`

This package contains only pure functions — no DB, no session pointers, no proto — so it is fully testable in isolation.

- [ ] **Step 1: Write the tests first**

```go
// internal/game/explore/explore_test.go
package explore_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/explore"
)

func TestValidateMode(t *testing.T) {
    for _, m := range []string{"lay_low", "hold_ground", "active_sensors", "case_it", "run_point", "shadow", "poke_around"} {
        assert.True(t, explore.ValidMode(m), "expected valid: %s", m)
    }
    assert.False(t, explore.ValidMode(""))
    assert.False(t, explore.ValidMode("sneak"))
    assert.False(t, explore.ValidMode("off")) // "off" is a clear, not a mode
}

func TestDangerDC(t *testing.T) {
    assert.Equal(t, 12, explore.DangerDC("safe"))
    assert.Equal(t, 16, explore.DangerDC("sketchy"))
    assert.Equal(t, 20, explore.DangerDC("dangerous"))
    assert.Equal(t, 24, explore.DangerDC("all_out_war"))
    assert.Equal(t, 16, explore.DangerDC(""))    // unset defaults to sketchy
    assert.Equal(t, 16, explore.DangerDC("unknown"))
}

func TestPokeAroundContext(t *testing.T) {
    skill, dc := explore.PokeAroundContext("history")
    assert.Equal(t, "intel", skill)
    assert.Equal(t, 15, dc)

    skill, dc = explore.PokeAroundContext("faction")
    assert.Equal(t, "conspiracy", skill) // default when no ranks provided
    assert.Equal(t, 17, dc)

    skill, dc = explore.PokeAroundContext("technology")
    assert.Equal(t, "tech_lore", skill)
    assert.Equal(t, 16, dc)

    skill, dc = explore.PokeAroundContext("creature")
    assert.Equal(t, "wasteland", skill)
    assert.Equal(t, 14, dc)

    skill, dc = explore.PokeAroundContext("") // unset
    assert.Equal(t, "intel", skill)
    assert.Equal(t, 15, dc)
}

func TestPokeAroundContext_FactionRankTieBreak(t *testing.T) {
    // When both conspiracy and factions ranks are available, higher rank wins.
    // Ties use conspiracy.
    skill := explore.PokeAroundFactionSkill(2, 1) // conspiracy=expert, factions=trained → conspiracy
    assert.Equal(t, "conspiracy", skill)

    skill = explore.PokeAroundFactionSkill(1, 2) // conspiracy=trained, factions=expert → factions
    assert.Equal(t, "factions", skill)

    skill = explore.PokeAroundFactionSkill(2, 2) // tie → conspiracy
    assert.Equal(t, "conspiracy", skill)
}

func TestImmediateFire(t *testing.T) {
    assert.True(t, explore.ImmediateFire("active_sensors"))
    assert.True(t, explore.ImmediateFire("case_it"))
    assert.False(t, explore.ImmediateFire("lay_low"))
    assert.False(t, explore.ImmediateFire("hold_ground"))
    assert.False(t, explore.ImmediateFire("poke_around"))
}

func TestDisplayName(t *testing.T) {
    assert.Equal(t, "Lay Low", explore.DisplayName("lay_low"))
    assert.Equal(t, "Hold Ground", explore.DisplayName("hold_ground"))
    assert.Equal(t, "Active Sensors", explore.DisplayName("active_sensors"))
    assert.Equal(t, "Case It", explore.DisplayName("case_it"))
    assert.Equal(t, "Run Point", explore.DisplayName("run_point"))
    assert.Equal(t, "Shadow", explore.DisplayName("shadow"))
    assert.Equal(t, "Poke Around", explore.DisplayName("poke_around"))
}

func TestValidModeProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        s := rapid.String().Draw(t, "s")
        valid := explore.ValidMode(s)
        if valid {
            assert.NotEmpty(t, explore.DisplayName(s))
        }
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/explore/... -v
```

Expected: compilation error — package does not exist.

- [ ] **Step 3: Implement the package**

```go
// internal/game/explore/explore.go
package explore

// validModes is the canonical set of explore mode IDs.
var validModes = map[string]string{
    "lay_low":        "Lay Low",
    "hold_ground":    "Hold Ground",
    "active_sensors": "Active Sensors",
    "case_it":        "Case It",
    "run_point":      "Run Point",
    "shadow":         "Shadow",
    "poke_around":    "Poke Around",
}

// immediateFireModes fire their room-entry hook immediately on mode set.
var immediateFireModes = map[string]bool{
    "active_sensors": true,
    "case_it":        true,
}

// ValidMode returns true if m is a valid exploration mode ID.
func ValidMode(m string) bool {
    _, ok := validModes[m]
    return ok
}

// DisplayName returns the human-readable name for a mode ID.
// Returns empty string for unknown modes.
func DisplayName(m string) string {
    return validModes[m]
}

// ImmediateFire returns true if the mode fires its room-entry hook on mode set.
func ImmediateFire(m string) bool {
    return immediateFireModes[m]
}

// DangerDC returns the skill check DC for a given room danger level string.
// Unset or unknown danger levels default to 16 (Sketchy).
func DangerDC(dangerLevel string) int {
    switch dangerLevel {
    case "safe":
        return 12
    case "sketchy":
        return 16
    case "dangerous":
        return 20
    case "all_out_war":
        return 24
    default:
        return 16
    }
}

// PokeAroundContext returns the skill ID and DC for a Poke Around check
// based on the room's "context" property. conspiracyRank and factionsRank
// are only used when context == "faction".
func PokeAroundContext(context string) (skill string, dc int) {
    switch context {
    case "history":
        return "intel", 15
    case "faction":
        return "conspiracy", 17 // default; caller should use PokeAroundFactionSkill for rank comparison
    case "technology":
        return "tech_lore", 16
    case "creature":
        return "wasteland", 14
    default:
        return "intel", 15
    }
}

// PokeAroundFactionSkill returns "factions" if factionsRank > conspiracyRank,
// otherwise "conspiracy" (ties use conspiracy).
func PokeAroundFactionSkill(conspiracyRank, factionsRank int) string {
    if factionsRank > conspiracyRank {
        return "factions"
    }
    return "conspiracy"
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/explore/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/explore/
git commit -m "feat(exploration): add pure explore mode logic package"
```

---

### Task 3: Command registration (CMD pattern)

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `api/proto/game/v1/game.pb.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add handler constant and BuiltinCommands entry**

In `internal/game/command/commands.go`:

Add constant (in the existing block of `Handler*` constants):
```go
HandlerExplore = "explore"
```

Add entry to `BuiltinCommands()` slice:
```go
{
    Name:     "explore",
    Aliases:  []string{"ex"},
    Help:     "Set or clear your exploration mode. Usage: explore [mode|shadow <name>|off]",
    Category: "exploration",
    Handler:  HandlerExplore,
},
```

- [ ] **Step 2: Add proto message and oneof entry**

In `api/proto/game/v1/game.proto`:

Add message (near other simple command request messages):
```protobuf
message ExploreRequest {
    string mode   = 1; // mode ID, "off", or "shadow"
    string target = 2; // ally name (only used when mode == "shadow")
}
```

Add to `ClientMessage` oneof payload (pick the next available field number — verify first):
```protobuf
ExploreRequest explore = <N>;
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
mise exec -- buf generate
```

Expected: `game.pb.go` updated with `ExploreRequest` type and `GetExplore()` accessor.

- [ ] **Step 4: Add bridge handler**

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap`:
```go
command.HandlerExplore: bridgeExplore,
```

Implement the bridge function (near other bridge functions in the same file):
```go
func bridgeExplore(bctx *bridgeContext) (bridgeResult, error) {
    args := strings.Fields(bctx.parsed.RawArgs)
    req := &gamev1.ExploreRequest{}
    if len(args) == 0 {
        req.Mode = "" // query current mode
    } else if strings.EqualFold(args[0], "shadow") && len(args) >= 2 {
        req.Mode = "shadow"
        req.Target = strings.Join(args[1:], " ")
    } else {
        req.Mode = strings.ToLower(args[0])
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Explore{Explore: req},
    }}, nil
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 6: Run full suite**

```bash
mise exec -- go test ./...
```

Expected: 100% pass, no regressions.

- [ ] **Step 7: Commit**

```bash
git add internal/game/command/commands.go api/proto/game/v1/game.proto api/proto/game/v1/game.pb.go internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(exploration): register explore command (CMD pattern: constant, proto, bridge)"
```

---

### Task 4: `handleExplore` in grpc_service.go (command core)

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

This task implements the explore command handler — setting/clearing the mode, rejecting in-combat use, and validating shadow targets. It does NOT yet fire hooks; that comes in Task 5.

- [ ] **Step 1: Write tests for the explore command**

In the gameserver test file, add:

```go
func TestHandleExplore_SetMode(t *testing.T) {
    // Set up a session not in combat
    // Send ExploreRequest{Mode: "lay_low"}
    // Assert sess.ExploreMode == "lay_low"
    // Assert confirmation message sent to player
}

func TestHandleExplore_ClearMode(t *testing.T) {
    // sess.ExploreMode = "lay_low"
    // Send ExploreRequest{Mode: "off"}
    // Assert sess.ExploreMode == ""
}

func TestHandleExplore_ReplacesExistingMode(t *testing.T) {
    // sess.ExploreMode = "lay_low"
    // Send ExploreRequest{Mode: "hold_ground"}
    // Assert sess.ExploreMode == "hold_ground" (no error)
}

func TestHandleExplore_RejectedInCombat(t *testing.T) {
    // sess.Status = InCombat
    // Send ExploreRequest{Mode: "lay_low"}
    // Assert error message returned; sess.ExploreMode unchanged
}

func TestHandleExplore_ShadowRequiresValidTarget(t *testing.T) {
    // No other players in room
    // Send ExploreRequest{Mode: "shadow", Target: "nobody"}
    // Assert error message; ExploreMode not set
}

func TestHandleExplore_QueryCurrentMode(t *testing.T) {
    // sess.ExploreMode = "lay_low"
    // Send ExploreRequest{Mode: ""}
    // Assert response contains "Lay Low"
}
```

Follow existing gameserver test patterns.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleExplore -v
```

Expected: FAIL.

- [ ] **Step 3: Add switch case and implement handleExplore**

In `grpc_service.go`, add to the message dispatch switch:
```go
case *gamev1.ClientMessage_Explore:
    return s.handleExplore(uid, p.Explore)
```

Implement `handleExplore`:
```go
func (s *GameServiceServer) handleExplore(uid string, req *gamev1.ExploreRequest) error {
    sess, ok := s.sessions.Get(uid)
    if !ok {
        return fmt.Errorf("session not found: %s", uid)
    }

    // Reject in combat (REQ-EXP-4)
    // statusInCombat = 2, defined in action_handler.go
    if sess.Status == statusInCombat {
        return s.sendError(uid, "You cannot change exploration mode during combat.")
    }

    mode := strings.ToLower(req.GetMode())

    // Query current mode (REQ-EXP-1 variant: no arg)
    if mode == "" {
        if sess.ExploreMode == "" {
            return s.sendConsole(uid, "No active exploration mode.")
        }
        msg := fmt.Sprintf("Exploration mode: %s", explore.DisplayName(sess.ExploreMode))
        if sess.ExploreMode == session.ExploreModeShadow && sess.ExploreShadowTarget != "" {
            // Check if target is in same room
            inRoom := false
            for _, other := range s.sessions.PlayersInRoomDetails(sess.RoomID) {
                if other.UID == sess.ExploreShadowTarget {
                    inRoom = true
                    break
                }
            }
            suffix := sess.ExploreShadowTarget
            if !inRoom {
                suffix += " — not present"
            }
            msg += fmt.Sprintf(" [shadowing %s]", suffix)
        }
        return s.sendConsole(uid, msg)
    }

    // Clear mode (REQ-EXP-2)
    if mode == "off" {
        sess.ExploreMode = ""
        sess.ExploreShadowTarget = ""
        return s.sendConsole(uid, "Exploration mode cleared.")
    }

    // Shadow requires a target (REQ-EXP-6, REQ-EXP-28)
    if mode == "shadow" {
        target := strings.TrimSpace(req.GetTarget())
        if target == "" {
            return s.sendError(uid, "Usage: explore shadow <player name>")
        }
        // Find target player in same room
        var targetUID string
        for _, other := range s.sessions.PlayersInRoomDetails(sess.RoomID) {
            if strings.EqualFold(other.CharacterName, target) && other.UID != uid {
                targetUID = other.UID
                break
            }
        }
        if targetUID == "" {
            return s.sendError(uid, fmt.Sprintf("No player named %q in this room.", target))
        }
        sess.ExploreMode = session.ExploreModeShadow
        sess.ExploreShadowTarget = targetUID
        return s.sendConsole(uid, fmt.Sprintf("Exploration mode: Shadow [shadowing %s]", target))
    }

    // Validate mode (REQ-EXP-1, REQ-EXP-3)
    if !explore.ValidMode(mode) {
        return s.sendError(uid, fmt.Sprintf("Unknown exploration mode %q. Valid modes: lay_low, hold_ground, active_sensors, case_it, run_point, shadow, poke_around", mode))
    }

    sess.ExploreMode = mode
    sess.ExploreShadowTarget = ""

    // Immediate-fire modes (REQ-EXP-5): fire room-entry hook right now
    if explore.ImmediateFire(mode) {
        room, ok := s.world.GetRoom(sess.RoomID)
        if ok {
            s.onRoomEntry(uid, sess, room)
        }
    }

    return s.sendConsole(uid, fmt.Sprintf("Exploration mode: %s", explore.DisplayName(mode)))
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleExplore -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
mise exec -- go test ./...
```

Expected: 100% pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(exploration): implement handleExplore command (set/clear/query/shadow)"
```

---

### Task 5: Room-entry hook (`onRoomEntry`)

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write tests for room-entry hook dispatch**

```go
func TestOnRoomEntry_LayLow_NoNPCs_NoCheck(t *testing.T) {
    // sess.ExploreMode = "lay_low", room has no NPCs
    // Call onRoomEntry
    // Assert no conditions applied, no message sent (REQ-EXP-10)
}

func TestOnRoomEntry_ActiveSensors_SuccessListsTech(t *testing.T) {
    // sess.ExploreMode = "active_sensors", room has tech items
    // Mock skill check to return Success
    // Assert console message sent with tech list (REQ-EXP-15)
}

func TestOnRoomEntry_CaseIt_Success_RevealsExits(t *testing.T) {
    // sess.ExploreMode = "case_it", room has hidden exits
    // Mock skill check to return Success
    // Assert console message sent (REQ-EXP-20)
    // Assert room view struct NOT modified (REQ-EXP-38)
}
```

Follow existing gameserver test patterns.

- [ ] **Step 2: Implement `onRoomEntry`**

Add `onRoomEntry` to `grpc_service.go`:

```go
// onRoomEntry fires exploration mode hooks for a player entering a room.
// Precondition: called after room description is sent to the player (REQ-EXP-38).
func (s *GameServiceServer) onRoomEntry(uid string, sess *session.PlayerSession, room *world.Room) {
    switch sess.ExploreMode {
    case session.ExploreModeLayLow:
        s.onRoomEntry_LayLow(uid, sess, room)
    case session.ExploreModeActiveSensors:
        s.onRoomEntry_ActiveSensors(uid, sess, room)
    case session.ExploreModeCaseIt:
        s.onRoomEntry_CaseIt(uid, sess, room)
    case session.ExploreModeShadow:
        // Shadow doesn't have its own room-entry effect;
        // it modifies the ally skill rank used by other modes' checks.
        // No direct action here.
    case session.ExploreModePokeAround:
        s.onRoomEntry_PokeAround(uid, sess, room)
    }
    // Clear Lay Low crit-fail room block when player exits the room where it occurred
    if sess.ExploreLayLowCritFailRoom != "" && sess.ExploreLayLowCritFailRoom != room.ID {
        sess.ExploreLayLowCritFailRoom = ""
    }
}
```

- [ ] **Step 3: Call `onRoomEntry` from `handleMove`**

In the `handleMove` function, after the room description (`RoomView`) is sent to the player, add:

```go
if newRoom, ok := s.world.GetRoom(result.View.RoomId); ok {
    s.onRoomEntry(uid, sess, newRoom)
}
```

This must be placed AFTER the `RoomView` send, not before (REQ-EXP-38).

- [ ] **Step 4: Implement Lay Low room-entry handler**

```go
func (s *GameServiceServer) onRoomEntry_LayLow(uid string, sess *session.PlayerSession, room *world.Room) {
    npcs := s.npcH.InstancesInRoom(room.ID)
    if len(npcs) == 0 {
        return // REQ-EXP-10: no check if no NPCs
    }

    // Find highest NPC Awareness DC
    maxDC := 0
    for _, npc := range npcs {
        dc := 10 + npc.Awareness
        if dc > maxDC {
            maxDC = dc
        }
    }

    // Resolve skill rank (Shadow modifier applies)
    ghostingRank := s.resolveSkillRank(uid, sess, "ghosting")
    roll := s.dice.Roll(1, 20, 0)
    abilityMod := (sess.Abilities.Quickness - 10) / 2 // Ghosting uses Quickness (Dex equivalent)
    result := skillcheck.Resolve(roll, abilityMod, ghostingRank, maxDC, skillcheck.TriggerDef{})

    switch result.Outcome {
    case skillcheck.CritSuccess:
        // Apply hidden + undetected (REQ-EXP-8)
        s.applyCondition(uid, sess, "hidden")
        s.applyCondition(uid, sess, "undetected")
    case skillcheck.Success:
        // Apply hidden only (REQ-EXP-8)
        s.applyCondition(uid, sess, "hidden")
    case skillcheck.Failure:
        // No effect (REQ-EXP-8a)
    case skillcheck.CritFailure:
        // Block hidden/undetected in this room for this visit (REQ-EXP-8a)
        sess.ExploreLayLowCritFailRoom = room.ID
    }
    // Roll is secret — never shown to player (REQ-EXP-38)
}
```

- [ ] **Step 5: Implement Active Sensors room-entry handler**

```go
func (s *GameServiceServer) onRoomEntry_ActiveSensors(uid string, sess *session.PlayerSession, room *world.Room) {
    // Room DangerLevel overrides Zone; fall back to zone if room is unset
    dangerLevel := room.DangerLevel
    if dangerLevel == "" {
        if zone, ok := s.world.GetZone(room.ZoneID); ok {
            dangerLevel = zone.DangerLevel
        }
    }
    dc := explore.DangerDC(dangerLevel)
    techLoreRank := s.resolveSkillRank(uid, sess, "tech_lore")
    roll := s.dice.Roll(1, 20, 0)
    abilityMod := (sess.Abilities.Reasoning - 10) / 2 // Tech Lore uses Reasoning
    result := skillcheck.Resolve(roll, abilityMod, techLoreRank, dc, skillcheck.TriggerDef{})

    switch result.Outcome {
    case skillcheck.CritSuccess, skillcheck.Success:
        // Enumerate detectable tech in room
        items := s.detectableTech(room, result.Outcome == skillcheck.CritSuccess)
        if len(items) == 0 {
            s.sendConsole(uid, "Active Sensors: No active technology detected in this area.")
        } else {
            s.sendConsole(uid, fmt.Sprintf("Active Sensors: Detected — %s.", strings.Join(items, ", ")))
        }
    case skillcheck.Failure, skillcheck.CritFailure:
        // No message (REQ-EXP-17)
    }
}
```

`detectableTech(room, includeConcealed bool) []string` — implement as a helper that iterates room equipment slots, floor items, and NPC-carried items to find technology with remaining charges. When `includeConcealed` is true, also include items marked as concealed.

- [ ] **Step 6: Implement Case It room-entry handler**

```go
func (s *GameServiceServer) onRoomEntry_CaseIt(uid string, sess *session.PlayerSession, room *world.Room) {
    dangerLevel := room.DangerLevel
    if dangerLevel == "" {
        if zone, ok := s.world.GetZone(room.ZoneID); ok {
            dangerLevel = zone.DangerLevel
        }
    }
    dc := explore.DangerDC(dangerLevel)
    awarenessRank := s.resolveSkillRank(uid, sess, "awareness")
    roll := s.dice.Roll(1, 20, 0)
    abilityMod := (sess.Abilities.Savvy - 10) / 2 // Awareness uses Savvy (Wis/Perception equivalent)
    result := skillcheck.Resolve(roll, abilityMod, awarenessRank, dc, skillcheck.TriggerDef{})

    switch result.Outcome {
    case skillcheck.CritSuccess:
        // Reveal hidden exits, items, traps, trap type, and rough DC (REQ-EXP-20, REQ-EXP-21)
        msg := s.buildCaseItMessage(room, true)
        s.sendConsole(uid, msg)
    case skillcheck.Success:
        // Reveal hidden exits, items, traps (REQ-EXP-20)
        msg := s.buildCaseItMessage(room, false)
        s.sendConsole(uid, msg)
    case skillcheck.Failure, skillcheck.CritFailure:
        // Nothing (REQ-EXP-22)
    }
}
```

- [ ] **Step 7: Implement Poke Around room-entry handler**

```go
func (s *GameServiceServer) onRoomEntry_PokeAround(uid string, sess *session.PlayerSession, room *world.Room) {
    context := room.Properties["context"]
    skillID, dc := explore.PokeAroundContext(context)

    // For "faction" context, pick higher-ranked skill (REQ-EXP-37)
    if context == "faction" {
        conspiracyRank := proficiencyToInt(sess.Skills["conspiracy"])
        factionsRank := proficiencyToInt(sess.Skills["factions"])
        skillID = explore.PokeAroundFactionSkill(conspiracyRank, factionsRank)
    }

    skillRank := s.resolveSkillRank(uid, sess, skillID)
    roll := s.dice.Roll(1, 20, 0)
    abilityMod := s.abilityModForSkill(sess, skillID)
    result := skillcheck.Resolve(roll, abilityMod, skillRank, dc, skillcheck.TriggerDef{})

    facts := room.LoreFacts // []string of lore facts — see Room struct
    if len(facts) == 0 {
        return // nothing to reveal
    }

    switch result.Outcome {
    case skillcheck.CritSuccess:
        count := 2
        if len(facts) < 2 {
            count = len(facts)
        }
        s.sendConsole(uid, fmt.Sprintf("Poke Around: %s", strings.Join(facts[:count], " ")))
    case skillcheck.Success:
        s.sendConsole(uid, fmt.Sprintf("Poke Around: %s", facts[0]))
    case skillcheck.Failure, skillcheck.CritFailure:
        // Nothing (REQ-EXP-36) — no false info
    }
}
```

Note: If `Room` does not have a `LoreFacts []string` field, add it to `internal/game/world/model.go` as an empty slice. Poke Around silently does nothing if no facts are defined for the room.

- [ ] **Step 8: Implement `resolveSkillRank` helper**

```go
// resolveSkillRank returns the effective skill rank for uid/sess on a given skill.
// If ExploreMode == shadow and the shadow target has a higher rank, uses the target's rank (REQ-EXP-29, REQ-EXP-30).
func (s *GameServiceServer) resolveSkillRank(uid string, sess *session.PlayerSession, skillID string) string {
    playerRank := sess.Skills[skillID]
    if playerRank == "" {
        playerRank = "untrained"
    }
    if sess.ExploreMode != session.ExploreModeShadow || sess.ExploreShadowTarget == "" {
        return playerRank
    }
    // Check if shadow target is in same room (REQ-EXP-31)
    target, ok := s.sessions.Get(sess.ExploreShadowTarget)
    if !ok || target.RoomID != sess.RoomID {
        return playerRank // suspended silently (REQ-EXP-31)
    }
    targetRank := target.Skills[skillID]
    if targetRank == "" {
        targetRank = "untrained"
    }
    if proficiencyToInt(targetRank) > proficiencyToInt(playerRank) {
        return targetRank // REQ-EXP-29
    }
    return playerRank // REQ-EXP-30
}
```

- [ ] **Step 9: Run tests**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestOnRoomEntry -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(exploration): implement onRoomEntry hook (lay_low, active_sensors, case_it, poke_around, shadow rank resolution)"
```

---

### Task 6: Combat-start hook (`onCombatStart`)

**Files:**
- Modify: `internal/gameserver/combat_handler.go` (in `startCombatLocked`)

- [ ] **Step 1: Write tests**

```go
func TestOnCombatStart_LayLowCleared(t *testing.T) {
    // sess.ExploreMode = "lay_low"
    // Trigger combat start
    // Assert sess.ExploreMode == "" (REQ-EXP-9, REQ-EXP-40)
}

func TestOnCombatStart_HoldGround_ShieldEquipped(t *testing.T) {
    // sess.ExploreMode = "hold_ground", shield equipped
    // First AP grant fires
    // Assert shield_raised condition applied at no AP cost (REQ-EXP-11)
}

func TestOnCombatStart_HoldGround_NoShield(t *testing.T) {
    // sess.ExploreMode = "hold_ground", no shield
    // Assert no error, no condition applied (REQ-EXP-12)
}

func TestOnCombatStart_RunPoint_GrantsBonus(t *testing.T) {
    // sess.ExploreMode = "run_point", two other players in same room
    // Assert both others get +1 circumstance initiative bonus (REQ-EXP-25)
    // Assert run point player does NOT get bonus (REQ-EXP-26)
}
```

- [ ] **Step 2: Implement `onCombatStart` hook in `startCombatLocked`**

In `combat_handler.go`, in `startCombatLocked`, after initiative is rolled (line ~1881) but before the initiative order is finalized (REQ-EXP-39):

```go
// Process exploration mode combat-start hooks (REQ-EXP-39, REQ-EXP-40)
for _, combatant := range combatants {
    if combatant.Type != "player" {
        continue
    }
    sess, ok := h.sessions.Get(combatant.UID)
    if !ok {
        continue
    }
    switch sess.ExploreMode {
    case session.ExploreModeLayLow:
        // Clear Lay Low before other hooks (REQ-EXP-40)
        sess.ExploreMode = ""
    case session.ExploreModeHoldGround:
        // Hold Ground: apply shield_raised at first AP grant (REQ-EXP-11)
        // Set a flag on the session so the AP-grant handler can fire it once
        sess.HoldGroundPending = true
    case session.ExploreModeRunPoint:
        // Run Point: grant +1 initiative bonus to all co-located players (REQ-EXP-25, REQ-EXP-26, REQ-EXP-27)
        for _, other := range h.sessions.PlayersInRoomDetails(sess.RoomID) {
            if other.UID == combatant.UID {
                continue // REQ-EXP-26: not to self
            }
            // Apply +1 circumstance bonus to other's initiative roll result
            // Find other in combatants slice and adjust
            for i, c := range combatants {
                if c.UID == other.UID {
                    combatants[i].Initiative += 1
                    break
                }
            }
        }
    }
}
```

Note: `HoldGroundPending bool` must be added to `PlayerSession`. Hold Ground's `shield_raised` application happens in the AP-grant handler (the code path that fires when it's the player's initiative slot). Find this handler and add:

```go
if sess.HoldGroundPending {
    sess.HoldGroundPending = false
    if sess.Equipment.HasShield() {
        s.applyCondition(uid, sess, "shield_raised")
    }
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestOnCombatStart -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/game/session/manager.go
git commit -m "feat(exploration): implement onCombatStart hook (lay_low clear, hold_ground, run_point initiative bonus)"
```

---

## Deferred

- **Lay Low condition enforcement** (REQ-EXP-8a, crit failure): `ExploreLayLowCritFailRoom` is set in `onRoomEntry_LayLow`. The condition application path (`sess.Conditions.Apply`) must be taught to check this field and reject `hidden`/`undetected` application when it matches the current room. Wire this into the condition application path in a follow-up task.
- **Poke Around content**: `Room.LoreFacts` field is added in Task 1. Population of lore fact YAML is content work, not implementation work.

---

## Verification Checklist

- [ ] `explore lay_low` sets ExploreMode; `explore off` clears it
- [ ] `explore` in combat returns error message; mode unchanged
- [ ] `explore shadow <name>` fails if target not in same room
- [ ] `active_sensors` and `case_it` fire immediately on mode set (REQ-EXP-5)
- [ ] Lay Low: no check when no NPCs; hidden+undetected on crit success; hidden on success; failure has no effect; crit fail sets `ExploreLayLowCritFailRoom` (REQ-EXP-8a — condition system must check this field to reject hidden/undetected; partial impl)
- [ ] Lay Low cleared when combat starts (REQ-EXP-9, REQ-EXP-40)
- [ ] Hold Ground: shield_raised applied at first AP grant at no AP cost; no effect/no error if no shield
- [ ] Active Sensors: console message lists tech on success; concealed on crit success; no message on failure
- [ ] Case It: reveals hidden exits/items/traps on success; trap type+DC on crit success
- [ ] Run Point: +1 initiative to all co-located players except self; evaluated at roll time
- [ ] Shadow: borrows ally rank when higher; suspends silently when ally not in room; resumes on re-entry
- [ ] Poke Around: one fact on success; two facts on crit success; no false info on failure
- [ ] Full test suite passes with zero failures
