# Character Sheet Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement a `char` command (alias `sheet`) that sends a `CharacterSheetView` proto message containing identity, ability scores, combat stats, equipped gear, and currency.

**Architecture:** New `CharacterSheetRequest`/`CharacterSheetView` proto pair wired end-to-end via the CMD pattern (constant → command → proto → bridge → grpc handler). `PlayerSession` gains `MaxHP` and `Abilities` fields populated at login. The grpc handler assembles the view from session state, jobRegistry, and `Equipment.ComputedDefenses`.

**Tech Stack:** Go 1.23, protobuf/grpc, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`

---

## Key File Locations

- `internal/game/session/manager.go` — PlayerSession struct
- `internal/game/command/commands.go` — HandlerXxx constants + BuiltinCommands()
- `internal/game/command/char.go` — **NEW** HandleChar function
- `internal/game/command/char_test.go` — **NEW** tests
- `api/proto/game/v1/game.proto` — proto messages
- `internal/gameserver/grpc_service.go` — handleChar + login population
- `internal/frontend/handlers/bridge_handlers.go` — bridgeChar

## Test Commands

```bash
# Run all non-postgres tests with race detector
mise exec -- go test -race $(mise exec -- go list ./... | grep -v postgres)

# Run command package tests
mise exec -- go test -race ./internal/game/command/... -v

# Run gameserver tests (includes TestAllCommandHandlersAreWired)
mise exec -- go test -race ./internal/gameserver/... -v
```

---

## Task 1: Add `MaxHP` and `Abilities` to `PlayerSession` and populate at login

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Read the current PlayerSession struct

Read `internal/game/session/manager.go`. Confirm `MaxHP int` and `Abilities character.AbilityScores` are absent.

Also read the login section of `internal/gameserver/grpc_service.go` — find where `sess.CurrentHP` is set (around where the character is loaded). This is where MaxHP and Abilities will be populated.

### Step 2: Add fields to PlayerSession

In `internal/game/session/manager.go`, add two fields to `PlayerSession`:

```go
import "github.com/cory-johannsen/mud/internal/game/character"

type PlayerSession struct {
    // ... existing fields ...
    MaxHP     int                      // character's maximum hit points
    Abilities character.AbilityScores  // six ability scores loaded at login
}
```

Add the import for `github.com/cory-johannsen/mud/internal/game/character` if not already present.

### Step 3: Populate at login in grpc_service.go

Find the login block in `internal/gameserver/grpc_service.go` where the character is loaded from DB and `sess.CurrentHP` is set. Add alongside it:

```go
sess.MaxHP = char.MaxHP
sess.Abilities = char.Abilities
```

Where `char` is the `*character.Character` loaded from DB. Find the exact variable name by reading the login code.

### Step 4: Build and run tests

```bash
mise exec -- go build ./... 2>&1
mise exec -- go test -race $(mise exec -- go list ./... | grep -v postgres) 2>&1
```

Expected: PASS (no behavior change, just new fields)

### Step 5: Commit

```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go
git commit -m "feat: add MaxHP and Abilities to PlayerSession, populate at login"
```

---

## Task 2: `HandleChar` command function with tests (TDD)

**Files:**
- Create: `internal/game/command/char.go`
- Create: `internal/game/command/char_test.go`

### Step 1: Write failing tests

```go
// internal/game/command/char_test.go
package command_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func newTestSession(class string) *session.PlayerSession {
	return &session.PlayerSession{
		CharName:  "TestChar",
		Class:     class,
		Level:     1,
		CurrentHP: 10,
		MaxHP:     10,
		Currency:  100,
		Abilities: character.AbilityScores{
			Brutality: 12, Grit: 10, Quickness: 14,
			Reasoning: 10, Savvy: 10, Flair: 10,
		},
		Backpack:   inventory.NewBackpack(20, 50.0),
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
	}
}

func TestHandleChar_ReturnsNonEmptyString(t *testing.T) {
	sess := newTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.NotEmpty(t, result)
}

func TestHandleChar_ShowsCharacterName(t *testing.T) {
	sess := newTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "TestChar")
}

func TestHandleChar_ShowsHP(t *testing.T) {
	sess := newTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "10")
}

func TestHandleChar_ShowsAbilityScore(t *testing.T) {
	sess := newTestSession("boot_gun")
	result := command.HandleChar(sess)
	// Brutality 12 should appear
	assert.Contains(t, result, "12")
}

func TestHandleChar_NilLoadoutSetDoesNotPanic(t *testing.T) {
	sess := newTestSession("boot_gun")
	sess.LoadoutSet = nil
	assert.NotPanics(t, func() {
		command.HandleChar(sess)
	})
}

func TestProperty_HandleChar_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		class := rapid.SampledFrom([]string{"boot_gun", "boot_machete", "", "unknown_class_xyz"}).Draw(rt, "class")
		level := rapid.IntRange(0, 20).Draw(rt, "level")
		hp := rapid.IntRange(0, 100).Draw(rt, "hp")
		sess := newTestSession(class)
		sess.Level = level
		sess.CurrentHP = hp
		assert.NotPanics(rt, func() {
			command.HandleChar(sess)
		})
	})
}
```

### Step 2: Run tests to verify they fail

```bash
mise exec -- go test ./internal/game/command/ -run TestHandleChar 2>&1
```

Expected: FAIL — `command.HandleChar undefined`

### Step 3: Implement `HandleChar`

```go
// internal/game/command/char.go
package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleChar returns a plain-text character sheet for the given session.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns a non-empty string; never panics regardless of session state.
func HandleChar(sess *session.PlayerSession) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== %s ===\n", sess.CharName))
	sb.WriteString(fmt.Sprintf("Class: %s  Level: %d\n", sess.Class, sess.Level))
	sb.WriteString(fmt.Sprintf("HP: %d / %d\n\n", sess.CurrentHP, sess.MaxHP))

	sb.WriteString("--- Abilities ---\n")
	a := sess.Abilities
	sb.WriteString(fmt.Sprintf("BRT: %d  GRT: %d  QCK: %d\n", a.Brutality, a.Grit, a.Quickness))
	sb.WriteString(fmt.Sprintf("RSN: %d  SAV: %d  FLR: %d\n\n", a.Reasoning, a.Savvy, a.Flair))

	sb.WriteString("--- Weapons ---\n")
	if sess.LoadoutSet != nil {
		preset := sess.LoadoutSet.ActivePreset()
		if preset != nil {
			mainName := "(none)"
			offName := "(none)"
			if preset.MainHand != nil {
				mainName = preset.MainHand.Def.Name
			}
			if preset.OffHand != nil {
				offName = preset.OffHand.Def.Name
			}
			sb.WriteString(fmt.Sprintf("Main: %s\nOff:  %s\n\n", mainName, offName))
		} else {
			sb.WriteString("(no active loadout)\n\n")
		}
	} else {
		sb.WriteString("(no loadout)\n\n")
	}

	sb.WriteString("--- Currency ---\n")
	sb.WriteString(fmt.Sprintf("%s\n", inventory.FormatRounds(sess.Currency)))

	return sb.String()
}
```

**Note:** Check that `preset.MainHand.Def.Name` is the correct path — read `internal/game/inventory/preset.go` to confirm. The `EquippedWeapon` struct may have a different field. Adjust if needed.

### Step 4: Run tests

```bash
mise exec -- go test -race ./internal/game/command/ -run TestHandleChar -v 2>&1
mise exec -- go test -race ./internal/game/command/ -run TestProperty_HandleChar -v 2>&1
```

Expected: PASS

### Step 5: Commit

```bash
git add internal/game/command/char.go internal/game/command/char_test.go
git commit -m "feat: add HandleChar command function"
```

---

## Task 3: Proto — add `CharacterSheetRequest` and `CharacterSheetView`

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`

### Step 1: Read the proto file

Read `api/proto/game/v1/game.proto`. Find:
- The last field number used in `ClientMessage` oneof (currently 32)
- The last field number used in `ServerEvent` oneof (currently 16)
- A good place to insert the new messages (after existing request/view pairs)

### Step 2: Add the new messages to the proto file

Add `CharacterSheetRequest` and `CharacterSheetView` message definitions:

```proto
message CharacterSheetRequest {}

message CharacterSheetView {
    string name      = 1;
    string job       = 2;
    string archetype = 3;
    string team      = 4;
    int32  level     = 5;

    int32 brutality  = 6;
    int32 grit       = 7;
    int32 quickness  = 8;
    int32 reasoning  = 9;
    int32 savvy      = 10;
    int32 flair      = 11;

    int32 current_hp    = 12;
    int32 max_hp        = 13;
    int32 ac_bonus      = 14;
    int32 check_penalty = 15;
    int32 speed_penalty = 16;

    string currency = 17;

    map<string, string> armor       = 18;
    map<string, string> accessories = 19;
    string main_hand = 20;
    string off_hand  = 21;
}
```

Add to `ClientMessage` oneof (use field 33 — one past the current last):
```proto
CharacterSheetRequest char_sheet = 33;
```

Add to `ServerEvent` oneof (use field 17 — one past current last):
```proto
CharacterSheetView character_sheet = 17;
```

### Step 3: Regenerate proto

```bash
make proto 2>&1
```

Expected: no errors, `internal/gameserver/gamev1/game.pb.go` regenerated.

### Step 4: Build

```bash
mise exec -- go build ./... 2>&1
```

Expected: PASS (no new wiring yet, just new types)

### Step 5: Commit

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add CharacterSheetRequest and CharacterSheetView proto messages"
```

---

## Task 4: Wire everything end-to-end

**Files:**
- Modify: `internal/game/command/commands.go` — HandlerChar constant + Command entry
- Modify: `internal/frontend/handlers/bridge_handlers.go` — bridgeChar + map registration
- Modify: `internal/gameserver/grpc_service.go` — handleChar + dispatch case

All wiring must be done in this task so `TestAllCommandHandlersAreWired` passes.

### Step 1: Add `HandlerChar` to commands.go

In `internal/game/command/commands.go`, add to the constants block:

```go
HandlerChar = "char"
```

Add to `BuiltinCommands()` slice:

```go
{Name: "char", Aliases: []string{"sheet"}, Help: "Display your character sheet", Category: CategoryWorld, Handler: HandlerChar},
```

### Step 2: Add `bridgeChar` to bridge_handlers.go

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap`:
```go
command.HandlerChar: bridgeChar,
```

Add the function:
```go
func bridgeChar(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_CharSheet{CharSheet: &gamev1.CharacterSheetRequest{}},
	}}, nil
}
```

**Note:** The proto field accessor name `ClientMessage_CharSheet` is generated from the field name `char_sheet` in the proto — verify the exact generated type name in `internal/gameserver/gamev1/game.pb.go` after `make proto`. Adjust if different.

### Step 3: Add `handleChar` to grpc_service.go

Add to the dispatch type switch:

```go
case *gamev1.ClientMessage_CharSheet:
    return s.handleChar(uid)
```

Add the handler method. Read the file to find a good place to insert it (near other character-related handlers):

```go
// handleChar builds and returns a CharacterSheetView for the requesting player.
//
// Precondition: uid must identify an active session.
// Postcondition: Returns CharacterSheetView on success; errorEvent if session not found.
func (s *GameServiceServer) handleChar(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}

	view := &gamev1.CharacterSheetView{
		Name:      sess.CharName,
		Level:     int32(sess.Level),
		CurrentHp: int32(sess.CurrentHP),
		MaxHp:     int32(sess.MaxHP),
		Brutality: int32(sess.Abilities.Brutality),
		Grit:      int32(sess.Abilities.Grit),
		Quickness: int32(sess.Abilities.Quickness),
		Reasoning: int32(sess.Abilities.Reasoning),
		Savvy:     int32(sess.Abilities.Savvy),
		Flair:     int32(sess.Abilities.Flair),
		Currency:  inventory.FormatRounds(sess.Currency),
	}

	// Job info from registry.
	if job, ok := s.jobRegistry.Job(sess.Class); ok {
		view.Job = job.Name
		view.Archetype = job.Archetype
	} else {
		view.Job = sess.Class
	}
	view.Team = s.jobRegistry.TeamFor(sess.Class)

	// Defense stats.
	dexMod := (sess.Abilities.Quickness - 10) / 2
	def := sess.Equipment.ComputedDefenses(s.invRegistry, dexMod)
	view.AcBonus = int32(def.ACBonus)
	view.CheckPenalty = int32(def.CheckPenalty)
	view.SpeedPenalty = int32(def.SpeedPenalty)

	// Armor slots.
	view.Armor = make(map[string]string)
	for slot, item := range sess.Equipment.Armor {
		if item != nil {
			view.Armor[string(slot)] = item.Name
		}
	}

	// Accessory slots.
	view.Accessories = make(map[string]string)
	for slot, item := range sess.Equipment.Accessories {
		if item != nil {
			view.Accessories[string(slot)] = item.Name
		}
	}

	// Weapons from active loadout.
	if sess.LoadoutSet != nil {
		if preset := sess.LoadoutSet.ActivePreset(); preset != nil {
			if preset.MainHand != nil {
				view.MainHand = preset.MainHand.Def.Name
			}
			if preset.OffHand != nil {
				view.OffHand = preset.OffHand.Def.Name
			}
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_CharacterSheet{CharacterSheet: view},
	}, nil
}
```

**Note:** Proto field accessor names (`CurrentHp`, `MaxHp`, etc.) are generated from snake_case proto field names. Verify exact names in `game.pb.go` after `make proto`. Also verify `ServerEvent_CharacterSheet` is the correct generated type name.

### Step 4: Run all tests

```bash
mise exec -- go build ./... 2>&1
mise exec -- go test -race $(mise exec -- go list ./... | grep -v postgres) 2>&1
```

Expected: all PASS including `TestAllCommandHandlersAreWired`.

### Step 5: Commit

```bash
git add internal/game/command/commands.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: wire char command end-to-end (handler, bridge, grpc)"
```

---

## Final Verification

```bash
mise exec -- go test -race $(mise exec -- go list ./... | grep -v postgres) 2>&1
```

Expected: all PASS, 0 failures.

Then use `superpowers:finishing-a-development-branch` to merge and deploy.
