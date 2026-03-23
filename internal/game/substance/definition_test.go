package substance_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/substance"
)

func TestSubstanceDef_Validate_RejectsEmptyID(t *testing.T) {
	def := &substance.SubstanceDef{
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestSubstanceDef_Validate_RejectsEmptyName(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestSubstanceDef_Validate_RejectsInvalidCategory(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "invalid",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "category")
}

func TestSubstanceDef_Validate_RejectsInvalidOnsetDelay(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "notaduration",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onset_delay")
}

func TestSubstanceDef_Validate_RejectsInvalidDuration(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "notaduration",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duration")
}

func TestSubstanceDef_Validate_RejectsInvalidRecoveryDuration(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "notaduration",
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recovery_duration")
}

func TestSubstanceDef_Validate_RejectsAddictionChanceOutOfRange(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   1.5,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "addiction_chance")
}

func TestSubstanceDef_Validate_RejectsOverdoseThresholdLessThan1(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		AddictionChance:   0.1,
		OverdoseThreshold: 0,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overdose_threshold")
}

func TestSubstanceDef_Validate_RejectsMedicineAddictiveTrue(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "test",
		Name:              "Test",
		Category:          "medicine",
		OnsetDelayStr:     "0s",
		DurationStr:       "1m",
		RecoveryDurStr:    "0s",
		Addictive:         true,
		AddictionChance:   0.1,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "medicine")
}

func TestSubstanceDef_Validate_ValidDef_NoError(t *testing.T) {
	def := &substance.SubstanceDef{
		ID:                "jet",
		Name:              "Jet",
		Category:          "drug",
		OnsetDelayStr:     "0s",
		DurationStr:       "20m",
		RecoveryDurStr:    "4h",
		Addictive:         true,
		AddictionChance:   0.20,
		OverdoseThreshold: 3,
	}
	err := def.Validate()
	assert.NoError(t, err)
}

func TestSubstanceRegistry_Get_Found(t *testing.T) {
	reg := substance.NewRegistry()
	def := &substance.SubstanceDef{ID: "jet", Name: "Jet", Category: "drug",
		OnsetDelayStr: "0s", DurationStr: "1m", RecoveryDurStr: "0s",
		AddictionChance: 0.1, OverdoseThreshold: 3}
	require.NoError(t, def.Validate())
	reg.Register(def)
	got, ok := reg.Get("jet")
	assert.True(t, ok)
	assert.Equal(t, "jet", got.ID)
}

func TestSubstanceRegistry_Get_NotFound(t *testing.T) {
	reg := substance.NewRegistry()
	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestSubstanceRegistry_LoadDirectory_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: test_drug
name: Test Drug
category: drug
onset_delay: "0s"
duration: "10m"
recovery_duration: "0s"
addictive: false
addiction_chance: 0.0
overdose_threshold: 5
`
	err := os.WriteFile(filepath.Join(dir, "test_drug.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	reg, err := substance.LoadDirectory(dir)
	require.NoError(t, err)
	got, ok := reg.Get("test_drug")
	assert.True(t, ok)
	assert.Equal(t, "Test Drug", got.Name)
}

func TestSubstanceRegistry_LoadDirectory_MissingDir_Error(t *testing.T) {
	_, err := substance.LoadDirectory("/no/such/dir/abc123")
	assert.Error(t, err)
}

func TestPropertySubstanceDef_ValidateNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		def := &substance.SubstanceDef{
			ID:                rapid.String().Draw(t, "id"),
			Name:              rapid.String().Draw(t, "name"),
			Category:          rapid.String().Draw(t, "category"),
			OnsetDelayStr:     rapid.String().Draw(t, "onset_delay"),
			DurationStr:       rapid.String().Draw(t, "duration"),
			RecoveryDurStr:    rapid.String().Draw(t, "recovery_duration"),
			AddictionChance:   rapid.Float64().Draw(t, "addiction_chance"),
			OverdoseThreshold: rapid.Int().Draw(t, "overdose_threshold"),
		}
		_ = def.Validate() // must not panic
	})
}
