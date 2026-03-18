# Job Technology Grants Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire archetype-owned slot progression and job-owned tech pools into the `AssignTechnologies` pipeline, then populate all 6 tech-using archetypes and their 55 jobs with Phase 1 YAML content.

**Architecture:** Add `MergeGrants`/`MergeLevelUpGrants` pure functions to the ruleset package, extend `Archetype` with `TechnologyGrants`/`LevelUpGrants` fields, update `AssignTechnologies` to merge archetype + job grants before processing, and populate archetype and job YAML files with slot progressions and tech pool entries using the 4 existing tech IDs.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`, YAML (`gopkg.in/yaml.v3`)

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/game/ruleset/technology_grants.go` | Modify | Add `MergeGrants`, `MergeLevelUpGrants`, helpers |
| `internal/game/ruleset/technology_grants_test.go` | Modify | Tests for merge functions (REQ-JTG1–5, REQ-JTG8, property) |
| `internal/game/ruleset/archetype.go` | Modify | Add `TechnologyGrants`, `LevelUpGrants` fields to `Archetype` |
| `internal/game/ruleset/archetype_test.go` | Modify/Create | Tests for archetype YAML round-trip with tech grants |
| `internal/gameserver/technology_assignment.go` | Modify | Update `AssignTechnologies` to merge archetype + job grants |
| `internal/gameserver/technology_assignment_test.go` | Modify | Tests for merged-grant behavior (REQ-JTG6, REQ-JTG7) |
| `content/archetypes/nerd.yaml` | Modify | Slot progression (prepared, levels 1–5) |
| `content/archetypes/zealot.yaml` | Modify | Slot progression (prepared, levels 1–5) |
| `content/archetypes/naturalist.yaml` | Modify | Slot progression (prepared, levels 1–5) |
| `content/archetypes/schemer.yaml` | Modify | Slot progression (prepared, levels 1–5) |
| `content/archetypes/influencer.yaml` | Modify | Slot progression (spontaneous, levels 1–5) |
| `content/archetypes/drifter.yaml` | Modify | Slot progression (prepared, levels 1+3+5) |
| `content/jobs/*.yaml` (55 files) | Modify | Add `technology_grants` pool entries per tradition |
| `docs/requirements/FEATURES.md` | Modify | Mark fixed-list item complete |

---

## Chunk 1 — Task 1: MergeGrants + MergeLevelUpGrants

### Task 1: Pure merge functions for TechnologyGrants

**Files:**
- Modify: `internal/game/ruleset/technology_grants.go`
- Modify: `internal/game/ruleset/technology_grants_test.go`

---

- [ ] **T1-S1: Write failing tests for MergeGrants**

  Append to `internal/game/ruleset/technology_grants_test.go`:

  ```go
  // REQ-JTG1: MergeGrants(nil, nil) returns nil.
  func TestMergeGrants_BothNil(t *testing.T) {
  	assert.Nil(t, ruleset.MergeGrants(nil, nil))
  }

  // REQ-JTG2: MergeGrants with one nil returns the other unchanged.
  func TestMergeGrants_NilA_ReturnsBUnchanged(t *testing.T) {
  	b := &ruleset.TechnologyGrants{Hardwired: []string{"x"}}
  	result := ruleset.MergeGrants(nil, b)
  	require.NotNil(t, result)
  	assert.Equal(t, []string{"x"}, result.Hardwired)
  }

  func TestMergeGrants_NilB_ReturnsAUnchanged(t *testing.T) {
  	a := &ruleset.TechnologyGrants{Hardwired: []string{"x"}}
  	result := ruleset.MergeGrants(a, nil)
  	require.NotNil(t, result)
  	assert.Equal(t, []string{"x"}, result.Hardwired)
  }

  // REQ-JTG3: Slot counts are summed per level.
  func TestMergeGrants_PreparedSlotsSummed(t *testing.T) {
  	a := &ruleset.TechnologyGrants{
  		Prepared: &ruleset.PreparedGrants{SlotsByLevel: map[int]int{1: 2}},
  	}
  	b := &ruleset.TechnologyGrants{
  		Prepared: &ruleset.PreparedGrants{SlotsByLevel: map[int]int{1: 1}},
  	}
  	result := ruleset.MergeGrants(a, b)
  	require.NotNil(t, result.Prepared)
  	assert.Equal(t, 3, result.Prepared.SlotsByLevel[1])
  }

  func TestMergeGrants_SpontaneousKnownAndUsesSummed(t *testing.T) {
  	a := &ruleset.TechnologyGrants{
  		Spontaneous: &ruleset.SpontaneousGrants{
  			KnownByLevel: map[int]int{1: 2},
  			UsesByLevel:  map[int]int{1: 3},
  		},
  	}
  	b := &ruleset.TechnologyGrants{
  		Spontaneous: &ruleset.SpontaneousGrants{
  			KnownByLevel: map[int]int{1: 1},
  			UsesByLevel:  map[int]int{1: 2},
  		},
  	}
  	result := ruleset.MergeGrants(a, b)
  	require.NotNil(t, result.Spontaneous)
  	assert.Equal(t, 3, result.Spontaneous.KnownByLevel[1])
  	assert.Equal(t, 5, result.Spontaneous.UsesByLevel[1])
  }

  // REQ-JTG4: Fixed and pool are unioned.
  func TestMergeGrants_PreparedFixedAndPoolAreUnioned(t *testing.T) {
  	a := &ruleset.TechnologyGrants{
  		Prepared: &ruleset.PreparedGrants{
  			Fixed: []ruleset.PreparedEntry{{ID: "x", Level: 1}},
  			Pool:  []ruleset.PreparedEntry{{ID: "y", Level: 1}},
  		},
  	}
  	b := &ruleset.TechnologyGrants{
  		Prepared: &ruleset.PreparedGrants{
  			Fixed: []ruleset.PreparedEntry{{ID: "z", Level: 1}},
  			Pool:  []ruleset.PreparedEntry{{ID: "w", Level: 1}},
  		},
  	}
  	result := ruleset.MergeGrants(a, b)
  	require.NotNil(t, result.Prepared)
  	assert.Len(t, result.Prepared.Fixed, 2)
  	assert.Len(t, result.Prepared.Pool, 2)
  }

  // REQ-JTG5 (property): Merged hardwired length equals sum of both inputs.
  func TestPropertyMergeGrants_HardwiredLengthIsSum(t *testing.T) {
  	rapid.Check(t, func(rt *rapid.T) {
  		nA := rapid.IntRange(0, 5).Draw(rt, "nA")
  		nB := rapid.IntRange(0, 5).Draw(rt, "nB")
  		aHW := make([]string, nA)
  		for i := range aHW {
  			aHW[i] = fmt.Sprintf("tech_a_%d", i)
  		}
  		bHW := make([]string, nB)
  		for i := range bHW {
  			bHW[i] = fmt.Sprintf("tech_b_%d", i)
  		}
  		a := &ruleset.TechnologyGrants{Hardwired: aHW}
  		b := &ruleset.TechnologyGrants{Hardwired: bHW}
  		result := ruleset.MergeGrants(a, b)
  		if len(result.Hardwired) != nA+nB {
  			rt.Fatalf("expected %d hardwired, got %d", nA+nB, len(result.Hardwired))
  		}
  	})
  }

  // REQ-JTG8: MergeLevelUpGrants merges maps key-by-key; solo keys pass through.
  func TestMergeLevelUpGrants_KeysFromBothMaps(t *testing.T) {
  	a := map[int]*ruleset.TechnologyGrants{
  		2: {Hardwired: []string{"x"}},
  		3: {Hardwired: []string{"y"}},
  	}
  	b := map[int]*ruleset.TechnologyGrants{
  		3: {Hardwired: []string{"z"}},
  		4: {Hardwired: []string{"w"}},
  	}
  	result := ruleset.MergeLevelUpGrants(a, b)
  	require.NotNil(t, result)
  	assert.Contains(t, result, 2)
  	assert.Contains(t, result, 3)
  	assert.Contains(t, result, 4)
  	// Level 3 merged: y + z
  	assert.Len(t, result[3].Hardwired, 2)
  }

  func TestMergeLevelUpGrants_BothNil(t *testing.T) {
  	assert.Nil(t, ruleset.MergeLevelUpGrants(nil, nil))
  }
  ```

  **Note:** `fmt` is already imported in the test file. Add `"fmt"` if not already present.

- [ ] **T1-S2: Run tests to confirm failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... \
    -run "TestMergeGrants|TestMergeLevelUpGrants|TestPropertyMergeGrants" -v 2>&1 | tail -10
  ```
  Expected: compile error — `MergeGrants` undefined.

- [ ] **T1-S3: Implement MergeGrants and MergeLevelUpGrants**

  Append to `internal/game/ruleset/technology_grants.go`:

  ```go
  // MergeGrants combines archetype-level grants (slot progression) with job-level grants
  // (fixed techs, pool options, optional extra slots).
  //
  // Precondition: either or both arguments may be nil.
  // Postcondition: returned grant is the union of both; nil if both are nil.
  func MergeGrants(archetype, job *TechnologyGrants) *TechnologyGrants {
  	if archetype == nil && job == nil {
  		return nil
  	}
  	if archetype == nil {
  		return job
  	}
  	if job == nil {
  		return archetype
  	}
  	merged := &TechnologyGrants{}

  	// Hardwired: union
  	merged.Hardwired = append(append([]string(nil), archetype.Hardwired...), job.Hardwired...)

  	// Prepared: sum slots, union fixed and pool
  	if archetype.Prepared != nil || job.Prepared != nil {
  		merged.Prepared = mergePreparedGrants(archetype.Prepared, job.Prepared)
  	}

  	// Spontaneous: sum known/uses, union fixed and pool
  	if archetype.Spontaneous != nil || job.Spontaneous != nil {
  		merged.Spontaneous = mergeSpontaneousGrants(archetype.Spontaneous, job.Spontaneous)
  	}

  	return merged
  }

  func mergePreparedGrants(a, b *PreparedGrants) *PreparedGrants {
  	out := &PreparedGrants{SlotsByLevel: make(map[int]int)}
  	if a != nil {
  		for lvl, n := range a.SlotsByLevel {
  			out.SlotsByLevel[lvl] += n
  		}
  		out.Fixed = append(out.Fixed, a.Fixed...)
  		out.Pool = append(out.Pool, a.Pool...)
  	}
  	if b != nil {
  		for lvl, n := range b.SlotsByLevel {
  			out.SlotsByLevel[lvl] += n
  		}
  		out.Fixed = append(out.Fixed, b.Fixed...)
  		out.Pool = append(out.Pool, b.Pool...)
  	}
  	return out
  }

  func mergeSpontaneousGrants(a, b *SpontaneousGrants) *SpontaneousGrants {
  	out := &SpontaneousGrants{
  		KnownByLevel: make(map[int]int),
  		UsesByLevel:  make(map[int]int),
  	}
  	if a != nil {
  		for lvl, n := range a.KnownByLevel {
  			out.KnownByLevel[lvl] += n
  		}
  		for lvl, n := range a.UsesByLevel {
  			out.UsesByLevel[lvl] += n
  		}
  		out.Fixed = append(out.Fixed, a.Fixed...)
  		out.Pool = append(out.Pool, a.Pool...)
  	}
  	if b != nil {
  		for lvl, n := range b.KnownByLevel {
  			out.KnownByLevel[lvl] += n
  		}
  		for lvl, n := range b.UsesByLevel {
  			out.UsesByLevel[lvl] += n
  		}
  		out.Fixed = append(out.Fixed, b.Fixed...)
  		out.Pool = append(out.Pool, b.Pool...)
  	}
  	return out
  }

  // MergeLevelUpGrants merges two level-keyed grant maps key by key.
  //
  // Precondition: either or both arguments may be nil.
  // Postcondition: returned map contains all keys from both inputs;
  // keys present in both are merged via MergeGrants.
  func MergeLevelUpGrants(archetype, job map[int]*TechnologyGrants) map[int]*TechnologyGrants {
  	if len(archetype) == 0 && len(job) == 0 {
  		return nil
  	}
  	out := make(map[int]*TechnologyGrants)
  	for lvl, g := range archetype {
  		out[lvl] = g
  	}
  	for lvl, g := range job {
  		if existing, ok := out[lvl]; ok {
  			out[lvl] = MergeGrants(existing, g)
  		} else {
  			out[lvl] = g
  		}
  	}
  	return out
  }
  ```

- [ ] **T1-S4: Run tests to confirm pass**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... \
    -run "TestMergeGrants|TestMergeLevelUpGrants|TestPropertyMergeGrants" -v 2>&1 | tail -20
  ```
  Expected: all PASS.

- [ ] **T1-S5: Run full ruleset suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -count=1 2>&1 | tail -5
  ```
  Expected: PASS.

- [ ] **T1-S6: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    internal/game/ruleset/technology_grants.go \
    internal/game/ruleset/technology_grants_test.go
  git commit -m "feat(ruleset): MergeGrants and MergeLevelUpGrants for archetype+job tech fusion (REQ-JTG1–5, REQ-JTG8)"
  ```

---

## Chunk 2 — Task 2: Archetype struct + YAML content

### Task 2: Add TechnologyGrants to Archetype and populate archetype YAMLs

**Files:**
- Modify: `internal/game/ruleset/archetype.go`
- Modify or Create: `internal/game/ruleset/archetype_test.go`
- Modify: `content/archetypes/nerd.yaml`
- Modify: `content/archetypes/zealot.yaml`
- Modify: `content/archetypes/naturalist.yaml`
- Modify: `content/archetypes/schemer.yaml`
- Modify: `content/archetypes/influencer.yaml`
- Modify: `content/archetypes/drifter.yaml`

---

- [ ] **T2-S1: Check if archetype_test.go exists**

  ```bash
  ls /home/cjohannsen/src/mud/internal/game/ruleset/archetype_test.go 2>/dev/null && echo EXISTS || echo MISSING
  ```

- [ ] **T2-S2: Write failing tests for archetype tech grant round-trip**

  If `archetype_test.go` exists, append to it. Otherwise create it as `package ruleset_test`.

  ```go
  // TestArchetype_TechnologyGrants_Prepared_RoundTrip verifies that an archetype YAML
  // with prepared technology_grants parses correctly.
  func TestArchetype_TechnologyGrants_Prepared_RoundTrip(t *testing.T) {
  	src := `
  id: test_arch
  name: Test
  key_ability: reasoning
  hit_points_per_level: 6
  ability_boosts:
    fixed: [reasoning]
    free: 1
  technology_grants:
    prepared:
      slots_by_level:
        1: 2
  level_up_grants:
    2:
      prepared:
        slots_by_level:
          1: 1
  `
  	var a ruleset.Archetype
  	require.NoError(t, yaml.Unmarshal([]byte(src), &a))
  	require.NotNil(t, a.TechnologyGrants)
  	require.NotNil(t, a.TechnologyGrants.Prepared)
  	assert.Equal(t, 2, a.TechnologyGrants.Prepared.SlotsByLevel[1])
  	require.NotNil(t, a.LevelUpGrants)
  	assert.Equal(t, 1, a.LevelUpGrants[2].Prepared.SlotsByLevel[1])
  }

  // TestArchetype_TechnologyGrants_Spontaneous_RoundTrip verifies spontaneous grants parse.
  func TestArchetype_TechnologyGrants_Spontaneous_RoundTrip(t *testing.T) {
  	src := `
  id: test_arch_spont
  name: Test Spont
  key_ability: flair
  hit_points_per_level: 8
  ability_boosts:
    fixed: [flair]
    free: 1
  technology_grants:
    spontaneous:
      known_by_level:
        1: 2
      uses_by_level:
        1: 2
  `
  	var a ruleset.Archetype
  	require.NoError(t, yaml.Unmarshal([]byte(src), &a))
  	require.NotNil(t, a.TechnologyGrants)
  	require.NotNil(t, a.TechnologyGrants.Spontaneous)
  	assert.Equal(t, 2, a.TechnologyGrants.Spontaneous.KnownByLevel[1])
  	assert.Equal(t, 2, a.TechnologyGrants.Spontaneous.UsesByLevel[1])
  }

  // TestArchetype_NoTechGrants_IsValid verifies non-tech archetypes (aggressor, criminal) load fine.
  func TestArchetype_NoTechGrants_IsValid(t *testing.T) {
  	src := `
  id: fighter
  name: Fighter
  key_ability: brutality
  hit_points_per_level: 10
  ability_boosts:
    fixed: [brutality]
    free: 2
  `
  	var a ruleset.Archetype
  	require.NoError(t, yaml.Unmarshal([]byte(src), &a))
  	assert.Nil(t, a.TechnologyGrants)
  	assert.Nil(t, a.LevelUpGrants)
  }
  ```

  **Imports needed** (add at top of file if not present):
  ```go
  import (
      "testing"
      "github.com/cory-johannsen/mud/internal/game/ruleset"
      "github.com/stretchr/testify/assert"
      "github.com/stretchr/testify/require"
      "gopkg.in/yaml.v3"
  )
  ```

- [ ] **T2-S3: Run tests to confirm failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... \
    -run "TestArchetype_TechnologyGrants|TestArchetype_No" -v 2>&1 | tail -10
  ```
  Expected: compile error or FAIL — `Archetype.TechnologyGrants` field undefined.

- [ ] **T2-S4: Add TechnologyGrants and LevelUpGrants to Archetype struct**

  In `internal/game/ruleset/archetype.go`, update the `Archetype` struct:

  ```go
  type Archetype struct {
  	ID                 string                    `yaml:"id"`
  	Name               string                    `yaml:"name"`
  	Description        string                    `yaml:"description"`
  	KeyAbility         string                    `yaml:"key_ability"`
  	HitPointsPerLevel  int                       `yaml:"hit_points_per_level"`
  	AbilityBoosts      *AbilityBoostGrant        `yaml:"ability_boosts"`
  	InnateTechnologies []InnateGrant             `yaml:"innate_technologies,omitempty"`
  	TechnologyGrants   *TechnologyGrants         `yaml:"technology_grants,omitempty"`
  	LevelUpGrants      map[int]*TechnologyGrants `yaml:"level_up_grants,omitempty"`
  }
  ```

  Note: `LoadArchetypes` requires NO changes — it already uses `yaml.Unmarshal` which handles new fields automatically. It does NOT call `Validate()` on archetype grants.

- [ ] **T2-S5: Run archetype tests to confirm pass**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... \
    -run "TestArchetype_TechnologyGrants|TestArchetype_No" -v 2>&1 | tail -10
  ```
  Expected: PASS.

- [ ] **T2-S6: Populate nerd archetype YAML**

  Append to `content/archetypes/nerd.yaml` (after the existing fields):

  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 2
  level_up_grants:
    2:
      prepared:
        slots_by_level:
          1: 1
    3:
      prepared:
        slots_by_level:
          1: 1
    4:
      prepared:
        slots_by_level:
          1: 1
    5:
      prepared:
        slots_by_level:
          1: 1
  ```

- [ ] **T2-S7: Populate zealot archetype YAML**

  Append to `content/archetypes/zealot.yaml`:

  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 2
  level_up_grants:
    2:
      prepared:
        slots_by_level:
          1: 1
    3:
      prepared:
        slots_by_level:
          1: 1
    4:
      prepared:
        slots_by_level:
          1: 1
    5:
      prepared:
        slots_by_level:
          1: 1
  ```

- [ ] **T2-S8: Populate naturalist archetype YAML**

  Append to `content/archetypes/naturalist.yaml`:

  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 2
  level_up_grants:
    2:
      prepared:
        slots_by_level:
          1: 1
    3:
      prepared:
        slots_by_level:
          1: 1
    4:
      prepared:
        slots_by_level:
          1: 1
    5:
      prepared:
        slots_by_level:
          1: 1
  ```

- [ ] **T2-S9: Populate schemer archetype YAML**

  Append to `content/archetypes/schemer.yaml`:

  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 2
  level_up_grants:
    2:
      prepared:
        slots_by_level:
          1: 1
    3:
      prepared:
        slots_by_level:
          1: 1
    4:
      prepared:
        slots_by_level:
          1: 1
    5:
      prepared:
        slots_by_level:
          1: 1
  ```

- [ ] **T2-S10: Populate influencer archetype YAML**

  Append to `content/archetypes/influencer.yaml`:

  ```yaml
  technology_grants:
    spontaneous:
      known_by_level:
        1: 2
      uses_by_level:
        1: 2
  level_up_grants:
    2:
      spontaneous:
        uses_by_level:
          1: 1
    3:
      spontaneous:
        uses_by_level:
          1: 1
    4:
      spontaneous:
        uses_by_level:
          1: 1
    5:
      spontaneous:
        uses_by_level:
          1: 1
  ```

- [ ] **T2-S11: Populate drifter archetype YAML**

  Append to `content/archetypes/drifter.yaml`:

  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 1
  level_up_grants:
    3:
      prepared:
        slots_by_level:
          1: 1
    5:
      prepared:
        slots_by_level:
          1: 1
  ```

- [ ] **T2-S12: Verify archetypes load**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... -count=1 2>&1 | tail -5
  ```
  Expected: PASS.

- [ ] **T2-S13: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    internal/game/ruleset/archetype.go \
    internal/game/ruleset/archetype_test.go \
    content/archetypes/nerd.yaml \
    content/archetypes/zealot.yaml \
    content/archetypes/naturalist.yaml \
    content/archetypes/schemer.yaml \
    content/archetypes/influencer.yaml \
    content/archetypes/drifter.yaml
  git commit -m "feat(ruleset,content): Archetype.TechnologyGrants field; slot progression for 6 archetypes"
  ```

---

## Chunk 3 — Task 3: AssignTechnologies merge

### Task 3: Update AssignTechnologies to merge archetype + job grants

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/technology_assignment_test.go`

---

- [ ] **T3-S1: Write failing tests**

  Append to `internal/gameserver/technology_assignment_test.go`:

  ```go
  // REQ-JTG6: AssignTechnologies returns a wrapped error when merged grants fail Validate().
  // REQ-JTG7: AssignTechnologies calls Validate() on the merged result before processing
  //           (also exercised by TestAssignTechnologies_ArchetypeSlots_JobPool_Merged on the success path).
  func TestAssignTechnologies_MergedGrantsValidationError(t *testing.T) {
  	ctx := context.Background()
  	sess := &session.PlayerSession{}
  	// Archetype provides 3 slots but no pool.
  	arch := &ruleset.Archetype{
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				SlotsByLevel: map[int]int{1: 3},
  			},
  		},
  	}
  	// Job provides only 1 pool entry — merged: 3 slots, 1 pool → invalid.
  	job := &ruleset.Job{
  		ID: "test_job",
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				Pool: []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
  			},
  		},
  	}
  	hw := &fakeHardwiredRepo{}
  	prep := &fakePreparedRepo{}
  	spont := &fakeSpontaneousRepo{}
  	inn := &fakeInnateRepo{}

  	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn)
  	require.Error(t, err)
  	assert.Contains(t, err.Error(), "invalid merged grants")
  }

  // REQ-JTG3 integration: AssignTechnologies uses merged slot count (archetype + job).
  func TestAssignTechnologies_ArchetypeSlots_JobPool_Merged(t *testing.T) {
  	ctx := context.Background()
  	sess := &session.PlayerSession{}
  	// Archetype: 2 prepared slots at level 1.
  	arch := &ruleset.Archetype{
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				SlotsByLevel: map[int]int{1: 2},
  			},
  		},
  	}
  	// Job: 2 pool entries to satisfy 2 slots.
  	job := &ruleset.Job{
  		ID: "test_nerd_job",
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				Pool: []ruleset.PreparedEntry{
  					{ID: "neural_shock", Level: 1},
  					{ID: "mind_spike", Level: 1},
  				},
  			},
  		},
  	}
  	hw := &fakeHardwiredRepo{}
  	prep := &fakePreparedRepo{}
  	spont := &fakeSpontaneousRepo{}
  	inn := &fakeInnateRepo{}

  	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn)
  	require.NoError(t, err)
  	// 2 slots filled from pool (auto-assign since pool size == slots).
  	assert.Len(t, sess.PreparedTechs[1], 2)
  }

  // TestAssignTechnologies_NilJobTechGrants_ArchetypeGrantsUsed verifies that when
  // job.TechnologyGrants is nil but archetype.TechnologyGrants is non-nil, grants are processed.
  func TestAssignTechnologies_NilJobTechGrants_ArchetypeGrantsUsed(t *testing.T) {
  	ctx := context.Background()
  	sess := &session.PlayerSession{}
  	// Archetype: 1 slot, 1 pool entry (valid merged grant).
  	arch := &ruleset.Archetype{
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				SlotsByLevel: map[int]int{1: 1},
  				Pool:         []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
  			},
  		},
  	}
  	// Job has no TechnologyGrants.
  	job := &ruleset.Job{ID: "no_grants_job"}
  	hw := &fakeHardwiredRepo{}
  	prep := &fakePreparedRepo{}
  	spont := &fakeSpontaneousRepo{}
  	inn := &fakeInnateRepo{}

  	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn)
  	require.NoError(t, err)
  	assert.Len(t, sess.PreparedTechs[1], 1)
  }
  ```

- [ ] **T3-S2: Run tests to confirm failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... \
    -run "TestAssignTechnologies_MergedGrants|TestAssignTechnologies_Archetype|TestAssignTechnologies_NilJob" \
    -v 2>&1 | tail -10
  ```
  Expected: FAIL — merged grants behavior not yet implemented.

- [ ] **T3-S3: Update AssignTechnologies**

  In `internal/gameserver/technology_assignment.go`, replace the preamble of `AssignTechnologies` (the `if job == nil || job.TechnologyGrants == nil` guard through `grants := job.TechnologyGrants`) with:

  ```go
  func AssignTechnologies(
  	ctx context.Context,
  	sess *session.PlayerSession,
  	characterID int64,
  	job *ruleset.Job,
  	archetype *ruleset.Archetype,
  	techReg *technology.Registry,
  	promptFn TechPromptFn,
  	hwRepo HardwiredTechRepo,
  	prepRepo PreparedTechRepo,
  	spontRepo SpontaneousTechRepo,
  	innateRepo InnateTechRepo,
  ) error {
  	if job == nil {
  		return nil
  	}

  	var archetypeGrants *ruleset.TechnologyGrants
  	if archetype != nil {
  		archetypeGrants = archetype.TechnologyGrants
  	}
  	grants := ruleset.MergeGrants(archetypeGrants, job.TechnologyGrants)

  	// Validate merged grants before processing.
  	if grants != nil {
  		if err := grants.Validate(); err != nil {
  			return fmt.Errorf("AssignTechnologies: invalid merged grants for job %s: %w", job.ID, err)
  		}
  	}
  ```

  The rest of the function (hardwired, innate, prepared, spontaneous blocks) continues unchanged, using `grants` (which is now the merged result).

  Update the function's doc comment postcondition:
  ```
  // Postcondition: If both archetype.TechnologyGrants and job.TechnologyGrants are nil,
  // all session tech fields remain nil (innate assignment still proceeds).
  ```

- [ ] **T3-S4: Run new tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... \
    -run "TestAssignTechnologies_MergedGrants|TestAssignTechnologies_Archetype|TestAssignTechnologies_NilJob" \
    -v 2>&1 | tail -15
  ```
  Expected: all PASS.

- [ ] **T3-S5: Run full gameserver test suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -5
  ```
  Expected: PASS.

- [ ] **T3-S6: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    internal/gameserver/technology_assignment.go \
    internal/gameserver/technology_assignment_test.go
  git commit -m "feat(gameserver): AssignTechnologies merges archetype+job grants; validates before processing (REQ-JTG6, REQ-JTG7)"
  ```

---

## Chunk 4 — Task 4: Job YAML content + FEATURES.md

### Task 4: Populate technology_grants in all 55 tech-using job YAMLs

**Files:**
- Modify: 9 nerd job YAMLs
- Modify: 10 zealot job YAMLs
- Modify: 8 naturalist job YAMLs
- Modify: 8 schemer job YAMLs
- Modify: 10 influencer job YAMLs
- Modify: 10 drifter job YAMLs
- Modify: `docs/requirements/FEATURES.md`

**Tradition → tech ID mapping:**
- Technical (nerd): `neural_shock`
- Fanatic Doctrine (zealot): `battle_fervor`
- Bio-Synthetic (naturalist, drifter): `acid_spray`
- Neural (influencer, schemer): `mind_spike`

---

- [ ] **T4-S1: Populate nerd jobs (8 standard + 1 extra-slot)**

  For the following 8 nerd jobs, append exactly this block at the end of each YAML file:
  `cooker`, `detective`, `grease_monkey`, `hoarder`, `journalist`, `narc`, `natural_mystic`, `specialist`

  The nerd archetype contributes 2 prepared slots at level 1. Each job's pool must have ≥ 2 entries to satisfy validation.

  ```yaml
  technology_grants:
    prepared:
      pool:
        - id: neural_shock
          level: 1
        - id: neural_shock
          level: 1
  ```

  For `engineer` only (1 extra slot; merged total: 3 slots → job must supply 3 pool entries):
  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 1
      pool:
        - id: neural_shock
          level: 1
        - id: neural_shock
          level: 1
        - id: neural_shock
          level: 1
  ```

  Merged result: 3 slots, 3 pool entries → valid.

- [ ] **T4-S2: Populate zealot jobs (9 standard + 1 extra-slot)**

  For the following 9 zealot jobs, append this block:
  `believer`, `cult_leader`, `follower`, `guard`, `hired_help`, `pastor`, `street_preacher`, `trainee`, `vigilante`

  ```yaml
  technology_grants:
    prepared:
      pool:
        - id: battle_fervor
          level: 1
        - id: battle_fervor
          level: 1
  ```

  For `medic` (extra slot):
  ```yaml
  technology_grants:
    prepared:
      slots_by_level:
        1: 1
      pool:
        - id: battle_fervor
          level: 1
        - id: battle_fervor
          level: 1
        - id: battle_fervor
          level: 1
  ```

  Note: Each job's pool must have ≥ archetype slots (2 for zealot base). Standard jobs need 2 pool entries. Medic (extra slot) needs 3.

- [ ] **T4-S3: Populate naturalist jobs (8 standard)**

  For all 8 naturalist jobs: `exterminator`, `fallen_trustafarian`, `freegan`, `hippie`, `hobo`, `laborer`, `rancher`, `tracker`

  ```yaml
  technology_grants:
    prepared:
      pool:
        - id: acid_spray
          level: 1
        - id: acid_spray
          level: 1
  ```

- [ ] **T4-S4: Populate schemer jobs (8 standard)**

  For all 8 schemer jobs: `dealer`, `grifter`, `illusionist`, `maker`, `mall_ninja`, `narcomancer`, `salesman`, `shit_stirrer`

  ```yaml
  technology_grants:
    prepared:
      pool:
        - id: mind_spike
          level: 1
        - id: mind_spike
          level: 1
  ```

- [ ] **T4-S5: Populate influencer jobs (10 standard)**

  For all 10 influencer jobs: `anarchist`, `antifa`, `bureaucrat`, `entertainer`, `exotic_dancer`, `extortionist`, `karen`, `libertarian`, `politician`, `schmoozer`

  ```yaml
  technology_grants:
    spontaneous:
      pool:
        - id: mind_spike
          level: 1
        - id: mind_spike
          level: 1
  ```

  Note: Influencer archetype provides `known_by_level: {1: 2}`. Job pool must have ≥ 2 entries.

- [ ] **T4-S6: Populate drifter jobs (10 standard)**

  For all 10 drifter jobs: `bagman`, `cop`, `driver`, `free_spirit`, `pilot`, `pirate`, `psychopath`, `scout`, `stalker`, `warden`

  ```yaml
  technology_grants:
    prepared:
      pool:
        - id: acid_spray
          level: 1
  ```

  Note: Drifter archetype provides 1 slot. Job pool needs ≥ 1 entry.

- [ ] **T4-S7: Build to verify all YAMLs are valid Go structs**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -10
  ```
  Expected: BUILD OK (YAML is not validated at build time, but code must compile).

- [ ] **T4-S8: Write an integration test verifying all content jobs load and validate**

  Append to `internal/game/ruleset/technology_grants_test.go`:

  ```go
  // TestAllTechJobsLoadAndMergeValid verifies that all job YAMLs with technology_grants,
  // when merged with their archetype's TechnologyGrants, produce valid grants.
  func TestAllTechJobsLoadAndMergeValid(t *testing.T) {
  	archetypes, err := ruleset.LoadArchetypes("../../../content/archetypes")
  	require.NoError(t, err)
  	archetypeMap := make(map[string]*ruleset.Archetype)
  	for _, a := range archetypes {
  		archetypeMap[a.ID] = a
  	}

  	jobs, err := ruleset.LoadJobs("../../../content/jobs")
  	require.NoError(t, err)

  	for _, job := range jobs {
  		if job.TechnologyGrants == nil {
  			continue // non-tech jobs: skip
  		}
  		arch, ok := archetypeMap[job.Archetype]
  		if !ok {
  			t.Errorf("job %s: archetype %q not found", job.ID, job.Archetype)
  			continue
  		}
  		var archetypeGrants *ruleset.TechnologyGrants
  		if arch != nil {
  			archetypeGrants = arch.TechnologyGrants
  		}
  		merged := ruleset.MergeGrants(archetypeGrants, job.TechnologyGrants)
  		if merged != nil {
  			if err := merged.Validate(); err != nil {
  				t.Errorf("job %s: merged grants invalid: %v", job.ID, err)
  			}
  		}
  	}
  }
  ```

  Check what `job.Archetype` field is called — look at the Job struct definition:
  ```bash
  grep -n "Archetype\|archetype" /home/cjohannsen/src/mud/internal/game/ruleset/job.go | head -5
  ```
  Adjust field name if needed.

- [ ] **T4-S9: Run integration test**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... \
    -run "TestAllTechJobsLoadAndMergeValid" -v 2>&1 | tail -20
  ```
  Expected: PASS. If any job fails validation, the test output will name it — fix that job's pool count.

- [ ] **T4-S10: Run full test suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | grep -E "FAIL|^ok"
  ```
  Expected: all packages pass.

- [ ] **T4-S11: Update FEATURES.md**

  In `docs/requirements/FEATURES.md`, change:
  ```
  - [ ] Fixed list of Technologies per job level, increases with level (higher level Technology slots and higher Job level)
  ```
  to:
  ```
  - [x] Fixed list of Technologies per job level, increases with level (higher level Technology slots and higher Job level) — Phase 1: slot progression on archetypes, pool entries on jobs; Phase 2 will expand tech library
  ```

- [ ] **T4-S12: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    content/jobs/*.yaml \
    internal/game/ruleset/technology_grants_test.go \
    docs/requirements/FEATURES.md
  git commit -m "feat(content): technology_grants populated for all 55 tech-using jobs (Phase 1)"
  ```

---

## Requirements Checklist

| REQ | Task | Description |
|---|---|---|
| REQ-JTG1 | Task 1 | `MergeGrants(nil, nil)` returns nil |
| REQ-JTG2 | Task 1 | `MergeGrants` with one nil returns the other unchanged |
| REQ-JTG3 | Task 1 + Task 3 | Merged slot counts equal sum; integration test in T3 |
| REQ-JTG4 | Task 1 | Fixed and pool are unioned |
| REQ-JTG5 | Task 1 | Property: merged hardwired length = sum of both |
| REQ-JTG6 | Task 3 | `AssignTechnologies` returns error on invalid merged grants |
| REQ-JTG7 | Task 3 | `AssignTechnologies` calls `Validate()` on merged result |
| REQ-JTG8 | Task 1 | `MergeLevelUpGrants` merges maps key-by-key |
