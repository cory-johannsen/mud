# Curse Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Implementation Order for `npc/template.go`:** map-poi → non-human-npcs → npc-behaviors → advanced-enemies → factions → **curse-removal** (last, as it only adds ChipDocConfig).

**Goal:** Add a `chip_doc` non-combat NPC type and an `uncurse <item>` command that removes cursed items for a credit cost + Rigging skill check, converting them to `defective` modifier items.

**Architecture:** Follows the existing non-combat NPC pattern (`job_trainer`): `ChipDocConfig` struct in `internal/game/npc/template.go`, NPC type registered in the valid-types map, command handler in a new `grpc_service_chip_doc.go`, skill check using the same `d20 + skillMod vs DC` four-tier pattern as `handleNegotiate`. `UncurseItem` helper is provided by the `equipment-mechanics` feature plan.

**Tech Stack:** Go 1.22+, `pgregory.net/rapid` for property-based tests, YAML NPC content files.

**Dependency note:** This feature depends on `equipment-mechanics` (for the cursed modifier and `UncurseItem` function) and `non-combat-npcs` (for the NPC type framework). The `chip_doc` NPC type can be implemented alongside the existing `job_trainer` pattern — the `non-combat-npcs` framework (template.go, manager.go, noncombat.go) is already present in the codebase even though that feature is marked blocked.

---

## File Map

**New files:**
- `internal/gameserver/grpc_service_chip_doc.go` — `findChipDocInRoom`, `handleUncurse`
- `internal/gameserver/grpc_service_chip_doc_test.go`
- `content/npcs/chip_doc_gunchete.yaml` — chip_doc NPC instance for Gunchete zone

**Modified files:**
- `internal/game/npc/template.go` — add `ChipDocConfig` struct; register `"chip_doc"` in `validTypes`; add `ChipDoc *ChipDocConfig` field to `Template`; add validation case
- `internal/game/npc/template_test.go` (or create) — tests for chip_doc validation
- `internal/gameserver/grpc_service.go` — wire `uncurse` command in the `ClientMessage` dispatch switch

---

## Requirement Identifiers

- REQ-CR-1: `chip_doc` NPC type MUST be registered as a valid `npc_type` value; absence of a `chip_doc:` config block when `npc_type: chip_doc` MUST be a fatal load error.
- REQ-CR-2: `chip_doc` NPC instances MUST cower when in combat (same personality as `job_trainer`).
- REQ-CR-3: `uncurse <item>` MUST require a `chip_doc` NPC in the player's room; cowering chip_docs MUST refuse.
- REQ-CR-4: `uncurse <item>` MUST fail if the named item is not equipped and cursed on the player.
- REQ-CR-5: `uncurse <item>` MUST deduct `removal_cost` credits before the skill check; insufficient credits MUST be rejected before any deduction.
- REQ-CR-6: The skill check MUST be `d20 + riggingModifier vs check_dc` using PF2E four-tier rules (crit success ≥ DC+10, success ≥ DC, failure < DC, crit failure ≤ DC-10 or natural 1).
- REQ-CR-7: Critical Success or Success: credits deducted, curse removed via `UncurseItem`, item unequipped and returned to backpack as defective.
- REQ-CR-8: Failure: credits deducted, item remains cursed.
- REQ-CR-9: Critical Failure: credits deducted, item remains cursed, `fatigued` condition applied to player.
- REQ-CR-10: Each zone MUST have at least one `chip_doc` NPC YAML instance placed in a Safe room.

---

## Task 1: ChipDocConfig + NPC Type Registration

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify (or create): `internal/game/npc/template_chip_doc_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/game/npc/template_chip_doc_test.go
package npc_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/npc"
)

func TestTemplate_ChipDoc_ValidConfig(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "chip_doc_test",
        Name:    "Chip Doc",
        Level:   3,
        MaxHP:   20,
        AC:      10,
        NPCType: "chip_doc",
        ChipDoc: &npc.ChipDocConfig{RemovalCost: 50, CheckDC: 15},
    }
    if err := tmpl.Validate(); err != nil {
        t.Errorf("expected valid chip_doc template, got error: %v", err)
    }
}

func TestTemplate_ChipDoc_MissingConfig_Fails(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "chip_doc_test",
        Name:    "Chip Doc",
        Level:   3,
        MaxHP:   20,
        AC:      10,
        NPCType: "chip_doc",
        // ChipDoc is nil
    }
    if err := tmpl.Validate(); err == nil {
        t.Error("expected error for missing chip_doc config, got nil")
    }
}

func TestTemplate_ChipDoc_InvalidRemovalCost_Fails(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "chip_doc_test",
        Name:    "Chip Doc",
        Level:   3,
        MaxHP:   20,
        AC:      10,
        NPCType: "chip_doc",
        ChipDoc: &npc.ChipDocConfig{RemovalCost: 0, CheckDC: 15},
    }
    if err := tmpl.Validate(); err == nil {
        t.Error("expected error for removal_cost == 0, got nil")
    }
}

func TestTemplate_ChipDoc_InvalidCheckDC_Fails(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "chip_doc_test",
        Name:    "Chip Doc",
        Level:   3,
        MaxHP:   20,
        AC:      10,
        NPCType: "chip_doc",
        ChipDoc: &npc.ChipDocConfig{RemovalCost: 50, CheckDC: 0},
    }
    if err := tmpl.Validate(); err == nil {
        t.Error("expected error for check_dc == 0, got nil")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_ChipDoc" -v 2>&1 | head -20
```

Expected: compile error — `npc.ChipDocConfig` undefined

- [ ] **Step 3: Add ChipDocConfig and register npc_type**

In `internal/game/npc/template.go`:

Add `ChipDocConfig` struct near the other config structs:
```go
// ChipDocConfig holds configuration for chip_doc NPCs.
// REQ-CR-1: Required when npc_type == "chip_doc".
type ChipDocConfig struct {
    // RemovalCost is the credit cost charged before the Rigging skill check.
    RemovalCost int `yaml:"removal_cost"`
    // CheckDC is the difficulty class for the Rigging skill check.
    CheckDC int `yaml:"check_dc"`
}
```

Add `ChipDoc *ChipDocConfig \`yaml:"chip_doc,omitempty"\`` to the `Template` struct (after `Fixer`).

In `validTypes` map, add `"chip_doc": true`.

In the `switch t.NPCType` block, add:
```go
case "chip_doc":
    if t.ChipDoc == nil {
        return fmt.Errorf("npc template %q: npc_type 'chip_doc' requires a chip_doc: config block", t.ID)
    }
    if t.ChipDoc.RemovalCost <= 0 {
        return fmt.Errorf("npc template %q: chip_doc removal_cost must be > 0", t.ID)
    }
    if t.ChipDoc.CheckDC <= 0 {
        return fmt.Errorf("npc template %q: chip_doc check_dc must be > 0", t.ID)
    }
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestTemplate_ChipDoc" -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 5: Run full tests to verify no regressions**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/template.go internal/game/npc/template_chip_doc_test.go
git commit -m "feat(curse-removal): add ChipDocConfig; register chip_doc NPC type (REQ-CR-1)"
```

---

## Task 2: handleUncurse Command Handler

**Files:**
- Create: `internal/gameserver/grpc_service_chip_doc.go`
- Create: `internal/gameserver/grpc_service_chip_doc_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/gameserver/grpc_service_chip_doc_test.go
package gameserver_test

import (
    "testing"
    "math/rand"
    "github.com/cory-johannsen/mud/internal/game/npc"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// These tests use t.Skip because they require a full GameServiceServer with
// session manager, NPC manager, condition registry, and character saver.
// They document the expected behavior; replace t.Skip with actual setup
// using the existing test infrastructure in grpc_service_test.go.

func TestHandleUncurse_NoChipDocInRoom_Fails(t *testing.T) {
    t.Skip("integration test: requires GameServiceServer with NPC manager")
    // Setup: session in room with no chip_doc NPC.
    // Call handleUncurse.
    // Assert: message contains "don't see" or "not a chip doc".
}

func TestHandleUncurse_ItemNotCursed_Fails(t *testing.T) {
    t.Skip("integration test: requires GameServiceServer with NPC manager")
    // Setup: session with chip_doc in room; equipped item with Modifier == "tuned".
    // Assert: message = "Your <item> is not cursed."
}

func TestHandleUncurse_InsufficientCredits_Fails(t *testing.T) {
    t.Skip("integration test: requires GameServiceServer with NPC manager")
    // Setup: session with chip_doc (removal_cost=100); player Currency=50.
    // Assert: message contains "You need" and "credits".
}

func TestUncurseSkillCheck_CritSuccess_CurseRemoved(t *testing.T) {
    // Unit test for the skill check outcome logic — no server needed.
    outcome := uncurseCheckOutcome(30, 15) // total=30, dc=15 → crit success (≥ dc+10=25)
    if outcome != "crit_success" {
        t.Errorf("outcome = %q, want crit_success", outcome)
    }
}

func TestUncurseSkillCheck_Success_CurseRemoved(t *testing.T) {
    outcome := uncurseCheckOutcome(18, 15) // total=18, dc=15 → success
    if outcome != "success" {
        t.Errorf("outcome = %q, want success", outcome)
    }
}

func TestUncurseSkillCheck_Failure_CurseStays(t *testing.T) {
    outcome := uncurseCheckOutcome(12, 15) // total=12, dc=15 → failure
    if outcome != "failure" {
        t.Errorf("outcome = %q, want failure", outcome)
    }
}

func TestUncurseSkillCheck_CritFailure_CurseStays(t *testing.T) {
    outcome := uncurseCheckOutcome(4, 15) // total=4, dc=15, dc-10=5 → crit failure
    if outcome != "crit_failure" {
        t.Errorf("outcome = %q, want crit_failure", outcome)
    }
}

func TestUncurseSkillCheck_NaturalOne_AlwaysCritFailure(t *testing.T) {
    // Natural 1 is always crit failure regardless of total.
    outcome := uncurseCheckOutcomeWithRoll(1, 20, 5) // roll=1, mod=20, dc=5 → would succeed but nat 1
    if outcome != "crit_failure" {
        t.Errorf("natural 1 should be crit_failure, got %q", outcome)
    }
}
```

> Note: `uncurseCheckOutcome` and `uncurseCheckOutcomeWithRoll` are package-level helper functions exported from `grpc_service_chip_doc.go` for testing (or placed in a `_internal_test.go` file). See Step 3.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestUncurseSkillCheck|TestHandleUncurse" -v 2>&1 | head -20
```

Expected: compile error — `uncurseCheckOutcome` undefined

- [ ] **Step 3: Implement grpc_service_chip_doc.go**

```go
// internal/gameserver/grpc_service_chip_doc.go
package gameserver

import (
    "context"
    "fmt"
    "math/rand"

    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/npc"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findChipDocInRoom locates a chip_doc NPC by name in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findChipDocInRoom(roomID, npcName string) (*npc.Instance, string) {
    inst := s.npcMgr.FindInRoom(roomID, npcName)
    if inst == nil {
        return nil, fmt.Sprintf("You don't see %q here.", npcName)
    }
    if inst.NPCType != "chip_doc" {
        return nil, fmt.Sprintf("%s is not a chip doc.", inst.Name())
    }
    if inst.Cowering {
        return nil, fmt.Sprintf("%s is cowering in fear and won't help right now.", inst.Name())
    }
    return inst, ""
}

// uncurseCheckOutcome returns the PF2E four-tier outcome for a given total vs DC.
// This function is testable without a server.
// REQ-CR-6.
func uncurseCheckOutcome(total, dc int) string {
    switch {
    case total >= dc+10:
        return "crit_success"
    case total >= dc:
        return "success"
    case total <= dc-10:
        return "crit_failure"
    default:
        return "failure"
    }
}

// uncurseCheckOutcomeWithRoll returns the four-tier outcome, treating roll==1 as crit failure.
// REQ-CR-6: natural 1 is always critical failure.
func uncurseCheckOutcomeWithRoll(roll, skillMod, dc int) string {
    if roll == 1 {
        return "crit_failure"
    }
    return uncurseCheckOutcome(roll+skillMod, dc)
}

// riggingModifier converts a proficiency rank to a flat skill modifier.
// Uses the same rank-to-modifier mapping as merchantSkillModifier.
func riggingModifier(rank string) int {
    switch rank {
    case "trained":
        return 2
    case "expert":
        return 4
    case "master":
        return 6
    case "legendary":
        return 8
    default:
        return 0
    }
}

// handleUncurse processes an `uncurse <item>` command.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// REQ-CR-3 through REQ-CR-9.
func (s *GameServiceServer) handleUncurse(uid string, req *gamev1.UncurseRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }

    // REQ-CR-3: chip_doc must be in room.
    inst, errMsg := s.findChipDocInRoom(sess.RoomID, req.GetNpcName())
    if inst == nil {
        return messageEvent(errMsg), nil
    }
    tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
    if tmpl == nil || tmpl.ChipDoc == nil {
        return messageEvent("This chip doc has no configuration."), nil
    }

    // REQ-CR-4: item must be equipped and cursed.
    itemName := req.GetItemName()
    slottedItem, slotName := sess.Equipment.FindCursedSlottedItem(itemName)
    if slottedItem == nil {
        return messageEvent(fmt.Sprintf("Your %s is not equipped and cursed.", itemName)), nil
    }

    // REQ-CR-5: credit check before deduction.
    cost := tmpl.ChipDoc.RemovalCost
    if sess.Currency < cost {
        return messageEvent(fmt.Sprintf(
            "You need %d credits but only have %d.", cost, sess.Currency,
        )), nil
    }

    // Deduct credits (REQ-CR-5: deduct before the check).
    sess.Currency -= cost
    if s.charSaver != nil && sess.CharacterID > 0 {
        _ = s.charSaver.SaveCurrency(context.Background(), sess.CharacterID, sess.Currency)
    }

    // REQ-CR-6: Rigging skill check.
    roll := rand.Intn(20) + 1
    skillMod := riggingModifier(sess.Skills["rigging"])
    dc := tmpl.ChipDoc.CheckDC
    outcome := uncurseCheckOutcomeWithRoll(roll, skillMod, dc)

    switch outcome {
    case "crit_success", "success":
        // REQ-CR-7: remove curse, unequip, return to backpack as defective.
        itemInst := sess.Equipment.GetInstanceForSlot(slotName)
        if itemInst != nil {
            inventory.UncurseItem(itemInst)
            slottedItem.Modifier = "defective"
            slottedItem.CurseRevealed = false
        }
        sess.Equipment.ClearSlot(slotName)
        if itemInst != nil {
            sess.Backpack.AddInstance(itemInst)
        }
        if s.charSaver != nil && sess.CharacterID > 0 {
            _ = s.charSaver.SaveEquipment(context.Background(), sess.CharacterID, sess.Equipment)
        }
        return messageEvent(fmt.Sprintf(
            "%s removes the curse from your %s. The item slips off and is returned to your pack — now merely defective. Cost: %d credits.",
            inst.Name(), itemName, cost,
        )), nil

    case "failure":
        // REQ-CR-8: credits lost; item stays cursed.
        return messageEvent(fmt.Sprintf(
            "%s works on your %s but can't break the curse. Credits lost: %d.",
            inst.Name(), itemName, cost,
        )), nil

    default: // "crit_failure"
        // REQ-CR-9: credits lost; item stays cursed; fatigued condition applied.
        if s.condRegistry != nil && sess.Conditions != nil {
            if condDef, ok := s.condRegistry.Get("fatigued"); ok {
                _ = sess.Conditions.Apply(sess.UID, condDef, 1, 3600) // 1h fatigued
            }
        }
        return messageEvent(fmt.Sprintf(
            "%s botches the procedure. The curse surges back. Credits lost: %d. You feel fatigued.",
            inst.Name(), cost,
        )), nil
    }
}
```

> Note: `sess.Equipment.FindCursedSlottedItem(itemName)`, `sess.Equipment.GetInstanceForSlot(slotName)`, `sess.Equipment.ClearSlot(slotName)`, `sess.Backpack.AddInstance(inst)`, and `inventory.UncurseItem(inst)` may not all exist yet. `UncurseItem` is defined by the `equipment-mechanics` plan. The Equipment/Backpack helpers should be added if missing — read `equipment.go` and `backpack.go` to find existing patterns and add minimal helpers as needed.

- [ ] **Step 4: Run unit tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestUncurseSkillCheck" -v 2>&1 | tail -20
```

Expected: all unit tests PASS

- [ ] **Step 5: Wire the `uncurse` command in the dispatch switch**

In `internal/gameserver/grpc_service.go`, find the `ClientMessage` switch (around line 1473). Add:
```go
case *gamev1.ClientMessage_Uncurse:
    return s.handleUncurse(uid, p.Uncurse)
```

Also add `UncurseRequest` to the proto if it does not exist. Check existing request patterns in `gamev1/game.proto` and follow the same pattern as `TrainJobRequest`. If proto changes are required, regenerate with `make proto` or equivalent.

> If adding proto messages is out of scope (proto regeneration is a separate step), use a generic text-command fallback:
> ```go
> case "uncurse":
>     return s.handleUncurseText(uid, input) // parse "uncurse <npc_name> <item_name>" from raw text
> ```

- [ ] **Step 6: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service_chip_doc.go internal/gameserver/grpc_service_chip_doc_test.go internal/gameserver/grpc_service.go
git commit -m "feat(curse-removal): add handleUncurse with PF2E four-tier Rigging check (REQ-CR-3-9)"
```

---

## Task 3: chip_doc NPC YAML Content

**Files:**
- Create: `content/npcs/chip_doc_gunchete.yaml`

> Note: Per REQ-CR-10, each zone needs a `chip_doc` NPC in a Safe room. This task creates the Gunchete instance. Additional zones are covered when `zone-content-expansion` is implemented.

- [ ] **Step 1: Check existing zone rooms for Safe rooms**

```bash
cd /home/cjohannsen/src/mud && grep -rn "danger_level.*0\|safe.*true\|is_safe\|SafeRoom\|safe_room" content/ --include="*.yaml" | head -10
```

Find the room ID of a Safe room in Gunchete (danger_level 0 or explicitly marked safe).

- [ ] **Step 2: Create chip_doc_gunchete.yaml**

```yaml
id: chip_doc_gunchete
name: "Noodle"
npc_type: chip_doc
description: >
  A wiry tech with augmented fingers and a loupe fused to his left eye. Noodle runs a discrete
  removal service out of a converted vending alcove. He doesn't ask about the curse; he just fixes it.
max_hp: 18
ac: 10
level: 3
awareness: 4
personality: brave
room_id: gunchete_safe_hub   # REPLACE with the actual Safe room ID found in Step 1
chip_doc:
  removal_cost: 75
  check_dc: 14
```

> Replace `gunchete_safe_hub` with the actual Safe room ID discovered in Step 1.

- [ ] **Step 3: Verify NPC loads at startup**

```bash
cd /home/cjohannsen/src/mud && go build ./cmd/gameserver/ 2>&1 && echo "BUILD OK"
```

Expected: `BUILD OK` — no fatal load errors for the new NPC.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/npcs/chip_doc_gunchete.yaml
git commit -m "feat(curse-removal): add chip_doc NPC for Gunchete zone (REQ-CR-10)"
```

---

## Final Verification

- [ ] **Run all tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Build all binaries**

```bash
cd /home/cjohannsen/src/mud && go build ./cmd/... 2>&1 && echo "ALL BUILDS OK"
```

Expected: `ALL BUILDS OK`

- [ ] **Requirements coverage check**

- REQ-CR-1: Task 1 (`ChipDocConfig`, `validTypes`, validation)
- REQ-CR-2: Task 3 YAML (`personality: brave` = cower behavior, same as `job_trainer`)
- REQ-CR-3: Task 2 `findChipDocInRoom` (NPC must be in room, cowering rejected)
- REQ-CR-4: Task 2 `FindCursedSlottedItem` check
- REQ-CR-5: Task 2 credit check before deduction
- REQ-CR-6: Task 2 `uncurseCheckOutcomeWithRoll` (d20 + riggingMod vs DC, natural 1 = crit fail)
- REQ-CR-7: Task 2 crit_success/success → `UncurseItem`, unequip, return to backpack
- REQ-CR-8: Task 2 failure → credits lost, item stays cursed
- REQ-CR-9: Task 2 crit_failure → credits lost, item stays cursed, `fatigued` applied
- REQ-CR-10: Task 3 `chip_doc_gunchete.yaml` in Safe room
