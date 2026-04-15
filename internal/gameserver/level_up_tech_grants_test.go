package gameserver

// Tests for applyLevelUpTechGrants (REQ-BUG99-1 through REQ-BUG99-6).
// Verifies that the extracted method applies tech grants correctly and issues
// tech trainer quests for deferred L2+ grants regardless of how the level-up
// was triggered (admin grant, combat XP, room discovery XP, skill check XP).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	questpkg "github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// bugLUTHardwiredRepo is a HardwiredTechRepo for BUG-99 tests.
type bugLUTHardwiredRepo struct{ stored []string }

func (r *bugLUTHardwiredRepo) GetAll(_ context.Context, _ int64) ([]string, error) {
	return r.stored, nil
}
func (r *bugLUTHardwiredRepo) SetAll(_ context.Context, _ int64, ids []string) error {
	r.stored = ids
	return nil
}

// bugLUTPreparedRepo is a PreparedTechRepo for BUG-99 tests.
type bugLUTPreparedRepo struct{ slots map[int][]*session.PreparedSlot }

func (r *bugLUTPreparedRepo) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
	return r.slots, nil
}
func (r *bugLUTPreparedRepo) Set(_ context.Context, _ int64, level, index int, techID string) error {
	if r.slots == nil {
		r.slots = make(map[int][]*session.PreparedSlot)
	}
	for len(r.slots[level]) <= index {
		r.slots[level] = append(r.slots[level], nil)
	}
	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
	return nil
}
func (r *bugLUTPreparedRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *bugLUTPreparedRepo) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
	if r.slots != nil {
		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
			slots[index].Expended = expended
		}
	}
	return nil
}

// bugLUTSpontaneousRepo is a SpontaneousTechRepo for BUG-99 tests.
type bugLUTSpontaneousRepo struct{ techs map[int][]string }

func (r *bugLUTSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	return r.techs, nil
}
func (r *bugLUTSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *bugLUTSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error {
	r.techs = nil
	return nil
}

// bugLUTInnateRepo is an InnateTechRepo for BUG-99 tests.
type bugLUTInnateRepo struct{ slots map[string]*session.InnateSlot }

func (r *bugLUTInnateRepo) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	return r.slots, nil
}
func (r *bugLUTInnateRepo) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses}
	return nil
}
func (r *bugLUTInnateRepo) DeleteAll(_ context.Context, _ int64) error { r.slots = nil; return nil }
func (r *bugLUTInnateRepo) Decrement(_ context.Context, _ int64, _ string) error { return nil }
func (r *bugLUTInnateRepo) RestoreAll(_ context.Context, _ int64) error           { return nil }

// bugLUTProgressRepo is a ProgressRepository for BUG-99 tests.
type bugLUTProgressRepo struct {
	pendingLevels []int
	setWasCalled  bool
}

func (r *bugLUTProgressRepo) GetProgress(_ context.Context, _ int64) (int, int, int, int, error) {
	return 1, 0, 10, 0, nil
}
func (r *bugLUTProgressRepo) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (r *bugLUTProgressRepo) IncrementPendingSkillIncreases(_ context.Context, _ int64, _ int) error {
	return nil
}
func (r *bugLUTProgressRepo) ConsumePendingBoost(_ context.Context, _ int64) error { return nil }
func (r *bugLUTProgressRepo) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	return nil
}
func (r *bugLUTProgressRepo) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (r *bugLUTProgressRepo) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	return nil
}
func (r *bugLUTProgressRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return r.pendingLevels, nil
}
func (r *bugLUTProgressRepo) SetPendingTechLevels(_ context.Context, _ int64, levels []int) error {
	r.pendingLevels = levels
	r.setWasCalled = true
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildApplyLevelUpTechGrantsSvc creates a minimal GameServiceServer wired for
// applyLevelUpTechGrants testing with a job that has LevelUpGrants at level 3
// for the provided grants definition. It registers a tech_trainer NPC with the
// provided findQuestID and questDef (when non-nil) to test trainer quest issuance.
func buildApplyLevelUpTechGrantsSvc(
	t *testing.T,
	grants *ruleset.TechnologyGrants,
	findQuestID string,
	questDef *questpkg.QuestDef,
) (*GameServiceServer, *session.PlayerSession) {
	t.Helper()

	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcMgr)

	// Wire XP service (not strictly needed for applyLevelUpTechGrants but keeps svc valid).
	cfg := &xp.XPConfig{
		BaseXP: 100, HPPerLevel: 5, BoostInterval: 5, LevelCap: 100,
		Awards: xp.Awards{KillXPPerNPCLevel: 50},
	}
	svc.SetXPService(xp.NewService(cfg, &grantXPProgressSaver{}))

	// Wire a tech registry with one prepared L2 neural tech.
	techReg := technology.NewRegistry()
	techReg.Register(&technology.TechnologyDef{
		ID:        "neural_strike",
		Name:      "Neural Strike",
		Tradition: technology.TraditionNeural,
		Level:     2,
		UsageType: technology.UsagePrepared,
		ActionCost: 1,
		Range:     technology.RangeMelee,
		Targets:   technology.TargetsSingle,
		Duration:  "instantaneous",
	})
	svc.techRegistry = techReg

	// Wire repos.
	svc.SetHardwiredTechRepo(&bugLUTHardwiredRepo{})
	svc.SetPreparedTechRepo(&bugLUTPreparedRepo{})
	svc.SetSpontaneousTechRepo(&bugLUTSpontaneousRepo{})
	svc.SetInnateTechRepo(&bugLUTInnateRepo{})
	svc.progressRepo = &bugLUTProgressRepo{}

	// Wire a job with level 3 tech grants.
	job := &ruleset.Job{
		ID:   "test_job_bug99",
		Name: "Test Job Bug99",
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			3: grants,
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.jobRegistry = jobReg

	// Optionally wire a tech_trainer NPC with a find quest and a quest service.
	if findQuestID != "" && questDef != nil {
		tmpl := &npc.Template{
			ID:      "neural_trainer_bug99",
			Name:    "Neural Trainer",
			NPCType: "tech_trainer",
			Level:   3,
			MaxHP:   20,
			AC:      11,
			TechTrainer: &npc.TechTrainerConfig{
				Tradition:     "neural",
				OfferedLevels: []int{2},
				BaseCost:      100,
				FindQuestID:   findQuestID,
			},
		}
		_, err := npcMgr.Spawn(tmpl, "room_a")
		require.NoError(t, err)

		reg := questpkg.QuestRegistry{questDef.ID: questDef}
		svc.questSvc = questpkg.NewService(reg, &noOpQuestRepo{}, nil, nil, nil)
	}

	// Add a player session.
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "bug99_player",
		Username:    "bug99_user",
		CharName:    "Bug99Char",
		CharacterID: 42,
		RoomID:      "room_a",
		CurrentHP:   20,
		MaxHP:       20,
		Role:        "player",
		Level:       2,
		Class:       "test_job_bug99",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("bug99_player")
	require.True(t, ok)
	if sess.ActiveQuests == nil {
		sess.ActiveQuests = make(map[string]*questpkg.ActiveQuest)
	}
	if sess.CompletedQuests == nil {
		sess.CompletedQuests = make(map[string]*time.Time)
	}

	return svc, sess
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestApplyLevelUpTechGrants_DeferredGrantIssuesTrainerQuest verifies that
// applyLevelUpTechGrants issues a find-trainer quest for L2+ deferred tech grants.
//
// Precondition: job has a LevelUpGrant at level 3 with a deferred prepared L2 tech slot;
// a tech_trainer NPC with FindQuestID="find_neural_trainer" matches the tradition;
// a quest with that ID exists in the registry.
// Postcondition: sess.ActiveQuests contains "find_neural_trainer" after calling
// applyLevelUpTechGrants(ctx, sess, fromLevel=2, toLevel=3).
func TestApplyLevelUpTechGrants_DeferredGrantIssuesTrainerQuest(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			// 1 slot at tech level 2, but 2 pool entries — forces deferred.
			SlotsByLevel: map[int]int{2: 1},
			Pool: []ruleset.PreparedEntry{
				{ID: "neural_strike", Level: 2},
				{ID: "neural_boost", Level: 2},
			},
		},
	}
	questDef := &questpkg.QuestDef{
		ID:    "find_neural_trainer",
		Title: "Find a Neural Trainer",
		Type:  "find_trainer",
	}

	svc, sess := buildApplyLevelUpTechGrantsSvc(t, grants, "find_neural_trainer", questDef)

	svc.applyLevelUpTechGrants(context.Background(), sess, 2, 3)

	assert.Contains(t, sess.ActiveQuests, "find_neural_trainer",
		"REQ-BUG99-1: applyLevelUpTechGrants must issue a find-trainer quest for deferred L2+ grants")
}

// TestApplyLevelUpTechGrants_AutoAssignNoPending verifies that when pool exactly
// matches open slots, no pending grants are created.
//
// Precondition: job has LevelUpGrant at level 3 with exactly 1 slot and 1 pool entry.
// Postcondition: sess.PendingTechGrants is empty after applyLevelUpTechGrants.
func TestApplyLevelUpTechGrants_AutoAssignNoPending(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Prepared: &ruleset.PreparedGrants{
			SlotsByLevel: map[int]int{1: 1},
			Pool:         []ruleset.PreparedEntry{{ID: "neural_strike", Level: 1}},
		},
	}

	svc, sess := buildApplyLevelUpTechGrantsSvc(t, grants, "", nil)

	svc.applyLevelUpTechGrants(context.Background(), sess, 2, 3)

	assert.Empty(t, sess.PendingTechGrants,
		"REQ-BUG99-2: auto-assign must not leave pending grants")
}

// TestApplyLevelUpTechGrants_NoOpWhenNoJobMatch verifies that applyLevelUpTechGrants
// is a no-op when the player's class has no matching job in the registry.
//
// Precondition: sess.Class = "unknown_job" which is not registered.
// Postcondition: no panic; sess.PendingTechGrants remains nil.
func TestApplyLevelUpTechGrants_NoOpWhenNoJobMatch(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Hardwired: []string{"neural_strike"},
	}
	svc, sess := buildApplyLevelUpTechGrantsSvc(t, grants, "", nil)
	sess.Class = "unknown_job"

	// Must not panic.
	svc.applyLevelUpTechGrants(context.Background(), sess, 2, 3)

	assert.Nil(t, sess.PendingTechGrants,
		"applyLevelUpTechGrants must be a no-op when the player's class is not in the job registry")
}

// TestApplyLevelUpTechGrants_MultiLevelRange verifies that when fromLevel=1 and toLevel=3,
// grants for level 3 are applied (level 2 has no grants in this config).
//
// Precondition: job has LevelUpGrant at level 3 only (hardwired).
// Postcondition: sess.HardwiredTechs contains "neural_strike" after the call.
func TestApplyLevelUpTechGrants_MultiLevelRange(t *testing.T) {
	grants := &ruleset.TechnologyGrants{
		Hardwired: []string{"neural_strike"},
	}

	svc, sess := buildApplyLevelUpTechGrantsSvc(t, grants, "", nil)

	svc.applyLevelUpTechGrants(context.Background(), sess, 1, 3)

	assert.Contains(t, sess.HardwiredTechs, "neural_strike",
		"applyLevelUpTechGrants must apply hardwired tech grant for level 3 when toLevel=3")
}

// TestProperty_ApplyLevelUpTechGrants_NeverErrors verifies applyLevelUpTechGrants
// never panics or errors given any valid level range.
//
// Precondition: fromLevel and toLevel are in [0, 20] with fromLevel <= toLevel.
// Postcondition: applyLevelUpTechGrants always returns without panic.
func TestProperty_ApplyLevelUpTechGrants_NeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fromLevel := rapid.IntRange(0, 19).Draw(rt, "fromLevel")
		toLevel := rapid.IntRange(fromLevel, 20).Draw(rt, "toLevel")

		grants := &ruleset.TechnologyGrants{
			Hardwired: []string{"neural_strike"},
		}
		// Use t (not rt) to satisfy *testing.T requirement.
		svc, sess := buildApplyLevelUpTechGrantsSvc(t, grants, "", nil)

		// Must not panic regardless of fromLevel/toLevel combination.
		svc.applyLevelUpTechGrants(context.Background(), sess, fromLevel, toLevel)
	})
}

// TestOnLevelUpFn_CalledWhenLevelUpOccurs verifies that the onLevelUpFn callback is
// invoked by pushXPMessages when level-up messages are present.
//
// Precondition: CombatHandler with an onLevelUpFn registered; sess.Level=3;
// pushXPMessages called with fromLevel=2 and one level-up message.
// Postcondition: onLevelUpFn is called with fromLevel=2 and toLevel=3.
func TestOnLevelUpFn_CalledWhenLevelUpOccurs(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	type callArgs struct {
		fromLevel int
		toLevel   int
	}
	var calls []callArgs
	h.SetOnLevelUpFn(func(_ context.Context, sess *session.PlayerSession, fromLevel, toLevel int) {
		calls = append(calls, callArgs{fromLevel: fromLevel, toLevel: toLevel})
	})

	sess := &session.PlayerSession{
		UID:       "onlevelup-player",
		Level:     3,
		MaxHP:     30,
		CurrentHP: 30,
		Entity:    session.NewBridgeEntity("onlevelup-player", 64),
	}

	levelMsgs := []string{"*** You reached level 3! ***"}
	h.pushXPMessages(sess, levelMsgs, 100, "Goblin", 2)

	require.Len(t, calls, 1, "onLevelUpFn must be called exactly once when a level-up occurs")
	assert.Equal(t, 2, calls[0].fromLevel, "onLevelUpFn must receive fromLevel=2")
	assert.Equal(t, 3, calls[0].toLevel, "onLevelUpFn must receive toLevel=3 (sess.Level)")
}

// TestOnLevelUpFn_NotCalledWithoutLevelUp verifies that onLevelUpFn is NOT invoked
// when no level-up messages are present (i.e., no level-up occurred).
//
// Precondition: CombatHandler with an onLevelUpFn registered; pushXPMessages called
// with empty levelMsgs.
// Postcondition: onLevelUpFn is not called.
func TestOnLevelUpFn_NotCalledWithoutLevelUp(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	var called bool
	h.SetOnLevelUpFn(func(_ context.Context, _ *session.PlayerSession, _, _ int) {
		called = true
	})

	sess := &session.PlayerSession{
		UID:       "nolevelup-player",
		Level:     2,
		MaxHP:     20,
		CurrentHP: 20,
		Entity:    session.NewBridgeEntity("nolevelup-player", 64),
	}

	// No level-up messages.
	h.pushXPMessages(sess, nil, 50, "Rat", 2)

	assert.False(t, called, "onLevelUpFn must NOT be called when no level-up occurred")
}
