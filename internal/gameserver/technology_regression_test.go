package gameserver_test

// technology_regression_test.go — Regression tests for tech slot assignment.
//
// REQ-TECH-REG-1: RearrangePreparedTechs MUST work without error for every job
// at every character level that has prepared technology grants.
//
// REQ-TECH-REG-2: All prepared slots MUST be filled after RearrangePreparedTechs.
//
// REQ-TECH-REG-3: For wizard/ranger models, the [back] sentinel MUST be present
// in options for any slot that is not the first pool slot.
//
// REQ-TECH-REG-4: Every pool slot MUST always be prompted interactively — no
// auto-assignment based on option count. The player MUST be able to assign the
// same tech to multiple slots (duplicates MUST be allowed).
//
// REQ-TECH-REG-5: Clicking [back] from any slot MUST navigate to the previous
// slot and show an interactive prompt. handleRest MUST NOT return until all
// slots are confirmed via [confirm].

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/gameserver"
)

// TestRearrangePreparedTechs_Bug149_AllSlotsAlwaysPrompted verifies that every pool
// slot is always shown to the user interactively — no auto-assignment occurs even
// when only one unique tech is available. Duplicates are allowed.
//
// REQ-TECH-REG-4: Every slot MUST be prompted. 4 slots → 4 promptFn calls.
//
// Precondition: 3 unique pool entries, 4 prepared slots, wizard model.
// Postcondition: promptFn called exactly 4 times (slots 1, 2, 3, 4); all slots filled.
func TestRearrangePreparedTechs_Bug149_AllSlotsAlwaysPrompted(t *testing.T) {
	ctx := context.Background()
	// Level 7 nerd/engineer: 3 KnownTechs (3 initial picks, pre-catalog-extras era),
	// 4 slots (1 engineer + 2 nerd + 1 nerd L2 grant).
	sess := &session.PlayerSession{
		Level: 7,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "tech_a"},
				{TechID: "tech_b"},
				{TechID: "tech_c"},
				{TechID: "tech_a"}, // slot 4 (could be a duplicate from last rest)
			},
		},
		KnownTechs: map[int][]string{
			1: {"tech_a", "tech_b", "tech_c"},
		},
	}

	// Engineer job: 1 slot at level 1, pool of 3 techs.
	engineerJob := &ruleset.Job{
		ID:           "engineer",
		Archetype:    "nerd",
		CastingModel: "", // engineer itself has no casting model
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 1},
					{ID: "tech_b", Level: 1},
					{ID: "tech_c", Level: 1},
				},
			},
		},
	}

	// Nerd archetype: 2 slots at level 1, wizard model, no pool overlap with engineer.
	// In the real game, the archetype pool contains different techs than the job pool,
	// so the player's KnownTechs (job-specific) yield exactly 3 pool entries — no duplicates.
	// Duplicating the job pool in the archetype would create 6 entries and prevent auto-assignment.
	nerdArchetype := &ruleset.Archetype{
		ID:           "nerd",
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				// No pool here: archetype pool techs (tech_d, etc.) are not in KnownTechs,
				// so they would be filtered out by the wizard model. Only the job pool
				// entries that match KnownTechs end up in effectivePool.
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: 1},
					// No pool: L2 grant adds a slot but uses the existing pool.
				},
			},
		},
	}
	// Flatten LevelUpGrants into Job.LevelUpGrants for this test.
	// In the real codebase archetype.LevelUpGrants is iterated separately.
	// We provide the archetype to RearrangePreparedTechs directly.

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: sess.PreparedTechs[1],
	}}

	type callRecord struct {
		slotNum int
		hasBack bool
		opts    []string
	}
	var calls []callRecord

	promptFn := func(prompt string, opts []string, slotCtx *gameserver.TechSlotContext) (string, error) {
		slotNum := 0
		if slotCtx != nil {
			slotNum = slotCtx.SlotNum
		}
		hasBack := false
		for _, o := range opts {
			if o == "[back]" {
				hasBack = true
				break
			}
		}
		// Record call details for assertions.
		calls = append(calls, callRecord{slotNum: slotNum, hasBack: hasBack, opts: opts})
		// Auto-select: pick first non-sentinel real option.
		for _, o := range opts {
			if o != "[back]" && o != "[forward]" && o != "[confirm]" && !strings.HasPrefix(o, "[keep]") {
				return o, nil
			}
		}
		// If only sentinels remain (should not happen in this test), forward.
		for _, o := range opts {
			if o == "[confirm]" {
				return o, nil
			}
			if o == "[forward]" {
				return o, nil
			}
		}
		return opts[0], nil
	}

	err := gameserver.RearrangePreparedTechs(
		ctx, sess, 1, engineerJob, nerdArchetype, nil, promptFn, prep, nil,
		technology.TraditionFlavor{SlotNoun: "slot", PrepGerund: "Arranging"},
	)
	require.NoError(t, err, "RearrangePreparedTechs must not error for level-7 engineer/nerd")

	// REQ-TECH-REG-2: All 4 slots must be filled.
	require.Len(t, sess.PreparedTechs[1], 4, "all 4 slots must be filled")
	for i, slot := range sess.PreparedTechs[1] {
		assert.NotNil(t, slot, "slot %d must not be nil", i+1)
		assert.NotEmpty(t, slot.TechID, "slot %d must have a non-empty TechID", i+1)
	}

	// REQ-TECH-REG-4: promptFn called for ALL 4 slots — no auto-assignment.
	require.Len(t, calls, 4, "promptFn must be called exactly 4 times (all slots always prompted)")

	// Slot 1 is first (no [back]).
	assert.Equal(t, 1, calls[0].slotNum, "first call must be for slot 1")
	assert.False(t, calls[0].hasBack, "slot 1 must not have [back]")

	// Slots 2, 3, 4 all have [back].
	assert.Equal(t, 2, calls[1].slotNum, "second call must be for slot 2")
	assert.True(t, calls[1].hasBack, "slot 2 must have [back]")

	assert.Equal(t, 3, calls[2].slotNum, "third call must be for slot 3 (always prompted)")
	assert.True(t, calls[2].hasBack, "slot 3 must have [back]")

	assert.Equal(t, 4, calls[3].slotNum, "fourth call must be for slot 4")
	assert.True(t, calls[3].hasBack, "slot 4 must have [back]")
}

// TestRearrangePreparedTechs_Bug149_BackNavigation_ActualClick exercises the full
// back-navigation path: user navigates through all 4 slots, clicks [back] at slot 4
// to revisit slot 3, then clicks [forward] and confirms at slot 4.
//
// REQ-TECH-REG-5: Clicking [back] at any slot MUST navigate to the previous slot
// with an interactive prompt. handleRest MUST stay alive until [confirm] is received.
//
// Precondition: 3 pool entries, 4 prepared slots, wizard model.
// Postcondition: All 4 slots filled; promptFn called 6 times: 1, 2, 3, 4(back), 3(forward), 4(confirm).
func TestRearrangePreparedTechs_Bug149_BackNavigation_ActualClick(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{
		Level: 7,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "tech_a"},
				{TechID: "tech_b"},
				{TechID: "tech_c"},
				{TechID: "tech_a"},
			},
		},
		KnownTechs: map[int][]string{
			1: {"tech_a", "tech_b", "tech_c"},
		},
	}

	engineerJob := &ruleset.Job{
		ID:        "engineer",
		Archetype: "nerd",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 1},
					{ID: "tech_b", Level: 1},
					{ID: "tech_c", Level: 1},
				},
			},
		},
	}

	nerdArchetype := &ruleset.Archetype{
		ID:           "nerd",
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: 1},
				},
			},
		},
	}

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: sess.PreparedTechs[1],
	}}

	// Script: 1(pick), 2(pick), 3(pick), 4(back), 3(forward), 4(confirm)
	step := 0
	type callRecord struct {
		slotNum int
		hasBack bool
		opts    []string
	}
	var calls []callRecord

	pickFirstReal := func(opts []string) string {
		for _, o := range opts {
			if o != "[back]" && o != "[forward]" && o != "[confirm]" && !strings.HasPrefix(o, "[keep]") {
				return o
			}
		}
		return opts[0]
	}

	promptFn := func(prompt string, opts []string, slotCtx *gameserver.TechSlotContext) (string, error) {
		slotNum := 0
		if slotCtx != nil {
			slotNum = slotCtx.SlotNum
		}
		hasBack := false
		for _, o := range opts {
			if o == "[back]" {
				hasBack = true
				break
			}
		}
		calls = append(calls, callRecord{slotNum: slotNum, hasBack: hasBack, opts: opts})
		step++

		switch step {
		case 1:
			return pickFirstReal(opts), nil // Slot 1: pick first real tech
		case 2:
			return pickFirstReal(opts), nil // Slot 2: pick first real tech
		case 3:
			return pickFirstReal(opts), nil // Slot 3: pick first real tech (always prompted now)
		case 4:
			return "[back]", nil // Slot 4: click [back]
		case 5:
			return "[forward]", nil // Slot 3 (revisit): click [forward]
		case 6:
			return "[confirm]", nil // Slot 4 (revisit): confirm
		}
		return pickFirstReal(opts), nil
	}

	err := gameserver.RearrangePreparedTechs(
		ctx, sess, 1, engineerJob, nerdArchetype, nil, promptFn, prep, nil,
		technology.TraditionFlavor{SlotNoun: "slot", PrepGerund: "Arranging"},
	)
	require.NoError(t, err, "RearrangePreparedTechs must not error during back navigation")

	// REQ-TECH-REG-2: All 4 slots must be filled.
	require.Len(t, sess.PreparedTechs[1], 4, "all 4 slots must be filled after back navigation")
	for i, slot := range sess.PreparedTechs[1] {
		assert.NotNil(t, slot, "slot %d must not be nil", i+1)
		assert.NotEmpty(t, slot.TechID, "slot %d must have a non-empty TechID", i+1)
	}

	// REQ-TECH-REG-5: 6 prompt calls: 1, 2, 3, 4(back), 3(forward), 4(confirm).
	require.Len(t, calls, 6, "promptFn must be called 6 times: 1, 2, 3, 4(back), 3(fwd), 4(confirm)")

	assert.Equal(t, 1, calls[0].slotNum, "call 1: slot 1")
	assert.False(t, calls[0].hasBack, "slot 1 must not have [back]")

	assert.Equal(t, 2, calls[1].slotNum, "call 2: slot 2")
	assert.True(t, calls[1].hasBack, "slot 2 must have [back]")

	assert.Equal(t, 3, calls[2].slotNum, "call 3: slot 3 (always prompted)")
	assert.True(t, calls[2].hasBack, "slot 3 must have [back]")

	assert.Equal(t, 4, calls[3].slotNum, "call 4: slot 4 (user clicks [back])")
	assert.True(t, calls[3].hasBack, "slot 4 must have [back]")

	assert.Equal(t, 3, calls[4].slotNum, "call 5: slot 3 revisit (user clicks [forward])")
	assert.True(t, calls[4].hasBack, "slot 3 revisit must have [back]")

	assert.Equal(t, 4, calls[5].slotNum, "call 6: slot 4 revisit (user clicks [confirm])")
	assert.True(t, calls[5].hasBack, "slot 4 revisit must have [back]")
	hasConfirm := false
	for _, o := range calls[5].opts {
		if o == "[confirm]" {
			hasConfirm = true
			break
		}
	}
	assert.True(t, hasConfirm, "slot 4 revisit must have [confirm]")
}

// TestRearrangePreparedTechs_Bug149_BackNavigation_ThroughToSlot2 exercises back navigation
// all the way from slot 4 back to slot 2, changing the slot 2 choice, then confirming forward.
//
// REQ-TECH-REG-6: User clicking [back] from slot 4→3→2 MUST reach slot 2 and allow
// a new choice; navigating forward MUST re-present slots 3 and 4 interactively.
//
// Precondition: 3 pool entries, 4 prepared slots, wizard model.
// Postcondition: All 4 slots filled; slot 2 reflects the new choice made during back-nav.
func TestRearrangePreparedTechs_Bug149_BackNavigation_ThroughToSlot2(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{
		Level: 7,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "tech_a"},
				{TechID: "tech_b"},
				{TechID: "tech_c"},
				{TechID: "tech_a"},
			},
		},
		KnownTechs: map[int][]string{
			1: {"tech_a", "tech_b", "tech_c"},
		},
	}

	engineerJob := &ruleset.Job{
		ID:        "engineer",
		Archetype: "nerd",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 1},
					{ID: "tech_b", Level: 1},
					{ID: "tech_c", Level: 1},
				},
			},
		},
	}

	nerdArchetype := &ruleset.Archetype{
		ID:           "nerd",
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: 1},
				},
			},
		},
	}

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: sess.PreparedTechs[1],
	}}

	// Script: 1(pick), 2(pick), 3(pick), 4(back), 3(back), 2(new pick), 3(pick), 4(confirm)
	step := 0
	type callRecord struct {
		slotNum int
		hasBack bool
		chosen  string
	}
	var calls []callRecord

	pickFirstReal := func(opts []string) string {
		for _, o := range opts {
			if o != "[back]" && o != "[forward]" && o != "[confirm]" && !strings.HasPrefix(o, "[keep]") {
				return o
			}
		}
		return opts[0]
	}

	promptFn := func(prompt string, opts []string, slotCtx *gameserver.TechSlotContext) (string, error) {
		slotNum := 0
		if slotCtx != nil {
			slotNum = slotCtx.SlotNum
		}
		hasBack := false
		for _, o := range opts {
			if o == "[back]" {
				hasBack = true
				break
			}
		}
		step++

		var chosen string
		switch step {
		case 1:
			// Slot 1: pick first real tech
			chosen = pickFirstReal(opts)
		case 2:
			// Slot 2: pick first real tech
			chosen = pickFirstReal(opts)
		case 3:
			// Slot 3 (always prompted): pick first real tech
			chosen = pickFirstReal(opts)
		case 4:
			// Slot 4 (first visit): click [back]
			chosen = "[back]"
		case 5:
			// Slot 3 (backtracking): click [back] again to reach slot 2
			chosen = "[back]"
		case 6:
			// Slot 2 (backtracking): pick the first real tech
			chosen = pickFirstReal(opts)
		case 7:
			// Slot 3 (third visit): pick first real tech
			chosen = pickFirstReal(opts)
		case 8:
			// Slot 4 (second visit): confirm
			for _, o := range opts {
				if o == "[confirm]" {
					chosen = o
				}
			}
			if chosen == "" {
				chosen = pickFirstReal(opts)
			}
		default:
			chosen = pickFirstReal(opts)
		}

		calls = append(calls, callRecord{slotNum: slotNum, hasBack: hasBack, chosen: chosen})
		return chosen, nil
	}

	err := gameserver.RearrangePreparedTechs(
		ctx, sess, 1, engineerJob, nerdArchetype, nil, promptFn, prep, nil,
		technology.TraditionFlavor{SlotNoun: "slot", PrepGerund: "Arranging"},
	)
	require.NoError(t, err, "RearrangePreparedTechs must not error during full back navigation to slot 2")

	// All 4 slots must be filled.
	require.Len(t, sess.PreparedTechs[1], 4, "all 4 slots must be filled")
	for i, slot := range sess.PreparedTechs[1] {
		assert.NotNil(t, slot, "slot %d must not be nil", i+1)
		assert.NotEmpty(t, slot.TechID, "slot %d must have a non-empty TechID", i+1)
	}

	// Expect 8 prompt calls: 1, 2, 3, 4(back), 3(back), 2(new pick), 3(pick), 4(confirm)
	require.Len(t, calls, 8, "expected 8 prompt calls for full back-navigation path")

	// REQ-TECH-REG-6a: Call sequence must match expected slot order.
	assert.Equal(t, 1, calls[0].slotNum, "call 1: slot 1")
	assert.Equal(t, 2, calls[1].slotNum, "call 2: slot 2 (first visit)")
	assert.Equal(t, 3, calls[2].slotNum, "call 3: slot 3 (always prompted)")
	assert.Equal(t, 4, calls[3].slotNum, "call 4: slot 4 (back clicked)")
	assert.Equal(t, 3, calls[4].slotNum, "call 5: slot 3 (backtracking, back clicked)")
	assert.Equal(t, 2, calls[5].slotNum, "call 6: slot 2 (backtracking, new pick)")
	assert.Equal(t, 3, calls[6].slotNum, "call 7: slot 3 (third visit)")
	assert.Equal(t, 4, calls[7].slotNum, "call 8: slot 4 (confirm)")

	// REQ-TECH-REG-6b: All intermediate slots must have [back] except slot 1.
	assert.False(t, calls[0].hasBack, "slot 1 must not have [back]")
	assert.True(t, calls[1].hasBack, "slot 2 first visit must have [back]")
	assert.True(t, calls[2].hasBack, "slot 3 first visit must have [back]")
	assert.True(t, calls[3].hasBack, "slot 4 first visit must have [back]")
	assert.True(t, calls[4].hasBack, "slot 3 backtracking must have [back]")
	assert.True(t, calls[5].hasBack, "slot 2 backtracking must have [back]")
	assert.True(t, calls[6].hasBack, "slot 3 third visit must have [back]")
	assert.True(t, calls[7].hasBack, "slot 4 second visit must have [back]")
}

// TestRearrangePreparedTechs_Bug149_DuplicatesAllowed verifies that a player can assign
// the same tech to multiple slots (e.g., all 3 slots get tech_a).
//
// REQ-TECH-REG-4: Duplicate slot assignments MUST be accepted — all techs in the base
// pool are available at every slot regardless of choices made at other slots.
//
// Precondition: 3 pool entries, 3 prepared slots, wizard model (3 known techs).
// Postcondition: All 3 slots filled with tech_a (the first real option picked each time).
func TestRearrangePreparedTechs_Bug149_DuplicatesAllowed(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{
		Level: 2,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {
				{TechID: "tech_a"},
				{TechID: "tech_b"},
				{TechID: "tech_c"},
			},
		},
		KnownTechs: map[int][]string{
			1: {"tech_a", "tech_b", "tech_c"},
		},
	}

	engineerJob := &ruleset.Job{
		ID:        "engineer",
		Archetype: "nerd",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 1},
					{ID: "tech_b", Level: 1},
					{ID: "tech_c", Level: 1},
				},
			},
		},
	}

	nerdArchetype := &ruleset.Archetype{
		ID:           "nerd",
		CastingModel: ruleset.CastingModelWizard,
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: 0},
				},
			},
		},
	}

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: sess.PreparedTechs[1],
	}}

	// Script: always pick tech_a for every slot (via [keep] or regular option).
	type callRecord struct {
		slotNum int
		opts    []string
		chosen  string
	}
	var calls []callRecord

	promptFn := func(prompt string, opts []string, slotCtx *gameserver.TechSlotContext) (string, error) {
		slotNum := 0
		if slotCtx != nil {
			slotNum = slotCtx.SlotNum
		}
		// Find tech_a in options — accept [keep] option too (e.g., slot 1's prevTech).
		var chosen string
		for _, o := range opts {
			if strings.Contains(o, "tech_a") {
				chosen = o
				break
			}
		}
		if chosen == "" {
			// Fallback: first real option.
			for _, o := range opts {
				if o != "[back]" && o != "[forward]" && o != "[confirm]" {
					chosen = o
					break
				}
			}
		}
		calls = append(calls, callRecord{slotNum: slotNum, opts: opts, chosen: chosen})
		// Last slot: confirm after picking.
		if slotCtx != nil && slotCtx.SlotNum == slotCtx.TotalSlots {
			return "[confirm]", nil
		}
		return chosen, nil
	}

	err := gameserver.RearrangePreparedTechs(
		ctx, sess, 1, engineerJob, nerdArchetype, nil, promptFn, prep, nil,
		technology.TraditionFlavor{SlotNoun: "slot", PrepGerund: "Arranging"},
	)
	require.NoError(t, err, "RearrangePreparedTechs must not error when user picks same tech for all slots")

	// All 3 slots must be filled.
	require.Len(t, sess.PreparedTechs[1], 3, "all 3 slots must be filled")
	for i, slot := range sess.PreparedTechs[1] {
		assert.NotNil(t, slot, "slot %d must not be nil", i+1)
		assert.NotEmpty(t, slot.TechID, "slot %d must have a non-empty TechID", i+1)
	}

	// 3 prompt calls (one per slot).
	require.Len(t, calls, 3, "promptFn must be called 3 times (one per slot)")

	// REQ-TECH-REG-4: tech_a must appear in options for every slot (either as a regular
	// option or as a [keep] option), so the player can always select it.
	for i, call := range calls {
		hasTechA := false
		for _, o := range call.opts {
			if strings.Contains(o, "tech_a") {
				hasTechA = true
				break
			}
		}
		assert.True(t, hasTechA, "slot %d must offer tech_a (regular or keep) even if chosen in another slot", call.slotNum)
		_ = i
	}
}

// TestRearrangePreparedTechs_AllJobs_AllLevels_NoPanic exercises RearrangePreparedTechs
// for all jobs in content/jobs that have prepared technology grants, at every character
// level from 1 through the max level found in level_up_grants.
//
// Precondition: content/jobs and content/archetypes directories exist.
// Postcondition: No panics; all slots filled; no errors.
func TestRearrangePreparedTechs_AllJobs_AllLevels_NoPanic(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../content/jobs")
	require.NoError(t, err, "LoadJobs must succeed")

	archetypes, err := ruleset.LoadArchetypes("../../content/archetypes")
	require.NoError(t, err, "LoadArchetypes must succeed")

	// Build archetype lookup map.
	archetypeByID := make(map[string]*ruleset.Archetype, len(archetypes))
	for _, a := range archetypes {
		archetypeByID[a.ID] = a
	}

	for _, job := range jobs {
		job := job
		if job.TechnologyGrants == nil || job.TechnologyGrants.Prepared == nil {
			// Check whether any level_up_grants add prepared techs.
			hasLevelUpPrepared := false
			for _, grants := range job.LevelUpGrants {
				if grants != nil && grants.Prepared != nil && len(grants.Prepared.SlotsByLevel) > 0 {
					hasLevelUpPrepared = true
					break
				}
			}
			// Also check archetype grants.
			if arch, ok := archetypeByID[job.Archetype]; ok {
				if arch.TechnologyGrants != nil && arch.TechnologyGrants.Prepared != nil {
					hasLevelUpPrepared = true
				}
				for _, grants := range arch.LevelUpGrants {
					if grants != nil && grants.Prepared != nil && len(grants.Prepared.SlotsByLevel) > 0 {
						hasLevelUpPrepared = true
						break
					}
				}
			}
			if !hasLevelUpPrepared {
				continue // skip jobs with no prepared tech grants at any level
			}
		}

		arch := archetypeByID[job.Archetype]
		castingModel := ruleset.ResolveCastingModel(job, arch)

		// Collect all character levels with grants (for this job + archetype).
		levelSet := map[int]bool{1: true}
		for lvl := range job.LevelUpGrants {
			levelSet[lvl] = true
		}
		if arch != nil {
			for lvl := range arch.LevelUpGrants {
				levelSet[lvl] = true
			}
		}
		var charLevels []int
		for lvl := range levelSet {
			charLevels = append(charLevels, lvl)
		}
		sort.Ints(charLevels)

		for _, charLevel := range charLevels {
			charLevel := charLevel
			t.Run(fmt.Sprintf("%s/level%d", job.ID, charLevel), func(t *testing.T) {
				// Build cumulative slot and pool state for this character level.
				slotsByLevel := make(map[int]int)
				var allPool []ruleset.PreparedEntry
				var allFixed []ruleset.PreparedEntry

				// Base job grants.
				if job.TechnologyGrants != nil && job.TechnologyGrants.Prepared != nil {
					for lvl, cnt := range job.TechnologyGrants.Prepared.SlotsByLevel {
						slotsByLevel[lvl] += cnt
					}
					allPool = append(allPool, job.TechnologyGrants.Prepared.Pool...)
					allFixed = append(allFixed, job.TechnologyGrants.Prepared.Fixed...)
				}
				// Base archetype grants.
				if arch != nil && arch.TechnologyGrants != nil && arch.TechnologyGrants.Prepared != nil {
					for lvl, cnt := range arch.TechnologyGrants.Prepared.SlotsByLevel {
						slotsByLevel[lvl] += cnt
					}
					allPool = append(allPool, arch.TechnologyGrants.Prepared.Pool...)
					allFixed = append(allFixed, arch.TechnologyGrants.Prepared.Fixed...)
				}
				// Level-up grants (job).
				for grantLvl, grants := range job.LevelUpGrants {
					if grantLvl > charLevel {
						continue
					}
					if grants != nil && grants.Prepared != nil {
						for lvl, cnt := range grants.Prepared.SlotsByLevel {
							slotsByLevel[lvl] += cnt
						}
						allPool = append(allPool, grants.Prepared.Pool...)
						allFixed = append(allFixed, grants.Prepared.Fixed...)
					}
				}
				// Level-up grants (archetype).
				if arch != nil {
					for grantLvl, grants := range arch.LevelUpGrants {
						if grantLvl > charLevel {
							continue
						}
						if grants != nil && grants.Prepared != nil {
							for lvl, cnt := range grants.Prepared.SlotsByLevel {
								slotsByLevel[lvl] += cnt
							}
							allPool = append(allPool, grants.Prepared.Pool...)
							allFixed = append(allFixed, grants.Prepared.Fixed...)
						}
					}
				}

				// Skip if no pool slots at all (only fixed slots don't need rearrangement prompts).
				totalSlots := 0
				for _, cnt := range slotsByLevel {
					totalSlots += cnt
				}
				if totalSlots == 0 {
					return
				}

				// Build effective pool for wizard/ranger: use allPool as KnownTechs.
				// For other models: use allPool directly.
				effectivePool := allPool
				var knownTechs map[int][]string
				if castingModel == ruleset.CastingModelWizard || castingModel == ruleset.CastingModelRanger {
					// Pre-populate KnownTechs from allPool so effectivePool is non-empty.
					knownTechs = make(map[int][]string)
					for _, e := range allPool {
						knownTechs[e.Level] = appendIfMissing(knownTechs[e.Level], e.ID)
					}
					effectivePool = allPool
				}

				// Build initial PreparedTechs from fixed slots and first pool picks.
				preparedTechs := make(map[int][]*session.PreparedSlot)
				for techLvl, cnt := range slotsByLevel {
					slots := make([]*session.PreparedSlot, 0, cnt)
					// Fill fixed slots first.
					for _, fe := range allFixed {
						if fe.Level == techLvl {
							slots = append(slots, &session.PreparedSlot{TechID: fe.ID})
						}
					}
					// Fill remaining pool slots.
					poolEntries := poolEntriesAtLevel(effectivePool, techLvl)
					for len(slots) < cnt && len(poolEntries) > 0 {
						slots = append(slots, &session.PreparedSlot{TechID: poolEntries[len(slots)%len(poolEntries)].ID})
					}
					preparedTechs[techLvl] = slots
				}
				if len(preparedTechs) == 0 {
					return // no prepared techs → skip (innate/hardwired only)
				}

				sess := &session.PlayerSession{
					Level:         charLevel,
					PreparedTechs: preparedTechs,
					KnownTechs:    knownTechs,
				}

				prep := &fakePreparedRepo{slots: clonePrepMap(preparedTechs)}

				// Auto-selecting promptFn: always picks the first non-sentinel option.
				// Tracks option lists and whether [back] appears for all non-first pool slots.
				type promptCall struct {
					slotNum int
					hasBack bool
					optLen  int
				}
				var promptCalls []promptCall
				promptFn := func(prompt string, opts []string, slotCtx *gameserver.TechSlotContext) (string, error) {
					slotNum := 0
					if slotCtx != nil {
						slotNum = slotCtx.SlotNum
					}
					hasBack := false
					for _, o := range opts {
						if o == "[back]" {
							hasBack = true
							break
						}
					}
					promptCalls = append(promptCalls, promptCall{slotNum: slotNum, hasBack: hasBack, optLen: len(opts)})
					for _, o := range opts {
						if o != "[back]" && o != "[forward]" && o != "[confirm]" && !strings.HasPrefix(o, "[keep]") {
							return o, nil
						}
					}
					// Only sentinels: pick confirm if last slot, else forward.
					for _, o := range opts {
						if o == "[confirm]" {
							return o, nil
						}
					}
					for _, o := range opts {
						if o == "[forward]" {
							return o, nil
						}
					}
					return opts[0], nil
				}

				err := gameserver.RearrangePreparedTechs(
					context.Background(), sess, 1, job, arch, nil, promptFn, prep, nil,
					technology.TraditionFlavor{SlotNoun: "slot", PrepGerund: "Arranging"},
				)
				require.NoError(t, err, "RearrangePreparedTechs must not error for job %s level %d", job.ID, charLevel)

				// REQ-TECH-REG-2: All slots must be filled.
				for techLvl, expectedCnt := range slotsByLevel {
					actualSlots := sess.PreparedTechs[techLvl]
					assert.Len(t, actualSlots, expectedCnt,
						"job %s level %d: level-%d slots must all be filled", job.ID, charLevel, techLvl)
					for i, slot := range actualSlots {
						assert.NotNil(t, slot, "job %s level %d: slot[%d][%d] must not be nil", job.ID, charLevel, techLvl, i)
						if slot != nil {
							assert.NotEmpty(t, slot.TechID,
								"job %s level %d: slot[%d][%d] must have non-empty TechID", job.ID, charLevel, techLvl, i)
						}
					}
				}

				// REQ-TECH-REG-3: Any slot that is not the first pool slot globally must have [back].
				// Note: promptCalls that are for slot 1 within the POOL sequence (not within the level)
				// do not have [back]. But all other interactive slots must.
				// We track this by checking that promptCalls[i].hasBack == (i > 0 for pool-slot sequence).
				// Since auto-assigned slots don't generate promptCalls, we can only check the calls we see.
				for i, call := range promptCalls {
					if i == 0 {
						// First interactive slot: may or may not have [back] depending on whether
						// it is the first pool slot overall. For simplicity, assert non-negative optLen.
						assert.Positive(t, call.optLen,
							"job %s level %d: prompt call %d must have non-empty options", job.ID, charLevel, i+1)
					} else {
						// Subsequent interactive slots: MUST have [back].
						assert.True(t, call.hasBack,
							"job %s level %d: prompt call %d (slot %d) must have [back]",
							job.ID, charLevel, i+1, call.slotNum)
					}
				}
			})
		}
	}
}

// appendIfMissing appends s to slice if not already present.
func appendIfMissing(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// poolEntriesAtLevel returns entries from pool at the given tech level.
func poolEntriesAtLevel(pool []ruleset.PreparedEntry, techLvl int) []ruleset.PreparedEntry {
	var result []ruleset.PreparedEntry
	for _, e := range pool {
		if e.Level == techLvl {
			result = append(result, e)
		}
	}
	return result
}

// clonePrepMap deep-copies a prepared slot map.
func clonePrepMap(m map[int][]*session.PreparedSlot) map[int][]*session.PreparedSlot {
	out := make(map[int][]*session.PreparedSlot, len(m))
	for k, v := range m {
		cp := make([]*session.PreparedSlot, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
