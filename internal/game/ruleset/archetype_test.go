package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
