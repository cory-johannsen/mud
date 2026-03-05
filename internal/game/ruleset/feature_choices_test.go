package ruleset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestClassFeature_WithChoicesBlock(t *testing.T) {
	src := `
class_features:
  - id: predators_eye
    name: "Predator's Eye"
    archetype: drifter
    job: ""
    pf2e: hunters_edge
    active: false
    activate_text: ""
    description: "Choose a favored target."
    choices:
      key: favored_target
      prompt: "Choose your favored target type"
      options: [human, robot, animal, mutant]
`
	features, err := ruleset.LoadClassFeaturesFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, features, 1)
	cf := features[0]
	require.NotNil(t, cf.Choices)
	assert.Equal(t, "favored_target", cf.Choices.Key)
	assert.Equal(t, "Choose your favored target type", cf.Choices.Prompt)
	assert.Equal(t, []string{"human", "robot", "animal", "mutant"}, cf.Choices.Options)
}

func TestClassFeature_WithoutChoicesBlock(t *testing.T) {
	src := `
class_features:
  - id: street_brawler
    name: Street Brawler
    archetype: aggressor
    job: ""
    pf2e: attack_of_opportunity
    active: false
    activate_text: ""
    description: "No choices needed."
`
	features, err := ruleset.LoadClassFeaturesFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Nil(t, features[0].Choices)
}

func TestFeat_WithChoicesBlock(t *testing.T) {
	src := `
feats:
  - id: weapon_focus
    name: Weapon Focus
    category: general
    choices:
      key: weapon_group
      prompt: "Choose a weapon group"
      options: [pistol, rifle, melee, explosive]
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	f := feats[0]
	require.NotNil(t, f.Choices)
	assert.Equal(t, "weapon_group", f.Choices.Key)
	assert.Equal(t, []string{"pistol", "rifle", "melee", "explosive"}, f.Choices.Options)
}

func TestFeat_WithoutChoicesBlock(t *testing.T) {
	src := `
feats:
  - id: toughness
    name: Toughness
    category: general
    description: "+8 max HP."
`
	feats, err := ruleset.LoadFeatsFromBytes([]byte(src))
	require.NoError(t, err)
	require.Len(t, feats, 1)
	assert.Nil(t, feats[0].Choices)
}

func TestPropertyFeatureChoices_OptionsRoundtrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		key := rapid.StringMatching(`[a-z_]{1,32}`).Draw(rt, "key")
		prompt := rapid.StringMatching(`[a-zA-Z ]{1,64}`).Draw(rt, "prompt")
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		options := make([]string, n)
		for i := range options {
			options[i] = rapid.StringMatching(`[a-z]{1,16}`).Draw(rt, "opt")
		}
		orig := ruleset.FeatureChoices{Key: key, Prompt: prompt, Options: options}
		data, err := yaml.Marshal(orig)
		if err != nil {
			rt.Fatalf("marshal: %v", err)
		}
		var got ruleset.FeatureChoices
		if err := yaml.Unmarshal(data, &got); err != nil {
			rt.Fatalf("unmarshal: %v", err)
		}
		if got.Key != orig.Key {
			rt.Fatalf("Key mismatch: got %q want %q", got.Key, orig.Key)
		}
		if len(got.Options) != len(orig.Options) {
			rt.Fatalf("Options len mismatch: got %d want %d", len(got.Options), len(orig.Options))
		}
	})
}
