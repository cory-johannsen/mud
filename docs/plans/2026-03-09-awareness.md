# Awareness (Perception) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement PF2E Perception as "Awareness" — a static bonus displayed on the character sheet, computed as `10 + SavvyMod + CombatProficiencyBonus(level, awarenessRank)`.

**Architecture:** Three changes: (1) add `awareness` field to `CharacterSheetView` proto; (2) populate it in `handleChar` in `grpc_service.go` using the existing `CombatProficiencyBonus` helper and the character's `Proficiencies["awareness"]` rank; (3) display it in `RenderCharacterSheet` in the Saves section. No new storage needed — proficiency ranks already persist in `character_proficiencies` table via the existing `Proficiencies` map on the session.

**Tech Stack:** Go, protobuf (game.proto), existing `combat.CombatProficiencyBonus`, `combat.AbilityMod`

---

## Task 1: Add `awareness` field to CharacterSheetView proto

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/gamev1/game.pb.go` (regenerated)

The next available field number in `CharacterSheetView` is 41 (gender is 40).

**Step 1: Add field to CharacterSheetView.**

Find the `CharacterSheetView` message in `game.proto`. After `string gender = 40;`, add:

```proto
    int32  awareness = 41; // static bonus: 10 + savvy_mod + awareness_proficiency_bonus
```

**Step 2: Regenerate.**
```bash
cd /home/cjohannsen/src/mud && make proto 2>&1
```
Expected: no errors.

**Step 3: Build check.**
```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```
Expected: no errors.

**Step 4: Commit.**
```bash
cd /home/cjohannsen/src/mud && git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): add awareness field to CharacterSheetView"
```

---

## Task 2: Compute and populate awareness in handleChar

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

**Formula:** `awareness = 10 + AbilityMod(sess.Abilities.Savvy) + CombatProficiencyBonus(level, sess.Proficiencies["awareness"])`

This follows the same pattern as the saves block (lines ~2772-2778 in `grpc_service.go`):
```go
view.ToughnessSave = int32(combat.AbilityMod(sess.Abilities.Grit) +
    combat.CombatProficiencyBonus(level, sess.Proficiencies["toughness"]))
```

**Step 1: Write failing test in `grpc_service_test.go` (or nearest relevant test file).**

Look at how `handleChar` is tested. Search:
```bash
grep -n "TestHandleChar\|handleChar" /home/cjohannsen/src/mud/internal/gameserver/grpc_service_test.go | head -10
```

Add a test:
```go
func TestHandleChar_Awareness_TrainedRank(t *testing.T) {
    // Setup: player with Savvy=14 (mod=+2), trained awareness, level=1
    // Expected: awareness = 10 + 2 + (1+2) = 15
    worldMgr, sessMgr := testWorldAndSession(t)
    _ = worldMgr
    uid := "player-uid"
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "tester", CharName: "Tester",
        RoomID: "grinders_row", Role: "player", CharacterID: 0,
        CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Abilities.Savvy = 14  // mod = +2
    sess.Level = 1
    sess.Proficiencies = map[string]string{"awareness": "trained"}

    svc := testMinimalService(t, sessMgr)
    evt, err := svc.handleChar(uid)
    require.NoError(t, err)
    sheet := evt.GetCharacterSheet()
    require.NotNil(t, sheet)
    // 10 + AbilityMod(14) + CombatProficiencyBonus(1, "trained") = 10 + 2 + 3 = 15
    assert.Equal(t, int32(15), sheet.GetAwareness())
}

func TestHandleChar_Awareness_UntrainedRank(t *testing.T) {
    // Savvy=10 (mod=0), untrained (bonus=0), level=1 → awareness = 10
    worldMgr, sessMgr := testWorldAndSession(t)
    _ = worldMgr
    uid := "player-uid-2"
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "tester2", CharName: "Tester2",
        RoomID: "grinders_row", Role: "player", CharacterID: 0,
        CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Abilities.Savvy = 10
    sess.Level = 1
    sess.Proficiencies = map[string]string{}

    svc := testMinimalService(t, sessMgr)
    evt, err := svc.handleChar(uid)
    require.NoError(t, err)
    sheet := evt.GetCharacterSheet()
    require.NotNil(t, sheet)
    assert.Equal(t, int32(10), sheet.GetAwareness())
}
```

Note: `testMinimalService` was added in the grant task. Find it — it's in `grpc_service_grant_test.go`. If it's unexported/package-local that's fine since the test is in the same package.

Run to confirm failure:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleChar_Awareness -v 2>&1 | head -15
```

**Step 2: Populate awareness in `handleChar`.**

In `handleChar`, in the saves block (around line 2772), add after the cool save line:

```go
// Awareness: 10 + savvy_mod + awareness proficiency bonus.
view.Awareness = int32(10 + combat.AbilityMod(sess.Abilities.Savvy) +
    combat.CombatProficiencyBonus(level, sess.Proficiencies["awareness"]))
```

**Step 3: Run tests.**
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleChar_Awareness -v 2>&1
```
Expected: PASS

**Step 4: Run full suite.**
```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```

**Step 5: Commit.**
```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat(server): compute and populate Awareness in handleChar"
```

---

## Task 3: Display Awareness on the character sheet

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Spec:** Awareness appears in the `--- Saves ---` section of the character sheet, on the same row as Toughness/Hustle/Cool saves (or on its own row if that row is full). Format: `Awareness: +N` using `signedInt`.

The current saves line (around line 733):
```go
left = append(left, slPlain(fmt.Sprintf("Toughness: %s  Hustle: %s  Cool: %s",
    signedInt(int(csv.GetToughnessSave())),
    signedInt(int(csv.GetHustleSave())),
    signedInt(int(csv.GetCoolSave())))))
```

Add Awareness on a new line immediately after:
```go
left = append(left, slPlain(fmt.Sprintf("Awareness: %s",
    signedInt(int(csv.GetAwareness())))))
```

**Step 1: Write failing test.**

In `text_renderer_test.go`, add:
```go
func TestRenderCharacterSheet_ShowsAwareness(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        Name:      "TestChar",
        Awareness: 15,
    }
    rendered := RenderCharacterSheet(csv, 80)
    assert.Contains(t, telnet.StripANSI(rendered), "Awareness: +15")
}

func TestRenderCharacterSheet_ShowsAwareness_NoBonus(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        Name:      "TestChar",
        Awareness: 10,
    }
    rendered := RenderCharacterSheet(csv, 80)
    assert.Contains(t, telnet.StripANSI(rendered), "Awareness: +10")
}
```

Run to confirm failure:
```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsAwareness -v 2>&1
```

**Step 2: Add Awareness to the Saves section in `RenderCharacterSheet`.**

Find the saves block:
```go
left = append(left, slPlain(fmt.Sprintf("Toughness: %s  Hustle: %s  Cool: %s",
    signedInt(int(csv.GetToughnessSave())),
    signedInt(int(csv.GetHustleSave())),
    signedInt(int(csv.GetCoolSave())))))
```

Add immediately after it:
```go
left = append(left, slPlain(fmt.Sprintf("Awareness: %s",
    signedInt(int(csv.GetAwareness())))))
```

**Step 3: Run tests.**
```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsAwareness -v 2>&1
```
Expected: PASS

**Step 4: Run full handler tests.**
```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... 2>&1 | tail -3
```

**Step 5: Commit.**
```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(renderer): display Awareness on character sheet"
```

---

## Task 4: Backfill awareness proficiency for existing characters

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

New characters should get `awareness: trained` as a baseline (all characters are at least trained in Perception in PF2E). Existing characters with no `awareness` entry need it backfilled at login.

The existing proficiency backfill pattern is in `grpc_service.go` around line 646 (the `characterProficienciesRepo` block). It already backfills job proficiencies and `unarmored: trained`. We need to add `awareness: trained` to that backfill if it's missing.

**Step 1: Write failing test.**

```go
func TestHandleChar_Awareness_BackfilledWhenMissing(t *testing.T) {
    // Player with no awareness proficiency → should default to trained (10 + mod + level+2)
    worldMgr, sessMgr := testWorldAndSession(t)
    _ = worldMgr
    uid := "player-bf"
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "bf", CharName: "BF",
        RoomID: "grinders_row", Role: "player", CharacterID: 0,
        CurrentHP: 10, MaxHP: 10,
    })
    require.NoError(t, err)
    sess.Abilities.Savvy = 10
    sess.Level = 1
    sess.Proficiencies = map[string]string{} // no awareness

    svc := testMinimalService(t, sessMgr)
    evt, err := svc.handleChar(uid)
    require.NoError(t, err)
    sheet := evt.GetCharacterSheet()
    // With trained backfill: 10 + 0 + (1+2) = 13
    assert.Equal(t, int32(13), sheet.GetAwareness())
}
```

Run to confirm it fails (currently gives 10 because untrained = 0 bonus):
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleChar_Awareness_Backfill -v 2>&1
```

**Step 2: Add awareness backfill in `handleChar`.**

In `handleChar`, just before computing `view.Awareness`, add:
```go
// Awareness defaults to trained if no rank is recorded.
if _, hasAwareness := sess.Proficiencies["awareness"]; !hasAwareness {
    if sess.Proficiencies == nil {
        sess.Proficiencies = make(map[string]string)
    }
    sess.Proficiencies["awareness"] = "trained"
}
```

This is a session-only default; it does NOT persist. If we want it persisted, we'd need the proficiencies repo — but since all new characters will get `awareness: trained` set by job proficiency backfill (which already runs at login), this in-handleChar guard is sufficient for the display case.

**Step 3: Run tests.**
```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleChar_Awareness -v 2>&1
```

**Step 4: Run full suite.**
```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```

**Step 5: Commit.**
```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat(server): default awareness to trained rank if not set"
```

---

## Task 5: Final verification, FEATURES.md update, deploy

**Step 1: Run full test suite.**
```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | grep -E "^(ok|FAIL)"
```
Expected: all ok.

**Step 2: Update FEATURES.md.**

Find:
```
- [ ] Perception
  - PF2E Perception should be implemented using the name Awareness
  - Awareness should be displayed on the character sheet
```

Replace with:
```
- [x] Perception
  - [x] PF2E Perception should be implemented using the name Awareness
  - [x] Awareness should be displayed on the character sheet
```

**Step 3: Commit.**
```bash
cd /home/cjohannsen/src/mud && git add docs/requirements/FEATURES.md
git commit -m "docs: mark Awareness feature complete"
```

**Step 4: Deploy.**
```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy 2>&1 | tail -8
```

**Step 5: Verify pods.**
```bash
sleep 15 && kubectl get pods -n mud
```
Expected: all Running.
