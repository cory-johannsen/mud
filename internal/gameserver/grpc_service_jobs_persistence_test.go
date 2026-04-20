package gameserver

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// --- job persistence CharacterSaver stub ---

type jobsCharSaver struct {
	mockCharSaver
	savedJobs      map[string]int
	savedActiveJob string
	saveCalls      atomic.Int32
}

func (j *jobsCharSaver) SaveJobs(_ context.Context, _ int64, jobs map[string]int, activeJobID string) error {
	j.saveCalls.Add(1)
	j.savedJobs = jobs
	j.savedActiveJob = activeJobID
	return nil
}
func (j *jobsCharSaver) SaveInstanceCharges(_ context.Context, _ int64, _, _ string, _ int, _ bool) error {
	return nil
}

func (j *jobsCharSaver) LoadJobs(_ context.Context, _ int64) (map[string]int, string, error) {
	if j.savedJobs == nil {
		return map[string]int{}, "", nil
	}
	return j.savedJobs, j.savedActiveJob, nil
}
func (j *jobsCharSaver) LoadFocusPoints(_ context.Context, _ int64) (int, error) { return 0, nil }
func (j *jobsCharSaver) SaveFocusPoints(_ context.Context, _ int64, _ int) error { return nil }
func (j *jobsCharSaver) SaveHotbars(_ context.Context, _ int64, _ [][10]session.HotbarSlot, _ int) error {
	return nil
}
func (j *jobsCharSaver) LoadHotbars(_ context.Context, _ int64) ([][10]session.HotbarSlot, int, error) {
	return [][10]session.HotbarSlot{{}}, 0, nil
}

// REQ-JOB-PERSIST-1: SaveJobs is called after handleTrainJob succeeds.
//
// Precondition: player is at a job trainer with the offered job; sufficient currency.
// Postcondition: SaveJobs called once; saved jobs contain the trained job at level 1.
func TestHandleTrainJob_PersistsJobs(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	saver := &jobsCharSaver{mockCharSaver: mockCharSaver{saved: make(map[int64]string)}}
	svc.charSaver = saver

	uid := "train_persist_u1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "tp_user", CharName: "TPChar",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
		CharacterID: 42,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer(uid)
	sess.Currency = 1000

	tmpl := &npc.Template{
		ID: "trainer_persist", Name: "Trainer", NPCType: "job_trainer",
		Level: 1, MaxHP: 10, AC: 10,
		JobTrainer: &npc.JobTrainerConfig{
			OfferedJobs: []npc.TrainableJob{
				{JobID: "scavenger", TrainingCost: 50},
			},
		},
	}
	_, err = npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Trainer", JobId: "scavenger"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "scavenger")
	assert.Equal(t, int32(1), saver.saveCalls.Load(), "SaveJobs must be called exactly once after training")
	assert.Equal(t, 1, saver.savedJobs["scavenger"])
	assert.Equal(t, "scavenger", saver.savedActiveJob)
}

// REQ-JOB-PERSIST-2: SaveJobs is called after handleSetJob succeeds.
//
// Precondition: player already has two jobs trained; calls SetJob to switch active job.
// Postcondition: SaveJobs called once; saved activeJobID matches the new job.
func TestHandleSetJob_PersistsActiveJob(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	saver := &jobsCharSaver{mockCharSaver: mockCharSaver{saved: make(map[int64]string)}}
	svc.charSaver = saver

	uid := "setjob_persist_u1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "sj_user", CharName: "SJChar",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
		CharacterID: 43,
	})
	require.NoError(t, err)
	sess, _ := sessMgr.GetPlayer(uid)
	sess.Jobs = map[string]int{"scavenger": 1, "enforcer": 1}
	sess.ActiveJobID = "scavenger"

	evt, err := svc.handleSetJob(uid, &gamev1.SetJobRequest{JobId: "enforcer"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "enforcer")
	assert.Equal(t, int32(1), saver.saveCalls.Load(), "SaveJobs must be called exactly once after set-job")
	assert.Equal(t, "enforcer", saver.savedActiveJob)
}

// --- healer capacity persistence stub ---

type fakeHealerCapacityRepo struct {
	data      map[string]int
	saveCalls atomic.Int32
}

func newFakeHealerCapacityRepo() *fakeHealerCapacityRepo {
	return &fakeHealerCapacityRepo{data: make(map[string]int)}
}

func (f *fakeHealerCapacityRepo) Save(_ context.Context, templateID string, capacityUsed int) error {
	f.saveCalls.Add(1)
	f.data[templateID] = capacityUsed
	return nil
}

func (f *fakeHealerCapacityRepo) LoadAll(_ context.Context) (map[string]int, error) {
	out := make(map[string]int, len(f.data))
	for k, v := range f.data {
		out[k] = v
	}
	return out, nil
}

// REQ-HEALER-PERSIST-1: After handleHeal, healerCapacityRepo.Save is called with the updated capacity.
//
// Precondition: healer has daily capacity; player has sufficient HP deficit and currency.
// Postcondition: Save called once; stored capacity equals HP healed.
func TestHandleHeal_PersistsCapacity(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	repo := newFakeHealerCapacityRepo()
	svc.healerCapacityRepo = repo

	_, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), repo.saveCalls.Load(), "Save must be called after heal")
	// Player healed 50 HP (50→100), capacity used = 50.
	assert.Equal(t, 50, repo.data["test_healer"])
}

// REQ-HEALER-PERSIST-2: After handleHealAmount, healerCapacityRepo.Save is called with the updated capacity.
//
// Precondition: healer has daily capacity; player requests specific amount.
// Postcondition: Save called once; stored capacity equals requested amount.
func TestHandleHealAmount_PersistsCapacity(t *testing.T) {
	svc, uid := newHealerTestServer(t)
	repo := newFakeHealerCapacityRepo()
	svc.healerCapacityRepo = repo

	_, err := svc.handleHealAmount(uid, &gamev1.HealAmountRequest{NpcName: "Clutch", Amount: 20})
	require.NoError(t, err)
	assert.Equal(t, int32(1), repo.saveCalls.Load(), "Save must be called after heal amount")
	assert.Equal(t, 20, repo.data["test_healer"])
}

// REQ-HEALER-PERSIST-3: InitHealerCapacities populates healerRuntimeStates from the repo.
//
// Precondition: repo contains stored capacity for a healer template; NPC instance is spawned.
// Postcondition: healerRuntimeStates[inst.ID].CapacityUsed matches stored value.
func TestInitHealerCapacities_LoadsFromRepo(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	repo := newFakeHealerCapacityRepo()
	repo.data["cap_healer_tmpl"] = 75
	svc.healerCapacityRepo = repo

	tmpl := &npc.Template{
		ID: "cap_healer_tmpl", Name: "CapHealer", NPCType: "healer",
		Level: 1, MaxHP: 10, AC: 10,
		Healer: &npc.HealerConfig{PricePerHP: 1, DailyCapacity: 100},
	}
	inst, err := npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	svc.InitHealerCapacities(context.Background())

	state := svc.healerStateFor(inst.ID)
	require.NotNil(t, state)
	assert.Equal(t, 75, state.CapacityUsed)
}

// REQ-HEALER-PERSIST-4: tickHealerCapacity persists reset capacity for each template.
//
// Precondition: healer runtime state has non-zero capacity; repo is set.
// Postcondition: after tick, Save called with 0 for each template; CapacityUsed == 0.
func TestTickHealerCapacity_PersistsReset(t *testing.T) {
	svc, _ := newHealerTestServer(t)
	repo := newFakeHealerCapacityRepo()
	repo.data["test_healer"] = 80
	svc.healerCapacityRepo = repo

	// Force a runtime state into the server with non-zero capacity.
	inst := svc.npcMgr.FindInRoom("room_a", "Clutch")
	require.NotNil(t, inst)
	svc.initHealerRuntimeState(inst)
	svc.healerRuntimeStates[inst.ID].CapacityUsed = 80

	svc.tickHealerCapacity()

	assert.Equal(t, 0, svc.healerRuntimeStates[inst.ID].CapacityUsed)
	assert.Equal(t, int32(1), repo.saveCalls.Load(), "Save must be called once per template on tick")
	assert.Equal(t, 0, repo.data["test_healer"])
}
