# Character Sheet Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add skills, feats, and class features sections to the `sheet`/`char` command output.

**Architecture:** Extend `CharacterSheetView` proto with three new repeated fields (reusing existing
entry types), populate them in `handleChar` by reusing the same DB fetch + registry lookup logic
from the individual command handlers, and render three new sections in `RenderCharacterSheet`.

**Tech Stack:** Go, protobuf (buf/protoc), pgx/v5, telnet ANSI color codes

---

## Context

The `sheet`/`char` command currently shows abilities, defense, weapons, armor, and currency.
Skills, feats, and class features already have their own commands (`skills`/`sk`, `feats`/`ft`,
`class_features`/`cf`), but FEATURES.md requires them to also appear on the character sheet.

**Key files:**
- `api/proto/game/v1/game.proto` — `CharacterSheetView` message (highest field: 21)
- `internal/gameserver/grpc_service.go` — `handleChar` (lines ~1642–1727), `handleSkills` (~2096), `handleFeats` (~2132), `handleClassFeatures` (~2171)
- `internal/frontend/handlers/text_renderer.go` — `RenderCharacterSheet` (lines ~281–344)
- `internal/gameserver/grpc_service_test.go` — existing server tests

**Relevant repos/registries on `GameServiceServer`:**
- `characterSkillsRepo *postgres.CharacterSkillsRepository` (field 96)
- `allSkills []*ruleset.Skill` + `skillRegistry` (implicit, look for it)
- `characterFeatsRepo *postgres.CharacterFeatsRepository`
- `featRegistry *ruleset.FeatRegistry`
- `characterClassFeaturesRepo *postgres.CharacterClassFeaturesRepository`
- `classFeatureRegistry *ruleset.ClassFeatureRegistry`

---

## Task 1: Extend `CharacterSheetView` proto

**Files:**
- Modify: `api/proto/game/v1/game.proto`

**Step 1: Add three repeated fields to `CharacterSheetView`**

Find the `CharacterSheetView` message (currently ends at field 21 `off_hand`). Add after `off_hand`:

```protobuf
repeated SkillEntry        skills         = 22;
repeated FeatEntry         feats          = 23;
repeated ClassFeatureEntry class_features = 24;
```

**Step 2: Regenerate proto**

```bash
make proto
```

Expected: no errors, regenerated files in `api/gen/go/game/v1/`.

**Step 3: Verify compilation**

```bash
mise run build
```

Expected: compiles cleanly (new fields are optional repeated — no existing code breaks).

**Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto api/gen/
git commit -m "feat: extend CharacterSheetView with skills/feats/class_features fields"
```

---

## Task 2: Populate skills/feats/class features in `handleChar`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Test: `internal/gameserver/grpc_service_test.go`

**Background:** `handleChar` (lines ~1642–1727) builds a `CharacterSheetView` and returns it.
The individual handlers `handleSkills`, `handleFeats`, `handleClassFeatures` each:
1. Get the character's session to obtain `characterID`
2. Call the repo's `GetAll(ctx, characterID)` to get IDs/proficiencies
3. Resolve via registry to build entry slices

We need to inline equivalent logic into `handleChar` to populate the three new fields.

**Step 1: Write the failing test**

In `internal/gameserver/grpc_service_test.go`, add a test that calls `handleChar` on a server
configured with mock skills/feats/class features repos and registries, and asserts the returned
`CharacterSheetView` has non-empty `Skills`, `Feats`, and `ClassFeatures` slices.

Use the existing test setup pattern (look at how `TestHandleChar` or similar is written).
If no unit test for `handleChar` exists, add one using the existing server construction helpers.

The test should:
- Create a server with `characterSkillsRepo`, `featRegistry`, `characterFeatsRepo`,
  `classFeatureRegistry`, `characterClassFeaturesRepo` all populated
- Call `handleChar(uid)` with a valid session UID
- Assert `view.GetSkills()` has length > 0
- Assert `view.GetFeats()` has length > 0
- Assert `view.GetClassFeatures()` has length > 0

**Step 2: Run test to verify it fails**

```bash
mise run test -- -run TestHandleChar -v ./internal/gameserver/...
```

Expected: FAIL (Skills/Feats/ClassFeatures slices are empty).

**Step 3: Implement — add skills population to `handleChar`**

Inside `handleChar`, after the existing `CharacterSheetView` is fully built and before the
`return` statement, add:

```go
// Skills
if s.characterSkillsRepo != nil && len(s.allSkills) > 0 {
    ctx := context.Background()
    skillMap, err := s.characterSkillsRepo.GetAll(ctx, session.CharacterID)
    if err == nil {
        for _, sk := range s.allSkills {
            rank := skillMap[sk.ID]
            if rank == "" {
                rank = "untrained"
            }
            view.Skills = append(view.Skills, &gamev1.SkillEntry{
                SkillId:     sk.ID,
                Name:        sk.Name,
                Ability:     sk.KeyAbility,
                Proficiency: rank,
            })
        }
    }
}
```

**Step 4: Add feats population**

```go
// Feats
if s.characterFeatsRepo != nil && s.featRegistry != nil {
    ctx := context.Background()
    featIDs, err := s.characterFeatsRepo.GetAll(ctx, session.CharacterID)
    if err == nil {
        for _, fid := range featIDs {
            f, ok := s.featRegistry.Feat(fid)
            if !ok {
                continue
            }
            view.Feats = append(view.Feats, &gamev1.FeatEntry{
                FeatId:       f.ID,
                Name:         f.Name,
                Active:       f.Active,
                Description:  f.Description,
                ActivateText: f.ActivateText,
            })
        }
    }
}
```

**Step 5: Add class features population**

```go
// Class Features
if s.characterClassFeaturesRepo != nil && s.classFeatureRegistry != nil {
    ctx := context.Background()
    cfIDs, err := s.characterClassFeaturesRepo.GetAll(ctx, session.CharacterID)
    if err == nil {
        for _, cfid := range cfIDs {
            cf, ok := s.classFeatureRegistry.ClassFeature(cfid)
            if !ok {
                continue
            }
            view.ClassFeatures = append(view.ClassFeatures, &gamev1.ClassFeatureEntry{
                FeatureId:    cf.ID,
                Name:         cf.Name,
                Archetype:    cf.Archetype,
                Job:          cf.Job,
                Active:       cf.Active,
                Description:  cf.Description,
                ActivateText: cf.ActivateText,
            })
        }
    }
}
```

**Step 6: Run test to verify it passes**

```bash
mise run test -- -run TestHandleChar -v ./internal/gameserver/...
```

Expected: PASS.

**Step 7: Run full test suite**

```bash
mise run test -- ./...
```

Expected: all tests pass.

**Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go
git commit -m "feat: populate skills/feats/class features in handleChar"
```

---

## Task 3: Render skills section in `RenderCharacterSheet`

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Test: `internal/frontend/handlers/text_renderer_test.go`

**Background:** `RenderCharacterSheet` (lines ~281–344) in `text_renderer.go` currently ends with
the currency section. We add three new sections. This task handles skills.

The `telnet` package provides color constants: `telnet.BrightCyan`, `telnet.White`, `telnet.Gray`
(or `telnet.Dim`), `telnet.Cyan`, `telnet.Yellow`, `telnet.Magenta`. Check what constants exist
in `internal/telnet/` before using them.

Proficiency ranks and their colors:
- `untrained` → dim/gray
- `trained` → white (default terminal color)
- `expert` → cyan
- `master` → yellow
- `legendary` → magenta

**Step 1: Write the failing test**

In `internal/frontend/handlers/text_renderer_test.go`, add:

```go
func TestRenderCharacterSheet_Skills(t *testing.T) {
    view := &gamev1.CharacterSheetView{
        Name:  "Test",
        Level: 1,
        Skills: []*gamev1.SkillEntry{
            {SkillId: "acrobatics", Name: "Acrobatics", Ability: "QCK", Proficiency: "trained"},
            {SkillId: "athletics",  Name: "Athletics",  Ability: "BRT", Proficiency: "untrained"},
            {SkillId: "stealth",    Name: "Stealth",    Ability: "QCK", Proficiency: "expert"},
        },
    }
    result := RenderCharacterSheet(view)
    assert.Contains(t, result, "Skills")
    assert.Contains(t, result, "Acrobatics")
    assert.Contains(t, result, "trained")
    assert.Contains(t, result, "untrained")
    assert.Contains(t, result, "expert")
}
```

**Step 2: Run test to verify it fails**

```bash
mise run test -- -run TestRenderCharacterSheet_Skills -v ./internal/frontend/handlers/...
```

Expected: FAIL (no "Skills" section in output).

**Step 3: Add a `proficiencyColor` helper**

Add before `RenderCharacterSheet`:

```go
func proficiencyColor(rank string) string {
    switch strings.ToLower(rank) {
    case "legendary":
        return telnet.Colorize(telnet.Magenta, rank)
    case "master":
        return telnet.Colorize(telnet.Yellow, rank)
    case "expert":
        return telnet.Colorize(telnet.Cyan, rank)
    case "trained":
        return telnet.Colorize(telnet.White, rank)
    default:
        return telnet.Colorize(telnet.Gray, rank) // untrained or unknown
    }
}
```

Check `internal/telnet/` for the exact constant names (`telnet.Gray`, `telnet.Dim`, etc.).
Use whatever dim/gray constant exists; if none exists use `telnet.White` for untrained.

**Step 4: Add skills section to `RenderCharacterSheet`**

After the currency section (just before `return b.String()`), add:

```go
// Skills
if skills := csv.GetSkills(); len(skills) > 0 {
    b.WriteString("\r\n")
    b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Skills ---"))
    b.WriteString("\r\n")
    // Two-column layout: left column = name+ability, right column = rank
    for i, sk := range skills {
        label := fmt.Sprintf("%-20s (%s)", sk.GetName(), sk.GetAbility())
        rank  := proficiencyColor(sk.GetProficiency())
        entry := fmt.Sprintf("  %-30s %s", label, rank)
        b.WriteString(entry)
        if i%2 == 1 {
            b.WriteString("\r\n")
        } else {
            b.WriteString("  ") // spacer between columns
        }
    }
    if len(skills)%2 != 0 {
        b.WriteString("\r\n") // close last row if odd count
    }
}
```

Note: If two-column layout looks messy with telnet escape codes, fall back to one entry per line
(simpler and more readable). Adjust based on visual testing.

**Step 5: Run test to verify it passes**

```bash
mise run test -- -run TestRenderCharacterSheet_Skills -v ./internal/frontend/handlers/...
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: render skills section in character sheet"
```

---

## Task 4: Render feats section in `RenderCharacterSheet`

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Test: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Write the failing test**

```go
func TestRenderCharacterSheet_Feats(t *testing.T) {
    view := &gamev1.CharacterSheetView{
        Name:  "Test",
        Level: 1,
        Feats: []*gamev1.FeatEntry{
            {FeatId: "iron_will", Name: "Iron Will", Active: false, Description: "Bonus to Will saves."},
            {FeatId: "power_attack", Name: "Power Attack", Active: true, ActivateText: "Strike hard.", Description: "Deal extra damage."},
        },
    }
    result := RenderCharacterSheet(view)
    assert.Contains(t, result, "Feats")
    assert.Contains(t, result, "Iron Will")
    assert.Contains(t, result, "Power Attack")
    assert.Contains(t, result, "[active]")
}
```

**Step 2: Run test to verify it fails**

```bash
mise run test -- -run TestRenderCharacterSheet_Feats -v ./internal/frontend/handlers/...
```

Expected: FAIL.

**Step 3: Add feats section to `RenderCharacterSheet`**

After the skills section, add:

```go
// Feats
if feats := csv.GetFeats(); len(feats) > 0 {
    b.WriteString("\r\n")
    b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Feats ---"))
    b.WriteString("\r\n")
    for _, ft := range feats {
        name := ft.GetName()
        if ft.GetActive() {
            name += " " + telnet.Colorize(telnet.Yellow, "[active]")
        }
        b.WriteString(fmt.Sprintf("  %s\r\n", name))
        if desc := ft.GetDescription(); desc != "" {
            b.WriteString(fmt.Sprintf("    %s\r\n", desc))
        }
    }
}
```

**Step 4: Run test to verify it passes**

```bash
mise run test -- -run TestRenderCharacterSheet_Feats -v ./internal/frontend/handlers/...
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: render feats section in character sheet"
```

---

## Task 5: Render class features section in `RenderCharacterSheet`

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Test: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Write the failing test**

```go
func TestRenderCharacterSheet_ClassFeatures(t *testing.T) {
    view := &gamev1.CharacterSheetView{
        Name:  "Test",
        Level: 1,
        ClassFeatures: []*gamev1.ClassFeatureEntry{
            {FeatureId: "brutal_surge", Name: "Brutal Surge", Archetype: "aggressor", Active: true, Description: "Enter a frenzy."},
            {FeatureId: "guerilla_warfare", Name: "Guerilla Warfare", Job: "soldier", Active: false, Description: "Urban cover bonus."},
        },
    }
    result := RenderCharacterSheet(view)
    assert.Contains(t, result, "Class Features")
    assert.Contains(t, result, "Brutal Surge")
    assert.Contains(t, result, "Guerilla Warfare")
    assert.Contains(t, result, "[active]")
}
```

**Step 2: Run test to verify it fails**

```bash
mise run test -- -run TestRenderCharacterSheet_ClassFeatures -v ./internal/frontend/handlers/...
```

Expected: FAIL.

**Step 3: Add class features section to `RenderCharacterSheet`**

Group into archetype features (those with non-empty `Archetype`) and job features (those with
non-empty `Job`). After the feats section, add:

```go
// Class Features
if cfs := csv.GetClassFeatures(); len(cfs) > 0 {
    b.WriteString("\r\n")
    b.WriteString(telnet.Colorize(telnet.BrightCyan, "--- Class Features ---"))
    b.WriteString("\r\n")

    // Archetype features first
    b.WriteString(telnet.Colorize(telnet.Cyan, "  Archetype:\r\n"))
    for _, cf := range cfs {
        if cf.GetArchetype() == "" {
            continue
        }
        name := cf.GetName()
        if cf.GetActive() {
            name += " " + telnet.Colorize(telnet.Yellow, "[active]")
        }
        b.WriteString(fmt.Sprintf("    %s\r\n", name))
        if desc := cf.GetDescription(); desc != "" {
            b.WriteString(fmt.Sprintf("      %s\r\n", desc))
        }
    }

    // Job features
    b.WriteString(telnet.Colorize(telnet.Cyan, "  Job:\r\n"))
    for _, cf := range cfs {
        if cf.GetJob() == "" {
            continue
        }
        name := cf.GetName()
        if cf.GetActive() {
            name += " " + telnet.Colorize(telnet.Yellow, "[active]")
        }
        b.WriteString(fmt.Sprintf("    %s\r\n", name))
        if desc := cf.GetDescription(); desc != "" {
            b.WriteString(fmt.Sprintf("      %s\r\n", desc))
        }
    }
}
```

**Step 4: Run test to verify it passes**

```bash
mise run test -- -run TestRenderCharacterSheet_ClassFeatures -v ./internal/frontend/handlers/...
```

Expected: PASS.

**Step 5: Run full test suite**

```bash
mise run test -- ./...
```

Expected: all tests pass.

**Step 6: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: render class features section in character sheet"
```

---

## Task 6: Update FEATURES.md and deploy

**Files:**
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Mark sheet-related items done in FEATURES.md**

In `docs/requirements/FEATURES.md`, change:
```
  - [ ] Skills should appear on the character sheet
  - [ ] Feats should appear on the character sheet
  - [ ] Class features should appear on the character sheet
```
to:
```
  - [x] Skills should appear on the character sheet
  - [x] Feats should appear on the character sheet
  - [x] Class features should appear on the character sheet
```

**Step 2: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark skills/feats/class features on sheet as complete"
```

**Step 3: Deploy**

```bash
make k8s-redeploy
```

Expected: Helm upgrade completes, pods rollout successfully.

**Step 4: Smoke test in the game**

Connect via telnet and run:
```
sheet
```

Verify the output shows Skills, Feats, and Class Features sections with correct formatting.
