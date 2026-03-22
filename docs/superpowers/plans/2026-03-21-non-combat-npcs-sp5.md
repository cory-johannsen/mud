# Non-Combat NPCs SP5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete remaining non-combat NPC types: `talk` command for Quest Givers, a named Crafter NPC, the `FixerConfig` data model with validation, a named Fixer NPC, and a feature-doc update marking SP4 complete.

**Architecture:** `FixerConfig` follows the existing type-config pattern in `noncombat.go` + `template.go`. The `talk` command follows the healer handler pattern: proto message → `handleTalk` in a new dedicated file → dispatched from `grpc_service.go`. Named NPC content is YAML-only.

**Tech Stack:** Go 1.26, `github.com/stretchr/testify`, `pgregory.net/rapid`, protobuf/protoc

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/game/npc/noncombat.go` | Modify | Add `FixerConfig` struct + `Validate()` |
| `internal/game/npc/noncombat_test.go` | Modify | Tests for `FixerConfig.Validate()` |
| `internal/game/npc/template.go` | Modify | Add `Fixer *FixerConfig` field; register `"fixer"` type |
| `internal/game/npc/template_test.go` | Modify | Tests for fixer template validation |
| `api/proto/game/v1/game.proto` | Modify | Add `TalkRequest` (field 101) to `ClientMessage` oneof |
| `internal/gameserver/gamev1/` | Regenerate | Run `make proto` after proto change |
| `internal/gameserver/grpc_service_quest_giver.go` | Create | `findQuestGiverInRoom` + `handleTalk` |
| `internal/gameserver/grpc_service_quest_giver_test.go` | Create | Tests for `handleTalk` |
| `internal/gameserver/grpc_service.go` | Modify | Wire `TalkRequest` dispatch case |
| `content/npcs/gail_grinder_graves.yaml` | Create | Named quest giver NPC |
| `content/npcs/sparks.yaml` | Create | Named crafter NPC |
| `content/npcs/dex.yaml` | Create | Named fixer NPC |
| `docs/features/non-combat-npcs.md` | Modify | Mark SP4 guard/hireling requirements complete |

---

## Task 1: FixerConfig Data Model

**Files:**
- Modify: `internal/game/npc/noncombat.go`
- Modify: `internal/game/npc/noncombat_test.go`

### Context
Open `internal/game/npc/noncombat.go`. The bottom of the file has `CrafterConfig` (an empty stub struct). Add `FixerConfig` after it, following the same pattern as `GuardConfig.Validate()` for multi-field validation.

`FixerConfig` fields (from `docs/superpowers/specs/2026-03-20-wanted-clearing-design.md`):
- `BaseCosts map[int]int` — keys 1–4, all positive (REQ-WC-2a)
- `NPCVariance float64` — must be > 0 (REQ-WC-1)
- `MaxWantedLevel int` — must be in range 1–4 (REQ-WC-2)
- `ClearRecordQuestID string` — optional, deferred to `wanted-clearing` feature

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/npc/noncombat_test.go`:

```go
func TestFixerConfig_ValidConfig(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
		NPCVariance:    1.2,
		MaxWantedLevel: 3,
	}
	assert.NoError(t, c.Validate())
}

func TestFixerConfig_NPCVarianceZero(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
		NPCVariance:    0,
		MaxWantedLevel: 3,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_NPCVarianceNegative(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
		NPCVariance:    -0.5,
		MaxWantedLevel: 3,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_MaxWantedLevelZero(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
		NPCVariance:    1.0,
		MaxWantedLevel: 0,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_MaxWantedLevelFive(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 3: 400, 4: 800},
		NPCVariance:    1.0,
		MaxWantedLevel: 5,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_BaseCostsMissingKey(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 200, 4: 800}, // missing key 3
		NPCVariance:    1.0,
		MaxWantedLevel: 3,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_BaseCostsNonPositiveValue(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      map[int]int{1: 100, 2: 0, 3: 400, 4: 800}, // zero value
		NPCVariance:    1.0,
		MaxWantedLevel: 3,
	}
	assert.Error(t, c.Validate())
}

func TestFixerConfig_NilBaseCosts(t *testing.T) {
	c := npc.FixerConfig{
		BaseCosts:      nil,
		NPCVariance:    1.0,
		MaxWantedLevel: 3,
	}
	assert.Error(t, c.Validate())
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/npc/... -run TestFixerConfig -v 2>&1 | head -30
```

Expected: compile error ("undefined: npc.FixerConfig")

- [ ] **Step 3: Implement FixerConfig**

Add to the bottom of `internal/game/npc/noncombat.go` (after `CrafterConfig`):

```go
// ---- Fixer ----

// FixerConfig holds the static configuration for a fixer NPC.
// Full bribe/fix command behavior is deferred to the wanted-clearing feature.
// REQ-WC-1, REQ-WC-2, REQ-WC-2a: validated in Validate().
type FixerConfig struct {
	// BaseCosts maps WantedLevel (1–4) to base credit cost for clearing that level.
	BaseCosts map[int]int `yaml:"base_costs"`
	// NPCVariance multiplies the base cost to produce the final price. Must be > 0.
	NPCVariance float64 `yaml:"npc_variance"`
	// MaxWantedLevel is the highest WantedLevel this fixer will negotiate. Range 1–4.
	MaxWantedLevel int `yaml:"max_wanted_level"`
	// ClearRecordQuestID is the quest that clears the criminal record. Empty until quests feature.
	ClearRecordQuestID string `yaml:"clear_record_quest_id,omitempty"`
}

// Validate enforces REQ-WC-1 (NPCVariance > 0), REQ-WC-2 (MaxWantedLevel 1–4),
// and REQ-WC-2a (BaseCosts has all keys 1–4 with positive values).
func (f FixerConfig) Validate() error {
	if f.NPCVariance <= 0 {
		return fmt.Errorf("fixer: npc_variance must be > 0, got %f", f.NPCVariance)
	}
	if f.MaxWantedLevel < 1 || f.MaxWantedLevel > 4 {
		return fmt.Errorf("fixer: max_wanted_level must be in range 1–4, got %d", f.MaxWantedLevel)
	}
	for _, key := range []int{1, 2, 3, 4} {
		v, ok := f.BaseCosts[key]
		if !ok {
			return fmt.Errorf("fixer: base_costs missing required key %d", key)
		}
		if v <= 0 {
			return fmt.Errorf("fixer: base_costs[%d] must be positive, got %d", key, v)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/npc/... -run TestFixerConfig -v 2>&1 | tail -20
```

Expected: all 8 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/noncombat.go internal/game/npc/noncombat_test.go
git commit -m "feat: add FixerConfig with Validate() (REQ-WC-1/2/2a)"
```

---

## Task 2: Register Fixer Type in Template

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go`

### Context
Open `internal/game/npc/template.go`. Three changes are needed:
1. Add `Fixer *FixerConfig \`yaml:"fixer,omitempty"\`` to the `Template` struct (after `Crafter *CrafterConfig`)
2. Add `"fixer": true` to the `validTypes` map in `Validate()`
3. Add a switch case for `"fixer"` after the `"crafter"` case

REQ-WC-3: Fixers MUST NOT enter initiative order. This is enforced by requiring `personality: cowardly` (always flee) in the YAML — add a check in the fixer switch case that rejects any personality other than `""` or `"cowardly"`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/npc/template_test.go`:

```go
func TestTemplate_FixerRequiresConfigBlock(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
`)
	_, err := npc.LoadTemplate(data)
	assert.Error(t, err, "fixer without fixer: block must error")
	assert.Contains(t, err.Error(), "requires a fixer:")
}

func TestTemplate_FixerWithInvalidConfig(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
fixer:
  npc_variance: 0
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	_, err := npc.LoadTemplate(data)
	assert.Error(t, err, "fixer with npc_variance=0 must error")
}

func TestTemplate_FixerValidLoads(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
fixer:
  npc_variance: 1.2
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	tmpl, err := npc.LoadTemplate(data)
	assert.NoError(t, err)
	assert.NotNil(t, tmpl.Fixer)
}

func TestTemplate_FixerNonCowardlyPersonalityErrors(t *testing.T) {
	data := []byte(`id: test_fixer
name: Test Fixer
npc_type: fixer
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: brave
fixer:
  npc_variance: 1.2
  max_wanted_level: 3
  base_costs:
    1: 100
    2: 200
    3: 400
    4: 800
`)
	_, err := npc.LoadTemplate(data)
	assert.Error(t, err, "fixer with personality 'brave' must error (REQ-WC-3)")
	assert.Contains(t, err.Error(), "personality")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/npc/... -run TestTemplate_Fixer -v 2>&1 | head -30
```

Expected: compile error or test failures (Fixer field not yet on Template)

- [ ] **Step 3: Add Fixer field to Template struct**

In `internal/game/npc/template.go`, find the `Template` struct. After the line with `Crafter *CrafterConfig`, add:

```go
Fixer      *FixerConfig      `yaml:"fixer,omitempty"`
```

- [ ] **Step 4: Register "fixer" in validTypes and add switch case**

In `Validate()` in `template.go`:

**In the validTypes map**, add `"fixer": true`:
```go
validTypes := map[string]bool{
    "combat": true, "merchant": true, "guard": true, "healer": true,
    "quest_giver": true, "hireling": true, "banker": true,
    "job_trainer": true, "crafter": true, "fixer": true,
}
```

**After the `"crafter"` case**, add:
```go
case "fixer":
    if t.Fixer == nil {
        return fmt.Errorf("npc template %q: npc_type 'fixer' requires a fixer: config block", t.ID)
    }
    if err := t.Fixer.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
    // REQ-WC-3: fixers must not enter initiative order; enforce cowardly (flee) personality.
    if t.Personality != "" && t.Personality != "cowardly" {
        return fmt.Errorf("npc template %q: fixer npc_type requires personality 'cowardly' or empty (got %q)", t.ID, t.Personality)
    }
    t.Personality = "cowardly" // normalise empty → cowardly
```

- [ ] **Step 5: Run all NPC tests**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -30
```

Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/template_test.go
git commit -m "feat: register fixer npc_type in Template with validation (REQ-WC-3)"
```

---

## Task 3: talk Command — Proto + Handler

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Create: `internal/gameserver/grpc_service_quest_giver.go`
- Create: `internal/gameserver/grpc_service_quest_giver_test.go`
- Modify: `internal/gameserver/grpc_service.go`

### Context
The `talk <npc>` command follows the identical pattern as `handleHeal`:
1. Add proto message → run `make proto`
2. Write handler in dedicated file
3. Add dispatch case in `grpc_service.go`

The handler uses `s.npcMgr.FindInRoom(roomID, npcName)` (exact method signature from `internal/game/npc/manager.go:238`). On match, it picks a random line from `tmpl.QuestGiver.PlaceholderDialog` using `rand.Intn`.

- [ ] **Step 1: Add TalkRequest to proto**

In `api/proto/game/v1/game.proto`, in the `ClientMessage` oneof after `DismissRequest dismiss = 100;`, add:

```protobuf
    TalkRequest      talk            = 101;
```

After the existing message definitions (after `DismissRequest`), add:

```protobuf
message TalkRequest {
  string npc_name = 1;  // case-insensitive prefix matched against NPC name in room
}
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
make proto 2>&1
```

Expected: no errors; `internal/gameserver/gamev1/game.pb.go` and `game_grpc.pb.go` regenerated.

- [ ] **Step 3: Write failing tests**

Create `internal/gameserver/grpc_service_quest_giver_test.go`:

```go
package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	// import gameserver test helpers as used in grpc_service_healer_test.go
)

// NOTE: Look at grpc_service_healer_test.go for the exact test helper setup pattern
// (newTestServer, seedNPC, etc.). Replicate that pattern for quest giver tests.

func TestHandleTalk_QuestGiverFound(t *testing.T) {
	// Setup: create a test server with a quest_giver NPC in the player's room.
	// Call handleTalk with a matching name.
	// Expect: response contains one of the dialog lines.
}

func TestHandleTalk_NPCNotInRoom(t *testing.T) {
	// Setup: player room has no NPC.
	// Call handleTalk("nobody").
	// Expect: message "No one named 'nobody' here."
}

func TestHandleTalk_WrongNPCType(t *testing.T) {
	// Setup: room has a "combat" NPC (not quest_giver).
	// Call handleTalk with that NPC's name.
	// Expect: message "No one named '<name>' here."
}

func TestHandleTalk_CaseInsensitiveMatch(t *testing.T) {
	// Setup: quest_giver NPC named "Gail".
	// Call handleTalk("gail").
	// Expect: returns a dialog line (match succeeds).
}

func TestProperty_HandleTalk_AlwaysReturnsDialogLine(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a non-empty PlaceholderDialog slice of arbitrary strings.
		// Call handleTalk on a matching quest giver.
		// Assert: returned message content is one of the dialog entries.
	})
}
```

**Important:** Look at `internal/gameserver/grpc_service_healer_test.go` for the exact test server/NPC setup helpers used in this package. Replicate that pattern exactly — do not invent new helpers.

- [ ] **Step 4: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleTalk -v 2>&1 | head -20
```

Expected: compile error (handleTalk not defined) or test failures.

- [ ] **Step 5: Implement handleTalk**

Create `internal/gameserver/grpc_service_quest_giver.go`:

```go
package gameserver

import (
	"fmt"
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findQuestGiverInRoom returns the first quest_giver NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findQuestGiverInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	if inst.NPCType != "quest_giver" {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	return inst, ""
}

// handleTalk responds to a talk command by returning a random line from the NPC's
// PlaceholderDialog.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleTalk(uid string, req *gamev1.TalkRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findQuestGiverInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.QuestGiver == nil {
		return messageEvent("That NPC has no dialog configured."), nil
	}
	dialog := tmpl.QuestGiver.PlaceholderDialog
	line := dialog[rand.Intn(len(dialog))]
	return messageEvent(fmt.Sprintf("%s says: %q", inst.Name(), line)), nil
}
```

- [ ] **Step 6: Wire dispatch case in grpc_service.go**

In `internal/gameserver/grpc_service.go`, find the dispatch switch (near the `case *gamev1.ClientMessage_Dismiss:` block). Add after it:

```go
case *gamev1.ClientMessage_Talk:
    return s.handleTalk(uid, p.Talk)
```

- [ ] **Step 7: Run all gameserver tests**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run "TestHandleTalk|TestProperty_HandleTalk" -v -race 2>&1 | tail -30
```

Expected: all tests PASS

- [ ] **Step 8: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud
make test-fast 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add api/proto/game/v1/game.proto \
        internal/gameserver/gamev1/ \
        internal/gameserver/grpc_service_quest_giver.go \
        internal/gameserver/grpc_service_quest_giver_test.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add talk command for quest giver NPCs (REQ-NPC-QG-1/2)"
```

---

## Task 4: Named NPC Content

**Files:**
- Create: `content/npcs/gail_grinder_graves.yaml`
- Create: `content/npcs/sparks.yaml`
- Create: `content/npcs/dex.yaml`

### Context
Use `content/npcs/marshal_ironsides.yaml` and `content/npcs/patch.yaml` as format references. Each file needs: `id`, `name`, `npc_type`, `max_hp`, `ac`, `level`, `awareness`, `personality`, and the type-specific config block.

Rustbucket Ridge rooms (from zone content — use existing room IDs from the zone YAML; check `content/zones/rustbucket_ridge.yaml` or similar for valid room IDs to use in NPC placement).

- [ ] **Step 1: Create Gail "Grinder" Graves (quest giver)**

```yaml
# content/npcs/gail_grinder_graves.yaml
id: gail_grinder_graves
name: Gail "Grinder" Graves
npc_type: quest_giver
max_hp: 18
ac: 11
level: 3
awareness: 4
personality: neutral
respawn_duration: 0s
quest_giver:
  placeholder_dialog:
    - "You want work? I got work. Come back when the heat dies down."
    - "Word is there's a shipment coming in through the back docks. Could use eyes on it."
    - "I don't ask where you've been, you don't ask where I send you. We clear?"
    - "Every piece of scrap in this yard has a story. Most of 'em end badly."
    - "The Grinder name wasn't earned by sitting still. Neither was mine."
  quest_ids: []
```

- [ ] **Step 2: Create Sparks (crafter)**

```yaml
# content/npcs/sparks.yaml
id: sparks
name: Sparks
npc_type: crafter
max_hp: 14
ac: 10
level: 2
awareness: 3
personality: neutral
respawn_duration: 0s
crafter: {}
```

- [ ] **Step 3: Create Dex (fixer)**

```yaml
# content/npcs/dex.yaml
id: dex
name: Dex
npc_type: fixer
max_hp: 22
ac: 12
level: 4
awareness: 5
personality: cowardly
respawn_duration: 0s
fixer:
  npc_variance: 1.15
  max_wanted_level: 3
  base_costs:
    1: 150
    2: 350
    3: 700
    4: 1500
```

- [ ] **Step 4: Verify NPC files load without error**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/npc/... -v -run TestLoad 2>&1 | tail -20
```

If no `TestLoad` exists, run the full NPC suite:

```bash
mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -20
```

Expected: all PASS (the loader validates all YAML files in `content/npcs/` at startup)

- [ ] **Step 5: Run full fast test suite**

```bash
cd /home/cjohannsen/src/mud
make test-fast 2>&1 | tail -10
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add content/npcs/gail_grinder_graves.yaml content/npcs/sparks.yaml content/npcs/dex.yaml
git commit -m "content: add named quest giver (Gail), crafter (Sparks), and fixer (Dex) NPCs"
```

---

## Task 5: Feature Doc Update

**Files:**
- Modify: `docs/features/non-combat-npcs.md`

### Context
SP4 (Guard + Hireling) implemented the following requirements. Mark them complete with `[x]`:
- REQ-NPC-6, REQ-NPC-7 (Guard aggression/room-entry)
- REQ-WC-2b (GuardConfig.MaxBribeWantedLevel validation)
- GuardConfig fields (Bribeable, MaxBribeWantedLevel, BaseCosts), WantedThreshold
- Named guard NPC (Marshal Ironsides)
- REQ-NPC-8 (Hireling combat ally), REQ-NPC-15 (atomic bind)
- hire/dismiss commands, zone follow, named hireling NPC (Patch)

Also mark the SP5 items added in this sub-project.

- [ ] **Step 1: Update the feature doc**

In `docs/features/non-combat-npcs.md`, change the Guard section checkboxes:

```markdown
- [x] Guard
  - [x] REQ-NPC-6: On Safe room second violation, all guards present MUST enter initiative and target the aggressor.
  - [x] REQ-NPC-7: Guards MUST check WantedLevel on room entry and on WantedLevel change events.
  - [x] REQ-WC-2b: `GuardConfig.MaxBribeWantedLevel` MUST be in range 1–4 when `Bribeable` is true; fatal load error otherwise.
  - [x] WantedThreshold-configurable aggression table
  - [x] `GuardConfig.Bribeable bool` field (default: false)
  - [x] `GuardConfig.MaxBribeWantedLevel int` field (default: 2)
  - [x] `GuardConfig.BaseCosts map[int]int` field for bribeable guards (all keys 1–4, positive values)
  - [x] Named NPC: Marshal Ironsides (guard, Rustbucket Ridge)
- [x] Hireling
  - [x] REQ-NPC-8: Hirelings MUST be combat allies; MUST NOT be targetable by player's own attacks.
  - [x] REQ-NPC-15: Hireling binding MUST be atomic check-and-set.
  - [x] `hire <npc>` and `dismiss` commands
  - [x] Zone follow tracking with `MaxFollowZones` limit
  - [x] Named NPC: Patch (hireling, Rustbucket Ridge)
```

Also mark the SP5 Quest Giver, Crafter, and Fixer items:

```markdown
- [x] Quest Giver
  - [x] `talk <npc>` command with placeholder dialog
  - [x] Named NPC: Gail "Grinder" Graves (Scrapshack 23)
- [x] Crafter (stub)
  - [x] `npc_type: "crafter"` declared; full behavior deferred to `crafting` feature
  - [x] Named NPC: Sparks (The Tinker's Den)
- [x] Fixer (data model — commands deferred to `wanted-clearing` feature)
  - [x] REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; fatal load error.
  - [x] REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; fatal load error.
  - [x] REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain all keys 1–4 with positive values.
  - [x] REQ-WC-3: Fixers MUST default to `flee` on combat start; MUST NOT enter initiative order.
  - [x] `Fixer *FixerConfig` field added to NPC Template struct
  - [x] `Template.Validate()` updated to recognize `"fixer"` type
  - [x] Named NPC: Dex (fixer, Rustbucket Ridge)
```

- [ ] **Step 2: Commit**

```bash
git add docs/features/non-combat-npcs.md
git commit -m "docs: mark SP4 guard/hireling and SP5 quest giver/crafter/fixer complete"
```

---

## Final Verification

- [ ] **Run full fast test suite one more time**

```bash
cd /home/cjohannsen/src/mud
make test-fast 2>&1 | tail -10
```

Expected: all PASS, no race conditions.

- [ ] **Run go vet**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go vet ./... 2>&1
```

Expected: no output (no issues).
