package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// newQuestTestServer builds a minimal GameServiceServer with a real quest.Service wired.
// The returned server has questSvc set to a Service backed by reg and a no-op repository.
//
// Precondition: t must be non-nil; reg may be empty.
// Postcondition: returns a non-nil *GameServiceServer with questSvc set and a registered player session.
func newQuestTestServer(t *testing.T, reg quest.QuestRegistry) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	svc.questSvc = quest.NewService(reg, &noOpQuestRepo{}, nil, nil, nil)

	uid := "quest_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "quest_user",
		CharName:  "QuestChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     100,
		Role:      "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// noOpQuestRepo is a no-op QuestRepository for test use.
type noOpQuestRepo struct{}

func (r *noOpQuestRepo) SaveQuestStatus(_ context.Context, _ int64, _, _ string, _ *time.Time) error {
	return nil
}

func (r *noOpQuestRepo) SaveObjectiveProgress(_ context.Context, _ int64, _, _ string, _ int) error {
	return nil
}

func (r *noOpQuestRepo) LoadQuests(_ context.Context, _ int64) ([]quest.QuestRecord, error) {
	return nil, nil
}

// TestQuestList_NoActiveQuests verifies questList returns a non-nil event when no quests are active.
func TestQuestList_NoActiveQuests(t *testing.T) {
	svc, uid := newQuestTestServer(t, quest.QuestRegistry{})
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	evt := svc.questList(sess)
	require.NotNil(t, evt, "questList must return a non-nil ServerEvent")
	msg := evt.GetMessage()
	require.NotNil(t, msg, "ServerEvent must contain a MessageEvent")
}

// TestQuestLog_UnknownID verifies questLog returns a non-nil event for an unknown quest ID.
func TestQuestLog_UnknownID(t *testing.T) {
	svc, uid := newQuestTestServer(t, quest.QuestRegistry{})
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	evt := svc.questLog(sess, "nonexistent_quest_id")
	require.NotNil(t, evt, "questLog must return a non-nil ServerEvent")
	msg := evt.GetMessage()
	require.NotNil(t, msg, "ServerEvent must contain a MessageEvent")
}
