# Sense Motive (`motive`) Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `motive <target>` command that uses awareness vs NPC Deception DC to reveal an NPC's combat state; works in and out of combat (out-of-combat behavior stubbed for later).

**Architecture:** Follow the full CMD-1 through CMD-7 pattern. Add `Deception int` to `npc.Template` and `npc.Instance`; add proto message; wire bridge handler; implement `handleMotive` in grpc_service.go. In-combat: costs 1 AP, rolls `d20 + skillRankBonus(awareness)` vs `10 + inst.Deception`, success reveals NPC HP tier. Out-of-combat: returns a stub message for later extension.

**Tech Stack:** Go, protobuf, pgregory.net/rapid (property tests)

---

## Chunk 1: NPC Deception field + command/proto/bridge wiring

### Task 1: Add Deception field to NPC Template and Instance

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`

**Background:**

`npc.Template` (template.go ~line 39) has `Perception int \`yaml:"perception"\`` but does **not** have a Stealth field. Add `Deception int \`yaml:"deception"\`` directly after `Perception int \`yaml:"perception"\``.

`npc.Instance` (instance.go ~line 39) has `Perception int` (no yaml tag) and `Stealth int \`yaml:"stealth"\`` (with yaml tag). For consistency with the `Stealth` declaration on Instance, add `Deception` **with** a yaml tag. The `NewInstanceWithResolver` function (~line 120) copies `Perception: tmpl.Perception` into the Instance return literal. Add `Deception: tmpl.Deception,` the same way. Note: `Stealth` is NOT copied from template (no `Stealth: tmpl.Stealth` in the literal) — but `Deception` must be, so add it explicitly.

No existing NPC YAMLs need updating — the field defaults to 0 (untrained).

- [ ] **Step 1: Write failing test for Deception field on Instance**

In `internal/game/npc/instance_test.go` (check if it exists; if not, create it) — or add to the existing template/instance tests. Add:

```go
func TestNewInstance_DeceptionCopiedFromTemplate(t *testing.T) {
    tmpl := &Template{
        ID: "test-npc", Name: "Test", Level: 1, MaxHP: 10, AC: 10,
        Deception: 7,
    }
    inst := NewInstance("inst-1", tmpl, "room_a")
    if inst.Deception != 7 {
        t.Errorf("expected Deception=7, got %d", inst.Deception)
    }
}

func TestNewInstance_DeceptionDefaultsToZero(t *testing.T) {
    tmpl := &Template{
        ID: "test-npc2", Name: "Test2", Level: 1, MaxHP: 10, AC: 10,
    }
    inst := NewInstance("inst-2", tmpl, "room_a")
    if inst.Deception != 0 {
        t.Errorf("expected Deception=0, got %d", inst.Deception)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestNewInstance_Deception" -v 2>&1 | tail -10
```

Expected: compile error — `Deception` field does not exist on Template or Instance.

- [ ] **Step 3: Add Deception to Template**

In `internal/game/npc/template.go`, add after `Perception int \`yaml:"perception"\``:

```go
// Deception is the NPC's deception skill modifier, used as the DC for the motive command.
// Zero means untrained. Loaded from YAML field "deception".
Deception int `yaml:"deception"`
```

- [ ] **Step 4: Add Deception to Instance and copy in NewInstanceWithResolver**

In `internal/game/npc/instance.go`, add to the Instance struct (after `Stealth int \`yaml:"stealth"\``):
```go
// Deception is the instance's deception skill modifier.
Deception int `yaml:"deception"`
```

In `NewInstanceWithResolver`, inside the `return &Instance{...}` literal, add:
```go
Deception: tmpl.Deception,
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... -run "TestNewInstance_Deception" -v 2>&1
```

Expected: PASS.

- [ ] **Step 6: Run full npc package tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/npc/... 2>&1
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/npc/template.go internal/game/npc/instance.go internal/game/npc/
git commit -m "feat(npc): add Deception field to Template and Instance for motive command"
```

---

### Task 2: Add command handler (CMD-1, CMD-2, CMD-3)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/motive.go`
- Create: `internal/game/command/motive_test.go`

**Background:**

Handler constants are at the top of commands.go. `HandlerSeek = "seek"` is at line 89. Add `HandlerMotive = "motive"` after it.

`BuiltinCommands()` ends around line 198. The last combat command entry is:
```go
{Name: "seek", Handler: HandlerSeek, Help: "...", Category: CategoryCombat},
```
Add a new entry after seek for motive.

The command handler pattern (from feint.go):
```go
type FeintRequest struct { Target string }
func HandleFeint(args []string) (*FeintRequest, error) { ... }
```

- [ ] **Step 1: Write failing tests**

Create `internal/game/command/motive_test.go`:

```go
package command

import (
	"testing"
	"pgregory.net/rapid"
)

func TestHandleMotive_WithTarget(t *testing.T) {
	req, err := HandleMotive([]string{"bandit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil MotiveRequest")
	}
	if req.Target != "bandit" {
		t.Errorf("expected Target=%q, got %q", "bandit", req.Target)
	}
}

func TestHandleMotive_NoArgs(t *testing.T) {
	req, err := HandleMotive(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil MotiveRequest")
	}
	if req.Target != "" {
		t.Errorf("expected empty Target, got %q", req.Target)
	}
}

func TestHandleMotive_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleMotive(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil MotiveRequest")
		}
		if len(args) >= 1 {
			if req.Target != args[0] {
				rt.Fatalf("expected Target=%q, got %q", args[0], req.Target)
			}
		} else {
			if req.Target != "" {
				rt.Fatalf("expected empty Target when no args, got %q", req.Target)
			}
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/command/... -run "TestHandleMotive" -v 2>&1 | tail -10
```

Expected: compile error — `HandleMotive` does not exist.

- [ ] **Step 3: Create motive.go**

Create `internal/game/command/motive.go`:

```go
package command

// MotiveRequest is the parsed form of the motive command.
//
// Precondition: Target may be empty (handler will return error in that case).
type MotiveRequest struct {
	Target string
}

// HandleMotive parses the arguments for the "motive" command.
//
// Precondition: args is the slice of words following "motive" (may be empty).
// Postcondition: Returns a non-nil *MotiveRequest and nil error always.
func HandleMotive(args []string) (*MotiveRequest, error) {
	req := &MotiveRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}
```

- [ ] **Step 4: Add HandlerMotive constant and BuiltinCommands entry**

In `internal/game/command/commands.go`:

Add constant after `HandlerSeek = "seek"` (line ~89):
```go
HandlerMotive = "motive"
```

Add entry in `BuiltinCommands()` after the seek entry:
```go
{Name: "motive", Aliases: []string{"mot"}, Help: "Read an NPC's intentions (awareness vs Deception DC; success reveals HP tier in combat). Costs 1 AP in combat.", Category: CategoryCombat, Handler: HandlerMotive},
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/command/... -run "TestHandleMotive" -v 2>&1
```

Expected: all 3 PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/command/motive.go internal/game/command/motive_test.go internal/game/command/commands.go
git commit -m "feat(command): add motive command handler (CMD-1, CMD-2, CMD-3)"
```

---

### Task 3: Proto message + bridge handler (CMD-4, CMD-5)

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`

**Background:**

In `game.proto`, the `ClientMessage` oneof currently ends at field 68 (`SwimRequest swim = 68;`). Add `MotiveRequest motive = 69;`.

Add `MotiveRequest` message definition — follow the exact pattern of `FeintRequest`:
```protobuf
message FeintRequest {
  string target = 1;
}
```

`make proto` regenerates `internal/gameserver/gamev1/game.pb.go`.

In `bridge_handlers.go`:
- The map `bridgeHandlerMap` (line ~42) maps `command.Handler*` constants to bridge functions.
- Add `command.HandlerMotive: bridgeMotive,` after the seek entry.
- Add `bridgeMotive` function after `bridgeSeek` (~line 1035).

The `TestAllCommandHandlersAreWired` test (in `grpc_service_commands_test.go`) validates that every `HandlerXxx` constant in `commands.go` has a corresponding entry in `bridgeHandlerMap`. It will fail until `HandlerMotive` is wired.

- [ ] **Step 1: Add MotiveRequest to game.proto**

In `api/proto/game/v1/game.proto`:

After `SwimRequest swim = 68;` in the `ClientMessage` oneof, add:
```protobuf
    MotiveRequest motive = 69;
```

Add the message definition after `message SwimRequest {}` (line ~800 in the proto file — the last message definition):
```protobuf
// MotiveRequest asks the server to read an NPC's intentions (awareness vs Deception DC).
message MotiveRequest {
  string target = 1;
}
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
make proto 2>&1
```

Expected: regenerates `internal/gameserver/gamev1/game.pb.go` with no errors.

- [ ] **Step 3: Add bridgeMotive and register it**

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap` after the seek entry:
```go
command.HandlerMotive: bridgeMotive,
```

Add function after `bridgeSeek`:
```go
// bridgeMotive builds a MotiveRequest with the target name.
//
// Precondition: bctx must be non-nil with a valid reqID and non-empty RawArgs.
// Postcondition: returns a non-nil msg containing a MotiveRequest when RawArgs is non-empty;
// otherwise returns done=true with a usage error event.
func bridgeMotive(bctx *bridgeContext) (bridgeResult, error) {
	if bctx.parsed.RawArgs == "" {
		return writeErrorPrompt(bctx, "Usage: motive <target>")
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Motive{Motive: &gamev1.MotiveRequest{Target: bctx.parsed.RawArgs}},
	}}, nil
}
```

- [ ] **Step 4: Verify TestAllCommandHandlersAreWired passes**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/... -run "TestAllCommandHandlersAreWired" -v 2>&1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(proto,bridge): add MotiveRequest and bridgeMotive (CMD-4, CMD-5)"
```

---

## Chunk 2: Server handler and tests (CMD-6, CMD-7)

### Task 4: Implement handleMotive + grpc tests (CMD-6, CMD-7)

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_motive_test.go`

**Background:**

`handleMotive` follows the exact pattern of `handleFeint` (grpc_service.go line 4530):
1. Get player session — return `fmt.Errorf` if not found
2. **Two branches based on combat status:**
   - **In combat** (`sess.Status == statusInCombat`): spend 1 AP, roll, compare vs DC, return result
   - **Out of combat**: return stub message (no AP cost)
3. Validate target is non-empty (error event if empty)
4. Find NPC in room via `s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())`
5. Combat path: `s.combatH.SpendAP(uid, 1)`, roll `1d20`, `skillRankBonus(sess.Skills["awareness"])`, DC = `10 + inst.Deception`
6. Success: reveal HP tier message. Failure: generic message.

**HP tier thresholds (based on CurrentHP / MaxHP ratio):**
- > 75% → `"unharmed"` (or "at full strength")
- 50–75% → `"lightly wounded"`
- 25–50% → `"bloodied"`
- ≤ 25% → `"badly wounded"`

**Success message format:**
`"Motive (deception DC 12): rolled 15+2=17 — success! Ganger appears bloodied — masking real injuries."`

**Failure message format:**
`"Motive (deception DC 12): rolled 8+2=10 — failure. You can't get a read on Ganger."`

**Out-of-combat stub message:**
`"There's no one here worth reading."` (returned when `sess.Status != statusInCombat`)

**Dispatch wiring:** In the `dispatch` function (~line 1237), add before `default`:
```go
case *gamev1.ClientMessage_Motive:
    return s.handleMotive(uid, p.Motive)
```

**Test service helper:** Use `newDisarmSvcWithCombat` as the pattern (from `grpc_service_disarm_test.go`). The motive test helper is identical except the function name. Re-use `makeTestConditionRegistry()` (no grabbed restriction needed for motive).

**Test cases required:**
1. `TestHandleMotive_NotInCombat` — out-of-combat returns stub message
2. `TestHandleMotive_NoTarget` — in combat, empty target returns error
3. `TestHandleMotive_TargetNotFound` — in combat, NPC not in room
4. `TestHandleMotive_RollFailure` — in combat, roll < DC, returns failure message
5. `TestHandleMotive_RollSuccess_HPTier` — in combat, roll >= DC, returns HP tier in message
6. `TestProperty_Motive_SuccessAlwaysRevealsHPTier` — property: when roll >= dc, message always contains one of the tier strings

**Deterministic dice for tests:** Use `dice.NewDeterministicSource` and `dice.NewRoller` (same pattern as flee/disarm tests).

For `TestHandleMotive_RollFailure`: NPC Deception=0, DC=10. Dice sequence: `[5, 3, 1]` — first two are initiative rolls consumed by `Attack()`, third is the motive roll (1). With awareness bonus=0: total=1 < DC=10 → failure.

For `TestHandleMotive_RollSuccess_HPTier`: NPC Deception=0, DC=10. Dice sequence: `[5, 3, 20]` — roll=20 >= DC=10 → success. NPC at full HP (MaxHP=20, CurrentHP=20) → "unharmed".

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_motive_test.go`:

```go
package gameserver

import (
	"testing"

	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newMotiveSvc builds a minimal GameServiceServer for handleMotive tests without combat.
func newMotiveSvc(t *testing.T, roller *dice.Roller, npcMgr *npc.Manager, combatHandler *CombatHandler) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	if npcMgr == nil {
		npcMgr = npc.NewManager()
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr
}

// newMotiveSvcWithCombat builds a full GameServiceServer with combat handler for handleMotive tests.
func newMotiveSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleMotive_NotInCombat verifies that motive outside combat returns a stub message.
//
// Precondition: sess.Status != statusInCombat.
// Postcondition: message event returned; no error.
func TestHandleMotive_NotInCombat(t *testing.T) {
	svc, sessMgr := newMotiveSvc(t, nil, nil, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_nc", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	evt, err := svc.handleMotive("u_mot_nc", &gamev1.MotiveRequest{Target: "bandit"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "no one")
}

// TestHandleMotive_NoTarget verifies that motive in combat with no target returns an error event.
//
// Precondition: sess.Status == statusInCombat; req.Target == "".
// Postcondition: error event with "Usage".
func TestHandleMotive_NoTarget(t *testing.T) {
	src := dice.NewDeterministicSource([]int{5, 3})
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_nt", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{ID: "ganger-mot-nt-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	_, err = svc.combatH.Attack("u_mot_nt", inst.ID)
	require.NoError(t, err)

	evt, err := svc.handleMotive("u_mot_nt", &gamev1.MotiveRequest{Target: ""})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "Usage")
}

// TestHandleMotive_TargetNotFound verifies that motive with an unknown target returns an error event.
//
// Precondition: sess.Status == statusInCombat; target NPC not in room.
// Postcondition: error event with target name.
func TestHandleMotive_TargetNotFound(t *testing.T) {
	src := dice.NewDeterministicSource([]int{5, 3})
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_nf", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{ID: "ganger-mot-nf-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	_, err = svc.combatH.Attack("u_mot_nf", inst.ID)
	require.NoError(t, err)

	evt, err := svc.handleMotive("u_mot_nf", &gamev1.MotiveRequest{Target: "ghost"})
	require.NoError(t, err)
	require.NotNil(t, evt)
	errEvt := evt.GetError()
	require.NotNil(t, errEvt)
	assert.Contains(t, errEvt.Message, "ghost")
}

// TestHandleMotive_RollFailure verifies that a roll below DC returns a failure message.
//
// Precondition: sess.Status == statusInCombat; dice roll produces total < DC.
// Postcondition: message event containing "failure" or "can't get a read".
func TestHandleMotive_RollFailure(t *testing.T) {
	// Dice: [5=player_init, 3=npc_init, 1=motive_roll]
	src := dice.NewDeterministicSource([]int{5, 3, 1})
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_fail", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	// NPC Deception=0: DC = 10+0 = 10. Roll=1+0=1 < 10 → failure.
	tmpl := &npc.Template{ID: "ganger-mot-fail-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10, Deception: 0}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	_, err = svc.combatH.Attack("u_mot_fail", inst.ID)
	require.NoError(t, err)

	evt, err := svc.handleMotive("u_mot_fail", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "failure")
}

// TestHandleMotive_RollSuccess_HPTier verifies that a roll >= DC returns a message with the NPC's HP tier.
//
// Precondition: sess.Status == statusInCombat; dice roll produces total >= DC; NPC at full HP.
// Postcondition: message event containing "unharmed" (NPC at 100% HP).
func TestHandleMotive_RollSuccess_HPTier(t *testing.T) {
	// Dice: [5=player_init, 3=npc_init, 20=motive_roll]
	src := dice.NewDeterministicSource([]int{5, 3, 20})
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_mot_succ", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	// NPC Deception=0: DC=10. Roll=20 >= 10 → success. NPC at full HP → "unharmed".
	tmpl := &npc.Template{ID: "ganger-mot-succ-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10, Deception: 0}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	_, err = svc.combatH.Attack("u_mot_succ", inst.ID)
	require.NoError(t, err)

	evt, err := svc.handleMotive("u_mot_succ", &gamev1.MotiveRequest{Target: "Ganger"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "unharmed")
}

// TestProperty_Motive_SuccessAlwaysRevealsHPTier verifies that when roll >= DC,
// the message always contains one of the known HP tier strings.
func TestProperty_Motive_SuccessAlwaysRevealsHPTier(t *testing.T) {
	hpTiers := []string{"unharmed", "lightly wounded", "bloodied", "badly wounded"}

	rapid.Check(t, func(rt *rapid.T) {
		// Use a roll of 20 to guarantee success regardless of Deception (max DC = 10+20=30, roll+bonus always at least 20 with deterministic 20).
		// Initiative rolls come first (2 rolls), then the motive roll.
		deception := rapid.IntRange(0, 9).Draw(rt, "deception") // DC = 10+deception; 20 > 10+9=19, so success guaranteed
		npcUID := "ganger-prop-" + rapid.StringMatching(`[a-z]{4}`).Draw(rt, "suffix")
		playerUID := "player-prop-" + rapid.StringMatching(`[a-z]{4}`).Draw(rt, "psuffix")

		src := dice.NewDeterministicSource([]int{5, 3, 20})
		roller := dice.NewRoller(src)
		svc, sessMgr, npcMgr, _ := newMotiveSvcWithCombat(t, roller)

		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: playerUID, Username: "Prop", CharName: "Prop", RoomID: "room_a", Role: "player",
		})
		require.NoError(rt, err)

		tmpl := &npc.Template{
			ID: npcUID, Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10,
			Deception: deception,
		}
		inst, err := npcMgr.Spawn(tmpl, "room_a")
		require.NoError(rt, err)
		_, err = svc.combatH.Attack(playerUID, inst.ID)
		require.NoError(rt, err)

		evt, err := svc.handleMotive(playerUID, &gamev1.MotiveRequest{Target: "Ganger"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		msg := evt.GetMessage()
		require.NotNil(rt, msg)

		found := false
		for _, tier := range hpTiers {
			if strings.Contains(msg.Content, tier) {
				found = true
				break
			}
		}
		assert.True(rt, found, "expected HP tier in message, got: %q", msg.Content)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleMotive|TestProperty_Motive" -v 2>&1 | tail -15
```

Expected: compile error — `handleMotive` does not exist; `gamev1.MotiveRequest` does not exist (if proto not yet done, do Task 3 first then return here).

**Note:** Task 3 (proto/bridge) must be complete before Task 4 tests will compile. Execute tasks in order: 1 → 2 → 3 → 4.

- [ ] **Step 3: Implement handleMotive in grpc_service.go**

Add after `handleSeek` (~line 5311). Insert the full function:

```go
// handleMotive performs an awareness skill check against the target NPC's Deception DC (10 + Deception).
// In combat: costs 1 AP; on success reveals the NPC's HP tier.
// Out of combat: returns a stub message for future non-combat NPC extension.
//
// Precondition: uid must be a valid player session; req.Target must name an NPC in the room when in combat.
// Postcondition: In combat on success, returns a message with the NPC's HP tier string.
func (s *GameServiceServer) handleMotive(uid string, req *gamev1.MotiveRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Out-of-combat stub: behavior added when non-combat NPCs are implemented.
	if sess.Status != statusInCombat {
		return messageEvent("There's no one here worth reading."), nil
	}

	if req.GetTarget() == "" {
		return errorEvent("Usage: motive <target>"), nil
	}

	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetTarget())
	if inst == nil {
		return errorEvent(fmt.Sprintf("Target %q not found in current room.", req.GetTarget())), nil
	}

	if s.combatH == nil {
		return errorEvent("Combat handler unavailable."), nil
	}
	if err := s.combatH.SpendAP(uid, 1); err != nil {
		return errorEvent(err.Error()), nil
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleMotive: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["awareness"])
	total := roll + bonus
	dc := 10 + inst.Deception

	detail := fmt.Sprintf("Motive (deception DC %d): rolled %d+%d=%d", dc, roll, bonus, total)
	if total < dc {
		return messageEvent(detail + fmt.Sprintf(" — failure. You can't get a read on %s.", inst.Name())), nil
	}

	tier := npcHPTier(inst.CurrentHP, inst.MaxHP)
	return messageEvent(detail + fmt.Sprintf(" — success! %s appears %s.", inst.Name(), tier)), nil
}

// npcHPTier returns a human-readable HP tier string for the given current/max HP values.
//
// Precondition: maxHP > 0.
// Postcondition: Returns one of "unharmed", "lightly wounded", "bloodied", "badly wounded".
func npcHPTier(currentHP, maxHP int) string {
	if maxHP <= 0 {
		return "badly wounded"
	}
	ratio := float64(currentHP) / float64(maxHP)
	switch {
	case ratio > 0.75:
		return "unharmed"
	case ratio > 0.50:
		return "lightly wounded"
	case ratio > 0.25:
		return "bloodied"
	default:
		return "badly wounded"
	}
}
```

- [ ] **Step 4: Wire dispatch case in grpc_service.go**

In the `dispatch` function (~line 1237), add before `default`:

```go
case *gamev1.ClientMessage_Motive:
    return s.handleMotive(uid, p.Motive)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleMotive|TestProperty_Motive" -v 2>&1
```

Expected: all 6 PASS.

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -25
```

Expected: all packages PASS.

- [ ] **Step 7: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, change:
```
  - [ ] Sense Motive command — implement `motive <target>` (Perception vs target Deception skill; on success reveals whether the NPC is bluffing, holding back an action, or concealing intent)
```
to:
```
  - [x] Sense Motive command — implement `motive <target>` (awareness vs Deception DC; in combat costs 1 AP and reveals NPC HP tier; out-of-combat behavior stubbed for non-combat NPC extension)
```

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_motive_test.go \
        docs/requirements/FEATURES.md
git commit -m "feat(gameserver): implement handleMotive — sense motive command (CMD-6, CMD-7)"
```
