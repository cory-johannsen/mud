package skillcheck_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTriggerDef_ParsesFromYAML(t *testing.T) {
	raw := `
skill: parkour
dc: 14
trigger: on_enter
outcomes:
  crit_success:
    message: "You vault effortlessly."
  success:
    message: "You pick your way through."
  failure:
    message: "You stumble."
    effect:
      type: damage
      formula: "1d4"
  crit_failure:
    message: "You fall hard."
    effect:
      type: damage
      formula: "2d4"
`
	var td skillcheck.TriggerDef
	err := yaml.Unmarshal([]byte(raw), &td)
	assert.NoError(t, err)
	assert.Equal(t, "parkour", td.Skill)
	assert.Equal(t, 14, td.DC)
	assert.Equal(t, "on_enter", td.Trigger)
	assert.NotNil(t, td.Outcomes.CritSuccess)
	assert.Equal(t, "You vault effortlessly.", td.Outcomes.CritSuccess.Message)
	assert.NotNil(t, td.Outcomes.Failure.Effect)
	assert.Equal(t, "damage", td.Outcomes.Failure.Effect.Type)
	assert.Equal(t, "1d4", td.Outcomes.Failure.Effect.Formula)
	assert.Nil(t, td.Outcomes.Success.Effect)
}

func TestCheckOutcome_Constants(t *testing.T) {
	assert.True(t, skillcheck.CritSuccess < skillcheck.Success)
	assert.True(t, skillcheck.Success < skillcheck.Failure)
	assert.True(t, skillcheck.Failure < skillcheck.CritFailure)
}

func TestCheckOutcome_String(t *testing.T) {
	assert.Equal(t, "crit_success", skillcheck.CritSuccess.String())
	assert.Equal(t, "success", skillcheck.Success.String())
	assert.Equal(t, "failure", skillcheck.Failure.String())
	assert.Equal(t, "crit_failure", skillcheck.CritFailure.String())
}

func TestOutcomeMap_ForOutcome(t *testing.T) {
	m := skillcheck.OutcomeMap{
		CritSuccess: &skillcheck.Outcome{Message: "great"},
		Success:     &skillcheck.Outcome{Message: "ok"},
		Failure:     &skillcheck.Outcome{Message: "bad"},
		CritFailure: &skillcheck.Outcome{Message: "terrible"},
	}
	assert.Equal(t, "great", m.ForOutcome(skillcheck.CritSuccess).Message)
	assert.Equal(t, "ok", m.ForOutcome(skillcheck.Success).Message)
	assert.Equal(t, "bad", m.ForOutcome(skillcheck.Failure).Message)
	assert.Equal(t, "terrible", m.ForOutcome(skillcheck.CritFailure).Message)
}
