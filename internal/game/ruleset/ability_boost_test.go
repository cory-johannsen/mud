package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadArchetypes_HasAbilityBoosts(t *testing.T) {
	archetypes, err := ruleset.LoadArchetypes("../../../content/archetypes")
	require.NoError(t, err)
	require.NotEmpty(t, archetypes)
	for _, a := range archetypes {
		assert.NotNil(t, a.AbilityBoosts, "archetype %q missing ability_boosts", a.ID)
		assert.Len(t, a.AbilityBoosts.Fixed, 2, "archetype %q must have exactly 2 fixed boosts", a.ID)
		assert.Equal(t, 2, a.AbilityBoosts.Free, "archetype %q must have free=2", a.ID)
	}
}

func TestLoadRegions_HasAbilityBoosts(t *testing.T) {
	regions, err := ruleset.LoadRegions("../../../content/regions")
	require.NoError(t, err)
	require.NotEmpty(t, regions)
	for _, r := range regions {
		assert.NotNil(t, r.AbilityBoosts, "region %q missing ability_boosts", r.ID)
		assert.Len(t, r.AbilityBoosts.Fixed, 2, "region %q must have exactly 2 fixed boosts", r.ID)
		assert.Equal(t, 1, r.AbilityBoosts.Free, "region %q must have free=1", r.ID)
	}
}

func TestAllAbilities_ReturnsSix(t *testing.T) {
	all := ruleset.AllAbilities()
	assert.Len(t, all, 6)
	assert.Contains(t, all, "brutality")
	assert.Contains(t, all, "grit")
	assert.Contains(t, all, "quickness")
	assert.Contains(t, all, "reasoning")
	assert.Contains(t, all, "savvy")
	assert.Contains(t, all, "flair")
}
