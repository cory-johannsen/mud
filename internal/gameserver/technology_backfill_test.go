package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ---------------------------------------------------------------------------
// Unit tests for BackfillLevelUpTechnologies
// ---------------------------------------------------------------------------

func makeTestSess(level int) *session.PlayerSession {
	return &session.PlayerSession{
		Level: level,
	}
}

// TestBackfill_L2SlotsAlreadyPopulated_Preserved verifies that existing L2+ prepared
// slots are preserved across login and not registered as pending grants when the
// expected slot count is already met.
//
// Bug #351: previous behavior unconditionally deleted all pool-assigned L2+ slots,
// destroying legitimate trainer-resolved and rest-rearranged selections.
// REQ-TTA-2 still applies for slots that have NEVER been resolved — those become pending —
// but slots already filled by the player's choices must round-trip across logout/login.
func TestBackfill_L2SlotsAlreadyPopulated_Preserved(t *testing.T) {
	t.Parallel()
	archetype := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2}, // creation: 2 level-1 slots
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{2: 2}, // level-up: 2 level-2 slots
				Pool: []ruleset.PreparedEntry{
					{ID: "tech_a", Level: 2},
					{ID: "tech_b", Level: 2},
				},
			}},
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	// Pre-populate L2 slots representing the player's trainer-resolved or rest-prepared selections.
	prepRepo := &lutPreparedRepo{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "creation_tech_x"}, {TechID: "creation_tech_y"}}, // 2 creation slots
			2: {{TechID: "tech_a"}, {TechID: "tech_b"}},                   // legitimate L2 selections
		},
	}
	hwRepo := &lutHardwiredRepo{}
	spontRepo := &lutSpontaneousRepo{}
	innateRepo := &lutInnateRepo{}

	sess := makeTestSess(3)
	pending, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, spontRepo, innateRepo, nil,
	)
	require.NoError(t, err)
	// L2 slots must NOT have been cleared.
	assert.Len(t, prepRepo.slots[2], 2, "existing L2 slots must be preserved across login")
	assert.Equal(t, "tech_a", prepRepo.slots[2][0].TechID)
	assert.Equal(t, "tech_b", prepRepo.slots[2][1].TechID)
	// All expected slots are present, so no pending grants for L2.
	if pending != nil && pending.Prepared != nil {
		assert.Equal(t, 0, pending.Prepared.SlotsByLevel[2], "no pending L2 slots when all are filled")
	}
	// L1 creation slots must remain untouched.
	assert.Len(t, prepRepo.slots[1], 2, "L1 creation slots must be preserved")
}

// TestBackfill_SameTechAtMultipleCastLevels_Preserved is the direct regression test
// for bug #351: preparing the same tech at multiple cast levels round-trips across
// logout/login.
func TestBackfill_SameTechAtMultipleCastLevels_Preserved(t *testing.T) {
	t.Parallel()
	// Nerd-style archetype: L1 slots from creation, L2 slots from level 3, L3 slots from level 5.
	archetype := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				Pool: []ruleset.PreparedEntry{
					{ID: "multi_round_kinetic_volley", Level: 1},
				},
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{2: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "corrosive_projectile", Level: 2},
				},
			}},
			5: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{3: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "terror_frequency", Level: 3},
				},
			}},
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	// Player has prepared the same tech (multi_round_kinetic_volley) at 3 different cast levels.
	prepRepo := &lutPreparedRepo{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "multi_round_kinetic_volley"}, {TechID: "multi_round_kinetic_volley"}},
			2: {{TechID: "multi_round_kinetic_volley"}},
			3: {{TechID: "multi_round_kinetic_volley"}},
		},
	}
	hwRepo := &lutHardwiredRepo{}
	sess := makeTestSess(5)
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "multi_round_kinetic_volley"}, {TechID: "multi_round_kinetic_volley"}},
		2: {{TechID: "multi_round_kinetic_volley"}},
		3: {{TechID: "multi_round_kinetic_volley"}},
	}

	pending, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)

	// All three cast-level slots must survive.
	require.Len(t, prepRepo.slots[1], 2, "L1 slots preserved")
	require.Len(t, prepRepo.slots[2], 1, "L2 slot preserved (was lost before fix)")
	require.Len(t, prepRepo.slots[3], 1, "L3 slot preserved (was lost before fix)")
	assert.Equal(t, "multi_round_kinetic_volley", prepRepo.slots[2][0].TechID)
	assert.Equal(t, "multi_round_kinetic_volley", prepRepo.slots[3][0].TechID)
	// Session must reflect the same.
	require.Len(t, sess.PreparedTechs[2], 1, "session L2 slot preserved")
	require.Len(t, sess.PreparedTechs[3], 1, "session L3 slot preserved")
	// No pending grants because expected counts are met.
	if pending != nil && pending.Prepared != nil {
		assert.Equal(t, 0, pending.Prepared.SlotsByLevel[2])
		assert.Equal(t, 0, pending.Prepared.SlotsByLevel[3])
	}
}

// TestBackfill_MissingL2SlotsReturnedAsPending verifies that a player at level 3
// who is missing L2 prepared slots receives them as pending grants (not auto-assigned).
// REQ-TTA-2: L2+ prepared slots must go through the trainer system.
func TestBackfill_MissingL2SlotsReturnedAsPending(t *testing.T) {
	t.Parallel()
	archetype := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{2: 2},
				Pool: []ruleset.PreparedEntry{
					{ID: "spell_a", Level: 2},
					{ID: "spell_b", Level: 2},
					{ID: "spell_c", Level: 2},
				},
			}},
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	prepRepo := &lutPreparedRepo{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "creation_x"}, {TechID: "creation_y"}}, // creation slots present
			// L2 slots missing — expect pending, NOT auto-assign
		},
	}
	hwRepo := &lutHardwiredRepo{}

	sess := makeTestSess(3)
	pending, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	// L2 slots must NOT have been auto-assigned.
	assert.Empty(t, prepRepo.slots[2], "L2 slots must not be auto-assigned")
	// Pending grants must be returned.
	require.NotNil(t, pending)
	require.NotNil(t, pending.Prepared)
	assert.Equal(t, 2, pending.Prepared.SlotsByLevel[2])
	assert.Len(t, pending.Prepared.Pool, 3)
}

// TestBackfill_AppliesMissingHardwiredTechs verifies that hardwired techs from
// level_up grants are added to a character that lacks them.
func TestBackfill_AppliesMissingHardwiredTechs(t *testing.T) {
	t.Parallel()
	archetype := &ruleset.Archetype{
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {Hardwired: []string{"passive_tech_2"}},
			4: {Hardwired: []string{"passive_tech_4"}},
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	hwRepo := &lutHardwiredRepo{stored: []string{"creation_tech"}}
	prepRepo := &lutPreparedRepo{}

	sess := makeTestSess(4)
	sess.HardwiredTechs = []string{"creation_tech"}

	pending, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	assert.Nil(t, pending, "hardwired-only backfill must return nil pending grants")
	assert.Contains(t, hwRepo.stored, "passive_tech_2")
	assert.Contains(t, hwRepo.stored, "passive_tech_4")
	assert.Contains(t, hwRepo.stored, "creation_tech") // must not be removed
}

// TestBackfill_SkipsHigherLevels verifies that grants for levels above the
// character's current level are not applied.
func TestBackfill_SkipsHigherLevels(t *testing.T) {
	t.Parallel()
	archetype := &ruleset.Archetype{
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {Hardwired: []string{"tech_lvl2"}},
			5: {Hardwired: []string{"tech_lvl5"}}, // above current level
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	hwRepo := &lutHardwiredRepo{}
	sess := makeTestSess(3)

	_, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, &lutPreparedRepo{}, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	assert.Contains(t, hwRepo.stored, "tech_lvl2")
	assert.NotContains(t, hwRepo.stored, "tech_lvl5")
}

// TestProperty_Backfill_L2PlusNeverAutoAssigned verifies that L2+ prepared slots
// are never auto-assigned by the backfill — they are always returned as pending.
// REQ-TTA-2: L2+ prepared grants unconditionally require a trainer.
func TestProperty_Backfill_L2PlusNeverAutoAssigned(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(3, 8).Draw(rt, "level")
		slotsPerLevel := rapid.IntRange(1, 3).Draw(rt, "slots")
		poolSize := rapid.IntRange(slotsPerLevel, 5).Draw(rt, "poolSize")

		pool := make([]ruleset.PreparedEntry, poolSize)
		for i := range pool {
			pool[i] = ruleset.PreparedEntry{ID: "p" + string(rune('a'+i)), Level: 2}
		}

		archetype := &ruleset.Archetype{
			LevelUpGrants: map[int]*ruleset.TechnologyGrants{
				3: {Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{2: slotsPerLevel},
					Pool:         pool,
				}},
			},
		}
		job := &ruleset.Job{}
		merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

		prepRepo := &lutPreparedRepo{}
		sess := makeTestSess(level)

		ctx := context.Background()

		pending, err := BackfillLevelUpTechnologies(ctx, sess, 1, job, archetype, merged, nil,
			&lutHardwiredRepo{}, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil)
		require.NoError(rt, err)

		// L2 slots must NEVER be auto-assigned.
		assert.Empty(rt, prepRepo.slots[2], "L2+ slots must never be auto-assigned by backfill")

		// Pending grants must be returned.
		if slotsPerLevel > 0 {
			require.NotNil(rt, pending, "pending grants must be returned for L2+ slots")
			require.NotNil(rt, pending.Prepared)
			assert.Equal(rt, slotsPerLevel, pending.Prepared.SlotsByLevel[2])
		}
	})
}

// TestBackfill_L1SlotsStillAutoAssigned verifies that L1 prepared slots continue
// to be auto-assigned as before (trainer gate only applies to L2+).
func TestBackfill_L1SlotsStillAutoAssigned(t *testing.T) {
	t.Parallel()
	// Creation grant: 1 L1 slot. Level-up at charLevel 2: 1 more L1 slot.
	// Total expected L1: 2. Existing: 1 (creation). Delta: 1 → auto-assign.
	archetype := &ruleset.Archetype{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1}, // creation: 1 L1 slot
			},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1}, // extra L1 slot at charLevel 2
				Pool: []ruleset.PreparedEntry{
					{ID: "extra_l1", Level: 1},
				},
			}},
		},
	}
	job := &ruleset.Job{}
	merged := ruleset.MergeLevelUpGrants(archetype.LevelUpGrants, job.LevelUpGrants)

	prepRepo := &lutPreparedRepo{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "creation_l1"}}, // 1 existing L1 slot
		},
	}
	hwRepo := &lutHardwiredRepo{}
	sess := makeTestSess(2)

	pending, err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	// L1 slot should be auto-assigned (pool size == open slots).
	assert.Len(t, prepRepo.slots[1], 2, "L1 missing slot should be auto-assigned")
	// No pending grants for L1.
	if pending != nil && pending.Prepared != nil {
		assert.Equal(t, 0, pending.Prepared.SlotsByLevel[1], "no L1 pending slots expected")
	}
}
