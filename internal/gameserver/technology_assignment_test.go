package gameserver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

// --- fakes ---

type fakeHardwiredRepo struct{ stored []string }

func (r *fakeHardwiredRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	return r.stored, nil
}
func (r *fakeHardwiredRepo) SetAll(_ context.Context, _ int64, ids []string) error {
	r.stored = ids
	return nil
}

type fakePreparedRepo struct{ slots map[int][]*session.PreparedSlot }

func (r *fakePreparedRepo) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *fakePreparedRepo) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *fakePreparedRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }

type fakeSpontaneousRepo struct{ techs map[int][]string }

func (r *fakeSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	return r.techs, nil
}
func (r *fakeSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *fakeSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error {
	r.techs = nil
	return nil
}

type fakeInnateRepo struct{ slots map[string]*session.InnateSlot }

func (r *fakeInnateRepo) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	return r.slots, nil
}
func (r *fakeInnateRepo) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses}
	return nil
}
func (r *fakeInnateRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }

// noPrompt returns the first option automatically (for testing auto-assign paths).
func noPrompt(options []string) (string, error) {
	if len(options) == 0 {
		return "", nil
	}
	return options[0], nil
}

// REQ-TG6: assignTechnologies with full job+archetype populates all four session fields
func TestAssignTechnologies_FullJob(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Hardwired: []string{"neural_shock"},
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Fixed:        []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
			},
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: 1},
				UsesByLevel:  map[int]int{1: 4},
				Fixed:        []ruleset.SpontaneousEntry{{ID: "battle_fervor", Level: 1}},
			},
		},
	}
	archetype := &ruleset.Archetype{
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "acid_spray", UsesPerDay: 2},
		},
	}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, archetype, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	assert.Equal(t, []string{"neural_shock"}, sess.HardwiredTechs)
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "mind_spike", sess.PreparedTechs[1][0].TechID)
	assert.Equal(t, []string{"battle_fervor"}, sess.SpontaneousTechs[1])
	require.NotNil(t, sess.InnateTechs["acid_spray"])
	assert.Equal(t, 2, sess.InnateTechs["acid_spray"].MaxUses)
}

// REQ-TG7: nil TechnologyGrants leaves all session fields nil
func TestAssignTechnologies_NilGrants(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	job := &ruleset.Job{TechnologyGrants: nil}
	archetype := &ruleset.Archetype{}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, archetype, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	assert.Nil(t, sess.HardwiredTechs)
	assert.Nil(t, sess.PreparedTechs)
	assert.Nil(t, sess.SpontaneousTechs)
	assert.Nil(t, sess.InnateTechs)
}

// REQ-TG8: auto-assigns prepared pool when len(pool at level) == open slots (no prompt)
func TestAssignTechnologies_PreparedAutoAssign(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "mind_spike", Level: 1}},
			},
		},
	}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, nil, nil, promptFn, hw, prep, spont, inn)
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt should not be called when pool == open slots")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "mind_spike", sess.PreparedTechs[1][0].TechID)
}

// REQ-TG9: auto-assigns spontaneous pool when len(pool at level) == open slots (no prompt)
func TestAssignTechnologies_SpontaneousAutoAssign(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: 1},
				UsesByLevel:  map[int]int{1: 4},
				Pool:         []ruleset.SpontaneousEntry{{ID: "battle_fervor", Level: 1}},
			},
		},
	}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, nil, nil, promptFn, hw, prep, spont, inn)
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt should not be called when pool == open slots")
	assert.Equal(t, []string{"battle_fervor"}, sess.SpontaneousTechs[1])
}

// REQ-TG10: LoadTechnologies populates all four session fields from repos
func TestLoadTechnologies(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	hw := &fakeHardwiredRepo{stored: []string{"neural_shock"}}
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "mind_spike"}},
	}}
	spont := &fakeSpontaneousRepo{techs: map[int][]string{
		1: {"battle_fervor"},
	}}
	inn := &fakeInnateRepo{slots: map[string]*session.InnateSlot{
		"acid_spray": {MaxUses: 3},
	}}

	err := gameserver.LoadTechnologies(ctx, sess, 1, hw, prep, spont, inn)
	require.NoError(t, err)

	assert.Equal(t, []string{"neural_shock"}, sess.HardwiredTechs)
	assert.Equal(t, map[int][]*session.PreparedSlot{1: {{TechID: "mind_spike"}}}, sess.PreparedTechs)
	assert.Equal(t, map[int][]string{1: {"battle_fervor"}}, sess.SpontaneousTechs)
	assert.Equal(t, map[string]*session.InnateSlot{"acid_spray": {MaxUses: 3}}, sess.InnateTechs)
}
