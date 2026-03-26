package ruleset_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// REQ-TG2: Job with technology_grants.hardwired YAML round-trips
func TestJob_TechnologyGrants_Hardwired_RoundTrip(t *testing.T) {
	src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
  hardwired:
    - neural_shock
    - mind_spike
`
	var j ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(src), &j))
	require.NotNil(t, j.TechnologyGrants)
	assert.Equal(t, []string{"neural_shock", "mind_spike"}, j.TechnologyGrants.Hardwired)
}

// REQ-TG3: Job with technology_grants.prepared YAML round-trips
func TestJob_TechnologyGrants_Prepared_RoundTrip(t *testing.T) {
	src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
  prepared:
    slots_by_level:
      1: 2
    fixed:
      - id: neural_shock
        level: 1
    pool:
      - id: mind_spike
        level: 1
      - id: arc_thought
        level: 1
`
	var j ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(src), &j))
	require.NotNil(t, j.TechnologyGrants)
	require.NotNil(t, j.TechnologyGrants.Prepared)
	assert.Equal(t, 2, j.TechnologyGrants.Prepared.SlotsByLevel[1])
	assert.Equal(t, "neural_shock", j.TechnologyGrants.Prepared.Fixed[0].ID)
	assert.Equal(t, 1, j.TechnologyGrants.Prepared.Fixed[0].Level)
	assert.Len(t, j.TechnologyGrants.Prepared.Pool, 2)
}

// REQ-TG4: Job with technology_grants.spontaneous YAML round-trips
func TestJob_TechnologyGrants_Spontaneous_RoundTrip(t *testing.T) {
	src := `
id: test_job
name: Test Job
archetype: aggressor
technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 4
    fixed:
      - id: battle_fervor
        level: 1
    pool:
      - id: acid_spray
        level: 1
      - id: neural_shock
        level: 1
`
	var j ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(src), &j))
	require.NotNil(t, j.TechnologyGrants)
	require.NotNil(t, j.TechnologyGrants.Spontaneous)
	assert.Equal(t, 2, j.TechnologyGrants.Spontaneous.KnownByLevel[1])
	assert.Equal(t, 4, j.TechnologyGrants.Spontaneous.UsesByLevel[1])
	assert.Equal(t, "battle_fervor", j.TechnologyGrants.Spontaneous.Fixed[0].ID)
	assert.Len(t, j.TechnologyGrants.Spontaneous.Pool, 2)
}

// REQ-TG5: Archetype with innate_technologies YAML round-trips
func TestArchetype_InnateTechnologies_RoundTrip(t *testing.T) {
	src := `
id: test_archetype
name: Test Archetype
innate_technologies:
  - id: acid_spray
    uses_per_day: 1
  - id: neural_shock
    uses_per_day: 0
`
	var a ruleset.Archetype
	require.NoError(t, yaml.Unmarshal([]byte(src), &a))
	require.Len(t, a.InnateTechnologies, 2)
	assert.Equal(t, "acid_spray", a.InnateTechnologies[0].ID)
	assert.Equal(t, 1, a.InnateTechnologies[0].UsesPerDay)
	assert.Equal(t, "neural_shock", a.InnateTechnologies[1].ID)
	assert.Equal(t, 0, a.InnateTechnologies[1].UsesPerDay)
}

// REQ-TG12: Validate returns error when pool+fixed < slots at any level
func TestTechnologyGrants_Validate_PoolTooSmall(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 3},
			Fixed:        []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
			Pool:         []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
			// 1 fixed + 1 pool = 2, but 3 slots required
		},
	}
	err := grants.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "level 1")
}

func TestTechnologyGrants_Validate_SufficientPool(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 2},
			Fixed:        []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
			Pool:         []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
		},
	}
	require.NoError(t, grants.Validate())
}

func TestTechnologyGrants_Validate_Spontaneous_PoolTooSmall(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{1: 3},
			UsesByLevel:  map[int]int{1: 4},
			Fixed:        []ruleset.SpontaneousEntry{{ID: "battle_fervor", Level: 1}},
			Pool:         []ruleset.SpontaneousEntry{{ID: "acid_spray", Level: 1}},
		},
	}
	err := grants.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "level 1")
}

// REQ-TG11 (property): TechnologyGrants YAML round-trip preserves all fields
func TestProperty_TechnologyGrants_YAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numFixed := rapid.IntRange(0, 3).Draw(rt, "numFixed")
		numPool := rapid.IntRange(0, 3).Draw(rt, "numPool")
		slots := numFixed + numPool
		if slots == 0 {
			slots = 1
			numPool = 1
		}

		prepFixed := make([]ruleset.PreparedEntry, numFixed)
		for i := range prepFixed {
			prepFixed[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("tech_%d", i), Level: 1}
		}
		prepPool := make([]ruleset.PreparedEntry, numPool)
		for i := range prepPool {
			prepPool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("pool_%d", i), Level: 1}
		}

		numSpontFixed := rapid.IntRange(0, 3).Draw(rt, "numSpontFixed")
		numSpontPool := rapid.IntRange(0, 3).Draw(rt, "numSpontPool")
		known := numSpontFixed + numSpontPool
		if known == 0 {
			known = 1
			numSpontPool = 1
		}
		spontFixed := make([]ruleset.SpontaneousEntry, numSpontFixed)
		for i := range spontFixed {
			spontFixed[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("spont_tech_%d", i), Level: 1}
		}
		spontPool := make([]ruleset.SpontaneousEntry, numSpontPool)
		for i := range spontPool {
			spontPool[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("spont_pool_%d", i), Level: 1}
		}

		g := &ruleset.TechnologyGrants{
			Hardwired: []string{"hw_tech"},
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: slots},
				Fixed:        prepFixed,
				Pool:         prepPool,
			},
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: known},
				UsesByLevel:  map[int]int{1: 4},
				Fixed:        spontFixed,
				Pool:         spontPool,
			},
		}
		data, err := yaml.Marshal(g)
		require.NoError(rt, err)
		var got ruleset.TechnologyGrants
		require.NoError(rt, yaml.Unmarshal(data, &got))
		assert.Equal(rt, g.Hardwired, got.Hardwired)
		require.NotNil(rt, got.Prepared)
		assert.Equal(rt, g.Prepared.SlotsByLevel, got.Prepared.SlotsByLevel)
		assert.ElementsMatch(rt, g.Prepared.Fixed, got.Prepared.Fixed)
		assert.ElementsMatch(rt, g.Prepared.Pool, got.Prepared.Pool)
		require.NotNil(rt, got.Spontaneous)
		assert.Equal(rt, g.Spontaneous.KnownByLevel, got.Spontaneous.KnownByLevel)
		assert.Equal(rt, g.Spontaneous.UsesByLevel, got.Spontaneous.UsesByLevel)
		assert.ElementsMatch(rt, g.Spontaneous.Fixed, got.Spontaneous.Fixed)
		assert.ElementsMatch(rt, g.Spontaneous.Pool, got.Spontaneous.Pool)
	})
}

// REQ-LUT1: Job with level_up_grants YAML round-trips without data loss
func TestJob_LevelUpGrants_RoundTrip(t *testing.T) {
	src := `
id: test_job
name: Test Job
archetype: aggressor
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool:
        - id: arc_thought
          level: 2
        - id: mind_spike
          level: 2
  5:
    spontaneous:
      known_by_level:
        2: 1
      pool:
        - id: acid_spray
          level: 2
`
	var job ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(src), &job))

	out, err := yaml.Marshal(&job)
	require.NoError(t, err)

	var job2 ruleset.Job
	require.NoError(t, yaml.Unmarshal(out, &job2))

	require.NotNil(t, job2.LevelUpGrants)
	require.Contains(t, job2.LevelUpGrants, 3)
	require.Contains(t, job2.LevelUpGrants, 5)
	assert.NotNil(t, job2.LevelUpGrants[3].Prepared)
	assert.Equal(t, 1, job2.LevelUpGrants[3].Prepared.SlotsByLevel[2])
	assert.Len(t, job2.LevelUpGrants[3].Prepared.Pool, 2)
	assert.NotNil(t, job2.LevelUpGrants[5].Spontaneous)
	assert.Equal(t, 1, job2.LevelUpGrants[5].Spontaneous.KnownByLevel[2])
}

// REQ-LUT8 (property): For any valid level_up_grants map, YAML marshal/unmarshal preserves all fields
func TestProperty_LevelUpGrants_YAMLRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		grants := make(map[int]*ruleset.TechnologyGrants, n)
		for i := 0; i < n; i++ {
			charLevel := rapid.IntRange(1, 20).Draw(rt, fmt.Sprintf("level%d", i))
			numHW := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("nhw%d", i))
			hw := make([]string, numHW)
			for j := 0; j < numHW; j++ {
				hw[j] = rapid.StringMatching(`[a-z_]{1,10}`).Draw(rt, fmt.Sprintf("hw%d_%d", i, j))
			}
			grants[charLevel] = &ruleset.TechnologyGrants{Hardwired: hw}
		}
		job := &ruleset.Job{
			ID:            "prop_job",
			Name:          "Prop Job",
			LevelUpGrants: grants,
		}

		out, err := yaml.Marshal(job)
		if err != nil {
			rt.Fatalf("marshal: %v", err)
		}
		var job2 ruleset.Job
		if err := yaml.Unmarshal(out, &job2); err != nil {
			rt.Fatalf("unmarshal: %v", err)
		}
		for lvl, g := range grants {
			g2, ok := job2.LevelUpGrants[lvl]
			if !ok {
				rt.Fatalf("missing level %d after round-trip", lvl)
			}
			if len(g.Hardwired) == 0 {
				assert.Empty(rt, g2.Hardwired, "hardwired mismatch at level %d", lvl)
			} else {
				assert.Equal(rt, g.Hardwired, g2.Hardwired, "hardwired mismatch at level %d", lvl)
			}
		}
	})
}

// REQ-LUT2, REQ-LUT9: LoadJobs rejects a YAML file with an invalid level_up_grants entry
// (pool + fixed < slots_by_level); error includes job ID and the failing character level.
func TestLoadJobs_RejectsInvalidLevelUpGrants(t *testing.T) {
	// pool + fixed < slots_by_level for level 2 (0 pool + 0 fixed < 1 slot required)
	src := `
id: bad_job
name: Bad Job
archetype: aggressor
tier: 1
level_up_grants:
  3:
    prepared:
      slots_by_level:
        2: 1
      pool: []
`
	tmpDir := t.TempDir()
	jobFile := filepath.Join(tmpDir, "jobs.yaml")
	require.NoError(t, os.WriteFile(jobFile, []byte(src), 0644))

	_, err := ruleset.LoadJobs(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad_job")
	assert.Contains(t, err.Error(), "3")
}

// REQ-TG12 (load-time): LoadJobs returns error when technology_grants pool is insufficient
func TestLoadJobs_RejectsInvalidTechnologyGrants(t *testing.T) {
	dir := t.TempDir()
	content := `
id: bad_job
name: Bad Job
archetype: aggressor
tier: 1
technology_grants:
  prepared:
    slots_by_level:
      1: 3
    fixed:
      - id: neural_shock
        level: 1
    pool:
      - id: mind_spike
        level: 1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad_job.yaml"), []byte(content), 0644))
	_, err := ruleset.LoadJobs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "level 1")
}

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
		assert.Equal(rt, nA+nB, len(result.Hardwired))
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
	// Level 2 is solo from a; verify content passed through unchanged.
	assert.Equal(t, []string{"x"}, result[2].Hardwired)
	// Level 4 is solo from b; verify content passed through unchanged.
	assert.Equal(t, []string{"w"}, result[4].Hardwired)
}

func TestMergeLevelUpGrants_BothNil(t *testing.T) {
	assert.Nil(t, ruleset.MergeLevelUpGrants(nil, nil))
}

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
