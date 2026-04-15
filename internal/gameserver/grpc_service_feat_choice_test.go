package gameserver

// REQ-FCM-1: handleJobGrants MUST populate PendingFeatChoices (not a raw label FeatName)
//             when the player has not yet selected from a feat choice pool.
// REQ-FCM-2: The JobFeatGrant row for an unresolved pool MUST have empty feat_id and feat_name.
// REQ-FCM-8: handleChooseFeat MUST validate that featID is in the pool and not already owned.
// REQ-FCM-9: handleChooseFeat MUST persist the feat and mark the grant level on success.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ---------------------------------------------------------------------------
// stubFeatLevelGrantsRepo — test double for CharacterFeatLevelGrantsRepo
// ---------------------------------------------------------------------------

// stubFeatLevelGrantsRepo is an in-memory CharacterFeatLevelGrantsRepo for testing.
//
// Precondition: none.
// Postcondition: IsLevelGranted returns the value stored by MarkLevelGranted.
type stubFeatLevelGrantsRepo struct {
	granted map[int64]map[int]bool
}

func (r *stubFeatLevelGrantsRepo) IsLevelGranted(_ context.Context, charID int64, level int) (bool, error) {
	if r.granted == nil {
		return false, nil
	}
	return r.granted[charID][level], nil
}

func (r *stubFeatLevelGrantsRepo) MarkLevelGranted(_ context.Context, charID int64, level int) error {
	if r.granted == nil {
		r.granted = make(map[int64]map[int]bool)
	}
	if r.granted[charID] == nil {
		r.granted[charID] = make(map[int]bool)
	}
	r.granted[charID][level] = true
	return nil
}

// ---------------------------------------------------------------------------
// buildChooseFeatSvc — helper that wires a minimal GameServiceServer for
// handleChooseFeat tests.
// ---------------------------------------------------------------------------

// buildChooseFeatSvc creates a GameServiceServer wired with the given featsRepo
// and levelGrantsRepo for testing handleChooseFeat.
//
// Precondition: t must be non-nil; feats and job must be non-nil.
// Postcondition: Returns non-nil svc and sessMgr.
func buildChooseFeatSvc(
	t *testing.T,
	feats []*ruleset.Feat,
	job *ruleset.Job,
	featsRepo *stubFeatsRepo,
	levelGrantsRepo *stubFeatLevelGrantsRepo,
) (*GameServiceServer, *session.Manager) {
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
		nil, featReg, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	// Inject the feat level grants repo directly (not exposed via newTestGameServiceServer).
	svc.featLevelGrantsRepo = levelGrantsRepo
	return svc, sessMgr
}

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

// ---------------------------------------------------------------------------
// handleChooseFeat tests (REQ-FCM-8, REQ-FCM-9)
// ---------------------------------------------------------------------------

// testChooseFeatSetup returns shared feats and job used across handleChooseFeat tests.
func testChooseFeatSetup() ([]*ruleset.Feat, *ruleset.Job) {
	feats := []*ruleset.Feat{
		{ID: "rage", Name: "Rage", Description: "You enter a furious rage.", Category: "combat"},
		{ID: "overpower", Name: "Overpower", Description: "Overwhelm your foe.", Category: "combat"},
	}
	job := &ruleset.Job{
		ID:   "test_job",
		Name: "Test Job",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Count: 1, Pool: []string{"rage", "overpower"}}},
		},
	}
	return feats, job
}

// TestHandleChooseFeat_ValidSelection_StoresFeat verifies that a valid feat choice
// persists the feat, marks the level granted, and returns a success event (REQ-FCM-8, REQ-FCM-9).
//
// Precondition: Player at level 2 with class "test_job"; feat "rage" is in the pool; player owns no feats.
// Postcondition: featsRepo contains "rage"; levelGrantsRepo marks level 2; event message mentions feat name.
func TestHandleChooseFeat_ValidSelection_StoresFeat(t *testing.T) {
	const uid = "fcm-choose-player"

	feats, job := testChooseFeatSetup()
	featsRepo := &stubFeatsRepo{data: map[int64][]string{}}
	levelGrantsRepo := &stubFeatLevelGrantsRepo{}

	svc, sessMgr := buildChooseFeatSvc(t, feats, job, featsRepo, levelGrantsRepo)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid,
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "test_job"
	sess.Level = 2

	evt, err := svc.handleChooseFeat(uid, 2, "rage")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// REQ-FCM-9: Feat must be persisted.
	storedFeats, repoErr := featsRepo.GetAll(context.Background(), sess.CharacterID)
	require.NoError(t, repoErr)
	assert.Contains(t, storedFeats, "rage", "REQ-FCM-9: feat must be stored in featsRepo")

	// REQ-FCM-9: Grant level must be marked.
	granted, repoErr := levelGrantsRepo.IsLevelGranted(context.Background(), sess.CharacterID, 2)
	require.NoError(t, repoErr)
	assert.True(t, granted, "REQ-FCM-9: level 2 must be marked as granted")

	// Success message must mention the feat name.
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a MessageEvent")
	assert.Contains(t, msg.Content, "Rage", "success message must name the feat")
}

// TestHandleChooseFeat_FeatNotInPool_ReturnsDenial verifies that choosing a feat
// not in the pool returns a denial and does not modify state (REQ-FCM-8).
//
// Precondition: Player at level 2 with class "test_job"; feat "snap_shot" is NOT in the pool.
// Postcondition: featsRepo is unmodified; levelGrantsRepo is unmodified; event message is a denial.
func TestHandleChooseFeat_FeatNotInPool_ReturnsDenial(t *testing.T) {
	const uid = "fcm-bad-pool-player"

	feats, job := testChooseFeatSetup()
	featsRepo := &stubFeatsRepo{data: map[int64][]string{}}
	levelGrantsRepo := &stubFeatLevelGrantsRepo{}

	svc, sessMgr := buildChooseFeatSvc(t, feats, job, featsRepo, levelGrantsRepo)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid,
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "test_job"
	sess.Level = 2

	evt, err := svc.handleChooseFeat(uid, 2, "snap_shot")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// REQ-FCM-8: No feat must be stored.
	storedFeats, repoErr := featsRepo.GetAll(context.Background(), sess.CharacterID)
	require.NoError(t, repoErr)
	assert.Empty(t, storedFeats, "REQ-FCM-8: featsRepo must remain unmodified on denial")

	// REQ-FCM-8: Level must not be marked.
	granted, repoErr := levelGrantsRepo.IsLevelGranted(context.Background(), sess.CharacterID, 2)
	require.NoError(t, repoErr)
	assert.False(t, granted, "REQ-FCM-8: level 2 must not be marked on denial")

	// Denial message must be present.
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a MessageEvent")
	assert.NotEmpty(t, msg.Content, "denial message must not be empty")
}

// TestHandleChooseFeat_AlreadyOwned_ReturnsDenial verifies that choosing an already-owned
// feat returns a denial and does not modify state (REQ-FCM-8).
//
// Precondition: Player at level 2 with class "test_job"; player already owns "rage".
// Postcondition: featsRepo unchanged; levelGrantsRepo unchanged; event message is a denial.
func TestHandleChooseFeat_AlreadyOwned_ReturnsDenial(t *testing.T) {
	const uid = "fcm-already-owned-player"

	feats, job := testChooseFeatSetup()
	// Player already owns "rage".
	featsRepo := &stubFeatsRepo{data: map[int64][]string{0: {"rage"}}}
	levelGrantsRepo := &stubFeatLevelGrantsRepo{}

	svc, sessMgr := buildChooseFeatSvc(t, feats, job, featsRepo, levelGrantsRepo)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid,
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Class = "test_job"
	sess.Level = 2

	initialFeats, _ := featsRepo.GetAll(context.Background(), sess.CharacterID)
	initialCount := len(initialFeats)

	evt, err := svc.handleChooseFeat(uid, 2, "rage")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// REQ-FCM-8: Feat count must not change.
	storedFeats, repoErr := featsRepo.GetAll(context.Background(), sess.CharacterID)
	require.NoError(t, repoErr)
	assert.Equal(t, initialCount, len(storedFeats), "REQ-FCM-8: featsRepo must remain unmodified when feat already owned")

	// REQ-FCM-8: Level must not be marked.
	granted, repoErr := levelGrantsRepo.IsLevelGranted(context.Background(), sess.CharacterID, 2)
	require.NoError(t, repoErr)
	assert.False(t, granted, "REQ-FCM-8: level 2 must not be marked when feat already owned")

	// Denial message must be present.
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a MessageEvent")
	assert.NotEmpty(t, msg.Content, "denial message must not be empty")
}
