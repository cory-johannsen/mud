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
// REQ-TECH-REG-4: Slot 3 auto-assignment (when only 1 option remains) MUST
// NOT prevent [back] from appearing at slot 4 for engineer/nerd (wizard model).

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

// TestRearrangePreparedTechs_Bug149_WizardBackNavigationAtSlot4 reproduces the
// exact Bug #149 scenario: level-7 nerd/engineer with wizard casting model,
// 4 level-1 slots, and 3 KnownTechs.
//
// Invariant: [back] MUST appear in slot-4 options even when slot 3 is auto-assigned.
//
// Precondition: 3 unique pool entries, 4 prepared slots, wizard model.
// Postcondition: promptFn is called for slots 1, 2, 4 (slot 3 auto-assigned);
// [back] is present in slot-4 options; all 4 slots are filled.
func TestRearrangePreparedTechs_Bug149_WizardBackNavigationAtSlot4(t *testing.T) {
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

	// REQ-TECH-REG-3: promptFn called for slots 1, 2, and 4 (slot 3 auto-assigned).
	// Exactly 3 interactive slots in this scenario.
	require.Len(t, calls, 3, "promptFn must be called exactly 3 times (slots 1, 2, 4; slot 3 auto-assigned)")

	// Slot 1 is first interactive (no [back]).
	assert.Equal(t, 1, calls[0].slotNum, "first call must be for slot 1")
	assert.False(t, calls[0].hasBack, "slot 1 must not have [back]")

	// Slot 2 is second interactive (has [back]).
	assert.Equal(t, 2, calls[1].slotNum, "second call must be for slot 2")
	assert.True(t, calls[1].hasBack, "slot 2 must have [back]")

	// REQ-TECH-REG-4: Slot 4 (3rd prompt call) MUST have [back] — this is the Bug #149 assertion.
	assert.Equal(t, 4, calls[2].slotNum, "third call must be for slot 4 (slot 3 auto-assigned)")
	assert.True(t, calls[2].hasBack, "slot 4 must have [back] when slot 3 was auto-assigned (Bug #149)")
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
