# Non-Combat NPCs — Guard + Hireling (Sub-Project 4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Guard (bribeable config + WantedThreshold-aware aggression) and Hireling (hire/dismiss commands, zone follow, daily cost, combat ally) non-combat NPC types with named NPC YAML for Rustbucket Ridge.

**Architecture:** Guard extends `GuardConfig` with `Bribeable`/`MaxBribeWantedLevel`/`BaseCosts` fields validated in `Template.Validate()`. Guard aggression respects each guard's `WantedThreshold` and emits a watch/warn message at sub-threshold wanted levels. Hireling tracks runtime state (`HirelingRuntimeState`) in a map on `GameServiceServer` using the same mutex-guarded pattern as the healer; atomic check-and-set for hire uses a write-lock + struct field check within a single critical section (idiomatic Go — no raw `sync/atomic`). DB persistence for `HirelingRuntimeState` is deferred to a future sub-project (same deferral as `HealerRuntimeState`); runtime state is in-memory only for this sub-project. Zone follow moves the hireling instance via `npcMgr.Move` on every player room transition. Daily cost is charged on the calendar tick's `Hour == 0` block. REQ-NPC-8 adds a player-owns-hireling guard in `CombatHandler.Attack`.

**Tech Stack:** Go 1.26, `github.com/stretchr/testify`, `pgregory.net/rapid`

---

## Prerequisite: Understand existing shape

Before starting any task, confirm the following already exist:
- `GuardConfig{WantedThreshold int, PatrolRoom string}` in `internal/game/npc/noncombat.go`
- `HirelingConfig{DailyCost int, CombatRole string, MaxFollowZones int}` in same file
- `HirelingRuntimeState{HiredByPlayerID string, ZonesFollowed int}` in same file
- `CombatHandler.InitiateGuardCombat` in `internal/gameserver/combat_handler.go` — already finds guard instances in room and calls `h.Attack(guardID, uid)` for each
- Room-entry guard check in `grpc_service.go`: `if wantedLevel >= 2 { s.combatH.InitiateGuardCombat(...) }`
- `defaultCombatResponse`: `"guard"` and `"hireling"` both map to `"engage"`
- Attack target check: `inst.NPCType != "guard" && inst.NPCType != "hireling"` already allows both as targets
- `s.npcMgr.Move(id, newRoomID string) error` in `internal/game/npc/manager.go`
- `s.npcMgr.FindInRoom(roomID, target string) *Instance` in `manager.go` — confirmed exists
- `s.npcH` (`*NPCHandler`) and `s.npcMgr` (`*npc.Manager`) both exist on `GameServiceServer`; service handler files (healer, job trainer) use `s.npcMgr` directly — follow that pattern
- Proto fields 94–98 already used (heal + job trainer); next field is 99
- Daily tick file: confirm before editing with `grep -n "Hour == 0\|tickHealerCapacity\|StartNPCTickHook" /home/cjohannsen/src/mud/internal/gameserver/grpc_service_npc_ticks.go | head -10`

---

## File Map

### New files
| File | Purpose |
|------|---------|
| `internal/gameserver/grpc_service_hireling.go` | Hireling runtime state map, hire/dismiss handlers, zone-follow on room move, daily cost tick |
| `internal/gameserver/grpc_service_hireling_test.go` | TDD tests for hire/dismiss/follow/tick |
| `content/npcs/marshal_ironsides.yaml` | Named guard NPC, Safe room in Rustbucket Ridge |
| `content/npcs/patch.yaml` | Named hireling NPC, Rustbucket Ridge |

### Modified files
| File | Change |
|------|--------|
| `internal/game/npc/noncombat.go` | Add `Bribeable bool`, `MaxBribeWantedLevel int`, `BaseCosts map[int]int` to `GuardConfig`; add `GuardConfig.Validate()` |
| `internal/game/npc/noncombat_test.go` | Add tests for `GuardConfig.Validate()` |
| `internal/game/npc/template.go` | Call `GuardConfig.Validate()` in `Template.Validate()` when `npc_type == "guard"` |
| `internal/game/npc/template_test.go` | Add guard config validation tests |
| `internal/gameserver/combat_handler.go` | Modify `InitiateGuardCombat` to compare per-guard `WantedThreshold`; emit watch/warn at sub-threshold; add player-owns-hireling guard in `Attack` |
| `internal/gameserver/combat_handler_guard_test.go` | Add WantedThreshold tests and watch/warn tests |
| `internal/gameserver/grpc_service.go` | Add `hirelingRuntimeStates map[string]*npc.HirelingRuntimeState`; wire hire/dismiss dispatch cases; emit guard watch/warn on room entry; call `tickHirelingDailyCost` on daily tick |
| `api/proto/game/v1/game.proto` | Add `HireRequest`, `DismissRequest` messages at fields 99–100 |
| `internal/gameserver/gamev1/` | Regenerate proto (`mise exec -- buf generate`) |

---

## Task 1 — GuardConfig Bribeable fields + Validate

**Files:**
- Modify: `internal/game/npc/noncombat.go`
- Modify: `internal/game/npc/noncombat_test.go`

### Steps

- [ ] **1a. Write failing tests** in `internal/game/npc/noncombat_test.go`:

```go
// TestGuardConfig_Validate_BribeableWithoutMaxLevel verifies fatal error when
// Bribeable is true and MaxBribeWantedLevel is zero (default).
//
// REQ-WC-2b: MaxBribeWantedLevel MUST be in range 1-4 when Bribeable is true.
func TestGuardConfig_Validate_BribeableWithoutMaxLevel(t *testing.T) {
	cfg := &npc.GuardConfig{
		WantedThreshold:    2,
		Bribeable:          true,
		MaxBribeWantedLevel: 0, // invalid
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_bribe_wanted_level")
}

// TestGuardConfig_Validate_BribeableMaxLevelOutOfRange verifies fatal error when
// MaxBribeWantedLevel is 5.
func TestGuardConfig_Validate_BribeableMaxLevelOutOfRange(t *testing.T) {
	cfg := &npc.GuardConfig{
		WantedThreshold:    2,
		Bribeable:          true,
		MaxBribeWantedLevel: 5,
		BaseCosts:          map[int]int{1: 100, 2: 200, 3: 300, 4: 400},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_bribe_wanted_level")
}

// TestGuardConfig_Validate_BribeableMissingBaseCostKey verifies fatal error when
// BaseCosts is missing a required key.
func TestGuardConfig_Validate_BribeableMissingBaseCostKey(t *testing.T) {
	cfg := &npc.GuardConfig{
		WantedThreshold:    2,
		Bribeable:          true,
		MaxBribeWantedLevel: 2,
		BaseCosts:          map[int]int{1: 100, 2: 200, 3: 300}, // missing key 4
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_costs")
}

// TestGuardConfig_Validate_BribeableZeroBaseCostValue verifies fatal error when
// a BaseCosts value is zero or negative.
func TestGuardConfig_Validate_BribeableZeroBaseCostValue(t *testing.T) {
	cfg := &npc.GuardConfig{
		WantedThreshold:    2,
		Bribeable:          true,
		MaxBribeWantedLevel: 2,
		BaseCosts:          map[int]int{1: 100, 2: 0, 3: 300, 4: 400},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_costs")
}

// TestGuardConfig_Validate_ValidBribeable verifies no error for a valid bribeable config.
func TestGuardConfig_Validate_ValidBribeable(t *testing.T) {
	cfg := &npc.GuardConfig{
		WantedThreshold:    2,
		Bribeable:          true,
		MaxBribeWantedLevel: 2,
		BaseCosts:          map[int]int{1: 100, 2: 200, 3: 300, 4: 400},
	}
	assert.NoError(t, cfg.Validate())
}

// TestGuardConfig_Validate_NonBribeableNoBases verifies no error for non-bribeable guard.
func TestGuardConfig_Validate_NonBribeableNoBases(t *testing.T) {
	cfg := &npc.GuardConfig{WantedThreshold: 2}
	assert.NoError(t, cfg.Validate())
}
```

- [ ] **1b. Run tests to verify they fail:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestGuardConfig_Validate" -v 2>&1 | head -15
```
Expected: FAIL (Validate method doesn't exist yet).

- [ ] **1c. Implement** — add to `GuardConfig` struct in `internal/game/npc/noncombat.go` after `PatrolRoom`:

```go
// GuardConfig holds the static configuration for a guard NPC.
type GuardConfig struct {
	WantedThreshold     int        `yaml:"wanted_threshold"`
	PatrolRoom          string     `yaml:"patrol_room,omitempty"`
	// Bribeable indicates whether this guard can be bribed to ignore a Wanted level.
	// REQ-WC-2b: when Bribeable is true, MaxBribeWantedLevel MUST be in range 1-4.
	Bribeable           bool       `yaml:"bribeable"`
	// MaxBribeWantedLevel is the highest WantedLevel this guard will accept a bribe for.
	// Required when Bribeable is true. Default: 2.
	MaxBribeWantedLevel int        `yaml:"max_bribe_wanted_level"`
	// BaseCosts maps WantedLevel (1-4) to the base bribe cost in credits.
	// Required when Bribeable is true. All keys 1-4 must be present with positive values.
	BaseCosts           map[int]int `yaml:"base_costs,omitempty"`
}

// Validate checks REQ-WC-2b: when Bribeable is true, MaxBribeWantedLevel must be
// in range 1-4 and BaseCosts must contain all keys 1-4 with positive values.
//
// Precondition: cfg must not be nil.
// Postcondition: Returns nil iff all bribeable constraints are satisfied.
func (cfg *GuardConfig) Validate() error {
	if !cfg.Bribeable {
		return nil
	}
	if cfg.MaxBribeWantedLevel < 1 || cfg.MaxBribeWantedLevel > 4 {
		return fmt.Errorf("guard: max_bribe_wanted_level must be in range 1-4 when bribeable (got %d)", cfg.MaxBribeWantedLevel)
	}
	for _, level := range []int{1, 2, 3, 4} {
		cost, ok := cfg.BaseCosts[level]
		if !ok {
			return fmt.Errorf("guard: base_costs must contain key %d when bribeable", level)
		}
		if cost <= 0 {
			return fmt.Errorf("guard: base_costs[%d] must be positive (got %d)", level, cost)
		}
	}
	return nil
}
```

**Note:** The existing `GuardConfig` struct only has `WantedThreshold` and `PatrolRoom`. Replace the entire struct definition with the above (which adds the three new fields and the `Validate()` method).

- [ ] **1d. Wire into `Template.Validate()`** in `internal/game/npc/template.go` — find the block that handles `npc_type == "guard"` and add a `cfg.Guard.Validate()` call. Search for `"guard"` in the existing validate function:

```bash
grep -n "guard\|Guard" /home/cjohannsen/src/mud/internal/game/npc/template.go | head -20
```

In the existing guard case in `Validate()`, add after the nil check:

```go
case "guard":
    if t.Guard == nil {
        return fmt.Errorf("npc template %q: npc_type is \"guard\" but guard config is missing", t.ID)
    }
    if err := t.Guard.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
```

Read `template.go` first to see the exact existing code before editing.

- [ ] **1e. Add template validation tests** — in `internal/game/npc/template_test.go`, add:

```go
// TestTemplate_Validate_BribeableGuard_Invalid verifies fatal error for invalid bribeable guard.
func TestTemplate_Validate_BribeableGuard_Invalid(t *testing.T) {
	tmpl := &npc.Template{
		ID: "bad_guard", Name: "Bad Guard", NPCType: "guard",
		Level: 2, MaxHP: 30, AC: 14,
		Guard: &npc.GuardConfig{
			WantedThreshold:    2,
			Bribeable:          true,
			MaxBribeWantedLevel: 0, // invalid
		},
	}
	err := tmpl.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_bribe_wanted_level")
}

// TestTemplate_Validate_BribeableGuard_Valid verifies valid bribeable guard passes.
func TestTemplate_Validate_BribeableGuard_Valid(t *testing.T) {
	tmpl := &npc.Template{
		ID: "good_guard", Name: "Good Guard", NPCType: "guard",
		Level: 2, MaxHP: 30, AC: 14,
		Guard: &npc.GuardConfig{
			WantedThreshold:    2,
			Bribeable:          true,
			MaxBribeWantedLevel: 2,
			BaseCosts:          map[int]int{1: 100, 2: 200, 3: 300, 4: 400},
		},
	}
	assert.NoError(t, tmpl.Validate())
}
```

- [ ] **1f. Run all npc tests:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -20
```
Expected: all pass.

- [ ] **1g. Run full suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **1h. Commit:**
```bash
git add internal/game/npc/noncombat.go internal/game/npc/noncombat_test.go internal/game/npc/template.go internal/game/npc/template_test.go
git commit -m "feat: extend GuardConfig with bribeable fields + REQ-WC-2b validation"
```

---

## Task 2 — WantedThreshold-aware guard aggression + watch/warn

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/combat_handler_guard_test.go`
- Modify: `internal/gameserver/grpc_service.go`

The current `InitiateGuardCombat` initiates combat for ALL guard instances regardless of their individual `WantedThreshold`. We fix this so each guard only engages if the player's wantedLevel meets that guard's threshold.

The room-entry check in `grpc_service.go` also emits a watch/warn message when wantedLevel > 0 but below the minimum guard threshold in the room.

### Steps

- [ ] **2a. Read existing `InitiateGuardCombat`** to understand the current structure:
```bash
sed -n '272,310p' /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go
```

- [ ] **2b. Write failing tests** in `internal/gameserver/combat_handler_guard_test.go`:

```go
// TestInitiateGuardCombat_RespectsWantedThreshold verifies that a guard with
// WantedThreshold=3 does NOT engage a player with WantedLevel=2.
func TestInitiateGuardCombat_RespectsWantedThreshold(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	var broadcastCalled bool
	h := &CombatHandler{
		sessions:    sessMgr,
		npcMgr:      npcMgr,
		worldMgr:    worldMgr,
		broadcastFn: func(roomID string, _ []*gamev1.CombatEvent) { broadcastCalled = true },
		engine:      combat.NewEngine(),
	}

	uid := "guard-threshold-player"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "u", CharName: "Tester",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	// Guard with WantedThreshold=3 — should NOT engage at wantedLevel=2.
	tmpl := &npc.Template{
		ID: "strict_guard", Name: "Strict Guard", NPCType: "guard",
		Level: 3, MaxHP: 40, AC: 14,
		Guard: &npc.GuardConfig{WantedThreshold: 3},
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	h.InitiateGuardCombat(uid, "zone-1", 2) // below threshold
	assert.False(t, broadcastCalled, "guard with threshold=3 must not engage at wantedLevel=2")
}

// TestInitiateGuardCombat_EngagesAtThreshold verifies a guard with WantedThreshold=3
// engages at wantedLevel=3.
func TestInitiateGuardCombat_EngagesAtThreshold(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	var broadcastCalled bool
	h := &CombatHandler{
		sessions:    sessMgr,
		npcMgr:      npcMgr,
		worldMgr:    worldMgr,
		broadcastFn: func(roomID string, _ []*gamev1.CombatEvent) { broadcastCalled = true },
		engine:      combat.NewEngine(),
	}

	uid := "guard-threshold-player-2"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "u2", CharName: "Tester2",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID: "strict_guard2", Name: "Strict Guard 2", NPCType: "guard",
		Level: 3, MaxHP: 40, AC: 14,
		Guard: &npc.GuardConfig{WantedThreshold: 3},
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	h.InitiateGuardCombat(uid, "zone-1", 3) // at threshold
	assert.True(t, broadcastCalled, "guard with threshold=3 must engage at wantedLevel=3")
}

// TestInitiateGuardCombat_DefaultThreshold verifies a guard with WantedThreshold=0
// uses the default threshold of 2.
func TestInitiateGuardCombat_DefaultThreshold(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	var broadcastCalled bool
	h := &CombatHandler{
		sessions:    sessMgr,
		npcMgr:      npcMgr,
		worldMgr:    worldMgr,
		broadcastFn: func(roomID string, _ []*gamev1.CombatEvent) { broadcastCalled = true },
		engine:      combat.NewEngine(),
	}

	uid := "guard-default-thresh"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "u3", CharName: "Tester3",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID: "default_guard", Name: "Default Guard", NPCType: "guard",
		Level: 2, MaxHP: 30, AC: 12,
		Guard: &npc.GuardConfig{WantedThreshold: 0}, // default → 2
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	h.InitiateGuardCombat(uid, "zone-1", 2) // should engage (default threshold = 2)
	assert.True(t, broadcastCalled, "guard with default threshold must engage at wantedLevel=2")
}
```

- [ ] **2c. Run tests to verify they fail:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestInitiateGuardCombat_Respects|TestInitiateGuardCombat_EngagesAtThreshold|TestInitiateGuardCombat_DefaultThreshold" -v 2>&1 | head -20
```

- [ ] **2d. Modify `InitiateGuardCombat`** in `combat_handler.go`.

Read the existing function first (lines 272–310). Then replace the guard-filtering loop:

```go
// InitiateGuardCombat finds guard NPCs in the player's current room and starts
// combat against the player. Only guards whose WantedThreshold (default 2) is
// <= wantedLevel are engaged. wantedLevel distinguishes detain (2) from kill (3-4).
// If no eligible guard NPCs are present in the room, this is a no-op.
//
// Precondition: uid MUST be a valid player UID; wantedLevel MUST be in [2, 4].
// Postcondition: if the player session exists and eligible guard NPCs are present,
// broadcastFn is called with a narrative CombatEvent and h.Attack is invoked for each.
// If the player session is not found or no eligible guards are present, this is a no-op.
func (h *CombatHandler) InitiateGuardCombat(uid, zoneID string, wantedLevel int) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	npcs := h.npcMgr.InstancesInRoom(sess.RoomID)
	var guardIDs []string
	for _, n := range npcs {
		if n.NPCType != "guard" {
			continue
		}
		tmpl := h.npcMgr.TemplateByID(n.TemplateID)
		threshold := 2
		if tmpl != nil && tmpl.Guard != nil && tmpl.Guard.WantedThreshold > 0 {
			threshold = tmpl.Guard.WantedThreshold
		}
		if wantedLevel >= threshold {
			guardIDs = append(guardIDs, n.ID)
		}
	}
	if len(guardIDs) == 0 {
		return
	}
	var narrative string
	if wantedLevel >= 3 {
		narrative = "The guards attack on sight!"
	} else {
		narrative = "Guards shout: Drop your weapon and surrender!"
	}
	h.broadcastFn(sess.RoomID, []*gamev1.CombatEvent{
		{Type: gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK, Narrative: narrative},
	})
	for _, guardID := range guardIDs {
		_, _ = h.Attack(guardID, uid)
	}
}
```

**Note:** `h.npcMgr.TemplateByID(n.TemplateID)` — verify this method exists:
```bash
grep -n "TemplateByID\|func.*Template" /home/cjohannsen/src/mud/internal/game/npc/manager.go | head -10
```
If it doesn't exist, add it to `manager.go`:
```go
// TemplateByID returns the Template for the given template ID, or nil if not found.
func (m *Manager) TemplateByID(id string) *Template {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.templates[id]
}
```
Then check that `m.templates` is the right field name for the template map:
```bash
grep -n "templates\|Templates" /home/cjohannsen/src/mud/internal/game/npc/manager.go | head -10
```

- [ ] **2e. Add watch/warn room-entry behavior** in `grpc_service.go`.

Find the room-entry guard check (around line 1752):
```go
wantedLevel := sess.WantedLevel[newRoom.ZoneID]
if wantedLevel >= 2 && s.combatH != nil {
    s.combatH.InitiateGuardCombat(uid, newRoom.ZoneID, wantedLevel)
}
```

Replace with:
```go
wantedLevel := sess.WantedLevel[newRoom.ZoneID]
if wantedLevel > 0 && s.npcH != nil {
    if wantedLevel >= 2 && s.combatH != nil {
        s.combatH.InitiateGuardCombat(uid, newRoom.ZoneID, wantedLevel)
    } else {
        // WantedLevel 1 (Flagged): guards watch and warn the player.
        for _, inst := range s.npcH.InstancesInRoom(newRoom.RoomId) {
            if inst.NPCType == "guard" {
                s.sendMessageToPlayer(uid, fmt.Sprintf("%s eyes you suspiciously. \"We're watching you.\"", inst.Name()))
                break
            }
        }
    }
}
```

Check that `s.sendMessageToPlayer` or an equivalent helper exists:
```bash
grep -n "sendMessageToPlayer\|pushMessageToPlayer\|pushMessage\b" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -5
```

If the helper doesn't exist, use the inline push pattern from elsewhere in the file (look at how healer pushes messages to player). Use the pattern that already exists — do not invent a new helper.

- [ ] **2f. Run guard tests:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestInitiateGuardCombat" -v 2>&1 | tail -20
```
Expected: all pass including the 3 new tests.

- [ ] **2g. Run full suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **2h. Commit:**
```bash
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_guard_test.go internal/gameserver/grpc_service.go
git commit -m "feat: WantedThreshold-aware guard aggression and watch/warn at WantedLevel 1"
```

---

## Task 3 — Proto messages for hire + dismiss

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/gamev1/` (regenerated)

### Steps

- [ ] **3a. Add proto messages** to `api/proto/game/v1/game.proto`.

After field 98 (`SetJobRequest set_job = 98;`), add to the `oneof payload`:
```protobuf
HireRequest          hire            = 99;
DismissRequest       dismiss         = 100;
```

After the `SetJobRequest` message definition, add:
```protobuf
message HireRequest {
  string npc_name = 1;
}

message DismissRequest {}
```

- [ ] **3b. Regenerate proto:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- buf generate 2>&1 | head -5
```

- [ ] **3c. Build to verify:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -5
```

- [ ] **3d. Commit:**
```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add proto messages for hire and dismiss commands"
```

---

## Task 4 — Hireling runtime state infrastructure

**Files:**
- Create: `internal/gameserver/grpc_service_hireling.go`
- Create: `internal/gameserver/grpc_service_hireling_test.go`
- Modify: `internal/gameserver/grpc_service.go`

Mirror the healer pattern (`grpc_service_healer.go`). Key difference: hireling binding uses `sync/atomic` on a `*string` field on `HirelingRuntimeState` for the atomic check-and-set (REQ-NPC-15).

### Steps

- [ ] **4a. Write failing tests** in `internal/gameserver/grpc_service_hireling_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newHirelingTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcMgr)

	uid := "hl_u1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "hl_user",
		CharName:  "HLChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     50,
		Role:      "player",
		Level:     3,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	tmpl := &npc.Template{
		ID:      "test_hireling",
		Name:    "Patch",
		NPCType: "hireling",
		Level:   3,
		MaxHP:   25,
		AC:      12,
		Hireling: &npc.HirelingConfig{
			DailyCost:      50,
			CombatRole:     "melee",
			MaxFollowZones: 2,
		},
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return svc, uid
}

// TestHandleHire_Success verifies hire deducts daily cost and binds hireling.
func TestHandleHire_Success(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Patch")
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 450, sess.Currency, "daily cost deducted")
}

// TestHandleHire_AlreadyHired verifies a hireling already hired by another player is rejected.
func TestHandleHire_AlreadyHired(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	// Pre-bind the hireling to another player.
	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	require.NotNil(t, inst)
	svc.initHirelingRuntimeState(inst)
	state := svc.hirelingStateFor(inst.ID)
	require.NotNil(t, state)
	state.HiredByPlayerID = "other_player"

	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "already")
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 500, sess.Currency, "currency unchanged when hire fails")
}

// TestHandleHire_InsufficientCredits verifies hire fails when player cannot afford daily cost.
func TestHandleHire_InsufficientCredits(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 10
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "credits")
	assert.Equal(t, 10, sess.Currency)
}

// TestHandleHire_NpcNotFound verifies error message when hireling not in room.
func TestHandleHire_NpcNotFound(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Nobody"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Nobody")
}

// TestHandleDismiss_Success verifies dismiss releases hireling binding.
func TestHandleDismiss_Success(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	// First hire.
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	evt, err := svc.handleDismiss(uid, &gamev1.DismissRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "dismiss")

	// Verify hireling is now available.
	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	if inst != nil {
		state := svc.hirelingStateFor(inst.ID)
		if state != nil {
			assert.Empty(t, state.HiredByPlayerID)
		}
	}
}

// TestHandleDismiss_NoHireling verifies dismiss is a no-op when player has no hireling.
func TestHandleDismiss_NoHireling(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	evt, err := svc.handleDismiss(uid, &gamev1.DismissRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no hireling")
}

// TestTickHirelingDailyCost_InsufficientCredits verifies auto-dismiss when player cannot pay.
func TestTickHirelingDailyCost_InsufficientCredits(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	// Hire the hireling.
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	// Drain currency so the player cannot pay the daily cost.
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 0

	svc.tickHirelingDailyCost()

	// Hireling should be auto-dismissed.
	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	if inst != nil {
		state := svc.hirelingStateFor(inst.ID)
		if state != nil {
			assert.Empty(t, state.HiredByPlayerID, "hireling auto-dismissed when player can't pay")
		}
	}
}
```

Also append these property-based tests (SWENG-5a) at the end of the same test block before the closing `}`:

```go
// TestProperty_HandleHire_CurrencyNeverNegative verifies that hiring a hireling
// never results in negative player currency.
//
// Precondition: player has 0 to 500 credits.
// Postcondition: currency is never negative after any hire attempt.
func TestProperty_HandleHire_CurrencyNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newHirelingTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = rapid.IntRange(0, 500).Draw(rt, "currency")
		_, _ = svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
		sess, _ = svc.sessions.GetPlayer(uid)
		if sess.Currency < 0 {
			rt.Fatalf("currency went negative: %d", sess.Currency)
		}
	})
}

// TestProperty_HandleHire_NeverPanics verifies handleHire never panics for any input.
//
// Precondition: any npc_name string.
// Postcondition: returns without panic.
func TestProperty_HandleHire_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newHirelingTestServer(t)
		npcName := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(rt, "npcName")
		_, _ = svc.handleHire(uid, &gamev1.HireRequest{NpcName: npcName})
	})
}
```

Make sure `pgregory.net/rapid` is imported in the test file.

- [ ] **4b. Run tests to verify they fail:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleHire|TestHandleDismiss|TestTickHireling|TestProperty_HandleHire" -v 2>&1 | head -20
```

- [ ] **4c. Implement** `internal/gameserver/grpc_service_hireling.go`:

```go
package gameserver

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var hirelingRuntimeMu sync.RWMutex

// initHirelingRuntimeState initialises runtime state for a hireling instance if absent.
//
// Precondition: inst must be non-nil.
// Postcondition: hirelingRuntimeStates[inst.ID] is set iff inst.NPCType == "hireling".
func (s *GameServiceServer) initHirelingRuntimeState(inst *npc.Instance) {
	if inst.NPCType != "hireling" {
		return
	}
	hirelingRuntimeMu.Lock()
	defer hirelingRuntimeMu.Unlock()
	if _, ok := s.hirelingRuntimeStates[inst.ID]; !ok {
		s.hirelingRuntimeStates[inst.ID] = &npc.HirelingRuntimeState{}
	}
}

// hirelingStateFor returns the HirelingRuntimeState for instID, or nil if absent.
func (s *GameServiceServer) hirelingStateFor(instID string) *npc.HirelingRuntimeState {
	hirelingRuntimeMu.RLock()
	defer hirelingRuntimeMu.RUnlock()
	return s.hirelingRuntimeStates[instID]
}

// findHirelingInRoom returns the first hireling NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findHirelingInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "hireling" {
		return nil, fmt.Sprintf("%s is not a hireling.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// findHiredHireling returns the hireling instance currently bound to uid, or nil.
//
// Precondition: uid is non-empty.
// Postcondition: Returns the bound hireling instance, or nil if none.
func (s *GameServiceServer) findHiredHireling(uid string) *npc.Instance {
	hirelingRuntimeMu.RLock()
	defer hirelingRuntimeMu.RUnlock()
	for instID, state := range s.hirelingRuntimeStates {
		if state.HiredByPlayerID == uid {
			if inst := s.npcMgr.InstanceByID(instID); inst != nil {
				return inst
			}
		}
	}
	return nil
}

// handleHire binds a hireling to the player.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleHire(uid string, req *gamev1.HireRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findHirelingInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Hireling == nil {
		return messageEvent("This NPC has no hireling configuration."), nil
	}
	cfg := tmpl.Hireling
	if sess.Currency < cfg.DailyCost {
		return messageEvent(fmt.Sprintf("You need %d credits to hire %s (daily cost).", cfg.DailyCost, inst.Name())), nil
	}

	// REQ-NPC-15: Atomic check-and-set — init state then check.
	s.initHirelingRuntimeState(inst)

	hirelingRuntimeMu.Lock()
	state := s.hirelingRuntimeStates[inst.ID]
	if state.HiredByPlayerID != "" {
		hirelingRuntimeMu.Unlock()
		return messageEvent(fmt.Sprintf("%s is already hired by someone else.", inst.Name())), nil
	}
	state.HiredByPlayerID = uid
	state.ZonesFollowed = 0
	hirelingRuntimeMu.Unlock()

	sess.Currency -= cfg.DailyCost
	return messageEvent(fmt.Sprintf("%s agrees to work with you for %d credits per day.", inst.Name(), cfg.DailyCost)), nil
}

// handleDismiss releases the player's hired hireling.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleDismiss(uid string, req *gamev1.DismissRequest) (*gamev1.ServerEvent, error) {
	if _, ok := s.sessions.GetPlayer(uid); !ok {
		return messageEvent("player not found"), nil
	}
	inst := s.findHiredHireling(uid)
	if inst == nil {
		return messageEvent("You have no hireling to dismiss."), nil
	}
	hirelingRuntimeMu.Lock()
	if state, ok := s.hirelingRuntimeStates[inst.ID]; ok {
		state.HiredByPlayerID = ""
		state.ZonesFollowed = 0
	}
	hirelingRuntimeMu.Unlock()
	return messageEvent(fmt.Sprintf("You dismiss %s. They head back to their post.", inst.Name())), nil
}

// tickHirelingDailyCost charges the daily cost for all hired hirelings.
// Players who cannot pay are auto-dismissed. Intended to be called once per in-game day.
//
// Precondition: s.hirelingRuntimeStates MUST NOT be nil.
// Postcondition: each hired hireling's daily cost is deducted; if insufficient, hireling is dismissed.
func (s *GameServiceServer) tickHirelingDailyCost() {
	hirelingRuntimeMu.Lock()
	defer hirelingRuntimeMu.Unlock()
	for instID, state := range s.hirelingRuntimeStates {
		if state.HiredByPlayerID == "" {
			continue
		}
		sess, ok := s.sessions.GetPlayer(state.HiredByPlayerID)
		if !ok {
			state.HiredByPlayerID = ""
			state.ZonesFollowed = 0
			continue
		}
		inst := s.npcMgr.InstanceByID(instID)
		if inst == nil {
			state.HiredByPlayerID = ""
			continue
		}
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil || tmpl.Hireling == nil {
			continue
		}
		if sess.Currency < tmpl.Hireling.DailyCost {
			state.HiredByPlayerID = ""
			state.ZonesFollowed = 0
			s.sendPlayerMessage(sess, fmt.Sprintf("%s has left your service — you couldn't cover their daily fee.", inst.Name()))
		} else {
			sess.Currency -= tmpl.Hireling.DailyCost
		}
	}
}
```

**Note:** Check if `s.npcMgr.InstanceByID(instID)` exists:
```bash
grep -n "InstanceByID\|func.*Instance\b" /home/cjohannsen/src/mud/internal/game/npc/manager.go | head -10
```
If it doesn't exist, add to `manager.go`:
```go
// InstanceByID returns the instance with the given ID, or nil if not found.
func (m *Manager) InstanceByID(id string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.instances[id]
}
```

Check if `s.sendPlayerMessage(sess, msg)` or equivalent helper exists. If not, implement inline using the same push pattern used elsewhere (look at healer or job trainer handler for the pattern, e.g., finding the proto marshal + entity push call sequence).

- [ ] **4d. Add fields to `GameServiceServer`** in `grpc_service.go`:

Find the `hirelingRuntimeStates` field — check if it already exists:
```bash
grep -n "hirelingRuntimeStates\|hireling" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

If missing, add alongside `healerRuntimeStates`:
```go
hirelingRuntimeStates map[string]*npc.HirelingRuntimeState
```

And initialize in `NewGameServiceServer` (find where `healerRuntimeStates` is initialized and add alongside):
```go
hirelingRuntimeStates: make(map[string]*npc.HirelingRuntimeState),
```

- [ ] **4e. Run hireling tests:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleHire|TestHandleDismiss|TestTickHireling|TestProperty_HandleHire" -v 2>&1 | tail -25
```
Expected: all 7 tests pass.

- [ ] **4f. Run full suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **4g. Commit:**
```bash
git add internal/gameserver/grpc_service_hireling.go internal/gameserver/grpc_service_hireling_test.go internal/gameserver/grpc_service.go
git commit -m "feat: hireling runtime state, hire/dismiss handlers, daily cost tick"
```

---

## Task 5 — Wire hire/dismiss dispatch + REQ-NPC-8 + daily tick

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_npc_ticks.go`
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/combat_handler_guard_test.go` (reuse file for hireling attack test)

### Steps

- [ ] **5a. Wire dispatch cases** in `grpc_service.go`.

Find the `case *gamev1.ClientMessage_SetJob:` dispatch (the last job trainer case), then add after it:

```go
case *gamev1.ClientMessage_Hire:
    return s.handleHire(uid, payload.Hire)
case *gamev1.ClientMessage_Dismiss:
    return s.handleDismiss(uid, payload.Dismiss)
```

- [ ] **5b. Wire daily tick** in `grpc_service_npc_ticks.go`.

First confirm the file and block location:
```bash
grep -n "Hour == 0\|tickHealerCapacity\|tickBankerRates" /home/cjohannsen/src/mud/internal/gameserver/grpc_service_npc_ticks.go | head -10
```

In the `if dt.Hour == 0 {` block where `tickHealerCapacity()` is called, add:
```go
s.tickHirelingDailyCost()
```

- [ ] **5c. Implement REQ-NPC-8 guard in `CombatHandler.Attack`.**

Find the attack target validation (around line 329):
```go
// REQ-NPC-4: non-combat NPCs cannot be attacked directly.
if inst.NPCType != "" && inst.NPCType != "combat" && inst.NPCType != "guard" && inst.NPCType != "hireling" {
    return nil, fmt.Errorf("%s is not a valid combat target", inst.Name())
}
```

After this block, add the hireling ownership guard:
```go
// REQ-NPC-8: a hireling bound to the attacking player MUST NOT be targetable
// by that player's own attacks.
if inst.NPCType == "hireling" && s.hirelingOwnerOf != nil {
    if ownerUID := s.hirelingOwnerOf(inst.ID); ownerUID == uid {
        return nil, fmt.Errorf("you cannot attack your own hireling")
    }
}
```

**Note:** `CombatHandler` does not currently hold a reference to `hirelingRuntimeStates`. The cleanest approach is to add an `hirelingOwnerOf func(instID string) string` callback to `CombatHandler` at construction time (same pattern as `broadcastFn`). Wire it in `NewGameServiceServer` where `CombatHandler` is created.

Check how `CombatHandler` is constructed:
```bash
grep -n "NewCombatHandler\|CombatHandler{" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -5
```
And the constructor:
```bash
grep -n "func NewCombatHandler" /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go
```

Add to `CombatHandler` struct:
```go
// hirelingOwnerOf returns the UID of the player who has hired the given hireling instance,
// or empty string if not hired. Used to enforce REQ-NPC-8.
hirelingOwnerOf func(instID string) string
```

In `Attack`, after the hireling NPCType check, call:
```go
if inst.NPCType == "hireling" && h.hirelingOwnerOf != nil {
    if owner := h.hirelingOwnerOf(inst.ID); owner == uid {
        return nil, fmt.Errorf("you cannot attack your own hireling")
    }
}
```

- [ ] **5d. Write REQ-NPC-8 test** in `combat_handler_guard_test.go` (append):

```go
// TestAttack_CannotAttackOwnHireling verifies REQ-NPC-8: a player cannot attack
// their own bound hireling.
func TestAttack_CannotAttackOwnHireling(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	uid := "hireling-owner"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "u", CharName: "Owner",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID: "my_hireling", Name: "Patch", NPCType: "hireling",
		Level: 2, MaxHP: 20, AC: 11,
		Hireling: &npc.HirelingConfig{DailyCost: 50, CombatRole: "melee"},
	}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	h := &CombatHandler{
		sessions:        sessMgr,
		npcMgr:          npcMgr,
		worldMgr:        worldMgr,
		broadcastFn:     func(string, []*gamev1.CombatEvent) {},
		engine:          combat.NewEngine(),
		hirelingOwnerOf: func(instID string) string {
			if instID == inst.ID {
				return uid
			}
			return ""
		},
	}

	_, err = h.Attack(uid, "Patch")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot attack your own hireling")
}
```

- [ ] **5e. Run tests:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -v 2>&1 | tail -30
```

- [ ] **5f. Run full suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **5g. Commit:**
```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_npc_ticks.go internal/gameserver/combat_handler.go internal/gameserver/combat_handler_guard_test.go
git commit -m "feat: wire hire/dismiss dispatch, REQ-NPC-8 attack guard, daily hireling cost tick"
```

---

## Task 6 — Zone follow tracking on room transition

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_hireling_test.go`

On every player room transition, if the player has a bound hireling, move the hireling to the player's new room. If the destination is a different zone, increment `ZonesFollowed`. If `MaxFollowZones > 0` and `ZonesFollowed` would exceed it, leave the hireling behind with a warning.

### Steps

- [ ] **6a. Write failing tests** — append to `grpc_service_hireling_test.go`:

```go
// TestHirelingFollowsPlayerInSameZone verifies hireling moves to player's new room
// within the same zone.
func TestHirelingFollowsPlayerInSameZone(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	// Hire Patch.
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	require.NotNil(t, inst)
	instID := inst.ID

	// Simulate player moving to room_b (same zone).
	svc.moveHirelingWithPlayer(uid, "room_b", "zone_a", "zone_a")

	movedInst := svc.npcMgr.InstanceByID(instID)
	require.NotNil(t, movedInst)
	assert.Equal(t, "room_b", movedInst.RoomID, "hireling should follow to room_b")
}

// TestHirelingZoneFollowLimit verifies hireling stays behind when MaxFollowZones exceeded.
func TestHirelingZoneFollowLimit(t *testing.T) {
	svc, uid := newHirelingTestServer(t)
	_, err := svc.handleHire(uid, &gamev1.HireRequest{NpcName: "Patch"})
	require.NoError(t, err)

	inst := svc.npcMgr.FindInRoom("room_a", "Patch")
	require.NotNil(t, inst)
	instID := inst.ID

	// MaxFollowZones = 2; exhaust it.
	svc.moveHirelingWithPlayer(uid, "room_b", "zone_a", "zone_b") // ZonesFollowed=1
	svc.moveHirelingWithPlayer(uid, "room_c", "zone_b", "zone_c") // ZonesFollowed=2
	// Third cross-zone move: hireling should stay behind.
	svc.moveHirelingWithPlayer(uid, "room_d", "zone_c", "zone_d") // limit hit

	movedInst := svc.npcMgr.InstanceByID(instID)
	require.NotNil(t, movedInst)
	assert.NotEqual(t, "room_d", movedInst.RoomID, "hireling must NOT follow when zone limit exceeded")
}
```

- [ ] **6b. Run to verify they fail:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHireling" -v 2>&1 | head -15
```

- [ ] **6c. Implement `moveHirelingWithPlayer`** in `grpc_service_hireling.go`:

```go
// moveHirelingWithPlayer moves the bound hireling (if any) to newRoomID when the
// player transitions rooms. If the move crosses a zone boundary, ZonesFollowed
// is incremented; if MaxFollowZones is reached the hireling stays behind with a warning.
//
// Precondition: uid, newRoomID, fromZoneID, toZoneID must be non-empty.
// Postcondition: if the player has a bound hireling and zone limit is not exceeded,
// the hireling instance is moved to newRoomID; otherwise it stays in its current room.
func (s *GameServiceServer) moveHirelingWithPlayer(uid, newRoomID, fromZoneID, toZoneID string) {
	inst := s.findHiredHireling(uid)
	if inst == nil {
		return
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)

	crossingZone := fromZoneID != toZoneID

	if crossingZone && tmpl != nil && tmpl.Hireling != nil && tmpl.Hireling.MaxFollowZones > 0 {
		hirelingRuntimeMu.Lock()
		state := s.hirelingRuntimeStates[inst.ID]
		if state != nil && state.ZonesFollowed >= tmpl.Hireling.MaxFollowZones {
			hirelingRuntimeMu.Unlock()
			// Warn the player; hireling stays.
			if sess, ok := s.sessions.GetPlayer(uid); ok {
				s.sendPlayerMessage(sess, fmt.Sprintf("%s cannot follow you any further and stays behind.", inst.Name()))
			}
			return
		}
		if state != nil {
			state.ZonesFollowed++
		}
		hirelingRuntimeMu.Unlock()
	}

	_ = s.npcMgr.Move(inst.ID, newRoomID)
}
```

- [ ] **6d. Hook into room transition** in `grpc_service.go`.

Find the room-entry guard check block (the one with `wantedLevel := sess.WantedLevel[newRoom.ZoneID]`). Immediately before it, add:

```go
// Move bound hireling with player on room transition.
if s.npcH != nil {
    s.moveHirelingWithPlayer(uid, newRoom.RoomId, oldRoom.ZoneId, newRoom.ZoneId)
}
```

**Note:** Check what the old room variable is called. Before the room transition, the code reads `sess.RoomID` as the old room. The `oldRoom` zone ID comes from `s.worldMgr.GetRoom(sess.RoomID)`. Find the exact pattern in the existing move handler.

```bash
grep -n "oldRoom\|prevRoom\|sess.RoomID.*ZoneID\|ZoneId\b" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -20
```

Use the correct variables for old/new zone IDs.

- [ ] **6e. Run hireling tests:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHireling" -v 2>&1 | tail -20
```

- [ ] **6f. Run full suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **6g. Commit:**
```bash
git add internal/gameserver/grpc_service_hireling.go internal/gameserver/grpc_service_hireling_test.go internal/gameserver/grpc_service.go
git commit -m "feat: hireling zone follow tracking on room transition (REQ-NPC-15 zone limit)"
```

---

## Task 7 — Named NPC YAML content

**Files:**
- Create: `content/npcs/marshal_ironsides.yaml`
- Create: `content/npcs/patch.yaml`

### Steps

- [ ] **7a. Confirm YAML field names** by checking the Guard struct tags:
```bash
grep -A3 "GuardConfig\|WantedThreshold\|Bribeable\|MaxBribeWanted\|BaseCosts" /home/cjohannsen/src/mud/internal/game/npc/noncombat.go | head -20
grep -A3 "HirelingConfig\|DailyCost\|CombatRole\|MaxFollowZones" /home/cjohannsen/src/mud/internal/game/npc/noncombat.go | head -15
```

- [ ] **7b. Create** `content/npcs/marshal_ironsides.yaml`:

```yaml
id: marshal_ironsides
name: Marshal Ironsides
npc_type: guard
description: >
  A broad-shouldered enforcer in scuffed riot gear, Marshal Ironsides patrols the
  safe zone of Rustbucket Ridge with a practiced eye for trouble. She's seen it all
  and takes no bribes — but she's not looking for a fight either, unless you give her one.
max_hp: 45
ac: 16
level: 5
awareness: 6
personality: brave
guard:
  wanted_threshold: 2
  bribeable: false
```

- [ ] **7c. Create** `content/npcs/patch.yaml`:

```yaml
id: patch
name: Patch
npc_type: hireling
description: >
  A wiry scavenger with a battered shotgun slung over one shoulder and a duct-taped
  med-pack on the other. Patch asks no questions and takes no prisoners — as long as
  the credits keep coming.
max_hp: 28
ac: 13
level: 4
awareness: 4
personality: opportunistic
hireling:
  daily_cost: 75
  combat_role: melee
  max_follow_zones: 3
```

- [ ] **7d. Verify YAML loads:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -15
```
The property test `TestProperty_AllExistingNPCTemplatesStillLoad` (if it exists) will catch parse errors. Also run:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -5
```

- [ ] **7e. Commit:**
```bash
git add content/npcs/marshal_ironsides.yaml content/npcs/patch.yaml
git commit -m "content: add named guard (Marshal Ironsides) and hireling (Patch) NPCs for Rustbucket Ridge"
```

---

## Task 8 — Final integration smoke test

### Steps

- [ ] **8a. Full test suite:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -15
```

- [ ] **8b. Vet:**
```bash
cd /home/cjohannsen/src/mud && mise exec -- go vet ./... 2>&1 | head -10
```

- [ ] **8c. Mark complete:** No commit needed unless fixups were required.

---

## Requirements Traceability

| Requirement | Task |
|-------------|------|
| REQ-WC-2b: `MaxBribeWantedLevel` in range 1–4 when `Bribeable` | Task 1 |
| REQ-NPC-6: 2nd safe violation → all guards enter initiative | Already implemented in `CheckSafeViolation` + `InitiateGuardCombat` |
| REQ-NPC-7: Guards check WantedLevel on room entry + change events | Task 2 (per-guard threshold) + already wired on change via enforcement.go |
| REQ-NPC-8: Hireling MUST NOT be targetable by player's own attacks | Task 5 |
| REQ-NPC-15: Hireling binding MUST be atomic check-and-set | Task 4 |
| Named guard NPC: Rustbucket Ridge Safe room | Task 7 |
| Named hireling NPC: Rustbucket Ridge | Task 7 |
