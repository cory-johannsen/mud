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
	"github.com/cory-johannsen/mud/internal/game/technology"
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
func (r *fakePreparedRepo) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	return nil
}

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
func (r *fakeInnateRepo) Decrement(_ context.Context, _ int64, _ string) error { return nil }
func (r *fakeInnateRepo) RestoreAll(_ context.Context, _ int64) error           { return nil }

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

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, archetype, nil, noPrompt, hw, prep, spont, inn, nil, nil)
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

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, archetype, nil, noPrompt, hw, prep, spont, inn, nil, nil)
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

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, nil, nil, promptFn, hw, prep, spont, inn, nil, nil)
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

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, nil, nil, promptFn, hw, prep, spont, inn, nil, nil)
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

		err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, arch, nil, noPrompt, hwRepo, prepRepo, spontRepo, innateRepo, nil, nil)
		if err != nil {
			rt.Fatalf("AssignTechnologies: %v", err)
		}

		sess2 := &session.PlayerSession{}
		err = gameserver.LoadTechnologies(context.Background(), sess2, 1, hwRepo, prepRepo, spontRepo, innateRepo, nil)
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

		err := gameserver.AssignTechnologies(context.Background(), sess, 1, job, arch, nil, trackingPrompt, hwRepo, prepRepo, spontRepo, innateRepo, nil, nil)
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

	err := gameserver.LoadTechnologies(ctx, sess, 1, hw, prep, spont, inn, nil)
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

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn, nil)
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

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn, nil)
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

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, noPrompt, hw, prep, spont, inn, nil)
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

	err := gameserver.LevelUpTechnologies(ctx, sess, 1, nil, nil, noPrompt, hw, prep, spont, inn, nil)
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

		err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, nil, noPrompt, prep, nil, technology.TraditionFlavor{})
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

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, nil, noPrompt, prep, nil, technology.TraditionFlavor{})
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

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, nil, promptFn, prep, nil, technology.TraditionFlavor{})
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

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, nil, noPrompt, prep, nil, technology.TraditionFlavor{})
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

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, nil, promptFn, prep, nil, technology.TraditionFlavor{})
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

// TestPartitionTechGrants_L2PreparedAlwaysDeferred verifies that prepared grants at
// tech level 2 are always placed in deferred, even when pool <= slots (REQ-TTA-2).
//
// Precondition: Grants with 1 L2 slot and 1 L2 pool entry (pool == slots, normally immediate).
// Postcondition: immediate is nil; deferred contains the L2 prepared grants.
func TestPartitionTechGrants_L2PreparedAlwaysDeferred(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{2: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "acid_clamp", Level: 2}},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	assert.Nil(t, immediate, "L2 grant must NOT be immediate")
	assert.NotNil(t, deferred, "L2 grant MUST be deferred")
	assert.Equal(t, 1, deferred.Prepared.SlotsByLevel[2])
}

// TestPartitionTechGrants_L1PreparedImmediateWhenPoolFits verifies that prepared grants
// at tech level 1 with pool <= slots remain immediate (existing behaviour unchanged).
func TestPartitionTechGrants_L1PreparedImmediateWhenPoolFits(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 2},
			Pool:         []ruleset.PreparedEntry{{ID: "tech_a", Level: 1}},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	assert.NotNil(t, immediate, "L1 grant with pool <= slots MUST be immediate")
	_ = deferred
}

// TestFilterGrantsByMaxTechLevel_ReturnsOnlyL1 verifies that filtering a mixed L1/L2
// grant returns only L1 entries.
func TestFilterGrantsByMaxTechLevel_ReturnsOnlyL1(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1, 2: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "tech_a", Level: 1},
				{ID: "tech_b", Level: 2},
			},
		},
	}
	filtered := gameserver.FilterGrantsByMaxTechLevel(grants, 1)
	require.NotNil(t, filtered)
	require.NotNil(t, filtered.Prepared)
	assert.Equal(t, map[int]int{1: 1}, filtered.Prepared.SlotsByLevel, "only L1 slots")
	assert.Len(t, filtered.Prepared.Pool, 1, "only L1 pool entries")
	assert.Equal(t, "tech_a", filtered.Prepared.Pool[0].ID)
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

// TestPartitionTechGrants_L2SpontaneousAlwaysDeferred verifies that spontaneous grants at
// tech level 2 are always placed in deferred (REQ-TTA-2).
//
// Precondition: Grants with 1 L2 spontaneous slot.
// Postcondition: immediate is nil; deferred contains the L2 spontaneous grants.
func TestPartitionTechGrants_L2SpontaneousAlwaysDeferred(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{2: 1},
			UsesByLevel:  map[int]int{2: 3},
			Pool:         []ruleset.SpontaneousEntry{{ID: "neural_override", Level: 2}},
		},
	}
	immediate, deferred := gameserver.PartitionTechGrants(grants)
	assert.Nil(t, immediate, "L2 spontaneous grant must NOT be immediate")
	assert.NotNil(t, deferred, "L2 spontaneous grant MUST be deferred")
	require.NotNil(t, deferred.Spontaneous)
	assert.Equal(t, 1, deferred.Spontaneous.KnownByLevel[2])
	assert.Equal(t, 3, deferred.Spontaneous.UsesByLevel[2], "UsesByLevel must be preserved")
}

// TestPartitionTechGrants_L1SpontaneousPreservesUsesByLevel verifies that UsesByLevel
// is preserved when L1 spontaneous grants are partitioned to immediate.
func TestPartitionTechGrants_L1SpontaneousPreservesUsesByLevel(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Spontaneous: &ruleset.SpontaneousGrants{
			KnownByLevel: map[int]int{1: 1},
			UsesByLevel:  map[int]int{1: 2},
			Pool:         []ruleset.SpontaneousEntry{{ID: "hack_basic", Level: 1}},
		},
	}
	immediate, _ := gameserver.PartitionTechGrants(grants)
	require.NotNil(t, immediate)
	require.NotNil(t, immediate.Spontaneous)
	assert.Equal(t, 2, immediate.Spontaneous.UsesByLevel[1], "UsesByLevel must be preserved in immediate")
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

	err := gameserver.ResolvePendingTechGrants(ctx, sess, 1, job, nil, noPrompt, hw, prep, spont, innate, nil, progressRepo)
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
			&fakeSpontaneousRepo{}, &fakeInnateRepo{}, nil, progressRepo)
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

// TestResolvePendingTechGrants_SkipsL2AndAbove verifies that ResolvePendingTechGrants
// auto-resolves L1 grants but leaves L2+ grants in sess.PendingTechGrants (REQ-TTA-2).
//
// Precondition: sess.PendingTechGrants[3] has both L1 and L2 prepared grants.
// Postcondition: L1 slot is filled; PendingTechGrants[3] still exists with only L2 grants.
func TestResolvePendingTechGrants_SkipsL2AndAbove(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{}
	hw := &fakeHardwiredRepo{}
	spont := &fakeSpontaneousRepo{}
	innate := &fakeInnateRepo{}
	progressRepo := &fakePendingTechLevelsRepo{}

	// char level 3 has an L1 slot (auto-resolvable) and an L2 slot (trainer-required).
	sess := &session.PlayerSession{
		Level: 5,
		PendingTechGrants: map[int]*ruleset.TechnologyGrants{
			3: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1, 2: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "l1_tech", Level: 1},
					{ID: "l2_tech", Level: 2},
				},
			}},
		},
	}

	err := gameserver.ResolvePendingTechGrants(ctx, sess, 1, &ruleset.Job{}, nil, noPrompt,
		hw, prep, spont, innate, nil, progressRepo)
	require.NoError(t, err)

	// L1 slot must be resolved into PreparedTechs.
	require.NotEmpty(t, prep.slots[1], "L1 prepared slot must be filled after resolve")

	// L2 grant must still be pending (not resolved).
	remaining, ok := sess.PendingTechGrants[3]
	require.True(t, ok, "PendingTechGrants[3] must still exist because L2 grant was not resolved")
	require.NotNil(t, remaining.Prepared, "remaining Prepared must be non-nil")
	assert.Equal(t, 1, remaining.Prepared.SlotsByLevel[2], "L2 slot must still be pending")
	_, l1Still := remaining.Prepared.SlotsByLevel[1]
	assert.False(t, l1Still, "L1 slot must be removed from PendingTechGrants after resolution")
}

// REQ-JTG6: AssignTechnologies returns a wrapped error when merged grants fail Validate().
// REQ-JTG7: AssignTechnologies calls Validate() on the merged result before processing
//
//	(also exercised by TestAssignTechnologies_ArchetypeSlots_JobPool_Merged on the success path).
func TestAssignTechnologies_MergedGrantsValidationError(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	// Archetype provides 3 slots but no pool.
	arch := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 3},
			},
		},
	}
	// Job provides only 1 pool entry — merged: 3 slots, 1 pool → invalid.
	job := &ruleset.Job{
		ID: "test_job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
			},
		},
	}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid merged grants")
}

// REQ-JTG3 integration: AssignTechnologies uses merged slot count (archetype + job).
func TestAssignTechnologies_ArchetypeSlots_JobPool_Merged(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	// Archetype: 2 prepared slots at level 1.
	arch := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
			},
		},
	}
	// Job: 2 pool entries to satisfy 2 slots.
	job := &ruleset.Job{
		ID: "test_nerd_job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{
					{ID: "neural_shock", Level: 1},
					{ID: "mind_spike", Level: 1},
				},
			},
		},
	}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.NoError(t, err)
	// 2 slots filled from pool (auto-assign since pool size == slots).
	assert.Len(t, sess.PreparedTechs[1], 2)
}

// TestAssignTechnologies_NilJobTechGrants_ArchetypeGrantsUsed verifies that when
// job.TechnologyGrants is nil but archetype.TechnologyGrants is non-nil, grants are processed.
func TestAssignTechnologies_NilJobTechGrants_ArchetypeGrantsUsed(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	// Archetype: 1 slot, 1 pool entry (valid merged grant).
	arch := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "neural_shock", Level: 1}},
			},
		},
	}
	// Job has no TechnologyGrants.
	job := &ruleset.Job{ID: "no_grants_job"}
	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, job, arch, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.NoError(t, err)
	assert.Len(t, sess.PreparedTechs[1], 1)
}

func TestAssignTechnologies_RegionInnateGrant(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	region := &ruleset.Region{
		ID: "gresham_outskirts",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "acid_spit", UsesPerDay: 1},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, nil, nil, noPrompt, hw, prep, spont, inn, nil, region)
	require.NoError(t, err)

	slot, ok := sess.InnateTechs["acid_spit"]
	require.True(t, ok, "expected acid_spit in session InnateTechs")
	assert.Equal(t, 1, slot.MaxUses)
	assert.Equal(t, 1, slot.UsesRemaining)

	repoSlot, repoOk := inn.slots["acid_spit"]
	require.True(t, repoOk, "expected acid_spit persisted to repo")
	assert.Equal(t, 1, repoSlot.MaxUses)
}

func TestAssignTechnologies_ArchetypeInnateGrant_NoJobNoRegion(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	archetype := &ruleset.Archetype{
		ID: "test_archetype",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "blackout_pulse", UsesPerDay: 0},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, archetype, nil, noPrompt, hw, prep, spont, inn, nil, nil)
	require.NoError(t, err)

	slot, ok := sess.InnateTechs["blackout_pulse"]
	require.True(t, ok, "expected blackout_pulse in session InnateTechs from archetype")
	assert.Equal(t, 0, slot.MaxUses)
	assert.Equal(t, 0, slot.UsesRemaining)
}

// REQ-LF7: RearrangePreparedTechs emits opening progress message as first sendFn call.
func TestRearrangePreparedTechs_SendFn_OpeningMessage(t *testing.T) {
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
				Fixed: []ruleset.PreparedEntry{{ID: "tech_a", Level: 1}},
			},
		},
	}
	flavor := technology.TraditionFlavor{
		LoadoutTitle: "Field Loadout",
		PrepVerb:     "Configure",
		PrepGerund:   "Configuring",
		SlotNoun:     "slot",
		RestMessage:  "Field loadout configured.",
	}

	var messages []string
	sendFn := func(msg string) { messages = append(messages, msg) }

	err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, nil, noPrompt, prep, sendFn, flavor)
	require.NoError(t, err)
	require.NotEmpty(t, messages, "sendFn must have been called")
	assert.Equal(t, "Configuring Field Loadout...", messages[0], "first message must be the opening progress message")
}

// REQ-LF8: RearrangePreparedTechs emits per-slot sendFn messages.
// Uses 1 fixed entry and 2 pool entries for 1 open slot so that pool > open slots,
// forcing the prompt path and the "choose from pool" message.
func TestRearrangePreparedTechs_SendFn_SlotMessages(t *testing.T) {
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old1"}, {TechID: "old2"}},
	}}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old1"}, {TechID: "old2"}},
		},
	}
	// 1 fixed + 2 pool entries for 1 open slot (2 total slots: 1 fixed, 1 open).
	// pool size (2) > open slots (1) → prompt fires for the open slot.
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				Fixed: []ruleset.PreparedEntry{{ID: "fixed_tech", Level: 1}},
				Pool: []ruleset.PreparedEntry{
					{ID: "pool_tech_a", Level: 1},
					{ID: "pool_tech_b", Level: 1},
				},
			},
		},
	}
	flavor := technology.TraditionFlavor{
		LoadoutTitle: "Chem Kit",
		PrepVerb:     "Mix",
		PrepGerund:   "Mixing",
		SlotNoun:     "dose",
		RestMessage:  "Chem kit mixed.",
	}

	var messages []string
	sendFn := func(msg string) { messages = append(messages, msg) }

	promptFn := func(options []string) (string, error) {
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, nil, promptFn, prep, sendFn, flavor)
	require.NoError(t, err)

	// Check fixed-slot message appears
	assert.Contains(t, messages, "Level 1, dose 1 (fixed): fixed_tech", "fixed slot message must be emitted")
	// Check open-pool message appears
	assert.Contains(t, messages, "Level 1, dose 2: choose from pool", "open pool message must be emitted")
}

// REQ-SSL4 (property): LevelUpTechnologies calls promptFn exactly N times when pool > open slots.
// All selected IDs come from the pool; no duplicates; session has exactly N entries at the level.
func TestPropertyLevelUpTechnologies_SpontaneousPromptCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate pool size 2-6 and open slots strictly less than pool size.
		nPool := rapid.IntRange(2, 6).Draw(rt, "nPool")
		nOpen := rapid.IntRange(1, nPool-1).Draw(rt, "nOpen")

		pool := make([]ruleset.SpontaneousEntry, nPool)
		for i := range pool {
			pool[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("prop_tech_%d", i), Level: 1}
		}

		grants := &ruleset.TechnologyGrants{
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: nOpen},
				// No Fixed entries — only prompt-chosen techs populate SpontaneousTechs[1].
				Pool: pool,
			},
		}

		sess := &session.PlayerSession{Level: 5}
		spont := &fakeSpontaneousRepo{}
		hw := &fakeHardwiredRepo{}
		prep := &fakePreparedRepo{}
		inn := &fakeInnateRepo{}

		promptCallCount := 0
		promptFn := func(options []string) (string, error) {
			promptCallCount++
			// Return the first option each time (greedy selection).
			if len(options) == 0 {
				return "", nil
			}
			return options[0], nil
		}

		ctx := context.Background()
		err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, promptFn, hw, prep, spont, inn, nil)
		if err != nil {
			rt.Fatalf("LevelUpTechnologies: %v", err)
		}

		// Invariant 1: promptFn called exactly nOpen times.
		if promptCallCount != nOpen {
			rt.Fatalf("expected promptFn called %d times, got %d", nOpen, promptCallCount)
		}

		chosen := sess.SpontaneousTechs[1]

		// Invariant 2: exactly nOpen entries in session.
		if len(chosen) != nOpen {
			rt.Fatalf("expected %d entries in SpontaneousTechs[1], got %d", nOpen, len(chosen))
		}

		// Invariant 3: all IDs are from the pool.
		validIDs := make(map[string]bool, nPool)
		for _, e := range pool {
			validIDs[e.ID] = true
		}
		for _, id := range chosen {
			if !validIDs[id] {
				rt.Fatalf("chosen tech %q not in pool", id)
			}
		}

		// Invariant 4: no duplicates.
		seen := make(map[string]bool, len(chosen))
		for _, id := range chosen {
			if seen[id] {
				rt.Fatalf("duplicate tech ID %q in SpontaneousTechs[1]", id)
			}
			seen[id] = true
		}
	})
}

// TestPropertyAssignTechnologies_PreparedOnlyRoundTrip verifies that AssignTechnologies
// followed by LoadTechnologies returns identical prepared slot assignments for a job
// that has no hardwired techs (prepared-only). This guards against BUG-11, where the
// "already assigned" check that only inspected the hardwired repo would fail to detect
// existing prepared assignments, causing them to be overwritten on every login.
func TestPropertyAssignTechnologies_PreparedOnlyRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		poolRaw := make([]ruleset.PreparedEntry, n)
		seenIDs := map[string]bool{}
		uniquePool := poolRaw[:0]
		for i := 0; i < n; i++ {
			id := rapid.StringMatching(`[a-z]{1,16}`).Draw(rt, fmt.Sprintf("poolID%d", i))
			if seenIDs[id] {
				continue
			}
			seenIDs[id] = true
			uniquePool = append(uniquePool, ruleset.PreparedEntry{ID: id, Level: 1})
		}
		if len(uniquePool) == 0 {
			rt.Skip()
		}

		hwRepo := &fakeHardwiredRepo{} // intentionally empty — no hardwired techs
		prepRepo := &fakePreparedRepo{}
		spontRepo := &fakeSpontaneousRepo{}
		innateRepo := &fakeInnateRepo{}

		job := &ruleset.Job{
			TechnologyGrants: &ruleset.TechnologyGrants{
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: len(uniquePool)},
					Pool:         uniquePool,
				},
			},
		}

		sess1 := &session.PlayerSession{}
		if err := gameserver.AssignTechnologies(context.Background(), sess1, 1, job, nil, nil, noPrompt, hwRepo, prepRepo, spontRepo, innateRepo, nil, nil); err != nil {
			rt.Fatalf("AssignTechnologies: %v", err)
		}

		// Invariant 1: prepared slots were persisted — repo must be non-empty.
		if len(prepRepo.slots) == 0 {
			rt.Fatalf("prepared repo is empty after AssignTechnologies; assignments not persisted")
		}

		// Simulate second login: load from repo into a fresh session.
		sess2 := &session.PlayerSession{}
		if err := gameserver.LoadTechnologies(context.Background(), sess2, 1, hwRepo, prepRepo, spontRepo, innateRepo, nil); err != nil {
			rt.Fatalf("LoadTechnologies: %v", err)
		}

		// Invariant 2: LoadTechnologies must restore exactly the same prepared slots.
		if len(sess2.PreparedTechs) == 0 {
			rt.Fatalf("PreparedTechs empty after LoadTechnologies; assignments lost on second login")
		}
		if !reflect.DeepEqual(sess1.PreparedTechs, sess2.PreparedTechs) {
			rt.Fatalf("round-trip mismatch: first=%v second=%v", sess1.PreparedTechs, sess2.PreparedTechs)
		}
	})
}

// REQ-TG-BUG6a: buildOptions with registry uses display name format "[id] Name — desc"
func TestBuildOptions_WithRegistry_UsesDisplayName(t *testing.T) {
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID:          "bio_synthetic",
		Name:        "Bio-Synthetic",
		Description: "Organic augmentation technologies.",
		Tradition:   technology.TraditionBioSynthetic,
		UsageType:   technology.UsageHardwired,
		Level:       1,
	})

	opts := gameserver.ExportedBuildOptions([]string{"bio_synthetic"}, []int{1}, reg)
	require.Len(t, opts, 1)
	assert.Equal(t, "[bio_synthetic] Bio-Synthetic \u2014 Organic augmentation technologies.", opts[0])
}

// REQ-TG-BUG6b: buildOptions without registry falls back to raw ID
func TestBuildOptions_NilRegistry_FallsBackToID(t *testing.T) {
	opts := gameserver.ExportedBuildOptions([]string{"bio_synthetic"}, []int{1}, nil)
	require.Len(t, opts, 1)
	assert.Equal(t, "bio_synthetic", opts[0])
}

// REQ-TG-BUG6c: parseTechID extracts ID from bracket notation "[id] Name — desc"
func TestParseTechID_BracketFormat(t *testing.T) {
	id := gameserver.ExportedParseTechID("[acid_arrow] Acid Arrow \u2014 Launches an acid projectile.")
	assert.Equal(t, "acid_arrow", id)
}

// REQ-TG-BUG6d: parseTechID falls back gracefully on old format "id — desc"
func TestParseTechID_LegacyFormat(t *testing.T) {
	id := gameserver.ExportedParseTechID("bio_synthetic \u2014 Organic augmentation technologies.")
	assert.Equal(t, "bio_synthetic", id)
}

// REQ-TG-BUG6e: parseTechID handles bare ID with no em-dash
func TestParseTechID_BareID(t *testing.T) {
	id := gameserver.ExportedParseTechID("bio_synthetic")
	assert.Equal(t, "bio_synthetic", id)
}

// REQ-RAR6: RearrangePreparedTechs must include archetype pool entries so that
// all slots can be filled when the job pool alone is insufficient (bug #40).
func TestRearrangePreparedTechs_ArchetypePoolIncluded(t *testing.T) {
	// Engineer has 3 pool entries at level 1 and 1 slot.
	// Nerd archetype has 5 pool entries at level 1 and 2 slots.
	// Total slots from session: 4.  Without archetype pool only 3 options exist → error.
	job := &ruleset.Job{
		ID: "engineer",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "rapid_forge_module", Level: 1},
					{ID: "electroconductive_coating", Level: 1},
					{ID: "servo_cable_actuator", Level: 1},
				},
			},
		},
	}
	archetype := &ruleset.Archetype{
		ID: "nerd",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				Pool: []ruleset.PreparedEntry{
					{ID: "nerd_tech_1", Level: 1},
					{ID: "nerd_tech_2", Level: 1},
					{ID: "nerd_tech_3", Level: 1},
					{ID: "nerd_tech_4", Level: 1},
					{ID: "nerd_tech_5", Level: 1},
				},
			},
		},
	}

	// Session has 4 slots at level 1 (job 1 + archetype 2 + level-up 1).
	sess := &session.PlayerSession{
		Level: 2,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "old_tech_a"},
				{TechID: "old_tech_b"},
				{TechID: "old_tech_c"},
				{TechID: "old_tech_d"},
			},
		},
	}

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {
			{TechID: "old_tech_a"},
			{TechID: "old_tech_b"},
			{TechID: "old_tech_c"},
			{TechID: "old_tech_d"},
		},
	}}

	promptCallCount := 0
	promptFn := func(options []string) (string, error) {
		promptCallCount++
		if len(options) == 0 {
			return "", fmt.Errorf("empty options on prompt call %d", promptCallCount)
		}
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, archetype, nil, promptFn, prep, nil, technology.TraditionFlavor{})
	require.NoError(t, err, "RearrangePreparedTechs must not fail when archetype pool entries are available")
	assert.Equal(t, 4, len(sess.PreparedTechs[1]), "all 4 slots must be filled")
	assert.Equal(t, 4, promptCallCount, "prompt must fire once per open slot")
}
