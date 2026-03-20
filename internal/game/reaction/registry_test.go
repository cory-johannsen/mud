package reaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

func TestReactionRegistry_GetReturnsNil_WhenNothingRegistered(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	result := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsNil_WrongUID(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid2", reaction.TriggerOnSaveFail)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsNil_WrongTrigger(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid1", reaction.TriggerOnDamageTaken)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsRegisteredReaction(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers:    []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Requirement: "wielding_melee_weapon",
		Effect:      reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.NotNil(t, result)
	assert.Equal(t, "uid1", result.UID)
	assert.Equal(t, "chrome_reflex", result.Feat)
	assert.Equal(t, "Chrome Reflex", result.FeatName)
	assert.Equal(t, def, result.Def)
}

func TestReactionRegistry_RegisterTwice_UpdatesInPlace(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def1 := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	def2 := reaction.ReactionDef{
		Triggers:    []reaction.ReactionTriggerType{reaction.TriggerOnSaveFail},
		Requirement: "wielding_melee_weapon",
		Effect:      reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def1)
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def2) // second registration

	result := reg.Get("uid1", reaction.TriggerOnSaveFail)
	require.NotNil(t, result)
	assert.Equal(t, def2, result.Def, "second registration should update in-place, not duplicate")
}

// REQ-CRX9: Register with two triggers — both are retrievable.
func TestReactionRegistry_MultiTrigger_BothRetrievable(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
		Effect: reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result1 := reg.Get("uid1", reaction.TriggerOnSaveFail)
	assert.NotNil(t, result1, "must be retrievable by TriggerOnSaveFail")
	assert.Equal(t, "chrome_reflex", result1.Feat)

	result2 := reg.Get("uid1", reaction.TriggerOnSaveCritFail)
	assert.NotNil(t, result2, "must be retrievable by TriggerOnSaveCritFail")
	assert.Equal(t, "chrome_reflex", result2.Feat)
}

// REQ-CRX9: Spending reaction on save_fail prevents crit_fail from firing in same round.
func TestReactionRegistry_CrossTrigger_OnlyOneFiresPerRound(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{
			reaction.TriggerOnSaveFail,
			reaction.TriggerOnSaveCritFail,
		},
		Effect: reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	reactionsRemaining := 1

	pr1 := reg.Get("uid1", reaction.TriggerOnSaveFail)
	require.NotNil(t, pr1)
	assert.Greater(t, reactionsRemaining, 0)
	reactionsRemaining--

	pr2 := reg.Get("uid1", reaction.TriggerOnSaveCritFail)
	require.NotNil(t, pr2, "reaction is registered for crit fail too")
	assert.Equal(t, 0, reactionsRemaining, "no reactions remaining after spending on save_fail")
}

// REQ-CRX2: Register with empty Triggers is a no-op.
func TestReactionRegistry_EmptyTriggers_NoOp(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	assert.Nil(t, reg.Get("uid1", reaction.TriggerOnSaveFail))
	assert.Nil(t, reg.Get("uid1", reaction.TriggerOnSaveCritFail))
}
