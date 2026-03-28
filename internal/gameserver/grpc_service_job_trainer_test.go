package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newJobTrainerTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "jt_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "jt_user",
		CharName:  "JTChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     50,
		Role:      "player",
		Level:     5,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 1000
	sess.Level = 5

	tmpl := &npc.Template{
		ID:      "test_trainer",
		Name:    "Rio Wrench",
		NPCType: "job_trainer",
		Level:   4,
		MaxHP:   25,
		AC:      11,
		JobTrainer: &npc.JobTrainerConfig{
			OfferedJobs: []npc.TrainableJob{
				{JobID: "scavenger", TrainingCost: 200, Prerequisites: npc.JobPrerequisites{MinLevel: 1}},
				{JobID: "infiltrator", TrainingCost: 500, Prerequisites: npc.JobPrerequisites{MinLevel: 3, RequiredJobs: []string{"scavenger"}}},
			},
		},
	}
	_, err = npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	return svc, uid
}

// TestHandleTrainJob_NoJobID_ListsAvailableJobs verifies that when no job ID is
// provided, the trainer lists their offered jobs as a menu (BUG-35 regression test).
func TestHandleTrainJob_NoJobID_ListsAvailableJobs(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: ""})
	require.NoError(t, err)
	content := evt.GetMessage().Content
	assert.Contains(t, content, "scavenger", "menu should list scavenger job")
	assert.Contains(t, content, "infiltrator", "menu should list infiltrator job")
	assert.Contains(t, content, "200", "menu should show training cost")
}

func TestHandleTrainJob_Success_FirstJob(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "scavenger")
	sess, _ := svc.sessions.GetPlayer(uid)
	assert.Equal(t, 1, sess.Jobs["scavenger"])
	assert.Equal(t, "scavenger", sess.ActiveJobID, "first trained job becomes active")
	assert.Equal(t, 800, sess.Currency)
}

func TestHandleTrainJob_AlreadyTrained(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Jobs["scavenger"] = 2
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "already trained")
}

func TestHandleTrainJob_InsufficientCredits(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 50
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "credits")
}

func TestHandleTrainJob_MissingPrerequisite(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "infiltrator"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "scavenger")
}

func TestHandleTrainJob_UnknownJob(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "ninja"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "ninja")
}

func TestHandleListJobs_Empty(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleListJobs(uid, &gamev1.ListJobsRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "no jobs")
}

func TestHandleListJobs_WithJobs(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Jobs["scavenger"] = 3
	sess.ActiveJobID = "scavenger"
	evt, err := svc.handleListJobs(uid, &gamev1.ListJobsRequest{})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "scavenger")
	assert.Contains(t, evt.GetMessage().Content, "[active]")
}

func TestHandleSetJob_Success(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Jobs["scavenger"] = 1
	sess.Jobs["infiltrator"] = 2
	sess.ActiveJobID = "scavenger"
	evt, err := svc.handleSetJob(uid, &gamev1.SetJobRequest{JobId: "infiltrator"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "infiltrator")
	assert.Equal(t, "infiltrator", sess.ActiveJobID)
}

func TestHandleSetJob_NotHeld(t *testing.T) {
	svc, uid := newJobTrainerTestServer(t)
	evt, err := svc.handleSetJob(uid, &gamev1.SetJobRequest{JobId: "infiltrator"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "not trained")
}

// TestProperty_HandleTrainJob_CurrencyNeverNegative verifies that training a job
// never results in negative currency for the player.
//
// Precondition: player has some currency (0 to 1000).
// Postcondition: currency is never negative after any train attempt.
func TestProperty_HandleTrainJob_CurrencyNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newJobTrainerTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = rapid.IntRange(0, 1000).Draw(rt, "currency")
		_, _ = svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
		sess, _ = svc.sessions.GetPlayer(uid)
		if sess.Currency < 0 {
			rt.Fatalf("currency went negative: %d", sess.Currency)
		}
	})
}

// TestProperty_HandleTrainJob_ActiveJobAlwaysInJobs verifies that after any training
// session, ActiveJobID is always a key present in sess.Jobs (or empty).
//
// Precondition: player may or may not already have jobs.
// Postcondition: ActiveJobID is either empty or a valid key in sess.Jobs.
func TestProperty_HandleTrainJob_ActiveJobAlwaysInJobs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newJobTrainerTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = 1000
		_, _ = svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
		sess, _ = svc.sessions.GetPlayer(uid)
		if sess.ActiveJobID != "" {
			if _, ok := sess.Jobs[sess.ActiveJobID]; !ok {
				rt.Fatalf("ActiveJobID %q not in Jobs map %v", sess.ActiveJobID, sess.Jobs)
			}
		}
	})
}

// TestProperty_HandleListJobs_NeverPanics verifies handleListJobs never panics
// for any session state.
//
// Precondition: player may have any combination of jobs.
// Postcondition: returns without panic.
func TestProperty_HandleListJobs_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid := newJobTrainerTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		numJobs := rapid.IntRange(0, 5).Draw(rt, "numJobs")
		jobIDs := []string{"scavenger", "infiltrator", "bruiser", "medic", "tech"}
		for i := 0; i < numJobs && i < len(jobIDs); i++ {
			sess.Jobs[jobIDs[i]] = rapid.IntRange(1, 10).Draw(rt, "level")
		}
		if numJobs > 0 {
			sess.ActiveJobID = jobIDs[0]
		}
		_, _ = svc.handleListJobs(uid, &gamev1.ListJobsRequest{})
	})
}
