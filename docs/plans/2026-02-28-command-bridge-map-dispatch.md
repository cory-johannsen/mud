# Command Bridge Map Dispatch Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Replace the monolithic switch in `game_bridge.go` with a map-based dispatch, add the three missing command handlers (loadout, unequip, equipment), and add a completeness test so missing dispatch entries are caught automatically.

**Architecture:** Extract each case from the `commandLoop` switch into a named `bridgeHandlerFunc`, register all handlers in a `var bridgeHandlers map[string]bridgeHandlerFunc`, and add a test that asserts the map contains every handler constant defined in `BuiltinCommands()`. The three missing commands require new proto messages (`LoadoutRequest`, `UnequipRequest`, `EquipmentRequest`), regenerating the proto, adding bridge entries, and adding server-side `handleLoadout`/`handleUnequip`/`handleEquipment` functions in `grpc_service.go`.

**Tech Stack:** Go 1.23, protobuf/grpc (protoc), `make proto` to regenerate, `pgregory.net/rapid` for property tests.

---

### Task 1: Add proto messages for loadout, unequip, equipment

**Files:**
- Modify: `api/proto/game/v1/game.proto`

**Context:** Three new commands need round-trips to the gameserver. They follow the same pattern as `InventoryRequest {}` (no args), `ReloadRequest { string weapon_id = 1; }` (one arg), etc.

**Step 1: Add three new request messages to `game.proto`**

After the `BalanceRequest {}` line (around line 272), add:

```proto
// LoadoutRequest asks the server to display or swap weapon presets.
// If preset is 0, the server returns the current loadout display.
message LoadoutRequest {
  string arg = 1;  // optional preset index ("1" or "2"), empty = display
}

// UnequipRequest asks the server to unequip the item in the given slot.
message UnequipRequest {
  string slot = 1;
}

// EquipmentRequest asks the server to display all equipped items.
message EquipmentRequest {}
```

**Step 2: Add the three messages to the `ClientMessage` oneof**

The current last entry is `TeleportRequest teleport = 26;`. Add after it:

```proto
    LoadoutRequest  loadout   = 27;
    UnequipRequest  unequip   = 28;
    EquipmentRequest equipment = 29;
```

**Step 3: Regenerate the proto**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1
```

Expected: no errors; `internal/gameserver/gamev1/game.pb.go` is updated.

**Step 4: Verify the build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

**Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add LoadoutRequest, UnequipRequest, EquipmentRequest proto messages"
```

---

### Task 2: Add server-side handlers in grpc_service.go

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

**Context:** `handleInventory` and `handleBalance` are the pattern to follow — they look up the session, call a command handler, and return `messageEvent(result)`. The three new commands follow this same pattern:
- `handleLoadout` → calls `command.HandleLoadout(sess, arg)`
- `handleUnequip` → calls `command.HandleUnequip(sess, arg)`
- `handleEquipment` → calls `command.HandleEquipment(sess)`

The `dispatch` function (around line 324) is a proto type switch. Add three new cases and three new handler functions.

**Step 1: Write failing test for handleLoadout**

Add to `internal/gameserver/grpc_service_test.go` (or create `grpc_service_commands_test.go`):

```go
func TestHandleLoadout_DisplaysPresets(t *testing.T) {
    s := newMinimalServer(t)
    uid := addTestPlayer(t, s)
    event, err := s.handleLoadout(uid, &gamev1.LoadoutRequest{Arg: ""})
    require.NoError(t, err)
    require.NotNil(t, event)
    msg := event.GetMessage()
    require.NotNil(t, msg)
    assert.Contains(t, msg.Text, "Preset")
}

func TestHandleUnequip_UnknownSlot(t *testing.T) {
    s := newMinimalServer(t)
    uid := addTestPlayer(t, s)
    event, err := s.handleUnequip(uid, &gamev1.UnequipRequest{Slot: "not_a_slot"})
    require.NoError(t, err)
    require.NotNil(t, event)
    msg := event.GetMessage()
    require.NotNil(t, msg)
    assert.Contains(t, msg.Text, "unknown slot")
}

func TestHandleEquipment_ReturnsEquipmentDisplay(t *testing.T) {
    s := newMinimalServer(t)
    uid := addTestPlayer(t, s)
    event, err := s.handleEquipment(uid, &gamev1.EquipmentRequest{})
    require.NoError(t, err)
    require.NotNil(t, event)
    msg := event.GetMessage()
    require.NotNil(t, msg)
    assert.Contains(t, msg.Text, "Weapons")
}
```

Note: `newMinimalServer` and `addTestPlayer` are helpers you may need to create or adapt from existing test helpers in `grpc_service_login_test.go`.

**Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleLoadout|TestHandleUnequip|TestHandleEquipment" -v 2>&1 | head -20
```

Expected: compile error or FAIL.

**Step 3: Add handler functions in grpc_service.go**

After `handleBalance` (around line 1060), add:

```go
// handleLoadout displays or swaps weapon presets for the player.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent with the loadout display or swap result.
func (s *GameServiceServer) handleLoadout(uid string, req *gamev1.LoadoutRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("player not found"), nil
    }
    return messageEvent(command.HandleLoadout(sess, req.GetArg())), nil
}

// handleUnequip removes the item in the given slot and returns it to the backpack.
//
// Precondition: uid must be a valid connected player; req.Slot must be a valid slot name.
// Postcondition: Returns a ServerEvent with the result string.
func (s *GameServiceServer) handleUnequip(uid string, req *gamev1.UnequipRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("player not found"), nil
    }
    return messageEvent(command.HandleUnequip(sess, req.GetSlot())), nil
}

// handleEquipment displays all equipped armor, accessories, and weapon presets.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a ServerEvent with the equipment display string.
func (s *GameServiceServer) handleEquipment(uid string, _ *gamev1.EquipmentRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return errorEvent("player not found"), nil
    }
    return messageEvent(command.HandleEquipment(sess)), nil
}
```

**Step 4: Wire the three cases into `dispatch`**

In the `dispatch` function (around line 370), after the `Teleport` case:

```go
case *gamev1.ClientMessage_Loadout:
    return s.handleLoadout(uid, p.Loadout)
case *gamev1.ClientMessage_Unequip:
    return s.handleUnequip(uid, p.Unequip)
case *gamev1.ClientMessage_Equipment:
    return s.handleEquipment(uid, p.Equipment)
```

**Step 5: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleLoadout|TestHandleUnequip|TestHandleEquipment" -v 2>&1
```

Expected: PASS.

**Step 6: Run full suite**

```bash
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: all pass.

**Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/
git commit -m "feat: add handleLoadout, handleUnequip, handleEquipment server handlers"
```

---

### Task 3: Replace the commandLoop switch with a map-based dispatch

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/game_bridge_test.go` (create if absent)

**Context:** `commandLoop` currently contains a large `switch cmd.Handler { case ...: }`. Each case builds a `*gamev1.ClientMessage`. We replace this with:

```go
type bridgeHandlerFunc func(reqID string, parsed command.ParsedCommand, conn *telnet.Conn, charName string) (*gamev1.ClientMessage, error, bool)
// bool = handled (false = use default "move" fallback)
```

Wait — looking at the actual cases more carefully, some cases (`help`, `quit`) don't build a `ClientMessage` at all — they do local work and `continue`. The cleanest type is:

```go
// bridgeResult is the result of a bridge handler.
// msg is the ClientMessage to send (nil = nothing to send).
// done is true if the loop should continue (no message to send).
// err is non-nil on fatal error.
type bridgeResult struct {
    msg  *gamev1.ClientMessage
    done bool  // true = handled locally, skip send
}

type bridgeHandlerFunc func(ctx *bridgeContext) (bridgeResult, error)

type bridgeContext struct {
    reqID    string
    parsed   command.ParsedCommand
    conn     *telnet.Conn
    charName string
    role     string
    stream   gamev1.GameService_SessionClient
}
```

**Step 1: Write the completeness test first (TDD)**

Create `internal/frontend/handlers/game_bridge_dispatch_test.go`:

```go
package handlers_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    "github.com/cory-johannsen/mud/internal/game/command"
)

// TestAllCommandHandlersAreWired asserts that every Handler constant
// registered in BuiltinCommands has a corresponding entry in the bridge
// dispatch map. This test exists to enforce the contract: adding a new
// command to commands.go MUST be accompanied by a bridge handler.
//
// Precondition: none.
// Postcondition: every cmd.Handler in BuiltinCommands() is a key in BridgeHandlers().
func TestAllCommandHandlersAreWired(t *testing.T) {
    registered := handlers.BridgeHandlers()
    for _, cmd := range command.BuiltinCommands() {
        if _, ok := registered[cmd.Handler]; !ok {
            t.Errorf("handler %q is registered in BuiltinCommands() but has no entry in BridgeHandlers() — add it to game_bridge.go", cmd.Handler)
        }
    }
}
```

**Step 2: Run test to verify it fails (BridgeHandlers doesn't exist yet)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1 | head -10
```

Expected: compile error "undefined: handlers.BridgeHandlers".

**Step 3: Extract all switch cases into named handler functions**

In `game_bridge.go`, add a new file or section with the `bridgeContext` type and one function per handler. Here are all cases from the existing switch, translated to the new shape. Each function is `func bridgeXxx(bctx *bridgeContext) (bridgeResult, error)`.

Create `internal/frontend/handlers/bridge_handlers.go`:

```go
package handlers

import (
    "fmt"
    "strings"

    "github.com/cory-johannsen/mud/internal/frontend/telnet"
    "github.com/cory-johannsen/mud/internal/game/command"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// bridgeContext carries all inputs needed by a bridge handler function.
type bridgeContext struct {
    reqID    string
    parsed   command.ParsedCommand
    conn     *telnet.Conn
    charName string
    role     string
    stream   gamev1.GameService_SessionClient
}

// bridgeResult is returned by every bridge handler.
// msg is the ClientMessage to send to the server (nil = nothing to send).
// done is true if the command was handled locally (prompt already written; skip send).
type bridgeResult struct {
    msg  *gamev1.ClientMessage
    done bool
}

// bridgeHandlerFunc is the signature for all bridge dispatch functions.
type bridgeHandlerFunc func(bctx *bridgeContext) (bridgeResult, error)

// BridgeHandlers returns the map from Handler constant → bridge function.
// This is exported so the completeness test can iterate it.
func BridgeHandlers() map[string]bridgeHandlerFunc {
    return bridgeHandlerMap
}

// bridgeHandlerMap is the single source of truth for command dispatch.
// Adding a new command to commands.go requires adding an entry here.
var bridgeHandlerMap = map[string]bridgeHandlerFunc{
    command.HandlerMove:      bridgeMove,
    command.HandlerLook:      bridgeLook,
    command.HandlerExits:     bridgeExits,
    command.HandlerSay:       bridgeSay,
    command.HandlerEmote:     bridgeEmote,
    command.HandlerWho:       bridgeWho,
    command.HandlerQuit:      bridgeQuit,
    command.HandlerHelp:      bridgeHelp,
    command.HandlerExamine:   bridgeExamine,
    command.HandlerAttack:    bridgeAttack,
    command.HandlerFlee:      bridgeFlee,
    command.HandlerPass:      bridgePass,
    command.HandlerStrike:    bridgeStrike,
    command.HandlerStatus:    bridgeStatus,
    command.HandlerEquip:     bridgeEquip,
    command.HandlerReload:    bridgeReload,
    command.HandlerFireBurst: bridgeFireBurst,
    command.HandlerFireAuto:  bridgeFireAuto,
    command.HandlerThrow:     bridgeThrow,
    command.HandlerInventory: bridgeInventory,
    command.HandlerGet:       bridgeGet,
    command.HandlerDrop:      bridgeDrop,
    command.HandlerBalance:   bridgeBalance,
    command.HandlerSetRole:   bridgeSetRole,
    command.HandlerTeleport:  bridgeTeleport,
    command.HandlerLoadout:   bridgeLoadout,
    command.HandlerUnequip:   bridgeUnequip,
    command.HandlerEquipment: bridgeEquipment,
}

func writeErrorAndPrompt(bctx *bridgeContext, msg string) (bridgeResult, error) {
    _ = bctx.conn.WriteLine(telnet.Colorize(telnet.Red, msg))
    _ = bctx.conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", bctx.charName))
    return bridgeResult{done: true}, nil
}

func bridgeMove(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: buildMoveMessage(bctx.reqID, bctx.parsed.Command)}, nil
}

func bridgeLook(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Look{Look: &gamev1.LookRequest{}},
    }}, nil
}

func bridgeExits(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Exits{Exits: &gamev1.ExitsRequest{}},
    }}, nil
}

func bridgeSay(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: say <message>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Say{Say: &gamev1.SayRequest{Message: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeEmote(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: emote <action>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Emote{Emote: &gamev1.EmoteRequest{Action: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeWho(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Who{Who: &gamev1.WhoRequest{}},
    }}, nil
}

func bridgeQuit(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Quit{Quit: &gamev1.QuitRequest{}},
    }}, nil
}

func bridgeHelp(bctx *bridgeContext) (bridgeResult, error) {
    // Help is handled locally — no server round-trip.
    // The caller (commandLoop) checks done=true and skips send.
    // The actual help rendering is done after dispatch returns.
    return bridgeResult{done: true}, nil
}

func bridgeExamine(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: examine <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Examine{Examine: &gamev1.ExamineRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeAttack(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: attack <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Attack{Attack: &gamev1.AttackRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeFlee(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Flee{Flee: &gamev1.FleeRequest{}},
    }}, nil
}

func bridgePass(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Pass{Pass: &gamev1.PassRequest{}},
    }}, nil
}

func bridgeStrike(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: strike <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Strike{Strike: &gamev1.StrikeRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeStatus(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
    }}, nil
}

func bridgeEquip(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: equip <weapon_id> [slot]")
    }
    parts := strings.SplitN(bctx.parsed.RawArgs, " ", 2)
    slot := ""
    if len(parts) == 2 {
        slot = strings.TrimSpace(parts[1])
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Equip{Equip: &gamev1.EquipRequest{WeaponId: strings.TrimSpace(parts[0]), Slot: slot}},
    }}, nil
}

func bridgeReload(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Reload{Reload: &gamev1.ReloadRequest{WeaponId: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeFireBurst(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: burst <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_FireBurst{FireBurst: &gamev1.FireBurstRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeFireAuto(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: auto <target>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_FireAutomatic{FireAutomatic: &gamev1.FireAutomaticRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeThrow(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: throw <explosive_id>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Throw{Throw: &gamev1.ThrowRequest{ExplosiveId: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeInventory(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_InventoryReq{InventoryReq: &gamev1.InventoryRequest{}},
    }}, nil
}

func bridgeGet(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: get <item>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_GetItem{GetItem: &gamev1.GetItemRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeDrop(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: drop <item>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_DropItem{DropItem: &gamev1.DropItemRequest{Target: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeBalance(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Balance{Balance: &gamev1.BalanceRequest{}},
    }}, nil
}

func bridgeSetRole(bctx *bridgeContext) (bridgeResult, error) {
    if len(bctx.parsed.Args) < 2 {
        _ = bctx.conn.WriteLine("Usage: setrole <username> <role>")
        _ = bctx.conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", bctx.charName))
        return bridgeResult{done: true}, nil
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_SetRole{SetRole: &gamev1.SetRoleRequest{
            TargetUsername: bctx.parsed.Args[0],
            Role:           bctx.parsed.Args[1],
        }},
    }}, nil
}

func bridgeTeleport(bctx *bridgeContext) (bridgeResult, error) {
    targetChar := strings.TrimSpace(bctx.parsed.RawArgs)
    if targetChar == "" {
        _ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Character name: "))
        line, err := bctx.conn.ReadLine()
        if err != nil {
            return bridgeResult{}, fmt.Errorf("reading teleport target: %w", err)
        }
        targetChar = strings.TrimSpace(line)
    }
    if targetChar == "" {
        return writeErrorAndPrompt(bctx, "Character name cannot be empty.")
    }
    _ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Room ID: "))
    line, err := bctx.conn.ReadLine()
    if err != nil {
        return bridgeResult{}, fmt.Errorf("reading teleport room: %w", err)
    }
    roomID := strings.TrimSpace(line)
    if roomID == "" {
        return writeErrorAndPrompt(bctx, "Room ID cannot be empty.")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_Teleport{Teleport: &gamev1.TeleportRequest{
            TargetCharacter: targetChar,
            RoomId:          roomID,
        }},
    }}, nil
}

func bridgeLoadout(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Loadout{Loadout: &gamev1.LoadoutRequest{Arg: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeUnequip(bctx *bridgeContext) (bridgeResult, error) {
    if bctx.parsed.RawArgs == "" {
        return writeErrorAndPrompt(bctx, "Usage: unequip <slot>")
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Unequip{Unequip: &gamev1.UnequipRequest{Slot: bctx.parsed.RawArgs}},
    }}, nil
}

func bridgeEquipment(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_Equipment{Equipment: &gamev1.EquipmentRequest{}},
    }}, nil
}
```

**Step 4: Replace the switch in commandLoop with map dispatch**

In `game_bridge.go`, replace the entire `switch cmd.Handler { ... }` block and the `if msg != nil { stream.Send(msg) }` block with:

```go
bctx := &bridgeContext{
    reqID:    reqID,
    parsed:   parsed,
    conn:     conn,
    charName: charName,
    role:     role,
    stream:   stream,
}

handlerFn, ok := bridgeHandlerMap[cmd.Handler]
if !ok {
    _ = conn.WriteLine(telnet.Colorf(telnet.Dim, "You don't know how to '%s'.", parsed.Command))
    _ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
    continue
}

result, err := handlerFn(bctx)
if err != nil {
    return err
}
if result.done {
    // Handler dealt with output locally (e.g., help). Special case: help needs registry.
    if cmd.Handler == command.HandlerHelp {
        h.showGameHelp(conn, registry, role)
        _ = conn.WritePrompt(telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName))
    }
    continue
}
if result.msg != nil {
    if err := stream.Send(result.msg); err != nil {
        return fmt.Errorf("sending message: %w", err)
    }
}
```

Also remove all the now-unused imports from `game_bridge.go` that were only needed by the old switch cases (they will now be in `bridge_handlers.go`). Keep only what `commandLoop` itself uses.

**Step 5: Run the completeness test**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

Expected: PASS — all handler constants are in the map.

**Step 6: Run full frontend test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... -v 2>&1 | tail -20
```

Expected: all pass.

**Step 7: Commit**

```bash
git add internal/frontend/handlers/
git commit -m "refactor: replace commandLoop switch with map-based dispatch; add completeness test"
```

---

### Task 4: Full build and test verification

**Step 1: Build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

**Step 2: Test (excluding Docker)**

```bash
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v storage/postgres) -count=1 2>&1 | tail -15
```

Expected: all pass.

**Step 3: Race detector (excluding Docker)**

```bash
cd /home/cjohannsen/src/mud && go test -race -timeout 60s $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: no races.

**Step 4: Commit if any fixes needed, then tag**

```bash
# Only if fixes were required:
git add -p
git commit -m "fix: address build/test issues from map dispatch refactor"
```

---

## Enforcement Contract

After this plan is implemented, the enforcement contract is:

- **To add a new command:** Add the `Handler` constant to `commands.go`, add an entry to `bridgeHandlerMap` in `bridge_handlers.go`, add a proto message + `ClientMessage` oneof entry in `game.proto`, run `make proto`, add a `dispatch` case + handler function in `grpc_service.go`.
- **If you forget the bridge entry:** `TestAllCommandHandlersAreWired` fails immediately.
- **If you forget the server handler:** The dispatch falls to `default: fmt.Errorf("unknown message type")` which returns an error to the client — visibly broken, not silently ignored.
