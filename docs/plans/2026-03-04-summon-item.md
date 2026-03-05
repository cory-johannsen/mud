# summon_item Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add a `summon_item <item_id> [quantity]` editor/admin command that places an item instance on the floor of the caller's current room.

**Architecture:** Follows the full CMD-1 through CMD-7 wiring pattern. The gRPC handler looks up the item in `s.invRegistry`, creates an `ItemInstance` with a new UUID, and calls `s.floorMgr.Drop(sess.RoomID, inst)`. Role check requires `editor` or `admin`.

**Tech Stack:** Go, protobuf/gRPC, `pgregory.net/rapid` for property tests, `github.com/google/uuid` for instance IDs.

---

### Task 1: Command constant, entry, and game-layer handler (CMD-1, CMD-2, CMD-3)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/summon_item.go`
- Create: `internal/game/command/summon_item_test.go`

**Step 1: Write the failing test**

Create `internal/game/command/summon_item_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleSummonItem_NoArgs(t *testing.T) {
	result := command.HandleSummonItem("")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_ItemIDOnly(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle")
	assert.Equal(t, "assault_rifle 1", result) // normalized: "itemID qty"
}

func TestHandleSummonItem_WithQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle 3")
	assert.Equal(t, "assault_rifle 3", result)
}

func TestHandleSummonItem_InvalidQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle abc")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_ZeroQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle 0")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestHandleSummonItem_NegativeQuantity(t *testing.T) {
	result := command.HandleSummonItem("assault_rifle -1")
	assert.Equal(t, "Usage: summon_item <item_id> [quantity]", result)
}

func TestPropertyHandleSummonItem_ValidQuantityAlwaysReturnsItemIDAndQty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		qty := rapid.IntRange(1, 1000).Draw(rt, "qty")
		result := command.HandleSummonItem("some_item " + fmt.Sprintf("%d", qty))
		assert.Equal(t, fmt.Sprintf("some_item %d", qty), result)
	})
}
```

Add `"fmt"` to imports.

**Step 2: Run test to verify it fails**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -run TestHandleSummonItem -v 2>&1 | head -20
```

Expected: FAIL — `HandleSummonItem` undefined.

**Step 3: Implement**

Create `internal/game/command/summon_item.go`:

```go
package command

import (
	"fmt"
	"strconv"
	"strings"
)

// HandleSummonItem parses and validates summon_item arguments.
// Precondition: args is the raw argument string after the command name.
// Postcondition: returns "itemID qty" on valid input, usage string otherwise.
func HandleSummonItem(args string) string {
	const usage = "Usage: summon_item <item_id> [quantity]"
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return usage
	}
	itemID := parts[0]
	qty := 1
	if len(parts) >= 2 {
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 {
			return usage
		}
		qty = n
	}
	return fmt.Sprintf("%s %d", itemID, qty)
}
```

Add `HandlerSummonItem` constant and `BuiltinCommands()` entry to `internal/game/command/commands.go`:

```go
// After the last HandlerXxx constant (currently HandlerUse):
HandlerSummonItem = "summon_item"
```

In `BuiltinCommands()`, append:
```go
{Name: "summon_item", Aliases: nil, Help: "Summon an item into the current room (editor+)", Category: CategoryAdmin, Handler: HandlerSummonItem},
```

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -timeout 60s 2>&1 | tail -10
```

Expected: `ok github.com/cory-johannsen/mud/internal/game/command`

**Step 5: Commit**

```
git add internal/game/command/commands.go internal/game/command/summon_item.go internal/game/command/summon_item_test.go
git commit -m "feat: add summon_item command constant and game-layer handler"
```

---

### Task 2: Proto message (CMD-4)

**Files:**
- Modify: `api/proto/game/v1/game.proto`

**Step 1: Add proto message and oneof field**

In `api/proto/game/v1/game.proto`, add to the `ClientMessage` oneof (currently last field is 42 — `ClassFeaturesRequest class_features = 42`):

```protobuf
SummonItemRequest summon_item = 43;
```

Add the message definition (near other admin command messages):

```protobuf
message SummonItemRequest {
  string item_id = 1;
  int32 quantity = 2;
}
```

**Step 2: Regenerate proto**

```
cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -5
```

Expected: No errors. Generated files updated in `api/proto/game/v1/`.

**Step 3: Verify build**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: No errors.

**Step 4: Commit**

```
git add api/proto/game/v1/
git commit -m "feat: add SummonItemRequest proto message"
```

---

### Task 3: Bridge handler (CMD-5)

**Files:**
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Test: `internal/frontend/handlers/bridge_handlers_test.go` (the existing `TestAllCommandHandlersAreWired` test must pass)

**Step 1: Add bridge handler function**

In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:

```go
command.HandlerSummonItem: bridgeSummonItem,
```

Add the function (near other admin bridge functions):

```go
func bridgeSummonItem(bctx *bridgeContext) (bridgeResult, error) {
	parsed := command.HandleSummonItem(strings.Join(bctx.parsed.Args, " "))
	if strings.HasPrefix(parsed, "Usage:") {
		_ = bctx.conn.WriteLine(parsed)
		_ = bctx.conn.WritePrompt(bctx.promptFn())
		return bridgeResult{done: true}, nil
	}
	parts := strings.Fields(parsed)
	itemID := parts[0]
	qty, _ := strconv.Atoi(parts[1]) // safe: HandleSummonItem already validated
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload: &gamev1.ClientMessage_SummonItem{SummonItem: &gamev1.SummonItemRequest{
			ItemId:   itemID,
			Quantity: int32(qty),
		}},
	}}, nil
}
```

Make sure `"strconv"` and `"strings"` are in the imports (they likely already are).

**Step 2: Run the wiring test**

```
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v 2>&1
```

Expected: PASS.

**Step 3: Run full handler tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -timeout 120s 2>&1 | tail -10
```

Expected: `ok github.com/cory-johannsen/mud/internal/frontend/handlers`

**Step 4: Commit**

```
git add internal/frontend/handlers/bridge_handlers.go
git commit -m "feat: add bridgeSummonItem bridge handler"
```

---

### Task 4: gRPC handler with TDD (CMD-6, CMD-7)

**Files:**
- Create: `internal/gameserver/summon_item_handler_test.go`
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Write the failing tests**

Create `internal/gameserver/summon_item_handler_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/api/proto/game/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// testServiceWithFloor creates a GameServiceServer with a real FloorManager and Registry.
// Returns the server and a helper to add a player session.
func testServiceWithFloor(t *testing.T) (*GameServiceServer, *inventory.FloorManager, *inventory.Registry) {
	t.Helper()
	floorMgr := inventory.NewFloorManager()
	reg := inventory.NewRegistry()
	// Register a test item
	_ = reg.RegisterItem(&inventory.ItemDef{
		ID:   "test_pistol",
		Name: "Test Pistol",
		Kind: "weapon",
	})
	svc := newTestGameServiceServer(t,
		withFloorMgr(floorMgr),
		withInvRegistry(reg),
	)
	return svc, floorMgr, reg
}

func addEditorSession(t *testing.T, svc *GameServiceServer, uid, roomID string) {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      roomID,
		Role:        "editor",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
	})
	require.NoError(t, err)
}

func addPlayerSession(t *testing.T, svc *GameServiceServer, uid, roomID string) {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 2,
		RoomID:      roomID,
		Role:        "player",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
	})
	require.NoError(t, err)
}

func TestHandleSummonItem_EditorSuccess(t *testing.T) {
	svc, floorMgr, _ := testServiceWithFloor(t)
	addEditorSession(t, svc, "u1", "room_a")

	resp, err := svc.handleSummonItem("u1", &gamev1.SummonItemRequest{
		ItemId:   "test_pistol",
		Quantity: 2,
	})
	require.NoError(t, err)
	msg := resp.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Summoned 2x Test Pistol")

	items := floorMgr.ItemsInRoom("room_a")
	require.Len(t, items, 1)
	assert.Equal(t, "test_pistol", items[0].ItemDefID)
	assert.Equal(t, 2, items[0].Quantity)
}

func TestHandleSummonItem_AdminSuccess(t *testing.T) {
	svc, floorMgr, _ := testServiceWithFloor(t)
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "admin1", Username: "admin1", CharName: "Admin", CharacterID: 3,
		RoomID: "room_b", Role: "admin", CurrentHP: 10, MaxHP: 10, Abilities: character.AbilityScores{},
	})
	require.NoError(t, err)

	resp, err := svc.handleSummonItem("admin1", &gamev1.SummonItemRequest{
		ItemId:   "test_pistol",
		Quantity: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, resp.GetMessage().Content, "Summoned 1x Test Pistol")
	items := floorMgr.ItemsInRoom("room_b")
	require.Len(t, items, 1)
}

func TestHandleSummonItem_PlayerDenied(t *testing.T) {
	svc, floorMgr, _ := testServiceWithFloor(t)
	addPlayerSession(t, svc, "p1", "room_a")

	resp, err := svc.handleSummonItem("p1", &gamev1.SummonItemRequest{
		ItemId:   "test_pistol",
		Quantity: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, resp.GetMessage().Content, "permission denied")
	assert.Empty(t, floorMgr.ItemsInRoom("room_a"))
}

func TestHandleSummonItem_UnknownItem(t *testing.T) {
	svc, floorMgr, _ := testServiceWithFloor(t)
	addEditorSession(t, svc, "e1", "room_a")

	resp, err := svc.handleSummonItem("e1", &gamev1.SummonItemRequest{
		ItemId:   "nonexistent_item",
		Quantity: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, resp.GetMessage().Content, "unknown item")
	assert.Empty(t, floorMgr.ItemsInRoom("room_a"))
}

func TestHandleSummonItem_SessionNotFound(t *testing.T) {
	svc, _, _ := testServiceWithFloor(t)

	resp, err := svc.handleSummonItem("nobody", &gamev1.SummonItemRequest{
		ItemId:   "test_pistol",
		Quantity: 1,
	})
	require.NoError(t, err)
	assert.Contains(t, resp.GetMessage().Content, "player not found")
}

func TestHandleSummonItem_DefaultQuantityOne(t *testing.T) {
	svc, floorMgr, _ := testServiceWithFloor(t)
	addEditorSession(t, svc, "e2", "room_c")

	resp, err := svc.handleSummonItem("e2", &gamev1.SummonItemRequest{
		ItemId:   "test_pistol",
		Quantity: 0, // zero means default to 1
	})
	require.NoError(t, err)
	assert.Contains(t, resp.GetMessage().Content, "Summoned 1x")
	items := floorMgr.ItemsInRoom("room_c")
	require.Len(t, items, 1)
	assert.Equal(t, 1, items[0].Quantity)
}

func TestPropertySummonItem_EditorAlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		qty := rapid.Int32Range(1, 100).Draw(rt, "qty")
		svc, floorMgr, _ := testServiceWithFloor(t)
		addEditorSession(t, svc, "e", "r1")

		resp, err := svc.handleSummonItem("e", &gamev1.SummonItemRequest{
			ItemId:   "test_pistol",
			Quantity: qty,
		})
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if resp.GetMessage() == nil || !strings.Contains(resp.GetMessage().Content, "Summoned") {
			rt.Fatalf("expected success message, got: %v", resp)
		}
		items := floorMgr.ItemsInRoom("r1")
		if len(items) != 1 || items[0].Quantity != int(qty) {
			rt.Fatalf("expected 1 item with qty %d, got %v", qty, items)
		}
	})
}
```

**Step 2: Run tests to verify they fail**

```
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleSummonItem -v 2>&1 | head -20
```

Expected: FAIL — `handleSummonItem` undefined.

**Step 3: Implement the gRPC handler**

In `internal/gameserver/grpc_service.go`, add to the dispatch switch (find the block of `case *gamev1.ClientMessage_Xxx:` lines around line 741-823):

```go
case *gamev1.ClientMessage_SummonItem:
    return s.handleSummonItem(uid, p.SummonItem)
```

Add the handler function (near other admin handlers):

```go
// handleSummonItem places an item instance on the floor of the caller's current room.
// Precondition: uid identifies an active session; req is non-nil.
// Postcondition: on success, one ItemInstance is added to the room floor and a success
// message is returned; on failure, an error message is returned with no side effects.
func (s *GameServiceServer) handleSummonItem(uid string, req *gamev1.SummonItemRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    if sess.Role != "editor" && sess.Role != "admin" {
        return messageEvent("permission denied: editor role required"), nil
    }
    if s.invRegistry == nil {
        return messageEvent("item registry unavailable"), nil
    }
    def, ok := s.invRegistry.Item(req.ItemId)
    if !ok {
        return messageEvent(fmt.Sprintf("unknown item: %q", req.ItemId)), nil
    }
    qty := int(req.Quantity)
    if qty < 1 {
        qty = 1
    }
    inst := inventory.ItemInstance{
        InstanceID: uuid.New().String(),
        ItemDefID:  req.ItemId,
        Quantity:   qty,
    }
    if s.floorMgr != nil {
        s.floorMgr.Drop(sess.RoomID, inst)
    }
    return messageEvent(fmt.Sprintf("Summoned %dx %s to the room.", qty, def.Name)), nil
}
```

Ensure `"github.com/google/uuid"` is in the imports (check existing imports; it may already be present).

**Step 4: Run tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleSummonItem|TestPropertySummonItem" -v -timeout 60s 2>&1 | tail -20
```

Expected: All PASS.

**Step 5: Run full gameserver tests**

```
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -timeout 120s 2>&1 | tail -10
```

Expected: `ok github.com/cory-johannsen/mud/internal/gameserver`

**Step 6: Commit**

```
git add internal/gameserver/summon_item_handler_test.go internal/gameserver/grpc_service.go
git commit -m "feat: implement handleSummonItem gRPC handler with TDD"
```

---

### Task 5: Full build, test, FEATURES.md update, deploy

**Step 1: Full build**

```
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: No errors.

**Step 2: Full non-DB test suite**

```
cd /home/cjohannsen/src/mud && go test $(go list ./... | grep -v storage/postgres) -timeout 120s 2>&1 | tail -20
```

Expected: All packages pass.

**Step 3: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, change:

```
- [ ] Admin command `summon_item` which take an item ID as a parameter.  An instance of the specified item must be added to the room.
```

to:

```
- [x] Admin command `summon_item` which take an item ID as a parameter.  An instance of the specified item must be added to the room.
```

**Step 4: Commit**

```
git add docs/requirements/FEATURES.md
git commit -m "docs: mark summon_item complete in FEATURES.md"
```

**Step 5: Deploy**

```
cd /home/cjohannsen/src/mud && make k8s-redeploy 2>&1 | tail -20
```

**Step 6: Verify pods**

```
kubectl get pods -l app=mud-gameserver 2>&1
```

Expected: All pods Running or Completed.
