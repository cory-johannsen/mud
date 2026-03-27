# Focus Points Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-character Focus Point pool that powers focus technologies, with spend/restore logic, DB persistence, and frontend display.

**Architecture:** Focus Points live in `PlayerSession` (in-memory) and `characters.focus_points` (persisted). `MaxFocusPoints` is derived at login from active class features and feats with `grants_focus_point: true`, capped at 3, and is never persisted. The frontend receives FP data via two proto messages: `CharacterSheetView` (character sheet) and `HpUpdateEvent` (prompt refresh after spend). Recalibrate (downtime) restoration is defined here as a callable function but is wired up in the `downtime` plan.

**Tech Stack:** Go, PostgreSQL, protobuf (buf), testify, rapid

**Dependencies:** `actions` feature (technology activation handler must exist)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/game/technology/model.go` | Add `FocusCost bool` field + `Validate()` check |
| Modify | `internal/game/ruleset/class_feature.go` | Add `GrantsFocusPoint bool` field |
| Modify | `internal/game/ruleset/feat.go` (or wherever Feat struct lives) | Add `GrantsFocusPoint bool` field |
| Modify | `internal/game/session/manager.go` | Add `FocusPoints int`, `MaxFocusPoints int` to `PlayerSession` |
| Add | `internal/storage/postgres/character_focus_points.go` | `SaveFocusPoints` and `LoadFocusPoints` repository methods |
| Add DB migration | `internal/storage/postgres/migrations/` | `ADD COLUMN focus_points int NOT NULL DEFAULT 0` |
| Modify | `api/proto/game/v1/game.proto` | Add `focus_points`/`max_focus_points` to `CharacterSheetView` and `HpUpdateEvent` |
| Regenerate | `api/proto/game/v1/game.pb.go` | `buf generate` |
| Modify | `internal/gameserver/grpc_service.go` | Login flow: load FP, compute MaxFP, clamp; tech activation: spend check |
| Modify | `internal/frontend/handlers/text_renderer.go` | Add Focus Points row to character sheet |
| Modify | `internal/frontend/handlers/game_bridge.go` | Add FP to prompt |
| Add | `internal/game/focuspoints/focuspoints.go` | Pure functions: `ComputeMax`, `Clamp`, `Spend`, `Restore` |
| Add | `internal/game/focuspoints/focuspoints_test.go` | Unit tests for all pure functions |

---

### Task 1: Add `focus_points` column to `characters` table

**Files:**
- Add: `internal/storage/postgres/migrations/<next_number>_add_focus_points.up.sql`
- Add: `internal/storage/postgres/migrations/<next_number>_add_focus_points.down.sql`

Find the highest-numbered migration file in `internal/storage/postgres/migrations/` and use the next number.

- [ ] **Step 1: Find current highest migration number**

```bash
ls internal/storage/postgres/migrations/ | sort | tail -5
```

- [ ] **Step 2: Create up migration**

```sql
-- <N>_add_focus_points.up.sql
ALTER TABLE characters ADD COLUMN focus_points int NOT NULL DEFAULT 0;
```

- [ ] **Step 3: Create down migration**

```sql
-- <N>_add_focus_points.down.sql
ALTER TABLE characters DROP COLUMN focus_points;
```

- [ ] **Step 4: Run the migration**

```bash
cd /home/cjohannsen/src/mud
mise exec -- make migrate-up
```

Expected: migration applies cleanly with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/migrations/
git commit -m "feat(focus-points): add focus_points column to characters table"
```

---

### Task 2: Pure Focus Points logic package

**Files:**
- Create: `internal/game/focuspoints/focuspoints.go`
- Create: `internal/game/focuspoints/focuspoints_test.go`

This package contains only pure functions — no DB, no session, no proto — so it is fully testable in isolation.

- [ ] **Step 1: Write the tests first**

```go
// internal/game/focuspoints/focuspoints_test.go
package focuspoints_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/stretchr/testify/assert"
    "github.com/cory-johannsen/mud/internal/game/focuspoints"
)

func TestComputeMax(t *testing.T) {
    assert.Equal(t, 0, focuspoints.ComputeMax(0))
    assert.Equal(t, 1, focuspoints.ComputeMax(1))
    assert.Equal(t, 3, focuspoints.ComputeMax(3))
    assert.Equal(t, 3, focuspoints.ComputeMax(4)) // capped at 3
    assert.Equal(t, 3, focuspoints.ComputeMax(10))
}

func TestSpend(t *testing.T) {
    cur, ok := focuspoints.Spend(2, 3)
    assert.True(t, ok)
    assert.Equal(t, 1, cur)

    cur, ok = focuspoints.Spend(0, 3)
    assert.False(t, ok)
    assert.Equal(t, 0, cur)
}

func TestRestore_CritSuccessAndSuccess(t *testing.T) {
    cur := focuspoints.Restore(1, 3, focuspoints.OutcomeCritSuccess)
    assert.Equal(t, 3, cur)
    cur = focuspoints.Restore(0, 3, focuspoints.OutcomeSuccess)
    assert.Equal(t, 3, cur)
}

func TestRestore_Failure(t *testing.T) {
    cur := focuspoints.Restore(1, 3, focuspoints.OutcomeFailure)
    assert.Equal(t, 2, cur)
    cur = focuspoints.Restore(3, 3, focuspoints.OutcomeFailure) // capped
    assert.Equal(t, 3, cur)
}

func TestRestore_CritFailure(t *testing.T) {
    cur := focuspoints.Restore(1, 3, focuspoints.OutcomeCritFailure)
    assert.Equal(t, 1, cur) // no change
}

func TestClamp(t *testing.T) {
    assert.Equal(t, 2, focuspoints.Clamp(5, 2))
    assert.Equal(t, 2, focuspoints.Clamp(2, 3))
    assert.Equal(t, 0, focuspoints.Clamp(0, 0))
}

func TestSpendProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        cur := rapid.IntRange(0, 10).Draw(t, "cur")
        max := rapid.IntRange(0, 10).Draw(t, "max")
        next, ok := focuspoints.Spend(cur, max)
        if cur > 0 {
            assert.True(t, ok)
            assert.Equal(t, cur-1, next)
        } else {
            assert.False(t, ok)
            assert.Equal(t, 0, next)
        }
    })
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/focuspoints/... -v
```

Expected: compilation error — package does not exist yet.

- [ ] **Step 3: Implement the package**

```go
// internal/game/focuspoints/focuspoints.go
package focuspoints

const maxCap = 3

// Outcome represents the result tier of a Recalibrate downtime roll.
type Outcome int

const (
    OutcomeCritSuccess Outcome = iota
    OutcomeSuccess
    OutcomeFailure
    OutcomeCritFailure
)

// ComputeMax returns min(grantCount, maxCap).
func ComputeMax(grantCount int) int {
    if grantCount > maxCap {
        return maxCap
    }
    return grantCount
}

// Spend decrements current by 1. Returns (new value, ok). ok=false if current==0.
func Spend(current, max int) (int, bool) {
    if current == 0 {
        return 0, false
    }
    return current - 1, true
}

// Restore applies Recalibrate outcome to current FP, returning new value.
func Restore(current, max int, outcome Outcome) int {
    switch outcome {
    case OutcomeCritSuccess, OutcomeSuccess:
        return max
    case OutcomeFailure:
        if current+1 > max {
            return max
        }
        return current + 1
    default: // OutcomeCritFailure
        return current
    }
}

// Clamp ensures current does not exceed max (used after MaxFP recompute at login).
func Clamp(current, max int) int {
    if current > max {
        return max
    }
    return current
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/focuspoints/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/game/focuspoints/
git commit -m "feat(focus-points): add pure FP logic package (ComputeMax, Spend, Restore, Clamp)"
```

---

### Task 3: Add `FocusCost` to `TechnologyDef` and `GrantsFocusPoint` to `ClassFeature` / `Feat`

**Files:**
- Modify: `internal/game/technology/model.go`
- Modify: `internal/game/ruleset/class_feature.go`
- Modify: wherever the `Feat` struct is defined (search for `type Feat struct` in `internal/`)

- [ ] **Step 1: Write failing tests for TechnologyDef validation**

Add to the existing technology model test file (find with `grep -r "TestTechnology" internal/`):

```go
func TestTechnologyDef_Validate_FocusCostPassiveConflict(t *testing.T) {
    tech := TechnologyDef{ID: "test", Passive: true, FocusCost: true}
    err := tech.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "focus_cost")
}

func TestTechnologyDef_Validate_FocusCostNotPassive(t *testing.T) {
    tech := TechnologyDef{ID: "test", Passive: false, FocusCost: true}
    err := tech.Validate()
    assert.NoError(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... -v
```

Expected: FAIL — `FocusCost` field does not exist.

- [ ] **Step 3: Add `FocusCost` field to `TechnologyDef`**

In `internal/game/technology/model.go`, add after the `ActionCost` field:

```go
FocusCost bool `yaml:"focus_cost,omitempty"`
```

In the `Validate()` method, add:

```go
if t.Passive && t.FocusCost {
    return fmt.Errorf("technology %q: focus_cost and passive cannot both be true", t.ID)
}
```

- [ ] **Step 4: Add `GrantsFocusPoint` to `ClassFeature`**

In `internal/game/ruleset/class_feature.go`, add at the end of the struct:

```go
GrantsFocusPoint bool `yaml:"grants_focus_point,omitempty"`
```

- [ ] **Step 5: Add `GrantsFocusPoint` to `Feat`**

Find the Feat struct definition and add:

```go
GrantsFocusPoint bool `yaml:"grants_focus_point,omitempty"`
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/technology/... ./internal/game/ruleset/... -v
```

Expected: all PASS, no regressions.

- [ ] **Step 7: Commit**

```bash
git add internal/game/technology/model.go internal/game/ruleset/class_feature.go
# add the feat file too
git commit -m "feat(focus-points): add FocusCost to TechnologyDef, GrantsFocusPoint to ClassFeature/Feat"
```

---

### Task 4: Add `FocusPoints` / `MaxFocusPoints` to `PlayerSession`

**Files:**
- Modify: `internal/game/session/manager.go`

- [ ] **Step 1: Add fields to `PlayerSession`**

In `internal/game/session/manager.go`, find `HeroPoints int` (around line 106) and add two fields after it:

```go
FocusPoints    int // current pool; loaded from DB at login; persisted on spend/restore
MaxFocusPoints int // derived at login from active feats + class features; not persisted
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/game/session/manager.go
git commit -m "feat(focus-points): add FocusPoints/MaxFocusPoints fields to PlayerSession"
```

---

### Task 5: Add `SaveFocusPoints` repository method

**Files:**
- Create: `internal/storage/postgres/character_focus_points.go`
- Add test to existing character repository test file

- [ ] **Step 1: Write failing test**

Find the character repository test file (search for `TestCharacterRepository` in `internal/storage/postgres/`). Add:

```go
func TestSaveFocusPoints(t *testing.T) {
    // Requires a real DB — follow existing integration test pattern
    // Create a character, save FP=2, reload and assert focus_points==2
}
```

Follow the existing integration test pattern for database tests in this project.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/storage/postgres/... -run TestSaveFocusPoints -v
```

Expected: FAIL — method does not exist.

- [ ] **Step 3: Implement `SaveFocusPoints`**

```go
// internal/storage/postgres/character_focus_points.go
package postgres

import "context"

// SaveFocusPoints persists the current focus_points value for a character.
// Precondition: characterID > 0, focusPoints >= 0
// Postcondition: characters.focus_points == focusPoints for this character
func (r *CharacterRepository) SaveFocusPoints(ctx context.Context, characterID int64, focusPoints int) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE characters SET focus_points = $1 WHERE id = $2`,
        focusPoints, characterID,
    )
    return err
}
```

Note: Check `internal/storage/postgres/` for the actual `CharacterRepository` struct name and `db` field name — adjust to match.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/storage/postgres/... -run TestSaveFocusPoints -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/character_focus_points.go
git commit -m "feat(focus-points): add SaveFocusPoints repository method"
```

---

### Task 6: Proto changes — add FP fields to `CharacterSheetView` and `HpUpdateEvent`

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `api/proto/game/v1/game.pb.go` (via `buf generate`)

- [ ] **Step 1: Add fields to `CharacterSheetView`**

In `api/proto/game/v1/game.proto`, find `CharacterSheetView` message. After the `hero_points` field (field number 42), add:

```protobuf
int32 focus_points     = 47;
int32 max_focus_points = 48;
```

(Use field numbers that do not conflict with existing fields — verify the highest existing field number in `CharacterSheetView` first and pick the next available numbers.)

- [ ] **Step 2: Add fields to `HpUpdateEvent`**

Find `HpUpdateEvent` message. After existing fields, add:

```protobuf
int32 focus_points     = 3;
int32 max_focus_points = 4;
```

(Verify existing field numbers first.)

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud
mise exec -- buf generate
```

Expected: `game.pb.go` regenerated with new accessor methods `GetFocusPoints()` and `GetMaxFocusPoints()` on both message types.

- [ ] **Step 4: Verify compilation**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto api/proto/game/v1/game.pb.go
git commit -m "feat(focus-points): add focus_points/max_focus_points to CharacterSheetView and HpUpdateEvent proto"
```

---

### Task 7: Login flow — load FP, compute MaxFP, clamp

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (Session() method, around line 447)

- [ ] **Step 1: Write integration test for login FP loading**

Find the existing session/login test in `internal/gameserver/`. Add a test that:
1. Creates a character with `focus_points = 2` in DB and a class feature with `grants_focus_point: true`
2. Opens a session
3. Asserts `sess.FocusPoints == 2` and `sess.MaxFocusPoints == 1` (from the one feature)

Follow the existing test pattern — look at existing session tests for the setup idiom.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestSession_LoadsFocusPoints -v
```

Expected: FAIL.

- [ ] **Step 3: Implement the login flow**

In the `Session()` method in `grpc_service.go`, after the character is loaded from DB, add:

```go
// Load and compute Focus Points.
// cfIDs and featIDs are already loaded earlier in Session() via
// characterClassFeaturesRepo.GetAll() and characterFeatsRepo.GetAll().
fpFromDB, err := s.charSaver.LoadFocusPoints(ctx, characterID)
if err != nil {
    return fmt.Errorf("load focus points: %w", err)
}

grantCount := 0
if s.classFeatureRegistry != nil {
    for _, id := range cfIDs {
        if cf, ok := s.classFeatureRegistry.ClassFeature(id); ok && cf.GrantsFocusPoint {
            grantCount++
        }
    }
}
if s.featRegistry != nil {
    featIDs, _ := s.characterFeatsRepo.GetAll(stream.Context(), characterID)
    for _, id := range featIDs {
        if f, ok := s.featRegistry.Feat(id); ok && f.GrantsFocusPoint {
            grantCount++
        }
    }
}

sess.MaxFocusPoints = focuspoints.ComputeMax(grantCount)
sess.FocusPoints = focuspoints.Clamp(fpFromDB, sess.MaxFocusPoints)
```

Note: `cfIDs` is already available in `Session()` from the earlier class-feature loading block (lines ~787–808). `s.characterFeatsRepo.GetAll()` mirrors the existing pattern used by the feat choice resolution block (lines ~869–900). If `characterFeatsRepo` has already been called earlier in the same Session() invocation for choice resolution, extract the result into a local variable and reuse it rather than calling it twice.

Also add `SaveFocusPoints` and `LoadFocusPoints` to the `CharacterSaver` interface in `internal/gameserver/grpc_service.go`, and implement both methods in `internal/storage/postgres/character_focus_points.go`:

```go
func (r *CharacterRepository) LoadFocusPoints(ctx context.Context, characterID int64) (int, error) {
    var fp int
    err := r.db.QueryRowContext(ctx,
        `SELECT focus_points FROM characters WHERE id = $1`,
        characterID,
    ).Scan(&fp)
    return fp, err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestSession_LoadsFocusPoints -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./...
```

Expected: 100% pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/storage/postgres/character_focus_points.go
git commit -m "feat(focus-points): load, compute, and clamp FP at login"
```

---

### Task 8: Technology activation — spend FP check

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (`handleUse()` function, around line 4883)

- [ ] **Step 1: Write failing test for FP spend**

In the gameserver test file, add:

```go
func TestHandleUse_FocusTech_NoFP_Fails(t *testing.T) {
    // Set up session with MaxFocusPoints=1, FocusPoints=0
    // Activate a technology with FocusCost=true
    // Assert error message contains "Not enough Focus Points. (0/1)"
}

func TestHandleUse_FocusTech_WithFP_Succeeds(t *testing.T) {
    // Set up session with MaxFocusPoints=1, FocusPoints=1
    // Activate a technology with FocusCost=true
    // Assert FocusPoints decremented to 0
    // Assert SaveFocusPoints was called with 0
}
```

Follow the existing `handleUse` test pattern.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_FocusTech -v
```

Expected: FAIL.

- [ ] **Step 3: Implement the spend check in `handleUse()`**

Find the technology activation block in `handleUse()`. Before the existing AP cost check, add:

```go
if tech.FocusCost {
    next, ok := focuspoints.Spend(sess.FocusPoints, sess.MaxFocusPoints)
    if !ok {
        return fmt.Errorf("Not enough Focus Points. (%d/%d)", sess.FocusPoints, sess.MaxFocusPoints)
    }
    sess.FocusPoints = next
    if err := s.charSaver.SaveFocusPoints(ctx, sess.CharacterID, sess.FocusPoints); err != nil {
        return fmt.Errorf("persist focus points: %w", err)
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/gameserver/... -run TestHandleUse_FocusTech -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
mise exec -- go test ./...
```

Expected: 100% pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(focus-points): enforce FP spend check in technology activation"
```

---

### Task 9: Populate FP fields in `CharacterSheetView` and `HpUpdateEvent`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (wherever `CharacterSheetView` and `HpUpdateEvent` are assembled)

- [ ] **Step 1: Find assembly sites**

```bash
grep -n "CharacterSheetView{" internal/gameserver/grpc_service.go | head -10
grep -n "HpUpdateEvent{" internal/gameserver/grpc_service.go | head -10
```

- [ ] **Step 2: Add FP fields to `CharacterSheetView` assembly**

Where `CharacterSheetView` is constructed, add:

```go
FocusPoints:    int32(sess.FocusPoints),
MaxFocusPoints: int32(sess.MaxFocusPoints),
```

- [ ] **Step 3: Add FP fields to `HpUpdateEvent` assembly**

Where `HpUpdateEvent` is constructed, add:

```go
FocusPoints:    int32(sess.FocusPoints),
MaxFocusPoints: int32(sess.MaxFocusPoints),
```

- [ ] **Step 4: Verify compilation and run suite**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go build ./...
mise exec -- go test ./...
```

Expected: compiles, 100% pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(focus-points): populate FP fields in CharacterSheetView and HpUpdateEvent"
```

---

### Task 10: Character sheet display

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` (after HP row, around line 715)
- Modify: `internal/frontend/handlers/text_renderer_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRenderCharacterSheet_ShowsFocusPoints_WhenMaxGTZero(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        FocusPoints:    2,
        MaxFocusPoints: 3,
    }
    result := telnet.StripANSI(RenderCharacterSheet(csv, 80))
    assert.Contains(t, result, "Focus Points: 2 / 3")
}

func TestRenderCharacterSheet_OmitsFocusPoints_WhenMaxIsZero(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        MaxFocusPoints: 0,
    }
    result := telnet.StripANSI(RenderCharacterSheet(csv, 80))
    assert.NotContains(t, result, "Focus Points")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsFocusPoints -v
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_OmitsFocusPoints -v
```

Expected: FAIL — FP row not present.

- [ ] **Step 3: Add FP row in `RenderCharacterSheet`**

In `internal/frontend/handlers/text_renderer.go`, find the HP row assembly (around line 714). After the HP row (and Hero Points row if present), add:

```go
if csv.GetMaxFocusPoints() > 0 {
    left = append(left, slPlain(fmt.Sprintf("Focus Points: %d / %d",
        csv.GetFocusPoints(), csv.GetMaxFocusPoints())))
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
mise exec -- go test ./...
```

Expected: 100% pass.

- [ ] **Step 6: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat(focus-points): add Focus Points row to character sheet (omitted when max=0)"
```

---

### Task 11: Prompt display

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go` (`BuildPrompt` function and its callers)

- [ ] **Step 1: Write failing tests**

Find the existing `BuildPrompt` test. Add:

```go
func TestBuildPrompt_ShowsFP_WhenMaxGTZero(t *testing.T) {
    result := BuildPrompt("Alice", 50, 100, 2, 3, nil)
    assert.Contains(t, telnet.StripANSI(result), "FP: 2/3")
}

func TestBuildPrompt_OmitsFP_WhenMaxIsZero(t *testing.T) {
    result := BuildPrompt("Alice", 50, 100, 0, 0, nil)
    assert.NotContains(t, result, "FP:")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestBuildPrompt -v
```

Expected: FAIL — function signature mismatch or FP not in output.

- [ ] **Step 3: Update `BuildPrompt` signature**

In `internal/frontend/handlers/game_bridge.go`, update `BuildPrompt` to accept two new parameters:

```go
func BuildPrompt(name string, currentHP, maxHP int32, focusPoints, maxFocusPoints int32, conditions []string) string {
```

In the prompt body, after the HP segment, conditionally add FP:

```go
if maxFocusPoints > 0 {
    prompt += telnet.Colorf(telnet.BrightBlue, " FP: %d/%d", focusPoints, maxFocusPoints)
}
```

Update all callers of `BuildPrompt` in `game_bridge.go` and in `game_bridge_test.go` to pass the new parameters. There are approximately 11 existing call sites in the test file — all must be updated to add `0, 0` (or appropriate values) as the new `focusPoints, maxFocusPoints` arguments, or the suite will fail to compile. Use `grep -n "BuildPrompt(" internal/frontend/handlers/` to find every call site before editing.

- [ ] **Step 4: Update stored state for FP in `game_bridge.go`**

Find where `currentHP` and `maxHP` are stored as atomic values in `game_bridge.go`. Add equivalent storage for `focusPoints` and `maxFocusPoints`. Update the `HpUpdateEvent` handler to populate these fields from `event.GetFocusPoints()` and `event.GetMaxFocusPoints()`.

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/frontend/handlers/... -run TestBuildPrompt -v
```

Expected: all PASS.

- [ ] **Step 6: Run full suite**

```bash
mise exec -- go test ./...
```

Expected: 100% pass.

- [ ] **Step 7: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go
git commit -m "feat(focus-points): add FP:N/M to player prompt when MaxFocusPoints > 0"
```

---

## Deferred

- **Recalibrate restoration** (REQ-FP-7, REQ-FP-8): The `focuspoints.Restore()` function is implemented in Task 2 and ready to call. Wiring it into the Recalibrate downtime activity is owned by the `downtime` plan.
- **Long rest restoration**: Owned by the `resting` plan.
- **MaxFP recompute on feat swap / level-up** (REQ-FP-2): Hook into the feat swap and level-up handlers to recompute `MaxFocusPoints` and clamp `FocusPoints`. Defer to the feat-import / job-development plans if those handlers are not yet implemented.

---

## Verification Checklist

- [ ] `focus_points int NOT NULL DEFAULT 0` column exists in `characters` table
- [ ] `TechnologyDef` with `focus_cost: true` + `passive: true` fails `Validate()`
- [ ] Technology with `focus_cost: true` costs 1 FP; `FocusPoints == 0` returns "Not enough Focus Points. (0/M)"
- [ ] `FocusPoints` is decremented and persisted before activation result is sent
- [ ] On login, `FocusPoints` is clamped to `MaxFocusPoints`
- [ ] `MaxFocusPoints` = min(count of `grants_focus_point: true` features/feats, 3)
- [ ] Character sheet shows `Focus Points: N / M` when `MaxFocusPoints > 0`; omitted when 0
- [ ] Prompt shows `FP: N/M` (with space after colon) when `MaxFocusPoints > 0`; omitted when 0
- [ ] Full test suite passes with zero failures
