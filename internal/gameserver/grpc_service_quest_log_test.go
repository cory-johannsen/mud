package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/session"
)

func newQuestLogTestServer(t *testing.T) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "ql_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "ql_user", CharName: "QlChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// REQ-QL-1: handleQuestLog MUST return a QuestLogView event.
func TestHandleQuestLog_ReturnsQuestLogView(t *testing.T) {
	svc, uid := newQuestLogTestServer(t)
	evt, err := svc.handleQuestLog(uid)
	require.NoError(t, err)
	view := evt.GetQuestLogView()
	require.NotNil(t, view, "expected QuestLogView, got %T", evt.GetPayload())
}

// REQ-QL-2: handleQuestLog MUST return an empty quests list when the player has no active quests.
func TestHandleQuestLog_EmptyWhenNoActiveQuests(t *testing.T) {
	svc, uid := newQuestLogTestServer(t)
	evt, err := svc.handleQuestLog(uid)
	require.NoError(t, err)
	view := evt.GetQuestLogView()
	require.NotNil(t, view)
	assert.Empty(t, view.Quests)
}

// REQ-QL-3: handleQuestLog MUST include active quests with objective progress when present.
func TestHandleQuestLog_ActiveQuestWithProgress(t *testing.T) {
	svc, uid := newQuestLogTestServer(t)

	// Inject a quest registry with one quest.
	reg := quest.QuestRegistry{
		"test_quest": &quest.QuestDef{
			ID:          "test_quest",
			Title:       "Test Quest",
			Description: "A test quest.",
			Rewards:     quest.QuestRewards{XP: 100, Credits: 50},
			Objectives: []quest.QuestObjective{
				{ID: "obj_1", Description: "Kill 3 bandits", Type: "kill", Quantity: 3},
			},
		},
	}
	svc.questSvc = quest.NewService(reg, nil, nil, nil, nil)

	// Put the player session in an active quest state.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.ActiveQuests = map[string]*quest.ActiveQuest{
		"test_quest": {
			QuestID:           "test_quest",
			ObjectiveProgress: map[string]int{"obj_1": 2},
		},
	}

	evt, err := svc.handleQuestLog(uid)
	require.NoError(t, err)
	view := evt.GetQuestLogView()
	require.NotNil(t, view)
	require.Len(t, view.Quests, 1)

	q := view.Quests[0]
	assert.Equal(t, "test_quest", q.QuestId)
	assert.Equal(t, "Test Quest", q.Title)
	assert.Equal(t, "active", q.Status)
	assert.Equal(t, int32(100), q.XpReward)
	assert.Equal(t, int32(50), q.CreditsReward)
	require.Len(t, q.Objectives, 1)
	assert.Equal(t, int32(2), q.Objectives[0].Current)
	assert.Equal(t, int32(3), q.Objectives[0].Required)
}

// REQ-QL-4: handleQuestLog MUST return a message event when the player session is not found.
func TestHandleQuestLog_PlayerNotFound(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	evt, err := svc.handleQuestLog("nonexistent_uid")
	require.NoError(t, err)
	assert.NotNil(t, evt.GetMessage())
}

// REQ-QL-5: handleQuestLog MUST only include active quests, not completed ones.
func TestHandleQuestLog_ExcludesCompletedQuests(t *testing.T) {
	svc, uid := newQuestLogTestServer(t)

	reg := quest.QuestRegistry{
		"done_quest": &quest.QuestDef{
			ID: "done_quest", Title: "Done", Description: "Completed.",
			Objectives: []quest.QuestObjective{{ID: "obj", Description: "do it", Type: "kill", Quantity: 1}},
		},
	}
	svc.questSvc = quest.NewService(reg, nil, nil, nil, nil)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Quest is not in ActiveQuests — it was never started.
	sess.ActiveQuests = map[string]*quest.ActiveQuest{}

	evt, err := svc.handleQuestLog(uid)
	require.NoError(t, err)
	view := evt.GetQuestLogView()
	require.NotNil(t, view)
	assert.Empty(t, view.Quests)
}

// REQ-QL-6: handleQuestLog MUST handle any number of active quests.
func TestProperty_QuestLog_ActiveQuestCount(t *testing.T) {
	svc, uid := newQuestLogTestServer(t)

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 8).Draw(rt, "quest_count")

		reg := quest.QuestRegistry{}
		activeQuests := map[string]*quest.ActiveQuest{}
		for i := range n {
			id := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "quest_id")
			id = id + "_" + string(rune('a'+i)) // ensure uniqueness
			reg[id] = &quest.QuestDef{
				ID: id, Title: "Quest " + id, Description: "desc",
				Objectives: []quest.QuestObjective{
					{ID: "obj", Description: "do it", Type: "kill", Quantity: 5},
				},
			}
			activeQuests[id] = &quest.ActiveQuest{
				QuestID:           id,
				ObjectiveProgress: map[string]int{"obj": rapid.IntRange(0, 5).Draw(rt, "progress")},
			}
		}
		svc.questSvc = quest.NewService(reg, nil, nil, nil, nil)

		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session not found")
		}
		sess.ActiveQuests = activeQuests

		evt, err := svc.handleQuestLog(uid)
		if err != nil {
			rt.Fatal("handleQuestLog failed:", err)
		}
		view := evt.GetQuestLogView()
		if view == nil {
			rt.Fatalf("expected QuestLogView, got %T", evt.GetPayload())
		}
		if len(view.Quests) != n {
			rt.Fatalf("expected %d quests, got %d", n, len(view.Quests))
		}
	})
}
