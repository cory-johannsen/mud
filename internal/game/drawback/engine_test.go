package drawback_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/drawback"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestDrawbackEngine_FireTrigger_AppliesCondition(t *testing.T) {
	condDef := &condition.ConditionDef{ID: "demoralized", Name: "Demoralized", DurationType: "permanent"}
	condRegistry := &stubConditionRegistry{defs: map[string]*condition.ConditionDef{"demoralized": condDef}}
	job := &ruleset.Job{
		ID: "goon",
		Drawbacks: []ruleset.DrawbackDef{
			{
				ID:                "blood_fury",
				Type:              "situational",
				Trigger:           drawback.TriggerOnLeaveCombatWithoutKill,
				EffectConditionID: "demoralized",
				Duration:          "1h",
			},
		},
	}
	activeSet := condition.NewActiveSet()
	engine := drawback.NewEngine(condRegistry)
	engine.FireTrigger("uid1", drawback.TriggerOnLeaveCombatWithoutKill, []*ruleset.Job{job}, activeSet, time.Now())
	assert.True(t, activeSet.Has("demoralized"))
	assert.Equal(t, "drawback:goon", activeSet.SourceOf("demoralized"))
}

func TestDrawbackEngine_FireTrigger_WrongTrigger_NoEffect(t *testing.T) {
	condDef := &condition.ConditionDef{ID: "demoralized", Name: "Demoralized", DurationType: "permanent"}
	condRegistry := &stubConditionRegistry{defs: map[string]*condition.ConditionDef{"demoralized": condDef}}
	job := &ruleset.Job{
		ID: "goon",
		Drawbacks: []ruleset.DrawbackDef{
			{ID: "blood_fury", Type: "situational", Trigger: drawback.TriggerOnLeaveCombatWithoutKill, EffectConditionID: "demoralized", Duration: "1h"},
		},
	}
	activeSet := condition.NewActiveSet()
	engine := drawback.NewEngine(condRegistry)
	engine.FireTrigger("uid1", drawback.TriggerOnFailSkillCheck, []*ruleset.Job{job}, activeSet, time.Now())
	assert.False(t, activeSet.Has("demoralized"))
}

type stubConditionRegistry struct {
	defs map[string]*condition.ConditionDef
}

func (r *stubConditionRegistry) Get(id string) (*condition.ConditionDef, bool) {
	def, ok := r.defs[id]
	return def, ok
}
