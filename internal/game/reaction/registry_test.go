package reaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid2", reaction.TriggerOnSaveFail)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsNil_WrongTrigger(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Trigger: reaction.TriggerOnSaveFail,
		Effect:  reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave},
	}
	reg.Register("uid1", "chrome_reflex", "Chrome Reflex", def)

	result := reg.Get("uid1", reaction.TriggerOnDamageTaken)
	assert.Nil(t, result)
}

func TestReactionRegistry_GetReturnsRegisteredReaction(t *testing.T) {
	reg := reaction.NewReactionRegistry()
	def := reaction.ReactionDef{
		Trigger:     reaction.TriggerOnSaveFail,
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
