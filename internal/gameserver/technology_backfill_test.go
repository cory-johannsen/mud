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

// TestBackfill_NoOpWhenAlreadyApplied verifies that a player who already has
// the correct number of prepared slots receives no additional slots.
func TestBackfill_NoOpWhenAlreadyApplied(t *testing.T) {
	t.Parallel()
	// Character level 3 with archetype level_up_grants giving 2 level-2 spell slots.
	// The player already has those 2 slots → backfill must be a no-op.
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

	prepRepo := &lutPreparedRepo{
		slots: map[int][]*session.PreparedSlot{
			1: {{TechID: "creation_tech_x"}, {TechID: "creation_tech_y"}}, // 2 creation slots
			2: {{TechID: "tech_a"}, {TechID: "tech_b"}},                   // already applied
		},
	}
	hwRepo := &lutHardwiredRepo{}
	spontRepo := &lutSpontaneousRepo{}
	innateRepo := &lutInnateRepo{}

	sess := makeTestSess(3)
	err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, spontRepo, innateRepo, nil,
	)
	require.NoError(t, err)
	// Level-2 slots must not have grown beyond 2.
	assert.Equal(t, 2, len(prepRepo.slots[2]))
}

// TestBackfill_AppliesMissingPreparedSlots verifies that a player at level 3 who
// is missing the level-3 archetype grant (2 level-2 slots) receives them.
func TestBackfill_AppliesMissingPreparedSlots(t *testing.T) {
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
			// level-2 slots missing
		},
	}
	hwRepo := &lutHardwiredRepo{}

	sess := makeTestSess(3)
	err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	// 2 level-2 slots must now exist (auto-filled from pool).
	require.Len(t, prepRepo.slots[2], 2)
	// Both should be valid pool entries.
	for _, slot := range prepRepo.slots[2] {
		assert.NotEmpty(t, slot.TechID)
	}
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

	err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
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

	err := BackfillLevelUpTechnologies(
		context.Background(), sess, 1,
		job, archetype, merged, nil,
		hwRepo, &lutPreparedRepo{}, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil,
	)
	require.NoError(t, err)
	assert.Contains(t, hwRepo.stored, "tech_lvl2")
	assert.NotContains(t, hwRepo.stored, "tech_lvl5")
}

// TestProperty_Backfill_IdempotentPreparedSlots verifies that calling
// BackfillLevelUpTechnologies twice produces the same slot count as calling it once.
func TestProperty_Backfill_IdempotentPreparedSlots(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(2, 8).Draw(rt, "level")
		slotsPerLevel := rapid.IntRange(1, 3).Draw(rt, "slots")
		poolSize := rapid.IntRange(2, 5).Draw(rt, "poolSize")

		pool := make([]ruleset.PreparedEntry, poolSize)
		for i := range pool {
			pool[i] = ruleset.PreparedEntry{ID: "p" + string(rune('a'+i)), Level: 2}
		}

		archetype := &ruleset.Archetype{
			LevelUpGrants: map[int]*ruleset.TechnologyGrants{
				2: {Prepared: &ruleset.PreparedGrants{
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

		// First call.
		err := BackfillLevelUpTechnologies(ctx, sess, 1, job, archetype, merged, nil,
			&lutHardwiredRepo{}, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil)
		require.NoError(rt, err)
		countAfterFirst := len(prepRepo.slots[2])

		// Second call (idempotent).
		sess2 := makeTestSess(level)
		sess2.PreparedTechs = sess.PreparedTechs
		err = BackfillLevelUpTechnologies(ctx, sess2, 1, job, archetype, merged, nil,
			&lutHardwiredRepo{}, prepRepo, &lutSpontaneousRepo{}, &lutInnateRepo{}, nil)
		require.NoError(rt, err)
		countAfterSecond := len(prepRepo.slots[2])

		assert.Equal(rt, countAfterFirst, countAfterSecond,
			"second call must not add more slots (idempotent)")
	})
}
