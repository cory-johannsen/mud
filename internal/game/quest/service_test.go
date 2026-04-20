package quest_test

import (
	"context"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/quest"
)

// fakeRepo is an in-memory QuestRepository for unit testing.
type fakeRepo struct {
	statuses    map[string]string
	progress    map[string]map[string]int
	completedAt map[string]*time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		statuses:    make(map[string]string),
		progress:    make(map[string]map[string]int),
		completedAt: make(map[string]*time.Time),
	}
}

func (f *fakeRepo) SaveQuestStatus(_ context.Context, _ int64, questID, status string, completedAt *time.Time) error {
	f.statuses[questID] = status
	f.completedAt[questID] = completedAt
	return nil
}

func (f *fakeRepo) SaveObjectiveProgress(_ context.Context, _ int64, questID, objectiveID string, progress int) error {
	if f.progress[questID] == nil {
		f.progress[questID] = make(map[string]int)
	}
	f.progress[questID][objectiveID] = progress
	return nil
}

func (f *fakeRepo) LoadQuests(_ context.Context, _ int64) ([]quest.QuestRecord, error) {
	return nil, nil
}

// fakeSession is a minimal in-memory SessionState for unit testing.
type fakeSession struct {
	activeQuests    map[string]*quest.ActiveQuest
	completedQuests map[string]*time.Time
	currency        int
	backpack        *inventory.Backpack
}

func newFakeSession() *fakeSession {
	return &fakeSession{
		activeQuests:    make(map[string]*quest.ActiveQuest),
		completedQuests: make(map[string]*time.Time),
	}
}

func (s *fakeSession) GetActiveQuests() map[string]*quest.ActiveQuest { return s.activeQuests }
func (s *fakeSession) GetCompletedQuests() map[string]*time.Time      { return s.completedQuests }
func (s *fakeSession) GetBackpack() *inventory.Backpack               { return s.backpack }
func (s *fakeSession) GetCurrency() int                               { return s.currency }
func (s *fakeSession) AddCurrency(delta int)                          { s.currency += delta }

func killQuestDef() *quest.QuestDef {
	return &quest.QuestDef{
		ID: "kill_rats", Title: "Kill Some Rats", GiverNPCID: "sally",
		Objectives: []quest.QuestObjective{
			{ID: "o1", Type: "kill", Description: "Kill 3 rats", TargetID: "rat", Quantity: 3},
		},
		Rewards: quest.QuestRewards{XP: 100, Credits: 50},
	}
}

func TestService_GetOfferable_ReturnsQuestForEligiblePlayer(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	offerable := svc.GetOfferable(sess, []string{"kill_rats"})
	if len(offerable) != 1 {
		t.Fatalf("expected 1 offerable quest, got %d", len(offerable))
	}
}

func TestService_GetOfferable_ExcludesActiveQuest(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{}}
	offerable := svc.GetOfferable(sess, []string{"kill_rats"})
	if len(offerable) != 0 {
		t.Fatal("expected no offerable quests when already active")
	}
}

func TestService_GetOfferable_ExcludesCompletedNonRepeatable(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	sess.completedQuests["kill_rats"] = new(time.Now())
	offerable := svc.GetOfferable(sess, []string{"kill_rats"})
	if len(offerable) != 0 {
		t.Fatal("expected no offerable quests when completed non-repeatable")
	}
}

func TestService_GetOfferable_PrerequisiteNotMet(t *testing.T) {
	def := killQuestDef()
	def.Prerequisites = []string{"prereq_quest"}
	reg := quest.QuestRegistry{"kill_rats": def}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	offerable := svc.GetOfferable(sess, []string{"kill_rats"})
	if len(offerable) != 0 {
		t.Fatal("expected no offerable quests when prerequisite not met")
	}
}

func TestService_GetOfferable_AbandonedPrerequisiteNotSatisfied(t *testing.T) {
	def := killQuestDef()
	def.Prerequisites = []string{"prereq_quest"}
	reg := quest.QuestRegistry{"kill_rats": def}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	sess.completedQuests["prereq_quest"] = nil // abandoned sentinel
	offerable := svc.GetOfferable(sess, []string{"kill_rats"})
	if len(offerable) != 0 {
		t.Fatal("abandoned prerequisite must not satisfy prerequisite check")
	}
}

func TestService_Accept_AddsToActiveQuests(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	title, objDescs, err := svc.Accept(context.Background(), sess, 1, "kill_rats")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if title != "Kill Some Rats" {
		t.Fatalf("unexpected title: %q", title)
	}
	if len(objDescs) != 1 {
		t.Fatalf("expected 1 objective description, got %d", len(objDescs))
	}
	if _, ok := sess.activeQuests["kill_rats"]; !ok {
		t.Fatal("expected kill_rats in ActiveQuests")
	}
	if repo.statuses["kill_rats"] != "active" {
		t.Fatalf("expected DB status active, got %q", repo.statuses["kill_rats"])
	}
}

func TestService_RecordKill_IncrementsProgress(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{"o1": 0}}
	if _, err := svc.RecordKill(context.Background(), sess, 1, "rat"); err != nil {
		t.Fatalf("RecordKill: %v", err)
	}
	if sess.activeQuests["kill_rats"].ObjectiveProgress["o1"] != 1 {
		t.Fatalf("expected progress 1, got %d", sess.activeQuests["kill_rats"].ObjectiveProgress["o1"])
	}
}

func TestService_RecordKill_ClampsAtQuantity(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{"o1": 3}}
	if _, err := svc.RecordKill(context.Background(), sess, 1, "rat"); err != nil {
		t.Fatalf("RecordKill: %v", err)
	}
	if sess.activeQuests["kill_rats"].ObjectiveProgress["o1"] != 3 {
		t.Fatalf("expected clamped at 3, got %d", sess.activeQuests["kill_rats"].ObjectiveProgress["o1"])
	}
}

func TestService_Abandon_NonRepeatableRequiresConfirm(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{}}
	msg, err := svc.Abandon(context.Background(), sess, 1, "kill_rats", false)
	if err != nil {
		t.Fatalf("Abandon: %v", err)
	}
	if msg == "" {
		t.Fatal("expected confirmation prompt message")
	}
	if _, ok := sess.activeQuests["kill_rats"]; !ok {
		t.Fatal("quest should still be active without confirm")
	}
}

func TestService_Abandon_NonRepeatableWithConfirm(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{}}
	_, err := svc.Abandon(context.Background(), sess, 1, "kill_rats", true)
	if err != nil {
		t.Fatalf("Abandon with confirm: %v", err)
	}
	if _, ok := sess.activeQuests["kill_rats"]; ok {
		t.Fatal("quest should be removed from ActiveQuests after abandon")
	}
	if _, ok := sess.completedQuests["kill_rats"]; !ok {
		t.Fatal("non-repeatable abandoned quest should be in CompletedQuests")
	}
	if sess.completedQuests["kill_rats"] != nil {
		t.Fatal("abandoned non-repeatable should have nil completedAt sentinel")
	}
}

func TestService_RecordFetch_IncrementsProgress(t *testing.T) {
	fetchDef := &quest.QuestDef{
		ID: "fetch_herbs", Title: "Gather Herbs", GiverNPCID: "herbalist",
		Objectives: []quest.QuestObjective{
			{ID: "o1", Type: "fetch", Description: "Gather 2 herbs", TargetID: "herb", Quantity: 2},
		},
	}
	reg := quest.QuestRegistry{"fetch_herbs": fetchDef}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["fetch_herbs"] = &quest.ActiveQuest{QuestID: "fetch_herbs", ObjectiveProgress: map[string]int{"o1": 0}}
	if _, err := svc.RecordFetch(context.Background(), sess, 1, "herb", 1); err != nil {
		t.Fatalf("RecordFetch: %v", err)
	}
	if sess.activeQuests["fetch_herbs"].ObjectiveProgress["o1"] != 1 {
		t.Fatalf("expected progress 1, got %d", sess.activeQuests["fetch_herbs"].ObjectiveProgress["o1"])
	}
}

func TestService_RecordKill_CompletesQuestWhenAllObjectivesMet(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)
	sess := newFakeSession()
	// Set progress to 2 out of 3 — one more kill will complete the quest.
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{"o1": 2}}
	if _, err := svc.RecordKill(context.Background(), sess, 1, "rat"); err != nil {
		t.Fatalf("RecordKill: %v", err)
	}
	if _, still := sess.activeQuests["kill_rats"]; still {
		t.Fatal("quest should be removed from ActiveQuests on completion")
	}
	if _, done := sess.completedQuests["kill_rats"]; !done {
		t.Fatal("quest should be in CompletedQuests on completion")
	}
}

// fakeXPAwarder implements XPAwarder for unit testing.
// It returns a pre-configured set of level-up messages on every AwardXPAmount call.
type fakeXPAwarder struct {
	levelUpMsgs []string
}

func (f *fakeXPAwarder) AwardXPAmount(_ context.Context, _ quest.SessionState, _ int64, _ int) ([]string, error) {
	return f.levelUpMsgs, nil
}

// TestService_Complete_IncludesLevelUpMessages verifies that when AwardXPAmount returns
// level-up messages, Complete() includes them in its return value.
//
// REQ-58-1: quest.Service.Complete() MUST include level-up messages returned by AwardXPAmount
// in its return value so callers can notify the player of pending boosts immediately upon
// quest completion.
func TestService_Complete_IncludesLevelUpMessages(t *testing.T) {
	awarder := &fakeXPAwarder{levelUpMsgs: []string{"You have levelled up!", "You have 1 pending stat boost."}}
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, awarder, nil, nil)
	sess := newFakeSession()
	// Progress at 2/3 — one more kill triggers completion.
	sess.activeQuests["kill_rats"] = &quest.ActiveQuest{QuestID: "kill_rats", ObjectiveProgress: map[string]int{"o1": 2}}

	msgs, err := svc.RecordKill(context.Background(), sess, 1, "rat")
	if err != nil {
		t.Fatalf("RecordKill: %v", err)
	}

	for _, want := range awarder.levelUpMsgs {
		found := false
		for _, got := range msgs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("level-up message %q not found in returned messages: %v", want, msgs)
		}
	}
}

func TestService_HydrateSession_LoadsActiveAndCompleted(t *testing.T) {
	reg := quest.QuestRegistry{"kill_rats": killQuestDef()}
	svc := quest.NewService(reg, newFakeRepo(), nil, nil, nil)
	sess := newFakeSession()
	records := []quest.QuestRecord{
		{CharacterID: 1, QuestID: "kill_rats", Status: "active", Progress: map[string]int{"o1": 1}},
		{CharacterID: 1, QuestID: "other_quest", Status: "completed", CompletedAt: new(time.Now())},
	}
	svc.HydrateSession(sess, records)
	if aq, ok := sess.activeQuests["kill_rats"]; !ok {
		t.Fatal("expected kill_rats in ActiveQuests")
	} else if aq.ObjectiveProgress["o1"] != 1 {
		t.Fatalf("expected progress 1, got %d", aq.ObjectiveProgress["o1"])
	}
	if _, ok := sess.completedQuests["other_quest"]; !ok {
		t.Fatal("expected other_quest in CompletedQuests")
	}
}

// REQ-QC-1: RecordKillWithResults MUST return a CompletionResult when a kill completes a quest.
func TestRecordKillWithResults_ReturnsCompletionResult(t *testing.T) {
	def := &quest.QuestDef{
		ID: "q1", Title: "Rat Hunt", Description: "Kill some rats.",
		Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", TargetID: "rat", Quantity: 1}},
		Rewards:    quest.QuestRewards{XP: 100, Credits: 50},
	}
	repo := newFakeRepo()
	svc := quest.NewService(quest.QuestRegistry{"q1": def}, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["q1"] = &quest.ActiveQuest{QuestID: "q1", ObjectiveProgress: map[string]int{"o1": 0}}

	results, msgs, err := svc.RecordKillWithResults(context.Background(), sess, 1, "rat")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 CompletionResult, got %d", len(results))
	}
	r := results[0]
	if r.QuestID != "q1" {
		t.Errorf("expected QuestID=q1, got %q", r.QuestID)
	}
	if r.Title != "Rat Hunt" {
		t.Errorf("expected Title=Rat Hunt, got %q", r.Title)
	}
	if r.XPReward != 100 {
		t.Errorf("expected XPReward=100, got %d", r.XPReward)
	}
	if r.CreditsReward != 50 {
		t.Errorf("expected CreditsReward=50, got %d", r.CreditsReward)
	}
	if len(msgs) == 0 {
		t.Error("expected completion messages")
	}
}

// TestRecordZoneMapUse_CompletesObjective verifies RecordZoneMapUse increments
// use_zone_map objectives matching the given zoneID and triggers completion.
func TestRecordZoneMapUse_CompletesObjective(t *testing.T) {
	def := &quest.QuestDef{
		ID:    "onboarding_find_zone_map",
		Title: "Find Your Bearings",
		Type:  "onboarding",
		Objectives: []quest.QuestObjective{
			{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	reg := quest.QuestRegistry{"onboarding_find_zone_map": def}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)

	sess := newFakeSession()
	sess.activeQuests["onboarding_find_zone_map"] = &quest.ActiveQuest{
		QuestID:           "onboarding_find_zone_map",
		ObjectiveProgress: map[string]int{"use_zone_map_obj": 0},
	}

	msgs, err := svc.RecordZoneMapUse(context.Background(), sess, 42, "felony_flats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected completion messages, got none")
	}
	if _, stillActive := sess.activeQuests["onboarding_find_zone_map"]; stillActive {
		t.Fatal("quest should be completed and removed from activeQuests")
	}
}

// TestRecordZoneMapUse_WrongZone verifies RecordZoneMapUse does not increment
// progress when the zoneID does not match the objective target_id.
func TestRecordZoneMapUse_WrongZone(t *testing.T) {
	def := &quest.QuestDef{
		ID:    "onboarding_find_zone_map",
		Title: "Find Your Bearings",
		Type:  "onboarding",
		Objectives: []quest.QuestObjective{
			{ID: "use_zone_map_obj", Type: "use_zone_map", Description: "Use the zone map", TargetID: "felony_flats", Quantity: 1},
		},
		Rewards: quest.QuestRewards{XP: 50},
	}
	reg := quest.QuestRegistry{"onboarding_find_zone_map": def}
	repo := newFakeRepo()
	svc := quest.NewService(reg, repo, nil, nil, nil)

	sess := newFakeSession()
	sess.activeQuests["onboarding_find_zone_map"] = &quest.ActiveQuest{
		QuestID:           "onboarding_find_zone_map",
		ObjectiveProgress: map[string]int{"use_zone_map_obj": 0},
	}

	msgs, err := svc.RecordZoneMapUse(context.Background(), sess, 42, "downtown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no messages for wrong zone, got %v", msgs)
	}
	aq := sess.activeQuests["onboarding_find_zone_map"]
	if aq.ObjectiveProgress["use_zone_map_obj"] != 0 {
		t.Fatal("progress should be unchanged for wrong zone")
	}
}

// REQ-QC-2: RecordKillWithResults MUST return empty results when no quest completes.
func TestRecordKillWithResults_EmptyWhenNoCompletion(t *testing.T) {
	def := &quest.QuestDef{
		ID: "q1", Title: "Rat Hunt", Description: "Kill some rats.",
		Objectives: []quest.QuestObjective{{ID: "o1", Type: "kill", TargetID: "rat", Quantity: 5}},
		Rewards:    quest.QuestRewards{XP: 100, Credits: 50},
	}
	repo := newFakeRepo()
	svc := quest.NewService(quest.QuestRegistry{"q1": def}, repo, nil, nil, nil)
	sess := newFakeSession()
	sess.activeQuests["q1"] = &quest.ActiveQuest{QuestID: "q1", ObjectiveProgress: map[string]int{"o1": 0}}

	results, msgs, err := svc.RecordKillWithResults(context.Background(), sess, 1, "rat")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 CompletionResults, got %d", len(results))
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d: %v", len(msgs), msgs)
	}
}
