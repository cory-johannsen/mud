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

// REQ-TG12 (load-time): LoadJobs returns error when technology_grants pool is insufficient
func TestLoadJobs_RejectsInvalidTechnologyGrants(t *testing.T) {
	dir := t.TempDir()
	content := `
id: bad_job
name: Bad Job
archetype: aggressor
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
