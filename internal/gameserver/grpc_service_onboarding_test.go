package gameserver

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// inMemoryQuestRepo is a minimal in-memory QuestRepository for gameserver tests.
//
// Precondition: none.
// Postcondition: persists statuses and progress in-memory; never errors.
type inMemoryQuestRepo struct {
	statuses    map[string]string
	progress    map[string]map[string]int
	completedAt map[string]*time.Time
}

func newInMemoryQuestRepo() *inMemoryQuestRepo {
	return &inMemoryQuestRepo{
		statuses:    make(map[string]string),
		progress:    make(map[string]map[string]int),
		completedAt: make(map[string]*time.Time),
	}
}

func (r *inMemoryQuestRepo) SaveQuestStatus(_ context.Context, _ int64, questID, status string, completedAt *time.Time) error {
	r.statuses[questID] = status
	r.completedAt[questID] = completedAt
	return nil
}

func (r *inMemoryQuestRepo) SaveObjectiveProgress(_ context.Context, _ int64, questID, objectiveID string, progress int) error {
	if r.progress[questID] == nil {
		r.progress[questID] = make(map[string]int)
	}
	r.progress[questID][objectiveID] = progress
	return nil
}

func (r *inMemoryQuestRepo) LoadQuests(_ context.Context, _ int64) ([]quest.QuestRecord, error) {
	return nil, nil
}

// repoRootForTest returns the repository root directory relative to this test file.
func repoRootForTest() string {
	_, filename, _, _ := runtime.Caller(0)
	// This file is at internal/gameserver/grpc_service_onboarding_test.go
	// Walk up two levels to reach the repo root.
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// TestProperty_IssueOnboardingQuest_IdempotentOnRepeat verifies that calling
// issueOnboardingQuest multiple times for the same character only results in the
// onboarding quest appearing exactly once in the session's active quests.
//
// Property: For any number of repeated calls >= 1, the quest count is always 1.
func TestProperty_IssueOnboardingQuest_IdempotentOnRepeat(t *testing.T) {
	repoRoot := repoRootForTest()
	questsDir := filepath.Join(repoRoot, "content", "quests")

	// Load the real quest registry from content/quests.
	reg, err := quest.LoadFromDir(questsDir)
	require.NoError(t, err, "loading quest registry from %s", questsDir)
	_, ok := reg["onboarding_find_zone_map"]
	require.True(t, ok, "onboarding_find_zone_map must be present in quest registry")

	worldMgr, sessMgr := testWorldAndSession(t)
	cmdRegistry := command.DefaultRegistry()
	worldHandler := NewWorldHandler(worldMgr, sessMgr, nil, nil, nil, nil)
	chatHandler := NewChatHandler(sessMgr)
	logger := zaptest.NewLogger(t)

	questRepo := newInMemoryQuestRepo()

	storage := StorageDeps{
		QuestRepo: questRepo,
	}
	content := ContentDeps{
		WorldMgr:      worldMgr,
		QuestRegistry: reg,
	}
	handlers := HandlerDeps{
		WorldHandler: worldHandler,
		ChatHandler:  chatHandler,
	}
	svc := NewGameServiceServer(storage, content, handlers, sessMgr, cmdRegistry, nil, logger)

	rapid.Check(t, func(rt *rapid.T) {
		// Draw a call count in [2, 5] to test multiple repeated calls.
		callCount := rapid.IntRange(2, 5).Draw(rt, "call_count")

		// Create a fresh session for each property iteration so state is isolated.
		uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")
		sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         uid,
			Username:    uid,
			CharName:    uid,
			CharacterID: 1,
			RoomID:      "room_a",
			CurrentHP:   10,
			MaxHP:       10,
			Role:        "player",
			Abilities:   character.AbilityScores{},
		})
		require.NoError(rt, addErr)
		// Ensure the session is cleaned up after each iteration.
		defer func() { _ = sessMgr.RemovePlayer(uid) }()

		ctx := context.Background()
		for i := 0; i < callCount; i++ {
			svc.issueOnboardingQuest(ctx, uid, sess)
		}

		activeQuests := sess.GetActiveQuests()
		count := 0
		for questID := range activeQuests {
			if questID == "onboarding_find_zone_map" {
				count++
			}
		}
		if count != 1 {
			rt.Fatalf("expected onboarding_find_zone_map to appear exactly once in active quests after %d calls, got %d", callCount, count)
		}
	})
}
