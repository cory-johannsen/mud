# Use Command Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `use` to fall through to room equipment when no feat matches, and add tab completion via a `TabCompleteRequest`/`TabCompleteResponse` proto round-trip.

**Architecture:** `handleUse` gets a two-stage lookup (feat/ability first, then `RoomEquipmentManager.GetInstance`). Tab completion uses a dedicated `HandlerTabComplete` CMD entry. The telnet frontend detects `0x09` in `ReadLine`, records the prefix as a side-channel field on the connection struct (REQ-USE-6: buffer unchanged), and the bridge loop dispatches a `TabCompleteRequest` before resuming normal input.

**Tech Stack:** Go, protobuf, testify, rapid

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/gameserver/grpc_service.go` | Extend `handleUse` fall-through; add `handleTabComplete` |
| Modify | `api/proto/game/v1/game.proto` | Add `TabCompleteRequest` (ClientMessage oneof 106); add `TabCompleteResponse` (ServerEvent oneof 26) |
| Regenerate | `api/proto/game/v1/game.pb.go` | `buf generate` |
| Modify | `internal/game/command/commands.go` | Add `HandlerTabComplete` constant; add hidden entry to `BuiltinCommands()` |
| Modify | `internal/frontend/handlers/bridge_handlers.go` | Add `bridgeTabComplete`; handle `TabCompleteResponse` server event; add map entry |
| Modify | `internal/frontend/telnet/conn.go` | Detect `0x09` in ReadLine; set `PendingTabCompletePrefix`; do not add tab to buffer |

---

### Task 1: Extend `handleUse` to fall through to room equipment

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

`handleUse` already handles feat/ability lookup. This task adds: if no feat/ability match, try `RoomEquipmentManager.GetInstance(sess.RoomID, target)` and call the existing `handleUseEquipment` path.

- [ ] **Step 1: Write failing tests**

```go
func TestHandleUse_FeatMatchFirst_EquipmentNotCalled(t *testing.T) {
    // Player has feat "medkit"; room has equipment named "medkit"
    // Send UseRequest{FeatId: "medkit"}
    // Assert feat is activated, not the equipment (REQ-USE-1)
}

func TestHandleUse_NoFeatMatch_FallsThruToEquipment(t *testing.T) {
    // Player has no feat "console"; room has equipment named "console"
    // Send UseRequest{FeatId: "console"}
    // Assert equipment is activated (REQ-USE-1)
}

func TestHandleUse_NeitherMatch_ReturnsError(t *testing.T) {
    // Player has no feat "mystery"; room has no equipment named "mystery"
    // Send UseRequest{FeatId: "mystery"}
    // Assert error "You don't know how to use that."
}

func TestHandleInteract_BypassesFeatLookup(t *testing.T) {
    // Player has feat "console"; room has equipment named "console"
    // Send InteractRequest{Target: "console"}
    // Assert equipment is activated, not the feat (REQ-USE-2)
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_.*Equipment -v
mise exec -- go test ./internal/gameserver/... -run TestHandleInteract -v
```

Expected: FAIL — fall-through not yet implemented.

- [ ] **Step 2: Implement fall-through in `handleUse`**

At the point where `handleUse` would normally return "You don't know how to use that." (when no feat/ability matched), add before that return:

```go
// REQ-USE-1: fall through to room equipment if no feat/ability matched
if abilityID != "" {
    inst := s.roomEquipMgr.GetInstance(sess.RoomID, abilityID)
    if inst != nil {
        return s.handleUseEquipment(uid, inst.InstanceID) // REQ-USE-4: same activation logic
    }
}
```

Note: `GetInstance` performs case-insensitive description matching internally (REQ-USE-3). No changes to `handleUseEquipment` or `handleInteract` are needed — `interact` already calls `handleUseEquipment` directly (REQ-USE-2).

- [ ] **Step 3: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_.*Equipment|TestHandleInteract -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(use-command): use falls through to room equipment when no feat matches (REQ-USE-1)"
```

---

### Task 2: Proto additions

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `api/proto/game/v1/game.pb.go`

- [ ] **Step 1: Verify highest field numbers**

Read `api/proto/game/v1/game.proto` and confirm:
- `ClientMessage` oneof highest field = 105 (`ReleaseRequest release_request`)
- `ServerEvent` oneof highest field = 25 (`HpUpdateEvent hp_update`)

New fields: `TabCompleteRequest` at 106, `TabCompleteResponse` at 26.

- [ ] **Step 2: Add proto messages**

```protobuf
message TabCompleteRequest {
  string prefix = 1; // current input buffer content
}

message TabCompleteResponse {
  repeated string completions = 1; // matching completions, sorted alphabetically
}
```

Add to `ClientMessage` oneof:
```protobuf
TabCompleteRequest tab_complete = 106;
```

Add to `ServerEvent` oneof:
```protobuf
TabCompleteResponse tab_complete = 26;
```

- [ ] **Step 3: Regenerate and build**

```bash
cd /home/cjohannsen/src/mud
mise exec -- buf generate
mise exec -- go build ./...
```

Expected: generates cleanly; build passes.

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto api/proto/game/v1/game.pb.go
git commit -m "feat(use-command): add TabCompleteRequest/TabCompleteResponse proto messages"
```

---

### Task 3: `HandlerTabComplete` command registration and bridge handler

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Write failing test**

```go
func TestBridgeTabComplete_BuildsTabCompleteRequest(t *testing.T) {
    // bctx with parsed.RawArgs = "use med"
    // Call bridgeTabComplete(bctx)
    // Assert result.msg is ClientMessage_TabComplete with Prefix = "use med"
}
```

- [ ] **Step 1a: Run test to verify it fails**

```bash
mise exec -- go test ./internal/frontend/handlers/... -run TestBridgeTabComplete -v
```

Expected: FAIL — `bridgeTabComplete` not yet defined.

- [ ] **Step 2: Add handler constant and BuiltinCommands entry**

```go
// In commands.go:
HandlerTabComplete = "__tabcomplete__"

// BuiltinCommands entry (hidden from help):
{Name: "__tabcomplete__", Aliases: nil, Help: "", Category: "internal", Handler: HandlerTabComplete, Hidden: true},
```

Note: Check whether `Command` struct has a `Hidden bool` field. If not, omit it — the command simply won't be documented and users won't type it manually. The frontend injects it synthetically (see Task 4).

- [ ] **Step 3: Add `bridgeTabComplete` and map entry**

```go
func bridgeTabComplete(bctx *bridgeContext) (bridgeResult, error) {
    prefix := strings.TrimSpace(bctx.parsed.RawArgs)
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload: &gamev1.ClientMessage_TabComplete{
            TabComplete: &gamev1.TabCompleteRequest{Prefix: prefix},
        },
    }}, nil
}
```

Add `command.HandlerTabComplete: bridgeTabComplete` to `bridgeHandlerMap` (or `BridgeHandlers()` return value).

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/frontend/handlers/... -run TestBridgeTabComplete -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/command/commands.go internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(use-command): add HandlerTabComplete command registration and bridge handler"
```

---

### Task 4: Frontend tab key interception and response rendering

**Files:**
- Modify: `internal/frontend/telnet/conn.go`
- Modify: `internal/frontend/handlers/bridge_handlers.go` (or wherever the bridge loop processes server events)

The key constraints: REQ-USE-5 requires completions to be displayed while the buffer remains unchanged (REQ-USE-6). Since `ReadLine` is a blocking byte-by-byte reader, the solution is to inject a `TabCompleter` callback into the conn struct. The bridge loop sets this callback before entering the read loop; `ReadLine` calls it synchronously when tab is pressed (while still mid-read), which dispatches the request and displays the response.

**Concurrency note:** The bridge architecture uses a `forwardServerEvents` goroutine that calls `stream.Recv()` continuously. A direct `stream.Recv()` inside the `TabCompleter` callback would race with this goroutine. The correct approach: route tab complete responses through the existing event channel infrastructure. The `TabCompleter` callback:
1. Sends the `TabCompleteRequest` via `stream.Send()` (command-loop goroutine — no race, sends are serialized)
2. Waits on a dedicated `conn.tabCompleteResponse chan *gamev1.TabCompleteResponse` channel
3. `forwardServerEvents` is modified to route `TabCompleteResponse` events to `conn.tabCompleteResponse` instead of the normal event path

This is safe because `stream.Send` remains on one goroutine and `stream.Recv` remains on the `forwardServerEvents` goroutine.

- [ ] **Step 1: Write failing tests**

```go
func TestConn_TabKey_InvokesTabCompleter(t *testing.T) {
    // Set conn.TabCompleter = func(prefix string) { captured = prefix }
    // Simulate input: 'u', 's', 'e', ' ', '\t', 'm', '\n'
    // Assert TabCompleter was called with prefix = "use "
    // Assert ReadLine returns "use m" (tab not in result, 'm' + Enter continue normally)
}

func TestConn_TabKey_NilCompleter_DoesNotPanic(t *testing.T) {
    // conn.TabCompleter = nil
    // Simulate input: 'u', '\t', '\n'
    // Assert ReadLine returns "u" (tab silently ignored if no completer)
}

func TestConn_TabKey_DoesNotModifyBuffer(t *testing.T) {
    // Simulate input: 'u', 's', 'e', ' ', '\t', '\n'
    // Assert ReadLine returns "use " (no tab character in result) (REQ-USE-6)
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/frontend/telnet/... -run TestConn_TabKey -v
```

Expected: FAIL — tab handling not yet implemented.

- [ ] **Step 2: Add `TabCompleter` callback and response channel to conn struct**

In `internal/frontend/telnet/conn.go`, add to the connection struct:

```go
// TabCompleter is called when the tab key is pressed mid-input.
// Receives the current buffer contents (without the tab) as the completion prefix.
// Set by the bridge loop before entering the read loop.
TabCompleter func(prefix string)

// tabCompleteResponse is a buffered channel (size 1) that receives TabCompleteResponse
// routed by forwardServerEvents when a TabCompleteResponse arrives.
// Set by the bridge loop alongside TabCompleter.
tabCompleteResponse chan *gamev1.TabCompleteResponse
```

- [ ] **Step 3: Modify `ReadLine` to detect `0x09`**

In the ReadLine input byte loop (and ReadLineSplit similarly), add explicit tab handling before the general "append to buffer" path:

```go
case b == '\t':
    // REQ-USE-5/6: invoke completer with current buffer; do NOT append tab
    if conn.TabCompleter != nil {
        conn.TabCompleter(string(buf))
    }
    continue // tab not appended; REQ-USE-6: buffer unchanged on screen
```

- [ ] **Step 4: Set `TabCompleter` in the bridge loop and route response in `forwardServerEvents`**

In `internal/frontend/handlers/` (search for where `forwardServerEvents` is defined and where `conn.ReadLine()` is called):

**4a: Initialize the channel and set the callback before the read loop:**

```go
conn.tabCompleteResponse = make(chan *gamev1.TabCompleteResponse, 1)
conn.TabCompleter = func(prefix string) {
    reqID := generateReqID() // use whatever request ID generation the bridge loop uses
    tabMsg := &gamev1.ClientMessage{
        RequestId: reqID,
        Payload: &gamev1.ClientMessage_TabComplete{
            TabComplete: &gamev1.TabCompleteRequest{Prefix: prefix},
        },
    }
    if err := stream.Send(tabMsg); err != nil {
        return
    }
    // Wait for response from forwardServerEvents goroutine (timeout: 5 seconds)
    select {
    case resp := <-conn.tabCompleteResponse:
        renderTabCompleteResponse(conn, resp) // REQ-USE-10
    case <-time.After(5 * time.Second):
        // Timeout — silently continue
    }
}
```

**4b: In `forwardServerEvents`, route `TabCompleteResponse` to the channel:**

Find the event dispatch in `forwardServerEvents` (where it reads a `ServerEvent` from `stream.Recv()` and routes to the event channel). Add before the default routing:

```go
if tc := evt.GetTabComplete(); tc != nil && conn.tabCompleteResponse != nil {
    select {
    case conn.tabCompleteResponse <- tc:
    default: // channel full — drop (shouldn't happen in practice)
    }
    continue // do not push to normal event channel
}
```

Note: Verify the request ID generation pattern from the existing bridge loop. Also verify the method name for writing console output to the conn (used in `renderTabCompleteResponse`). Also add `"time"` to imports if not already present.

- [ ] **Step 5: Implement `renderTabCompleteResponse`**

```go
// renderTabCompleteResponse renders tab completions as a console message (REQ-USE-10).
func renderTabCompleteResponse(conn *telnet.Conn, resp *gamev1.TabCompleteResponse) {
    if resp == nil {
        return
    }
    completions := resp.GetCompletions()
    if len(completions) == 0 {
        conn.WriteConsole("No completions found.")
        return
    }
    // REQ-USE-11: show only first 10 with count of remaining
    shown := completions
    suffix := ""
    if len(completions) > 10 {
        shown = completions[:10]
        suffix = fmt.Sprintf(" ... (%d more)", len(completions)-10)
    }
    if len(shown) == 1 {
        conn.WriteConsole(shown[0])
        return
    }
    // 2–10 completions: space-separated list with command prefix extracted
    conn.WriteConsole(strings.Join(shown, "  ") + suffix)
}
```

Note: `conn.WriteConsole` — verify the correct method name for writing to the console region from the frontend connection. Check existing usage in bridge_handlers.go.

- [ ] **Step 6: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/frontend/telnet/... -run TestConn_TabKey -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/frontend/telnet/conn.go internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(use-command): frontend tab key interception and completion display (REQ-USE-5/6/10/11)"
```

---

### Task 5: `handleTabComplete` implementation

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Write failing tests**

```go
func TestHandleTabComplete_EmptyPrefix_ReturnsAllCommands(t *testing.T) {
    // prefix = ""
    // Assert completions include known commands (REQ-USE-7)
}

func TestHandleTabComplete_SingleWordPrefix_FiltersCommands(t *testing.T) {
    // prefix = "mo"
    // Assert completions include "move", "money", etc. but not "attack"
    // REQ-USE-7
}

func TestHandleTabComplete_UsePrefix_ReturnsFeatNames(t *testing.T) {
    // Player has active feat "medkit"; prefix = "use med"
    // Assert completions include "use medkit" (REQ-USE-8)
}

func TestHandleTabComplete_UsePrefix_ReturnsEquipmentDescriptions(t *testing.T) {
    // Room has equipment named "control panel"; prefix = "use cont"
    // Assert completions include "use control panel" (REQ-USE-8)
}

func TestHandleTabComplete_SortedAndDeduped(t *testing.T) {
    // Player has feat "medkit"; room has equipment "medkit"
    // prefix = "use med"
    // Assert completions are sorted and "use medkit" appears only once (REQ-USE-9)
}

func TestHandleTabComplete_Property_CompletionsAlwaysSorted(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        prefix := rapid.String().Draw(t, "prefix")
        // completions from handleTabComplete must always be in ascending lexicographic order
    })
}
```

- [ ] **Step 1a: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleTabComplete -v
```

Expected: FAIL — `handleTabComplete` not yet defined.

- [ ] **Step 2: Implement `handleTabComplete`**

The dispatch function in `grpc_service.go` receives a `ClientMessage` and returns `(*gamev1.ServerEvent, error)` (check the actual signature of the dispatch function — verify by reading existing handler cases). Follow the same pattern:

Add to dispatch switch:
```go
case *gamev1.ClientMessage_TabComplete:
    return s.handleTabComplete(uid, p.TabComplete.GetPrefix(), sess)
```

Implement (returns `*gamev1.ServerEvent` — matching dispatch return type):

```go
func (s *GameServiceServer) handleTabComplete(uid, prefix string, sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
    prefix = strings.ToLower(strings.TrimSpace(prefix))
    var completions []string
    seen := map[string]bool{}

    addCompletion := func(c string) {
        if !seen[c] {
            seen[c] = true
            completions = append(completions, c)
        }
    }

    if !strings.Contains(prefix, " ") {
        // REQ-USE-7: complete command names/aliases
        for _, cmd := range command.BuiltinCommands() {
            if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
                addCompletion(cmd.Name)
            }
            for _, alias := range cmd.Aliases {
                if strings.HasPrefix(strings.ToLower(alias), prefix) {
                    addCompletion(alias)
                }
            }
        }
    } else {
        // REQ-USE-8: contextual completion for use/interact
        parts := strings.SplitN(prefix, " ", 2)
        cmdWord := parts[0]
        partial := ""
        if len(parts) == 2 {
            partial = parts[1]
        }

        if cmdWord == "use" || cmdWord == "interact" {
            // Active feats
            featIDs, _ := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
            for _, fid := range featIDs {
                feat, ok := s.featRegistry.Feat(fid)
                if !ok || !feat.Active {
                    continue
                }
                name := strings.ToLower(feat.Name)
                if strings.HasPrefix(name, partial) {
                    addCompletion(cmdWord + " " + feat.Name)
                }
            }
            // Room equipment descriptions
            for _, inst := range s.roomEquipMgr.EquipmentInRoom(sess.RoomID) {
                desc := strings.ToLower(inst.Description)
                if strings.HasPrefix(desc, partial) {
                    addCompletion(cmdWord + " " + inst.Description)
                }
            }
        }
        // Other commands: no contextual completion (per spec)
    }

    // REQ-USE-9: sort and deduplicate
    sort.Strings(completions)

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_TabComplete{
            TabComplete: &gamev1.TabCompleteResponse{Completions: completions},
        },
    }, nil
}
```

Note: `sort` must be imported. Also ensure the dispatch function's actual signature is matched — read the existing handler cases (e.g., `handleRest`, `handleMove`) to confirm return type before implementing.

- [ ] **Step 3: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run TestHandleTabComplete -v
mise exec -- go test ./...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(use-command): implement handleTabComplete with command and contextual completions (REQ-USE-7/8/9)"
```

---

## Verification Checklist

- [ ] `use medkit` activates feat "medkit" (not room equipment if both exist) (REQ-USE-1)
- [ ] `use console` with no matching feat activates room equipment named "console" (REQ-USE-1)
- [ ] `interact console` activates room equipment, bypassing feat lookup (REQ-USE-2)
- [ ] Room equipment lookup uses case-insensitive description match (REQ-USE-3)
- [ ] Room equipment activated via `use` executes script/skill-check/fallback as `handleUseEquipment` (REQ-USE-4)
- [ ] Tab key sends `TabCompleteRequest` with current buffer as prefix (REQ-USE-5)
- [ ] Tab key does NOT modify or clear the current input buffer (REQ-USE-6)
- [ ] Single-word prefix completes against command names and aliases (REQ-USE-7)
- [ ] `use <partial>` / `interact <partial>` complete feats + room equipment (REQ-USE-8)
- [ ] Completions sorted alphabetically; duplicates removed (REQ-USE-9)
- [ ] Completions displayed as console message; input not auto-filled (REQ-USE-10)
- [ ] More than 10 matches: first 10 shown + "... (N more)" (REQ-USE-11)
- [ ] Full test suite passes with zero failures
