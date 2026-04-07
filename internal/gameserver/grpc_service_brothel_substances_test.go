package gameserver

// grpc_service_brothel_substances_test.go — REQ-BR-T9
// Verifies that all 10 disease substance YAML files and the flair_bonus_1 condition
// load from content/ without validation errors.

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/substance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrothelDiseaseSubstances_AllLoad verifies REQ-BR-T9:
// all 10 disease substance YAMLs in content/substances/ load and validate without error.
//
// Precondition: content/substances/ exists and contains all 10 disease substance files.
// Postcondition: Each disease substance ID is present in the registry with no load error.
func TestBrothelDiseaseSubstances_AllLoad(t *testing.T) {
	reg, err := substance.LoadDirectory("../../content/substances")
	require.NoError(t, err, "LoadDirectory must succeed for content/substances")

	diseaseIDs := []string{
		"street_fever",
		"crotch_rot",
		"swamp_itch",
		"track_rash",
		"gutter_flu",
		"rust_pox",
		"neon_blight",
		"wet_lung",
		"chrome_mange",
		"black_tongue",
	}

	for _, id := range diseaseIDs {
		t.Run(id, func(t *testing.T) {
			def, ok := reg.Get(id)
			require.True(t, ok, "disease substance %q must be present in the registry", id)
			assert.Equal(t, "disease", def.Category, "substance %q must have category=disease", id)
			assert.False(t, def.Addictive, "substance %q must not be addictive", id)
			assert.Equal(t, 0.0, def.AddictionChance, "substance %q must have addiction_chance=0.0", id)
		})
	}
}

// TestBrothelFlairBonus1Condition_Loads verifies that the flair_bonus_1 condition
// YAML loads without error and has correct fields.
//
// Precondition: content/conditions/flair_bonus_1.yaml exists.
// Postcondition: flair_bonus_1 is present in the condition registry with flair_bonus=1, max_stacks=1.
func TestBrothelFlairBonus1Condition_Loads(t *testing.T) {
	reg, err := condition.LoadDirectory("../../content/conditions")
	require.NoError(t, err, "LoadDirectory must succeed for content/conditions")

	def, ok := reg.Get("flair_bonus_1")
	require.True(t, ok, "condition flair_bonus_1 must be present in the registry")
	assert.Equal(t, "timed", def.DurationType, "flair_bonus_1 must have duration_type=timed")
	assert.Equal(t, 1, def.MaxStacks, "flair_bonus_1 must have max_stacks=1")
	assert.Equal(t, 1, def.FlairBonus, "flair_bonus_1 must have flair_bonus=1")
}

// TestProperty_BrothelDiseaseSubstances_AllCategoryDisease is a property-style check
// verifying REQ-BR-T9 for the disease category invariant across all 10 substances.
//
// Precondition: all 10 disease substance files have category=disease.
// Postcondition: every substance with an ID in diseaseIDs has category=disease.
func TestProperty_BrothelDiseaseSubstances_AllCategoryDisease(t *testing.T) {
	reg, err := substance.LoadDirectory("../../content/substances")
	require.NoError(t, err)

	diseaseIDs := []string{
		"street_fever", "crotch_rot", "swamp_itch", "track_rash", "gutter_flu",
		"rust_pox", "neon_blight", "wet_lung", "chrome_mange", "black_tongue",
	}

	for _, id := range diseaseIDs {
		def, ok := reg.Get(id)
		if !assert.True(t, ok, "substance %q must exist", id) {
			continue
		}
		assert.Equal(t, "disease", def.Category, "substance %q category must be disease", id)
	}
}
