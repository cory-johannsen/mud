package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHandleTrainSkill_NoArgs_ReturnsError(t *testing.T) {
	_, err := command.HandleTrainSkill(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

func TestHandleTrainSkill_UnknownSkill_ReturnsError(t *testing.T) {
	_, err := command.HandleTrainSkill([]string{"notaskill"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown skill")
}

func TestHandleTrainSkill_ValidSkill_ReturnsNormalizedID(t *testing.T) {
	id, err := command.HandleTrainSkill([]string{"Parkour"})
	assert.NoError(t, err)
	assert.Equal(t, "parkour", id)
}

func TestHandleTrainSkill_AllValidSkills(t *testing.T) {
	for _, skillID := range command.ValidSkillIDs {
		id, err := command.HandleTrainSkill([]string{skillID})
		assert.NoError(t, err)
		assert.Equal(t, skillID, id)
	}
}

func TestPropertyHandleTrainSkill_ValidAlwaysSucceeds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		skill := rapid.SampledFrom(command.ValidSkillIDs).Draw(rt, "skill")
		id, err := command.HandleTrainSkill([]string{skill})
		if err != nil {
			rt.Fatalf("unexpected error for valid skill %q: %v", skill, err)
		}
		if id != skill {
			rt.Fatalf("expected %q got %q", skill, id)
		}
	})
}
