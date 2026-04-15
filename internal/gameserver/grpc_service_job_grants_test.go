package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func buildJobGrantsService(t *testing.T, job *ruleset.Job, feats []*ruleset.Feat) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	featReg := ruleset.NewFeatRegistry(feats)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, jobReg, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, featReg, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleJobGrants_ReturnsFeatAndTechGrants verifies the handler returns fixed feat grants
// and hardwired tech grants from the job definition.
func TestHandleJobGrants_ReturnsFeatAndTechGrants(t *testing.T) {
	const uid = "jg-player"

	job := &ruleset.Job{
		ID:   "scout",
		Name: "Scout",
		FeatGrants: &ruleset.FeatGrants{
			Fixed: []string{"snap_shot"},
		},
		TechnologyGrants: &ruleset.TechnologyGrants{
			Hardwired: []string{"thermal_vision"},
		},
	}
	feats := []*ruleset.Feat{
		{ID: "snap_shot", Name: "Snap Shot"},
	}

	svc, sessMgr := buildJobGrantsService(t, job, feats)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Scout", CharName: "Scout",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "scout"

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr, "expected JobGrantsResponse")

	require.Len(t, gr.FeatGrants, 1)
	assert.Equal(t, "snap_shot", gr.FeatGrants[0].FeatId)
	assert.Equal(t, "Snap Shot", gr.FeatGrants[0].FeatName)
	assert.Equal(t, int32(1), gr.FeatGrants[0].GrantLevel)

	require.Len(t, gr.TechGrants, 1)
	assert.Equal(t, "thermal_vision", gr.TechGrants[0].TechId)
	assert.Equal(t, "hardwired", gr.TechGrants[0].TechType)
	assert.Equal(t, int32(1), gr.TechGrants[0].GrantLevel)
}

// TestHandleJobGrants_LevelUpGrantsIncludedWithCorrectLevel verifies that level-up feat
// and tech grants are returned with their correct grant levels.
func TestHandleJobGrants_LevelUpGrantsIncludedWithCorrectLevel(t *testing.T) {
	const uid = "jg-levelup"

	job := &ruleset.Job{
		ID:   "infiltrator",
		Name: "Infiltrator",
		FeatGrants: &ruleset.FeatGrants{
			Fixed: []string{"cover_art"},
		},
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			5:  {Fixed: []string{"shadow_step"}},
			10: {Fixed: []string{"vanish"}},
		},
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {Hardwired: []string{"chameleon_skin"}},
		},
	}
	feats := []*ruleset.Feat{
		{ID: "cover_art", Name: "Cover Art"},
		{ID: "shadow_step", Name: "Shadow Step"},
		{ID: "vanish", Name: "Vanish"},
	}

	svc, sessMgr := buildJobGrantsService(t, job, feats)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Infiltrator", CharName: "Infiltrator",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "infiltrator"
	sess.Level = 10 // must be at least 10 to see level-10 grants

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	// Collect grant levels by feat id.
	levels := map[string]int32{}
	for _, fg := range gr.FeatGrants {
		levels[fg.FeatId] = fg.GrantLevel
	}
	assert.Equal(t, int32(1), levels["cover_art"], "cover_art should be at level 1")
	assert.Equal(t, int32(5), levels["shadow_step"], "shadow_step should be at level 5")
	assert.Equal(t, int32(10), levels["vanish"], "vanish should be at level 10")

	// Check tech level-up grant.
	require.Len(t, gr.TechGrants, 1)
	assert.Equal(t, "chameleon_skin", gr.TechGrants[0].TechId)
	assert.Equal(t, int32(3), gr.TechGrants[0].GrantLevel)
}

// buildJobGrantsServiceWithArchetype constructs a GameServiceServer with both a job and an archetype wired.
func buildJobGrantsServiceWithArchetype(t *testing.T, job *ruleset.Job, arch *ruleset.Archetype, feats []*ruleset.Feat) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	featReg := ruleset.NewFeatRegistry(feats)
	archetypes := map[string]*ruleset.Archetype{arch.ID: arch}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, jobReg, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, featReg, nil,
		nil, nil, nil, nil, nil, archetypes, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// TestHandleJobGrants_ArchetypeLevelUpFeatGrants_IncludedInResponse_REQ_BUG30 verifies that
// the archetype's level-up feat grants (choices) appear in the JobGrantsResponse at the
// correct grant levels. This is the root cause of bug #30.
//
// Precondition: job has archetype="aggressor"; archetype has level_up_feat_grants at levels 2 and 4.
// Postcondition: response includes two choice grants at levels 2 and 4.
func TestHandleJobGrants_ArchetypeLevelUpFeatGrants_IncludedInResponse_REQ_BUG30(t *testing.T) {
	const uid = "jg-archetype"

	job := &ruleset.Job{
		ID:        "brawler",
		Name:      "Brawler",
		Archetype: "aggressor",
		FeatGrants: &ruleset.FeatGrants{
			Fixed: []string{"sucker_punch"},
		},
	}
	arch := &ruleset.Archetype{
		ID:   "aggressor",
		Name: "Aggressor",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Count: 1, Pool: []string{"rage", "overpower"}}},
			4: {Choices: &ruleset.FeatChoices{Count: 1, Pool: []string{"rage", "overpower"}}},
		},
	}
	feats := []*ruleset.Feat{
		{ID: "sucker_punch", Name: "Sucker Punch"},
		{ID: "rage", Name: "Rage"},
		{ID: "overpower", Name: "Overpower"},
	}

	svc, sessMgr := buildJobGrantsServiceWithArchetype(t, job, arch, feats)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Brawler", CharName: "Brawler",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "brawler"
	sess.Level = 4 // must be at least 4 to see level-2 and level-4 grants

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr, "expected JobGrantsResponse")

	// Should have: level-1 fixed (sucker_punch) + level-2 choice + level-4 choice = 3.
	require.Len(t, gr.FeatGrants, 3, "expected 3 feat grants: 1 fixed + 2 choice-level grants")

	// Level 1 fixed.
	assert.Equal(t, int32(1), gr.FeatGrants[0].GrantLevel)
	assert.Equal(t, "sucker_punch", gr.FeatGrants[0].FeatId)

	// Level 2 choice — grant row is now a structural placeholder; details live in PendingFeatChoices.
	assert.Equal(t, int32(2), gr.FeatGrants[1].GrantLevel)
	assert.Empty(t, gr.FeatGrants[1].FeatId, "choice grant has no fixed feat_id (REQ-BUG30)")
	assert.Empty(t, gr.FeatGrants[1].FeatName, "choice grant row must have empty feat_name (REQ-FCM-2)")

	// Level 4 choice — same structural placeholder.
	assert.Equal(t, int32(4), gr.FeatGrants[2].GrantLevel)
	assert.Empty(t, gr.FeatGrants[2].FeatId, "choice grant has no fixed feat_id (REQ-BUG30)")
	assert.Empty(t, gr.FeatGrants[2].FeatName, "choice grant row must have empty feat_name (REQ-FCM-2)")

	// PendingFeatChoices must have entries for levels 2 and 4.
	require.Len(t, gr.PendingFeatChoices, 2, "REQ-FCM-1: must have 2 PendingFeatChoices for 2 unresolved pools")
	pfcByLevel := map[int32]struct{ Count int32; OptionIDs []string }{}
	for _, pfc := range gr.PendingFeatChoices {
		ids := make([]string, len(pfc.Options))
		for i, opt := range pfc.Options {
			ids[i] = opt.FeatId
		}
		pfcByLevel[pfc.GrantLevel] = struct{ Count int32; OptionIDs []string }{Count: pfc.Count, OptionIDs: ids}
	}
	assert.Contains(t, pfcByLevel, int32(2), "REQ-FCM-1: PendingFeatChoices must include level 2")
	assert.Equal(t, int32(1), pfcByLevel[int32(2)].Count, "REQ-FCM-1: count must be 1 at level 2")
	assert.ElementsMatch(t, []string{"rage", "overpower"}, pfcByLevel[int32(2)].OptionIDs, "REQ-FCM-1: level-2 options must be rage and overpower")
	assert.Contains(t, pfcByLevel, int32(4), "REQ-FCM-1: PendingFeatChoices must include level 4")
	assert.ElementsMatch(t, []string{"rage", "overpower"}, pfcByLevel[int32(4)].OptionIDs, "REQ-FCM-1: level-4 options must be rage and overpower")
}

// TestHandleJobGrants_GeneralCount_IncludedAsGrant verifies that a feat grant with
// GeneralCount > 0 is rendered as a descriptive grant entry (not a fixed feat_id).
func TestHandleJobGrants_GeneralCount_IncludedAsGrant(t *testing.T) {
	const uid = "jg-general"

	job := &ruleset.Job{
		ID:   "anarchist",
		Name: "Anarchist",
		FeatGrants: &ruleset.FeatGrants{
			GeneralCount: 1,
			Fixed:        []string{"street_knowledge"},
		},
	}
	feats := []*ruleset.Feat{{ID: "street_knowledge", Name: "Street Knowledge"}}

	svc, sessMgr := buildJobGrantsService(t, job, feats)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Anarchist", CharName: "Anarchist",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "anarchist"

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	// Should have: street_knowledge (fixed) + general feat grant = 2.
	require.Len(t, gr.FeatGrants, 2, "expected fixed + general-count grant")

	var hasGeneral bool
	for _, fg := range gr.FeatGrants {
		if fg.FeatId == "" && strings.Contains(fg.FeatName, "general feat") {
			hasGeneral = true
		}
	}
	assert.True(t, hasGeneral, "response must include a general feat grant entry")
}

// TestHandleJobGrants_GeneralCount_ResolvesToPlayerFeat verifies that when a player has
// an unattributed feat in their character sheet, the general-count slot resolves to that
// feat rather than showing the "Choose N general feat" label.
func TestHandleJobGrants_GeneralCount_ResolvesToPlayerFeat(t *testing.T) {
	const uid = "jg-general-resolved"
	const generalFeatID = "street_knowledge"

	job := &ruleset.Job{
		ID:   "fixer",
		Name: "Fixer",
		FeatGrants: &ruleset.FeatGrants{
			GeneralCount: 1,
			Fixed:        []string{"under_the_radar"},
		},
	}
	feats := []*ruleset.Feat{
		{ID: "under_the_radar", Name: "Under the Radar"},
		{ID: generalFeatID, Name: "Street Knowledge"},
	}

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	featReg := ruleset.NewFeatRegistry(feats)
	// featsRepo returns both feats for characterID 0.
	featsRepo := &stubFeatsRepo{data: map[int64][]string{
		0: {"under_the_radar", generalFeatID},
	}}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, jobReg, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, featReg, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Fixer", CharName: "Fixer",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "fixer"

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	// Should have: under_the_radar (fixed) + street_knowledge (resolved general) = 2.
	require.Len(t, gr.FeatGrants, 2, "expected fixed + resolved general feat")

	var foundGeneral bool
	for _, fg := range gr.FeatGrants {
		if fg.FeatId == generalFeatID {
			foundGeneral = true
			assert.Equal(t, "Street Knowledge", fg.FeatName)
			assert.Equal(t, int32(1), fg.GrantLevel)
		}
	}
	assert.True(t, foundGeneral, "general-count slot must resolve to the player's actual feat, not a label")
	// Ensure no label entries remain.
	for _, fg := range gr.FeatGrants {
		assert.NotContains(t, fg.FeatName, "general feat", "label must not appear when feat is resolved")
	}
}

// TestHandleJobGrants_GeneralCount_DoesNotConsumeChoicePoolFeats verifies that feats belonging
// to archetype/job choice pools are NOT attributed to general-count slots, ensuring they remain
// available for their own choice slots at higher levels.
func TestHandleJobGrants_GeneralCount_DoesNotConsumeChoicePoolFeats(t *testing.T) {
	const uid = "jg-pool-exclusion"
	const generalFeatID = "street_smarts" // not in any choice pool

	job := &ruleset.Job{
		ID:        "enforcer",
		Name:      "Enforcer",
		Archetype: "bruiser",
		FeatGrants: &ruleset.FeatGrants{
			GeneralCount: 1,
			Fixed:        []string{"iron_fist"},
		},
	}
	arch := &ruleset.Archetype{
		ID:   "bruiser",
		Name: "Bruiser",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Count: 1, Pool: []string{"rage", "overpower"}}},
		},
	}
	feats := []*ruleset.Feat{
		{ID: "iron_fist", Name: "Iron Fist"},
		{ID: "rage", Name: "Rage"},
		{ID: "overpower", Name: "Overpower"},
		{ID: generalFeatID, Name: "Street Smarts"},
	}

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	featReg := ruleset.NewFeatRegistry(feats)
	archetypes := map[string]*ruleset.Archetype{arch.ID: arch}
	// Player has iron_fist (fixed), rage (choice pool), and street_smarts (general).
	featsRepo := &stubFeatsRepo{data: map[int64][]string{
		0: {"iron_fist", "rage", generalFeatID},
	}}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, jobReg, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, featReg, featsRepo,
		nil, nil, nil, nil, nil, archetypes, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Enforcer", CharName: "Enforcer",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "enforcer"
	sess.Level = 2

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	byLevel := map[int32][]*gamev1.JobFeatGrant{}
	for _, fg := range gr.FeatGrants {
		byLevel[fg.GrantLevel] = append(byLevel[fg.GrantLevel], fg)
	}

	// Level 1: iron_fist (fixed) + street_smarts (general — NOT rage which is in pool).
	require.Len(t, byLevel[1], 2, "level 1 must have fixed + general feat")
	ids1 := map[string]bool{}
	for _, fg := range byLevel[1] {
		ids1[fg.FeatId] = true
	}
	assert.True(t, ids1["iron_fist"], "fixed feat must be at level 1")
	assert.True(t, ids1[generalFeatID], "general feat must resolve to street_smarts, not a pool feat")
	assert.False(t, ids1["rage"], "rage is a pool feat and must NOT be claimed by general slot")

	// Level 2: rage (resolved from choice pool).
	require.Len(t, byLevel[2], 1, "level 2 must have one choice grant")
	assert.Equal(t, "rage", byLevel[2][0].FeatId, "choice pool must resolve to rage at level 2")
}

// TestHandleJobGrants_NoJobReturnsMessage verifies that a player with no job class gets a message.
func TestHandleJobGrants_NoJobReturnsMessage(t *testing.T) {
	const uid = "jg-nojob"
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Drifter", CharName: "Drifter",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "" // no job

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.GetMessage(), "expected a message event when no job is set")
}

// TestHandleJobGrants_PreparedSlotsByLevel_EmittedAsSyntheticGrants verifies that
// PreparedGrants.SlotsByLevel entries are emitted as synthetic prepared_slot JobTechGrant entries.
//
// Precondition: job TechnologyGrants.Prepared.SlotsByLevel = {1: 2, 2: 1}, player level = 1.
// Postcondition: response contains 2 prepared_slot entries with correct TechName, TechLevel, TechId, GrantLevel.
func TestHandleJobGrants_PreparedSlotsByLevel_EmittedAsSyntheticGrants(t *testing.T) {
	const uid = "jg-prep-slots"

	job := &ruleset.Job{
		ID:   "techie",
		Name: "Techie",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{
					1: 2,
					2: 1,
				},
			},
		},
	}

	svc, sessMgr := buildJobGrantsService(t, job, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Techie", CharName: "Techie",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "techie"

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	// Filter to prepared_slot grants only.
	var slotGrants []*gamev1.JobTechGrant
	for _, tg := range gr.TechGrants {
		if tg.TechType == "prepared_slot" {
			slotGrants = append(slotGrants, tg)
		}
	}
	require.Len(t, slotGrants, 2, "expected 2 prepared_slot grants")

	// Grants are emitted sorted by tech level.
	assert.Equal(t, int32(1), slotGrants[0].TechLevel)
	assert.Equal(t, "+2 Prepared Slot (Level 1 tech)", slotGrants[0].TechName)
	assert.Empty(t, slotGrants[0].TechId, "synthetic grant has no TechId")
	assert.Equal(t, int32(1), slotGrants[0].GrantLevel)

	assert.Equal(t, int32(2), slotGrants[1].TechLevel)
	assert.Equal(t, "+1 Prepared Slot (Level 2 tech)", slotGrants[1].TechName)
	assert.Empty(t, slotGrants[1].TechId, "synthetic grant has no TechId")
	assert.Equal(t, int32(1), slotGrants[1].GrantLevel)
}

// TestHandleJobGrants_SpontaneousUsesByLevel_EmittedAsSyntheticGrants verifies that
// SpontaneousGrants.UsesByLevel entries are emitted as synthetic spontaneous_use JobTechGrant entries.
//
// Precondition: job TechnologyGrants.Spontaneous.UsesByLevel = {1: 3}, player level = 1.
// Postcondition: response contains 1 spontaneous_use entry with correct TechName, TechLevel, TechId, GrantLevel.
func TestHandleJobGrants_SpontaneousUsesByLevel_EmittedAsSyntheticGrants(t *testing.T) {
	const uid = "jg-spont-uses"

	job := &ruleset.Job{
		ID:   "hacker",
		Name: "Hacker",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Spontaneous: &ruleset.SpontaneousGrants{
				UsesByLevel: map[int]int{
					1: 3,
				},
			},
		},
	}

	svc, sessMgr := buildJobGrantsService(t, job, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Hacker", CharName: "Hacker",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "hacker"

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	var useGrants []*gamev1.JobTechGrant
	for _, tg := range gr.TechGrants {
		if tg.TechType == "spontaneous_use" {
			useGrants = append(useGrants, tg)
		}
	}
	require.Len(t, useGrants, 1, "expected 1 spontaneous_use grant")

	assert.Equal(t, "+3 Use (Level 1 tech)", useGrants[0].TechName)
	assert.Equal(t, int32(1), useGrants[0].TechLevel)
	assert.Empty(t, useGrants[0].TechId, "synthetic grant has no TechId")
	assert.Equal(t, int32(1), useGrants[0].GrantLevel)
}

// TestHandleJobGrants_LevelUp_SlotAndUseGrantsAtCorrectLevel verifies that slot and use grants
// from LevelUpGrants carry the correct GrantLevel matching the level-up tier they belong to.
//
// Precondition: LevelUpGrants[3].Prepared.SlotsByLevel = {2: 1}, LevelUpGrants[5].Spontaneous.UsesByLevel = {1: 2}.
// Player level = 5 (sees both level-3 and level-5 grants).
// Postcondition: prepared_slot entry has GrantLevel=3; spontaneous_use entry has GrantLevel=5.
func TestHandleJobGrants_LevelUp_SlotAndUseGrantsAtCorrectLevel(t *testing.T) {
	const uid = "jg-levelup-slots"

	job := &ruleset.Job{
		ID:   "engineer",
		Name: "Engineer",
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: {
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{2: 1},
				},
			},
			5: {
				Spontaneous: &ruleset.SpontaneousGrants{
					UsesByLevel: map[int]int{1: 2},
				},
			},
		},
	}

	svc, sessMgr := buildJobGrantsService(t, job, nil)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Engineer", CharName: "Engineer",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "engineer"
	sess.Level = 5

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr)

	var slotGrant, useGrant *gamev1.JobTechGrant
	for _, tg := range gr.TechGrants {
		switch tg.TechType {
		case "prepared_slot":
			slotGrant = tg
		case "spontaneous_use":
			useGrant = tg
		}
	}

	require.NotNil(t, slotGrant, "expected a prepared_slot grant")
	assert.Equal(t, int32(3), slotGrant.GrantLevel, "prepared_slot should carry GrantLevel=3")

	require.NotNil(t, useGrant, "expected a spontaneous_use grant")
	assert.Equal(t, int32(5), useGrant.GrantLevel, "spontaneous_use should carry GrantLevel=5")
}
