# Saving Rolls Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Toughness/Hustle/Cool saving rolls using the existing proficiency infrastructure, character sheet display, and update the grenade resolver to use the new ResolveSave function.

**Architecture:** Save proficiency ranks ("toughness", "hustle", "cool") reuse the existing `character_proficiencies` table and backfill pipeline. The save formula is `1d20 + ability_mod + CombatProficiencyBonus(level, rank)`. Three new Combatant fields carry the ability mods and three carry the ranks; a new `ResolveSave` function in `resolver.go` handles the roll; the character sheet gains a `--- Saves ---` section.

**Tech Stack:** Go, existing `internal/game/combat`, `internal/gameserver`, `api/proto/game/v1/game.proto`, `content/jobs/*.yaml`, `pgregory.net/rapid` for property-based tests.

---

### Task 1: Add toughness/hustle/cool proficiency ranks to all job YAMLs

**Files:**
- Modify: `content/jobs/*.yaml` (all 76 files)

The `proficiencies` section of each job YAML currently only has weapon/armor entries. Add save ranks based on the job archetype:

**Assignment rules:**
- All jobs: `toughness: trained`, `hustle: trained`, `cool: trained` as baseline
- Combat/fighter archetype jobs (aggressor, beat_down_artist, boot_gun, boot_machete, brawler, enforcer, ganger, hitman, mercenary, soldier, vigilante, warlord): `toughness: expert`
- Agile/rogue archetype jobs (burglar, car_jacker, criminal, drifter, fence, grafter, grifter, pickpocket, runner, scavenger, smuggler, spy): `hustle: expert`
- Social/mental archetype jobs (anarchist, bureaucrat, con_artist, cultist, fixer, influencer, mediator, nerd, schemer, zealot): `cool: expert`
- All other jobs: trained in all three

**Step 1: Add saves to a sample job**

Edit `content/jobs/anarchist.yaml` — add under existing `proficiencies:` block:
```yaml
  toughness: trained
  hustle: trained
  cool: expert
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/game/... -timeout 60s -run TestContent 2>&1 | tail -10`
Expected: PASS (the loader accepts new categories)

**Step 2: Add saves to all 76 job YAMLs**

For each job in `content/jobs/`, add the three save proficiency lines following the assignment rules above. Use the grep pattern to find which already have proficiencies: `grep -l "proficiencies:" content/jobs/*.yaml`

After editing all files, run:
```
cd /home/cjohannsen/src/mud && go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"
```
Expected: all ok (except pre-existing postgres timeout).

**Step 3: Commit**

```bash
git add content/jobs/
git commit -m "feat: add toughness/hustle/cool save proficiency ranks to all job YAMLs"
```

---

### Task 2: Add save fields to Combatant and wire from session

**Files:**
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/game/combat/combat_test.go` (or create if needed)

**Step 1: Write failing test**

In `internal/game/combat/combat_test.go` (or wherever Combatant tests live — check existing test files), add:

```go
func TestCombatant_SaveFields(t *testing.T) {
    c := &Combatant{
        GritMod:      2,
        QuicknessMod: 1,
        SavvyMod:     3,
        ToughnessRank: "trained",
        HustleRank:    "expert",
        CoolRank:      "untrained",
    }
    assert.Equal(t, 2, c.GritMod)
    assert.Equal(t, "expert", c.HustleRank)
    assert.Equal(t, "untrained", c.CoolRank)
}
```

Run: `go test ./internal/game/combat/... -run TestCombatant_SaveFields -v`
Expected: FAIL (fields don't exist)

**Step 2: Add fields to Combatant**

In `internal/game/combat/combat.go`, add to the `Combatant` struct after `ArmorProficiencyRank`:

```go
// Save ability modifiers (derived from character ability scores at combat start).
GritMod      int // used for Toughness saves
QuicknessMod int // used for Hustle saves
SavvyMod     int // used for Cool saves
// Save proficiency ranks from character_proficiencies.
ToughnessRank string // "untrained", "trained", "expert", "master", "legendary"
HustleRank    string
CoolRank      string
```

**Step 3: Wire from session in combat_handler.go**

In `internal/gameserver/combat_handler.go`, in `startCombatLocked`, after the block that sets `playerCbt.WeaponProficiencyRank`, add:

```go
// Wire save ability mods from character ability scores.
playerCbt.GritMod = combat.AbilityMod(sess.Abilities.Grit)
playerCbt.QuicknessMod = combat.AbilityMod(sess.Abilities.Quickness)
playerCbt.SavvyMod = combat.AbilityMod(sess.Abilities.Savvy)

// Wire save proficiency ranks from session.
playerCbt.ToughnessRank = sess.Proficiencies["toughness"]
playerCbt.HustleRank = sess.Proficiencies["hustle"]
playerCbt.CoolRank = sess.Proficiencies["cool"]
if playerCbt.ToughnessRank == "" { playerCbt.ToughnessRank = "untrained" }
if playerCbt.HustleRank == "" { playerCbt.HustleRank = "untrained" }
if playerCbt.CoolRank == "" { playerCbt.CoolRank = "untrained" }
```

**Step 4: Run tests**

```
go test ./internal/game/combat/... ./internal/gameserver/... -timeout 60s 2>&1 | tail -10
```

Fix any compilation errors (update test files that construct `Combatant` literals if needed — they'll need the new fields, but zero values are fine).

**Step 5: Commit**

```bash
git add internal/game/combat/combat.go internal/gameserver/combat_handler.go internal/game/combat/combat_test.go
git commit -m "feat: add save fields to Combatant and wire from PlayerSession"
```

---

### Task 3: ResolveSave function and update grenade resolver

**Files:**
- Modify: `internal/game/combat/resolver.go`
- Modify: `internal/game/combat/resolver_test.go` (or equivalent test file)

**Step 1: Write failing tests**

In the combat resolver test file, add:

```go
func TestResolveSave_Untrained_LowTotal(t *testing.T) {
    // untrained = 0 proficiency bonus; with low ability mod vs high DC should fail
    c := &Combatant{Level: 1, GritMod: -1, ToughnessRank: "untrained"}
    // Use a deterministic source that always rolls 1
    src := rand.New(rand.NewSource(0))
    // Roll enough times to find a failure vs DC 15
    outcomes := map[Outcome]int{}
    for range 100 {
        outcome := ResolveSave("toughness", c, 15, src)
        outcomes[outcome]++
    }
    // With untrained at level 1, GritMod -1, should see failures
    assert.Greater(t, outcomes[OutcomeFailure]+outcomes[OutcomeCritFailure], 0)
}

func TestProperty_ResolveSave_TrainedBeatsLowDC(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        level := rapid.IntRange(5, 20).Draw(rt, "level")
        abilityMod := rapid.IntRange(2, 5).Draw(rt, "mod")
        c := &Combatant{Level: level, GritMod: abilityMod, ToughnessRank: "trained"}
        src := rand.New(rand.NewSource(rapid.Int64().Draw(rt, "seed")))
        // trained at level 5+, mod 2+: minimum total = 1 + mod + (level+2) >= 1+2+7 = 10
        // vs DC 5 should always succeed or crit_success
        outcome := ResolveSave("toughness", c, 5, src)
        if outcome == OutcomeFailure || outcome == OutcomeCritFailure {
            rt.Fatalf("trained combatant failed DC 5 at level %d mod %d", level, abilityMod)
        }
    })
}

func TestResolveSave_UnknownType_ReturnsCritFailure(t *testing.T) {
    c := &Combatant{Level: 1, ToughnessRank: "trained"}
    src := rand.New(rand.NewSource(42))
    outcome := ResolveSave("unknown", c, 10, src)
    assert.Equal(t, OutcomeCritFailure, outcome)
}
```

Run: `go test ./internal/game/combat/... -run TestResolveSave -v`
Expected: FAIL (function doesn't exist)

**Step 2: Implement ResolveSave**

In `internal/game/combat/resolver.go`, add:

```go
// ResolveSave resolves a saving throw for a combatant against a DC.
// saveType must be "toughness", "hustle", or "cool".
// Returns OutcomeCritFailure for unknown save types.
//
// Precondition: combatant and src must be non-nil; dc >= 0.
// Postcondition: Returns a 4-tier Outcome based on 1d20 + ability_mod + proficiency_bonus vs dc.
func ResolveSave(saveType string, combatant *Combatant, dc int, src rand.Source) Outcome {
    var abilityMod int
    var rank string
    switch saveType {
    case "toughness":
        abilityMod = combatant.GritMod
        rank = combatant.ToughnessRank
    case "hustle":
        abilityMod = combatant.QuicknessMod
        rank = combatant.HustleRank
    case "cool":
        abilityMod = combatant.SavvyMod
        rank = combatant.CoolRank
    default:
        return OutcomeCritFailure
    }
    rng := rand.New(src)
    roll := rng.Intn(20) + 1
    total := roll + abilityMod + CombatProficiencyBonus(combatant.Level, rank)
    return OutcomeFor(total, dc)
}
```

**Step 3: Update grenade resolver to use ResolveSave**

In `internal/game/combat/resolver.go`, find the grenade save section (around line 148-151):

```go
// OLD:
saveRaw := src.Intn(20) + 1
saveTotal := saveRaw + target.DexMod
saveOutcome := OutcomeFor(saveTotal, grenade.SaveDC)
```

Replace with:
```go
saveOutcome := ResolveSave("hustle", target, grenade.SaveDC, src)
saveRaw := 0    // not tracked individually anymore
saveTotal := 0  // not tracked individually anymore
```

Check the `GrenadeResult` struct — if `SaveRoll` and `SaveTotal` fields are exported and used in tests, keep them but set them to 0 (or remove them if unused outside tests). Read the struct definition and usages before deciding.

**Step 4: Run tests**

```
go test ./internal/game/combat/... -timeout 60s -v 2>&1 | tail -20
```

Fix any failures.

**Step 5: Commit**

```bash
git add internal/game/combat/resolver.go internal/game/combat/resolver_test.go
git commit -m "feat: add ResolveSave function; update grenade resolver to use hustle save"
```

---

### Task 4: Add saves to CharacterSheetView proto and renderer

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go` (handleCharSheet population)
- Modify: `internal/frontend/handlers/text_renderer.go` (RenderCharacterSheet)
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Add save fields to proto**

In `api/proto/game/v1/game.proto`, in `CharacterSheetView` message, add after field 25 (proficiencies):

```protobuf
int32 toughness_save = 26; // static bonus (no d20): ability_mod + proficiency_bonus
int32 hustle_save    = 27;
int32 cool_save      = 28;
```

Run: `cd /home/cjohannsen/src/mud && make proto`
Expected: regenerates `internal/gameserver/gamev1/game.pb.go` cleanly.

**Step 2: Populate save fields in handleCharSheet**

In `internal/gameserver/grpc_service.go`, find `handleCharSheet` where `CharacterSheetView` is built. After the proficiencies block, add:

```go
level := 1 // placeholder until levelling is implemented
view.ToughnessSave = int32(combat.AbilityMod(int(sess.Abilities.Grit)) +
    combat.CombatProficiencyBonus(level, sess.Proficiencies["toughness"]))
view.HustleSave = int32(combat.AbilityMod(int(sess.Abilities.Quickness)) +
    combat.CombatProficiencyBonus(level, sess.Proficiencies["hustle"]))
view.CoolSave = int32(combat.AbilityMod(int(sess.Abilities.Savvy)) +
    combat.CombatProficiencyBonus(level, sess.Proficiencies["cool"]))
```

**Step 3: Write failing renderer test**

In `internal/frontend/handlers/text_renderer_test.go`, add:

```go
func TestRenderCharacterSheet_Saves(t *testing.T) {
    view := &gamev1.CharacterSheetView{
        Name:          "Hero",
        Level:         1,
        ToughnessSave: 5,
        HustleSave:    3,
        CoolSave:      2,
    }
    result := RenderCharacterSheet(view, 80)
    assert.Contains(t, result, "Saves")
    assert.Contains(t, result, "Toughness")
    assert.Contains(t, result, "+5")
    assert.Contains(t, result, "Hustle")
    assert.Contains(t, result, "+3")
    assert.Contains(t, result, "Cool")
    assert.Contains(t, result, "+2")
}
```

Run: `go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_Saves -v`
Expected: FAIL

**Step 4: Add saves section to RenderCharacterSheet**

In `internal/frontend/handlers/text_renderer.go`, in `RenderCharacterSheet`, add a `--- Saves ---` section to the `left` column, after the `--- Defense ---` section and before `--- Weapons ---`. Format:

```go
left = append(left, slPlain(""))
left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Saves ---")))
toughStr := fmt.Sprintf("+%d", csv.GetToughnessSave())
if csv.GetToughnessSave() < 0 { toughStr = fmt.Sprintf("%d", csv.GetToughnessSave()) }
hustleStr := fmt.Sprintf("+%d", csv.GetHustleSave())
if csv.GetHustleSave() < 0 { hustleStr = fmt.Sprintf("%d", csv.GetHustleSave()) }
coolStr := fmt.Sprintf("+%d", csv.GetCoolSave())
if csv.GetCoolSave() < 0 { coolStr = fmt.Sprintf("%d", csv.GetCoolSave()) }
left = append(left, slPlain(fmt.Sprintf("Toughness: %s  Hustle: %s  Cool: %s",
    toughStr, hustleStr, coolStr)))
```

**Step 5: Run all tests**

```
go test ./internal/frontend/handlers/... ./internal/gameserver/... -timeout 60s 2>&1 | tail -10
go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"
```

**Step 6: Update FEATURES.md**

Mark `[ ] Saving rolls` as `[x]`.

**Step 7: Commit and deploy**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
    internal/gameserver/grpc_service.go \
    internal/frontend/handlers/text_renderer.go \
    internal/frontend/handlers/text_renderer_test.go \
    docs/requirements/FEATURES.md
git commit -m "feat: add Toughness/Hustle/Cool saves to character sheet"
make k8s-redeploy 2>&1 | tail -8
```
