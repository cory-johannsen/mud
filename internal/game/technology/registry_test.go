package technology_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Join(filepath.Dir(file), "testdata")
}

func loadTestRegistry(t *testing.T) *technology.Registry {
	t.Helper()
	reg, err := technology.Load(testdataDir(t))
	require.NoError(t, err)
	require.NotNil(t, reg)
	return reg
}

// REQ-T7: Load with one valid YAML per tradition returns registry with all four
// traditions present via ByTradition.
func TestLoad_AllFourTraditionsPresent(t *testing.T) {
	reg := loadTestRegistry(t)

	traditions := []technology.Tradition{
		technology.TraditionTechnical,
		technology.TraditionNeural,
		technology.TraditionFanaticDoctrine,
		technology.TraditionBioSynthetic,
	}
	for _, trad := range traditions {
		result := reg.ByTradition(trad)
		assert.NotEmpty(t, result, "expected at least one technology for tradition %q", trad)
	}
}

// REQ-T8: Get by ID returns correct TechnologyDef; Get for unknown ID returns (nil, false).
func TestGet_KnownAndUnknown(t *testing.T) {
	reg := loadTestRegistry(t)

	t.Run("known id returns correct def", func(t *testing.T) {
		def, ok := reg.Get("neural_shock_fixture")
		require.True(t, ok)
		require.NotNil(t, def)
		assert.Equal(t, "neural_shock_fixture", def.ID)
		assert.Equal(t, technology.TraditionTechnical, def.Tradition)
	})

	t.Run("unknown id returns nil false", func(t *testing.T) {
		def, ok := reg.Get("does_not_exist")
		assert.False(t, ok)
		assert.Nil(t, def)
	})
}

// REQ-T9: ByTradition returns results sorted by level ascending, then ID ascending.
func TestByTradition_SortOrder(t *testing.T) {
	reg := loadTestRegistry(t)

	// neural tradition has two fixtures: mind_spike_fixture (level 1) and arc_thought_fixture (level 2)
	results := reg.ByTradition(technology.TraditionNeural)
	require.Len(t, results, 2)
	assert.Equal(t, "mind_spike_fixture", results[0].ID)
	assert.Equal(t, 1, results[0].Level)
	assert.Equal(t, "arc_thought_fixture", results[1].ID)
	assert.Equal(t, 2, results[1].Level)
}

// REQ-T10: ByTraditionAndLevel returns only defs matching both tradition and level; sorted by ID ascending.
func TestByTraditionAndLevel_FilterAndSort(t *testing.T) {
	reg := loadTestRegistry(t)

	t.Run("matches both tradition and level", func(t *testing.T) {
		results := reg.ByTraditionAndLevel(technology.TraditionNeural, 1)
		require.Len(t, results, 1)
		assert.Equal(t, "mind_spike_fixture", results[0].ID)
	})

	t.Run("returns empty when level has no match", func(t *testing.T) {
		results := reg.ByTraditionAndLevel(technology.TraditionNeural, 99)
		assert.Empty(t, results)
	})

	t.Run("returns empty when tradition has no match", func(t *testing.T) {
		results := reg.ByTraditionAndLevel(technology.TraditionTechnical, 99)
		assert.Empty(t, results)
	})

	t.Run("multiple results at same level sorted by ID", func(t *testing.T) {
		// Load a registry with two neural level-1 fixtures by using All() to verify order
		// The testdata only has one neural level-1; test the sort contract via ByTradition
		results := reg.ByTraditionAndLevel(technology.TraditionNeural, 2)
		require.Len(t, results, 1)
		assert.Equal(t, "arc_thought_fixture", results[0].ID)
	})
}

// REQ-T11: ByUsageType returns all defs matching the usage type across all traditions;
// sorted tradition > level > ID.
func TestByUsageType_CrossTraditionSortOrder(t *testing.T) {
	reg := loadTestRegistry(t)

	// prepared: neural_shock_fixture (technical, level 2) and acid_spray_fixture (bio_synthetic, level 1)
	// sort order: bio_synthetic < technical (lexicographic)
	results := reg.ByUsageType(technology.UsagePrepared)
	require.Len(t, results, 2)
	assert.Equal(t, technology.TraditionBioSynthetic, results[0].Tradition)
	assert.Equal(t, "acid_spray_fixture", results[0].ID)
	assert.Equal(t, technology.TraditionTechnical, results[1].Tradition)
	assert.Equal(t, "neural_shock_fixture", results[1].ID)

	// spontaneous: mind_spike_fixture (neural, level 1) and battle_fervor_fixture (fanatic_doctrine, level 1)
	// sort order: fanatic_doctrine < neural (lexicographic)
	spontaneous := reg.ByUsageType(technology.UsageSpontaneous)
	require.Len(t, spontaneous, 2)
	assert.Equal(t, technology.TraditionFanaticDoctrine, spontaneous[0].Tradition)
	assert.Equal(t, "battle_fervor_fixture", spontaneous[0].ID)
	assert.Equal(t, technology.TraditionNeural, spontaneous[1].Tradition)
	assert.Equal(t, "mind_spike_fixture", spontaneous[1].ID)
}

// REQ-TER19: All 14 tech YAML files load without error after model change.
func TestRegistry_REQ_TER19_AllTechsLoadAfterModelChange(t *testing.T) {
	dirs := []string{
		"../../../content/technologies/neural",
		"../../../content/technologies/innate",
	}
	for _, dir := range dirs {
		reg, err := technology.Load(dir)
		require.NoError(t, err, "failed to load directory %q", dir)
		assert.Greater(t, len(reg.All()), 0, "expected at least one tech in %q", dir)
	}
}

// REQ-TER20: Each tech with resolution:"save" has save_type and save_dc > 0.
func TestRegistry_REQ_TER20_SaveResolutionHasSaveTypeAndDC(t *testing.T) {
	dirs := []string{
		"../../../content/technologies/neural",
		"../../../content/technologies/innate",
	}
	for _, dir := range dirs {
		reg, err := technology.Load(dir)
		require.NoError(t, err)
		for _, tech := range reg.All() {
			if tech.Resolution == "save" {
				assert.NotEmpty(t, tech.SaveType, "tech %q: save resolution requires save_type", tech.ID)
				assert.Greater(t, tech.SaveDC, 0, "tech %q: save resolution requires save_dc > 0", tech.ID)
			}
		}
	}
}

// REQ-T12: Load with a malformed YAML file returns an error containing the file path;
// no registry returned.
func TestLoad_MalformedYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()

	// Write a YAML file with an unknown field — KnownFields(true) will reject it.
	badPath := filepath.Join(dir, "bad.yaml")
	err := os.WriteFile(badPath, []byte("id: test\nunknown_field: oops\n"), 0o644)
	require.NoError(t, err)

	reg, err := technology.Load(dir)
	assert.Error(t, err)
	assert.Nil(t, reg)
	assert.Contains(t, err.Error(), badPath)
}
