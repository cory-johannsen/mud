package gameserver_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

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

type fakePreparedRepo struct {
	slots map[int][]*session.PreparedSlot
}

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

type fakeInnateRepo struct {
	slots map[string]*session.InnateSlot
}

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
// Precondition: called only on test scenarios where auto-assign does not trigger the prompt;
// the len(options) == 0 guard is a safety fallback.
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

// TestPropertyAssignTechnologies_HardwiredRoundTrip verifies that AssignTechnologies
// followed by LoadTechnologies returns the identical hardwired IDs for any arbitrary input.
func TestPropertyAssignTechnologies_HardwiredRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		ids := make([]string, n)
		for i := 0; i < n; i++ {
			ids[i] = rapid.StringMatching(`[a-z_]{1,20}`).Draw(rt, fmt.Sprintf("id%d", i))
		}
		// deduplicate
		seen := map[string]bool{}
		unique := ids[:0]
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				unique = append(unique, id)
			}
		}
		ids = unique

		hwRepo := &fakeHardwiredRepo{}
		prepRepo := &fakePreparedRepo{}
		spontRepo := &fakeSpontaneousRepo{}
		innateRepo := &fakeInnateRepo{}
		sess := &session.PlayerSession{}

		job := &ruleset.Job{TechnologyGrants: &ruleset.TechnologyGrants{Hardwired: ids}}
		arch := &ruleset.Archetype{}

		err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, arch, nil, noPrompt, hwRepo, prepRepo, spontRepo, innateRepo)
		if err != nil {
			rt.Fatalf("AssignTechnologies: %v", err)
		}

		sess2 := &session.PlayerSession{}
		err = gameserver.LoadTechnologies(context.Background(), sess2, 1, hwRepo, prepRepo, spontRepo, innateRepo)
		if err != nil {
			rt.Fatalf("LoadTechnologies: %v", err)
		}

		if !reflect.DeepEqual(sess2.HardwiredTechs, ids) {
			rt.Fatalf("round-trip mismatch: got %v want %v", sess2.HardwiredTechs, ids)
		}
	})
}

// TestPropertyAssignTechnologies_AutoAssignNeverPrompts verifies that when the pool size
// exactly equals the open slot count, the promptFn is never invoked.
func TestPropertyAssignTechnologies_AutoAssignNeverPrompts(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		pool := make([]ruleset.PreparedEntry, n)
		for i := 0; i < n; i++ {
			pool[i] = ruleset.PreparedEntry{
				ID:    rapid.StringMatching(`[a-z_]{1,20}`).Draw(rt, fmt.Sprintf("poolID%d", i)),
				Level: 1,
			}
		}
		// deduplicate pool by ID
		seenPool := map[string]bool{}
		uniquePool := pool[:0]
		for _, e := range pool {
			if !seenPool[e.ID] {
				seenPool[e.ID] = true
				uniquePool = append(uniquePool, e)
			}
		}
		pool = uniquePool

		promptCalled := false
		trackingPrompt := func(options []string) (string, error) {
			promptCalled = true
			return options[0], nil
		}

		grants := &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: len(pool)},
				Pool:         pool,
			},
		}
		job := &ruleset.Job{TechnologyGrants: grants}
		arch := &ruleset.Archetype{}

		hwRepo := &fakeHardwiredRepo{}
		prepRepo := &fakePreparedRepo{}
		spontRepo := &fakeSpontaneousRepo{}
		innateRepo := &fakeInnateRepo{}
		sess := &session.PlayerSession{}

		err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, arch, nil, trackingPrompt, hwRepo, prepRepo, spontRepo, innateRepo)
		if err != nil {
			rt.Fatalf("AssignTechnologies: %v", err)
		}
		if promptCalled {
			rt.Fatalf("promptFn was called but should not have been (auto-assign)")
		}
	})
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

// REQ-LUT3: LevelUpTechnologies appends hardwired IDs, deduplicating against existing
func TestLevelUpTechnologies_HardwiredAppendAndDedup(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{stored: []string{"existing_tech"}}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{HardwiredTechs: []string{"existing_tech"}}

	grants := &ruleset.TechnologyGrants{
		Hardwired: []string{"new_tech", "existing_tech"}, // existing_tech is a duplicate
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	// existing_tech should not be duplicated; new_tech appended
	assert.Equal(t, []string{"existing_tech", "new_tech"}, sess.HardwiredTechs)
	assert.Equal(t, []string{"existing_tech", "new_tech"}, hw.stored)
}

// REQ-LUT4: LevelUpTechnologies fills prepared slots after existing indices (no collision)
func TestLevelUpTechnologies_PreparedSlotIndexOffset(t *testing.T) {
	ctx := context.Background()
	// Pre-populate 1 existing level-1 slot at index 0
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "original_tech"}},
	}}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "original_tech"}},
		},
	}

	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Fixed:        []ruleset.PreparedEntry{{ID: "new_tech", Level: 1}},
		},
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	// Level-1 slots: index 0 = original_tech, index 1 = new_tech
	require.Len(t, sess.PreparedTechs[1], 2)
	assert.Equal(t, "original_tech", sess.PreparedTechs[1][0].TechID)
	assert.Equal(t, "new_tech", sess.PreparedTechs[1][1].TechID)

	// Repo: index 0 unchanged, index 1 set to new_tech
	require.Len(t, prep.slots[1], 2)
	assert.Equal(t, "original_tech", prep.slots[1][0].TechID)
	assert.Equal(t, "new_tech", prep.slots[1][1].TechID)
}

// REQ-LUT5: LevelUpTechnologies adds spontaneous techs without removing existing ones
func TestLevelUpTechnologies_SpontaneousAppendsToExisting(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{techs: map[int][]string{
		1: {"existing_spont"},
	}}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{
		SpontaneousTechs: map[int][]string{
			1: {"existing_spont"},
		},
	}

	grants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{1: 1},
			Fixed:        []ruleset.SpontaneousEntry{{ID: "new_spont", Level: 1}},
		},
	}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"existing_spont", "new_spont"}, sess.SpontaneousTechs[1])
	assert.ElementsMatch(t, []string{"existing_spont", "new_spont"}, spont.techs[1])
}

// REQ-LUT6: LevelUpTechnologies with nil grants is a no-op
func TestLevelUpTechnologies_NilGrantsNoOp(t *testing.T) {
	ctx := context.Background()
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}
	sess := &session.PlayerSession{HardwiredTechs: []string{"existing"}}

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, nil, nil, noPrompt, hw, prep, spont, inn)
	require.NoError(t, err)
	assert.Equal(t, []string{"existing"}, sess.HardwiredTechs)
	assert.Nil(t, hw.stored) // SetAll never called
}

// REQ-RAR1 (property): All chosen techs after RearrangePreparedTechs come from
// the aggregated pool or fixed entries for their level.
func TestPropertyRearrangePreparedTechs_ChosenFromPool(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numFixed := rapid.IntRange(0, 2).Draw(rt, "numFixed")
		numPool := rapid.IntRange(1, 3).Draw(rt, "numPool")
		numSlots := numFixed + numPool

		fixed := make([]ruleset.PreparedEntry, numFixed)
		for i := 0; i < numFixed; i++ {
			fixed[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("fixed_%d", i), Level: 1}
		}
		pool := make([]ruleset.PreparedEntry, numPool)
		for i := 0; i < numPool; i++ {
			pool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("pool_%d", i), Level: 1}
		}

		existingSlots := make([]*session.PreparedSlot, numSlots)
		for i := range existingSlots {
			existingSlots[i] = &session.PreparedSlot{TechID: "old"}
		}
		prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{1: existingSlots}}
		sess := &session.PlayerSession{
			PreparedTechs: map[int][]*session.PreparedSlot{1: existingSlots},
		}
		job := &ruleset.Job{
			TechnologyGrants: &ruleset.TechnologyGrants{
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: numSlots},
					Fixed:        fixed,
					Pool:         pool,
				},
			},
		}

		err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, noPrompt, prep)
		if err != nil {
			rt.Fatalf("RearrangePreparedTechs: %v", err)
		}

		validIDs := make(map[string]bool)
		for _, e := range fixed {
			validIDs[e.ID] = true
		}
		for _, e := range pool {
			validIDs[e.ID] = true
		}
		for _, slot := range sess.PreparedTechs[1] {
			if !validIDs[slot.TechID] {
				rt.Fatalf("tech %q not in valid set", slot.TechID)
			}
		}
	})
}

// REQ-RAR2: Fixed entries occupy indices 0..n-1; pool selections follow at n..m-1.
func TestRearrangePreparedTechs_FixedFirst(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old1"}, {TechID: "old2"}},
	}}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old1"}, {TechID: "old2"}},
		},
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				Fixed:        []ruleset.PreparedEntry{{ID: "fixed_tech", Level: 1}},
				Pool:         []ruleset.PreparedEntry{{ID: "pool_tech", Level: 1}},
			},
		},
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep)
	require.NoError(t, err)
	require.Len(t, sess.PreparedTechs[1], 2)
	assert.Equal(t, "fixed_tech", sess.PreparedTechs[1][0].TechID, "fixed at index 0")
	assert.Equal(t, "pool_tech", sess.PreparedTechs[1][1].TechID, "pool at index 1")
}

// REQ-RAR3: LevelUpGrants entries above sess.Level are excluded from the pool.
// With level 3 excluded, the pool has exactly 1 entry for 1 slot → auto-assign fires.
// If level 3 were included, pool would have 2 entries for 1 slot → prompt would fire.
func TestRearrangePreparedTechs_LevelUpGrantsFiltered(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old"}},
	}}
	sess := &session.PlayerSession{
		Level: 2,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old"}},
		},
	}
	job := &ruleset.Job{
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{{ID: "level2_pool", Level: 1}},
			}},
			3: {Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{{ID: "level3_pool", Level: 1}},
			}},
		},
	}

	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, promptFn, prep)
	require.NoError(t, err)

	// Level3 excluded → pool has 1 entry for 1 slot → auto-assign, no prompt
	assert.False(t, promptCalled, "auto-assign fires when level3 excluded (pool==open)")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "level2_pool", sess.PreparedTechs[1][0].TechID)
}

// REQ-RAR4: Empty PreparedTechs is a no-op; DeleteAll is never called.
func TestRearrangePreparedTechs_EmptySession_NoOp(t *testing.T) {
	ctx := context.Background()
	// Populate repo to detect if DeleteAll is called (DeleteAll sets slots=nil).
	existingRepo := map[int][]*session.PreparedSlot{1: {{TechID: "db_slot"}}}
	prep := &fakePreparedRepo{slots: existingRepo}
	sess := &session.PlayerSession{} // no PreparedTechs

	job := &ruleset.Job{} // no grants

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep)
	require.NoError(t, err)
	// Repo unchanged: DeleteAll was not called
	assert.NotNil(t, prep.slots, "repo.slots must not be nil on no-op")
	assert.Equal(t, "db_slot", prep.slots[1][0].TechID, "repo must be unchanged on no-op")
}

// --- fakePendingTechLevelsRepo ---

type fakePendingTechLevelsRepo struct {
	levels       []int
	setWasCalled bool
}

func (r *fakePendingTechLevelsRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return r.levels, nil
}
func (r *fakePendingTechLevelsRepo) SetPendingTechLevels(_ context.Context, _ int64, levels []int) error {
	r.levels = levels
	r.setWasCalled = true
	return nil
}

// REQ-RAR5: Auto-assign fires when len(pool at level) == open slots; no prompt invoked.
func TestRearrangePreparedTechs_AutoAssignNoPrompt(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old"}},
	}}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old"}},
		},
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "only_option", Level: 1}},
			},
		},
	}

	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, promptFn, prep)
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt must not be called when pool == open slots")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "only_option", sess.PreparedTechs[1][0].TechID)
}

// REQ-ILT2: All-immediate grants (pool <= open slots) → deferred is nil.
func TestPartitionTechGrants_AllImmediate(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 2},
			Fixed:        []ruleset.PreparedEntry{{ID: "fixed", Level: 1}},
			Pool:         []ruleset.PreparedEntry{{ID: "only_pool", Level: 1}},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	assert.NotNil(t, immediate)
	assert.Nil(t, deferred, "no player choice needed when pool <= open slots")
}

// REQ-ILT1 (partition): Pool > open slots → deferred is non-nil for that level.
func TestPartitionTechGrants_DeferredWhenPoolExceedsSlots(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "pool_a", Level: 1},
				{ID: "pool_b", Level: 1},
			},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	assert.Nil(t, immediate, "no immediate grants when no fixed/auto-assign at this level")
	require.NotNil(t, deferred)
	require.NotNil(t, deferred.Prepared)
	assert.Equal(t, 1, deferred.Prepared.SlotsByLevel[1])
}

// REQ-ILT1: Hardwired entries always go to immediate.
func TestPartitionTechGrants_HardwiredAlwaysImmediate(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Hardwired: []string{"hw1", "hw2"},
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "p1", Level: 1},
				{ID: "p2", Level: 1},
			},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	require.NotNil(t, immediate)
	assert.Equal(t, []string{"hw1", "hw2"}, immediate.Hardwired)
	require.NotNil(t, deferred)
}

// REQ-ILT5: ResolvePendingTechGrants prompts for each pending level in ascending order,
// calls LevelUpTechnologies, and clears each entry.
func TestResolvePendingTechGrants_ResolvesAndClears(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{}
	hw := &fakeHardwiredRepo{}
	spont := &fakeSpontaneousRepo{}
	innate := &fakeInnateRepo{}
	progressRepo := &fakePendingTechLevelsRepo{}

	sess := &session.PlayerSession{
		Level: 3,
		PendingTechGrants: map[int]*ruleset.TechnologyGrants{
			2: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "level2_tech", Level: 1}},
			}},
		},
	}
	job := &ruleset.Job{
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: sess.PendingTechGrants[2],
		},
	}

	err := gameserver.ResolvePendingTechGrants(ctx, sess, 1, job, nil, noPrompt, hw, prep, spont, innate, progressRepo)
	require.NoError(t, err)
	assert.Empty(t, sess.PendingTechGrants, "pending grants must be cleared after resolution")
	assert.True(t, progressRepo.setWasCalled, "SetPendingTechLevels must be called after resolution")
}

// REQ-ILT7 (property): After ResolvePendingTechGrants, all chosen tech IDs are valid pool members.
func TestPropertyResolvePendingTechGrants_ChosenFromPool(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nPool := rapid.IntRange(1, 4).Draw(rt, "nPool")
		pool := make([]ruleset.PreparedEntry, nPool)
		for i := range pool {
			pool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("tech_%d", i), Level: 1}
		}
		// Open slots = 1 (pool > open → was deferred)
		grants := &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         pool,
			},
		}
		sess := &session.PlayerSession{
			Level:             5,
			PendingTechGrants: map[int]*ruleset.TechnologyGrants{3: grants},
		}
		prep := &fakePreparedRepo{}
		progressRepo := &fakePendingTechLevelsRepo{}

		err := gameserver.ResolvePendingTechGrants(context.Background(), sess, 1,
			&ruleset.Job{}, nil, noPrompt, &fakeHardwiredRepo{}, prep,
			&fakeSpontaneousRepo{}, &fakeInnateRepo{}, progressRepo)
		if err != nil {
			rt.Fatalf("ResolvePendingTechGrants: %v", err)
		}
		validIDs := make(map[string]bool)
		for _, e := range pool {
			validIDs[e.ID] = true
		}
		for _, slot := range prep.slots[1] {
			if !validIDs[slot.TechID] {
				rt.Fatalf("chosen tech %q not in pool", slot.TechID)
			}
		}
	})
}
