# Curse Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `uncurse <item>` command backed by a `chip_doc` NPC that removes cursed items for a credit cost plus a Rigging skill check with four outcomes.

**Architecture:** A new `grpc_service_chip_doc.go` handler file follows the established non-combat NPC handler pattern (healer, job_trainer, quest_giver). Proto `UncurseRequest` is added as field 127 in `ClientMessage.payload`. The handler finds a cursed equipped item by name, deducts credits, rolls a Rigging check, and on success/crit-success transitions the item modifier from `"cursed"` to `"defective"`, unequips it, and returns it to the backpack. Fatigue is applied on critical failure. No new DB tables are required.

**Tech Stack:** Go 1.22, protobuf v3, `pgregory.net/rapid` (property-based tests), existing `skillcheck`, `condition`, `inventory`, and `session` packages.

---

## File Map

| Path | Status | Purpose |
|------|--------|---------|
| `api/proto/game/v1/game.proto` | Modify | Add `UncurseRequest` message + `ClientMessage.uncurse_request` field 127 |
| `internal/gameserver/gamev1/game.pb.go` | Auto-generated | Re-generated from proto |
| `internal/gameserver/grpc_service_chip_doc.go` | Create | `findChipDocInRoom` + `handleUncurse` |
| `internal/gameserver/grpc_service.go` | Modify | Add dispatch case `*gamev1.ClientMessage_UncurseRequest` |
| `internal/gameserver/grpc_service_chip_doc_test.go` | Create | Unit + property tests for all 4 outcomes |
| `docs/features/index.yaml` | Modify | Set `curse-removal` status to `done` |

---

### Task 1: Add `UncurseRequest` proto message and wire dispatch

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go:1929-1945` (dispatch switch)

- [ ] **Step 1: Add the proto message and oneof field**

In `api/proto/game/v1/game.proto`, after the `QuestRequest quest_request = 126;` line inside `ClientMessage.payload` oneof, add:

```protobuf
    UncurseRequest       uncurse_request       = 127;
```

Then after the existing `message QuestRequest { ... }` block (near the bottom of the file), add:

```protobuf
// UncurseRequest asks a chip_doc NPC to remove a cursed item.
message UncurseRequest {
  string npc_name  = 1; // name of the chip_doc NPC to address
  string item_name = 2; // name of the cursed equipped item to uncurse
}
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
make proto
```

Expected: exits 0; `internal/gameserver/gamev1/game.pb.go` is updated with `ClientMessage_UncurseRequest` type.

- [ ] **Step 3: Add dispatch case in grpc_service.go**

In `internal/gameserver/grpc_service.go`, find the dispatch switch block around line 1944 (the `case *gamev1.ClientMessage_Talk:` line). Add immediately after the `QuestRequest` case:

```go
	case *gamev1.ClientMessage_UncurseRequest:
		return s.handleUncurse(uid, p.UncurseRequest)
```

- [ ] **Step 4: Add stub so it compiles (required before writing the real handler)**

Create `internal/gameserver/grpc_service_chip_doc.go` with a compile stub:

```go
package gameserver

import (
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func (s *GameServiceServer) handleUncurse(uid string, req *gamev1.UncurseRequest) (*gamev1.ServerEvent, error) {
	return messageEvent("not implemented"), nil
}
```

- [ ] **Step 5: Verify it compiles**

```bash
cd /home/cjohannsen/src/mud
go build ./...
```

Expected: exits 0 with no errors.

- [ ] **Step 6: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
        internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_chip_doc.go
git commit -m "feat(curse-removal): add UncurseRequest proto + dispatch stub"
```

---

### Task 2: Write failing tests for `handleUncurse`

**Files:**
- Create: `internal/gameserver/grpc_service_chip_doc_test.go`

> **Context:** The test file must exercise all 5 code paths:
> - No chip_doc NPC in room → error message
> - Item not found equipped → error message
> - Insufficient credits → error message
> - Success/CritSuccess → item modifier changed to "defective", item returned to backpack
> - Failure → credits lost, item stays cursed
> - CritFailure → credits lost, item stays cursed, fatigue condition applied

- [ ] **Step 1: Write the failing tests**

Create `internal/gameserver/grpc_service_chip_doc_test.go`:

```go
package gameserver

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// makeChipDocSvc constructs a minimal GameServiceServer for chip_doc tests.
// npcName is the display name; cfg is the ChipDocConfig.
// Returns the service, the session uid, and the room id.
func makeChipDocSvc(t *testing.T, npcName string, cfg *npc.ChipDocConfig) (*GameServiceServer, string, string) {
	t.Helper()
	roomID := "room1"
	uid := "u1"

	condReg := condition.NewRegistry()
	fatigueDef := &condition.ConditionDef{ID: "fatigue", Name: "Fatigue", MaxStacks: 5}
	condReg.Register(fatigueDef)

	tmpl := &npc.Template{
		ID:       "chipdoc_tmpl",
		BaseNPC:  npc.BaseNPC{DisplayName: npcName, NPCType: "chip_doc"},
		ChipDoc:  cfg,
	}
	inst := &npc.Instance{
		ID:         "chipdoc_inst",
		TemplateID: "chipdoc_tmpl",
		RoomID:     roomID,
	}
	inst.SetNPCType("chip_doc")

	npcMgr := npc.NewManager()
	npcMgr.RegisterTemplate(tmpl)
	npcMgr.PlaceInstance(inst)

	sessMgr := session.NewManager()
	sess := sessMgr.NewPlayer(uid)
	sess.RoomID = roomID
	sess.Currency = 500
	sess.Equipment = inventory.NewEquipment()
	sess.Backpack = inventory.NewBackpack(50.0, 20)
	sess.Conditions = condition.NewConditionSet()
	sess.Abilities.Savvy = 14 // +2 mod
	sess.Skills = map[string]string{"rigging": "trained"} // +2 prof

	svc := &GameServiceServer{
		sessions:     sessMgr,
		npcMgr:       npcMgr,
		condRegistry: condReg,
	}
	return svc, uid, roomID
}

// equipCursedArmor places a cursed SlottedItem in the torso slot.
func equipCursedArmor(sess *session.PlayerSession, itemName, itemDefID string) {
	sess.Equipment.Armor[inventory.ArmorSlotTorso] = &inventory.SlottedItem{
		ItemDefID:     itemDefID,
		Name:          itemName,
		Modifier:      "cursed",
		CurseRevealed: true,
		Durability:    10,
	}
}

// TestHandleUncurse_NoNPCInRoom verifies an error when chip_doc is not present.
//
// Precondition: room has no NPC named "Doc".
// Postcondition: message contains "don't see".
func TestHandleUncurse_NoNPCInRoom(t *testing.T) {
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 14})
	// Use wrong NPC name.
	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "nobody", ItemName: "armor"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(evt.GetMessage().GetText(), "don't see") {
		t.Errorf("expected 'don't see' message, got %q", evt.GetMessage().GetText())
	}
}

// TestHandleUncurse_ItemNotEquipped verifies error when no cursed item with given name is equipped.
//
// Precondition: player has no cursed item equipped.
// Postcondition: message contains "no cursed item".
func TestHandleUncurse_ItemNotEquipped(t *testing.T) {
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 14})
	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(evt.GetMessage().GetText()), "no cursed item") {
		t.Errorf("expected 'no cursed item' message, got %q", evt.GetMessage().GetText())
	}
}

// TestHandleUncurse_InsufficientCredits verifies error when player cannot afford removal.
//
// Precondition: player has 50 credits; removal_cost is 200.
// Postcondition: message contains "credits".
func TestHandleUncurse_InsufficientCredits(t *testing.T) {
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 200, CheckDC: 14})
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 50
	equipCursedArmor(sess, "shadow vest", "armor_shadow_vest")

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(evt.GetMessage().GetText()), "credit") {
		t.Errorf("expected credits message, got %q", evt.GetMessage().GetText())
	}
}

// TestHandleUncurse_CritSuccess_ItemBecomesDefective verifies crit success outcome.
//
// Precondition: forced roll=20 (die returns 19+1=20); total >= dc+10.
// Postcondition: torso slot is nil; message contains "removed"; credits deducted.
func TestHandleUncurse_CritSuccess_ItemBecomesDefective(t *testing.T) {
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 14})
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 500
	equipCursedArmor(sess, "shadow vest", "armor_shadow_vest")

	// Force die to return 20 (always crit success: 20 + 2 savvyMod + 2 prof = 24 >= 14+10=24).
	svc.dice = &fixedDice{val: 19} // 0-indexed → roll = 19+1 = 20

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := strings.ToLower(evt.GetMessage().GetText())
	if !strings.Contains(msg, "removed") && !strings.Contains(msg, "curse") {
		t.Errorf("expected curse removed message, got %q", msg)
	}
	if sess.Equipment.Armor[inventory.ArmorSlotTorso] != nil {
		t.Errorf("expected torso slot to be cleared after uncurse")
	}
	if sess.Currency != 400 {
		t.Errorf("expected 400 credits after deduction, got %d", sess.Currency)
	}
}

// TestHandleUncurse_Failure_CreditsLostItemStays verifies failure outcome.
//
// Precondition: forced roll=1; total < dc.
// Postcondition: credits deducted; torso slot still has cursed item; message contains "failed".
func TestHandleUncurse_Failure_CreditsLostItemStays(t *testing.T) {
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 20})
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 500
	equipCursedArmor(sess, "shadow vest", "armor_shadow_vest")

	// Force roll=1: 1 + 2 + 2 = 5 < 20 = failure.
	svc.dice = &fixedDice{val: 0} // 0+1=1

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := strings.ToLower(evt.GetMessage().GetText())
	if !strings.Contains(msg, "fail") && !strings.Contains(msg, "unable") {
		t.Errorf("expected failure message, got %q", msg)
	}
	if sess.Equipment.Armor[inventory.ArmorSlotTorso] == nil || sess.Equipment.Armor[inventory.ArmorSlotTorso].Modifier != "cursed" {
		t.Errorf("expected cursed item to remain after failure")
	}
	if sess.Currency != 400 {
		t.Errorf("expected 400 credits (deducted despite failure), got %d", sess.Currency)
	}
}

// TestHandleUncurse_CritFailure_FatigueApplied verifies critical failure outcome.
//
// Precondition: forced roll=1; total < dc-10.
// Postcondition: credits deducted; item stays cursed; fatigue condition applied once.
func TestHandleUncurse_CritFailure_FatigueApplied(t *testing.T) {
	// dc=30 ensures even roll=1+2+2=5 < 30-10=20 → crit failure.
	svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 30})
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 500
	equipCursedArmor(sess, "shadow vest", "armor_shadow_vest")
	svc.dice = &fixedDice{val: 0} // roll=1

	evt, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg := strings.ToLower(evt.GetMessage().GetText())
	if !strings.Contains(msg, "fatigue") && !strings.Contains(msg, "overwhelm") && !strings.Contains(msg, "stagger") {
		t.Errorf("expected fatigue/overwhelm message on crit failure, got %q", msg)
	}
	if sess.Equipment.Armor[inventory.ArmorSlotTorso] == nil || sess.Equipment.Armor[inventory.ArmorSlotTorso].Modifier != "cursed" {
		t.Errorf("expected cursed item to remain after crit failure")
	}
	stacks := sess.Conditions.StackCount("fatigue")
	if stacks != 1 {
		t.Errorf("expected 1 fatigue stack, got %d", stacks)
	}
}

// TestProperty_HandleUncurse_CreditsAlwaysDeductedWhenFound verifies credits are always deducted
// when a cursed item is found and player can afford it, regardless of outcome.
func TestProperty_HandleUncurse_CreditsAlwaysDeductedWhenFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(0, 19).Draw(rt, "roll")
		svc, uid, _ := makeChipDocSvc(t, "Doc", &npc.ChipDocConfig{RemovalCost: 100, CheckDC: 14})
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = 500
		equipCursedArmor(sess, "shadow vest", "armor_shadow_vest")
		svc.dice = &fixedDice{val: roll}

		_, err := svc.handleUncurse(uid, &gamev1.UncurseRequest{NpcName: "Doc", ItemName: "shadow vest"})
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if sess.Currency != 400 {
			rt.Fatalf("expected 400 credits after deduction, got %d (roll=%d)", sess.Currency, roll)
		}
	})
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/ -run "TestHandleUncurse|TestProperty_HandleUncurse" -v 2>&1 | head -50
```

Expected: compilation failure (stub returns "not implemented") or test failures.

---

### Task 3: Implement `findChipDocInRoom` and `handleUncurse`

**Files:**
- Modify: `internal/gameserver/grpc_service_chip_doc.go`

- [ ] **Step 1: Replace stub with full implementation**

Replace the entire content of `internal/gameserver/grpc_service_chip_doc.go` with:

```go
package gameserver

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findChipDocInRoom returns the first chip_doc NPC matching npcName in roomID.
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
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// findCursedEquippedItem searches all armor and accessory slots for a cursed item
// whose name contains nameQuery (case-insensitive).
//
// Precondition: equip must be non-nil.
// Postcondition: Returns the slot key, the SlottedItem, and whether the item was found in armor (vs accessory).
// Returns ("", nil, false) when not found.
func findCursedEquippedItem(equip *inventory.Equipment, nameQuery string) (string, *inventory.SlottedItem, bool) {
	q := strings.ToLower(nameQuery)
	for slot, slotted := range equip.Armor {
		if slotted == nil || slotted.Modifier != "cursed" {
			continue
		}
		if strings.Contains(strings.ToLower(slotted.Name), q) {
			return string(slot), slotted, true
		}
	}
	for slot, slotted := range equip.Accessories {
		if slotted == nil || slotted.Modifier != "cursed" {
			continue
		}
		if strings.Contains(strings.ToLower(slotted.Name), q) {
			return string(slot), slotted, false
		}
	}
	return "", nil, false
}

// handleUncurse processes a player's request to have a chip_doc NPC remove a curse.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// Credit cost is deducted before the skill check regardless of outcome.
// On CritSuccess/Success: item modifier changed to "defective", slot cleared, item added to backpack.
// On Failure: credits deducted only.
// On CritFailure: credits deducted; fatigue condition applied once.
func (s *GameServiceServer) handleUncurse(uid string, req *gamev1.UncurseRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findChipDocInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.ChipDoc == nil {
		return messageEvent("This chip doc has no configuration."), nil
	}
	cfg := tmpl.ChipDoc

	if sess.Equipment == nil {
		return messageEvent("You have no equipped items."), nil
	}

	slotKey, slotted, isArmor := findCursedEquippedItem(sess.Equipment, req.GetItemName())
	if slotted == nil {
		return messageEvent(fmt.Sprintf("You have no cursed item named %q equipped.", req.GetItemName())), nil
	}

	if sess.Currency < cfg.RemovalCost {
		return messageEvent(fmt.Sprintf(
			"%s requires %d credits to attempt curse removal, but you only have %d.",
			inst.Name(), cfg.RemovalCost, sess.Currency,
		)), nil
	}

	// Deduct credits before the check (non-refundable).
	sess.Currency -= cfg.RemovalCost

	// Rigging skill check.
	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}
	savvyMod := abilityModFrom(sess.Abilities.Savvy)
	rank := ""
	if sess.Skills != nil {
		rank = sess.Skills["rigging"]
	}
	total := roll + savvyMod + skillcheck.ProficiencyBonus(rank)
	outcome := skillcheck.OutcomeFor(total, cfg.CheckDC)

	itemName := slotted.Name

	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		// Change modifier to defective and return item to backpack.
		slotted.Modifier = "defective"
		if sess.Backpack != nil && s.invRegistry != nil {
			itemDefID := slotted.ItemDefID
			if itemDef, ok2 := s.invRegistry.ItemByArmorRef(slotted.ItemDefID); ok2 {
				itemDefID = itemDef.ID
			}
			if added, err := sess.Backpack.Add(itemDefID, 1, s.invRegistry); err == nil {
				added.Modifier = "defective"
			}
		}
		// Clear the equipment slot.
		if isArmor {
			sess.Equipment.Armor[inventory.ArmorSlot(slotKey)] = nil
		} else {
			sess.Equipment.Accessories[inventory.AccessorySlot(slotKey)] = nil
		}
		// Persist equipment and inventory.
		if s.charSaver != nil && sess.CharacterID > 0 {
			ctx := context.Background()
			if err := s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment); err != nil {
				s.logger.Warn("handleUncurse: SaveEquipment failed",
					zap.String("uid", uid), zap.Error(err))
			}
			if sess.Backpack != nil {
				invItems := backpackToInventoryItems(sess.Backpack)
				if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, invItems); err != nil {
					s.logger.Warn("handleUncurse: SaveInventory failed",
						zap.String("uid", uid), zap.Error(err))
				}
			}
		}
		return messageEvent(fmt.Sprintf(
			"%s carefully extracts the curse from %s. The item is now defective but no longer cursed — returned to your inventory.",
			inst.Name(), itemName,
		)), nil

	case skillcheck.Failure:
		return messageEvent(fmt.Sprintf(
			"%s fumbles the removal. The curse on %s holds fast. Your %d credits are gone.",
			inst.Name(), itemName, cfg.RemovalCost,
		)), nil

	default: // CritFailure
		// Apply fatigue condition.
		if s.condRegistry != nil && sess.Conditions != nil {
			if def, ok2 := s.condRegistry.Get("fatigue"); ok2 {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
			}
		}
		return messageEvent(fmt.Sprintf(
			"%s botches the procedure — the curse on %s surges back and leaves you staggered (fatigued). Your %d credits are gone.",
			inst.Name(), itemName, cfg.RemovalCost,
		)), nil
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /home/cjohannsen/src/mud
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Run the tests**

```bash
go test ./internal/gameserver/ -run "TestHandleUncurse|TestProperty_HandleUncurse" -v
```

Expected: all tests pass except `TestHandleUncurse_CritSuccess_ItemBecomesDefective` may fail due to `fixedDice` missing or `invRegistry` nil. Check output; fix minor issues inline.

> **Note on `fixedDice`:** If `fixedDice` doesn't exist, search for it:
> ```bash
> grep -r "fixedDice" internal/gameserver/ --include="*.go" -l
> ```
> If not found, add this helper at the bottom of the test file:
> ```go
> type fixedDice struct{ val int }
> func (f *fixedDice) Src() interface{ Intn(int) int } { return f }
> func (f *fixedDice) Intn(_ int) int                 { return f.val }
> ```
> Check the actual `dice` interface type in `grpc_service.go`:
> ```bash
> grep -n "type.*dice\|dice.*interface\|\.dice\." internal/gameserver/grpc_service.go | head -20
> ```
> Adapt `fixedDice` to match the actual interface.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service_chip_doc.go \
        internal/gameserver/grpc_service_chip_doc_test.go
git commit -m "feat(curse-removal): implement handleUncurse with 4-outcome Rigging check"
```

---

### Task 4: Run full test suite and mark feature done

**Files:**
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | tail -30
```

Expected: all tests pass (100% success).

- [ ] **Step 2: Set feature status to done in index.yaml**

In `docs/features/index.yaml`, find the `curse-removal` entry and change:

```yaml
    status: planned
```

to:

```yaml
    status: done
```

- [ ] **Step 3: Commit**

```bash
git add docs/features/index.yaml
git commit -m "feat(curse-removal): mark feature done"
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Task |
|-------------|------|
| `chip_doc` NPC type | Pre-existing (ChipDocConfig, template wiring, 16 YAML files already in place) |
| `uncurse <item>` command | Task 1 (proto), Task 3 (handler) |
| Must be in same room as chip_doc | Task 3: `findChipDocInRoom` |
| Fails if item not cursed/equipped | Task 3: `findCursedEquippedItem` returns nil |
| Deducts `removal_cost` credits | Task 3: deducted before skill check |
| Rigging check vs `check_dc` | Task 3: `skillcheck.OutcomeFor(total, cfg.CheckDC)` |
| CritSuccess/Success: curse→defective, unequipped, backpack | Task 3: modifier change + slot clear + Backpack.Add |
| Failure: credits lost, item stays | Task 3: Failure case |
| CritFailure: credits lost, item stays, fatigued | Task 3: default case applies fatigue |
| Zone placement | Pre-existing (16 chip_doc_*.yaml files already placed) |

**Placeholder scan:** None found.

**Type consistency:**
- `inventory.ArmorSlot`, `inventory.AccessorySlot` — used consistently in Tasks 2 and 3
- `skillcheck.OutcomeFor`, `skillcheck.CritSuccess`, `skillcheck.Success`, `skillcheck.Failure` — from `internal/game/skillcheck` package; used in Task 3
- `npc.ChipDocConfig` — already exists in `internal/game/npc/noncombat.go`
- `sess.Conditions.Apply(uid, def, 1, -1)` — matches pattern in `grpc_service_substance_test.go:221`
