package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/quest"
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
	view := evt.GetQuestGiverView()
	require.NotNil(t, view, "expected QuestGiverView, got: %T", evt.GetPayload())
	assert.Equal(t, "Gail", view.NpcName)
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
	view := evt.GetQuestGiverView()
	require.NotNil(t, view, "expected QuestGiverView for case-insensitive match")
	assert.Equal(t, "Gail", view.NpcName)
}

// newQuestGiverWithQuestsServer creates a server with a quest_giver NPC that has questID configured.
// The quest registry is backed by def. Returns (svc, uid, npcTemplateID).
func newQuestGiverWithQuestsServer(t *testing.T, questID string, def *quest.QuestDef) (*GameServiceServer, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	reg := quest.QuestRegistry{questID: def}
	svc.questSvc = quest.NewService(reg, &noOpQuestRepo{}, nil, nil, nil)

	uid := "tq_u1"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "tq_user",
		CharName:  "TQChar",
		RoomID:    "room_a",
		CurrentHP: 50,
		MaxHP:     100,
		Role:      "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID:      def.GiverNPCID,
		Name:    "Questor",
		NPCType: "quest_giver",
		Level:   1,
		MaxHP:   20,
		AC:      10,
		QuestGiver: &npc.QuestGiverConfig{
			QuestIDs:          []string{questID},
			PlaceholderDialog: []string{"I have work for you."},
		},
	}
	_, err = npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	return svc, uid
}

// TestHandleTalk_CompletesReadyKillQuest verifies that talking to a quest giver with
// a kill quest where all objectives are already satisfied completes the quest.
//
// Precondition: player has an active kill quest with all objectives at max progress.
// Postcondition: quest is removed from ActiveQuests after handleTalk.
func TestHandleTalk_CompletesReadyKillQuest(t *testing.T) {
	const questID = "test_kill_quest"
	def := &quest.QuestDef{
		ID:         questID,
		Title:      "Test Kill Quest",
		GiverNPCID: "test_giver",
		Objectives: []quest.QuestObjective{
			{ID: "kill_rats", Type: "kill", Description: "Kill 2 rats", TargetID: "rat", Quantity: 2},
		},
		Rewards: quest.QuestRewards{XP: 100, Credits: 50},
	}
	svc, uid := newQuestGiverWithQuestsServer(t, questID, def)

	// Place the quest in the player's ActiveQuests with all objectives done.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.ActiveQuests[questID] = &quest.ActiveQuest{
		QuestID:           questID,
		ObjectiveProgress: map[string]int{"kill_rats": 2},
	}

	// Talk to the quest giver — should complete the quest.
	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "Questor"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Quest must be removed from ActiveQuests.
	_, stillActive := sess.ActiveQuests[questID]
	assert.False(t, stillActive, "quest should have been completed and removed from ActiveQuests")
}

// TestHandleTalk_DoesNotCompleteIncompleteKillQuest verifies that talking to a quest giver
// does NOT complete a kill quest whose objectives are not yet fully satisfied.
//
// Precondition: player has an active kill quest with progress below required.
// Postcondition: quest remains in ActiveQuests after handleTalk.
func TestHandleTalk_DoesNotCompleteIncompleteKillQuest(t *testing.T) {
	const questID = "test_partial_quest"
	def := &quest.QuestDef{
		ID:         questID,
		Title:      "Partial Quest",
		GiverNPCID: "test_giver",
		Objectives: []quest.QuestObjective{
			{ID: "kill_rats", Type: "kill", Description: "Kill 3 rats", TargetID: "rat", Quantity: 3},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	svc, uid := newQuestGiverWithQuestsServer(t, questID, def)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	// Only 1 of 3 objectives done.
	sess.ActiveQuests[questID] = &quest.ActiveQuest{
		QuestID:           questID,
		ObjectiveProgress: map[string]int{"kill_rats": 1},
	}

	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "Questor"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Quest must remain active.
	_, stillActive := sess.ActiveQuests[questID]
	assert.True(t, stillActive, "incomplete quest should remain in ActiveQuests")
}

// TestHandleTalk_CompletesNoQuestForOtherNPC verifies that a quest given by a
// different NPC is not completed when talking to an unrelated quest giver.
//
// Precondition: player has a ready quest, but its giver NPC is not the one being talked to.
// Postcondition: quest remains in ActiveQuests.
func TestHandleTalk_CompletesNoQuestForOtherNPC(t *testing.T) {
	const questID = "other_npc_quest"
	// Quest giver NPC ID is "other_giver"; the NPC we talk to has ID "test_giver".
	def := &quest.QuestDef{
		ID:         questID,
		Title:      "Other NPC Quest",
		GiverNPCID: "other_giver", // different from the NPC being talked to
		Objectives: []quest.QuestObjective{
			{ID: "kill_one", Type: "kill", Description: "Kill 1 rat", TargetID: "rat", Quantity: 1},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	// Build server manually so the NPC template ID ("test_giver") differs from def.GiverNPCID ("other_giver").
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)
	svc.questSvc = quest.NewService(quest.QuestRegistry{questID: def}, &noOpQuestRepo{}, nil, nil, nil)

	uid := "other_npc_u"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "other_npc_user", CharName: "OtherChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)

	// Spawn NPC with ID "test_giver" — does NOT match the quest's GiverNPCID "other_giver".
	tmpl := &npc.Template{
		ID: "test_giver", Name: "LocalGiver", NPCType: "quest_giver",
		Level: 1, MaxHP: 10, AC: 10,
		QuestGiver: &npc.QuestGiverConfig{
			QuestIDs:          []string{questID},
			PlaceholderDialog: []string{"Nothing here."},
		},
	}
	_, err = npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.ActiveQuests[questID] = &quest.ActiveQuest{
		QuestID:           questID,
		ObjectiveProgress: map[string]int{"kill_one": 1},
	}

	evt, err := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "LocalGiver"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Quest should NOT be completed — the NPC's template ID "test_giver" ≠ def.GiverNPCID "other_giver".
	_, stillActive := sess.ActiveQuests[questID]
	assert.True(t, stillActive, "quest for a different NPC should not be completed")
}

// TestProperty_HandleTalk_CompleteOnlyReadyQuests verifies that for any mix of
// ready and incomplete quests, exactly the ready ones are completed.
func TestProperty_HandleTalk_CompleteOnlyReadyQuests(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcManager := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

	uid := "prop_tq_u"
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "prop_tq_user", CharName: "PropTQChar",
		RoomID: "room_a", CurrentHP: 50, MaxHP: 100, Role: "player",
	})
	require.NoError(t, err)

	tmpl := &npc.Template{
		ID: "prop_giver", Name: "PropQuestor", NPCType: "quest_giver",
		Level: 1, MaxHP: 10, AC: 10,
		QuestGiver: &npc.QuestGiverConfig{
			QuestIDs:          []string{"q_ready", "q_partial"},
			PlaceholderDialog: []string{"Work awaits."},
		},
	}
	inst, err := npcManager.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	defer npcManager.Remove(inst.ID)

	rapid.Check(t, func(rt *rapid.T) {
		required := rapid.IntRange(1, 5).Draw(rt, "required")
		progress := rapid.IntRange(0, required+1).Draw(rt, "progress")

		readyDef := &quest.QuestDef{
			ID: "q_ready", Title: "Ready Quest", GiverNPCID: "prop_giver",
			Objectives: []quest.QuestObjective{
				{ID: "obj1", Type: "kill", Description: "Kill enemies", TargetID: "enemy", Quantity: required},
			},
			Rewards: quest.QuestRewards{XP: 10},
		}
		partialDef := &quest.QuestDef{
			ID: "q_partial", Title: "Partial Quest", GiverNPCID: "prop_giver",
			Objectives: []quest.QuestObjective{
				{ID: "obj2", Type: "kill", Description: "Kill more", TargetID: "enemy2", Quantity: required + 1},
			},
			Rewards: quest.QuestRewards{XP: 10},
		}
		reg := quest.QuestRegistry{"q_ready": readyDef, "q_partial": partialDef}
		svc.questSvc = quest.NewService(reg, &noOpQuestRepo{}, nil, nil, nil)

		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Fatal("session not found")
		}
		// Reset quest state each iteration.
		sess.ActiveQuests = map[string]*quest.ActiveQuest{
			"q_ready": {
				QuestID:           "q_ready",
				ObjectiveProgress: map[string]int{"obj1": required}, // exactly at required — should complete
			},
			"q_partial": {
				QuestID:           "q_partial",
				ObjectiveProgress: map[string]int{"obj2": progress % required}, // below required — should NOT complete
			},
		}
		if progress%required >= required {
			sess.ActiveQuests["q_partial"].ObjectiveProgress["obj2"] = required - 1
		}
		sess.CompletedQuests = map[string]*time.Time{}

		evt, talkErr := svc.handleTalk(uid, &gamev1.TalkRequest{NpcName: "PropQuestor"})
		if talkErr != nil {
			rt.Fatal("handleTalk error:", talkErr)
		}
		if evt == nil {
			rt.Fatal("handleTalk returned nil event")
		}

		// q_ready should always be completed.
		if _, stillActive := sess.ActiveQuests["q_ready"]; stillActive {
			rt.Fatalf("q_ready (all objectives met) must be completed but is still in ActiveQuests")
		}

		// q_partial should remain active (progress < required).
		if _, stillActive := sess.ActiveQuests["q_partial"]; !stillActive {
			rt.Fatalf("q_partial (objectives not met) must remain in ActiveQuests but was completed")
		}
	})
}

func TestProperty_HandleTalk_AlwaysReturnsQuestGiverView(t *testing.T) {
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

		view := evt.GetQuestGiverView()
		if view == nil {
			rt.Fatalf("expected QuestGiverView, got payload type %T", evt.GetPayload())
		}
		if view.NpcName != "PropGiver" {
			rt.Fatalf("expected NpcName=PropGiver, got %q", view.NpcName)
		}
	})
}
