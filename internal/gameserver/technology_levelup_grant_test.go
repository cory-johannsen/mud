package gameserver

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ---------------------------------------------------------------------------
// Test doubles (local; mirrors the ones in technology_assignment_test.go but
// package-internal so they can be used with the unexported handleGrant).
// ---------------------------------------------------------------------------

type lutHardwiredRepo struct{ stored []string }

func (r *lutHardwiredRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	return r.stored, nil
}
func (r *lutHardwiredRepo) SetAll(_ context.Context, _ int64, ids []string) error {
	r.stored = ids
	return nil
}

type lutPreparedRepo struct{ slots map[int][]*session.PreparedSlot }

func (r *lutPreparedRepo) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *lutPreparedRepo) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *lutPreparedRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *lutPreparedRepo) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	return nil
}

type lutSpontaneousRepo struct{ techs map[int][]string }

func (r *lutSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	return r.techs, nil
}
func (r *lutSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *lutSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error {
	r.techs = nil
	return nil
}

type lutInnateRepo struct{ slots map[string]*session.InnateSlot }

func (r *lutInnateRepo) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	return r.slots, nil
}
func (r *lutInnateRepo) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses}
	return nil
}
func (r *lutInnateRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }

// ---------------------------------------------------------------------------
// REQ-LUT7: handleGrant applies LevelUpTechnologies for each level gained in
// ascending order when XP grant causes multi-level advancement.
// ---------------------------------------------------------------------------

// TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel verifies that when a
// target advances multiple levels in one XP grant, LevelUpTechnologies is
// applied for each new level in ascending order.
//
// Precondition: target starts at level 2 with 0 XP; job has distinct hardwired
// tech grants at levels 3 and 4; 1600 XP is granted (sufficient to reach level 4
// since 4²×100=1600).
// Postcondition: sess.HardwiredTechs contains level3_tech before level4_tech,
// confirming ascending-order application.
func TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel(t *testing.T) {
	charSaver := &grantCharSaver{}
	progressRepo := &grantProgressRepo{}
	svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})

	// Wire XP service.
	xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
	svc.SetXPService(xpSvc)

	// Build a job registry with level-up grants at levels 3 and 4.
	jobReg := ruleset.NewJobRegistry()
	testJob := &ruleset.Job{
		ID:   "test_job_lut7",
		Name: "Test Job LUT7",
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Hardwired: []string{"level3_tech"}},
			4: {Hardwired: []string{"level4_tech"}},
		},
	}
	jobReg.Register(testJob)
	svc.SetJobRegistry(jobReg)

	// Wire tech repos.
	hw := &lutHardwiredRepo{}
	svc.SetHardwiredTechRepo(hw)
	svc.SetPreparedTechRepo(&lutPreparedRepo{})
	svc.SetSpontaneousTechRepo(&lutSpontaneousRepo{})
	svc.SetInnateTechRepo(&lutInnateRepo{})

	// Add editor and target; target starts at level 2 with CharacterID and Class set.
	addEditorForGrant(t, svc, "editor_lut7")
	target := addTargetForGrant(t, svc, "target_lut7", "LUT7Char")
	target.Level = 2
	target.Experience = 0
	target.CharacterID = 42
	target.Class = "test_job_lut7"

	// Grant 1600 XP: with BaseXP=100, level N requires N²×100 cumulative XP.
	// From level 2: need 4²×100=1600 total to reach level 4.
	evt, err := svc.handleGrant("editor_lut7", &gamev1.GrantRequest{
		GrantType: "xp",
		CharName:  "LUT7Char",
		Amount:    1600,
	})

	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.NotNil(t, evt.GetMessage(), "expected success MessageEvent")
	assert.Equal(t, 4, target.Level, "target must reach level 4")

	// level3_tech must appear before level4_tech (ascending order).
	require.GreaterOrEqual(t, len(hw.stored), 2, "both level3_tech and level4_tech must be granted")
	assert.Equal(t, "level3_tech", hw.stored[0], "level3_tech must be granted first (ascending order)")
	assert.Equal(t, "level4_tech", hw.stored[1], "level4_tech must be granted second")
}

// TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel_Property is the
// property-based companion to the table test above.
//
// Property: for any starting level L in [1,8] and target level T in [L+1, L+3],
// the hardwired techs granted equal exactly the union of grants for each level
// in [L+1..T] in ascending order, with no duplicates from already-held techs.
//
// Precondition: job has a distinct hardwired tech at each level 2-11.
// Postcondition: hw.stored matches expected slice derived from levelUp range.
func TestHandleGrant_LevelUp_AppliesTechGrantsForEachLevel_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		startLevel := rapid.IntRange(1, 8).Draw(rt, "startLevel")
		levelsGained := rapid.IntRange(1, 3).Draw(rt, "levelsGained")
		targetLevel := startLevel + levelsGained

		charSaver := &grantCharSaver{}
		progressRepo := &grantProgressRepo{}
		svc := testServiceForGrant(t, grantTestOptions{charSaver: charSaver, progressRepo: progressRepo})

		xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
		svc.SetXPService(xpSvc)

		// Build job with distinct tech at each level 2-11.
		levelGrants := make(map[int]*ruleset.TechnologyGrants)
		expectedTechs := make([]string, 0, levelsGained)
		for lvl := startLevel + 1; lvl <= targetLevel; lvl++ {
			techID := "prop_tech_level_" + strconv.Itoa(lvl)
			levelGrants[lvl] = &ruleset.TechnologyGrants{Hardwired: []string{techID}}
			expectedTechs = append(expectedTechs, techID)
		}
		jobReg := ruleset.NewJobRegistry()
		jobReg.Register(&ruleset.Job{
			ID:            "prop_job",
			Name:          "Prop Job",
			LevelUpGrants: levelGrants,
		})
		svc.SetJobRegistry(jobReg)

		hw := &lutHardwiredRepo{}
		svc.SetHardwiredTechRepo(hw)
		svc.SetPreparedTechRepo(&lutPreparedRepo{})
		svc.SetSpontaneousTechRepo(&lutSpontaneousRepo{})
		svc.SetInnateTechRepo(&lutInnateRepo{})

		addEditorForGrant(t, svc, "prop_editor")
		target := addTargetForGrant(t, svc, "prop_target", "PropChar")
		target.Level = startLevel
		target.Experience = 0
		target.CharacterID = 99
		target.Class = "prop_job"

		// XP needed to reach targetLevel: targetLevel²×100 cumulative.
		neededXP := targetLevel * targetLevel * 100

		_, err := svc.handleGrant("prop_editor", &gamev1.GrantRequest{
			GrantType: "xp",
			CharName:  "PropChar",
			Amount:    int32(neededXP),
		})
		require.NoError(rt, err)

		assert.Equal(rt, targetLevel, target.Level, "target must reach expected level")
		assert.Equal(rt, expectedTechs, hw.stored, "hardwired techs must match expected ascending order")
	})
}

