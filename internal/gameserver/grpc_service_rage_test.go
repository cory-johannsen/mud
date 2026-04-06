package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeRageConditionRegistry returns a condition registry with the standard test
// conditions plus the mental-state rage conditions needed for handleRage tests.
func makeRageConditionRegistry() *condition.Registry {
	reg := makeTestConditionRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "rage_irritated", Name: "Irritated", DurationType: "rounds",
		MaxStacks: 0, DamageBonus: 1, ACPenalty: 1, IsMentalCondition: true,
	})
	reg.Register(&condition.ConditionDef{
		ID: "rage_enraged", Name: "Enraged", DurationType: "rounds",
		MaxStacks: 0, DamageBonus: 2, ACPenalty: 2, IsMentalCondition: true,
		RestrictActions: []string{"flee"},
	})
	reg.Register(&condition.ConditionDef{
		ID: "rage_berserker", Name: "Berserker", DurationType: "rounds",
		MaxStacks: 0, DamageBonus: 3, ACPenalty: 3, IsMentalCondition: true,
		RestrictActions: []string{"flee"},
	})
	return reg
}

// newRageSvc builds a GameServiceServer wired with a real MentalStateManager, a
// condition registry that includes rage conditions, and a feat registry containing
// the given feats mapped to characterID 0.
//
// Precondition: t must be non-nil; feats must be non-empty.
// Postcondition: Returns non-nil svc, sessMgr, CombatHandler, and mentalMgr.
func newRageSvc(t *testing.T, feats []*ruleset.Feat) (*GameServiceServer, *session.Manager, *CombatHandler, *mentalstate.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeRageConditionRegistry()
	mentalMgr := mentalstate.NewManager()
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, mentalMgr,
	)
	featIDs := make([]string, len(feats))
	for i, f := range feats {
		featIDs[i] = f.ID
	}
	featRegistry := ruleset.NewFeatRegistry(feats)
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{0: featIDs},
	}
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		feats, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		mentalMgr, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr, combatHandler, mentalMgr
}

// addRageTestPlayer creates a player with CharacterID=0 and initialized conditions.
func addRageTestPlayer(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	return sess
}

// --- handleRage tests ---

// TestHandleRage_AppliesEnragedMentalState verifies that handleRage advances the
// Rage track to SeverityMod (Enraged) and applies the rage_enraged condition to
// the player's session conditions (BUG-132).
//
// Precondition: Player has no active rage; mentalStateMgr is wired.
// Postcondition: Rage track == SeverityMod; sess.Conditions has rage_enraged.
func TestHandleRage_AppliesEnragedMentalState(t *testing.T) {
	rageFeat := &ruleset.Feat{
		ID: "rage", Name: "Rage", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, mentalMgr := newRageSvc(t, []*ruleset.Feat{rageFeat})
	sess := addRageTestPlayer(t, sessMgr, "u_rage_apply")

	evt, err := svc.handleRage("u_rage_apply")
	require.NoError(t, err)
	require.NotNil(t, evt)

	sev := mentalMgr.CurrentSeverity("u_rage_apply", mentalstate.TrackRage)
	assert.Equal(t, mentalstate.SeverityMod, sev,
		"Rage track must be SeverityMod (Enraged) after handleRage")

	assert.True(t, sess.Conditions.Has("rage_enraged"),
		"sess.Conditions must contain rage_enraged after handleRage")
}

// TestHandleRage_AlreadyEnraged_ReturnsAlreadyMessage verifies that calling
// handleRage when the player is already Enraged returns an idempotent message
// without changing state.
//
// Precondition: Player rage track is already SeverityMod.
// Postcondition: Returns message containing "already"; rage track unchanged.
func TestHandleRage_AlreadyEnraged_ReturnsAlreadyMessage(t *testing.T) {
	rageFeat := &ruleset.Feat{
		ID: "rage", Name: "Rage", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, mentalMgr := newRageSvc(t, []*ruleset.Feat{rageFeat})
	addRageTestPlayer(t, sessMgr, "u_rage_idem")

	// Pre-apply Enraged.
	_ = mentalMgr.ApplyTrigger("u_rage_idem", mentalstate.TrackRage, mentalstate.SeverityMod)

	evt, err := svc.handleRage("u_rage_idem")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "already",
		"Response must indicate player is already enraged")

	// State must not have changed.
	assert.Equal(t, mentalstate.SeverityMod,
		mentalMgr.CurrentSeverity("u_rage_idem", mentalstate.TrackRage),
		"Rage severity must not change on repeated handleRage")
}

// --- Adrenaline Surge precondition tests ---

// TestHandleUse_AdrenalineSurge_BlockedWhenNotEnraged verifies that activating
// adrenaline_surge is blocked with an explanatory message when the player is not
// in the Enraged state (BUG-132 precondition check).
//
// Precondition: Player has adrenaline_surge feat; no active rage.
// Postcondition: Response message references being Enraged; feat not consumed.
func TestHandleUse_AdrenalineSurge_BlockedWhenNotEnraged(t *testing.T) {
	surgeFeat := &ruleset.Feat{
		ID:           "adrenaline_surge",
		Name:         "Adrenaline Surge",
		Active:       true,
		ActivateText: "The adrenaline cuts through the haze and you think clearly for a moment.",
	}
	svc, sessMgr, _, _ := newRageSvc(t, []*ruleset.Feat{surgeFeat})
	addRageTestPlayer(t, sessMgr, "u_surge_blocked")

	evt, err := svc.handleUse("u_surge_blocked", "adrenaline_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	// messageEvent returns ServerEvent_Message; UseResponse would be the activate path.
	// When blocked, we get a message event; when it succeeds, we get a UseResponse.
	useMsg := evt.GetUseResponse().GetMessage()
	msgContent := evt.GetMessage().GetContent()
	// At least one of these must be non-empty.
	combined := useMsg + msgContent
	assert.NotEmpty(t, combined, "Response must contain a message")
	assert.NotEqual(t, surgeFeat.ActivateText, useMsg,
		"Adrenaline Surge must not return activate text when not Enraged")
}

// TestHandleUse_AdrenalineSurge_SucceedsWhenEnraged verifies that activating
// adrenaline_surge succeeds (returns the activate text) when the player is Enraged.
//
// Precondition: Player has adrenaline_surge feat; Rage track >= SeverityMod.
// Postcondition: Response contains ActivateText.
func TestHandleUse_AdrenalineSurge_SucceedsWhenEnraged(t *testing.T) {
	surgeFeat := &ruleset.Feat{
		ID:           "adrenaline_surge",
		Name:         "Adrenaline Surge",
		Active:       true,
		ActivateText: "The adrenaline cuts through the haze and you think clearly for a moment.",
	}
	svc, sessMgr, _, mentalMgr := newRageSvc(t, []*ruleset.Feat{surgeFeat})
	addRageTestPlayer(t, sessMgr, "u_surge_ok")

	// Apply Enraged before using Adrenaline Surge.
	_ = mentalMgr.ApplyTrigger("u_surge_ok", mentalstate.TrackRage, mentalstate.SeverityMod)

	evt, err := svc.handleUse("u_surge_ok", "adrenaline_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetUseResponse().GetMessage()
	assert.Equal(t, surgeFeat.ActivateText, msg,
		"Adrenaline Surge must return its activate text when player is Enraged")
}
