# Terrain Types Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Climb and Swim commands with full 4-tier PF2E outcomes, including the `submerged` condition and per-round drowning damage.

**Architecture:** Both commands follow the grapple/trip CMD-1–7 pipeline: constants → BuiltinCommands → command handler → proto → bridge → gRPC dispatch. Terrain is read from `Room.Properties`. Climb uses `up`/`down` exits; Swim uses water exits or surfaces when submerged. Drowning damage is applied at round-start (combat) and per-dispatch (out-of-combat).

**Tech Stack:** Go, pgregory.net/rapid (property-based testing), protobuf, mise toolchain.

**Spec:** `docs/superpowers/specs/2026-03-11-terrain-types-design.md`

---

## Chunk 1: Foundation — submerged condition + proto + command registration

### Task 1: Create `submerged` condition YAML

**Files:**
- Create: `content/conditions/submerged.yaml`

Reference: `content/conditions/prone.yaml`

- [ ] **Step 1: Create the file**

```yaml
id: submerged
name: Submerged
description: |
  You are pulled under the water. You cannot attack or reload.
  Swim or Escape to surface.
duration_type: permanent
max_stacks: 1
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
damage_bonus: 0
reflex_bonus: 0
stealth_bonus: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 2: Write a load test in `internal/game/condition/`**

Find the existing condition load tests. Add to an appropriate test file (or create `submerged_test.go`):

```go
func TestSubmergedConditionLoads(t *testing.T) {
    reg := NewRegistry()
    err := reg.LoadDirectory("../../../content/conditions")
    require.NoError(t, err)
    def, ok := reg.Get("submerged")
    require.True(t, ok, "submerged condition must be registered")
    assert.Equal(t, "submerged", def.ID)
    assert.Equal(t, 1, def.MaxStacks)
    assert.Equal(t, "permanent", def.DurationType)
}
```

- [ ] **Step 3: Run and confirm test fails**

```bash
mise run go test ./internal/game/condition/... -run TestSubmergedConditionLoads -v
```

Expected: FAIL — `submerged` not found (file doesn't exist yet if you wrote the test first; adjust order as needed).

- [ ] **Step 4: Run again with file in place — confirm pass**

```bash
mise run go test ./internal/game/condition/... -run TestSubmergedConditionLoads -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add content/conditions/submerged.yaml internal/game/condition/
git commit -m "feat(condition): add submerged condition YAML and load test"
```

---

### Task 2: Proto — ClimbRequest and SwimRequest

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/gamev1/game.pb.go` (generated — do not edit by hand)

- [ ] **Step 1: Add proto messages**

Open `api/proto/game/v1/game.proto`. Find the `ClientMessage` oneof (currently ends with field 66 `seek`). Add:

```protobuf
// ClimbRequest asks the server to attempt climbing a climbable surface.
message ClimbRequest {}

// SwimRequest asks the server to attempt swimming or surfacing.
message SwimRequest {}
```

Then in the `ClientMessage` oneof, add:

```protobuf
ClimbRequest climb = 67;
SwimRequest swim = 68;
```

- [ ] **Step 2: Regenerate**

```bash
make proto
```

Expected: No errors; `internal/gameserver/gamev1/game.pb.go` updated.

- [ ] **Step 3: Confirm build**

```bash
mise run go build ./...
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): add ClimbRequest and SwimRequest messages (fields 67, 68)"
```

---

### Task 3: Command constants and BuiltinCommands entries

**Files:**
- Modify: `internal/game/command/commands.go`

- [ ] **Step 1: Add Handler constants**

Find the block of `Handler*` constants (ends with `HandlerSeek`). Add:

```go
HandlerClimb = "climb"
HandlerSwim  = "swim"
```

- [ ] **Step 2: Add BuiltinCommands entries**

Find `BuiltinCommands()`. Add after the `seek` entry:

```go
{Name: "climb", Aliases: []string{"cl"}, Help: "Climb a climbable surface (athletics vs DC; costs 2 AP in combat).", Category: CategoryMovement, Handler: HandlerClimb},
{Name: "swim", Aliases: []string{"sw"}, Help: "Swim through water or surface when submerged (athletics vs DC; costs 2 AP in combat).", Category: CategoryMovement, Handler: HandlerSwim},
```

> Note: Use `CategoryMovement` if it exists; otherwise check what category `seek` uses and match it.

- [ ] **Step 3: Run existing command tests**

```bash
mise run go test ./internal/game/command/... -v
```

Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/game/command/commands.go
git commit -m "feat(command): register HandlerClimb and HandlerSwim constants and BuiltinCommands entries"
```

---

## Chunk 2: Climb command — parser, bridge, gRPC handler

### Task 4: Climb command parser and property-based tests

**Files:**
- Create: `internal/game/command/climb.go`
- Create: `internal/game/command/climb_test.go`

Reference: `internal/game/command/grapple.go` and `grapple_test.go`.

- [ ] **Step 1: Write failing property-based tests**

Create `internal/game/command/climb_test.go`:

```go
package command_test

import (
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleClimb_AlwaysReturnsRequest(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleClimb(args)
		assert.NoError(rt, err)
		assert.NotNil(rt, req)
	})
}

// TestClimbOutcomes verifies 4-tier outcome thresholds for the climb skill check.
// This tests the outcome logic directly (not the full gRPC handler).
// CritSuccess: total >= dc+10; Success: dc <= total < dc+10;
// Failure: dc-10 <= total < dc; CritFailure: total < dc-10.
func TestClimbOutcomes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dc := rapid.IntRange(5, 30).Draw(rt, "dc")
		roll := rapid.IntRange(1, 30).Draw(rt, "roll")
		// Verify OutcomeFor boundaries match expectations.
		// CritSuccess boundary.
		critSuccessRoll := dc + 10
		assert.Equal(rt, combat.CritSuccess, combat.OutcomeFor(critSuccessRoll, dc))
		// Success boundary.
		successRoll := dc
		assert.Equal(rt, combat.Success, combat.OutcomeFor(successRoll, dc))
		// Failure boundary.
		failureRoll := dc - 1
		assert.Equal(rt, combat.Failure, combat.OutcomeFor(failureRoll, dc))
		// CritFailure boundary.
		critFailRoll := dc - 10
		assert.Equal(rt, combat.CritFailure, combat.OutcomeFor(critFailRoll, dc))
		// Arbitrary roll is one of the four outcomes.
		o := combat.OutcomeFor(roll, dc)
		assert.True(rt, o == combat.CritSuccess || o == combat.Success || o == combat.Failure || o == combat.CritFailure)
		_ = dc
	})
}
```

> Note: `TestClimbOutcomes` imports `github.com/cory-johannsen/mud/internal/game/combat` — add to import block.

- [ ] **Step 2: Run and confirm fail**

```bash
mise run go test ./internal/game/command/... -run TestHandleClimb -v
```

Expected: compile error — `HandleClimb` undefined.

- [ ] **Step 3: Implement climb.go**

Create `internal/game/command/climb.go`:

```go
package command

// ClimbRequest is the parsed form of the climb command.
// Precondition: none — no arguments are required.
// Postcondition: Always returns a non-nil *ClimbRequest with nil error.
type ClimbRequest struct{}

// HandleClimb parses the arguments for the "climb" command.
// Precondition: args is the slice of words following "climb" (may be empty).
// Postcondition: Returns a non-nil *ClimbRequest and nil error always.
func HandleClimb(args []string) (*ClimbRequest, error) {
	return &ClimbRequest{}, nil
}
```

- [ ] **Step 4: Run and confirm pass**

```bash
mise run go test ./internal/game/command/... -run TestHandleClimb -v
```

Expected: PASS.

- [ ] **Step 5: Run TestClimbOutcomes**

```bash
mise run go test ./internal/game/command/... -run TestClimbOutcomes -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/game/command/climb.go internal/game/command/climb_test.go
git commit -m "feat(command): add HandleClimb parser with property-based outcome tests"
```

---

### Task 5: Climb bridge handler

**Files:**
- Modify: `internal/frontend/handlers/bridge_handlers.go`

Reference: `bridgeGrapple` pattern.

- [ ] **Step 1: Add bridgeClimb to bridgeHandlerMap**

Find the `bridgeHandlerMap` initializer. Add:

```go
command.HandlerClimb: bridgeClimb,
```

- [ ] **Step 2: Implement bridgeClimb**

Add the function (near other Athletics bridges):

```go
func bridgeClimb(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Climb{Climb: &gamev1.ClimbRequest{}},
	}}, nil
}
```

- [ ] **Step 3: Run wiring test**

```bash
mise run go test ./internal/frontend/... -run TestAllCommandHandlersAreWired -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(bridge): add bridgeClimb handler and register in bridgeHandlerMap"
```

---

### Task 6: handleClimb gRPC handler and integration tests

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_climb_test.go`

Reference: `handleGrapple` in `grpc_service.go` and `grpc_service_grapple_test.go`.

> **IMPORTANT:** Read `grpc_service_grapple_test.go` completely before writing any test. Use its server/session construction helpers exactly. Do NOT commit any test file with placeholder bodies or `t.Skip` — all tests must have real implementations before committing.

- [ ] **Step 1: Wire dispatch in grpc_service.go**

Find the `dispatch` type switch. Add:

```go
case *gamev1.ClientMessage_Climb:
    return s.handleClimb(uid, p.Climb)
```

- [ ] **Step 4: Implement handleClimb**

Add to `grpc_service.go` (after handleSeek or near other Athletics handlers):

```go
// handleClimb processes a ClimbRequest from the player.
//
// Precondition: uid is a valid connected player session.
// Postcondition: Player moves via vertical exit on success; falling damage applied on critical failure.
func (s *GameServiceServer) handleClimb(uid string, _ *gamev1.ClimbRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	room := s.worldMgr.Room(sess.RoomID)
	if room == nil {
		return messageEvent("Room not found."), nil
	}

	// Check climbable property.
	if room.Properties["climbable"] != "true" {
		return messageEvent("There is nothing to climb here."), nil
	}

	// Find vertical exit: prefer "up", fall back to "down".
	exit, found := room.ExitForDirection(world.Up)
	if !found {
		exit, found = room.ExitForDirection(world.Down)
	}
	if !found {
		return messageEvent("The climbable surface has no clear route up or down."), nil
	}

	// Spend AP if in combat.
	inCombat := sess.Status == statusInCombat
	if inCombat {
		if !s.combatH.SpendAP(uid, 2) {
			return messageEvent("Not enough action points to climb."), nil
		}
	}

	// Parse DC (default 15).
	dc := 15
	if dcStr, ok := room.Properties["climb_dc"]; ok {
		if parsed, err := strconv.Atoi(dcStr); err == nil {
			dc = parsed
		}
	}

	// Roll athletics check.
	// s.dice.RollExpr returns a result struct; call .Total() for the integer value.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		// Move the player to the target room.
		s.movePlayer(uid, exit.TargetRoom)
		return messageEvent(fmt.Sprintf(
			"You climb successfully (rolled %d+%d=%d vs DC %d). You arrive at %s.",
			roll, bonus, total, dc, s.worldMgr.Room(exit.TargetRoom).Title,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You fail to climb (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		// Apply falling damage.
		dmgResult, _ := s.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		s.applyDamageToPlayer(uid, dmg)

		msg := fmt.Sprintf(
			"You fall! (rolled %d+%d=%d vs DC %d) Taking %d falling damage.",
			roll, bonus, total, dc, dmg,
		)

		// Apply prone in combat only (TERRAIN-9).
		// Use duration -1 for permanent conditions.
		if inCombat {
			if def, ok := s.condReg.Get("prone"); ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
				msg += " You are knocked prone."
			}
		}
		return messageEvent(msg), nil
	}
}
```

> **Note:** `s.movePlayer`, `s.applyDamageToPlayer`, `s.condReg`, `messageEvent`, and `statusInCombat` follow existing patterns in `grpc_service.go`. Check the file for exact function/field names used in `handleGrapple` and `handleTrip`.

- [ ] **Step 3: Build**

```bash
mise run go build ./...
```

Expected: No errors.

- [ ] **Step 4: Write integration tests**

Read `grpc_service_grapple_test.go` completely. Using the same server/session construction helpers, create `internal/gameserver/grpc_service_climb_test.go` with complete (non-placeholder) test bodies:

- `TestHandleClimb_RoomNotClimbable` — build a session in a plain room (no `climbable` property); send ClimbRequest; assert response message contains "nothing to climb".
- `TestHandleClimb_NoVerticalExit` — build a session in a room with `climbable: "true"` but no `up`/`down` exits; assert error message.
- `TestHandleClimb_CritFailure_InCombat` — fix dice to return a crit-failure roll (total < dc-10); assert player HP decreased and `prone` condition is set.
- `TestHandleClimb_CritFailure_OutOfCombat` — same but out of combat; assert HP decreased and `prone` condition is NOT set.

Include `prone` and `submerged` in the test condition registry (extend `makeTestConditionRegistry` or build a local helper).

- [ ] **Step 5: Run climb tests**

```bash
mise run go test ./internal/gameserver/... -run TestHandleClimb -v
```

Expected: All PASS.

- [ ] **Step 6: Run full suite**

```bash
mise run go test ./...
```

Expected: All PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_climb_test.go
git commit -m "feat(gameserver): implement handleClimb with 4-tier outcomes, falling damage, prone"
```

---

## Chunk 3: Swim command — parser, bridge, gRPC handler, submerged enforcement

### Task 7: Swim command parser and property-based tests

**Files:**
- Create: `internal/game/command/swim.go`
- Create: `internal/game/command/swim_test.go`

- [ ] **Step 1: Write failing property-based tests**

Create `internal/game/command/swim_test.go`:

```go
package command_test

import (
	"testing"

	"pgregory.net/rapid"
	"github.com/stretchr/testify/assert"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleSwim_AlwaysReturnsRequest(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := command.HandleSwim(args)
		assert.NoError(rt, err)
		assert.NotNil(rt, req)
	})
}

// TestSwimOutcomes verifies 4-tier outcome thresholds for the swim skill check.
func TestSwimOutcomes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dc := rapid.IntRange(5, 30).Draw(rt, "dc")
		roll := rapid.IntRange(1, 30).Draw(rt, "roll")
		// CritSuccess boundary.
		assert.Equal(rt, combat.CritSuccess, combat.OutcomeFor(dc+10, dc))
		// Success boundary.
		assert.Equal(rt, combat.Success, combat.OutcomeFor(dc, dc))
		// Failure boundary.
		assert.Equal(rt, combat.Failure, combat.OutcomeFor(dc-1, dc))
		// CritFailure boundary.
		assert.Equal(rt, combat.CritFailure, combat.OutcomeFor(dc-10, dc))
		// Arbitrary roll maps to one of the four outcomes.
		o := combat.OutcomeFor(roll, dc)
		assert.True(rt, o == combat.CritSuccess || o == combat.Success || o == combat.Failure || o == combat.CritFailure)
		_ = dc
	})
}
```

- [ ] **Step 2: Run and confirm compile error**

```bash
mise run go test ./internal/game/command/... -run TestHandleSwim -v
```

Expected: compile error — `HandleSwim` undefined.

- [ ] **Step 3: Implement swim.go**

Create `internal/game/command/swim.go`:

```go
package command

// SwimRequest is the parsed form of the swim command.
// Precondition: none — no arguments are required.
// Postcondition: Always returns a non-nil *SwimRequest with nil error.
type SwimRequest struct{}

// HandleSwim parses the arguments for the "swim" command.
// Precondition: args is the slice of words following "swim" (may be empty).
// Postcondition: Returns a non-nil *SwimRequest and nil error always.
func HandleSwim(args []string) (*SwimRequest, error) {
	return &SwimRequest{}, nil
}
```

- [ ] **Step 4: Run and confirm pass**

```bash
mise run go test ./internal/game/command/... -run TestHandleSwim -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/command/swim.go internal/game/command/swim_test.go
git commit -m "feat(command): add HandleSwim parser with property-based tests"
```

---

### Task 8: Swim bridge handler

**Files:**
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add bridgeSwim to bridgeHandlerMap**

```go
command.HandlerSwim: bridgeSwim,
```

- [ ] **Step 2: Implement bridgeSwim**

```go
func bridgeSwim(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Swim{Swim: &gamev1.SwimRequest{}},
	}}, nil
}
```

- [ ] **Step 3: Run wiring test**

```bash
mise run go test ./internal/frontend/... -run TestAllCommandHandlersAreWired -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/frontend/handlers/bridge_handlers.go
git commit -m "feat(bridge): add bridgeSwim handler and register in bridgeHandlerMap"
```

---

### Task 9: handleSwim gRPC handler, submerged blocking, and integration tests

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_swim_test.go`

- [ ] **Step 1: Wire dispatch in grpc_service.go**

Find the dispatch type switch. Add:

```go
case *gamev1.ClientMessage_Swim:
    return s.handleSwim(uid, p.Swim)
```

- [ ] **Step 2: Add submerged check at top of blocked command handlers**

In `handleAttack`, `handleBurst`, `handleAuto`, and `handleReload` (find these in grpc_service.go), add at the very top after session retrieval:

```go
if sess.Conditions.Has("submerged") {
    return messageEvent("You are submerged underwater and cannot attack. Swim or Escape to surface."), nil
}
```

- [ ] **Step 3: Implement handleSwim**

Add to `grpc_service.go`:

```go
// handleSwim processes a SwimRequest from the player.
//
// Precondition: uid is a valid connected player session.
// Postcondition: Player moves on success; submerged condition applied on critical failure.
func (s *GameServiceServer) handleSwim(uid string, _ *gamev1.SwimRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", uid)
	}

	room := s.worldMgr.Room(sess.RoomID)
	if room == nil {
		return messageEvent("Room not found."), nil
	}

	isWaterRoom := room.Properties["water_terrain"] == "true"
	isSubmerged := sess.Conditions.Has("submerged")

	if !isWaterRoom && !isSubmerged {
		return messageEvent("There is no water here to swim in."), nil
	}

	// Apply out-of-combat drowning damage before acting if submerged.
	inCombat := sess.Status == statusInCombat
	if isSubmerged && !inCombat {
		dmg, _ := s.dice.RollExpr("1d6")
		if dmg < 1 {
			dmg = 1
		}
		s.applyDamageToPlayer(uid, dmg)
	}

	// Spend AP if in combat.
	if inCombat {
		if !s.combatH.SpendAP(uid, 2) {
			return messageEvent("Not enough action points to swim."), nil
		}
	}

	// Parse DC (default 12).
	dc := 12
	if dcStr, ok := room.Properties["water_dc"]; ok {
		if parsed, err := strconv.Atoi(dcStr); err == nil {
			dc = parsed
		}
	}

	// Roll athletics check.
	// s.dice.RollExpr returns a result struct; call .Total() for the integer value.
	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, err
	}
	roll := rollResult.Total()
	bonus := skillRankBonus(sess.Skills["athletics"])
	total := roll + bonus

	outcome := combat.OutcomeFor(total, dc)

	switch outcome {
	case combat.CritSuccess, combat.Success:
		// Clear submerged if present.
		if isSubmerged {
			sess.Conditions.Remove(uid, "submerged")
			return messageEvent(fmt.Sprintf(
				"You surface! (rolled %d+%d=%d vs DC %d)",
				roll, bonus, total, dc,
			)), nil
		}
		// Move to water exit (first available exit in the room).
		if len(room.Exits) > 0 {
			s.movePlayer(uid, room.Exits[0].TargetRoom)
			return messageEvent(fmt.Sprintf(
				"You swim through (rolled %d+%d=%d vs DC %d).",
				roll, bonus, total, dc,
			)), nil
		}
		return messageEvent(fmt.Sprintf(
			"You swim in place (rolled %d+%d=%d vs DC %d). No exit available.",
			roll, bonus, total, dc,
		)), nil

	case combat.Failure:
		return messageEvent(fmt.Sprintf(
			"You fail to make progress (rolled %d+%d=%d vs DC %d).",
			roll, bonus, total, dc,
		)), nil

	default: // CritFailure
		dmgResult, _ := s.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		s.applyDamageToPlayer(uid, dmg)

		// Apply submerged condition. Use duration -1 for permanent conditions.
		msg := fmt.Sprintf(
			"You are pulled under! (rolled %d+%d=%d vs DC %d) Taking %d drowning damage.",
			roll, bonus, total, dc, dmg,
		)
		if def, ok := s.condReg.Get("submerged"); ok {
			_ = sess.Conditions.Apply(uid, def, 1, -1)
			msg += " You are submerged."
		}
		return messageEvent(msg), nil
	}
}
```

- [ ] **Step 4: Build**

```bash
mise run go build ./...
```

Expected: No errors.

- [ ] **Step 5: Write integration tests**

> **IMPORTANT:** Read `grpc_service_grapple_test.go` completely before writing tests. Do NOT commit placeholder `{ ... }` bodies. All test functions must be fully implemented before committing.

Create `internal/gameserver/grpc_service_swim_test.go` with complete test bodies following the grapple test pattern:

- `TestHandleSwim_RoomNotWater_NotSubmerged` — plain room, no submerged condition; assert "no water" error message.
- `TestHandleSwim_CritFailure` — fix dice to produce crit failure; assert `submerged` condition applied and HP decreased.
- `TestHandleSwim_SubmergedSurface` — start with `submerged` condition applied; fix dice to produce success; assert `submerged` cleared.
- `TestHandleSwim_BlocksAttackWhenSubmerged` — apply `submerged`; send AttackRequest (or whichever attack message); assert blocked with error message.

Include `submerged` in the test condition registry for all tests.

- [ ] **Step 6: Run swim tests**

```bash
mise run go test ./internal/gameserver/... -run TestHandleSwim -v
```

Expected: All PASS.

- [ ] **Step 7: Run full suite**

```bash
mise run go test ./...
```

Expected: All PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_swim_test.go
git commit -m "feat(gameserver): implement handleSwim with 4-tier outcomes, submerged condition, attack blocking"
```

---

### Task 10: Per-round drowning damage in combat

**Files:**
- Modify: `internal/gameserver/combat_handler.go` (the CombatHandler's turn-start processing)
- Modify: `internal/gameserver/grpc_service_swim_test.go` (add round-start test)

The spec (TERRAIN-13) requires drowning damage at turn-start in `CombatHandler`. Find where `CombatHandler` processes the beginning of each player's turn (where AP is assigned and conditions are ticked). This is in `internal/gameserver/combat_handler.go`.

- [ ] **Step 1: Locate the turn-start hook in CombatHandler**

```bash
grep -n "StartRound\|startRound\|TurnStart\|turnStart\|ActionQueue\|NewActionQueue" internal/gameserver/combat_handler.go | head -30
```

Read the function that resets action queues / starts each player's combat turn. This is where the drowning damage hook goes.

- [ ] **Step 2: Add drowning damage at turn-start**

In `CombatHandler`, in the function that processes the start of a player's turn (after AP is set, before actions), add:

```go
// Apply per-round drowning damage if the player is submerged (TERRAIN-13).
if sess.Conditions.Has("submerged") {
    dmgResult, _ := s.dice.RollExpr("1d6")
    dmg := dmgResult.Total()
    if dmg < 1 {
        dmg = 1
    }
    s.applyDamageToPlayer(uid, dmg)
    s.notifyPlayer(uid, fmt.Sprintf("You take %d drowning damage (submerged).", dmg))
}
```

> Note: Use the exact notification helper used by other turn-start events in `combat_handler.go`. Adapt the helper name (`s.notifyPlayer`, `s.sendToPlayer`, etc.) to match what exists.

- [ ] **Step 3: Write a round-start drowning test**

Add to `grpc_service_swim_test.go` with a complete (non-placeholder) test body. Study the existing combat round-start tests in `combat_handler_test.go` for the construction pattern:

```go
// TestSubmergedDrowningOnRoundStart — drowning damage is applied at the start
// of the player's turn when the submerged condition is active.
func TestSubmergedDrowningOnRoundStart(t *testing.T) {
    // 1. Build a combat session with the player having the submerged condition.
    // 2. Record player HP before round start.
    // 3. Trigger round/turn start.
    // 4. Assert player HP has decreased by at least 1.
    // Follow the pattern in combat_handler_test.go exactly.
}
```

- [ ] **Step 4: Run test**

```bash
mise run go test ./internal/gameserver/... -run TestSubmergedDrowning -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
mise run go test ./...
```

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/combat_handler.go internal/gameserver/grpc_service_swim_test.go
git commit -m "feat(gameserver): apply per-round drowning damage at CombatHandler turn-start when submerged"
```

---

## Chunk 4: Zone content and verification

### Task 11: Add terrain rooms to a zone

**Files:**
- Modify: one zone YAML in `content/zones/` (use an existing zone that makes narrative sense, or the training zone)

- [ ] **Step 1: Identify a suitable zone**

```bash
ls content/zones/
```

Choose a zone where a climbable wall or water area fits. If none fits, use the first zone file as a demonstration zone.

- [ ] **Step 2: Add a climbable room pair**

```yaml
- id: room_cliff_base
  title: "Base of the Cliff"
  description: "A sheer rock face rises before you. Hand-holds have been carved into the stone."
  properties:
    climbable: "true"
    climb_dc: "15"
  exits:
    - direction: up
      target_room: room_cliff_top
  map_x: <choose unused coordinates>
  map_y: <choose unused coordinates>

- id: room_cliff_top
  title: "Top of the Cliff"
  description: "You stand on a rocky ledge overlooking the valley below."
  exits:
    - direction: down
      target_room: room_cliff_base
  map_x: <choose unused coordinates>
  map_y: <choose unused coordinates>
```

- [ ] **Step 3: Add a water room**

```yaml
- id: room_flooded_passage
  title: "Flooded Passage"
  description: "Murky water fills the corridor. The current is strong."
  properties:
    water_terrain: "true"
    water_dc: "12"
  exits:
    - direction: east
      target_room: <existing room ID>
  map_x: <choose unused coordinates>
  map_y: <choose unused coordinates>
```

- [ ] **Step 4: Validate zone loads**

```bash
mise run go test ./internal/game/world/... -v
```

Expected: All PASS (zone validation passes).

- [ ] **Step 5: Run full suite**

```bash
mise run go test ./...
```

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add content/zones/
git commit -m "content(zones): add climbable cliff and flooded passage rooms for terrain testing"
```

---

### Task 12: Final verification and FEATURES.md update

**Files:**
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Run full test suite**

```bash
mise run go test ./...
```

Expected: All PASS, 0 failures.

- [ ] **Step 2: Build final binary**

```bash
mise run go build ./...
```

Expected: No errors.

- [ ] **Step 3: Mark terrain types complete in FEATURES.md**

Find the Terrain Types section. Mark both sub-items complete:
- `[x]` Climbable surfaces — `climb` command, Athletics vs DC, 4-tier outcomes, falling damage + prone on crit fail
- `[x]` Water terrain — `swim` command, Athletics vs DC, 4-tier outcomes, drowning damage + submerged condition on crit fail

- [ ] **Step 4: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark terrain types (climb + swim) complete in FEATURES.md"
```
