package gameserver

// REQ-TTA-1: Tech trainers MUST resolve pending L2+ prepared and spontaneous slots.
// REQ-TTA-2: L2+ slots always require a trainer (unconditionally deferred).
// REQ-TTA-12: Pending slots are persisted in DB and decremented on resolution.

import (
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
)

// newTechTrainerTestServer builds a minimal GameServiceServer with:
//   - a "neural" tech_trainer NPC in room_a offering level 2
//   - a player with 500 credits in room_a
//   - a technology registry with one neural L2 prepared tech
//
// Returns (svc, uid, trainerName).
func newTechTrainerTestServer(t *testing.T) (*GameServiceServer, string, string) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcMgr)

	// Register a neural L2 tech in the registry.
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

	trainerName := "Mira Synapse"
	tmpl := &npc.Template{
		ID:      "test_neural_trainer",
		Name:    trainerName,
		NPCType: "tech_trainer",
		Level:   3,
		MaxHP:   20,
		AC:      11,
		TechTrainer: &npc.TechTrainerConfig{
			Tradition:     "neural",
			OfferedLevels: []int{2},
			BaseCost:      100,
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	uid := "tt_u1"
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "tt_user",
		CharName:  "TTChar",
		RoomID:    "room_a",
		CurrentHP: 30,
		MaxHP:     30,
		Role:      "player",
		Level:     3,
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500

	// Give the player a pending neural L2 prepared slot.
	sess.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
		3: {
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{2: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "neural_strike", Level: 2},
				},
			},
		},
	}
	if sess.PreparedTechs == nil {
		sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
	}
	if sess.CompletedQuests == nil {
		sess.CompletedQuests = make(map[string]*time.Time)
	}
	if sess.ActiveQuests == nil {
		sess.ActiveQuests = make(map[string]*questpkg.ActiveQuest)
	}

	return svc, uid, trainerName
}

// TestHandleTrainTech_NoTrainerInRoom verifies that handleTrainTech returns a denial
// when no NPC with the given name is in the player's room.
//
// Precondition: Player in room_a; no NPC named "Ghost" in the room.
// Postcondition: Response is non-nil; message is non-empty denial.
func TestHandleTrainTech_NoTrainerInRoom(t *testing.T) {
	svc, uid, _ := newTechTrainerTestServer(t)
	evt, err := svc.handleTrainTech(uid, "Ghost", "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "REQ-TTA-1: denial message must be non-empty when trainer not found")
}

// TestHandleTrainTech_InsufficientFunds verifies that handleTrainTech denies training
// when the player cannot afford the cost.
//
// Precondition: Player has 0 credits; trainer present; pending slot exists.
// Postcondition: Response is a denial; currency unchanged; pending slot not consumed.
func TestHandleTrainTech_InsufficientFunds(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.Currency = 0

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "denial message must be non-empty")
	assert.Contains(t, msg, "credits", "REQ-TTA-1: denial message must mention credits")
	// Pending grant should still be present.
	assert.NotEmpty(t, sess.PendingTechGrants, "REQ-TTA-1: pending grant must not be consumed on denial")
}

// TestHandleTrainTech_Success verifies that handleTrainTech fills a pending slot,
// deducts currency, and removes the grant from PendingTechGrants.
//
// Precondition: Player has funds; trainer present; pending neural L2 slot exists.
// Postcondition: PreparedTechs[2] contains "neural_strike"; currency deducted; PendingTechGrants empty.
func TestHandleTrainTech_Success(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	initialCurrency := sess.Currency

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "success message must be non-empty")

	// Slot filled.
	require.NotNil(t, sess.PreparedTechs, "REQ-TTA-1: PreparedTechs must be populated")
	slots := sess.PreparedTechs[2]
	require.NotEmpty(t, slots, "REQ-TTA-1: level-2 slots must be non-empty")
	found := false
	for _, s := range slots {
		if s.TechID == "neural_strike" {
			found = true
		}
	}
	assert.True(t, found, "REQ-TTA-1: neural_strike must be in prepared level-2 slots")

	// Currency deducted.
	cost := 100 * 2 // BaseCost * level
	assert.Equal(t, initialCurrency-cost, sess.Currency, "REQ-TTA-1: cost must be deducted")

	// Pending grant consumed or removed.
	hasSlot := false
	for _, g := range sess.PendingTechGrants {
		if g.Prepared != nil {
			if g.Prepared.SlotsByLevel[2] > 0 {
				hasSlot = true
			}
		}
	}
	assert.False(t, hasSlot, "REQ-TTA-1: pending slot must be consumed after training")
}

// TestHandleTrainTech_PrereqNotMet verifies that when trainer prerequisites are not met,
// training is denied.
//
// Precondition: Trainer requires quest "find_trainer_quest" to be completed; player has not completed it.
// Postcondition: Response is a denial mentioning the unmet prerequisite.
func TestHandleTrainTech_PrereqNotMet(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcMgr)

	techReg := technology.NewRegistry()
	techReg.Register(&technology.TechnologyDef{
		ID:        "neural_beam",
		Name:      "Neural Beam",
		Tradition: technology.TraditionNeural,
		Level:     2,
		UsageType: technology.UsagePrepared,
		ActionCost: 1,
		Range:     technology.RangeMelee,
		Targets:   technology.TargetsSingle,
		Duration:  "instantaneous",
	})
	svc.techRegistry = techReg

	trainerName := "Gated Trainer"
	tmpl := &npc.Template{
		ID:      "gated_neural_trainer",
		Name:    trainerName,
		NPCType: "tech_trainer",
		Level:   3,
		MaxHP:   20,
		AC:      11,
		TechTrainer: &npc.TechTrainerConfig{
			Tradition:     "neural",
			OfferedLevels: []int{2},
			BaseCost:      100,
			Prerequisites: &npc.TechTrainPrereqs{
				Operator: "and",
				Conditions: []npc.TechTrainCondition{
					{Type: "quest_complete", QuestID: "find_trainer_quest"},
				},
			},
		},
	}
	_, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	uid := "tt_prereq_u1"
	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       uid,
		Username:  "prereq_user",
		CharName:  "PrereqChar",
		RoomID:    "room_a",
		CurrentHP: 30,
		MaxHP:     30,
		Role:      "player",
		Level:     3,
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.Currency = 500
	sess.CompletedQuests = make(map[string]*time.Time)
	sess.ActiveQuests = make(map[string]*questpkg.ActiveQuest)
	sess.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
		3: {
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{2: 1},
				Pool: []ruleset.PreparedEntry{
					{ID: "neural_beam", Level: 2},
				},
			},
		},
	}
	if sess.PreparedTechs == nil {
		sess.PreparedTechs = make(map[int][]*session.PreparedSlot)
	}

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_beam")
	require.NoError(t, err)
	require.NotNil(t, evt)
	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "REQ-TTA-1: denial message must be non-empty when prereq not met")
}

// TestProperty_HandleTrainTech_CurrencyNeverNegative verifies that handleTrainTech
// never results in negative currency for the player.
//
// Precondition: player has currency in range [0, 1000].
// Postcondition: currency is never negative after any train attempt.
func TestProperty_HandleTrainTech_CurrencyNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid, trainerName := newTechTrainerTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.Currency = rapid.IntRange(0, 1000).Draw(rt, "currency")
		_, _ = svc.handleTrainTech(uid, trainerName, "neural_strike")
		sess, _ = svc.sessions.GetPlayer(uid)
		if sess.Currency < 0 {
			rt.Fatalf("currency went negative: %d", sess.Currency)
		}
	})
}

// TestDoTrainTech_WizardModelPopulatesKnownTechs verifies that after training a prepared tech
// with a wizard casting model, the tech appears in sess.KnownTechs.
//
// Precondition: Player has wizard casting model and a pending prepared L2 neural tech.
// Postcondition: sess.KnownTechs[2] contains "neural_strike" after successful training.
// REQ-TC-11: Trainer populates KnownTechs for wizard and ranger models.
func TestDoTrainTech_WizardModelPopulatesKnownTechs(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.CastingModel = ruleset.CastingModelWizard
	sess.Currency = 500

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "success message must be non-empty")

	sess, _ = svc.sessions.GetPlayer(uid)
	require.NotNil(t, sess.KnownTechs, "REQ-TC-11: KnownTechs must be populated for wizard model")
	found := false
	for _, id := range sess.KnownTechs[2] {
		if id == "neural_strike" {
			found = true
		}
	}
	assert.True(t, found, "REQ-TC-11: neural_strike must appear in KnownTechs[2] for wizard model")
}

// TestDoTrainTech_RangerModelPopulatesKnownTechs verifies that after training a prepared tech
// with a ranger casting model, the tech appears in sess.KnownTechs.
//
// Precondition: Player has ranger casting model and a pending prepared L2 neural tech.
// Postcondition: sess.KnownTechs[2] contains "neural_strike" after successful training.
// REQ-TC-11: Trainer populates KnownTechs for wizard and ranger models.
func TestDoTrainTech_RangerModelPopulatesKnownTechs(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.CastingModel = ruleset.CastingModelRanger
	sess.Currency = 500

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "success message must be non-empty")

	sess, _ = svc.sessions.GetPlayer(uid)
	require.NotNil(t, sess.KnownTechs, "REQ-TC-11: KnownTechs must be populated for ranger model")
	found := false
	for _, id := range sess.KnownTechs[2] {
		if id == "neural_strike" {
			found = true
		}
	}
	assert.True(t, found, "REQ-TC-11: neural_strike must appear in KnownTechs[2] for ranger model")
}

// TestDoTrainTech_DruidModelDoesNotPopulateKnownTechs verifies that after training a prepared tech
// with a druid casting model, KnownTechs is NOT populated (druid uses the full pool at rest).
//
// Precondition: Player has druid casting model and a pending prepared L2 neural tech.
// Postcondition: sess.KnownTechs does NOT contain "neural_strike" after training.
// REQ-TC-22: For druid model, trainer assigns PreparedTechs but does NOT add to KnownTechs.
func TestDoTrainTech_DruidModelDoesNotPopulateKnownTechs(t *testing.T) {
	svc, uid, trainerName := newTechTrainerTestServer(t)
	sess, _ := svc.sessions.GetPlayer(uid)
	sess.CastingModel = ruleset.CastingModelDruid
	sess.Currency = 500

	evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "success message must be non-empty")

	sess, _ = svc.sessions.GetPlayer(uid)
	// PreparedTechs must still be filled.
	require.NotNil(t, sess.PreparedTechs, "REQ-TC-22: PreparedTechs must be populated for druid model")
	slots := sess.PreparedTechs[2]
	require.NotEmpty(t, slots, "REQ-TC-22: level-2 prepared slots must be non-empty for druid model")

	// KnownTechs must NOT contain the tech.
	for _, ids := range sess.KnownTechs {
		for _, id := range ids {
			assert.NotEqual(t, "neural_strike", id,
				"REQ-TC-22: neural_strike must NOT appear in KnownTechs for druid model")
		}
	}
}

// TestProperty_DoTrainTech_WizardKnownTechsNeverEmpty verifies that for a wizard model,
// KnownTechs is never empty after a successful training.
//
// Precondition: Player has wizard casting model; training succeeds.
// Postcondition: KnownTechs is non-empty after successful training.
func TestProperty_DoTrainTech_WizardKnownTechsNeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, uid, trainerName := newTechTrainerTestServer(t)
		sess, _ := svc.sessions.GetPlayer(uid)
		sess.CastingModel = ruleset.CastingModelWizard
		sess.Currency = 500

		evt, err := svc.handleTrainTech(uid, trainerName, "neural_strike")
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if evt == nil {
			rt.Fatalf("expected non-nil event")
		}
		sess, _ = svc.sessions.GetPlayer(uid)
		if len(sess.KnownTechs) == 0 {
			rt.Fatalf("REQ-TC-11: KnownTechs must be non-empty after wizard training")
		}
	})
}
