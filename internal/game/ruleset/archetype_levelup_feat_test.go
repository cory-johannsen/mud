package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestArchetype_LevelUpFeatGrants_ParsesFromYAML(t *testing.T) {
	raw := `
id: test_archetype
name: Test
key_ability: brutality
hit_points_per_level: 10
level_up_feat_grants:
  2:
    choices:
      pool: [feat_a, feat_b]
      count: 1
  4:
    fixed: [feat_c]
`
	var a ruleset.Archetype
	require.NoError(t, yaml.Unmarshal([]byte(raw), &a))
	require.NotNil(t, a.LevelUpFeatGrants)
	assert.NotNil(t, a.LevelUpFeatGrants[2])
	assert.Equal(t, 1, a.LevelUpFeatGrants[2].Choices.Count)
	assert.Equal(t, []string{"feat_a", "feat_b"}, a.LevelUpFeatGrants[2].Choices.Pool)
	assert.NotNil(t, a.LevelUpFeatGrants[4])
	assert.Equal(t, []string{"feat_c"}, a.LevelUpFeatGrants[4].Fixed)
}

func TestJob_LevelUpFeatGrants_ParsesFromYAML(t *testing.T) {
	raw := `
id: test_job
name: Test Job
archetype: test_archetype
tier: 1
key_ability: brutality
hit_points_per_level: 10
level_up_feat_grants:
  2:
    fixed: [job_feat_a]
`
	var j ruleset.Job
	require.NoError(t, yaml.Unmarshal([]byte(raw), &j))
	require.NotNil(t, j.LevelUpFeatGrants)
	assert.Equal(t, []string{"job_feat_a"}, j.LevelUpFeatGrants[2].Fixed)
}
