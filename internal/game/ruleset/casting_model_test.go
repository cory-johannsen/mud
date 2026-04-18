package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestResolveCastingModel_JobOverridesArchetype(t *testing.T) {
	job := &ruleset.Job{CastingModel: ruleset.CastingModelSpontaneous}
	arch := &ruleset.Archetype{CastingModel: ruleset.CastingModelWizard}
	assert.Equal(t, ruleset.CastingModelSpontaneous, ruleset.ResolveCastingModel(job, arch))
}

func TestResolveCastingModel_ArchetypeUsedWhenJobEmpty(t *testing.T) {
	job := &ruleset.Job{}
	arch := &ruleset.Archetype{CastingModel: ruleset.CastingModelDruid}
	assert.Equal(t, ruleset.CastingModelDruid, ruleset.ResolveCastingModel(job, arch))
}

func TestResolveCastingModel_NoneWhenBothEmpty(t *testing.T) {
	assert.Equal(t, ruleset.CastingModelNone, ruleset.ResolveCastingModel(&ruleset.Job{}, &ruleset.Archetype{}))
	assert.Equal(t, ruleset.CastingModelNone, ruleset.ResolveCastingModel(nil, nil))
}

func TestProperty_ResolveCastingModel_JobAlwaysWins(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		models := []ruleset.CastingModel{
			ruleset.CastingModelWizard, ruleset.CastingModelDruid,
			ruleset.CastingModelRanger, ruleset.CastingModelSpontaneous, ruleset.CastingModelNone,
		}
		jobModel := models[rapid.IntRange(0, len(models)-1).Draw(rt, "job")]
		archModel := models[rapid.IntRange(0, len(models)-1).Draw(rt, "arch")]
		job := &ruleset.Job{CastingModel: jobModel}
		arch := &ruleset.Archetype{CastingModel: archModel}
		result := ruleset.ResolveCastingModel(job, arch)
		if jobModel != ruleset.CastingModelNone && jobModel != "" {
			if result != jobModel {
				rt.Fatalf("expected job model %q to win, got %q", jobModel, result)
			}
		}
	})
}
