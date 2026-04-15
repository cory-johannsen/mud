package gameserver

// REQ-FCM-1: handleJobGrants MUST populate PendingFeatChoices (not a raw label FeatName)
//             when the player has not yet selected from a feat choice pool.
// REQ-FCM-2: The JobFeatGrant row for an unresolved pool MUST have empty feat_id and feat_name.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// TestHandleJobGrants_UnresolvedChoicePool_PopulatesPendingFeatChoices verifies
// that handleJobGrants returns a PendingFeatChoice (not a raw label string) when
// the player has not yet selected from a feat choice pool (REQ-FCM-1, REQ-FCM-2).
//
// Precondition: Player has no feats from the choice pool; feat registry has pool feats.
// Postcondition: JobGrantsResponse.PendingFeatChoices has one entry; FeatGrant for
//
//	that level has empty feat_id and feat_name.
func TestHandleJobGrants_UnresolvedChoicePool_PopulatesPendingFeatChoices(t *testing.T) {
	const uid = "fcm-test-player"

	feats := []*ruleset.Feat{
		{ID: "rage", Name: "Rage", Description: "You enter a furious rage.", Category: "combat"},
		{ID: "overpower", Name: "Overpower", Description: "Overwhelm your foe.", Category: "combat"},
	}

	job := &ruleset.Job{
		ID:   "brawler_fcm",
		Name: "Brawler FCM",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Count: 1, Pool: []string{"rage", "overpower"}}},
		},
	}

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	featReg := ruleset.NewFeatRegistry(feats)

	// Player has NO feats from the choice pool — featsRepo returns empty for characterID 0.
	featsRepo := &stubFeatsRepo{data: map[int64][]string{}}

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
		UID: uid, Username: "BrawlerFCM", CharName: "BrawlerFCM",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "brawler_fcm"
	sess.Level = 2

	resp, err := svc.handleJobGrants(uid)
	require.NoError(t, err)
	require.NotNil(t, resp)

	gr := resp.GetJobGrantsResponse()
	require.NotNil(t, gr, "expected JobGrantsResponse")

	// REQ-FCM-2: The feat grant row for level 2 must have empty feat_id and feat_name.
	var level2Grant *struct{ FeatId, FeatName string }
	for _, fg := range gr.FeatGrants {
		if fg.GrantLevel == 2 {
			level2Grant = &struct{ FeatId, FeatName string }{FeatId: fg.FeatId, FeatName: fg.FeatName}
			break
		}
	}
	require.NotNil(t, level2Grant, "REQ-FCM-2: must have a feat grant row at level 2 for the unresolved pool")
	assert.Empty(t, level2Grant.FeatId, "REQ-FCM-2: feat_id must be empty for unresolved choice pool")
	assert.Empty(t, level2Grant.FeatName, "REQ-FCM-2: feat_name must be empty for unresolved choice pool — raw label must not be used")

	// REQ-FCM-1: PendingFeatChoices must have exactly one entry for level 2.
	require.Len(t, gr.PendingFeatChoices, 1, "REQ-FCM-1: must have exactly 1 PendingFeatChoice for the unresolved pool")
	pfc := gr.PendingFeatChoices[0]
	assert.Equal(t, int32(2), pfc.GrantLevel, "REQ-FCM-1: PendingFeatChoice must be at grant level 2")
	assert.Equal(t, int32(1), pfc.Count, "REQ-FCM-1: Count must equal the pool's choice count")

	// Options must list both pool feats with name and description populated.
	require.Len(t, pfc.Options, 2, "REQ-FCM-1: must have 2 options matching the pool size")
	optByID := map[string]struct{ Name, Description, Category string }{}
	for _, opt := range pfc.Options {
		optByID[opt.FeatId] = struct{ Name, Description, Category string }{
			Name:        opt.Name,
			Description: opt.Description,
			Category:    opt.Category,
		}
	}
	assert.Contains(t, optByID, "rage", "REQ-FCM-1: options must include 'rage'")
	assert.Contains(t, optByID, "overpower", "REQ-FCM-1: options must include 'overpower'")
	assert.Equal(t, "Rage", optByID["rage"].Name)
	assert.Equal(t, "You enter a furious rage.", optByID["rage"].Description)
	assert.Equal(t, "combat", optByID["rage"].Category)
	assert.Equal(t, "Overpower", optByID["overpower"].Name)
}
