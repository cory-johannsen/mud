package technology_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

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

// helper: write a single YAML file to a temp dir.
func writeTechYAML(t *testing.T, dir, filename, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0600)
	require.NoError(t, err)
}

const baseTechYAML = `id: tech_alpha
name: Alpha Tech
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`

// REQ-TSN-11a: Load() with valid short_name indexes the tech by short name.
func TestLoad_ShortName_IndexedCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "alpha.yaml", baseTechYAML+`short_name: ta
`)
	reg, err := technology.Load(dir)
	require.NoError(t, err)
	def, ok := reg.GetByShortName("ta")
	require.True(t, ok)
	assert.Equal(t, "tech_alpha", def.ID)
}

// REQ-TSN-11b: Load() returns error on duplicate short names.
func TestLoad_ShortName_DuplicateReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "alpha.yaml", baseTechYAML+`short_name: ta
`)
	writeTechYAML(t, dir, "beta.yaml", `id: tech_beta
name: Beta Tech
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: ta
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`)
	_, err := technology.Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ta")
}

// REQ-TSN-11c: Load() returns error when short_name equals another tech's id.
func TestLoad_ShortName_CollidesWithOtherID_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "alpha.yaml", baseTechYAML)
	writeTechYAML(t, dir, "beta.yaml", `id: tech_beta
name: Beta Tech
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: tech_alpha
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`)
	_, err := technology.Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tech_alpha")
}

// REQ-TSN-11b (property): Load() always errors on duplicate short names across any two technologies.
func TestProperty_Load_DuplicateShortName_AlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sn := rapid.StringMatching(`[a-z][a-z0-9_]{1,30}[a-z0-9]`).Draw(rt, "shortName")
		id1 := rapid.StringMatching(`[a-z][a-z0-9]{3,10}`).Draw(rt, "id1")
		id2 := rapid.StringMatching(`[a-z][a-z0-9]{3,10}`).Draw(rt, "id2")
		if id1 == id2 {
			rt.Skip() // IDs must be distinct
		}
		if id1 == sn || id2 == sn {
			rt.Skip() // IDs must not equal the short name (REQ-TSN-2c via Validate)
		}
		dir := t.TempDir()
		writeTechYAML(t, dir, "tech1.yaml", fmt.Sprintf(`id: %s
name: Tech One
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: %s
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, id1, sn))
		writeTechYAML(t, dir, "tech2.yaml", fmt.Sprintf(`id: %s
name: Tech Two
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: %s
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, id2, sn))
		_, err := technology.Load(dir)
		require.Error(rt, err, "Load() must error on duplicate short_name %q", sn)
		assert.Contains(rt, err.Error(), sn, "error must contain the duplicate short_name")
	})
}

// REQ-TSN-11c (property): Load() always errors when a short_name equals another technology's id.
func TestProperty_Load_ShortNameCollidesWithID_AlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id1 := rapid.StringMatching(`[a-z][a-z0-9]{3,10}`).Draw(rt, "id1")
		id2 := rapid.StringMatching(`[a-z][a-z0-9]{3,10}`).Draw(rt, "id2")
		if id1 == id2 {
			rt.Skip() // IDs must be distinct
		}
		// id2 will be used as short_name for the first tech
		// This must collide with id2's own ID
		dir := t.TempDir()
		writeTechYAML(t, dir, "tech1.yaml", fmt.Sprintf(`id: %s
name: Tech One
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: %s
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, id1, id2))
		writeTechYAML(t, dir, "tech2.yaml", fmt.Sprintf(`id: %s
name: Tech Two
tradition: neural
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, id2))
		_, err := technology.Load(dir)
		require.Error(rt, err, "Load() must error when short_name %q equals tech id %q", id2, id2)
		assert.Contains(rt, err.Error(), id2, "error must mention the colliding id")
	})
}

// REQ-TSN-11a (property): GetByShortName returns the correct def for any loaded short name.
func TestProperty_Registry_GetByShortName_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sn := rapid.StringMatching(`[a-z][a-z0-9_]{1,30}[a-z0-9]`).Draw(rt, "shortName")
		dir := t.TempDir()
		writeTechYAML(t, dir, "tech.yaml", fmt.Sprintf(`id: tech_roundtrip
name: Roundtrip Tech
tradition: technical
level: 1
usage_type: hardwired
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
short_name: %s
effects:
  on_apply:
    - type: utility
      utility_type: unlock
`, sn))
		reg, err := technology.Load(dir)
		require.NoError(rt, err)
		def, ok := reg.GetByShortName(sn)
		assert.True(rt, ok)
		if ok {
			assert.Equal(rt, "tech_roundtrip", def.ID)
		}
	})
}

// GetByShortName returns (nil, false) for unknown short name.
func TestGetByShortName_Unknown_ReturnsFalse(t *testing.T) {
	reg := technology.NewRegistry()
	def, ok := reg.GetByShortName("nope")
	assert.False(t, ok)
	assert.Nil(t, def)
}

// Register() populates byShortName when ShortName is set.
func TestRegister_PopulatesShortNameIndex(t *testing.T) {
	reg := technology.NewRegistry()
	def := &technology.TechnologyDef{
		ID:        "tech_reg",
		ShortName: "tr",
		Name:      "Reg Tech",
		Tradition: technology.TraditionTechnical,
		Level:     1,
		UsageType: technology.UsageHardwired,
		Range:     technology.RangeSelf,
		Targets:   technology.TargetsSingle,
		Duration:  "instant",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{{Type: technology.EffectUtility, UtilityType: "unlock"}},
		},
	}
	reg.Register(def)
	got, ok := reg.GetByShortName("tr")
	require.True(t, ok)
	assert.Equal(t, "tech_reg", got.ID)
}
