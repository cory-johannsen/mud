# Ready Action Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `ready <action> when <trigger>` command, which costs 2 AP and stores a (trigger, action) pair that fires automatically as a Reaction when the trigger condition occurs during round resolution.

**Architecture:** `PlayerSession` gains `ReadiedTrigger`/`ReadiedAction` string fields. The existing `buildReactionCallback` in `reaction_handler.go` is extended to check these fields and execute the readied action automatically (no AP cost, no prompt) via a new `executeReadiedAction` helper. A new `TriggerOnEnemyEntersRoom` constant fires from the existing `onCombatantMoved` callback (after consumable traps, REQ-READY-15). The `enemy_attacks_me` and `ally_attacked` triggers reuse existing `TriggerOnDamageTaken` and `TriggerOnAllyDamaged` fire points already in `round.go`. `ReactionsRemaining` (existing `int` field on `PlayerSession`) serves as the "reaction used" gate — checking `ReactionsRemaining <= 0` is equivalent to the spec's `ReactionUsed == true`.

**Tech Stack:** Go, gRPC/protobuf (`api/proto/game/v1/game.proto`), `pgregory.net/rapid` for property tests, `github.com/stretchr/testify`

---

## File Map

| File | Change |
|---|---|
| `internal/game/reaction/trigger.go` | Add `TriggerOnEnemyEntersRoom` constant |
| `internal/game/session/manager.go` | Add `ReadiedTrigger string`, `ReadiedAction string` fields to `PlayerSession` |
| `internal/gameserver/combat_handler.go` | Clear `ReadiedTrigger`/`ReadiedAction` at end of round; notify player via `sess.Entity.Push` if expired |
| `api/proto/game/v1/game.proto` | Add `ReadyRequest` message; add `ready = 86` to `ClientMessage.payload` oneof |
| `internal/game/command/commands.go` | Add `HandlerReady = "ready"` constant; add command entry with alias `rdy` |
| `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeReady` handler; register in `bridgeHandlerMap` |
| `internal/gameserver/grpc_service_ready.go` | New: `handleReady` handler with alias tables |
| `internal/gameserver/grpc_service_ready_test.go` | New: tests for `handleReady` and session fields |
| `internal/gameserver/grpc_service.go` | Add dispatch case for `*gamev1.ClientMessage_Ready` |
| `internal/gameserver/reaction_handler.go` | Extend `buildReactionCallback` + add `executeReadiedAction`, `executeReadiedStrike`, `executeReadiedStep`, `executeReadiedRaiseShield` |
| `internal/gameserver/grpc_service_reaction_test.go` | Add ready-action reaction tests |
| `internal/gameserver/grpc_service_trap.go` | Extend `WireConsumableTrapTrigger` to also call `checkEnemyEntersReadyTrigger`; add new function |
| `docs/features/ready-action.md` | New feature doc |
| `docs/features/index.yaml` | Update `ready-action` status to `complete` |

---

## Task 1: Reaction Trigger Constant + Session Fields + End-of-Round Expiry

**Files:**
- Modify: `internal/game/reaction/trigger.go`
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/grpc_service_ready_test.go`

- [ ] **Step 1: Write failing test for new trigger constant**

Create `internal/game/reaction/trigger_test.go`:

```go
package reaction_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestTriggerOnEnemyEntersRoom_Exists(t *testing.T) {
    assert.Equal(t, reaction.ReactionTriggerType("on_enemy_enters_room"), reaction.TriggerOnEnemyEntersRoom)
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/reaction/... -run TestTriggerOnEnemyEntersRoom_Exists -v
```

Expected: FAIL — `TriggerOnEnemyEntersRoom` undefined

- [ ] **Step 3: Add TriggerOnEnemyEntersRoom to trigger.go**

In `internal/game/reaction/trigger.go`, after the `TriggerOnAllyDamaged` constant:

```go
// TriggerOnEnemyEntersRoom fires when an NPC combatant moves in the player's current room.
// Fire point: after consumable trap evaluation in the onCombatantMoved callback (REQ-READY-15).
TriggerOnEnemyEntersRoom ReactionTriggerType = "on_enemy_enters_room"
```

- [ ] **Step 4: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/reaction/... -run TestTriggerOnEnemyEntersRoom_Exists -v
```

Expected: PASS

- [ ] **Step 5: Write failing test for session fields**

Create `internal/gameserver/grpc_service_ready_test.go`:

```go
package gameserver

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/character"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestPlayerSession_ReadiedFields_DefaultEmpty(t *testing.T) {
    mgr := session.NewManager()
    _, err := mgr.AddPlayer(session.AddPlayerOptions{
        UID: "u1", Username: "u1_user", CharName: "u1_char",
        CharacterID: 1, RoomID: "r1",
        CurrentHP: 10, MaxHP: 10,
        Abilities: character.AbilityScores{},
        Role: "player", Level: 1,
    })
    require.NoError(t, err)
    sess, ok := mgr.GetPlayer("u1")
    require.True(t, ok)
    assert.Equal(t, "", sess.ReadiedTrigger, "ReadiedTrigger must default to empty")
    assert.Equal(t, "", sess.ReadiedAction, "ReadiedAction must default to empty")
}
```

- [ ] **Step 6: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestPlayerSession_ReadiedFields_DefaultEmpty -v
```

Expected: FAIL — `ReadiedTrigger` undefined on `PlayerSession`

- [ ] **Step 7: Add ReadiedTrigger and ReadiedAction to PlayerSession**

In `internal/game/session/manager.go`, after the `ReactionFn` field (currently the last field in `PlayerSession`):

```go
// ReadiedTrigger is the trigger alias this player is waiting for ("enemy_enters",
// "enemy_attacks_me", "ally_attacked"). Empty string means no readied action this round.
// Not persisted. In-session only.
ReadiedTrigger string
// ReadiedAction is the action to execute when ReadiedTrigger fires
// ("strike", "step", "raise_shield"). Empty when ReadiedTrigger is empty.
// Not persisted. In-session only.
ReadiedAction string
```

- [ ] **Step 8: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestPlayerSession_ReadiedFields_DefaultEmpty -v
```

Expected: PASS

- [ ] **Step 9: Write failing test for end-of-round expiry clearing**

Add to `internal/gameserver/grpc_service_ready_test.go`:

```go
func TestReadyAction_ClearReadiedAction_ClearsFields(t *testing.T) {
    mgr := session.NewManager()
    _, err := mgr.AddPlayer(session.AddPlayerOptions{
        UID: "u1", Username: "u1_user", CharName: "u1_char",
        CharacterID: 1, RoomID: "r1",
        CurrentHP: 10, MaxHP: 10,
        Abilities: character.AbilityScores{},
        Role: "player", Level: 1,
    })
    require.NoError(t, err)
    sess, ok := mgr.GetPlayer("u1")
    require.True(t, ok)
    sess.ReadiedTrigger = "enemy_enters"
    sess.ReadiedAction = "strike"

    clearReadiedAction(sess)

    assert.Equal(t, "", sess.ReadiedTrigger)
    assert.Equal(t, "", sess.ReadiedAction)
}
```

- [ ] **Step 10: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestReadyAction_ClearReadiedAction -v
```

Expected: FAIL — `clearReadiedAction` undefined

- [ ] **Step 11: Add clearReadiedAction helper and expiry logic to combat_handler.go**

In `internal/gameserver/combat_handler.go`, add the helper function near the end of the file:

```go
// clearReadiedAction clears ReadiedTrigger and ReadiedAction on a session.
// Called at end-of-round to enforce REQ-READY-1.
func clearReadiedAction(sess *session.PlayerSession) {
    sess.ReadiedTrigger = ""
    sess.ReadiedAction = ""
}
```

Then locate the end-of-round per-player reset loop (around line 1564, where `sess.ReactionsRemaining = 1` is set). Add the expiry notification BEFORE the ReactionsRemaining reset:

```go
// REQ-READY-1: Clear readied action at end of every round; notify player if it expired unfired.
if sess.ReadiedTrigger != "" {
    // Notify the player that their readied action expired without firing.
    actionName := sess.ReadiedAction
    _ = sess // suppress linter if needed
    evt := &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_Message{
            Message: &gamev1.MessageEvent{Content: "Your readied " + actionName + " expires. (No refund.)"},
        },
    }
    if data, err := proto.Marshal(evt); err == nil {
        _ = sess.Entity.Push(data)
    }
}
clearReadiedAction(sess)
sess.ReactionsRemaining = 1
```

Note: `proto` must already be imported in `combat_handler.go`. Check `import` block; if absent, add `"google.golang.org/protobuf/proto"`.

- [ ] **Step 12: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestReadyAction_ClearReadiedAction -v
```

Expected: PASS

- [ ] **Step 13: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 14: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/reaction/trigger.go internal/game/reaction/trigger_test.go internal/game/session/manager.go internal/gameserver/combat_handler.go internal/gameserver/grpc_service_ready_test.go
git commit -m "feat: add TriggerOnEnemyEntersRoom, ReadiedTrigger/ReadiedAction session fields, end-of-round expiry"
```

---

## Task 2: Proto + Command + Bridge Handler

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add ReadyRequest to proto**

In `api/proto/game/v1/game.proto`:

1. Add the message definition (near other action request messages like `DeployTrapRequest`):

```protobuf
message ReadyRequest {
    string action = 1;   // "strike" | "step" | "shield"
    string trigger = 2;  // "enters" | "attacks" | "ally"
}
```

2. In `ClientMessage.payload` oneof, add after the `deploy_trap = 85` line:

```protobuf
ReadyRequest ready = 86;
```

- [ ] **Step 2: Regenerate proto**

```
cd /home/cjohannsen/src/mud && make proto
```

Expected: Generated files updated; `ReadyRequest` and `ClientMessage_Ready` now exist.

- [ ] **Step 3: Write failing test for command registration**

Add to `internal/gameserver/grpc_service_ready_test.go` (file started in Task 1):

```go
func TestHandlerReady_Registered(t *testing.T) {
    cmds := command.DefaultRegistry().Commands()
    var found bool
    for _, c := range cmds {
        if c.Handler == command.HandlerReady {
            found = true
            break
        }
    }
    assert.True(t, found, "HandlerReady must be registered in command registry")
}
```

Note: Check the actual method name on the command registry for listing all commands; it may be `.All()`, `.Commands()`, or similar. Read `internal/game/command/registry.go` to confirm before writing the test.

- [ ] **Step 4: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandlerReady_Registered -v
```

Expected: FAIL — `HandlerReady` undefined

- [ ] **Step 5: Add HandlerReady to commands.go**

In `internal/game/command/commands.go`, add constant after `HandlerDeployTrap`:

```go
HandlerReady = "ready"
```

Add command entry in `BuiltinCommands()` (near the deploy_trap entry):

```go
{Name: "ready", Aliases: []string{"rdy"}, Help: "ready <action> when <trigger> — ready a reaction (2 AP); actions: strike/step/shield; triggers: enters/attacks/ally", Category: CategoryCombat, Handler: HandlerReady},
```

- [ ] **Step 6: Run test to verify it passes**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandlerReady_Registered -v
```

Expected: PASS

- [ ] **Step 7: Write failing test for bridge handler**

Read `internal/frontend/handlers/bridge_handlers_test.go` to understand the test pattern, then add:

```go
func TestBridgeReady_ParsesActionAndTrigger(t *testing.T) {
    // Read existing bridge handler tests to confirm the exact helper for creating bctx.
    // Below follows the same pattern as existing bridge tests in this package.
    bctx := makeBridgeContext("ready", "strike when enters", "req1")
    result, err := bridgeReady(bctx)
    require.NoError(t, err)
    msg := result.msg
    require.NotNil(t, msg)
    rr := msg.GetReady()
    require.NotNil(t, rr)
    assert.Equal(t, "strike", rr.Action)
    assert.Equal(t, "enters", rr.Trigger)
}

func TestBridgeReady_MissingArgs_ReturnsUsageError(t *testing.T) {
    bctx := makeBridgeContext("ready", "", "req1")
    result, err := bridgeReady(bctx)
    require.NoError(t, err)
    assert.Contains(t, result.prompt, "Usage:")
}
```

Note: `makeBridgeContext` may or may not exist. Read the test file to determine the actual helper pattern used in other bridge handler tests, and follow it exactly.

- [ ] **Step 8: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestBridgeReady -v
```

Expected: FAIL — `bridgeReady` undefined

- [ ] **Step 9: Add bridgeReady to bridge_handlers.go**

Parse `"strike when enters"` by splitting on `" when "`:

```go
func bridgeReady(bctx *bridgeContext) (bridgeResult, error) {
    args := strings.TrimSpace(bctx.parsed.RawArgs)
    parts := strings.SplitN(args, " when ", 2)
    if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
        return writeErrorPrompt(bctx, "Usage: ready <action> when <trigger>\n  actions: strike, step, shield\n  triggers: enters, attacks, ally")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_Ready{Ready: &gamev1.ReadyRequest{
            Action:  strings.TrimSpace(parts[0]),
            Trigger: strings.TrimSpace(parts[1]),
        }},
    }}, nil
}
```

Register in `bridgeHandlerMap`:

```go
command.HandlerReady: bridgeReady,
```

- [ ] **Step 10: Run bridge tests to verify they pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/handlers/... -run TestBridgeReady -v
```

Expected: PASS

- [ ] **Step 11: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 12: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/game.proto internal/game/command/commands.go internal/frontend/handlers/bridge_handlers.go
# Also stage any generated proto files:
git add internal/gameserver/gamev1/
git commit -m "feat: add ReadyRequest proto, HandlerReady command, bridgeReady handler"
```

---

## Task 3: handleReady Handler + Dispatch

**Files:**
- Create: `internal/gameserver/grpc_service_ready.go`
- Modify: `internal/gameserver/grpc_service_ready_test.go` (append tests)
- Modify: `internal/gameserver/grpc_service.go`

### Alias tables

Action aliases → canonical action names:
- `"strike"` → `"strike"`
- `"step"` → `"step"`
- `"shield"` → `"raise_shield"`

Trigger aliases → canonical trigger names:
- `"enters"` → `"enemy_enters"`
- `"attacks"` → `"enemy_attacks_me"`
- `"ally"` → `"ally_attacked"`

### Preconditions

1. Session exists
2. `sess.Status == statusInCombat` (REQ-READY-2) — `statusInCombat` is a package-level constant in `action_handler.go`
3. `combatH` is non-nil (required for AP spend)
4. `sess.ReactionsRemaining > 0` (REQ-READY-5)
5. `sess.ReadiedTrigger == ""` (REQ-READY-4)
6. Action alias recognized (REQ-READY-6)
7. Trigger alias recognized (REQ-READY-6)
8. `combatH.SpendAP(uid, 2)` succeeds (REQ-READY-3)

- [ ] **Step 1: Write failing tests**

Append to `internal/gameserver/grpc_service_ready_test.go` (after existing Task 1 tests):

```go
// newReadyService creates a minimal GameServiceServer for handleReady tests.
// It has no combatH, so tests that need AP spending require a mock or skip.
func newReadyTestService(t *testing.T) *GameServiceServer {
    t.Helper()
    return newSummonItemService(t, nil, nil)
}

func addReadyCombatPlayer(t *testing.T, svc *GameServiceServer, uid string) *session.PlayerSession {
    t.Helper()
    addSummonTestPlayer(t, svc, uid, "room_a", "player")
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.Status = statusInCombat
    sess.ReactionsRemaining = 1
    return sess
}

func TestHandleReady_SessionNotFound_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    resp, err := svc.handleReady("no_such_uid", &gamev1.ReadyRequest{Action: "strike", Trigger: "enters"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "not found")
}

func TestHandleReady_NotInCombat_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    addSummonTestPlayer(t, svc, "u1", "room_a", "player")
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "strike", Trigger: "enters"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "combat")
}

func TestHandleReady_ReactionSpent_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    sess := addReadyCombatPlayer(t, svc, "u1")
    sess.ReactionsRemaining = 0
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "strike", Trigger: "enters"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "reaction")
}

func TestHandleReady_AlreadyReadied_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    sess := addReadyCombatPlayer(t, svc, "u1")
    sess.ReadiedTrigger = "enemy_enters"
    sess.ReadiedAction = "strike"
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "step", Trigger: "ally"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "already")
}

func TestHandleReady_UnknownAction_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    addReadyCombatPlayer(t, svc, "u1")
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "bogus", Trigger: "enters"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "action")
}

func TestHandleReady_UnknownTrigger_ReturnsError(t *testing.T) {
    svc := newReadyTestService(t)
    addReadyCombatPlayer(t, svc, "u1")
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "strike", Trigger: "bogus"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "trigger")
}

func TestHandleReady_NilCombatH_ReturnsError(t *testing.T) {
    // combatH is nil in newReadyTestService; with valid action/trigger, handler must
    // return an error (can't spend AP without combatH).
    svc := newReadyTestService(t)
    addReadyCombatPlayer(t, svc, "u1")
    resp, err := svc.handleReady("u1", &gamev1.ReadyRequest{Action: "strike", Trigger: "enters"})
    require.NoError(t, err)
    errEvt := resp.GetError()
    require.NotNil(t, errEvt)
    // Error is "combat handler unavailable" (before alias validation) or "not enough AP"
    // depending on validation order. Either is acceptable.
    assert.NotEmpty(t, errEvt.Message)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleReady_" -v 2>&1 | head -30
```

Expected: FAIL — `handleReady` undefined

- [ ] **Step 3: Create grpc_service_ready.go**

```go
package gameserver

import (
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// actionAliases maps ready-action command aliases to canonical action names.
var actionAliases = map[string]string{
    "strike": "strike",
    "step":   "step",
    "shield": "raise_shield",
}

// triggerAliases maps ready-action trigger aliases to canonical trigger names.
var triggerAliases = map[string]string{
    "enters":  "enemy_enters",
    "attacks": "enemy_attacks_me",
    "ally":    "ally_attacked",
}

// triggerDescriptionsReady provides human-readable descriptions for each canonical trigger.
var triggerDescriptionsReady = map[string]string{
    "enemy_enters":     "an enemy enters the room",
    "enemy_attacks_me": "an enemy attacks you",
    "ally_attacked":    "an ally is attacked",
}

// actionNamesReady provides human-readable names for each canonical action.
var actionNamesReady = map[string]string{
    "strike":      "Strike",
    "step":        "Step",
    "raise_shield": "Raise Shield",
}

// handleReady handles the `ready <action> when <trigger>` command (REQ-READY-1 through REQ-READY-6).
//
// Preconditions:
//   - Player session exists.
//   - Player is in combat.
//   - combatH is non-nil.
//   - ReactionsRemaining > 0.
//   - ReadiedTrigger is empty.
//   - Action and trigger aliases are recognized.
//   - SpendAP(uid, 2) succeeds.
//
// Postcondition: Deducts 2 AP; sets ReadiedTrigger and ReadiedAction; returns confirmation message.
func (s *GameServiceServer) handleReady(uid string, req *gamev1.ReadyRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("Player not found."), nil
    }

    // REQ-READY-2: Must be in combat.
    if sess.Status != statusInCombat {
        return errorEvent("You can only ready an action while in combat."), nil
    }

    if s.combatH == nil {
        return errorEvent("Combat handler unavailable."), nil
    }

    // REQ-READY-5: Reaction already spent.
    if sess.ReactionsRemaining <= 0 {
        return errorEvent("Your reaction is already spent this round."), nil
    }

    // REQ-READY-4: Already readied.
    if sess.ReadiedTrigger != "" {
        return errorEvent("You already have a readied action this round."), nil
    }

    // REQ-READY-6: Validate action alias.
    canonicalAction, ok := actionAliases[req.GetAction()]
    if !ok {
        return errorEvent("Unknown action. Valid actions: strike, step, shield."), nil
    }

    // REQ-READY-6: Validate trigger alias.
    canonicalTrigger, ok := triggerAliases[req.GetTrigger()]
    if !ok {
        return errorEvent("Unknown trigger. Valid triggers: enters, attacks, ally."), nil
    }

    // REQ-READY-3: Spend 2 AP.
    if err := s.combatH.SpendAP(uid, 2); err != nil {
        return errorEvent(err.Error()), nil
    }

    sess.ReadiedTrigger = canonicalTrigger
    sess.ReadiedAction = canonicalAction

    actionName := actionNamesReady[canonicalAction]
    triggerDesc := triggerDescriptionsReady[canonicalTrigger]
    return messageEvent("You ready a " + actionName + ". Waiting for: " + triggerDesc + "."), nil
}
```

- [ ] **Step 4: Add dispatch case in grpc_service.go**

Find the `case *gamev1.ClientMessage_DeployTrap:` dispatch case and add after it:

```go
case *gamev1.ClientMessage_Ready:
    return s.handleReady(uid, p.Ready)
```

- [ ] **Step 5: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleReady_" -v
```

Expected: All pass

- [ ] **Step 6: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service_ready.go internal/gameserver/grpc_service_ready_test.go internal/gameserver/grpc_service.go
git commit -m "feat: add handleReady handler with validation and AP spend"
```

---

## Task 4: Extend buildReactionCallback for Ready Action Execution

**Files:**
- Modify: `internal/gameserver/reaction_handler.go`
- Modify: `internal/gameserver/grpc_service_reaction_test.go`

### Architecture

When `buildReactionCallback` fires with a trigger type that matches `sess.ReadiedTrigger`, execute the readied action automatically. The trigger-to-ReadiedTrigger mapping:

| `reaction.ReactionTriggerType` | `sess.ReadiedTrigger` |
|---|---|
| `TriggerOnDamageTaken` | `"enemy_attacks_me"` |
| `TriggerOnAllyDamaged` | `"ally_attacked"` |
| `TriggerOnEnemyEntersRoom` | `"enemy_enters"` |

The readied action check runs BEFORE the existing tech reaction logic. If a readied action fires, `ReactionsRemaining` is decremented (gates further tech reactions).

### executeReadiedStrike implementation note

Use `combat.ResolveAttack(attacker, target, s.dice.Src())` from `internal/game/combat/resolver.go`. The `Combatant` struct has fields `AC` (not `ArmorClass`), `StrMod`, `AttackMod`, `ACMod`, `Position`, `WeaponDamageType`, `WeaponProficiencyRank`, `Level`. After damage: update NPC HP via `h.npcMgr` if available, or player HP via `sess.CurrentHP`.

For the ready-action Strike, we perform one attack (not the full 2-attack `handleStrike` routine). This satisfies REQ-READY-10 (same resolution logic = same `ResolveAttack` function).

For the ready-action Step, move `combatant.Position += 5` (toward), which matches `handleStep`'s position update logic.

For the ready-action Raise Shield, apply shield_raised condition + ACMod via `s.combatH.ApplyCombatantACMod(uid, uid, +2)` (method confirmed to exist in `combat_handler.go`).

All messages use `s.pushMessageToUID(uid, text)` (exists at line 2743 of `grpc_service.go`) — no stream parameter needed.

- [ ] **Step 1: Write failing tests**

Append to `internal/gameserver/grpc_service_reaction_test.go`:

```go
// TestReadyAction_DamageTaken_FiresWhenMatches verifies that when TriggerOnDamageTaken fires
// and the player has ReadiedTrigger="enemy_attacks_me", executeReadiedAction is called:
// ReactionsRemaining decrements and ReadiedTrigger/ReadiedAction are cleared.
func TestReadyAction_DamageTaken_FiresWhenMatches(t *testing.T) {
    worldMgr, sessMgr := testWorldAndSession(t)
    cmdRegistry := command.DefaultRegistry()
    worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
    chatHandler := NewChatHandler(sessMgr)
    logger := zaptest.NewLogger(t)
    svc := NewGameServiceServer(
        worldMgr, sessMgr, cmdRegistry,
        worldHandler, chatHandler, logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil,
        nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
        nil,
        nil,
        nil, nil,
    )
    addSummonTestPlayer(t, svc, "player1", "room_a", "player")
    sess, _ := svc.sessions.GetPlayer("player1")
    sess.Status = statusInCombat
    sess.ReactionsRemaining = 1
    sess.ReadiedTrigger = "enemy_attacks_me"
    sess.ReadiedAction = "strike"

    dmg := 5
    reactionFn := svc.buildReactionCallback("player1", sess, nil)
    spent, err := reactionFn("player1", reaction.TriggerOnDamageTaken, reaction.ReactionContext{
        TriggerUID:    "player1",
        SourceUID:     "npc_goblin",
        DamagePending: &dmg,
    })

    require.NoError(t, err)
    assert.True(t, spent, "reaction must be marked spent when readied trigger matches")
    assert.Equal(t, 0, sess.ReactionsRemaining, "ReactionsRemaining must decrement")
    assert.Equal(t, "", sess.ReadiedTrigger, "ReadiedTrigger must be cleared")
    assert.Equal(t, "", sess.ReadiedAction, "ReadiedAction must be cleared")
}

func TestReadyAction_TriggerMismatch_DoesNotFire(t *testing.T) {
    worldMgr, sessMgr := testWorldAndSession(t)
    cmdRegistry := command.DefaultRegistry()
    worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
    chatHandler := NewChatHandler(sessMgr)
    logger := zaptest.NewLogger(t)
    svc := NewGameServiceServer(
        worldMgr, sessMgr, cmdRegistry,
        worldHandler, chatHandler, logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil,
        nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
        nil,
        nil,
        nil, nil,
    )
    addSummonTestPlayer(t, svc, "player1", "room_a", "player")
    sess, _ := svc.sessions.GetPlayer("player1")
    sess.Status = statusInCombat
    sess.ReactionsRemaining = 1
    sess.ReadiedTrigger = "ally_attacked"  // waiting for ally trigger
    sess.ReadiedAction = "strike"

    dmg := 5
    reactionFn := svc.buildReactionCallback("player1", sess, nil)
    // Fire a different trigger: TriggerOnDamageTaken != "ally_attacked"
    spent, err := reactionFn("player1", reaction.TriggerOnDamageTaken, reaction.ReactionContext{
        TriggerUID:    "player1",
        SourceUID:     "npc_goblin",
        DamagePending: &dmg,
    })

    require.NoError(t, err)
    assert.False(t, spent, "reaction must NOT fire when trigger doesn't match ReadiedTrigger")
    assert.Equal(t, 1, sess.ReactionsRemaining, "ReactionsRemaining must be unchanged")
    assert.Equal(t, "ally_attacked", sess.ReadiedTrigger, "ReadiedTrigger must be unchanged")
}

func TestReadyAction_NoReadiedTrigger_FallsThroughToTechReaction(t *testing.T) {
    // When ReadiedTrigger is empty, the callback must fall through to tech reaction logic.
    // With no registered tech reactions, spent=false.
    worldMgr, sessMgr := testWorldAndSession(t)
    cmdRegistry := command.DefaultRegistry()
    worldHandler := NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil)
    chatHandler := NewChatHandler(sessMgr)
    logger := zaptest.NewLogger(t)
    svc := NewGameServiceServer(
        worldMgr, sessMgr, cmdRegistry,
        worldHandler, chatHandler, logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil,
        nil,
        nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
        nil,
        nil,
        nil, nil,
    )
    addSummonTestPlayer(t, svc, "player1", "room_a", "player")
    sess, _ := svc.sessions.GetPlayer("player1")
    sess.Status = statusInCombat
    sess.ReactionsRemaining = 1
    // ReadiedTrigger is "" (default)

    dmg := 5
    reactionFn := svc.buildReactionCallback("player1", sess, nil)
    spent, err := reactionFn("player1", reaction.TriggerOnDamageTaken, reaction.ReactionContext{
        TriggerUID:    "player1",
        SourceUID:     "npc_goblin",
        DamagePending: &dmg,
    })

    require.NoError(t, err)
    assert.False(t, spent, "no reaction registered and no readied action: spent must be false")
    assert.Equal(t, 1, sess.ReactionsRemaining, "ReactionsRemaining unchanged when not spent")
}
```

Note: The `NewGameServiceServer` call above must match the current signature exactly. Read `internal/gameserver/grpc_service.go` to confirm the current parameter count and order before writing these tests. Use the same pattern as `newSummonItemServiceOpts` in `summon_item_handler_test.go` as a reference for the nil-filled constructor.

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestReadyAction_" -v 2>&1 | head -30
```

Expected: FAIL — ready-action check not yet in `buildReactionCallback`

- [ ] **Step 3: Add triggerToReadiedTrigger map and executeReadiedAction to reaction_handler.go**

In `internal/gameserver/reaction_handler.go`, after the imports block and before `buildReactionCallback`, add:

```go
// triggerToReadiedTrigger maps a ReactionTriggerType to the canonical ReadiedTrigger string.
var triggerToReadiedTrigger = map[reaction.ReactionTriggerType]string{
    reaction.TriggerOnDamageTaken:     "enemy_attacks_me",
    reaction.TriggerOnAllyDamaged:     "ally_attacked",
    reaction.TriggerOnEnemyEntersRoom: "enemy_enters",
}
```

Then add the execution helpers after `buildReactionCallback`:

```go
// executeReadiedAction executes the player's readied action targeting sourceUID (REQ-READY-10, 11, 12).
// Does not cost AP. Decrements ReactionsRemaining, clears ReadiedTrigger/ReadiedAction, and notifies.
//
// Precondition: sess.ReadiedTrigger != "" and sess.ReadiedAction != "".
// Postcondition: Action executed; readied state cleared; ReactionsRemaining decremented.
func (s *GameServiceServer) executeReadiedAction(uid string, sess *session.PlayerSession, sourceUID string) {
    action := sess.ReadiedAction
    actionName := actionNamesReady[action]

    // REQ-READY-12: Decrement reaction and clear state.
    sess.ReactionsRemaining--
    clearReadiedAction(sess)

    // Notify player (REQ-READY-12 step 5).
    s.pushMessageToUID(uid, "Your readied "+actionName+" fires!")

    // Notify room (REQ-READY-12 step 6).
    if playerSess, ok := s.sessions.GetPlayer(uid); ok {
        s.broadcastToRoom(playerSess.RoomID, uid, &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{Content: playerSess.CharName + " reacts with a " + actionName + "!"},
            },
        })
    }

    // Execute action (REQ-READY-10: same resolution logic as non-readied equivalent).
    switch action {
    case "strike":
        s.executeReadiedStrike(uid, sess, sourceUID)
    case "step":
        s.executeReadiedStep(uid, sess)
    case "raise_shield":
        s.executeReadiedRaiseShield(uid, sess)
    }
}

// executeReadiedStrike performs one attack against targetUID at no AP cost.
// Uses combat.ResolveAttack, which is the same function used in round.go (REQ-READY-10).
//
// Precondition: sess must have an active combat via combatH.
// Postcondition: One attack resolved; target HP updated; narrative sent to player.
func (s *GameServiceServer) executeReadiedStrike(uid string, sess *session.PlayerSession, targetUID string) {
    if s.combatH == nil || s.dice == nil {
        s.pushMessageToUID(uid, "Your readied Strike cannot resolve — combat unavailable.")
        return
    }
    cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
    if !ok {
        return
    }
    attacker := cbt.GetCombatant(uid)
    target := cbt.GetCombatant(targetUID)
    if attacker == nil || target == nil {
        s.pushMessageToUID(uid, "Your readied Strike cannot find the target.")
        return
    }

    result := combat.ResolveAttack(attacker, target, s.dice.Src())
    dmg := result.EffectiveDamage()
    if dmg > 0 {
        target.ApplyDamage(dmg)
        // Update NPC HP in manager if target is an NPC.
        if inst, found := s.npcMgr.Get(targetUID); found {
            inst.CurrentHP = target.CurrentHP
        }
    }

    var msg string
    switch result.Outcome {
    case combat.CritSuccess:
        msg = fmt.Sprintf("Your readied Strike critically hits %s for %d damage!", target.Name, dmg)
    case combat.Success:
        msg = fmt.Sprintf("Your readied Strike hits %s for %d damage!", target.Name, dmg)
    default:
        msg = fmt.Sprintf("Your readied Strike misses %s.", target.Name)
    }
    s.pushMessageToUID(uid, msg)
}

// executeReadiedStep moves the player 5ft toward their target at no AP cost.
// Uses the same position update as handleStep (REQ-READY-10).
//
// Precondition: sess must have an active combat via combatH.
// Postcondition: Player combatant's Position updated by 5.
func (s *GameServiceServer) executeReadiedStep(uid string, sess *session.PlayerSession) {
    if s.combatH == nil {
        return
    }
    cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
    if !ok {
        return
    }
    combatant := cbt.GetCombatant(uid)
    if combatant == nil {
        return
    }
    combatant.Position += 5
    s.pushMessageToUID(uid, "Your readied Step moves you 5 ft toward your opponent.")
}

// executeReadiedRaiseShield raises the player's shield at no AP cost.
// Uses the same condition and ACMod application as handleRaiseShield (REQ-READY-10).
//
// Precondition: sess must have a shield equipped.
// Postcondition: shield_raised condition applied; +2 ACMod applied.
func (s *GameServiceServer) executeReadiedRaiseShield(uid string, sess *session.PlayerSession) {
    if sess.LoadoutSet == nil {
        s.pushMessageToUID(uid, "Your readied Raise Shield fails — no equipment loaded.")
        return
    }
    preset := sess.LoadoutSet.ActivePreset()
    if preset == nil || preset.OffHand == nil || !preset.OffHand.Def.IsShield() {
        s.pushMessageToUID(uid, "Your readied Raise Shield fails — no shield equipped.")
        return
    }
    if s.combatH != nil {
        _ = s.combatH.ApplyCombatantACMod(uid, uid, +2)
    }
    if s.condRegistry != nil {
        if def, ok := s.condRegistry.Get("shield_raised"); ok {
            if sess.Conditions == nil {
                sess.Conditions = condition.NewActiveSet()
            }
            _ = sess.Conditions.Apply(uid, def, 1, -1)
        }
    }
    s.pushMessageToUID(uid, "Your readied Raise Shield activates! (+2 AC until start of next turn)")
}
```

Note: `s.npcMgr` — check the `GameServiceServer` struct for the exact field name for the NPC manager. It may be `npcMgr`, `npcManager`, or similar. Read the struct definition in `grpc_service.go`.

Note: `s.broadcastToRoom(roomID, excludeUID, evt)` takes a `*gamev1.ServerEvent` — confirm this matches the current signature.

- [ ] **Step 4: Extend buildReactionCallback to check ReadiedTrigger**

In `internal/gameserver/reaction_handler.go`, inside `buildReactionCallback`, at the START of the inner function (before the existing `if sess.ReactionsRemaining <= 0` check), add:

```go
// REQ-READY-14: Check readied action before tech reactions.
// If the incoming trigger matches the player's readied trigger, execute it automatically.
if sess.ReadiedTrigger != "" {
    if expectedTrigger, ok := triggerToReadiedTrigger[trigger]; ok && sess.ReadiedTrigger == expectedTrigger {
        s.executeReadiedAction(triggerUID, sess, ctx.SourceUID)
        return true, nil
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestReadyAction_" -v
```

Expected: PASS

- [ ] **Step 6: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/reaction_handler.go internal/gameserver/grpc_service_reaction_test.go
git commit -m "feat: execute readied strike/step/raise-shield from buildReactionCallback"
```

---

## Task 5: TriggerOnEnemyEntersRoom Fire Point + Feature Docs

**Files:**
- Modify: `internal/gameserver/grpc_service_trap.go`
- Create: `docs/features/ready-action.md`
- Modify: `docs/features/index.yaml`

### Architecture

`WireConsumableTrapTrigger` sets the single `onCombatantMoved` callback. Extend it to call both `checkConsumableTraps` (traps first, REQ-READY-15) and `checkEnemyEntersReadyTrigger` (ready action second).

`checkEnemyEntersReadyTrigger(roomID, movedCombatantID string)`:
1. Verify mover is an NPC (only NPC movement triggers `enemy_enters`)
2. For each player UID in the room (via `s.sessions.PlayerUIDsInRoom(roomID)`), check if their `ReadiedTrigger == "enemy_enters"` and call their `sess.ReactionFn`

`PlayerUIDsInRoom` exists at line 394 of `session/manager.go` — confirmed.

- [ ] **Step 1: Write failing tests**

In `internal/gameserver/grpc_service_trap_internal_test.go`, append:

```go
func TestCheckEnemyEntersTrigger_NPCMover_FiresReadiedAction(t *testing.T) {
    svc := newTrapTestService(t)
    addSummonTestPlayer(t, svc, "p1", "room1", "player")
    sess, ok := svc.sessions.GetPlayer("p1")
    require.True(t, ok)
    sess.Status = statusInCombat
    sess.ReadiedTrigger = "enemy_enters"
    sess.ReadiedAction = "step"
    sess.ReactionsRemaining = 1

    fired := false
    sess.ReactionFn = func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
        if uid == "p1" && trigger == reaction.TriggerOnEnemyEntersRoom && ctx.SourceUID == "npc_goblin" {
            fired = true
            return true, nil
        }
        return false, nil
    }

    // Create a mock combat with "npc_goblin" as NPC combatant in room1.
    // checkEnemyEntersReadyTrigger needs combatH.CombatantsInRoom to return an NPC.
    // Use the existing test combat setup helpers from this file.
    // (Read this test file to see how combatH is set up in existing trap tests.)

    svc.checkEnemyEntersReadyTrigger("room1", "npc_goblin")

    assert.True(t, fired, "TriggerOnEnemyEntersRoom must fire for player with enemy_enters readied trigger")
}

func TestCheckEnemyEntersTrigger_PlayerMover_DoesNotFire(t *testing.T) {
    svc := newTrapTestService(t)
    addSummonTestPlayer(t, svc, "p1", "room1", "player")
    sess, ok := svc.sessions.GetPlayer("p1")
    require.True(t, ok)
    sess.ReadiedTrigger = "enemy_enters"
    sess.ReactionsRemaining = 1

    fired := false
    sess.ReactionFn = func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
        fired = true
        return true, nil
    }

    // Mover is "p1" — a player, not an NPC. Should not fire.
    svc.checkEnemyEntersReadyTrigger("room1", "p1")
    assert.False(t, fired, "TriggerOnEnemyEntersRoom must NOT fire when mover is a player")
}
```

Note: Before writing the NPC mover test, read `internal/gameserver/grpc_service_trap_internal_test.go` to understand how `newTrapTestService` is defined and how combatH is populated with combatants. The test may require a mock combat handler or injecting combatants via the engine. Follow the existing pattern exactly.

- [ ] **Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestCheckEnemyEntersTrigger" -v 2>&1 | head -20
```

Expected: FAIL — `checkEnemyEntersReadyTrigger` undefined

- [ ] **Step 3: Implement checkEnemyEntersReadyTrigger**

In `internal/gameserver/grpc_service_trap.go`, add after `WireConsumableTrapTrigger`:

```go
// checkEnemyEntersReadyTrigger fires TriggerOnEnemyEntersRoom for all players in the room
// who have ReadiedTrigger == "enemy_enters", when an NPC combatant moves.
//
// Precondition: movedCombatantID must be a combatant in roomID.
// Postcondition: ReactionFn is called for each matching player; reaction state managed by callback.
func (s *GameServiceServer) checkEnemyEntersReadyTrigger(roomID, movedCombatantID string) {
    if s.combatH == nil {
        return
    }
    // Only NPC movement triggers enemy_enters (REQ-READY-8).
    combatants := s.combatH.CombatantsInRoom(roomID)
    var moverIsNPC bool
    for _, c := range combatants {
        if c.ID == movedCombatantID && c.Kind == combat.KindNPC {
            moverIsNPC = true
            break
        }
    }
    if !moverIsNPC {
        return
    }

    // Fire for all players in the room with matching readied trigger.
    uids := s.sessions.PlayerUIDsInRoom(roomID)
    for _, uid := range uids {
        sess, ok := s.sessions.GetPlayer(uid)
        if !ok || sess.ReadiedTrigger != "enemy_enters" || sess.ReactionFn == nil {
            continue
        }
        _, _ = sess.ReactionFn(uid, reaction.TriggerOnEnemyEntersRoom, reaction.ReactionContext{
            TriggerUID: uid,
            SourceUID:  movedCombatantID,
        })
    }
}
```

- [ ] **Step 4: Extend WireConsumableTrapTrigger (REQ-READY-15: traps first, then ready action)**

In `internal/gameserver/grpc_service_trap.go`, modify `WireConsumableTrapTrigger`:

```go
func (s *GameServiceServer) WireConsumableTrapTrigger() {
    if s.combatH == nil {
        return
    }
    s.combatH.SetOnCombatantMoved(func(roomID, movedCombatantID string) {
        // REQ-READY-15: Trap evaluation fires first.
        s.checkConsumableTraps(roomID, movedCombatantID)
        // REQ-READY-8: enemy_enters ready trigger fires after trap evaluation.
        s.checkEnemyEntersReadyTrigger(roomID, movedCombatantID)
    })
}
```

- [ ] **Step 5: Run tests to verify they pass**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestCheckEnemyEntersTrigger" -v
```

Expected: PASS

- [ ] **Step 6: Run full test suite**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 7: Create feature doc**

Create `docs/features/ready-action.md`:

```markdown
# Ready Action

Players may spend 2 AP to ready an action that fires automatically as a Reaction when a trigger condition occurs during round resolution. If the trigger is not met by end of round, the readied action expires with no AP refund.

See `docs/superpowers/specs/2026-03-20-ready-action-design.md` for the full design spec.

## Requirements

- [x] REQ-READY-1: `ReadiedTrigger` and `ReadiedAction` cleared at end of every round.
- [x] REQ-READY-2: `ready` fails if not in combat.
- [x] REQ-READY-3: `ready` fails if fewer than 2 AP remain.
- [x] REQ-READY-4: `ready` fails if a readied action is already set.
- [x] REQ-READY-5: `ready` fails if `ReactionsRemaining == 0`.
- [x] REQ-READY-6: `ready` fails if action or trigger alias is unrecognized.
- [x] REQ-READY-7: Trigger evaluation occurs after each qualifying combat event.
- [x] REQ-READY-8: `enemy_enters` reuses the room-entry event from the consumable-traps `onCombatantMoved` callback.
- [x] REQ-READY-9: `enemy_attacks_me` fires before damage from the triggering attack resolves (via `TriggerOnDamageTaken`).
- [x] REQ-READY-10: Readied action execution uses the same resolution logic as the non-readied equivalent (`combat.ResolveAttack`, `Position +=5`, shield condition).
- [x] REQ-READY-11: Readied action execution does not cost AP.
- [x] REQ-READY-12: Readied action execution decrements `ReactionsRemaining` and clears readied state.
- [x] REQ-READY-13: Expired readied actions notify the player with no AP refund.
- [x] REQ-READY-14: Trigger evaluation uses the existing `ReactionCallback` mechanism (`buildReactionCallback`).
- [x] REQ-READY-15: `TriggerOnEnemyEntersRoom` fires after trap trigger evaluation.

## Implementation

Completed 2026-03-21. `PlayerSession` gained `ReadiedTrigger`/`ReadiedAction` fields. `buildReactionCallback` checks `ReadiedTrigger` before tech reactions and dispatches `executeReadiedAction` (Strike/Step/Raise Shield) without AP cost or prompt. `TriggerOnEnemyEntersRoom` fires from `checkEnemyEntersReadyTrigger` after `checkConsumableTraps` in the `onCombatantMoved` callback. `handleReady` dispatched via proto field 86.
```

- [ ] **Step 8: Update docs/features/index.yaml**

Find the `ready-action` entry (should already exist with status `planned`) and update:

```yaml
  - slug: ready-action
    name: Ready Action
    status: complete
    priority: 242
    category: combat
    file: docs/features/ready-action.md
    effort: "M"
    dependencies:
      - actions
      - consumable-traps
```

- [ ] **Step 9: Run full test suite one final time**

```
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -20
```

Expected: All pass

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service_trap.go docs/features/ready-action.md docs/features/index.yaml
git commit -m "feat: fire TriggerOnEnemyEntersRoom after trap check; add ready-action feature docs"
```
