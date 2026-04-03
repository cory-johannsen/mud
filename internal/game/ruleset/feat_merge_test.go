package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestMergeFeatGrants_NilInputs(t *testing.T) {
	assert.Nil(t, ruleset.MergeFeatGrants(nil, nil))
	assert.Equal(t, &ruleset.FeatGrants{Fixed: []string{"a"}}, ruleset.MergeFeatGrants(&ruleset.FeatGrants{Fixed: []string{"a"}}, nil))
	assert.Equal(t, &ruleset.FeatGrants{Fixed: []string{"b"}}, ruleset.MergeFeatGrants(nil, &ruleset.FeatGrants{Fixed: []string{"b"}}))
}

func TestMergeFeatGrants_MergesFixed(t *testing.T) {
	a := &ruleset.FeatGrants{Fixed: []string{"feat_a"}}
	b := &ruleset.FeatGrants{Fixed: []string{"feat_b"}}
	merged := ruleset.MergeFeatGrants(a, b)
	assert.ElementsMatch(t, []string{"feat_a", "feat_b"}, merged.Fixed)
}

func TestMergeFeatGrants_MergesChoices(t *testing.T) {
	a := &ruleset.FeatGrants{Choices: &ruleset.FeatChoices{Pool: []string{"x", "y"}, Count: 1}}
	b := &ruleset.FeatGrants{Choices: &ruleset.FeatChoices{Pool: []string{"z"}, Count: 1}}
	merged := ruleset.MergeFeatGrants(a, b)
	assert.Equal(t, 2, merged.Choices.Count)
	assert.ElementsMatch(t, []string{"x", "y", "z"}, merged.Choices.Pool)
}

func TestMergeFeatLevelUpGrants_NilInputs(t *testing.T) {
	assert.Nil(t, ruleset.MergeFeatLevelUpGrants(nil, nil))
}

func TestMergeFeatLevelUpGrants_MergesByLevel(t *testing.T) {
	arch := map[int]*ruleset.FeatGrants{
		2: {Choices: &ruleset.FeatChoices{Pool: []string{"a"}, Count: 1}},
	}
	job := map[int]*ruleset.FeatGrants{
		2: {Fixed: []string{"b"}},
		4: {Fixed: []string{"c"}},
	}
	merged := ruleset.MergeFeatLevelUpGrants(arch, job)
	assert.ElementsMatch(t, []string{"b"}, merged[2].Fixed)
	assert.Equal(t, []string{"a"}, merged[2].Choices.Pool)
	assert.Equal(t, []string{"c"}, merged[4].Fixed)
}

func TestProperty_MergeFeatLevelUpGrants_ContainsAllKeys(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		archKeys := rapid.SliceOfN(rapid.IntRange(2, 10), 0, 5).Draw(rt, "archKeys")
		jobKeys  := rapid.SliceOfN(rapid.IntRange(2, 10), 0, 5).Draw(rt, "jobKeys")

		arch := make(map[int]*ruleset.FeatGrants)
		for _, k := range archKeys {
			arch[k] = &ruleset.FeatGrants{Fixed: []string{"arch_feat"}}
		}
		job := make(map[int]*ruleset.FeatGrants)
		for _, k := range jobKeys {
			job[k] = &ruleset.FeatGrants{Fixed: []string{"job_feat"}}
		}

		merged := ruleset.MergeFeatLevelUpGrants(arch, job)

		for _, k := range archKeys {
			if _, ok := merged[k]; !ok {
				rt.Fatalf("archetype key %d missing from merged result", k)
			}
		}
		for _, k := range jobKeys {
			if _, ok := merged[k]; !ok {
				rt.Fatalf("job key %d missing from merged result", k)
			}
		}
	})
}
