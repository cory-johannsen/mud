# Job Development Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the job system with tier advancement (Basic → Specialist → Expert), full drawback definitions (passive + situational), multi-job cumulative benefits, and drawback YAML for all 52 applicable jobs.

**Architecture:** Four layers: (1) schema extensions to `Job` struct and `JobPrerequisites`, (2) a `DrawbackEngine` package wired into existing trigger points, (3) a `ComputeHeldJobBenefits` pure function that aggregates multi-job benefits, (4) a `CharacterJobsRepository` and `character_jobs` DB table for multi-job persistence with atomic `train` transactions. `PlayerSession` gains `HeldJobs []string` and `ActiveJobID string`. The `train` handler gains feat prerequisite validation, atomic credit+job DB transaction, passive drawback application, and cumulative benefit recomputation. A new `ApplyTagged`/`RemoveBySource`/`Active` API on `ActiveSet` enables source-tracked conditions.

**Tech Stack:** Go, YAML, pgx (existing), `internal/game/condition`, `internal/game/ruleset`, `internal/game/drawback` (new), `internal/gameserver`

---

## Reference

- **Spec:** `docs/superpowers/specs/2026-03-20-job-development-design.md`
- **Key existing files:**
  - `internal/game/ruleset/job.go` — `Job`, `JobDrawback` (to be replaced)
  - `internal/game/condition/active.go` — `ActiveCondition`, `ActiveSet.Apply`
  - `internal/game/npc/noncombat.go` — `JobPrerequisites`, `CheckJobPrerequisites`
  - `internal/gameserver/grpc_service_job_trainer.go` — `handleTrainJob`
  - `internal/gameserver/combat_handler.go` — `SetOnCombatEnd`, `onCombatEndFn`
  - `internal/game/character/builder.go` — `BuildSkillsFromJob`, `BuildFeatsFromJob`
  - `internal/game/session/manager.go` — `PlayerSession.Jobs map[string]int` (add `HeldJobs []string`, `ActiveJobID string`)

## File Structure

| File | Action |
|---|---|
| `internal/game/ruleset/job.go` | Replace `JobDrawback` with `DrawbackDef`; add `Tier`, `AdvancementRequirements`; validate `tier` in `LoadJobs` |
| `internal/game/ruleset/job_test.go` | Add tests for tier validation and drawback struct loading |
| `content/jobs/*.yaml` | Add `tier: 1` to all 80 job files (Tasks 2) |
| `internal/game/condition/active.go` | Add `Source string` to `ActiveCondition`; add `ApplyTagged`, `RemoveBySource`, `TickCalendar` |
| `internal/game/condition/active_test.go` | Tests for new methods |
| `internal/game/npc/noncombat.go` | Add `RequiredFeats []string` to `JobPrerequisites`; update `CheckJobPrerequisites` |
| `internal/game/npc/noncombat_test.go` | Tests for feat prerequisite validation |
| `internal/gameserver/grpc_service_job_trainer.go` | Add feat prereq check + passive drawback application in `handleTrainJob` |
| `internal/gameserver/grpc_service_job_trainer_test.go` | Tests for feat prereq rejection and drawback application |
| `internal/game/character/builder.go` | Add `ComputeHeldJobBenefits` pure function |
| `internal/game/character/builder_test.go` | Property-based test for benefit aggregation |
| `internal/game/drawback/engine.go` | New: `DrawbackEngine` with `FireTrigger`, trigger/type constants |
| `internal/game/drawback/engine_test.go` | New: tests for trigger dispatch |
| `internal/gameserver/grpc_service.go` | Wire drawback triggers at combat end, room entry; login passive application |
| `internal/storage/postgres/character_jobs.go` | New: `CharacterJobsRepository` with `AddJob`, `RemoveJob`, `ListJobs` |
| `internal/storage/postgres/character_jobs_test.go` | New: integration tests for repository |
| DB migration | New: `character_jobs` table |
| `internal/game/session/manager.go` | Add `HeldJobs []string`, `ActiveJobID string` to `PlayerSession` |
| `content/jobs/*.yaml` | Add drawback definitions to ~52 applicable job files (Task 12) |

---

## Task 1: Extend Job Struct — Tier, AdvancementRequirements, DrawbackDef

**Files:**
- Modify: `internal/game/ruleset/job.go`
- Modify: `internal/game/ruleset/job_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/ruleset/job_test.go`:

```go
func TestLoadJob_MissingTier_Fatal(t *testing.T) {
    // job YAML without tier field should cause LoadJobs error
    dir := t.TempDir()
    writeJobFile(t, dir, "notier.yaml", `
id: notier
name: No Tier
archetype: test
key_ability: grit
hit_points_per_level: 8
`)
    _, err := ruleset.LoadJobs(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "tier")
}

func TestLoadJob_TierPresent_NoError(t *testing.T) {
    dir := t.TempDir()
    writeJobFile(t, dir, "tiered.yaml", `
id: tiered
name: Tiered
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
`)
    jobs, err := ruleset.LoadJobs(dir)
    require.NoError(t, err)
    require.Len(t, jobs, 1)
    assert.Equal(t, 1, jobs[0].Tier)
}

func TestLoadJob_DrawbackDef_FullSchema(t *testing.T) {
    dir := t.TempDir()
    writeJobFile(t, dir, "withdrawback.yaml", `
id: withdrawback
name: With Drawback
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
drawbacks:
  - id: glass_jaw
    type: passive
    description: "You hit hard but can't take a hit."
    stat_modifier:
      stat: grit
      amount: -1
  - id: blood_fury
    type: situational
    trigger: on_leave_combat_without_kill
    effect_condition_id: demoralized
    duration: "1h"
    description: "If you didn't finish anyone off, you spiral."
`)
    jobs, err := ruleset.LoadJobs(dir)
    require.NoError(t, err)
    require.Len(t, jobs, 1)
    require.Len(t, jobs[0].Drawbacks, 2)
    assert.Equal(t, "glass_jaw", jobs[0].Drawbacks[0].ID)
    assert.Equal(t, "passive", jobs[0].Drawbacks[0].Type)
    assert.NotNil(t, jobs[0].Drawbacks[0].StatModifier)
    assert.Equal(t, "grit", jobs[0].Drawbacks[0].StatModifier.Stat)
    assert.Equal(t, -1, jobs[0].Drawbacks[0].StatModifier.Amount)
    assert.Equal(t, "situational", jobs[0].Drawbacks[1].Type)
    assert.Equal(t, "on_leave_combat_without_kill", jobs[0].Drawbacks[1].Trigger)
    assert.Equal(t, "1h", jobs[0].Drawbacks[1].Duration)
}

func TestLoadJob_InvalidDrawbackDuration_Fatal(t *testing.T) {
    dir := t.TempDir()
    writeJobFile(t, dir, "badduration.yaml", `
id: badduration
name: Bad Duration
archetype: test
key_ability: grit
hit_points_per_level: 8
tier: 1
drawbacks:
  - id: bad_timer
    type: situational
    trigger: on_leave_combat_without_kill
    effect_condition_id: demoralized
    duration: "not-a-duration"
    description: "Invalid duration."
`)
    _, err := ruleset.LoadJobs(dir)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "duration")
}

// helper used across job tests
func writeJobFile(t *testing.T, dir, name, content string) {
    t.Helper()
    if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
        t.Fatalf("writeJobFile: %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run "TestLoadJob_" -v
```
Expected: FAIL — `Tier` field doesn't exist yet; `DrawbackDef` not yet defined

- [ ] **Step 3: Replace `JobDrawback` and add new types in `job.go`**

Replace the existing `JobDrawback` struct and add new types. Also add `Tier` and `AdvancementRequirements` to `Job`. Add validation in `LoadJobs`:

```go
// DrawbackDef defines a mandatory flaw tied to a job (replaces JobDrawback).
// Type is "passive" or "situational" (REQ-JD-8, REQ-JD-10).
type DrawbackDef struct {
    ID                string        `yaml:"id"`
    Type              string        `yaml:"type"` // "passive" | "situational"
    Description       string        `yaml:"description"`
    ConditionID       string        `yaml:"condition_id,omitempty"`
    StatModifier      *StatModifier `yaml:"stat_modifier,omitempty"`
    Trigger           string        `yaml:"trigger,omitempty"`
    EffectConditionID string        `yaml:"effect_condition_id,omitempty"`
    Duration          string        `yaml:"duration,omitempty"` // Go time.Duration string; default "1h"
}

// StatModifier is a persistent stat penalty applied while a job is held.
type StatModifier struct {
    Stat   string `yaml:"stat"`
    Amount int    `yaml:"amount"`
}

// AdvancementRequirements defines prerequisites for advancing to this job tier.
type AdvancementRequirements struct {
    MinLevel           int               `yaml:"min_level,omitempty"`
    RequiredFeats      []string          `yaml:"required_feats,omitempty"`
    RequiredSkillRanks map[string]string `yaml:"required_skill_ranks,omitempty"`
    PrerequisiteJobs   []string          `yaml:"prerequisite_jobs,omitempty"`
}
```

Update `Job` struct:
```go
type Job struct {
    // ... existing fields ...
    Tier                    int                     `yaml:"tier"`
    AdvancementRequirements AdvancementRequirements `yaml:"advancement_requirements,omitempty"`
    Drawbacks               []DrawbackDef           `yaml:"drawbacks,omitempty"`
    // Remove: Drawbacks []JobDrawback (replaced)
}
```

Update `LoadJobs` to validate tier and drawback durations after unmarshaling each job:
```go
// After yaml.Unmarshal and technology grants validation:
if j.Tier == 0 {
    return nil, fmt.Errorf("job %q: missing required field 'tier'", j.ID)
}
for _, db := range j.Drawbacks {
    if db.Duration != "" {
        if _, err := time.ParseDuration(db.Duration); err != nil {
            return nil, fmt.Errorf("job %q drawback %q: invalid duration %q: %w", j.ID, db.ID, db.Duration, err)
        }
    }
}
```

Also add `"time"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run "TestLoadJob_" -v
```
Expected: All PASS. Also run full suite:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -v 2>&1 | tail -20
```
Expected: All PASS (existing tests may fail if `JobDrawback` was used elsewhere — fix any compilation errors first)

- [ ] **Step 5: Migrate existing job YAML drawbacks to new DrawbackDef format**

Several existing job YAML files use the old `JobDrawback` format (`name:` + `description:`). These will silently drop data when the struct is changed to `DrawbackDef`. Find and migrate them:

```bash
cd /home/cjohannsen/src/mud && grep -rl "drawbacks:" content/jobs/ | xargs grep -l "^  - name:" | head -20
```

For each file found, convert the old format to the new `DrawbackDef` format. The old format:
```yaml
drawbacks:
  - name: Wanted
    description: "You are wanted by the authorities..."
```

Becomes (preserving the description, adding required `id` and `type` fields):
```yaml
drawbacks:
  - id: wanted
    type: passive
    description: "You are wanted by the authorities..."
```

Generate an `id` by snake_casing the old `name` value. Confirm all migrated files load correctly:
```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadJobs -v
```

- [ ] **Step 5b: Add REQ-JD-2 default min_level logic to `LoadJobs`**

After the tier validation in `LoadJobs`, add default `min_level` for `AdvancementRequirements`:

```go
// REQ-JD-2: default min_level for advancement_requirements if absent
if j.Tier == 2 && j.AdvancementRequirements.MinLevel == 0 {
    j.AdvancementRequirements.MinLevel = 10
}
if j.Tier == 3 && j.AdvancementRequirements.MinLevel == 0 {
    j.AdvancementRequirements.MinLevel = 15
}
```

Add a test:
```go
func TestLoadJob_Tier2_DefaultsMinLevel10(t *testing.T) {
    dir := t.TempDir()
    writeJobFile(t, dir, "specialist.yaml", `
id: specialist_goon
name: Specialist Goon
archetype: aggressor
key_ability: brutality
hit_points_per_level: 10
tier: 2
`)
    jobs, err := ruleset.LoadJobs(dir)
    require.NoError(t, err)
    assert.Equal(t, 10, jobs[0].AdvancementRequirements.MinLevel)
}
```

- [ ] **Step 6: Fix any compilation breakage from JobDrawback removal**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1 | head -30
```
Any references to `JobDrawback` must be updated to use `DrawbackDef` (check all files referencing the old type).

- [ ] **Step 7: Commit**

```bash
git add internal/game/ruleset/job.go internal/game/ruleset/job_test.go content/jobs/
git commit -m "feat(jobs): add Tier/AdvancementRequirements/DrawbackDef to Job struct; validate tier; migrate existing drawbacks (REQ-JD-1/2)"
```

---

## Task 2: Add `tier: 1` to All 80 Job YAML Files

**Files:**
- Modify: `content/jobs/*.yaml` (all 80 files)

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ruleset/job_test.go`:

```go
func TestLoadJobs_AllHaveTier(t *testing.T) {
    jobs, err := ruleset.LoadJobs("../../../content/jobs")
    require.NoError(t, err)
    for _, j := range jobs {
        assert.NotZero(t, j.Tier, "job %q missing tier field", j.ID)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadJobs_AllHaveTier -v
```
Expected: FAIL — existing jobs have no tier field

- [ ] **Step 3: Add `tier: 1` to every job YAML file**

All existing jobs are Basic (tier 1). Use a script to add the field to each file:

```bash
cd /home/cjohannsen/src/mud
for f in content/jobs/*.yaml; do
  # Add "tier: 1" after the "hit_points_per_level:" line if not already present
  if ! grep -q "^tier:" "$f"; then
    sed -i '/^hit_points_per_level:/a tier: 1' "$f"
  fi
done
```

Verify all 80 files were updated:
```bash
grep -L "^tier:" content/jobs/*.yaml | wc -l
```
Expected: 0 (no files missing tier)

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run "TestLoadJob" -v
```
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add content/jobs/
git commit -m "feat(jobs): add tier: 1 to all 80 existing job YAML files (all are Basic tier)"
```

---

## Task 3: Add Source Tracking to ActiveCondition

**Files:**
- Modify: `internal/game/condition/active.go`
- Modify: `internal/game/condition/active_test.go`

The spec requires passive drawback conditions to be tagged with `drawback:<job_id>` source (REQ-JD-8) so they can be removed when a job is dropped. Situational drawback conditions need real-time expiry (REQ-JD-11). Add three things: `Source string` field, `ApplyTagged` method, `RemoveBySource` method, and `TickCalendar` method for real-time expiry.

- [ ] **Step 1: Write the failing tests**

Add to `internal/game/condition/active_test.go`:

```go
func TestActiveSet_ApplyTagged_SetsSource(t *testing.T) {
    set := condition.NewActiveSet()
    def := &condition.ConditionDef{ID: "frightened", DurationType: "permanent"}
    err := set.ApplyTagged("uid1", def, 1, -1, "drawback:goon")
    require.NoError(t, err)
    require.True(t, set.Has("frightened"))
    assert.Equal(t, "drawback:goon", set.SourceOf("frightened"))
}

func TestActiveSet_RemoveBySource_RemovesMatchingConditions(t *testing.T) {
    set := condition.NewActiveSet()
    def1 := &condition.ConditionDef{ID: "frightened", DurationType: "permanent"}
    def2 := &condition.ConditionDef{ID: "fatigued", DurationType: "permanent"}
    def3 := &condition.ConditionDef{ID: "stunned", DurationType: "permanent"}
    _ = set.ApplyTagged("u", def1, 1, -1, "drawback:goon")
    _ = set.ApplyTagged("u", def2, 1, -1, "drawback:goon")
    _ = set.ApplyTagged("u", def3, 1, -1, "drawback:thug")
    set.RemoveBySource("u", "drawback:goon")
    assert.False(t, set.Has("frightened"))
    assert.False(t, set.Has("fatigued"))
    assert.True(t, set.Has("stunned"))
}

func TestActiveSet_TickCalendar_ExpiresTimedConditions(t *testing.T) {
    set := condition.NewActiveSet()
    def := &condition.ConditionDef{ID: "demoralized", DurationType: "permanent"}
    expiresAt := time.Now().Add(-time.Second) // already expired
    err := set.ApplyTaggedWithExpiry("u", def, 1, "drawback:goon", expiresAt)
    require.NoError(t, err)
    require.True(t, set.Has("demoralized"))
    expired := set.TickCalendar("u", time.Now())
    assert.Contains(t, expired, "demoralized")
    assert.False(t, set.Has("demoralized"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... -run "TestActiveSet_ApplyTagged|TestActiveSet_RemoveBySource|TestActiveSet_TickCalendar" -v
```
Expected: FAIL — methods don't exist

- [ ] **Step 3: Add Source field and new methods to `active.go`**

Add `Source string` and `ExpiresAt *time.Time` fields to `ActiveCondition`:

```go
type ActiveCondition struct {
    Def               *ConditionDef
    Stacks            int
    DurationRemaining int        // -1 = permanent or until_save
    Source            string     // e.g. "drawback:goon"; empty for non-tagged conditions
    ExpiresAt         *time.Time // non-nil for real-time-expiring conditions
}
```

Add these methods to `ActiveSet`:

```go
// ApplyTagged is like Apply but attaches a source tag to the condition.
//
// Precondition: def must not be nil; source may be empty.
// Postcondition: Has(def.ID) is true; condition Source is set to source.
func (s *ActiveSet) ApplyTagged(uid string, def *ConditionDef, stacks, duration int, source string) error {
    if err := s.Apply(uid, def, stacks, duration); err != nil {
        return err
    }
    if ac, ok := s.conditions[def.ID]; ok {
        ac.Source = source
    }
    return nil
}

// ApplyTaggedWithExpiry applies a condition with a source tag and a real-time expiry.
// The condition is removed by TickCalendar when now >= expiresAt.
//
// Precondition: def must not be nil.
func (s *ActiveSet) ApplyTaggedWithExpiry(uid string, def *ConditionDef, stacks int, source string, expiresAt time.Time) error {
    if err := s.Apply(uid, def, stacks, -1); err != nil {
        return err
    }
    if ac, ok := s.conditions[def.ID]; ok {
        ac.Source = source
        t := expiresAt
        ac.ExpiresAt = &t
    }
    return nil
}

// SourceOf returns the Source tag of the active condition with id, or "" if not present.
func (s *ActiveSet) SourceOf(id string) string {
    if ac, ok := s.conditions[id]; ok {
        return ac.Source
    }
    return ""
}

// RemoveBySource removes all conditions whose Source equals source.
//
// Postcondition: Has(id) is false for all conditions with matching source.
func (s *ActiveSet) RemoveBySource(uid, source string) {
    for id, ac := range s.conditions {
        if ac.Source == source {
            s.Remove(uid, id)
        }
    }
}

// TickCalendar removes all conditions with a non-nil ExpiresAt that is before or equal to now.
// Returns the IDs of removed conditions.
//
// Postcondition: all expired real-time conditions are removed.
func (s *ActiveSet) TickCalendar(uid string, now time.Time) []string {
    var expired []string
    for id, ac := range s.conditions {
        if ac.ExpiresAt != nil && !now.Before(*ac.ExpiresAt) {
            expired = append(expired, id)
            s.Remove(uid, id)
        }
    }
    return expired
}

// Active returns a snapshot of all active conditions for uid.
//
// Postcondition: returned slice is a copy; mutations do not affect the set.
func (s *ActiveSet) Active(uid string) []ActiveCondition {
    result := make([]ActiveCondition, 0, len(s.conditions))
    for _, ac := range s.conditions {
        result = append(result, *ac)
    }
    return result
}
```

Add `"time"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/condition/... -v
```
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/condition/active.go internal/game/condition/active_test.go
git commit -m "feat(condition): add Source/ExpiresAt to ActiveCondition; add ApplyTagged, RemoveBySource, TickCalendar"
```

---

## Task 4: Add RequiredFeats to JobPrerequisites

**Files:**
- Modify: `internal/game/npc/noncombat.go`
- Modify: `internal/game/npc/noncombat_test.go` (or create if not present)

- [ ] **Step 1: Write failing unit test in `noncombat_test.go`**

Add to `internal/game/npc/noncombat_test.go` (create if not present):

```go
func TestCheckJobPrerequisites_RequiredFeat_Missing(t *testing.T) {
    job := npc.TrainableJob{
        JobID: "specialist_goon",
        Prerequisites: npc.JobPrerequisites{
            RequiredFeats: []string{"raging_threat"},
        },
    }
    err := npc.CheckJobPrerequisites(job, 10, map[string]int{}, map[string]int{}, map[string]string{}, []string{})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "raging_threat")
}

func TestCheckJobPrerequisites_RequiredFeat_Present(t *testing.T) {
    job := npc.TrainableJob{
        JobID: "specialist_goon",
        Prerequisites: npc.JobPrerequisites{
            RequiredFeats: []string{"raging_threat"},
        },
    }
    err := npc.CheckJobPrerequisites(job, 10, map[string]int{}, map[string]int{}, map[string]string{}, []string{"raging_threat"})
    require.NoError(t, err)
}
```

- [ ] **Step 1b: Run unit test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestCheckJobPrerequisites_RequiredFeat" -v
```
Expected: FAIL — `RequiredFeats` field and parameter don't exist

- [ ] **Step 1c: Write integration-level failing test**

Add to `internal/gameserver/grpc_service_job_trainer_test.go`:

```go
func TestHandleTrainJob_MissingRequiredFeat_Rejected(t *testing.T) {
    // Setup: job requires feat "raging_threat" which player does not hold
    // Expect: messageEvent with "feat" in the message
    // (Use the existing test harness pattern in grpc_service_job_trainer_test.go)
    s := newTestServer(t)
    uid := "player1"
    setupPlayerSession(t, s, uid, map[string]any{
        "feats": []string{}, // no feats
    })
    setupJobTrainerNPC(t, s, uid, npc.TrainableJob{
        JobID:        "specialist_goon",
        TrainingCost: 0,
        Prerequisites: npc.JobPrerequisites{
            RequiredFeats: []string{"raging_threat"},
        },
    })
    req := &gamev1.TrainJobRequest{NpcName: "Trainer", JobId: "specialist_goon"}
    evt, err := s.handleTrainJob(uid, req)
    require.NoError(t, err)
    assert.Contains(t, strings.ToLower(evt.GetMessage().GetText()), "feat")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandleTrainJob_MissingRequiredFeat_Rejected -v
```
Expected: FAIL — `RequiredFeats` field doesn't exist yet

- [ ] **Step 3: Add `RequiredFeats` to `JobPrerequisites` in noncombat.go**

```go
// JobPrerequisites (updated):
type JobPrerequisites struct {
    MinLevel       int               `yaml:"min_level,omitempty"`
    MinJobLevel    map[string]int    `yaml:"min_job_level,omitempty"`
    MinAttributes  map[string]int    `yaml:"min_attributes,omitempty"`
    MinSkillRanks  map[string]string `yaml:"min_skill_ranks,omitempty"`
    RequiredJobs   []string          `yaml:"required_jobs,omitempty"`
    RequiredFeats  []string          `yaml:"required_feats,omitempty"` // NEW (REQ-JD-3)
}
```

Update `CheckJobPrerequisites` signature to accept playerFeats:

```go
// CheckJobPrerequisites validates all prerequisites for training a job.
// Added parameter: playerFeats []string — the feat IDs the player currently holds.
func CheckJobPrerequisites(job TrainableJob, level int, playerJobs map[string]int, playerAttrs map[string]int, playerSkills map[string]string, playerFeats []string) error {
    // ... existing checks ...
    // New: feat check after existing checks
    for _, featID := range job.Prerequisites.RequiredFeats {
        found := false
        for _, heldFeat := range playerFeats {
            if heldFeat == featID {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("prerequisite not met: you must have feat %q", featID)
        }
    }
    return nil
}
```

- [ ] **Step 4: Update `handleTrainJob` to pass player feats**

In `grpc_service_job_trainer.go`, feats are stored in the DB via `s.characterFeatsRepo`. `PlayerSession` does not have a `Feats` field — feats are loaded on demand from the repo. Update the `CheckJobPrerequisites` call:

```go
playerFeats, featErr := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
if featErr != nil {
    playerFeats = []string{}
}
if err := npc.CheckJobPrerequisites(*trainable, sess.Level, playerJobs, playerAttrs, playerSkills, playerFeats); err != nil {
    return messageEvent(err.Error()), nil
}
```

`s.characterFeatsRepo` is the `*postgres.CharacterFeatsRepository` field on `GameServiceServer`. Verify the exact field name via `grep -n "characterFeatsRepo" internal/gameserver/grpc_service.go`.

- [ ] **Step 5: Fix any other callers of CheckJobPrerequisites**

```bash
cd /home/cjohannsen/src/mud && grep -rn "CheckJobPrerequisites" --include="*.go"
```
Update all callers to pass an additional `[]string` argument.

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... ./internal/game/npc/... -v 2>&1 | tail -30
```
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/game/npc/noncombat.go internal/gameserver/grpc_service_job_trainer.go internal/gameserver/grpc_service_job_trainer_test.go
git commit -m "feat(jobs): add RequiredFeats to JobPrerequisites; validate feat prereqs in handleTrainJob (REQ-JD-3)"
```

---

## Task 5: ComputeHeldJobBenefits — Cumulative Multi-Job Stats

**Files:**
- Modify: `internal/game/character/builder.go`
- Modify: `internal/game/character/builder_test.go`

Currently `BuildSkillsFromJob`/`BuildFeatsFromJob` work on a single job. When a player trains a second job, those functions won't automatically apply the new job's benefits. Add a `ComputeHeldJobBenefits` pure function that accumulates across all held jobs.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/character/builder_test.go`:

```go
func TestComputeHeldJobBenefits_UnionsSkillsAndFeats(t *testing.T) {
    job1 := &ruleset.Job{
        ID: "job1",
        SkillGrants: &ruleset.SkillGrants{Fixed: []string{"parkour", "muscle"}},
        FeatGrants:  &ruleset.FeatGrants{Fixed: []string{"toughness"}},
    }
    job2 := &ruleset.Job{
        ID: "job2",
        SkillGrants: &ruleset.SkillGrants{Fixed: []string{"muscle", "ghosting"}},
        FeatGrants:  &ruleset.FeatGrants{Fixed: []string{"fleet", "toughness"}},
    }
    skills, feats := character.ComputeHeldJobBenefits([]*ruleset.Job{job1, job2})
    assert.Equal(t, "trained", skills["parkour"])
    assert.Equal(t, "trained", skills["muscle"])
    assert.Equal(t, "trained", skills["ghosting"])
    assert.Contains(t, feats, "toughness")
    assert.Contains(t, feats, "fleet")
    // No duplicate feats
    count := 0
    for _, f := range feats {
        if f == "toughness" {
            count++
        }
    }
    assert.Equal(t, 1, count, "toughness should appear once (deduplicated)")
}

func TestComputeHeldJobBenefits_DrawbackStatModifiers(t *testing.T) {
    job := &ruleset.Job{
        ID: "job1",
        Drawbacks: []ruleset.DrawbackDef{
            {ID: "glass_jaw", Type: "passive", StatModifier: &ruleset.StatModifier{Stat: "grit", Amount: -1}},
        },
    }
    _, _, mods := character.ComputeHeldJobBenefitsWithDrawbacks([]*ruleset.Job{job})
    require.Len(t, mods, 1)
    assert.Equal(t, "grit", mods[0].Stat)
    assert.Equal(t, -1, mods[0].Amount)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/character/... -run "TestComputeHeldJobBenefits" -v
```
Expected: FAIL — function doesn't exist

- [ ] **Step 3: Implement `ComputeHeldJobBenefits` in builder.go**

```go
// skillRankOrder maps rank names to numeric values for comparison (REQ-JD-7).
var skillRankOrder = map[string]int{
    "untrained": 0,
    "trained":   1,
    "expert":    2,
    "master":    3,
    "legendary": 4,
}

// higherRank returns the higher of two skill rank strings.
// If both are unknown, returns the first.
func higherRank(a, b string) string {
    if skillRankOrder[b] > skillRankOrder[a] {
        return b
    }
    return a
}

// ComputeHeldJobBenefits aggregates skills and feats from all held jobs.
// Returns (skills map[skill_id]rank, feats []string) — deduped union.
//
// Precondition: jobs may be empty (returns empty maps).
// Postcondition: for overlapping skills, the highest rank wins (REQ-JD-7).
// feats has each feat ID exactly once (deduplicated).
// REQ-JD-14: This is a pure function with no side effects.
func ComputeHeldJobBenefits(jobs []*ruleset.Job) (map[string]string, []string) {
    skills := make(map[string]string)
    featSet := make(map[string]bool)
    for _, job := range jobs {
        if job.SkillGrants != nil {
            for _, s := range job.SkillGrants.Fixed {
                // Grant "trained" rank; keep higher rank if already present (REQ-JD-7)
                if existing, ok := skills[s]; ok {
                    skills[s] = higherRank(existing, "trained")
                } else {
                    skills[s] = "trained"
                }
            }
        }
        if job.FeatGrants != nil {
            for _, f := range job.FeatGrants.Fixed {
                featSet[f] = true
            }
        }
    }
    feats := make([]string, 0, len(featSet))
    for f := range featSet {
        feats = append(feats, f)
    }
    return skills, feats
}

// Note: `SkillGrants` only has `Fixed []string` and `Choices *SkillChoices`. All fixed
// skills are granted at "trained" rank. If ranked grants are needed in future, extend
// SkillGrants with a Ranked map[string]string field and add a loop here.

// ComputeHeldJobBenefitsWithDrawbacks is like ComputeHeldJobBenefits but also
// returns passive stat modifiers from all held jobs' drawbacks.
//
// REQ-JD-9: Passive drawback stat modifiers are included here.
// REQ-JD-14: Pure function; no side effects.
func ComputeHeldJobBenefitsWithDrawbacks(jobs []*ruleset.Job) (map[string]string, []string, []ruleset.StatModifier) {
    skills, feats := ComputeHeldJobBenefits(jobs)
    var mods []ruleset.StatModifier
    seen := make(map[string]bool)
    for _, job := range jobs {
        for _, db := range job.Drawbacks {
            if db.Type == "passive" && db.StatModifier != nil {
                key := job.ID + ":" + db.ID
                if !seen[key] {
                    seen[key] = true
                    mods = append(mods, *db.StatModifier)
                }
            }
        }
    }
    return skills, feats, mods
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/character/... -v
```
Expected: All PASS

- [ ] **Step 5: Wire `ComputeHeldJobBenefits` into login and `handleTrainJob`**

In `handleTrainJob` (after adding the new job), call `ComputeHeldJobBenefits` and persist the updated skills/feats. Read `grpc_service_job_trainer.go` first to understand the current persistence flow, then add:

```go
// After sess.Jobs[jobID] = 1:
heldJobs := s.resolveHeldJobs(sess) // helper: looks up Job objects for all sess.Jobs keys
_, newFeats, _ := character.ComputeHeldJobBenefitsWithDrawbacks(heldJobs)

// Feats are stored in character_feats DB table — load, merge, save.
currentFeats, _ := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
featSet := make(map[string]bool, len(currentFeats))
for _, f := range currentFeats {
    featSet[f] = true
}
for _, feat := range newFeats {
    if !featSet[feat] {
        featSet[feat] = true
        currentFeats = append(currentFeats, feat)
        // Update passive feat cache for feats that are not player-activated
        if f, ok := s.featRegistry.Feat(feat); ok && !f.Active {
            sess.PassiveFeats[feat] = true
        }
    }
}
if s.characterFeatsRepo != nil && sess.CharacterID > 0 {
    _ = s.characterFeatsRepo.SetAll(context.Background(), sess.CharacterID, currentFeats)
}
```

Note: `s.characterFeatsRepo` is `*postgres.CharacterFeatsRepository` with `GetAll(ctx, charID) ([]string, error)` and `SetAll(ctx, charID, feats []string) error`. `sess.PassiveFeats map[string]bool` is the runtime feat cache for passive (non-active) feats — update it here so the session reflects the new job's passive feats immediately.

Also apply passive stat modifiers to the session's in-memory `Abilities` (REQ-JD-9). The base ability scores are loaded at login from the DB; the modifiers are a runtime adjustment:

```go
// Apply passive stat modifiers from all held jobs (REQ-JD-9)
_, _, statMods := character.ComputeHeldJobBenefitsWithDrawbacks(heldJobs)
for _, mod := range statMods {
    switch strings.ToLower(mod.Stat) {
    case "brutality":
        sess.Abilities.Brutality += mod.Amount
    case "grit":
        sess.Abilities.Grit += mod.Amount
    case "quickness":
        sess.Abilities.Quickness += mod.Amount
    case "reasoning":
        sess.Abilities.Reasoning += mod.Amount
    case "savvy":
        sess.Abilities.Savvy += mod.Amount
    case "flair":
        sess.Abilities.Flair += mod.Amount
    }
}
```

**Important:** This same block must also run in the login session-setup path (wherever abilities are loaded in `grpc_service.go`) so stat modifiers are active from login, not only after training a new job. Add the same loop in the login path referencing `resolveHeldJobs(sess)`.

Add a `resolveHeldJobs` helper to `grpc_service_job_trainer.go`:
```go
// resolveHeldJobs returns Job objects for all jobs held in the session.
func (s *GameServiceServer) resolveHeldJobs(sess *session.PlayerSession) []*ruleset.Job {
    var jobs []*ruleset.Job
    for jobID := range sess.Jobs {
        if j, ok := s.jobRegistry.Job(jobID); ok {
            jobs = append(jobs, j)
        }
    }
    return jobs
}
```

Verify `s.jobRegistry` field name via `grep -n "jobRegistry" internal/gameserver/grpc_service.go | head -5`.

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: No FAIL lines

- [ ] **Step 7: Commit**

```bash
git add internal/game/character/builder.go internal/game/character/builder_test.go internal/gameserver/grpc_service_job_trainer.go
git commit -m "feat(jobs): add ComputeHeldJobBenefits; apply cumulative skills/feats when training a new job (REQ-JD-6/7)"
```

---

## Task 6: DrawbackEngine — Trigger Registration and Dispatch

**Files:**
- Create: `internal/game/drawback/engine.go`
- Create: `internal/game/drawback/engine_test.go`

The `DrawbackEngine` fires per-player conditions when situational drawback triggers occur. It is stateless per trigger call — it reads current job definitions from the registry and the player's held jobs from the session.

- [ ] **Step 1: Write the failing test**

Create `internal/game/drawback/engine_test.go`:

```go
package drawback_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/drawback"
    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestDrawbackEngine_FireTrigger_AppliesCondition(t *testing.T) {
    // player holds a job with situational drawback: on_leave_combat_without_kill → demoralized for 1h
    condDef := &condition.ConditionDef{ID: "demoralized", DurationType: "permanent"}
    condRegistry := &stubConditionRegistry{defs: map[string]*condition.ConditionDef{"demoralized": condDef}}
    job := &ruleset.Job{
        ID: "goon",
        Drawbacks: []ruleset.DrawbackDef{
            {
                ID:                "blood_fury",
                Type:              "situational",
                Trigger:           drawback.TriggerOnLeaveCombatWithoutKill,
                EffectConditionID: "demoralized",
                Duration:          "1h",
            },
        },
    }
    activeSet := condition.NewActiveSet()
    engine := drawback.NewEngine(condRegistry)
    engine.FireTrigger("uid1", drawback.TriggerOnLeaveCombatWithoutKill, []*ruleset.Job{job}, activeSet, time.Now())
    assert.True(t, activeSet.Has("demoralized"))
    assert.Equal(t, "drawback:goon", activeSet.SourceOf("demoralized"))
}

func TestDrawbackEngine_FireTrigger_WrongTrigger_NoEffect(t *testing.T) {
    condDef := &condition.ConditionDef{ID: "demoralized", DurationType: "permanent"}
    condRegistry := &stubConditionRegistry{defs: map[string]*condition.ConditionDef{"demoralized": condDef}}
    job := &ruleset.Job{
        ID: "goon",
        Drawbacks: []ruleset.DrawbackDef{
            {ID: "blood_fury", Type: "situational", Trigger: drawback.TriggerOnLeaveCombatWithoutKill, EffectConditionID: "demoralized", Duration: "1h"},
        },
    }
    activeSet := condition.NewActiveSet()
    engine := drawback.NewEngine(condRegistry)
    engine.FireTrigger("uid1", drawback.TriggerOnFailSkillCheck, []*ruleset.Job{job}, activeSet, time.Now())
    assert.False(t, activeSet.Has("demoralized"))
}

// stubConditionRegistry for tests
type stubConditionRegistry struct {
    defs map[string]*condition.ConditionDef
}
func (r *stubConditionRegistry) Get(id string) (*condition.ConditionDef, bool) {
    def, ok := r.defs[id]
    return def, ok
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/drawback/... -v 2>&1 | head -20
```
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Create `internal/game/drawback/engine.go`**

```go
package drawback

import (
    "time"

    "github.com/cory-johannsen/mud/internal/game/condition"
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

// Drawback type constants.
const (
    DrawbackPassive     = "passive"
    DrawbackSituational = "situational"
)

// Trigger IDs for situational drawbacks (REQ-JD-10).
const (
    TriggerOnLeaveCombatWithoutKill          = "on_leave_combat_without_kill"
    TriggerOnTakeDamageInOneHitAboveThreshold = "on_take_damage_in_one_hit_above_threshold"
    TriggerOnFailSkillCheck                  = "on_fail_skill_check"
    TriggerOnEnterRoomDangerLevel            = "on_enter_room_danger_level"
)

// ConditionDefLookup provides condition definitions by ID.
// Matches the interface of *condition.Registry (method: Get).
type ConditionDefLookup interface {
    Get(id string) (*condition.ConditionDef, bool)
}

// Engine evaluates situational drawback triggers and applies their conditions.
//
// Precondition: condDefs must not be nil.
type Engine struct {
    condDefs ConditionDefLookup
}

// NewEngine creates a new DrawbackEngine.
func NewEngine(condDefs ConditionDefLookup) *Engine {
    return &Engine{condDefs: condDefs}
}

// FireTrigger evaluates all held jobs' drawbacks for the given trigger and applies
// any matching conditions to activeSet.
//
// Precondition: trigger is one of the Trigger* constants; jobs and activeSet are non-nil.
// Postcondition: matching situational drawback conditions are applied to activeSet
// with source "drawback:<job_id>" and a real-time ExpiresAt derived from now + duration.
func (e *Engine) FireTrigger(uid string, trigger string, jobs []*ruleset.Job, activeSet *condition.ActiveSet, now time.Time) {
    for _, job := range jobs {
        for _, db := range job.Drawbacks {
            if db.Type != "situational" || db.Trigger != trigger {
                continue
            }
            def, ok := e.condDefs.Get(db.EffectConditionID)
            if !ok {
                continue
            }
            dur := time.Hour // default 1h
            if db.Duration != "" {
                if parsed, err := time.ParseDuration(db.Duration); err == nil {
                    dur = parsed
                }
            }
            expiresAt := now.Add(dur)
            source := "drawback:" + job.ID
            _ = activeSet.ApplyTaggedWithExpiry(uid, def, 1, source, expiresAt)
        }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/drawback/... -v
```
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/game/drawback/
git commit -m "feat(drawback): add DrawbackEngine with FireTrigger; trigger constants for 4 situational events (REQ-JD-10)"
```

---

## Task 7: Wire Drawback Triggers into Game Server

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/grpc_service_job_trainer.go`

Wire the DrawbackEngine into the existing trigger points: combat end, room entry, damage application, and skill check resolution. Also apply passive drawback conditions at login.

- [ ] **Step 1: Write failing integration test**

Add to `internal/gameserver/grpc_service_job_trainer_test.go`:

```go
func TestHandleTrainJob_PassiveDrawbackConditionApplied(t *testing.T) {
    // Player trains a job with a passive drawback condition_id
    // Expect: condition is applied to player session after training
    s := newTestServer(t)
    uid := "player1"
    setupPlayerSession(t, s, uid, map[string]any{})
    setupJobWithPassiveDrawbackCondition(t, s, "job_with_passive", "light_sensitive")
    req := &gamev1.TrainJobRequest{NpcName: "Trainer", JobId: "job_with_passive"}
    _, err := s.handleTrainJob(uid, req)
    require.NoError(t, err)
    sess, ok := s.sessions.GetPlayer(uid)
    require.True(t, ok)
    assert.True(t, sess.Conditions.Has("light_sensitive"), "passive drawback condition should be applied")
    assert.Equal(t, "drawback:job_with_passive", sess.Conditions.SourceOf("light_sensitive"))
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestHandleTrainJob_PassiveDrawbackConditionApplied -v
```
Expected: FAIL

- [ ] **Step 3: Add DrawbackEngine to GameServiceServer**

In `grpc_service.go`, add `drawbackEngine *drawback.Engine` field to `GameServiceServer`. Initialize it in the constructor, passing the condition registry. Add a `drawbackEngine` field:

```go
import "github.com/cory-johannsen/mud/internal/game/drawback"

// In GameServiceServer struct:
drawbackEngine *drawback.Engine
```

In the `NewGameServiceServer` constructor, initialize:
```go
s.drawbackEngine = drawback.NewEngine(s.condRegistry)
```

Check the actual constructor signature and available condition registry field.

- [ ] **Step 4: Apply passive drawbacks in `handleTrainJob`**

In `grpc_service_job_trainer.go`, after computing `resolveHeldJobs` and persisting skills/feats, apply passive drawback conditions:

```go
// Apply passive drawbacks from the newly trained job
for _, db := range newJob.Drawbacks {
    if db.Type != "passive" {
        continue
    }
    source := "drawback:" + jobID
    if db.ConditionID != "" {
        def, ok := s.condRegistry.Get(db.ConditionID)
        if ok {
            _ = sess.Conditions.ApplyTagged(uid, def, 1, -1, source)
        }
    }
    // StatModifier is handled by ComputeHeldJobBenefitsWithDrawbacks at login
}
```

- [ ] **Step 5: Apply passive drawbacks at login**

In `grpc_service.go`, in the session setup path (where skills/feats are loaded after login), after loading the character's job list:

```go
// Apply passive drawback conditions for all held jobs (REQ-JD-8)
for _, job := range resolvedHeldJobs {
    for _, db := range job.Drawbacks {
        if db.Type != "passive" || db.ConditionID == "" {
            continue
        }
        source := "drawback:" + job.ID
        if !sess.Conditions.Has(db.ConditionID) { // deduplicate by condition ID
            if def, ok := s.condRegistry.Get(db.ConditionID); ok {
                _ = sess.Conditions.ApplyTagged(uid, def, 1, -1, source)
            }
        }
    }
}
```

- [ ] **Step 6: Wire combat end trigger for `on_leave_combat_without_kill`**

In `grpc_service.go`, in the `SetOnCombatEnd` callback (line ~308), add drawback trigger firing for players who got 0 kills:

```go
s.combatH.SetOnCombatEnd(func(roomID string) {
    // existing logic ...

    // Fire on_leave_combat_without_kill for players with 0 kills in this combat
    for _, uid := range playerUIDsInRoom(roomID) {
        sess, ok := s.sessions.GetPlayer(uid)
        if !ok {
            continue
        }
        kills := s.combatH.KillsInCombat(uid) // check if this method exists; if not, track via combat end data
        if kills == 0 {
            heldJobs := s.resolveHeldJobs(sess)
            s.drawbackEngine.FireTrigger(uid, drawback.TriggerOnLeaveCombatWithoutKill, heldJobs, sess.Conditions, time.Now())
        }
    }
})
```

**Note:** Read `combat_handler.go` lines 1040–1700 before implementing to understand what data is available in the combat end callback. The exact kill tracking mechanism must be verified in the combat resolution code before coding this step.

- [ ] **Step 7: Wire room entry trigger for `on_enter_room_danger_level`**

In the room-entry handler (wherever players move between rooms), after the room's danger level is determined, add:

```go
if roomDangerLevel >= 3 {
    heldJobs := s.resolveHeldJobs(sess)
    s.drawbackEngine.FireTrigger(uid, drawback.TriggerOnEnterRoomDangerLevel, heldJobs, sess.Conditions, time.Now())
}
```

Find the correct room-entry hook via `grep -n "handleMove\|MoveToRoom\|enterRoom" internal/gameserver/*.go`.

- [ ] **Step 8: Wire calendar tick for condition expiry**

In the calendar tick handler in `grpc_service.go` (in the goroutine listening on `calCh`, around line 1164), add condition expiry:

```go
case dt, ok := <-calCh:
    // ... existing period change logic ...
    // Expire real-time conditions for this session
    sess.Conditions.TickCalendar(uid, time.Now())
```

- [ ] **Step 9: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: No FAIL lines

- [ ] **Step 10: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_job_trainer.go
git commit -m "feat(drawback): wire DrawbackEngine triggers — combat end, room entry, calendar tick expiry; passive drawbacks on train/login (REQ-JD-8/10/11)"
```

---

## Task 8: Skill Check and Damage Triggers

**Files:**
- Modify: skill check resolution handler (find via grep)
- Modify: damage application handler in `combat_handler.go`
- Test: `internal/game/drawback/engine_test.go`

- [ ] **Step 1: Locate skill check and damage handler call sites**

```bash
grep -n "handleSkillCheck\|resolveSkillCheck\|SkillCheck\|skillCheck" /home/cjohannsen/src/mud/internal/gameserver/*.go | grep -v "_test" | head -15
grep -n "applyDamage\|ApplyDamage\|takeDamage\|TakeDamage\|dealDamage" /home/cjohannsen/src/mud/internal/gameserver/*.go | grep -v "_test" | head -15
```

Read the identified handler functions to understand their signatures and what constitutes a failure result (skill check) or above-threshold damage.

- [ ] **Step 2: Write failing integration tests**

After reading the handler signatures in Step 1, add to `internal/gameserver/grpc_service_drawback_test.go`:

```go
package gameserver_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// setupJobWithSituationalDrawback registers a job on the session that has a
// situational drawback with the given trigger and effect condition.
func setupJobWithSituationalDrawback(t *testing.T, s *GameServiceServer, uid, trigger, conditionID string) {
    t.Helper()
    sess, ok := s.sessions.GetPlayer(uid)
    require.True(t, ok)
    // Register a synthetic job in the job registry with a situational drawback
    // (use s.jobRegistry or the test server's stub mechanism — see how other tests seed the registry)
    sess.Jobs["test_job_"+trigger] = 1
}

// TestSkillCheckFailure_DrawbackConditionApplied verifies that a failed skill check
// fires the on_fail_skill_check drawback trigger.
// REPLACE the handler call below with the actual skill-check handler found in Step 1.
func TestSkillCheckFailure_DrawbackConditionApplied(t *testing.T) {
    s := newTestServer(t)
    uid := "player1"
    setupPlayerSession(t, s, uid, map[string]any{})
    setupJobWithSituationalDrawback(t, s, uid, "on_fail_skill_check", "demoralized")

    // Call the skill check handler with parameters that produce a failure.
    // E.g.: evt, err := s.handleSkillCheck(uid, &gamev1.SkillCheckRequest{Skill: "rigging", DC: 999})
    // (DC 999 ensures failure regardless of player skill)
    // IMPLEMENT based on actual handler signature found in Step 1.
    // After calling the handler:
    sess, ok := s.sessions.GetPlayer(uid)
    require.True(t, ok)
    assert.True(t, sess.Conditions.Has("demoralized"),
        "on_fail_skill_check drawback condition should be applied after a failed skill check")
}

// TestTakeMassiveDamage_DrawbackConditionApplied verifies that taking ≥50% max HP
// in one hit fires the on_take_damage_in_one_hit_above_threshold trigger.
// REPLACE the handler call below with the actual damage handler found in Step 1.
func TestTakeMassiveDamage_DrawbackConditionApplied(t *testing.T) {
    s := newTestServer(t)
    uid := "player1"
    setupPlayerSession(t, s, uid, map[string]any{"max_hp": 100, "hp": 100})
    setupJobWithSituationalDrawback(t, s, uid, "on_take_damage_in_one_hit_above_threshold", "shaken")

    // Call the damage application path with damage=51 (>50% of max_hp=100).
    // E.g.: s.applyDamageToPlayer(uid, 51)
    // (IMPLEMENT based on actual function signature found in Step 1)
    // After calling:
    sess, ok := s.sessions.GetPlayer(uid)
    require.True(t, ok)
    assert.True(t, sess.Conditions.Has("shaken"),
        "on_take_damage_above_threshold drawback condition should be applied after a 51-damage hit")
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestSkillCheckFailure_DrawbackConditionApplied|TestTakeMassiveDamage_DrawbackConditionApplied" -v 2>&1 | tail -20
```
Expected: FAIL — after filling in the handler calls from Step 1, tests compile but fail because wiring isn't done yet

- [ ] **Step 4: Wire `on_fail_skill_check` trigger**

In the skill check resolution handler, after a failed check result is determined:

```go
if checkFailed {
    sess, ok := s.sessions.GetPlayer(uid)
    if ok {
        heldJobs := s.resolveHeldJobs(sess)
        s.drawbackEngine.FireTrigger(uid, drawback.TriggerOnFailSkillCheck, heldJobs, sess.Conditions, time.Now())
    }
}
```

- [ ] **Step 5: Wire `on_take_damage_in_one_hit_above_threshold` trigger**

In the damage application path (after damage is computed and applied to a player), add:

```go
if damageDealt >= sess.MaxHP/2 {
    heldJobs := s.resolveHeldJobs(sess)
    s.drawbackEngine.FireTrigger(uid, drawback.TriggerOnTakeDamageInOneHitAboveThreshold, heldJobs, sess.Conditions, time.Now())
}
```

- [ ] **Step 6: Run all tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: No FAIL lines; `TestSkillCheckFailure_DrawbackConditionApplied` and `TestTakeMassiveDamage_DrawbackConditionApplied` now pass

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/ internal/game/drawback/
git commit -m "feat(drawback): wire on_fail_skill_check and on_take_damage_above_threshold triggers (REQ-JD-10)"
```

---

## Task 9: Wire DI for DrawbackEngine

**Files:**
- Modify: `cmd/gameserver/wire.go` or `internal/gameserver/providers.go`
- Modify: `cmd/gameserver/inject_gameserver.go` (Wire-generated injector)
- Test: `internal/gameserver/` integration test (build test)

- [ ] **Step 1: Write failing build test**

The Wire-generated injector must compile with the DrawbackEngine provider injected. The "failing test" here is the build itself: running `go build ./cmd/gameserver/...` with `GameServiceServer.drawbackEngine` declared but the provider not yet in the Wire graph will fail. Confirm the field is declared but not yet wired:

```bash
cd /home/cjohannsen/src/mud && grep -n "drawbackEngine" internal/gameserver/grpc_service.go | head -5
```

If the field exists but `ProvideDrawbackEngine` is not in any ProviderSet, add a compile-time test to `internal/gameserver/providers_test.go`:

```go
func TestProvideDrawbackEngine_ReturnsNonNil(t *testing.T) {
    reg := condition.NewRegistry()
    engine := ProvideDrawbackEngine(reg)
    require.NotNil(t, engine)
}
```

Run to confirm it fails (function not defined):

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestProvideDrawbackEngine -v 2>&1 | head -10
```
Expected: FAIL — `ProvideDrawbackEngine` not defined

- [ ] **Step 2: Find Wire provider files**

```bash
find /home/cjohannsen/src/mud -name "*.go" | xargs grep -l "wire.Build\|ProviderSet" | head -10
```

- [ ] **Step 3: Add DrawbackEngine provider**

In the appropriate providers file:

```go
// ProvideDrawbackEngine creates a DrawbackEngine using the condition registry.
func ProvideDrawbackEngine(condRegistry *condition.Registry) *drawback.Engine {
    return drawback.NewEngine(condRegistry)
}
```

Add to the wire ProviderSet and verify `wire.go` compiles.

- [ ] **Step 4: Run wire generation and build**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go generate ./cmd/gameserver/... 2>&1
mise exec -- go build ./... 2>&1 | head -20
```
Expected: Clean build

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: No FAIL lines; `TestProvideDrawbackEngine_ReturnsNonNil` passes

- [ ] **Step 6: Commit**

```bash
git add cmd/gameserver/ internal/gameserver/providers.go internal/gameserver/providers_test.go
git commit -m "feat(drawback): wire DrawbackEngine into DI graph via Wire provider"
```

---

## Task 10: CharacterJobsRepository — Multi-Job Persistence and setjob

**Files:**
- Create: `internal/storage/postgres/character_jobs.go`
- Create: `internal/storage/postgres/character_jobs_test.go`
- Create: DB migration (find migration tool/directory used in repo)
- Modify: `internal/game/session/manager.go` — add `HeldJobs []string`, `ActiveJobID string`
- Modify: `internal/gameserver/grpc_service_job_trainer.go` — atomic train transaction, setjob update

This task satisfies REQ-JD-4, REQ-JD-5, REQ-JD-13, and Spec §2.1.

- [ ] **Step 1: Locate DB migration directory and migration tooling**

```bash
find /home/cjohannsen/src/mud -name "*.sql" | head -10
find /home/cjohannsen/src/mud -name "migrate*" | head -5
grep -r "migrate\|migration" /home/cjohannsen/src/mud/Makefile | head -10
```

Note the naming convention for migration files (e.g., `NNNN_description.up.sql`).

- [ ] **Step 2: Write failing test for CharacterJobsRepository**

Create `internal/storage/postgres/character_jobs_test.go`:

```go
package postgres_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCharacterJobsRepository_AddAndList(t *testing.T) {
    // Requires test DB; skip if not available (same pattern as other postgres tests)
    db := openTestDB(t) // reuse existing test helper in this package
    repo := postgres.NewCharacterJobsRepository(db)
    ctx := context.Background()
    const charID int64 = 9999
    t.Cleanup(func() { _, _ = db.Exec(ctx, "DELETE FROM character_jobs WHERE character_id=$1", charID) })

    err := repo.AddJob(ctx, charID, "thug")
    require.NoError(t, err)
    err = repo.AddJob(ctx, charID, "goon")
    require.NoError(t, err)

    jobs, err := repo.ListJobs(ctx, charID)
    require.NoError(t, err)
    assert.ElementsMatch(t, []string{"thug", "goon"}, jobs)

    err = repo.RemoveJob(ctx, charID, "thug")
    require.NoError(t, err)
    jobs, err = repo.ListJobs(ctx, charID)
    require.NoError(t, err)
    assert.Equal(t, []string{"goon"}, jobs)
}
```

- [ ] **Step 3: Run test to verify it fails (table doesn't exist)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/storage/postgres/... -run TestCharacterJobsRepository -v 2>&1 | head -20
```
Expected: FAIL — `character_jobs` relation does not exist

- [ ] **Step 4: Create DB migration for `character_jobs` table**

Create migration file (use next sequential number in the migration directory):

```sql
-- character_jobs: stores the full set of jobs a character holds.
-- characters.job (existing) remains as the active job ID for backward compat.
CREATE TABLE IF NOT EXISTS character_jobs (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    job_id       TEXT   NOT NULL,
    PRIMARY KEY (character_id, job_id)
);
```

Apply migration in development:

```bash
cd /home/cjohannsen/src/mud && make migrate  # or equivalent; check Makefile
```

- [ ] **Step 5: Implement CharacterJobsRepository**

Create `internal/storage/postgres/character_jobs.go`:

```go
package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterJobsRepository manages the character_jobs table.
type CharacterJobsRepository struct {
    db *pgxpool.Pool
}

// NewCharacterJobsRepository creates a new repository.
func NewCharacterJobsRepository(db *pgxpool.Pool) *CharacterJobsRepository {
    return &CharacterJobsRepository{db: db}
}

// AddJob inserts a job for a character. No-op if already present (ON CONFLICT DO NOTHING).
//
// Precondition: characterID > 0; jobID non-empty.
func (r *CharacterJobsRepository) AddJob(ctx context.Context, characterID int64, jobID string) error {
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_jobs (character_id, job_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
        characterID, jobID)
    return err
}

// RemoveJob removes a job for a character.
func (r *CharacterJobsRepository) RemoveJob(ctx context.Context, characterID int64, jobID string) error {
    _, err := r.db.Exec(ctx,
        `DELETE FROM character_jobs WHERE character_id=$1 AND job_id=$2`,
        characterID, jobID)
    return err
}

// ListJobs returns all job IDs for a character.
func (r *CharacterJobsRepository) ListJobs(ctx context.Context, characterID int64) ([]string, error) {
    rows, err := r.db.Query(ctx,
        `SELECT job_id FROM character_jobs WHERE character_id=$1 ORDER BY job_id`,
        characterID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var jobs []string
    for rows.Next() {
        var id string
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        jobs = append(jobs, id)
    }
    return jobs, rows.Err()
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/storage/postgres/... -run TestCharacterJobsRepository -v
```
Expected: PASS

- [ ] **Step 7: Add `HeldJobs []string` and `ActiveJobID string` to `PlayerSession`**

In `internal/game/session/manager.go`, locate the `PlayerSession` struct and add the two fields:

```go
HeldJobs   []string // all job IDs the player currently holds; loaded at login (REQ-JD-4)
ActiveJobID string  // the active job; mirrors characters.job column (REQ-JD-5)
```

These fields sit alongside the existing `Jobs map[string]int`. At login, populate `HeldJobs` from the `character_jobs` table via `CharacterJobsRepository.ListJobs`.

- [ ] **Step 8: Make `handleTrainJob` atomic (REQ-JD-13)**

In `grpc_service_job_trainer.go`, wrap credit deduction and job insertion in a single DB transaction. The transaction must:
1. Deduct `trainable.TrainingCost` from `characters.currency` (or the existing currency column — verify via `grep -n "currency\|Currency" internal/storage/postgres/*.go`)
2. Insert the new job via `CharacterJobsRepository.AddJob` within the same `pgx.Tx`
3. Rollback if either step fails

```go
// Atomic credit deduction + job insertion (REQ-JD-13)
tx, err := s.db.Begin(ctx)
if err != nil {
    return messageEvent("internal error starting transaction"), nil
}
defer tx.Rollback(ctx)

if _, err := tx.Exec(ctx,
    `UPDATE characters SET currency = currency - $1 WHERE id = $2`,
    trainable.TrainingCost, sess.CharacterID); err != nil {
    return messageEvent("failed to deduct training cost"), nil
}
if _, err := tx.Exec(ctx,
    `INSERT INTO character_jobs (character_id, job_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
    sess.CharacterID, jobID); err != nil {
    return messageEvent("failed to record job training"), nil
}
if err := tx.Commit(ctx); err != nil {
    return messageEvent("failed to complete training transaction"), nil
}
// Update in-memory session after successful commit
sess.Currency -= trainable.TrainingCost
```

Verify `s.db` field name: `grep -n "\.db\b\|pgxpool\|Pool" internal/gameserver/grpc_service.go | head -10`

- [ ] **Step 9: Implement `setjob` update to `characters.job` (REQ-JD-5)**

In `handleSetJob` in `grpc_service_job_trainer.go`, after updating `sess.ActiveJobID`, persist to `characters.job`:

```go
if s.db != nil && sess.CharacterID > 0 {
    if _, err := s.db.Exec(context.Background(),
        `UPDATE characters SET job = $1 WHERE id = $2`,
        jobID, sess.CharacterID); err != nil {
        s.logger.Warn("failed to persist active job", zap.Error(err))
    }
}
```

- [ ] **Step 10: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | head -30
```
Expected: No FAIL lines

- [ ] **Step 11: Commit**

```bash
git add internal/storage/postgres/character_jobs.go internal/storage/postgres/character_jobs_test.go \
        internal/game/session/manager.go internal/gameserver/grpc_service_job_trainer.go
git commit -m "feat(jobs): add CharacterJobsRepository; atomic train transaction; setjob persists active job; HeldJobs on PlayerSession (REQ-JD-4/5/13)"
```

---

## Task 12: Drawback YAML Content for 52 Jobs

**Files:**
- Modify: ~52 files in `content/jobs/*.yaml`

This is content work. Each job gets 1–3 drawbacks appropriate to its theme. Passive drawbacks use existing condition IDs (e.g., `fatigued`, `clumsy`, `frightened`) or `stat_modifier`. Situational drawbacks reference the 4 supported trigger IDs.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/ruleset/job_test.go`:

```go
func TestLoadJobs_DrawbacksValidStructure(t *testing.T) {
    jobs, err := ruleset.LoadJobs("../../../content/jobs")
    require.NoError(t, err)
    var jobsWithDrawbacks int
    for _, j := range jobs {
        for _, db := range j.Drawbacks {
            assert.NotEmpty(t, db.ID, "job %q drawback missing id", j.ID)
            assert.Contains(t, []string{"passive", "situational"}, db.Type, "job %q drawback %q has invalid type", j.ID, db.ID)
            assert.NotEmpty(t, db.Description, "job %q drawback %q missing description", j.ID, db.ID)
            if db.Type == "situational" {
                validTriggers := []string{
                    "on_leave_combat_without_kill",
                    "on_take_damage_in_one_hit_above_threshold",
                    "on_fail_skill_check",
                    "on_enter_room_danger_level",
                }
                assert.Contains(t, validTriggers, db.Trigger, "job %q drawback %q invalid trigger", j.ID, db.ID)
                assert.NotEmpty(t, db.EffectConditionID, "job %q drawback %q missing effect_condition_id", j.ID, db.ID)
            }
        }
        if len(j.Drawbacks) > 0 {
            jobsWithDrawbacks++
        }
    }
    assert.GreaterOrEqual(t, jobsWithDrawbacks, 52, "expected at least 52 jobs to have drawbacks")
}
```

- [ ] **Step 2: Run test to verify it fails (or passes 0/52)**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadJobs_DrawbacksValidStructure -v
```
Expected: FAIL (0 jobs with drawbacks currently)

- [ ] **Step 3: Add drawback content to 52 job YAML files**

Read the spec section 3 (Drawbacks) for the schema. Refer to `docs/superpowers/specs/2026-03-20-job-development-design.md` for the full drawback definition format.

For each job, design 1–3 drawbacks appropriate to the job's theme and archetype. Use these guidelines:

- **Aggressor archetype** (thug, goon, enforcer, etc.): passive HP or defense penalty; situational triggers on non-kill combat or taking massive damage
- **Operator archetype** (hacker, fixer, etc.): social penalty (stat_modifier on Flair or smooth_talk)
- **Scout archetype** (runner, ghost, etc.): fatigue-related passive; situational on fail skill check
- **Support archetype** (medic, quartermaster, etc.): passive condition (fatigued) when out of combat supplies

**Example drawback entries for `content/jobs/thug.yaml`:**
```yaml
drawbacks:
  - id: glass_jaw
    type: passive
    description: "You dish it out but you can't always take it. -1 to Fortitude saves."
    stat_modifier:
      stat: fortitude
      amount: -1
  - id: blood_frenzy
    type: situational
    trigger: on_leave_combat_without_kill
    effect_condition_id: demoralized
    duration: "1h"
    description: "If you didn't finish someone off, you spiral into frustration."
```

**Example for `content/jobs/goon.yaml`:**
```yaml
drawbacks:
  - id: tunnel_vision
    type: passive
    description: "Your rep for brute force makes people underestimate your smarts. -1 to Intel checks."
    stat_modifier:
      stat: intel
      amount: -1
  - id: rage_spiral
    type: situational
    trigger: on_take_damage_in_one_hit_above_threshold
    effect_condition_id: frightened
    duration: "30m"
    description: "Taking a massive hit sends you into a momentary panic."
```

**List of available condition IDs to verify first:**
```bash
grep "^  - id:" /home/cjohannsen/src/mud/content/conditions.yaml | head -30
```

Add drawbacks to at least 52 of the 80 job files. Jobs with no natural drawback need not receive one (the spec says "drawbacks for 52 jobs" — not all 80).

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/ruleset/... -run TestLoadJobs -v
```
Expected: All PASS including DrawbacksValidStructure

- [ ] **Step 5: Commit**

```bash
git add content/jobs/
git commit -m "feat(jobs): add drawback definitions to 52 job YAML files (REQ-JD-8/10)"
```

---

## Final Verification

- [ ] **Full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok"
```
Expected: No FAIL lines

- [ ] **Server builds cleanly**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./cmd/gameserver/... ./cmd/devserver/... 2>&1
```
Expected: No errors

- [ ] **All job files have tier**

```bash
grep -L "^tier:" /home/cjohannsen/src/mud/content/jobs/*.yaml | wc -l
```
Expected: 0

- [ ] **No duplicate drawback IDs within any job**

```bash
# Manual spot check: grep for duplicate IDs within individual job files
for f in /home/cjohannsen/src/mud/content/jobs/*.yaml; do
  count=$(grep -c "^  - id:" "$f" 2>/dev/null || echo 0)
  unique=$(grep "^  - id:" "$f" 2>/dev/null | sort -u | wc -l)
  if [ "$count" != "$unique" ]; then echo "DUPLICATE in $f"; fi
done
```
Expected: No DUPLICATE lines
