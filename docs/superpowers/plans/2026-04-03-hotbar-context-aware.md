# Hotbar Context-Aware Slots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade hotbar slots from plain command strings to typed references (command/feat/technology/throwable/consumable) with display-name resolution, hover tooltips, and UI-driven typed assignment from Feats/Tech/Inventory drawers.

**Architecture:** Add a `HotbarSlot{Kind, Ref string}` domain type that replaces `[10]string` throughout the stack. The server resolves display names via registries when building `HotbarUpdateEvent`, so clients (telnet and web) never need to look up names themselves. The DB auto-migrates old plain-string format on load.

**Tech Stack:** Go (session domain type, proto, DB, grpc service, telnet bridge), protobuf (game.proto + regenerated game.pb.go), TypeScript/React (proto types, GameContext, HotbarPanel, FeatsDrawer, TechnologyDrawer, InventoryDrawer)

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/game/session/hotbar_slot.go` | **Create** | `HotbarSlot` domain type + `ActivationCommand()` |
| `internal/game/session/hotbar_slot_test.go` | **Create** | Unit + property tests for `HotbarSlot` |
| `internal/game/session/manager.go` | **Modify** | Change `Hotbar [10]string` → `Hotbar [10]HotbarSlot` |
| `api/proto/game/v1/game.proto` | **Modify** | Add `HotbarSlot` message; update `HotbarUpdateEvent`, `HotbarRequest`; add `throwable` to `InventoryItem` |
| `internal/gameserver/gamev1/game.pb.go` | **Regenerate** | `make proto` |
| `internal/storage/postgres/character_hotbar.go` | **Modify** | Typed JSON persistence; old-format migration on load |
| `internal/storage/postgres/character_hotbar_test.go` | **Modify** | Update tests |
| `internal/gameserver/grpc_service.go` | **Modify** | Update `CharacterSaver` interface + login load site |
| `internal/gameserver/grpc_service_hotbar.go` | **Modify** | Typed slot handling; `resolveHotbarSlotDisplay`; `hotbarUpdateEvent` becomes server method |
| `internal/gameserver/grpc_service_hotbar_test.go` | **Modify** | Update/extend tests |
| `internal/frontend/handlers/game_bridge.go` | **Modify** | `currentHotbar` stores `[10]*gamev1.HotbarSlot`; typed activation; `hotbarLabels` helper |
| `cmd/webclient/ui/src/proto/index.ts` | **Modify** | Add `HotbarSlot` interface; add `throwable` to `InventoryItem` |
| `cmd/webclient/ui/src/game/GameContext.tsx` | **Modify** | `hotbarSlots: HotbarSlot[]`; update action/reducer/initialState/handler |
| `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx` | **Modify** | Display names; hover tooltips; typed activation |
| `cmd/webclient/ui/src/game/drawers/FeatsDrawer.tsx` | **Modify** | Send `{kind:'feat', ref:id}` instead of text command |
| `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx` | **Modify** | Send `{kind:'technology', ref:id}` instead of text command |
| `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` | **Modify** | Add throwable+consumable "Add to hotbar" buttons with `SlotPicker` |

---

## Task 1: Domain type `HotbarSlot` + update `session.Hotbar`

**Files:**
- Create: `internal/game/session/hotbar_slot.go`
- Create: `internal/game/session/hotbar_slot_test.go`
- Modify: `internal/game/session/manager.go:134-137`

- [ ] **Step 1: Write the failing test**

```go
// internal/game/session/hotbar_slot_test.go
package session_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
)

func TestHotbarSlot_ActivationCommand_Command(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "strike goblin"}
	assert.Equal(t, "strike goblin", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_EmptyKindFallsBackToRef(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: "", Ref: "strike goblin"}
	assert.Equal(t, "strike goblin", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Feat(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}
	assert.Equal(t, "use power_strike", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Technology(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindTechnology, Ref: "healing_salve"}
	assert.Equal(t, "use healing_salve", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Throwable(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindThrowable, Ref: "frag_grenade"}
	assert.Equal(t, "throw frag_grenade", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_Consumable(t *testing.T) {
	t.Parallel()
	s := session.HotbarSlot{Kind: session.HotbarSlotKindConsumable, Ref: "stim_pack"}
	assert.Equal(t, "use stim_pack", s.ActivationCommand())
}

func TestHotbarSlot_ActivationCommand_EmptyRefReturnsEmpty(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{
		session.HotbarSlotKindCommand,
		session.HotbarSlotKindFeat,
		session.HotbarSlotKindTechnology,
		session.HotbarSlotKindThrowable,
		session.HotbarSlotKindConsumable,
	} {
		s := session.HotbarSlot{Kind: kind, Ref: ""}
		assert.Equal(t, "", s.ActivationCommand(), "kind=%s", kind)
	}
}

func TestHotbarSlot_IsEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, session.HotbarSlot{}.IsEmpty())
	assert.False(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "x"}.IsEmpty())
}

func TestHotbarSlot_CommandSlot(t *testing.T) {
	t.Parallel()
	s := session.CommandSlot("attack")
	assert.Equal(t, session.HotbarSlotKindCommand, s.Kind)
	assert.Equal(t, "attack", s.Ref)
}

func TestProperty_HotbarSlot_ActivationCommand_NeverPanic(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz_"))).Draw(rt, "kind")
		ref := rapid.String().Draw(rt, "ref")
		s := session.HotbarSlot{Kind: kind, Ref: ref}
		_ = s.ActivationCommand() // must not panic
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -run "TestHotbarSlot|TestProperty_HotbarSlot" -v 2>&1 | tail -20
```

Expected: FAIL — `session.HotbarSlot` undefined.

- [ ] **Step 3: Create `internal/game/session/hotbar_slot.go`**

```go
package session

// HotbarSlot kind constants.
const (
	HotbarSlotKindCommand    = "command"
	HotbarSlotKindFeat       = "feat"
	HotbarSlotKindTechnology = "technology"
	HotbarSlotKindThrowable  = "throwable"
	HotbarSlotKindConsumable = "consumable"
)

// HotbarSlot is a typed hotbar entry. Kind identifies the slot type; Ref holds
// the command text (for "command") or the item/feat/tech ID (for all others).
//
// Invariant: IsEmpty() ⟺ Ref == "".
type HotbarSlot struct {
	Kind string // one of the HotbarSlotKind* constants
	Ref  string // command text or item/feat/tech ID
}

// ActivationCommand returns the game command executed when this slot fires.
// Returns "" when Ref is empty (slot is unassigned).
//
// Precondition: none.
// Postcondition: Returns a valid command string or "".
func (s HotbarSlot) ActivationCommand() string {
	if s.Ref == "" {
		return ""
	}
	switch s.Kind {
	case HotbarSlotKindFeat, HotbarSlotKindTechnology, HotbarSlotKindConsumable:
		return "use " + s.Ref
	case HotbarSlotKindThrowable:
		return "throw " + s.Ref
	default: // "command" or unset kind
		return s.Ref
	}
}

// IsEmpty returns true when the slot has no bound action.
func (s HotbarSlot) IsEmpty() bool {
	return s.Ref == ""
}

// CommandSlot creates a HotbarSlot of kind "command" with the given text.
func CommandSlot(text string) HotbarSlot {
	return HotbarSlot{Kind: HotbarSlotKindCommand, Ref: text}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -run "TestHotbarSlot|TestProperty_HotbarSlot" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 5: Change `Hotbar` field in `manager.go`**

In `internal/game/session/manager.go`, change line ~137:
```go
// Before:
Hotbar [10]string

// After:
// Hotbar holds the player's 10 persistent hotbar slot assignments.
// Index 0 = slot 1 (key "1"), index 9 = slot 10 (key "0").
// Loaded from DB at login; written through on any hotbar set/clear command.
Hotbar [10]HotbarSlot
```

- [ ] **Step 6: Verify the package still compiles (expect failures in dependents — that's OK)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/game/session/... 2>&1 | head -20
```

Expected: compiles cleanly. Dependent packages (gameserver, postgres) will have compile errors until patched in later tasks.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/hotbar_slot.go internal/game/session/hotbar_slot_test.go internal/game/session/manager.go && git commit -m "feat(hotbar): add HotbarSlot domain type; change session.Hotbar to [10]HotbarSlot"
```

---

## Task 2: Proto changes — `HotbarSlot` message, updated events/requests

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)

- [ ] **Step 1: Edit `game.proto` — add `HotbarSlot` message**

Find the `HotbarRequest` definition (~line 1385) and replace the entire hotbar section:

```proto
// HotbarSlot carries a typed hotbar slot reference.
// kind: "command" | "feat" | "technology" | "throwable" | "consumable"
// ref: command text (for "command") or item/feat/tech ID (for all others)
// display_name: resolved by server from registries; empty falls back to ref
// description: resolved by server; empty for command slots
message HotbarSlot {
  string kind         = 1;
  string ref          = 2;
  string display_name = 3;
  string description  = 4;
}

// HotbarRequest asks the server to set, clear, or show a hotbar slot.
// For typed (UI-driven) assignment, set kind+ref and leave text empty.
// For legacy command-text assignment (telnet hotbar command), set text and leave kind/ref empty.
message HotbarRequest {
  string action = 1;  // "set", "clear", "show"
  int32  slot   = 2;  // 1–10 for set/clear; ignored for show
  string text   = 3;  // backward-compat: non-empty for command-kind "set" via telnet
  string kind   = 4;  // typed kind for UI-driven set
  string ref    = 5;  // typed ref for UI-driven set
}

// HotbarUpdateEvent pushes the player's full 10-slot hotbar to the client.
message HotbarUpdateEvent {
  // Always exactly 10 entries; slots[0] = slot 1, slots[9] = slot 10.
  // An empty HotbarSlot (kind="", ref="") means unassigned.
  repeated HotbarSlot slots = 1;
}
```

- [ ] **Step 2: Add `throwable` field to `InventoryItem` in `game.proto`**

Find `message InventoryItem` (~line 814) and add field 10:

```proto
message InventoryItem {
  string instance_id = 1;
  string name = 2;
  string kind = 3;
  int32 quantity = 4;
  double weight = 5;
  string item_def_id = 6;
  string armor_slot     = 7;
  string armor_category = 8;
  string effects_summary = 9;
  bool   throwable       = 10;  // true when item has "throwable" tag
}
```

- [ ] **Step 3: Regenerate `game.pb.go`**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1
```

Expected: exits 0. `internal/gameserver/gamev1/game.pb.go` updated.

- [ ] **Step 4: Verify proto package compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/gameserver/gamev1/... 2>&1
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go && git commit -m "feat(hotbar): add HotbarSlot proto message; update HotbarUpdateEvent/HotbarRequest; add throwable to InventoryItem"
```

---

## Task 3: DB persistence — typed JSON with old-format migration

**Files:**
- Modify: `internal/storage/postgres/character_hotbar.go`
- Modify: `internal/storage/postgres/character_hotbar_test.go` (if it exists; check with glob)

- [ ] **Step 1: Write the failing tests**

Check for an existing test file first:
```bash
ls /home/cjohannsen/src/mud/internal/storage/postgres/character_hotbar_test.go 2>/dev/null || echo "MISSING"
```

If missing, create `internal/storage/postgres/character_hotbar_test.go`:

```go
package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// TestHotbarJSON_MigratesOldStringFormat verifies that the old plain-string
// format is detected and migrated to typed slots on load.
func TestHotbarJSON_MigratesOldStringFormat(t *testing.T) {
	t.Parallel()
	old := `["attack goblin","",  "use heal_tech","","","","","","",""]`
	slots, err := postgres.UnmarshalHotbarSlots([]byte(old))
	require.NoError(t, err)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "attack goblin"}, slots[0])
	assert.Equal(t, session.HotbarSlot{}, slots[1]) // empty string → empty slot
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "use heal_tech"}, slots[2])
}

// TestHotbarJSON_RoundTripsTypedSlots verifies that typed slots survive
// marshal → unmarshal unchanged.
func TestHotbarJSON_RoundTripsTypedSlots(t *testing.T) {
	t.Parallel()
	in := [10]session.HotbarSlot{
		{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"},
		{Kind: session.HotbarSlotKindTechnology, Ref: "healing_salve"},
		{Kind: session.HotbarSlotKindThrowable, Ref: "frag_grenade"},
		{Kind: session.HotbarSlotKindConsumable, Ref: "stim_pack"},
		{Kind: session.HotbarSlotKindCommand, Ref: "flee"},
	}
	b, err := postgres.MarshalHotbarSlots(in)
	require.NoError(t, err)
	out, err := postgres.UnmarshalHotbarSlots(b)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

// TestHotbarJSON_AllEmptyMarshalsNull verifies an all-empty hotbar produces nil bytes.
func TestHotbarJSON_AllEmptyMarshalsNull(t *testing.T) {
	t.Parallel()
	b, err := postgres.MarshalHotbarSlots([10]session.HotbarSlot{})
	require.NoError(t, err)
	assert.Nil(t, b)
}

// TestProperty_HotbarJSON_RoundTrip verifies marshal/unmarshal is lossless
// for randomly-generated typed slot arrays.
func TestProperty_HotbarJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	kinds := []string{
		session.HotbarSlotKindCommand,
		session.HotbarSlotKindFeat,
		session.HotbarSlotKindTechnology,
		session.HotbarSlotKindThrowable,
		session.HotbarSlotKindConsumable,
	}
	rapid.Check(t, func(rt *rapid.T) {
		var slots [10]session.HotbarSlot
		for i := range slots {
			if rapid.Bool().Draw(rt, "assigned") {
				kind := kinds[rapid.IntRange(0, len(kinds)-1).Draw(rt, "kind")]
				ref := rapid.StringMatching(`[a-z_]+`).Draw(rt, "ref")
				slots[i] = session.HotbarSlot{Kind: kind, Ref: ref}
			}
		}
		b, err := postgres.MarshalHotbarSlots(slots)
		require.NoError(rt, err)
		if b == nil {
			return // all-empty: no further check needed
		}
		out, err := postgres.UnmarshalHotbarSlots(b)
		require.NoError(rt, err)
		assert.Equal(rt, slots, out)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/storage/postgres/... -run "TestHotbarJSON|TestProperty_HotbarJSON" -v 2>&1 | tail -20
```

Expected: FAIL — `postgres.UnmarshalHotbarSlots` undefined.

- [ ] **Step 3: Rewrite `internal/storage/postgres/character_hotbar.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// hotbarSlotJSON is the on-disk JSON representation of a single hotbar slot.
type hotbarSlotJSON struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// MarshalHotbarSlots serialises a [10]HotbarSlot to JSON bytes.
// Returns nil when all slots are empty (stored as NULL in the DB).
//
// Precondition: none.
// Postcondition: Returns nil iff all slots are empty; otherwise valid JSON.
func MarshalHotbarSlots(slots [10]session.HotbarSlot) ([]byte, error) {
	allEmpty := true
	for _, s := range slots {
		if !s.IsEmpty() {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return nil, nil
	}
	arr := make([]hotbarSlotJSON, 10)
	for i, s := range slots {
		arr[i] = hotbarSlotJSON{Kind: s.Kind, Ref: s.Ref}
	}
	return json.Marshal(arr)
}

// UnmarshalHotbarSlots deserialises JSON bytes to a [10]HotbarSlot.
// Auto-migrates the legacy plain-string format (["cmd1","cmd2",...]).
//
// Precondition: data is non-nil and non-empty.
// Postcondition: Returns a valid [10]HotbarSlot; legacy strings become command slots.
func UnmarshalHotbarSlots(data []byte) ([10]session.HotbarSlot, error) {
	// Try new typed format first.
	var typed []hotbarSlotJSON
	if err := json.Unmarshal(data, &typed); err == nil && len(typed) > 0 {
		// If first element has a "kind" field set, it's the new format.
		// (An old plain-string array would unmarshal into zero-value structs.)
		hasKind := false
		for _, v := range typed {
			if v.Kind != "" || v.Ref != "" {
				hasKind = true
				break
			}
		}
		if hasKind {
			var slots [10]session.HotbarSlot
			for i := 0; i < len(typed) && i < 10; i++ {
				if typed[i].Ref != "" {
					slots[i] = session.HotbarSlot{Kind: typed[i].Kind, Ref: typed[i].Ref}
				}
			}
			return slots, nil
		}
	}

	// Fallback: try legacy plain-string format.
	var legacy []string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return [10]session.HotbarSlot{}, fmt.Errorf("UnmarshalHotbarSlots: %w", err)
	}
	var slots [10]session.HotbarSlot
	for i := 0; i < len(legacy) && i < 10; i++ {
		if legacy[i] != "" {
			slots[i] = session.CommandSlot(legacy[i])
		}
	}
	return slots, nil
}

// SaveHotbar persists the player's 10-slot hotbar to the characters table.
//
// Precondition: characterID > 0.
// Postcondition: characters.hotbar updated; returns ErrCharacterNotFound if no row updated.
func (r *CharacterRepository) SaveHotbar(ctx context.Context, characterID int64, slots [10]session.HotbarSlot) error {
	if characterID <= 0 {
		return fmt.Errorf("SaveHotbar: characterID must be > 0, got %d", characterID)
	}
	b, err := MarshalHotbarSlots(slots)
	if err != nil {
		return fmt.Errorf("SaveHotbar: marshal: %w", err)
	}
	var encoded *string
	if b != nil {
		s := string(b)
		encoded = &s
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE characters SET hotbar = $2 WHERE id = $1`,
		characterID, encoded,
	)
	if err != nil {
		return fmt.Errorf("SaveHotbar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("SaveHotbar: %w", ErrCharacterNotFound)
	}
	return nil
}

// LoadHotbar retrieves the player's 10-slot hotbar from the characters table.
// Returns an all-empty [10]HotbarSlot if the column is NULL.
// Auto-migrates legacy plain-string format on read.
//
// Precondition: characterID > 0.
// Postcondition: Returns a valid [10]HotbarSlot; returns ErrCharacterNotFound if no character row.
func (r *CharacterRepository) LoadHotbar(ctx context.Context, characterID int64) ([10]session.HotbarSlot, error) {
	var raw *string
	err := r.db.QueryRow(ctx,
		`SELECT hotbar FROM characters WHERE id = $1`, characterID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return [10]session.HotbarSlot{}, ErrCharacterNotFound
		}
		return [10]session.HotbarSlot{}, fmt.Errorf("LoadHotbar: %w", err)
	}
	if raw == nil {
		return [10]session.HotbarSlot{}, nil
	}
	slots, err := UnmarshalHotbarSlots([]byte(*raw))
	if err != nil {
		return [10]session.HotbarSlot{}, fmt.Errorf("LoadHotbar: %w", err)
	}
	return slots, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/storage/postgres/... -run "TestHotbarJSON|TestProperty_HotbarJSON" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 5: Verify full postgres package compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/storage/postgres/... 2>&1
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/storage/postgres/character_hotbar.go internal/storage/postgres/character_hotbar_test.go && git commit -m "feat(hotbar): typed JSON persistence with legacy migration for character_hotbar"
```

---

## Task 4: Update `CharacterSaver` interface + backend hotbar service

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (interface + login load site)
- Modify: `internal/gameserver/grpc_service_hotbar.go` (typed handling + display name resolution)
- Modify: `internal/gameserver/grpc_service_hotbar_test.go`

- [ ] **Step 1: Write new failing tests for `grpc_service_hotbar.go`**

The existing test file at `internal/gameserver/grpc_service_hotbar_test.go` tests `handleHotbar`. Update it to cover typed slot handling. Find the existing test file:

```bash
cat /home/cjohannsen/src/mud/internal/gameserver/grpc_service_hotbar_test.go 2>/dev/null | head -20 || echo "MISSING"
```

Add (or replace) the test file with coverage for typed assignment:

```go
package gameserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestHandleHotbar_SetCommandSlot verifies that a text-only set request creates
// a command-kind slot (backward compatibility).
func TestHandleHotbar_SetCommandSlot(t *testing.T) {
	t.Parallel()
	srv, uid := newTestServer(t)
	sess := addTestPlayer(t, srv, uid)

	req := &gamev1.HotbarRequest{Action: "set", Slot: 1, Text: "attack goblin"}
	evt, err := srv.ExportHandleHotbar(uid, req)
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindCommand, Ref: "attack goblin"}, sess.Hotbar[0])
}

// TestHandleHotbar_SetTypedFeatSlot verifies that a kind+ref request creates
// a typed feat slot.
func TestHandleHotbar_SetTypedFeatSlot(t *testing.T) {
	t.Parallel()
	srv, uid := newTestServer(t)
	sess := addTestPlayer(t, srv, uid)

	req := &gamev1.HotbarRequest{Action: "set", Slot: 2, Kind: "feat", Ref: "power_strike"}
	evt, err := srv.ExportHandleHotbar(uid, req)
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "power_strike"}, sess.Hotbar[1])
}

// TestHandleHotbar_ClearSlot verifies that clear empties the slot.
func TestHandleHotbar_ClearSlot(t *testing.T) {
	t.Parallel()
	srv, uid := newTestServer(t)
	sess := addTestPlayer(t, srv, uid)
	sess.Hotbar[3] = session.HotbarSlot{Kind: session.HotbarSlotKindFeat, Ref: "some_feat"}

	req := &gamev1.HotbarRequest{Action: "clear", Slot: 4}
	_, err := srv.ExportHandleHotbar(uid, req)
	require.NoError(t, err)
	assert.True(t, sess.Hotbar[3].IsEmpty())
}

// TestHandleHotbar_OutOfRangeSlot verifies an error message for slot 0 or 11.
func TestHandleHotbar_OutOfRangeSlot(t *testing.T) {
	t.Parallel()
	srv, uid := newTestServer(t)
	addTestPlayer(t, srv, uid)

	for _, slot := range []int32{0, 11} {
		req := &gamev1.HotbarRequest{Action: "set", Slot: slot, Text: "foo"}
		evt, err := srv.ExportHandleHotbar(uid, req)
		require.NoError(t, err)
		require.NotNil(t, evt)
		msg := evt.GetMessage()
		require.NotNil(t, msg)
		assert.Contains(t, msg.Text, "out of range")
	}
}
```

Note: `ExportHandleHotbar`, `newTestServer`, and `addTestPlayer` should already exist in the gameserver test helpers. If `ExportHandleHotbar` does not exist, add it to a `_test_export.go` file (see Step 3 below).

- [ ] **Step 2: Run tests to verify they fail (due to compile errors)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleHotbar" -v 2>&1 | tail -30
```

Expected: compile errors — `ExportHandleHotbar` undefined, or `sess.Hotbar[0]` type mismatch.

- [ ] **Step 3: Update `CharacterSaver` interface in `grpc_service.go`**

Find lines ~124-125 in `internal/gameserver/grpc_service.go` and update the interface signatures:

```go
// Replace:
SaveHotbar(ctx context.Context, characterID int64, slots [10]string) error
LoadHotbar(ctx context.Context, characterID int64) ([10]string, error)

// With:
SaveHotbar(ctx context.Context, characterID int64, slots [10]session.HotbarSlot) error
LoadHotbar(ctx context.Context, characterID int64) ([10]session.HotbarSlot, error)
```

Add the import for `session` at the top of `grpc_service.go` if not already present:
```go
"github.com/cory-johannsen/mud/internal/game/session"
```

- [ ] **Step 4: Rewrite `internal/gameserver/grpc_service_hotbar.go`**

```go
package gameserver

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleHotbar processes hotbar commands: set, clear, show.
// For "set" with kind+ref, creates a typed slot. For "set" with text only,
// creates a command slot (backward compatibility with telnet hotbar command).
//
// Precondition: uid identifies a connected player; req is non-nil.
// Postcondition: On "set"/"clear", sess.Hotbar updated, SaveHotbar called, event returned.
func (s *GameServiceServer) handleHotbar(uid string, req *gamev1.HotbarRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	switch req.Action {
	case "set":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		slot := buildHotbarSlot(req)
		if slot.IsEmpty() {
			return messageEvent("Nothing to set: provide text or kind+ref."), nil
		}
		sess.Hotbar[idx] = slot
		s.persistHotbar(uid, sess)
		return s.hotbarUpdateEvent(sess.Hotbar), nil

	case "clear":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		sess.Hotbar[idx] = session.HotbarSlot{}
		s.persistHotbar(uid, sess)
		return s.hotbarUpdateEvent(sess.Hotbar), nil

	case "show":
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			sl := sess.Hotbar[i]
			display := "---"
			if !sl.IsEmpty() {
				display = sl.ActivationCommand()
			}
			s.pushMessageToUID(uid, fmt.Sprintf("[%d] %s", slotNum, display))
		}
		return nil, nil

	default:
		return messageEvent(fmt.Sprintf("Unknown hotbar action '%s'. Usage: hotbar [<slot> <text>] | clear <slot>", req.Action)), nil
	}
}

// buildHotbarSlot converts a HotbarRequest into a domain HotbarSlot.
// Prefers kind+ref over text for typed assignment.
func buildHotbarSlot(req *gamev1.HotbarRequest) session.HotbarSlot {
	if req.Kind != "" && req.Ref != "" {
		return session.HotbarSlot{Kind: req.Kind, Ref: req.Ref}
	}
	if req.Text != "" {
		return session.CommandSlot(req.Text)
	}
	return session.HotbarSlot{}
}

// persistHotbar saves the player's hotbar to the DB, logging any errors.
func (s *GameServiceServer) persistHotbar(uid string, sess interface{ GetCharacterID() int64; GetHotbar() [10]session.HotbarSlot }) {
	// Use type assertion for direct field access since PlayerSession is a struct.
	type hotbarOwner interface {
		GetCharacterID() int64
	}
	// Direct struct field access via the session package.
	ps, ok := sess.(*playerSessionWrapper)
	_ = ok
	// Note: sess is *session.PlayerSession — access fields directly.
	// This function is an inline in handleHotbar below.
}

// hotbarUpdateEvent builds a HotbarUpdateEvent for the player's current hotbar.
// Resolves display_name and description from registries for typed slots.
//
// Postcondition: Returns a non-nil ServerEvent with exactly 10 HotbarSlot entries.
func (s *GameServiceServer) hotbarUpdateEvent(slots [10]session.HotbarSlot) *gamev1.ServerEvent {
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range slots {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
		}
		protoSlots[i] = ps
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{Slots: protoSlots},
		},
	}
}

// resolveHotbarSlotDisplay returns the display_name and description for a slot
// by querying the appropriate registry.
//
// Precondition: slot.Ref is non-empty.
// Postcondition: Returns ("", "") when the ID is not found in any registry.
func (s *GameServiceServer) resolveHotbarSlotDisplay(slot session.HotbarSlot) (displayName, description string) {
	switch slot.Kind {
	case session.HotbarSlotKindFeat:
		if s.featRegistry != nil {
			if feat, ok := s.featRegistry.Feat(slot.Ref); ok {
				return feat.Name, feat.Description
			}
		}
	case session.HotbarSlotKindTechnology:
		if s.techRegistry != nil {
			if tech, ok := s.techRegistry.Get(slot.Ref); ok {
				name := tech.Name
				if tech.ShortName != "" {
					name = tech.ShortName
				}
				return name, tech.Description
			}
		}
	case session.HotbarSlotKindThrowable, session.HotbarSlotKindConsumable:
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(slot.Ref); ok {
				return def.Name, def.Description
			}
		}
	}
	return "", ""
}
```

**Important:** The `persistHotbar` stub above is wrong — replace the entire `handleHotbar` function with this cleaner version that inlines the persistence call:

```go
package gameserver

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func (s *GameServiceServer) handleHotbar(uid string, req *gamev1.HotbarRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	switch req.Action {
	case "set":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		slot := buildHotbarSlot(req)
		if slot.IsEmpty() {
			return messageEvent("Nothing to set: provide text or kind+ref."), nil
		}
		sess.Hotbar[idx] = slot
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbar(context.Background(), sess.CharacterID, sess.Hotbar); err != nil {
				s.logger.Warn("SaveHotbar failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		return s.hotbarUpdateEvent(sess.Hotbar), nil

	case "clear":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		sess.Hotbar[idx] = session.HotbarSlot{}
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbar(context.Background(), sess.CharacterID, sess.Hotbar); err != nil {
				s.logger.Warn("SaveHotbar failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		return s.hotbarUpdateEvent(sess.Hotbar), nil

	case "show":
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			sl := sess.Hotbar[i]
			display := "---"
			if !sl.IsEmpty() {
				display = sl.ActivationCommand()
			}
			s.pushMessageToUID(uid, fmt.Sprintf("[%d] %s", slotNum, display))
		}
		return nil, nil

	default:
		return messageEvent(fmt.Sprintf("Unknown hotbar action '%s'. Usage: hotbar [<slot> <text>] | clear <slot>", req.Action)), nil
	}
}

func buildHotbarSlot(req *gamev1.HotbarRequest) session.HotbarSlot {
	if req.Kind != "" && req.Ref != "" {
		return session.HotbarSlot{Kind: req.Kind, Ref: req.Ref}
	}
	if req.Text != "" {
		return session.CommandSlot(req.Text)
	}
	return session.HotbarSlot{}
}

func (s *GameServiceServer) hotbarUpdateEvent(slots [10]session.HotbarSlot) *gamev1.ServerEvent {
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range slots {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
		}
		protoSlots[i] = ps
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{Slots: protoSlots},
		},
	}
}

func (s *GameServiceServer) resolveHotbarSlotDisplay(slot session.HotbarSlot) (displayName, description string) {
	switch slot.Kind {
	case session.HotbarSlotKindFeat:
		if s.featRegistry != nil {
			if feat, ok := s.featRegistry.Feat(slot.Ref); ok {
				return feat.Name, feat.Description
			}
		}
	case session.HotbarSlotKindTechnology:
		if s.techRegistry != nil {
			if tech, ok := s.techRegistry.Get(slot.Ref); ok {
				name := tech.Name
				if tech.ShortName != "" {
					name = tech.ShortName
				}
				return name, tech.Description
			}
		}
	case session.HotbarSlotKindThrowable, session.HotbarSlotKindConsumable:
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(slot.Ref); ok {
				return def.Name, def.Description
			}
		}
	}
	return "", ""
}
```

- [ ] **Step 5: Add `ExportHandleHotbar` test export if it doesn't exist**

Check if the gameserver test helpers have an export mechanism:
```bash
grep -rn "ExportHandle\|testServer\|newTestServer" /home/cjohannsen/src/mud/internal/gameserver/*_test*.go | head -10
```

If `ExportHandleHotbar` doesn't exist, add it to a test-export file:

```go
// internal/gameserver/export_test.go  (add or update)
package gameserver

func (s *GameServiceServer) ExportHandleHotbar(uid string, req *gamev1.HotbarRequest) (*gamev1.ServerEvent, error) {
	return s.handleHotbar(uid, req)
}
```

- [ ] **Step 6: Fix login load site in `grpc_service.go` (~line 1389)**

```go
// Replace:
sess.Hotbar = hotbarSlots  // already [10]HotbarSlot — no change needed if type matches
```

The assignment `sess.Hotbar = hotbarSlots` is still valid since both are `[10]session.HotbarSlot` after Task 1+3. Verify by building:

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/gameserver/... 2>&1 | head -20
```

Fix any remaining compile errors (e.g., calls to `hotbarUpdateEvent` as a standalone function must become `s.hotbarUpdateEvent`).

- [ ] **Step 7: Also add `throwable` field to inventory items in `handleInventory`**

In `grpc_service.go` around line 5428 in `handleInventory`, add throwable detection:

```go
// After the existing kind detection, before appending:
throwable := false
if s.invRegistry != nil {
    if def, ok := s.invRegistry.Item(inst.ItemDefID); ok {
        // ... (existing name/kind/weight/effects logic is already here)
        throwable = def.HasTag("throwable")
    }
}
items = append(items, &gamev1.InventoryItem{
    InstanceId:     inst.InstanceID,
    Name:           name,
    Kind:           kind,
    Quantity:       int32(inst.Quantity),
    Weight:         weight * float64(inst.Quantity),
    ItemDefId:      inst.ItemDefID,
    ArmorSlot:      armorSlot,
    ArmorCategory:  armorCategory,
    EffectsSummary: effectsSummary,
    Throwable:      throwable,
})
```

- [ ] **Step 8: Run the full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -timeout 120s 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_hotbar.go internal/gameserver/grpc_service_hotbar_test.go && git commit -m "feat(hotbar): typed slot backend — CharacterSaver interface, hotbar service, display resolution, throwable items"
```

---

## Task 5: Telnet bridge — typed slot activation

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`

- [ ] **Step 1: Locate the three hotbar-related areas in `game_bridge.go`**

```bash
grep -n "currentHotbar\|WriteHotbar\|HotbarUpdate" /home/cjohannsen/src/mud/internal/frontend/handlers/game_bridge.go
```

There are three areas to update:
1. **Declaration** (~line 235-236): `currentHotbar.Store([10]string{})`
2. **Slot activation** (~line 491): `hb, _ := currentHotbar.Load().([10]string)`
3. **HotbarUpdate event handler** (~line 1079-1087): parse slots and call WriteHotbar
4. **Resize handlers** (~lines 745-764): reload and re-render hotbar

- [ ] **Step 2: Add helper functions at the bottom of `game_bridge.go`**

Add these two helpers before the last closing brace or in a logical spot:

```go
// hotbarSlotCommand converts a proto HotbarSlot to the game command it activates.
// Returns "" when slot is nil or has no ref.
func hotbarSlotCommand(slot *gamev1.HotbarSlot) string {
	if slot == nil || slot.GetRef() == "" {
		return ""
	}
	switch slot.GetKind() {
	case "feat", "technology", "consumable":
		return "use " + slot.GetRef()
	case "throwable":
		return "throw " + slot.GetRef()
	default: // "command" or empty kind
		return slot.GetRef()
	}
}

// hotbarLabels extracts display labels from proto slots for WriteHotbar.
// Uses display_name when available; falls back to ref.
func hotbarLabels(slots [10]*gamev1.HotbarSlot) [10]string {
	var labels [10]string
	for i, s := range slots {
		if s == nil {
			continue
		}
		if s.GetDisplayName() != "" {
			labels[i] = s.GetDisplayName()
		} else {
			labels[i] = s.GetRef()
		}
	}
	return labels
}
```

- [ ] **Step 3: Change `currentHotbar` declaration and initialization (~line 235-236)**

```go
// Replace:
var currentHotbar atomic.Value
currentHotbar.Store([10]string{})

// With:
var currentHotbar atomic.Value
currentHotbar.Store([10]*gamev1.HotbarSlot{})
```

- [ ] **Step 4: Update slot activation in `commandLoop` (~line 491)**

```go
// Replace:
hb, _ := currentHotbar.Load().([10]string)
var idx int
if line[0] == '0' {
    idx = 9
} else {
    idx = int(line[0] - '1')
}
stored := hb[idx]
if stored == "" {
    // ... unassigned message
    continue
}
// Replace input with stored command and fall through to parse.
line = stored

// With:
hb, _ := currentHotbar.Load().([10]*gamev1.HotbarSlot)
var idx int
if line[0] == '0' {
    idx = 9
} else {
    idx = int(line[0] - '1')
}
cmd := hotbarSlotCommand(hb[idx])
if cmd == "" {
    slotNum := idx + 1
    msg := fmt.Sprintf("Slot %d is unassigned.", slotNum)
    if conn.IsSplitScreen() {
        _ = conn.WriteConsole(telnet.Colorize(telnet.Dim, msg))
        _ = conn.WritePromptSplit(session.CurrentPrompt())
    } else {
        _ = conn.WriteLine(msg)
        _ = conn.WritePrompt(session.CurrentPrompt())
    }
    continue
}
line = cmd
```

- [ ] **Step 5: Update the `HotbarUpdate` event handler (~line 1079)**

```go
// Replace:
case *gamev1.ServerEvent_HotbarUpdate:
    slots := p.HotbarUpdate.GetSlots()
    var arr [10]string
    for i := 0; i < len(slots) && i < 10; i++ {
        arr[i] = slots[i]
    }
    currentHotbar.Store(arr)
    if conn.IsSplitScreen() {
        _ = conn.WriteHotbar(arr)
    }
    continue

// With:
case *gamev1.ServerEvent_HotbarUpdate:
    protoSlots := p.HotbarUpdate.GetSlots()
    var arr [10]*gamev1.HotbarSlot
    for i := 0; i < len(protoSlots) && i < 10; i++ {
        arr[i] = protoSlots[i]
    }
    currentHotbar.Store(arr)
    if conn.IsSplitScreen() {
        _ = conn.WriteHotbar(hotbarLabels(arr))
    }
    continue
```

- [ ] **Step 6: Update the resize handler calls (~lines 745-764)**

Find all occurrences of:
```go
hb, _ := currentHotbar.Load().([10]string)
_ = conn.WriteHotbar(hb)
```

Replace each with:
```go
hb, _ := currentHotbar.Load().([10]*gamev1.HotbarSlot)
_ = conn.WriteHotbar(hotbarLabels(hb))
```

- [ ] **Step 7: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./internal/frontend/... 2>&1 | head -20
```

Expected: no errors.

- [ ] **Step 8: Run frontend tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/... -timeout 60s 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/game_bridge.go && git commit -m "feat(hotbar): telnet bridge uses typed HotbarSlot for activation and display"
```

---

## Task 6: TypeScript proto types + GameContext

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts`
- Modify: `cmd/webclient/ui/src/game/GameContext.tsx`

- [ ] **Step 1: Add `HotbarSlot` interface and update `InventoryItem` in `proto/index.ts`**

Add `HotbarSlot` near the top of the file, after the existing simple interfaces:

```typescript
// HotbarSlot carries a typed hotbar slot reference.
export interface HotbarSlot {
  kind: string        // "command" | "feat" | "technology" | "throwable" | "consumable"
  ref: string         // command text or item/feat/tech ID
  displayName?: string
  display_name?: string
  description?: string
}
```

Update `InventoryItem` to add `throwable`:

```typescript
export interface InventoryItem {
  instanceId?: string
  name: string
  kind?: string
  quantity?: number
  weight?: number
  itemDefId?: string
  item_def_id?: string
  armorSlot?: string
  armor_slot?: string
  armorCategory?: string
  armor_category?: string
  effectsSummary?: string
  effects_summary?: string
  throwable?: boolean
}
```

- [ ] **Step 2: Update `GameContext.tsx` — GameState, Action, reducer, initialState, handler**

**a) Import `HotbarSlot`** at the top of `GameContext.tsx`:
```typescript
import type {
  // ... existing imports ...
  HotbarSlot,
} from '../proto'
```

**b) Change `GameState.hotbarSlots`** type:
```typescript
// Replace:
hotbarSlots: string[]

// With:
hotbarSlots: HotbarSlot[]
```

**c) Change `SET_HOTBAR` action type**:
```typescript
// Replace:
| { type: 'SET_HOTBAR'; slots: string[] }

// With:
| { type: 'SET_HOTBAR'; slots: HotbarSlot[] }
```

**d) Reducer case is unchanged** (already uses `action.slots`).

**e) Change `initialState.hotbarSlots`**:
```typescript
// Replace:
hotbarSlots: Array(10).fill(''),

// With:
hotbarSlots: Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[],
```

**f) Update `HotbarUpdate` case** in the WebSocket handler (~line 421):
```typescript
// Replace:
case 'HotbarUpdate': {
  const hu = payload as { slots?: string[] }
  dispatch({ type: 'SET_HOTBAR', slots: Array.isArray(hu.slots) ? hu.slots : Array(10).fill('') })
  break
}

// With:
case 'HotbarUpdate': {
  const hu = payload as { slots?: HotbarSlot[] }
  const slots = Array.isArray(hu.slots)
    ? hu.slots
    : (Array(10).fill({ kind: 'command', ref: '' }) as HotbarSlot[])
  dispatch({ type: 'SET_HOTBAR', slots })
  break
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run type-check 2>&1 | tail -30
```

If `type-check` script doesn't exist:
```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npx tsc --noEmit 2>&1 | tail -30
```

Expected: no errors (there will be errors in HotbarPanel and drawers — those are fixed in Tasks 7–10; if they block, fix them in this task).

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/proto/index.ts cmd/webclient/ui/src/game/GameContext.tsx && git commit -m "feat(hotbar): TypeScript HotbarSlot type; update GameContext to typed slots"
```

---

## Task 7: `HotbarPanel.tsx` — typed display, activation, and tooltips

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx`

- [ ] **Step 1: Rewrite `HotbarPanel.tsx`**

The key changes:
- `hotbarSlots: HotbarSlot[]` (already set by GameContext)
- `activate(idx)`: compute command from slot kind+ref (mirrors server logic)
- Display: `slot.displayName || slot.display_name || slot.ref || '—'`
- Tooltip: typed slots show `displayName + description`; command slots show raw `ref`
- Edit popup: for `command` kind, allow changing the `ref` text; for typed slots, edit popup can clear only (typed assignment is UI-driven)

```typescript
// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked.
// REQ-HCA-6: telnet label uses display_name (done server-side via WriteHotbar labels).
// REQ-HCA-7: web slot displays display_name (not raw ref).
// REQ-HCA-8: hover tooltip shows display_name + description for typed; raw ref for command.
import { useEffect, useRef, useState } from 'react'
import { useGame } from '../GameContext'
import type { HotbarSlot } from '../../proto'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

const COMBAT_TARGET_CMDS = new Set([
  'attack', 'att', 'kill',
  'strike', 'st',
  'burst', 'bf',
])

function slotActivationCommand(slot: HotbarSlot): string {
  if (!slot.ref) return ''
  switch (slot.kind) {
    case 'feat':
    case 'technology':
    case 'consumable':
      return `use ${slot.ref}`
    case 'throwable':
      return `throw ${slot.ref}`
    default: // 'command' or ''
      return slot.ref
  }
}

function slotDisplayLabel(slot: HotbarSlot): string {
  return slot.displayName ?? slot.display_name ?? slot.ref ?? ''
}

function slotTooltip(slot: HotbarSlot): string {
  if (!slot.ref) return 'empty'
  if (slot.kind === 'command' || !slot.kind) {
    return `${slot.ref} (right-click to edit)`
  }
  const name = slot.displayName ?? slot.display_name ?? slot.ref
  const desc = slot.description ? `\n${slot.description}` : ''
  return `${name}${desc}\n(right-click to edit)`
}

interface EditPopupProps {
  slotIndex: number
  slot: HotbarSlot
  onSave: (slot: number, text: string) => void
  onClear: (slot: number) => void
  onCancel: () => void
}

function EditPopup({ slotIndex, slot, onSave, onClear, onCancel }: EditPopupProps) {
  const isCommand = !slot.kind || slot.kind === 'command'
  const [text, setText] = useState(isCommand ? (slot.ref ?? '') : '')
  const inputRef = useRef<HTMLInputElement>(null)
  const slotKey = KEYS[slotIndex]

  useEffect(() => {
    if (isCommand) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [isCommand])

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && isCommand) {
      e.preventDefault()
      if (text.trim()) onSave(slotIndex + 1, text.trim())
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  const label = isCommand ? `Slot ${slotKey}` : `Slot ${slotKey}: ${slotDisplayLabel(slot)}`

  return (
    <div style={styles.popupOverlay} onClick={onCancel}>
      <div style={styles.popup} onClick={(e) => e.stopPropagation()}>
        <div style={styles.popupTitle}>{label}</div>
        {isCommand && (
          <input
            ref={inputRef}
            style={styles.popupInput}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="command…"
            spellCheck={false}
          />
        )}
        {!isCommand && (
          <div style={{ color: '#888', fontSize: '0.78rem', fontFamily: 'monospace' }}>
            {slot.description ?? 'Typed slot — use the Feats/Tech/Inventory drawer to reassign.'}
          </div>
        )}
        <div style={styles.popupButtons}>
          {isCommand && (
            <button
              style={styles.saveBtn}
              onClick={() => { if (text.trim()) onSave(slotIndex + 1, text.trim()) }}
              disabled={!text.trim()}
              type="button"
            >
              Save
            </button>
          )}
          <button style={styles.clearBtn} onClick={() => onClear(slotIndex + 1)} type="button">
            Clear
          </button>
          <button style={styles.cancelBtn} onClick={onCancel} type="button">
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

export function HotbarPanel() {
  const { state, sendCommand, sendMessage } = useGame()
  const { hotbarSlots, combatRound, characterInfo } = state
  const [editingSlot, setEditingSlot] = useState<number | null>(null)

  function activate(idx: number) {
    const slot = hotbarSlots[idx]
    if (!slot?.ref) return

    let cmd = slotActivationCommand(slot)
    if (!cmd) return

    // Auto-fill combat target when the command has no argument and we're in combat.
    if (combatRound) {
      const words = cmd.trim().split(/\s+/)
      const verb = words[0].toLowerCase()
      if (words.length === 1 && COMBAT_TARGET_CMDS.has(verb)) {
        const playerName = characterInfo?.name ?? ''
        const turnOrder = combatRound.turnOrder ?? combatRound.turn_order ?? []
        const target = turnOrder.find((n) => n !== playerName)
        if (target) {
          cmd = `${cmd} ${target}`
        }
      }
    }

    sendCommand(cmd)
  }

  function handleSave(slot: number, text: string) {
    sendMessage('HotbarRequest', { action: 'set', slot, text })
    setEditingSlot(null)
  }

  function handleClear(slot: number) {
    sendMessage('HotbarRequest', { action: 'clear', slot })
    setEditingSlot(null)
  }

  return (
    <>
      {editingSlot !== null && (
        <EditPopup
          slotIndex={editingSlot}
          slot={hotbarSlots[editingSlot] ?? { kind: 'command', ref: '' }}
          onSave={handleSave}
          onClear={handleClear}
          onCancel={() => setEditingSlot(null)}
        />
      )}
      <div className="hotbar">
        {KEYS.map((key, i) => {
          const slot = hotbarSlots[i] ?? { kind: 'command', ref: '' }
          const label = slotDisplayLabel(slot)
          const isEmpty = !slot.ref
          return (
            <button
              key={key}
              className={`hotbar-slot${isEmpty ? ' hotbar-slot-empty' : ''}`}
              onClick={() => activate(i)}
              onContextMenu={(e) => { e.preventDefault(); setEditingSlot(i) }}
              title={slotTooltip(slot)}
              type="button"
            >
              <span className="hotbar-key">{key}</span>
              <span className="hotbar-label">{label || '—'}</span>
            </button>
          )
        })}
      </div>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  popupOverlay: {
    position: 'fixed',
    inset: 0,
    zIndex: 200,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex',
    alignItems: 'flex-end',
    justifyContent: 'center',
    paddingBottom: '60px',
  },
  popup: {
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '6px',
    padding: '0.75rem',
    minWidth: '260px',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    fontFamily: 'monospace',
  },
  popupTitle: { color: '#7af', fontSize: '0.75rem', textTransform: 'uppercase', letterSpacing: '0.08em' },
  popupInput: {
    background: '#111',
    border: '1px solid #555',
    borderRadius: '3px',
    color: '#e0c060',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    padding: '0.3rem 0.5rem',
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box' as const,
  },
  popupButtons: { display: 'flex', gap: '0.4rem' },
  saveBtn: { padding: '0.2rem 0.6rem', background: '#1a2a1a', border: '1px solid #4a6a2a', color: '#8d4', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem' },
  clearBtn: { padding: '0.2rem 0.6rem', background: '#2a1a1a', border: '1px solid #5a2a2a', color: '#c66', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem' },
  cancelBtn: { padding: '0.2rem 0.6rem', background: 'none', border: '1px solid #444', color: '#666', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.78rem', marginLeft: 'auto' },
}
```

- [ ] **Step 2: Build the web UI**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -20
```

Expected: exits 0 (may have TS errors in drawers until Tasks 8–10 fix them).

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/game/panels/HotbarPanel.tsx && git commit -m "feat(hotbar): HotbarPanel typed activation, display names, and hover tooltips"
```

---

## Task 8: `FeatsDrawer.tsx` — send `feat` typed slot

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/FeatsDrawer.tsx`

- [ ] **Step 1: Update `SlotPicker` props and `FeatItem` in `FeatsDrawer.tsx`**

**a) Update the `SlotPicker` component** to accept `HotbarSlot[]` instead of `string[]`:

```typescript
import type { FeatEntry, HotbarSlot } from '../../proto'

// Replace SlotPicker props:
function SlotPicker({
  hotbarSlots,
  onPick,
  onCancel,
}: {
  hotbarSlots: HotbarSlot[]
  onPick: (slot: number) => void
  onCancel: () => void
}) {
  return createPortal(
    <div style={styles.slotPickerOverlay} onClick={onCancel}>
      <div style={styles.slotPickerModal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.slotPickerHeader}>
          <span style={styles.slotPickerLabel}>Add to Hotbar</span>
          <button style={styles.cancelBtn} onClick={onCancel} type="button">✕</button>
        </div>
        <div style={styles.slotPickerGrid}>
          {SLOT_KEYS.map((key, i) => {
            const slot = hotbarSlots[i]
            const current = slot?.ref ?? ''
            const label = slot?.displayName ?? slot?.display_name ?? current
            return (
              <button
                key={key}
                style={{ ...styles.slotBtn, ...(current ? styles.slotBtnOccupied : {}) }}
                onClick={() => onPick(i + 1)}
                title={current ? `Replace: ${label}` : `Slot ${key} (empty)`}
                type="button"
              >
                <span style={styles.slotBtnKey}>{key}</span>
                {current && <span style={styles.slotBtnCurrent}>{label}</span>}
              </button>
            )
          })}
        </div>
      </div>
    </div>,
    document.body
  )
}
```

**b) Update `FeatItem` props and `handlePick`**:

```typescript
function FeatItem({
  feat,
  hotbarSlots,
  sendMessage,
}: {
  feat: FeatEntry
  hotbarSlots: HotbarSlot[]
  sendMessage: (type: string, payload: object) => void
}) {
  const [picking, setPicking] = useState(false)

  function handlePick(slot: number) {
    const ref = feat.featId ?? feat.feat_id ?? ''
    sendMessage('HotbarRequest', { action: 'set', slot, kind: 'feat', ref })
    setPicking(false)
  }

  // ... rest of FeatItem JSX unchanged ...
}
```

**c) Update `FeatsDrawer` top-level** — propagate `HotbarSlot[]`:

```typescript
export function FeatsDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage } = useGame()
  // ...
  return (
    // ... pass state.hotbarSlots (now HotbarSlot[]) to FeatItem
  )
}
```

The `state.hotbarSlots` is now `HotbarSlot[]` so no cast needed — just pass through.

- [ ] **Step 2: Verify TypeScript compilation**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npx tsc --noEmit 2>&1 | grep "FeatsDrawer" | head -10
```

Expected: no errors in FeatsDrawer.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/game/drawers/FeatsDrawer.tsx && git commit -m "feat(hotbar): FeatsDrawer sends feat-typed hotbar slot"
```

---

## Task 9: `TechnologyDrawer.tsx` — send `technology` typed slot

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx`

- [ ] **Step 1: Update `SlotPicker` and tech item hotbar handlers in `TechnologyDrawer.tsx`**

The pattern is identical to FeatsDrawer. There are multiple `SlotPicker` usages in TechnologyDrawer (prepared tech, spontaneous tech, innate tech). Update all of them.

**a) Import `HotbarSlot`**:
```typescript
import type { HotbarSlot } from '../../proto'
```

**b) Update each `SlotPicker` props** from `hotbarSlots: string[]` to `hotbarSlots: HotbarSlot[]`:

The SlotPicker component in TechnologyDrawer is identical in structure to FeatsDrawer. Apply the same changes: `current` from `hotbarSlots[i] ?? ''` → `hotbarSlots[i]?.ref ?? ''`; label from `current` → `slot?.displayName ?? slot?.display_name ?? current`.

**c) Update each `handlePick` function** for prepared/spontaneous/innate tech:

Find all occurrences of:
```typescript
sendMessage('HotbarRequest', { action: 'set', slot: s, text: `use ${shortName || techId}` })
```

Replace with:
```typescript
sendMessage('HotbarRequest', { action: 'set', slot: s, kind: 'technology', ref: techId })
```

The display name is now resolved server-side, so the `shortName` is no longer needed for hotbar assignment.

**d) Update all component props** that accept `hotbarSlots: string[]` → `hotbarSlots: HotbarSlot[]`.

- [ ] **Step 2: Verify TypeScript compilation**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npx tsc --noEmit 2>&1 | grep "TechnologyDrawer" | head -10
```

Expected: no errors in TechnologyDrawer.

- [ ] **Step 3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/game/drawers/TechnologyDrawer.tsx && git commit -m "feat(hotbar): TechnologyDrawer sends technology-typed hotbar slot"
```

---

## Task 10: `InventoryDrawer.tsx` — throwable + consumable hotbar buttons

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`

- [ ] **Step 1: Add `SlotPicker` component to `InventoryDrawer.tsx`**

Add a local `SlotPicker` at the top of the file (before `ConsumableRow`). This is the same design as in FeatsDrawer/TechnologyDrawer, but it needs to accept `HotbarSlot[]`:

```typescript
import type { InventoryItem, HotbarSlot } from '../../proto'

const SLOT_KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

function SlotPicker({
  hotbarSlots,
  onPick,
  onCancel,
}: {
  hotbarSlots: HotbarSlot[]
  onPick: (slot: number) => void
  onCancel: () => void
}) {
  return (
    <div style={slotPickerStyles.overlay} onClick={onCancel}>
      <div style={slotPickerStyles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={slotPickerStyles.header}>
          <span>Add to Hotbar</span>
          <button onClick={onCancel} type="button">✕</button>
        </div>
        <div style={slotPickerStyles.grid}>
          {SLOT_KEYS.map((key, i) => {
            const slot = hotbarSlots[i]
            const current = slot?.ref ?? ''
            const label = slot?.displayName ?? slot?.display_name ?? current
            return (
              <button
                key={key}
                style={{ padding: '0.2rem 0.4rem', background: current ? '#2a2a1a' : '#111', border: '1px solid #444', color: current ? '#e0c060' : '#555', borderRadius: '3px', cursor: 'pointer', fontFamily: 'monospace', fontSize: '0.75rem', minWidth: '60px' }}
                onClick={() => onPick(i + 1)}
                title={current ? `Replace: ${label}` : `Slot ${key} (empty)`}
                type="button"
              >
                {key}{current ? `: ${label.slice(0, 8)}` : ''}
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}

const slotPickerStyles: Record<string, React.CSSProperties> = {
  overlay: { position: 'fixed', inset: 0, zIndex: 300, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center' },
  modal: { background: '#1a1a1a', border: '1px solid #444', borderRadius: '6px', padding: '0.75rem', minWidth: '280px', fontFamily: 'monospace' },
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem', color: '#7af', fontSize: '0.75rem', textTransform: 'uppercase' as const },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '0.3rem' },
}
```

- [ ] **Step 2: Update `ConsumableRow` to add "Add to hotbar" button**

```typescript
function ConsumableRow({
  item,
  sendCommand,
  sendMessage,
  hotbarSlots,
}: {
  item: InventoryItem
  sendCommand: (raw: string) => void
  sendMessage: (type: string, payload: object) => void
  hotbarSlots: HotbarSlot[]
}) {
  const itemDefId = item.itemDefId ?? item.item_def_id ?? ''
  const qty = item.quantity ?? 1
  const [picking, setPicking] = useState(false)

  function handleConsume() {
    sendCommand(`use ${itemDefId}`)
    sendMessage('InventoryRequest', {})
  }

  function handleHotbarPick(slot: number) {
    sendMessage('HotbarRequest', { action: 'set', slot, kind: 'consumable', ref: itemDefId })
    setPicking(false)
  }

  return (
    <>
      {picking && (
        <SlotPicker hotbarSlots={hotbarSlots} onPick={handleHotbarPick} onCancel={() => setPicking(false)} />
      )}
      <tr>
        <td title={item.effectsSummary ?? item.effects_summary ?? undefined}>{item.name}</td>
        <td>{item.kind}</td>
        <td>{qty}</td>
        <td>{(item.weight ?? 0).toFixed(1)}</td>
        <td style={{ display: 'flex', gap: '0.3rem', flexWrap: 'wrap' as const }}>
          <button
            style={{ ...styles.actionBtn, background: '#a74', ...(qty <= 0 ? styles.actionBtnDisabled : {}) }}
            disabled={qty <= 0}
            onClick={handleConsume}
            type="button"
            title={item.effectsSummary ?? item.effects_summary ?? `Consume ${item.name}`}
          >
            Consume
          </button>
          <button
            style={{ ...styles.actionBtn, background: '#2a3a4a' }}
            onClick={() => setPicking(true)}
            type="button"
            title={`Add ${item.name} to hotbar`}
          >
            + Hotbar
          </button>
        </td>
      </tr>
    </>
  )
}
```

- [ ] **Step 3: Add `ThrowableRow` component**

```typescript
function ThrowableRow({
  item,
  sendCommand,
  sendMessage,
  hotbarSlots,
}: {
  item: InventoryItem
  sendCommand: (raw: string) => void
  sendMessage: (type: string, payload: object) => void
  hotbarSlots: HotbarSlot[]
}) {
  const itemDefId = item.itemDefId ?? item.item_def_id ?? ''
  const qty = item.quantity ?? 1
  const [picking, setPicking] = useState(false)

  function handleThrow() {
    sendCommand(`throw ${itemDefId}`)
  }

  function handleHotbarPick(slot: number) {
    sendMessage('HotbarRequest', { action: 'set', slot, kind: 'throwable', ref: itemDefId })
    setPicking(false)
  }

  return (
    <>
      {picking && (
        <SlotPicker hotbarSlots={hotbarSlots} onPick={handleHotbarPick} onCancel={() => setPicking(false)} />
      )}
      <tr>
        <td>{item.name}</td>
        <td>{item.kind}</td>
        <td>{qty}</td>
        <td>{(item.weight ?? 0).toFixed(1)}</td>
        <td style={{ display: 'flex', gap: '0.3rem', flexWrap: 'wrap' as const }}>
          <button
            style={{ ...styles.actionBtn, background: '#6a3a2a', ...(qty <= 0 ? styles.actionBtnDisabled : {}) }}
            disabled={qty <= 0}
            onClick={handleThrow}
            type="button"
          >
            Throw
          </button>
          <button
            style={{ ...styles.actionBtn, background: '#2a3a4a' }}
            onClick={() => setPicking(true)}
            type="button"
            title={`Add ${item.name} to hotbar`}
          >
            + Hotbar
          </button>
        </td>
      </tr>
    </>
  )
}
```

- [ ] **Step 4: Update `InventoryDrawer` to pass `hotbarSlots` and render `ThrowableRow`**

In the `InventoryDrawer` function, update the render logic to:
1. Pass `hotbarSlots` down to `ConsumableRow`
2. Detect `item.throwable` and render `ThrowableRow` instead of generic row

```typescript
export function InventoryDrawer({ onClose }: { onClose: () => void }) {
  const { state, sendMessage, sendCommand } = useGame()
  const { hotbarSlots } = state
  // ... existing useEffect ...

  return (
    // ... existing structure, update row rendering:
    // For each item in inventoryView.items:
    //   if item.kind === 'consumable': <ConsumableRow ... hotbarSlots={hotbarSlots} />
    //   else if item.throwable:         <ThrowableRow  ... hotbarSlots={hotbarSlots} />
    //   else:                           existing rows (WeaponRow, ArmorRow, etc.)
  )
}
```

In the actual item rendering section (find `item.kind === 'consumable'` check at ~line 208):

```typescript
if (item.kind === 'consumable') {
  return (
    <ConsumableRow key={i} item={item} sendCommand={sendCommand} sendMessage={sendMessage} hotbarSlots={hotbarSlots} />
  )
}
if (item.throwable) {
  return (
    <ThrowableRow key={i} item={item} sendCommand={sendCommand} sendMessage={sendMessage} hotbarSlots={hotbarSlots} />
  )
}
```

- [ ] **Step 5: Verify TypeScript compilation**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npx tsc --noEmit 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 6: Full web build**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -10
```

Expected: exits 0.

- [ ] **Step 7: Run the full Go test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... -timeout 120s 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx && git commit -m "feat(hotbar): InventoryDrawer adds throwable and consumable typed hotbar slots"
```

---

## Post-Implementation Checklist (spec compliance)

| Requirement | Covered by |
|-------------|-----------|
| REQ-HCA-1: Five slot kinds including command backward compat | Task 1 (domain type), Task 4 (backend) |
| REQ-HCA-2: feat slot → `use <id>` | Tasks 1, 4, 8 |
| REQ-HCA-3: technology slot → `use <id>` | Tasks 1, 4, 9 |
| REQ-HCA-4: throwable slot → `throw <id>` | Tasks 1, 4, 10 |
| REQ-HCA-5: consumable slot → `use <id>` | Tasks 1, 4, 10 |
| REQ-HCA-6: telnet label uses display name | Task 5 (hotbarLabels) |
| REQ-HCA-7: web slot shows display name | Task 7 (HotbarPanel) |
| REQ-HCA-8: hover tooltip shows name+description for typed; raw command for command | Task 7 (slotTooltip) |
| REQ-HCA-9: Feats tab sends feat-typed slot | Task 8 |
| REQ-HCA-10: Technologies tab sends technology-typed slot | Task 9 |
| REQ-HCA-11: Inventory throwable items have "Add to hotbar" | Task 10 |
| REQ-HCA-12: Inventory consumable items have "Add to hotbar" | Task 10 |
| REQ-HCA-13: `hotbar <slot> <text>` still assigns command slot | Task 4 (backward compat in buildHotbarSlot) |
| REQ-HCA-14: DB migrates old format on load | Task 3 |
| REQ-HCA-15: proto HotbarSlot + HotbarUpdateEvent change | Task 2 |
| REQ-HCA-16: Tests for each slot kind activation, display name, DB round-trip | Tasks 1, 3, 4 |
