# Downtime Actions — Forge Papers Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the `forge_papers` downtime activity by adding three new item types (forgery_supplies, undetectable_forgery, convincing_forgery), gating activity start on having forgery_supplies in inventory, delivering output items on resolution, and updating docs to exclude Long-Term Rest and stub Craft/Retrain as separate features.

**Architecture:** Three new YAML item files are added to `content/items/`. `downtimeStart` in `grpc_service_downtime.go` gains a forge_papers gate that checks for `forgery_supplies` via `FindByItemDefID` and consumes one via `Remove`. `resolveForgePapers` in `grpc_service_downtime_resolvers.go` uses `AddInstance` to deliver output items; on CritSuccess it also refunds one `forgery_supplies`. Docs are updated to mark the feature done and create stubs for Craft and Retrain as new planned features.

**Tech Stack:** Go, testify, `pgregory.net/rapid`, YAML

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `content/items/forgery_supplies.yaml` | New consumable item used by forge_papers |
| Create | `content/items/undetectable_forgery.yaml` | Output item for crit success |
| Create | `content/items/convincing_forgery.yaml` | Output item for success |
| Modify | `internal/gameserver/grpc_service_downtime.go` | Add forge_papers pre-start gate |
| Modify | `internal/gameserver/grpc_service_downtime_resolvers.go` | Complete resolveForgePapers |
| Modify | `internal/gameserver/grpc_service_downtime_test.go` | Tests for forge_papers gate and resolver |
| Modify | `docs/features/downtime-actions.md` | Document Long-Term Rest exclusion, mark done |
| Create | `docs/features/craft-downtime.md` | Stub for Craft as separate planned feature |
| Create | `docs/features/retrain-downtime.md` | Stub for Retrain as separate planned feature |
| Modify | `docs/features/index.yaml` | Mark downtime-actions done, add craft-downtime and retrain-downtime as planned |

---

### Task 1: Add Item YAML Files

**Files:**
- Create: `content/items/forgery_supplies.yaml`
- Create: `content/items/undetectable_forgery.yaml`
- Create: `content/items/convincing_forgery.yaml`

- [ ] **Step 1: Create forgery_supplies.yaml**

```yaml
id: forgery_supplies
name: Forgery Supplies
description: A kit of specialized inks, paper stock, and precision tools used to forge official documents. Consumed when starting a Forge Papers downtime activity.
kind: tool
weight: 1.5
stackable: true
max_stack: 10
value: 50
tags:
  - forgery
  - consumable
```

Save to `content/items/forgery_supplies.yaml`.

- [ ] **Step 2: Create undetectable_forgery.yaml**

```yaml
id: undetectable_forgery
name: Undetectable Forgery
description: A masterfully produced set of false documents. Indistinguishable from the real thing to all but the most expert scrutiny.
kind: document
weight: 0.1
stackable: true
max_stack: 10
value: 200
tags:
  - forgery
  - document
```

Save to `content/items/undetectable_forgery.yaml`.

- [ ] **Step 3: Create convincing_forgery.yaml**

```yaml
id: convincing_forgery
name: Convincing Forgery
description: A competently produced set of false documents. Will fool most officials and casual inspections.
kind: document
weight: 0.1
stackable: true
max_stack: 10
value: 100
tags:
  - forgery
  - document
```

Save to `content/items/convincing_forgery.yaml`.

- [ ] **Step 4: Verify items load**

```bash
mise exec -- go run ./cmd/import-content/... --dry-run 2>&1 | grep -i "forgery\|error" || mise exec -- go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add content/items/forgery_supplies.yaml content/items/undetectable_forgery.yaml content/items/convincing_forgery.yaml
git commit -m "feat(downtime-actions): add forgery_supplies, undetectable_forgery, convincing_forgery items"
```

---

### Task 2: Gate Forge Papers Start on Forgery Supplies

**Files:**
- Modify: `internal/gameserver/grpc_service_downtime.go`
- Modify: `internal/gameserver/grpc_service_downtime_test.go` (or create `grpc_service_downtime_forgepapers_test.go`)

- [ ] **Step 1: Write failing tests**

Find the existing downtime test file:
```bash
ls internal/gameserver/grpc_service_downtime*test* 2>/dev/null || ls internal/gameserver/*downtime*test* 2>/dev/null
```

Add these tests to the appropriate test file (or create `internal/gameserver/grpc_service_downtime_forgepapers_test.go`):

```go
package gameserver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestDowntimeStart_ForgePapers_BlockedWithoutSupplies(t *testing.T) {
    svc, uid := newRestTestSvc(t) // reuse rest test server pattern
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.DowntimeBusy = false

    evt := svc.downtimeStart(uid, sess, "forge_papers")
    require.NotNil(t, evt)
    msg := extractEventMessage(evt)
    assert.Contains(t, msg, "forgery supplies", "should report missing supplies")
    assert.False(t, sess.DowntimeBusy, "should not start without supplies")
}

func TestDowntimeStart_ForgePapers_ConsumesSupplies(t *testing.T) {
    svc, uid := newRestTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    sess.DowntimeBusy = false
    require.NoError(t, sess.Backpack.AddInstance(&inventory.ItemInstance{
        InstanceID: "fs-1",
        ItemDefID:  "forgery_supplies",
        Quantity:   1,
    }))

    evt := svc.downtimeStart(uid, sess, "forge_papers")
    require.NotNil(t, evt)
    assert.True(t, sess.DowntimeBusy, "should start when supplies present")
    found := sess.Backpack.FindByItemDefID("forgery_supplies")
    assert.Empty(t, found, "forgery_supplies should be consumed")
}
```

NOTE: `extractEventMessage` is likely a helper already in the test file. Search for it:
```bash
grep -n "extractEventMessage\|func extract" internal/gameserver/grpc_service_downtime_test.go 2>/dev/null | head -5
```
If it doesn't exist, add:
```go
func extractEventMessage(evt *gamev1.ServerEvent) string {
    if msg := evt.GetMessage(); msg != nil {
        return msg.Text
    }
    return ""
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestDowntimeStart_ForgePapers" -v 2>&1 | head -30
```

Expected: FAIL — forge_papers currently starts without checking supplies.

- [ ] **Step 3: Add forge_papers gate to downtimeStart**

In `internal/gameserver/grpc_service_downtime.go`, after line:
```go
act, ok := downtime.ActivityByAlias(alias)
if !ok {
    return messageEvent("Unknown downtime activity.")
}
```

Add the forge_papers gate (before the `durationMin` line):

```go
// Gate forge_papers on having forgery_supplies in inventory. (REQ-DA-FORGE-1)
if act.ID == "forge_papers" {
    if sess.Backpack == nil {
        return messageEvent("You need forgery supplies to begin forging papers.")
    }
    instances := sess.Backpack.FindByItemDefID("forgery_supplies")
    if len(instances) == 0 {
        return messageEvent("You need forgery supplies to begin forging papers.")
    }
    // Consume one forgery_supplies at activity start.
    if err := sess.Backpack.Remove(instances[0].InstanceID, 1); err != nil {
        return messageEvent("You need forgery supplies to begin forging papers.")
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestDowntimeStart_ForgePapers" -v
```

Expected: PASS.

- [ ] **Step 5: Run full gameserver suite**

```bash
mise exec -- go test ./internal/gameserver/... -count=1 -timeout 120s
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_downtime.go internal/gameserver/grpc_service_downtime_forgepapers_test.go
git commit -m "feat(downtime-actions): gate forge_papers start on forgery_supplies inventory check"
```

---

### Task 3: Complete resolveForgePapers with Item Delivery

**Files:**
- Modify: `internal/gameserver/grpc_service_downtime_resolvers.go`
- Modify: `internal/gameserver/grpc_service_downtime_forgepapers_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/gameserver/grpc_service_downtime_forgepapers_test.go`:

```go
func TestResolveForgePapers_CritSuccess_DeliverUndetectableAndRefund(t *testing.T) {
    svc, uid := newRestTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    // Set up skill so crit success is guaranteed — override skillCheckOutcome via stub or
    // use the pattern from existing resolver tests (inspect grpc_service_downtime_test.go
    // for how outcome is controlled in tests).
    // If no override pattern exists, set sess.Skills so the hustle modifier is +999.
    // hustle skill: find the skill ID in content/skills.yaml or via grep.
    // Use a large enough bonus that DC 15 is always a crit success.
    sess.SkillBonus = map[string]int{"hustle": 999} // adjust field name to actual PlayerSession field

    svc.resolveForgePapers(uid, sess)

    undetectable := sess.Backpack.FindByItemDefID("undetectable_forgery")
    assert.Len(t, undetectable, 1, "should receive undetectable_forgery on crit success")
    refund := sess.Backpack.FindByItemDefID("forgery_supplies")
    assert.Len(t, refund, 1, "should receive forgery_supplies refund on crit success")
}

func TestResolveForgePapers_Success_DeliverConvincingForgery(t *testing.T) {
    svc, uid := newRestTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    // Set up so skill check always succeeds (not crit).
    // Inspect how skillCheckOutcome works — it may use a dice roller that can be seeded.
    // If svc has a mockDice or roller field, seed it for a success roll.
    // Alternatively: check if there's a test helper that forces outcome.
    // As fallback: run resolveForgePapers multiple times and assert at least one convincing_forgery appears.
    // Use the pattern from grpc_service_downtime_test.go for forcing outcomes.

    // Stub: if no seeding available, just verify function doesn't panic and returns a message.
    // Replace with outcome-forcing approach once you've read existing resolver tests.
    svc.resolveForgePapers(uid, sess)
    // At minimum: no panic, backpack not nil
    assert.NotNil(t, sess.Backpack)
}

func TestResolveForgePapers_Failure_NoItems(t *testing.T) {
    svc, uid := newRestTestSvc(t)
    sess, ok := svc.sessions.GetPlayer(uid)
    require.True(t, ok)
    // Force failure outcome (negative modifier).
    sess.SkillBonus = map[string]int{"hustle": -999} // adjust to actual field

    svc.resolveForgePapers(uid, sess)

    undetectable := sess.Backpack.FindByItemDefID("undetectable_forgery")
    convincing := sess.Backpack.FindByItemDefID("convincing_forgery")
    refund := sess.Backpack.FindByItemDefID("forgery_supplies")
    assert.Empty(t, undetectable, "no items on failure")
    assert.Empty(t, convincing, "no items on failure")
    assert.Empty(t, refund, "no refund on failure")
}
```

**IMPORTANT NOTE:** Before implementing Step 1, read `internal/gameserver/grpc_service_downtime_test.go` to understand how existing resolver tests control skill check outcomes. The `SkillBonus` field name above may not match — look at `PlayerSession` fields and `skillCheckOutcome` internals. Follow the exact pattern used elsewhere.

- [ ] **Step 2: Read existing resolver tests to understand outcome control**

```bash
grep -n "skillCheck\|SkillBonus\|outcome\|mock\|roller\|dice" internal/gameserver/grpc_service_downtime_test.go | head -20
grep -n "SkillBonus\|skillCheck\|Skills\b" internal/game/session/manager.go | head -20
```

Adjust the test setup in Step 1 to use the actual field names and patterns found.

- [ ] **Step 3: Run tests to verify they fail**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestResolveForgePapers" -v 2>&1 | head -30
```

Expected: FAIL — current stub sends messages only, delivers no items.

- [ ] **Step 4: Implement resolveForgePapers**

Replace the stub in `internal/gameserver/grpc_service_downtime_resolvers.go` (lines ~259-277):

```go
// resolveForgePapers resolves the "Forge Papers" downtime activity.
//
// Skill: hustle (Flair). DC: 15.
// CritSuccess: deliver "undetectable_forgery" + refund 1 "forgery_supplies".
// Success: deliver "convincing_forgery".
// Failure/CritFail: message only; supplies already consumed at start.
//
// Precondition: sess is non-nil; state already cleared; forgery_supplies consumed at activity start.
// Postcondition: Items added to backpack per outcome; console message delivered.
func (s *GameServiceServer) resolveForgePapers(uid string, sess *session.PlayerSession) {
    outcome := s.skillCheckOutcome(sess, "hustle", defaultDC)

    switch outcome {
    case skillcheck.CritSuccess:
        if sess.Backpack != nil {
            _ = sess.Backpack.AddInstance(&inventory.ItemInstance{
                InstanceID: uuid.New().String(),
                ItemDefID:  "undetectable_forgery",
                Quantity:   1,
            })
            // Refund one forgery_supplies on critical success. (REQ-DA-FORGE-2)
            _ = sess.Backpack.AddInstance(&inventory.ItemInstance{
                InstanceID: uuid.New().String(),
                ItemDefID:  "forgery_supplies",
                Quantity:   1,
            })
        }
        s.pushMessageToUID(uid, "Forge Papers complete. Critical success — you produced an undetectable forgery and recovered your supplies.")
    case skillcheck.Success:
        if sess.Backpack != nil {
            _ = sess.Backpack.AddInstance(&inventory.ItemInstance{
                InstanceID: uuid.New().String(),
                ItemDefID:  "convincing_forgery",
                Quantity:   1,
            })
        }
        s.pushMessageToUID(uid, "Forge Papers complete. Success — you produced a convincing forgery.")
    case skillcheck.Failure:
        s.pushMessageToUID(uid, "Forge Papers complete. Failure — the papers look suspicious. Your supplies are wasted.")
    default:
        s.pushMessageToUID(uid, "Forge Papers complete. Critical failure — you produced nothing usable and ruined your supplies.")
    }
}
```

**Check imports needed:** The file likely needs `inventory` and `uuid` imports. Verify:
```bash
grep -n "^import\|\"github.com/cory-johannsen/mud/internal/game/inventory\"\|\"github.com/google/uuid\"" internal/gameserver/grpc_service_downtime_resolvers.go | head -10
grep -rn "uuid.New\|google/uuid" internal/gameserver/ | head -5
```

Add any missing imports. The `uuid` package should already be vendored (used elsewhere in gameserver). If not, use `fmt.Sprintf("%d", time.Now().UnixNano())` as a fallback InstanceID — but check first.

- [ ] **Step 5: Run tests to verify they pass**

```bash
mise exec -- go test ./internal/gameserver/... -run "TestResolveForgePapers" -v
```

Expected: PASS.

- [ ] **Step 6: Run full gameserver suite**

```bash
mise exec -- go test ./internal/gameserver/... -count=1 -timeout 120s
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service_downtime_resolvers.go internal/gameserver/grpc_service_downtime_forgepapers_test.go
git commit -m "feat(downtime-actions): complete resolveForgePapers with item delivery and crit refund"
```

---

### Task 4: Update Docs and Mark Feature Done

**Files:**
- Modify: `docs/features/downtime-actions.md`
- Create: `docs/features/craft-downtime.md`
- Create: `docs/features/retrain-downtime.md`
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Update downtime-actions.md**

Read the current file:
```bash
cat docs/features/downtime-actions.md
```

Mark all downtime activity checkboxes as complete, and add a "Excluded Activities" section documenting Long-Term Rest:

Add before or after the requirements list:

```markdown
## Excluded Activities

- **Long-Term Rest** — Intentionally excluded. Long-Term Rest restores the full HP pool after 8 hours of rest, which has no meaningful value in Gunchete's design (HP recovery is handled via the rest/camping system). Adding it as a downtime activity would duplicate existing mechanics and create confusion.
```

Also mark the `forge_papers` checkbox and any other completed activities as `[x]`.

- [ ] **Step 2: Create craft-downtime.md**

```markdown
# Craft (Downtime)

Craft a non-magical item using the Crafting skill. Requires a recipe, raw materials, and access to appropriate tools. The player spends downtime days to produce items at a reduced cost compared to purchasing them.

## Requirements

- REQ-CRAFT-DT-1: The `craft` downtime command MUST gate on the player having the required raw materials in inventory.
- REQ-CRAFT-DT-2: On success, the crafted item MUST be added to the player's backpack.
- REQ-CRAFT-DT-3: On failure, raw materials MUST NOT be refunded.
- REQ-CRAFT-DT-4: On critical success, one batch of raw materials MUST be refunded.
- REQ-CRAFT-DT-5: The crafting downtime activity MUST integrate with the existing `content/recipes.yaml` recipe system.
- REQ-CRAFT-DT-6: Available recipes MUST be filtered to those the player's character meets prerequisites for.
```

Save to `docs/features/craft-downtime.md`.

- [ ] **Step 3: Create retrain-downtime.md**

```markdown
# Retrain (Downtime)

Spend downtime to retrain a feat or skill advancement — replacing a previously chosen option with a new one of the same category.

## Requirements

- REQ-RETRAIN-DT-1: The `retrain` downtime command MUST present the player with a list of retrain-eligible feats and skill choices.
- REQ-RETRAIN-DT-2: The player MUST select the feat/skill to replace and the replacement before the activity begins.
- REQ-RETRAIN-DT-3: On completion, the old feat/skill MUST be removed and the new feat/skill MUST be added to the character.
- REQ-RETRAIN-DT-4: Changes MUST be persisted via the character repository.
- REQ-RETRAIN-DT-5: Retraining MUST be blocked if the feat/skill being replaced is a prerequisite for another feat the character currently has.
```

Save to `docs/features/retrain-downtime.md`.

- [ ] **Step 4: Update index.yaml**

In `docs/features/index.yaml`:

1. Change `downtime-actions` status from `planned` to `done`.
2. Add `craft-downtime` as a new entry with status `planned`.
3. Add `retrain-downtime` as a new entry with status `planned`.

Find the downtime-actions entry and update it:
```yaml
  - slug: downtime-actions
    name: Downtime Actions
    status: done
```

Add new entries:
```yaml
  - slug: craft-downtime
    name: Craft (Downtime)
    status: planned

  - slug: retrain-downtime
    name: Retrain (Downtime)
    status: planned
```

- [ ] **Step 5: Run build check**

```bash
mise exec -- go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add docs/features/downtime-actions.md docs/features/craft-downtime.md docs/features/retrain-downtime.md docs/features/index.yaml
git commit -m "docs: mark downtime-actions done; exclude Long-Term Rest; stub craft-downtime and retrain-downtime"
```

---

## Verification Checklist

- [ ] `forge_papers` blocked with message when `forgery_supplies` not in inventory
- [ ] `forge_papers` starts and consumes one `forgery_supplies` when present
- [ ] CritSuccess: `undetectable_forgery` + `forgery_supplies` refund added to backpack
- [ ] Success: `convincing_forgery` added to backpack
- [ ] Failure: no items delivered, no refund
- [ ] CritFail: no items delivered, no refund
- [ ] Full test suite passes with zero failures
- [ ] downtime-actions marked done in index.yaml
- [ ] Long-Term Rest exclusion documented
- [ ] craft-downtime and retrain-downtime feature stubs created
