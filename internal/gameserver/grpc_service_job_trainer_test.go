package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
