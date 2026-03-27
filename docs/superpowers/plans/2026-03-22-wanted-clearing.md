# Wanted Level Clearing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement bribe, surrender, and release mechanics for reducing a player's WantedLevel, plus the detained condition with DB persistence.

**Architecture:** Proto messages for four new commands; two new gRPC handlers (`bribe`/`bribe confirm` two-step, `surrender`, `release`); detained condition with three new enforcement flags; `DetainedUntil` on `PlayerSession` persisted to DB; detention lifecycle checked at command dispatch, reconnect, and regen tick.

**Tech Stack:** Go, protobuf/gRPC, PostgreSQL, YAML content, existing condition/session/enforcement systems.

---

## Already Implemented (Verify Before Coding)

The following were implemented in the non-combat-npcs SP5 work. Each task that depends on them begins with a verification step.

- `FixerConfig` struct with `Validate()` enforcing REQ-WC-1, REQ-WC-2, REQ-WC-2a — in `internal/game/npc/noncombat.go`
- `Template.Fixer *FixerConfig` field — in `internal/game/npc/template.go`
- `Template.Validate()` recognizes `"fixer"` type and requires non-nil `Fixer` field
- `GuardConfig.Bribeable bool`, `GuardConfig.MaxBribeWantedLevel int`, `GuardConfig.BaseCosts map[int]int` with validation
- `content/npcs/dex.yaml` — named fixer NPC for Rustbucket Ridge
- Fixer default `flee` personality enforcement

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `api/proto/game/v1/game.proto` | Modify | Add BribeRequest, BribeConfirmRequest, SurrenderRequest, ReleaseRequest |
| `api/game/v1/game.pb.go` | Regenerated | Proto generated output |
| `api/game/v1/game_grpc.pb.go` | Regenerated | gRPC generated output |
| `internal/frontend/telnet/grpc_bridge.go` | Modify | Add bridge cases for 4 new commands |
| `internal/frontend/handlers/command_handler.go` | Modify | Add HandlerBribe, HandlerBribeConfirm, HandlerSurrender, HandlerRelease constants + registration |
| `internal/game/condition/definition.go` | Modify | Add PreventMovement, PreventCommands, PreventTargeting bool fields |
| `content/conditions/detained.yaml` | Create | Detained condition YAML |
| `internal/game/session/manager.go` | Modify | Add PendingBribeNPCName, PendingBribeAmount, DetainedUntil, DetentionGraceUntil to PlayerSession |
| `internal/storage/postgres/migrations/034_detained_until.up.sql` | Create | Add detained_until column to characters |
| `internal/storage/postgres/migrations/034_detained_until.down.sql` | Create | Drop detained_until column |
| `internal/storage/postgres/character_repository.go` | Modify | Persist/load DetainedUntil; add CharacterDetentionUpdater interface |
| `internal/gameserver/grpc_service_bribe.go` | Create | handleBribe, handleBribeConfirm gRPC handlers |
| `internal/gameserver/grpc_service_surrender.go` | Create | handleSurrender gRPC handler + detention completion helper |
| `internal/gameserver/grpc_service_release.go` | Create | handleRelease gRPC handler |
| `internal/gameserver/grpc_service.go` | Modify | Register new handlers in dispatch; enforce detained condition on command dispatch |
| `internal/gameserver/enforcement.go` | Modify | Add movement enforcement for detained condition |
| `docs/features/wanted-clearing.md` | Modify | Mark all checkboxes complete |

---

## Task 1: Proto + Command Wiring

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/telnet/grpc_bridge.go`
- Modify: `internal/frontend/handlers/command_handler.go`

- [ ] **Step 1: Add four new proto messages**

In `api/proto/game/v1/game.proto`, after the `TalkRequest` message (currently the last request message), add:

```protobuf
message BribeRequest {
  string npc_name = 1;  // optional; required when multiple bribeable NPCs present
}

message BribeConfirmRequest {}

message SurrenderRequest {}

message ReleaseRequest {
  string player_name = 1;  // name of detained player to release
}
```

In the `ClientMessage` oneof, after `talk_request = 101`, add:

```protobuf
BribeRequest bribe_request = 102;
BribeConfirmRequest bribe_confirm_request = 103;
SurrenderRequest surrender_request = 104;
ReleaseRequest release_request = 105;
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: no errors; `api/game/v1/game.pb.go` and `api/game/v1/game_grpc.pb.go` updated.

- [ ] **Step 3: Add Handler constants and registration**

In `internal/frontend/handlers/command_handler.go`, add constants alongside existing `HandlerTalk`:

```go
const (
    HandlerBribe        = "bribe"
    HandlerBribeConfirm = "bribe confirm"
    HandlerSurrender    = "surrender"
    HandlerRelease      = "release"
)
```

Register in the same location where `HandlerTalk` is registered:

```go
h.Register(HandlerBribe, h.handleBribeCommand)
h.Register(HandlerBribeConfirm, h.handleBribeConfirmCommand)
h.Register(HandlerSurrender, h.handleSurrenderCommand)
h.Register(HandlerRelease, h.handleReleaseCommand)
```

The handler stub methods (forwarding to gRPC) follow the same pattern as `handleTalkCommand`. Add them to `command_handler.go`:

```go
func (h *CommandHandler) handleBribeCommand(ctx context.Context, args []string) error {
    npcName := ""
    if len(args) > 0 {
        npcName = strings.Join(args, " ")
    }
    _, err := h.client.SendCommand(ctx, &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_BribeRequest{
            BribeRequest: &gamev1.BribeRequest{NpcName: npcName},
        },
    })
    return err
}

func (h *CommandHandler) handleBribeConfirmCommand(ctx context.Context, args []string) error {
    _, err := h.client.SendCommand(ctx, &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_BribeConfirmRequest{
            BribeConfirmRequest: &gamev1.BribeConfirmRequest{},
        },
    })
    return err
}

func (h *CommandHandler) handleSurrenderCommand(ctx context.Context, args []string) error {
    _, err := h.client.SendCommand(ctx, &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_SurrenderRequest{
            SurrenderRequest: &gamev1.SurrenderRequest{},
        },
    })
    return err
}

func (h *CommandHandler) handleReleaseCommand(ctx context.Context, args []string) error {
    if len(args) == 0 {
        return fmt.Errorf("release requires a player name")
    }
    _, err := h.client.SendCommand(ctx, &gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_ReleaseRequest{
            ReleaseRequest: &gamev1.ReleaseRequest{PlayerName: strings.Join(args, " ")},
        },
    })
    return err
}
```

- [ ] **Step 4: Add bridge cases**

In `internal/frontend/telnet/grpc_bridge.go`, in the command routing switch, add cases matching existing pattern (e.g., `case "talk"`):

```go
case "bribe":
    if len(args) > 0 && args[0] == "confirm" {
        return HandlerBribeConfirm, nil
    }
    return HandlerBribe, args
case "surrender":
    return HandlerSurrender, nil
case "release":
    return HandlerRelease, args
```

- [ ] **Step 5: Build to verify compilation**

```bash
cd /home/cjohannsen/src/mud && make build
```

Expected: clean build. Handler stubs compile; no handlers yet register on the server side.

- [ ] **Step 6: Commit**

```bash
git add api/proto/game/v1/game.proto api/game/v1/ \
    internal/frontend/handlers/command_handler.go \
    internal/frontend/telnet/grpc_bridge.go
git commit -m "feat: add proto messages and client wiring for bribe/surrender/release commands"
```

---

## Task 2: Bribe Command

**Files:**
- Modify: `internal/game/session/manager.go`
- Create: `internal/gameserver/grpc_service_bribe.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Verify FixerConfig and GuardConfig already exist**

```bash
grep -n "FixerConfig\|GuardConfig\|Bribeable\|BaseCosts\|NPCVariance\|MaxBribeWanted" \
    /home/cjohannsen/src/mud/internal/game/npc/noncombat.go | head -30
```

Expected: All fields present. If missing, stop and raise to human.

- [ ] **Step 2: Write failing tests for bribe handler**

Create `internal/gameserver/grpc_service_bribe_test.go`:

```go
package gameserver_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    gamev1 "github.com/cory-johannsen/mud/api/game/v1"
)

func TestHandleBribe_NoWantedLevel(t *testing.T) {
    // Player with WantedLevel 0 in current zone should get error message
    // Uses newTestGameServiceServer helper
    // Set up session with WantedLevel 0
    // Call handleBribe
    // Assert event contains "You are not wanted" or similar
}

func TestHandleBribe_NoBribeableNPC(t *testing.T) {
    // Room has no bribeable NPCs
    // Assert event contains "no one here who can help"
}

func TestHandleBribe_ShowsCostAndPrompt(t *testing.T) {
    // Room has fixer, player has WantedLevel 1
    // Assert event shows cost and "type 'bribe confirm' to proceed"
    // Assert PendingBribeNPCName and PendingBribeAmount set on session
}

func TestHandleBribeConfirm_NoPendingBribe(t *testing.T) {
    // No pending bribe state on session
    // Assert event contains "no pending bribe"
}

func TestHandleBribeConfirm_InsufficientCredits(t *testing.T) {
    // PendingBribeAmount > player credits
    // Assert event contains "insufficient credits"
}

func TestHandleBribeConfirm_Success(t *testing.T) {
    // Enough credits, valid pending bribe
    // Assert credits deducted, WantedLevel decremented, pending state cleared
}
```

- [ ] **Step 3: Run failing tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleBribe -v
```

Expected: compile error or FAIL (handlers not implemented yet).

- [ ] **Step 4: Add pending bribe state to PlayerSession**

In `internal/game/session/manager.go`, add fields to `PlayerSession`:

```go
// Bribe pending state (cleared on bribe confirm or any other command)
PendingBribeNPCName string
PendingBribeAmount  int
```

- [ ] **Step 5: Implement bribe handlers**

Create `internal/gameserver/grpc_service_bribe.go`:

```go
package gameserver

import (
    "context"
    "fmt"
    "math"

    gamev1 "github.com/cory-johannsen/mud/api/game/v1"
    "github.com/cory-johannsen/mud/internal/game/npc"
)

// zoneMultiplier returns the bribe cost multiplier for a given danger level.
func zoneMultiplier(dangerLevel string) float64 {
    switch dangerLevel {
    case "safe":
        return 0.8
    case "sketchy":
        return 1.0
    case "dangerous":
        return 1.5
    case "all_out_war":
        return 2.5
    default:
        return 1.0
    }
}

func (s *GameServiceServer) handleBribe(ctx context.Context, req *gamev1.BribeRequest) ([]*gamev1.ServerEvent, error) {
    sess := s.getSession(ctx)
    if sess == nil {
        return nil, errNoSession
    }

    // REQ-WC-5: fail if WantedLevel is 0 in current zone
    zoneID := sess.CurrentZoneID
    wantedLevel := sess.WantedLevel[zoneID]
    if wantedLevel == 0 {
        return messageEvents("You are not wanted here. There is nothing to clear."), nil
    }

    // Gather bribeable NPCs in room
    room := s.worldMgr.GetRoom(sess.CurrentRoomID)
    if room == nil {
        return nil, fmt.Errorf("room not found")
    }
    npcsInRoom := s.npcMgr.GetNPCsInRoom(sess.CurrentRoomID)

    type briberInfo struct {
        name      string
        maxLevel  int
        baseCosts map[int]int
        variance  float64
    }

    var candidates []briberInfo
    for _, n := range npcsInRoom {
        tmpl := n.Template()
        switch tmpl.NPCType {
        case "fixer":
            if tmpl.Fixer != nil && wantedLevel <= tmpl.Fixer.MaxWantedLevel {
                candidates = append(candidates, briberInfo{
                    name:      tmpl.Name,
                    maxLevel:  tmpl.Fixer.MaxWantedLevel,
                    baseCosts: tmpl.Fixer.BaseCosts,
                    variance:  tmpl.Fixer.NPCVariance,
                })
            }
        case "guard":
            if tmpl.Guard != nil && tmpl.Guard.Bribeable && wantedLevel <= tmpl.Guard.MaxBribeWantedLevel {
                candidates = append(candidates, briberInfo{
                    name:      tmpl.Name,
                    maxLevel:  tmpl.Guard.MaxBribeWantedLevel,
                    baseCosts: tmpl.Guard.BaseCosts,
                    variance:  1.0,
                })
            }
        }
    }

    // REQ-WC-7: fail if no bribeable NPC present
    if len(candidates) == 0 {
        return messageEvents("There is no one here who can help you with that."), nil
    }

    // REQ-WC-9a: disambiguate when multiple bribeable NPCs present
    var target briberInfo
    if req.NpcName == "" {
        if len(candidates) > 1 {
            names := make([]string, len(candidates))
            for i, c := range candidates {
                names[i] = c.name
            }
            return messageEvents(fmt.Sprintf("Multiple people here could help. Specify one: %s", joinNames(names))), nil
        }
        target = candidates[0]
    } else {
        found := false
        for _, c := range candidates {
            if npc.NameMatches(c.name, req.NpcName) {
                target = c
                found = true
                break
            }
        }
        if !found {
            return messageEvents(fmt.Sprintf("There is no bribeable NPC named %q here.", req.NpcName)), nil
        }
    }

    // REQ-WC-8: check max wanted level cap
    if wantedLevel > target.maxLevel {
        return messageEvents(fmt.Sprintf("%s won't help someone as wanted as you.", target.name)), nil
    }

    // Compute cost
    danger := s.worldMgr.GetZoneDangerLevel(zoneID)
    mult := zoneMultiplier(danger)
    baseCost := target.baseCosts[wantedLevel]
    cost := int(math.Floor(float64(baseCost) * mult * target.variance))

    // REQ-WC-9: store pending state and prompt for confirmation
    sess.PendingBribeNPCName = target.name
    sess.PendingBribeAmount = cost

    return messageEvents(fmt.Sprintf(
        "%s will clear your record for %d credits. Type 'bribe confirm' to proceed.",
        target.name, cost,
    )), nil
}

func (s *GameServiceServer) handleBribeConfirm(ctx context.Context, _ *gamev1.BribeConfirmRequest) ([]*gamev1.ServerEvent, error) {
    sess := s.getSession(ctx)
    if sess == nil {
        return nil, errNoSession
    }

    if sess.PendingBribeNPCName == "" {
        return messageEvents("You have no pending bribe to confirm."), nil
    }

    cost := sess.PendingBribeAmount
    sess.PendingBribeNPCName = ""
    sess.PendingBribeAmount = 0

    // REQ-WC-6: check credits
    if sess.Character.Credits < cost {
        return messageEvents(fmt.Sprintf("You need %d credits but only have %d.", cost, sess.Character.Credits)), nil
    }

    // Deduct and decrement
    sess.Character.Credits -= cost
    zoneID := sess.CurrentZoneID
    if sess.WantedLevel[zoneID] > 0 {
        sess.WantedLevel[zoneID]--
    }

    if err := s.wantedSaver.Upsert(ctx, sess.Character.ID, zoneID, sess.WantedLevel[zoneID]); err != nil {
        return nil, fmt.Errorf("persist wanted level: %w", err)
    }
    if err := s.charRepo.UpdateCredits(ctx, sess.Character.ID, sess.Character.Credits); err != nil {
        return nil, fmt.Errorf("persist credits: %w", err)
    }

    return messageEvents(fmt.Sprintf(
        "Deal done. Your record is cleared. Remaining wanted level: %d.",
        sess.WantedLevel[zoneID],
    )), nil
}
```

- [ ] **Step 6: Register handlers in grpc_service.go dispatch**

In `internal/gameserver/grpc_service.go`, in the `SendCommand` dispatch switch, add:

```go
case *gamev1.ClientMessage_BribeRequest:
    events, err = s.handleBribe(ctx, p.BribeRequest)
case *gamev1.ClientMessage_BribeConfirmRequest:
    events, err = s.handleBribeConfirm(ctx, p.BribeConfirmRequest)
```

- [ ] **Step 7: Run bribe tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleBribe -v
```

Expected: all PASS.

- [ ] **Step 8: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud && make test-fast
```

Expected: 100% pass.

- [ ] **Step 9: Commit**

```bash
git add internal/game/session/manager.go \
    internal/gameserver/grpc_service_bribe.go \
    internal/gameserver/grpc_service_bribe_test.go \
    internal/gameserver/grpc_service.go
git commit -m "feat: implement bribe command with two-step confirm and cost formula"
```

---

## Task 3: Detained Condition

**Files:**
- Modify: `internal/game/condition/definition.go`
- Create: `content/conditions/detained.yaml`
- Modify: `internal/gameserver/grpc_service.go` (command dispatch enforcement)
- Modify: `internal/gameserver/enforcement.go` (movement enforcement)

- [ ] **Step 1: Write failing tests for detained enforcement and visibility**

Create `internal/gameserver/grpc_service_detained_test.go`:

```go
package gameserver_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestDetained_BlocksMovement(t *testing.T) {
    // Player has detained condition
    // Attempt to move (e.g., north)
    // Assert event contains "detained" and movement is blocked
}

func TestDetained_BlocksCommands(t *testing.T) {
    // Player has detained condition
    // Attempt any command other than look/who
    // Assert event contains "detained" and command is blocked
}

func TestDetained_BlocksTargeting(t *testing.T) {
    // Player has detained condition
    // NPC or other player attempts to target detained player
    // Assert targeting is rejected
}

func TestDetained_VisibleInRoomLook(t *testing.T) {
    // REQ-WC-11: detained player must be visible to others in the room
    // Player A is detained; Player B looks at the room
    // Assert Player B's look response contains "<PlayerA> is detained here."
}
```

- [ ] **Step 2: Add enforcement flags to ConditionDef**

In `internal/game/condition/definition.go`, add to the `Definition` struct (after existing fields):

```go
// Enforcement flags
PreventMovement  bool `yaml:"prevents_movement"`
PreventCommands  bool `yaml:"prevents_commands"`
PreventTargeting bool `yaml:"prevents_targeting"`
```

- [ ] **Step 3: Create detained.yaml**

Create `content/conditions/detained.yaml`:

```yaml
id: detained
name: Detained
description: You are restrained and cannot act.
duration_type: permanent
prevents_movement: true
prevents_commands: true
prevents_targeting: true
```

- [ ] **Step 4: Run failing tests to verify**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestDetained -v
```

Expected: compile errors or FAIL (enforcement not implemented yet).

- [ ] **Step 5: Enforce PreventCommands in command dispatch**

In `internal/gameserver/grpc_service.go`, in `SendCommand` before the command switch, add:

```go
// REQ-WC-10: detained players cannot use commands
if sess := s.getSession(ctx); sess != nil {
    if s.conditionMgr.HasCondition(sess.Character.ID, "detained") {
        return &gamev1.ServerResponse{
            Events: messageEvents("You are detained and cannot act."),
        }, nil
    }
}
```

Note: `look` and `who` should be exempt from this block. Check the existing command dispatch pattern to find where exemptions should be placed (look at how combat-blocked commands work for reference).

- [ ] **Step 6: Enforce PreventMovement in enforcement.go**

In `internal/gameserver/enforcement.go`, in the movement check function (find the function that validates player movement), add:

```go
// REQ-WC-10: detained players cannot move
if s.conditionMgr.HasCondition(characterID, "detained") {
    return false, "You are detained and cannot move."
}
```

- [ ] **Step 7: Enforce PreventTargeting in combat targeting**

In the combat handler or targeting logic, add a check before a target is accepted:

```go
if s.conditionMgr.HasCondition(targetCharID, "detained") {
    return nil, fmt.Errorf("you cannot target a detained player")
}
```

- [ ] **Step 7a: Add detained player visibility to room look (REQ-WC-11)**

Find where the room description is assembled for the `look` command (likely in `internal/gameserver/grpc_service.go` or `internal/game/world/room.go` in the `RenderRoomView` path). Read these files to find the exact location, then add a suffix for each character in the room who has the `detained` condition:

```go
// After building room occupant list, for each player in room:
if s.conditionMgr.HasCondition(playerID, "detained") {
    roomText += fmt.Sprintf("\n%s is detained here.", playerName)
}
```

The exact implementation depends on where player presence is rendered — follow the pattern used to display NPCs in the room to find the right injection point.

- [ ] **Step 8: Run detained tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestDetained -v
```

Expected: all PASS.

- [ ] **Step 9: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud && make test-fast
```

Expected: 100% pass.

- [ ] **Step 10: Commit**

```bash
git add internal/game/condition/definition.go \
    content/conditions/detained.yaml \
    internal/gameserver/grpc_service.go \
    internal/gameserver/enforcement.go \
    internal/gameserver/grpc_service_detained_test.go
git commit -m "feat: add detained condition with enforcement and room visibility (REQ-WC-10/11)"
```

---

## Task 4: Surrender + Detention Lifecycle

**Files:**
- Modify: `internal/game/session/manager.go`
- Create: `internal/storage/postgres/migrations/034_detained_until.up.sql`
- Create: `internal/storage/postgres/migrations/034_detained_until.down.sql`
- Modify: `internal/storage/postgres/character_repository.go`
- Create: `internal/gameserver/grpc_service_surrender.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_surrender_test.go`:

```go
package gameserver_test

func TestSurrender_NoGuardInRoom(t *testing.T) {
    // Room has no guard NPC
    // Assert event: "There is no one here to surrender to."
}

func TestSurrender_WantedLevelZero(t *testing.T) {
    // Player WantedLevel is 0
    // Assert event: "You are not wanted here."
}

func TestSurrender_SetsDetainedConditionAndTimer(t *testing.T) {
    // Room has guard, WantedLevel 2 (Burned)
    // Assert detained condition applied
    // Assert DetainedUntil set to now + 1 real minute (±5s tolerance)
    // Assert WantedLevel NOT yet decremented (decrements on release)
}

func TestDetentionCompletion_DecrementsWantedLevel(t *testing.T) {
    // Session has DetainedUntil in the past
    // Trigger detention completion check
    // Assert detained condition removed
    // Assert WantedLevel decremented by 1
    // Assert 5-second DetentionGraceUntil set
}

func TestDetentionCompletion_OfflineReconnect(t *testing.T) {
    // DetainedUntil in the past when player connects
    // Assert completion runs on connect
}
```

- [ ] **Step 2: Add session fields**

In `internal/game/session/manager.go`, add to `PlayerSession`:

```go
// Detention state
DetainedUntil        *time.Time // nil means not detained; non-nil is game-clock expiry
DetentionGraceUntil  time.Time  // 5-second grace window after detention completes
```

- [ ] **Step 3: Create DB migration**

Create `internal/storage/postgres/migrations/034_detained_until.up.sql`:

```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS detained_until TIMESTAMPTZ;
```

Create `internal/storage/postgres/migrations/034_detained_until.down.sql`:

```sql
ALTER TABLE characters DROP COLUMN IF EXISTS detained_until;
```

- [ ] **Step 4: Add DB persistence for DetainedUntil**

In `internal/storage/postgres/character_repository.go`:

1. Add `CharacterDetentionUpdater` interface (or extend existing updater):

```go
type CharacterDetentionUpdater interface {
    UpdateDetainedUntil(ctx context.Context, characterID int64, detainedUntil *time.Time) error
}
```

2. Add `UpdateDetainedUntil` method to `CharacterRepository`.

3. In the character `Get`/`Load` path, scan `detained_until` into `Character.DetainedUntil`.

- [ ] **Step 5: Run failing tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestSurrender -v
```

Expected: compile errors or FAIL.

- [ ] **Step 6: Implement handleSurrender**

Create `internal/gameserver/grpc_service_surrender.go`:

```go
package gameserver

import (
    "context"
    "fmt"
    "time"

    gamev1 "github.com/cory-johannsen/mud/api/game/v1"
)

// detentionDuration returns the real-time detention duration for a given wanted level.
// Per spec: 1 in-game hour = 1 real minute.
//   WL1 = 30 in-game min = 30s real
//   WL2 = 1 in-game hr   = 1m real
//   WL3 = 3 in-game hr   = 3m real
//   WL4 = 8 in-game hr   = 8m real
func detentionDuration(wantedLevel int) time.Duration {
    switch wantedLevel {
    case 1:
        return 30 * time.Second
    case 2:
        return 1 * time.Minute
    case 3:
        return 3 * time.Minute
    case 4:
        return 8 * time.Minute
    default:
        return time.Minute
    }
}

func (s *GameServiceServer) handleSurrender(ctx context.Context, _ *gamev1.SurrenderRequest) ([]*gamev1.ServerEvent, error) {
    sess := s.getSession(ctx)
    if sess == nil {
        return nil, errNoSession
    }

    // REQ-WC-13: fail if WantedLevel 0
    zoneID := sess.CurrentZoneID
    wantedLevel := sess.WantedLevel[zoneID]
    if wantedLevel == 0 {
        return messageEvents("You are not wanted here. There is nothing to surrender for."), nil
    }

    // REQ-WC-12: fail if no guard in room
    npcsInRoom := s.npcMgr.GetNPCsInRoom(sess.CurrentRoomID)
    hasGuard := false
    for _, n := range npcsInRoom {
        if n.Template().NPCType == "guard" {
            hasGuard = true
            break
        }
    }
    if !hasGuard {
        return messageEvents("There is no one here to surrender to."), nil
    }

    // Apply detained condition and set timer
    dur := detentionDuration(wantedLevel)
    now := time.Now()
    expiry := now.Add(dur)
    sess.DetainedUntil = &expiry

    s.conditionMgr.Apply(ctx, sess.Character.ID, "detained")

    if err := s.charRepo.UpdateDetainedUntil(ctx, sess.Character.ID, sess.DetainedUntil); err != nil {
        return nil, fmt.Errorf("persist detained_until: %w", err)
    }

    return messageEvents(fmt.Sprintf(
        "You drop your weapon and surrender. You will be released in %s.",
        dur.String(),
    )), nil
}

// checkDetentionCompletion checks whether a player's detention has expired and
// completes it if so. Should be called at command dispatch, login, and regen tick.
// REQ-WC-14b: runs on reconnect; REQ-WC-14c: sets 5-second grace window.
func (s *GameServiceServer) checkDetentionCompletion(ctx context.Context, sess *session.PlayerSession) {
    if sess.DetainedUntil == nil {
        return
    }
    if time.Now().Before(*sess.DetainedUntil) {
        return
    }

    // Detention expired — complete it
    sess.DetainedUntil = nil
    s.conditionMgr.Remove(ctx, sess.Character.ID, "detained")

    // Decrement WantedLevel
    zoneID := sess.CurrentZoneID
    if sess.WantedLevel[zoneID] > 0 {
        sess.WantedLevel[zoneID]--
    }
    _ = s.wantedSaver.Upsert(ctx, sess.Character.ID, zoneID, sess.WantedLevel[zoneID])
    _ = s.charRepo.UpdateDetainedUntil(ctx, sess.Character.ID, nil)

    // REQ-WC-14c: 5-second grace window before guards re-evaluate
    sess.DetentionGraceUntil = time.Now().Add(5 * time.Second)

    // Broadcast to room
    s.broadcastToRoom(sess.CurrentRoomID, fmt.Sprintf("%s has been released from detention.", sess.Character.Name))
}
```

- [ ] **Step 7: Wire detention completion checks**

In `internal/gameserver/grpc_service.go`:

1. At the start of `SendCommand`, after getting session, call `s.checkDetentionCompletion(ctx, sess)`.
2. In the player login/reconnect handler (`handleLogin` or equivalent), call `s.checkDetentionCompletion(ctx, sess)`.
3. In the regen tick (wherever HP/condition ticks happen), call `s.checkDetentionCompletion(ctx, sess)`.

Also update the guard re-evaluation in enforcement to respect `DetentionGraceUntil`:

```go
if time.Now().Before(sess.DetentionGraceUntil) {
    return // grace window still active; skip guard re-evaluation
}
```

- [ ] **Step 8: Register surrender in dispatch**

In `internal/gameserver/grpc_service.go` dispatch switch:

```go
case *gamev1.ClientMessage_SurrenderRequest:
    events, err = s.handleSurrender(ctx, p.SurrenderRequest)
```

- [ ] **Step 9: Run surrender tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestSurrender\|TestDetention -v
```

Expected: all PASS.

- [ ] **Step 10: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud && make test-fast
```

Expected: 100% pass.

- [ ] **Step 11: Commit**

```bash
git add internal/game/session/manager.go \
    internal/storage/postgres/migrations/034_detained_until.up.sql \
    internal/storage/postgres/migrations/034_detained_until.down.sql \
    internal/storage/postgres/character_repository.go \
    internal/gameserver/grpc_service_surrender.go \
    internal/gameserver/grpc_service_surrender_test.go \
    internal/gameserver/grpc_service.go
git commit -m "feat: implement surrender command with detention timer, DB persistence, and completion lifecycle"
```

---

## Task 5: Release Command

**Files:**
- Create: `internal/gameserver/grpc_service_release.go`
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_release_test.go`:

```go
package gameserver_test

func TestRelease_TargetNotDetained(t *testing.T) {
    // Target player does not have detained condition
    // Assert event: "<player> is not detained."
}

func TestRelease_TargetNotInRoom(t *testing.T) {
    // Target player not in same room
    // Assert event: "<player> is not here."
}

func TestRelease_SkillCheckSuccess(t *testing.T) {
    // Target is detained, skill check passes (mock dice)
    // Assert detained condition removed from target
    // Assert target WantedLevel unchanged (REQ-WC-15)
    // Assert success message
}

func TestRelease_SkillCheckFailure(t *testing.T) {
    // Target is detained, skill check fails (mock dice)
    // Assert detained condition still present on target
    // Assert failure message
}
```

- [ ] **Step 2: Run failing tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestRelease -v
```

Expected: compile errors or FAIL.

- [ ] **Step 3: Implement handleRelease**

Create `internal/gameserver/grpc_service_release.go`:

```go
package gameserver

import (
    "context"
    "fmt"

    gamev1 "github.com/cory-johannsen/mud/api/game/v1"
)

// releaseDC returns the skill check DC for the release command based on room danger level.
func releaseDC(dangerLevel string) int {
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

func (s *GameServiceServer) handleRelease(ctx context.Context, req *gamev1.ReleaseRequest) ([]*gamev1.ServerEvent, error) {
    sess := s.getSession(ctx)
    if sess == nil {
        return nil, errNoSession
    }

    // Find target player in same room
    targetSess := s.sessionMgr.GetSessionByPlayerName(req.PlayerName)
    if targetSess == nil || targetSess.CurrentRoomID != sess.CurrentRoomID {
        return messageEvents(fmt.Sprintf("%s is not here.", req.PlayerName)), nil
    }

    // REQ-WC-15: target must be detained
    if !s.conditionMgr.HasCondition(targetSess.Character.ID, "detained") {
        return messageEvents(fmt.Sprintf("%s is not detained.", req.PlayerName)), nil
    }

    // REQ-WC-16a: DC uses room's current danger level
    zoneID := sess.CurrentZoneID
    danger := s.worldMgr.GetZoneDangerLevel(zoneID)
    dc := releaseDC(danger)

    // Skill check: player's choice of Grift or Ghosting (use higher of the two)
    grift := sess.Character.Skills["Grift"]
    ghosting := sess.Character.Skills["Ghosting"]
    skill := grift
    if ghosting > grift {
        skill = ghosting
    }

    roll := s.roller.Roll(1, 20) + skill
    if roll >= dc {
        // Success: remove detained condition; do NOT modify WantedLevel (REQ-WC-15)
        targetSess.DetainedUntil = nil
        s.conditionMgr.Remove(ctx, targetSess.Character.ID, "detained")
        _ = s.charRepo.UpdateDetainedUntil(ctx, targetSess.Character.ID, nil)
        return messageEvents(fmt.Sprintf("You free %s from detention.", req.PlayerName)), nil
    }

    return messageEvents(fmt.Sprintf(
        "You fail to free %s (rolled %d vs DC %d).",
        req.PlayerName, roll, dc,
    )), nil
}
```

- [ ] **Step 4: Register in dispatch**

In `internal/gameserver/grpc_service.go` dispatch switch:

```go
case *gamev1.ClientMessage_ReleaseRequest:
    events, err = s.handleRelease(ctx, p.ReleaseRequest)
```

- [ ] **Step 5: Run release tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestRelease -v
```

Expected: all PASS.

- [ ] **Step 6: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud && make test-fast
```

Expected: 100% pass.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service_release.go \
    internal/gameserver/grpc_service_release_test.go \
    internal/gameserver/grpc_service.go
git commit -m "feat: implement release command with Grift/Ghosting skill check vs danger DC"
```

---

## Task 6: Feature Doc Update

**Files:**
- Modify: `docs/features/wanted-clearing.md`

- [ ] **Step 1: Mark all checkboxes complete**

Open `docs/features/wanted-clearing.md` and mark all `- [ ]` checkboxes as `- [x]`.

- [ ] **Step 2: Update index.yaml status**

In `docs/features/index.yaml`, set `wanted-clearing` status to `done`.

- [ ] **Step 3: Final full test suite**

```bash
cd /home/cjohannsen/src/mud && make test-fast
```

Expected: 100% pass.

- [ ] **Step 4: Commit**

```bash
git add docs/features/wanted-clearing.md docs/features/index.yaml
git commit -m "docs: mark wanted-clearing feature as done"
```

---

## Deferred: Quest-Based Clearing (REQ-WC-17)

**REQ-WC-17** (quest completion with `wanted_reduction: N` decrements WantedLevel) and the fixer `ClearRecordQuestID` wiring are **intentionally NOT implemented in this plan**. Per the spec (section 4), full quest wiring is deferred to the `quests` feature (priority 380). The `FixerConfig.ClearRecordQuestID` field is defined in the data model (already present from SP5) but remains empty in all fixer YAML until the quests feature is built.
