package gameserver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// failingSkillsRepo is a CharacterSkillsRepository that always returns an error
// from UpgradeSkill, used to test the UpgradeSkill failure path.
//
// Precondition: none.
// Postcondition: UpgradeSkill always returns upgradeErr; other methods succeed.
type failingSkillsRepo struct {
	stubSkillsRepo
	upgradeErr   error
	upgradeCalls atomic.Int32
}

func (r *failingSkillsRepo) UpgradeSkill(_ context.Context, _ int64, _, _ string) error {
	r.upgradeCalls.Add(1)
	return r.upgradeErr
}

// mockProgressRepo is an in-memory ProgressRepository test double.
//
// Precondition: none.
// Postcondition: ConsumePendingSkillIncrease returns consumeErr if set; all other methods succeed with no-op.
type mockProgressRepo struct {
	consumeErr     error
	consumeCalls   atomic.Int32
	incrementCalls atomic.Int32
}

func (m *mockProgressRepo) GetProgress(_ context.Context, _ int64) (int, int, int, int, error) {
	return 1, 0, 10, 0, nil
}
func (m *mockProgressRepo) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockProgressRepo) IncrementPendingSkillIncreases(_ context.Context, _ int64, _ int) error {
	m.incrementCalls.Add(1)
	return nil
}
func (m *mockProgressRepo) ConsumePendingBoost(_ context.Context, _ int64) error {
	return nil
}
func (m *mockProgressRepo) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	m.consumeCalls.Add(1)
	return m.consumeErr
}
func (m *mockProgressRepo) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (m *mockProgressRepo) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	return nil
}
func (m *mockProgressRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return nil, nil
}
func (m *mockProgressRepo) SetPendingTechLevels(_ context.Context, _ int64, _ []int) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helper: build a GameServiceServer wired for handleTrainSkill tests.
// ---------------------------------------------------------------------------

// trainSkillTestOptions holds optional overrides for testServiceForTrainSkill.
//
// Precondition: none — all fields are optional.
type trainSkillTestOptions struct {
	skillsRepo   CharacterSkillsRepository
	progressRepo ProgressRepository
}

// testServiceForTrainSkill creates a minimal GameServiceServer suitable for handleTrainSkill tests.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a configured *GameServiceServer with optional skills and progress repos.
func testServiceForTrainSkill(t *testing.T, opts trainSkillTestOptions) *GameServiceServer {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	npcMgr := npc.NewManager()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)
	svc := NewGameServiceServer(
		worldMgr, sessMgr, cmdRegistry,
		worldHandler, chatHandler, logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
	)
	svc.characterSkillsRepo = opts.skillsRepo
	svc.progressRepo = opts.progressRepo
	return svc
}

// addPlayerForTrainSkill adds a player session with the given uid and initial pending skill increases.
//
// Precondition: svc must have a valid session manager.
// Postcondition: Player is in the session manager with PendingSkillIncreases set; session is returned.
func addPlayerForTrainSkill(t *testing.T, svc *GameServiceServer, uid string, pendingIncreases int, level int) *session.PlayerSession {
	t.Helper()
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      "room_a",
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
		Level:       level,
	})
	require.NoError(t, err)
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingSkillIncreases = pendingIncreases
	return sess
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleTrainSkill_PlayerNotFound verifies that handleTrainSkill returns a Go error
// when the uid does not identify an active session.
//
// Precondition: no session for uid exists.
// Postcondition: Returns a non-nil error.
func TestHandleTrainSkill_PlayerNotFound(t *testing.T) {
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{})

	_, err := svc.handleTrainSkill("no_such_player", "parkour")

	assert.Error(t, err, "handleTrainSkill must return error for unknown uid")
}

// TestHandleTrainSkill_NoPendingIncreases verifies that when a player has no pending
// skill increases, a MessageEvent containing "no pending" is returned and no DB calls are made.
//
// Precondition: uid identifies an active session; PendingSkillIncreases == 0.
// Postcondition: Returns a MessageEvent with "no pending"; ConsumePendingSkillIncrease not called.
func TestHandleTrainSkill_NoPendingIncreases(t *testing.T) {
	progress := &mockProgressRepo{}
	skills := &stubSkillsRepo{data: map[int64]map[string]string{}}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	addPlayerForTrainSkill(t, svc, "u_no_pending", 0, 1)

	evt, err := svc.handleTrainSkill("u_no_pending", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a MessageEvent")
	assert.Contains(t, msg.Content, "no pending", "message must indicate no pending skill increases")
	assert.Equal(t, int32(0), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must not be called")
}

// TestHandleTrainSkill_AlreadyLegendary verifies that attempting to advance a skill
// already at legendary rank returns a MessageEvent containing "maximum rank".
//
// Precondition: uid has a pending increase; skill is already "legendary".
// Postcondition: Returns a MessageEvent with "maximum rank"; ConsumePendingSkillIncrease not called.
func TestHandleTrainSkill_AlreadyLegendary(t *testing.T) {
	progress := &mockProgressRepo{}
	skills := &stubSkillsRepo{data: map[int64]map[string]string{
		1: {"parkour": "legendary"},
	}}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	addPlayerForTrainSkill(t, svc, "u_legendary", 1, 99)

	// Seed session skills to match repo state.
	sess, ok := svc.sessions.GetPlayer("u_legendary")
	require.True(t, ok)
	sess.Skills = map[string]string{"parkour": "legendary"}

	evt, err := svc.handleTrainSkill("u_legendary", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "maximum rank", "message must indicate skill is at maximum rank")
	assert.Equal(t, int32(0), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must not be called")
}

// TestHandleTrainSkill_LevelGateNotMet verifies that attempting to advance a skill
// when the level requirement is not met returns a MessageEvent containing "level".
//
// Precondition: uid has a pending increase; skill is "trained"; player is level 1 (gate for expert is 15).
// Postcondition: Returns a MessageEvent with "level"; ConsumePendingSkillIncrease not called.
func TestHandleTrainSkill_LevelGateNotMet(t *testing.T) {
	progress := &mockProgressRepo{}
	skills := &stubSkillsRepo{data: map[int64]map[string]string{}}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	addPlayerForTrainSkill(t, svc, "u_gate", 1, 1)

	// Seed session with trained rank to trigger expert gate (requires level 15).
	sess, ok := svc.sessions.GetPlayer("u_gate")
	require.True(t, ok)
	sess.Skills = map[string]string{"parkour": "trained"}

	evt, err := svc.handleTrainSkill("u_gate", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "level", "message must indicate a level requirement")
	assert.Equal(t, int32(0), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must not be called")
}

// TestHandleTrainSkill_UpgradeSkillFailure verifies that when UpgradeSkill returns an error,
// an error MessageEvent is returned and ConsumePendingSkillIncrease is NOT called.
//
// Precondition: uid has a pending increase; skill is valid; UpgradeSkill returns an error.
// Postcondition: Returns error MessageEvent; ConsumePendingSkillIncrease not called; session unchanged.
func TestHandleTrainSkill_UpgradeSkillFailure(t *testing.T) {
	progress := &mockProgressRepo{}
	skills := &failingSkillsRepo{
		upgradeErr: errors.New("db error"),
	}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	sess := addPlayerForTrainSkill(t, svc, "u_upgrade_fail", 1, 1)
	initialIncreases := sess.PendingSkillIncreases

	evt, err := svc.handleTrainSkill("u_upgrade_fail", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Failed", "message must indicate upgrade failure")
	assert.Equal(t, int32(0), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must not be called when UpgradeSkill fails")
	assert.Equal(t, initialIncreases, sess.PendingSkillIncreases, "PendingSkillIncreases must not change on UpgradeSkill failure")
}

// TestHandleTrainSkill_ConsumePendingFailure verifies that when ConsumePendingSkillIncrease returns
// an error, an error MessageEvent is returned and the session is not mutated.
//
// Precondition: uid has a pending increase; UpgradeSkill succeeds; ConsumePendingSkillIncrease fails.
// Postcondition: Returns error MessageEvent; session skills and pending count unchanged.
func TestHandleTrainSkill_ConsumePendingFailure(t *testing.T) {
	progress := &mockProgressRepo{consumeErr: errors.New("consume error")}
	skills := &stubSkillsRepo{data: map[int64]map[string]string{}}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	sess := addPlayerForTrainSkill(t, svc, "u_consume_fail", 1, 1)
	initialIncreases := sess.PendingSkillIncreases

	evt, err := svc.handleTrainSkill("u_consume_fail", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "Failed", "message must indicate consume failure")
	assert.Equal(t, int32(1), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must have been called once")
	assert.Equal(t, initialIncreases, sess.PendingSkillIncreases, "PendingSkillIncreases must not change on consume failure")
}

// TestHandleTrainSkill_HappyPath verifies that a valid trainskill request advances the skill,
// decrements PendingSkillIncreases, and returns a confirmation message.
//
// Precondition: uid has one pending increase; skill is untrained; all repo calls succeed.
// Postcondition: skill advanced to "trained"; PendingSkillIncreases decremented; confirmation returned.
func TestHandleTrainSkill_HappyPath(t *testing.T) {
	progress := &mockProgressRepo{}
	skills := &stubSkillsRepo{data: map[int64]map[string]string{}}
	svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
	sess := addPlayerForTrainSkill(t, svc, "u_happy_train", 1, 1)

	evt, err := svc.handleTrainSkill("u_happy_train", "parkour")

	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "parkour", "confirmation must name the skill")
	assert.Contains(t, msg.Content, "trained", "confirmation must name the new rank")
	assert.Equal(t, int32(1), progress.consumeCalls.Load(), "ConsumePendingSkillIncrease must be called exactly once")
	assert.Equal(t, "trained", sess.Skills["parkour"], "session skill must be updated to trained")
	assert.Equal(t, 0, sess.PendingSkillIncreases, "PendingSkillIncreases must be decremented")
}

// TestHandleTrainSkill_AllValidSkills_HappyPath is a property-based test verifying that every
// valid skill ID can be advanced from untrained to trained (level 1, no gate) without error.
//
// Precondition: all repos succeed; player has one pending increase; skill is untrained.
// Postcondition: skill advances to "trained"; PendingSkillIncreases decremented.
func TestHandleTrainSkill_AllValidSkills_HappyPath(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		skillID := rapid.SampledFrom(command.ValidSkillIDs).Draw(rt, "skillID")

		progress := &mockProgressRepo{}
		skills := &stubSkillsRepo{data: map[int64]map[string]string{}}
		svc := testServiceForTrainSkill(t, trainSkillTestOptions{skillsRepo: skills, progressRepo: progress})
		sess := addPlayerForTrainSkill(t, svc, "u_prop_train", 1, 1)

		evt, err := svc.handleTrainSkill("u_prop_train", skillID)

		if err != nil {
			rt.Fatalf("expected no error for valid skill %q, got %v", skillID, err)
		}
		if evt == nil || evt.GetMessage() == nil {
			rt.Fatalf("expected non-nil MessageEvent for valid skill %q", skillID)
		}
		if sess.Skills[skillID] != "trained" {
			rt.Fatalf("expected session skill %q = trained, got %q", skillID, sess.Skills[skillID])
		}
		if sess.PendingSkillIncreases != 0 {
			rt.Fatalf("expected PendingSkillIncreases=0, got %d", sess.PendingSkillIncreases)
		}
	})
}
