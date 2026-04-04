package gameserver

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
