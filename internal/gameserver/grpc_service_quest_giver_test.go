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

func newQuestGiverTestServer(t *testing.T, dialog []string) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "talk_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "talk_user", CharName: "TalkChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)

	if dialog != nil {
		tmpl := &npc.Template{
			ID: "test_qg", Name: "Gail", NPCType: "quest_giver",
			Level: 3, MaxHP: 18, AC: 11,
			QuestGiver: &npc.QuestGiverConfig{PlaceholderDialog: dialog},
		}
		_, err = npcManager.Spawn(tmpl, "room_a")
		require.NoError(t, err)
	}
	return svc, uid
}

func TestHandleTalk_QuestGiverFound(t *testing.T) {
	dialog := []string{"Hello, stranger.", "Got work if you want it."}
	svc, uid := newQuestGiverTestServer(t, dialog)
	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "Gail"})
	require.NoError(t, err)
	content := evt.GetMessage().Content
	assert.True(t,
		content == `Gail says: "Hello, stranger."` || content == `Gail says: "Got work if you want it."`,
		"unexpected response: %s", content,
	)
}

func TestHandleTalk_NPCNotInRoom(t *testing.T) {
	svc, uid := newQuestGiverTestServer(t, nil) // no NPC spawned
	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "nobody"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "nobody")
}

func TestHandleTalk_WrongNPCType(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "talk_u2"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "talk_user2", CharName: "TalkChar2",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)

	// Spawn a combat NPC (not quest_giver)
	combatTmpl := &npc.Template{
		ID: "bandit", Name: "Bandit", NPCType: "combat",
		Level: 2, MaxHP: 20, AC: 10,
	}
	_, err = npcManager.Spawn(combatTmpl, "room_a")
	require.NoError(t, err)

	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "Bandit"})
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "Bandit")
	assert.Contains(t, evt.GetMessage().Content, "No one named")
}

func TestHandleTalk_CaseInsensitiveMatch(t *testing.T) {
	dialog := []string{"What do you want?"}
	svc, uid := newQuestGiverTestServer(t, dialog)
	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "gail"}) // lowercase
	require.NoError(t, err)
	assert.Contains(t, evt.GetMessage().Content, "What do you want?")
}

func TestProperty_HandleTalk_AlwaysReturnsDialogLine(t *testing.T) {
	// Create the server once outside rapid.Check to avoid data races with the
	// zap test logger (which is tied to the outer *testing.T and races when
	// testWorldAndSession is called from within rapid's goroutine).
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "prop_talk_u"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "prop_talk", CharName: "PropChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a non-empty dialog slice (1–10 arbitrary strings).
		dialog := rapid.SliceOfN(rapid.StringMatching(`[A-Za-z .,!?]{1,40}`), 1, 10).Draw(rt, "dialog")

		tmpl := &npc.Template{
			ID: "prop_qg", Name: "PropGiver", NPCType: "quest_giver",
			Level: 1, MaxHP: 10, AC: 10,
			QuestGiver: &npc.QuestGiverConfig{PlaceholderDialog: dialog},
		}
		inst, spawnErr := npcManager.Spawn(tmpl, "room_a")
		if spawnErr != nil {
			rt.Fatal("spawn failed:", spawnErr)
		}

		evt, talkErr := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "PropGiver"})
		_ = npcManager.Remove(inst.ID)
		if talkErr != nil {
			rt.Fatal("handleTalk failed:", talkErr)
		}

		content := evt.GetMessage().Content
		found := false
		for _, line := range dialog {
			if content == `PropGiver says: "`+line+`"` {
				found = true
				break
			}
		}
		rt.Log("response:", content)
		if !found {
			rt.Fatalf("response %q is not one of the dialog lines", content)
		}
	})
}
