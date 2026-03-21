package reaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestReactionTriggerType_AllValuesNonEmpty(t *testing.T) {
	triggers := []reaction.ReactionTriggerType{
		reaction.TriggerOnSaveFail,
		reaction.TriggerOnSaveCritFail,
		reaction.TriggerOnDamageTaken,
		reaction.TriggerOnEnemyMoveAdjacent,
		reaction.TriggerOnConditionApplied,
		reaction.TriggerOnAllyDamaged,
		reaction.TriggerOnFall,
	}
	for _, t2 := range triggers {
		assert.NotEmpty(t, string(t2))
	}
}

func TestReactionDef_YAMLRoundTrip(t *testing.T) {
	original := reaction.ReactionDef{
		Triggers:    []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Requirement: "wielding_melee_weapon",
		Effect: reaction.ReactionEffect{
			Type: reaction.ReactionEffectRerollSave,
			Keep: "better",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded reaction.ReactionDef
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestReactionDef_YAMLRoundTrip_NoRequirement(t *testing.T) {
	original := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnEnemyMoveAdjacent},
		Effect: reaction.ReactionEffect{
			Type:   reaction.ReactionEffectStrike,
			Target: "trigger_source",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded reaction.ReactionDef
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestTriggerOnEnemyEntersRoom_Exists(t *testing.T) {
	assert.Equal(t, reaction.ReactionTriggerType("on_enemy_enters_room"), reaction.TriggerOnEnemyEntersRoom)
}

// REQ-CRX8: multi-trigger YAML round-trip.
func TestReactionDef_YAMLRoundTrip_MultiTrigger(t *testing.T) {
	original := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
		Effect: reaction.ReactionEffect{
			Type: reaction.ReactionEffectRerollSave,
			Keep: "better",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var decoded reaction.ReactionDef
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}
