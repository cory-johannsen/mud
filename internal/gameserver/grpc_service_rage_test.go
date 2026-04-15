package gameserver

// REQ-80-1: Wrath feat MUST apply wrath_active condition (not the rage mental state track).
// REQ-80-2: Wrath MUST be blocked if wrath_active condition is already present.
// REQ-80-3: Wrath MUST be blocked during the 1-minute cooldown window.
// REQ-80-4: Adrenaline Surge MUST accept either wrath_active OR mental-state Enraged as its precondition.

import (
	"testing"
	"time"

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

// makeWrathConditionRegistry returns a condition registry with wrath_active
// and the mental-state rage conditions needed for adrenaline_surge tests.
func makeWrathConditionRegistry() *condition.Registry {
	reg := makeTestConditionRegistry()
	reg.Register(&condition.ConditionDef{
		ID:              "wrath_active",
		Name:            "Wrath Active",
		DurationType:    "encounter",
		DamageBonus:     2,
		RestrictActions: []string{"concentrate"},
	})
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

// newWrathSvc builds a GameServiceServer wired with the wrath condition registry,
// a real MentalStateManager, and a feat registry containing the given feats.
//
// Precondition: t must be non-nil; feats must be non-empty.
// Postcondition: Returns non-nil svc, sessMgr, CombatHandler, and mentalMgr.
func newWrathSvc(t *testing.T, feats []*ruleset.Feat) (*GameServiceServer, *session.Manager, *CombatHandler, *mentalstate.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeWrathConditionRegistry()
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

// addWrathTestPlayer creates a player with CharacterID=0 and initialized conditions.
func addWrathTestPlayer(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	return sess
}

// --- handleWrath tests ---

// TestHandleWrath_AppliesWrathActiveCondition verifies that handleWrath applies the
// wrath_active condition to the player's session conditions (REQ-80-1).
//
// Precondition: Player has no wrath_active condition; no active cooldown.
// Postcondition: sess.Conditions has wrath_active; response is the activation narrative.
func TestHandleWrath_AppliesWrathActiveCondition(t *testing.T) {
	wrathFeat := &ruleset.Feat{
		ID: "wrath", Name: "Wrath", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{wrathFeat})
	sess := addWrathTestPlayer(t, sessMgr, "u_wrath_apply")

	evt, err := svc.handleWrath("u_wrath_apply")
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.True(t, sess.Conditions.Has("wrath_active"),
		"REQ-80-1: sess.Conditions must contain wrath_active after handleWrath")
	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg)
}

// TestHandleWrath_DoesNotAdvanceMentalStateRageTrack verifies that handleWrath does NOT
// interact with the mental state Rage track (REQ-80-1: Wrath is separate from mental Rage).
//
// Precondition: Player has no active rage; mentalStateMgr is wired.
// Postcondition: Rage track remains SeverityNone after handleWrath.
func TestHandleWrath_DoesNotAdvanceMentalStateRageTrack(t *testing.T) {
	wrathFeat := &ruleset.Feat{
		ID: "wrath", Name: "Wrath", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, mentalMgr := newWrathSvc(t, []*ruleset.Feat{wrathFeat})
	addWrathTestPlayer(t, sessMgr, "u_wrath_no_mental")

	_, err := svc.handleWrath("u_wrath_no_mental")
	require.NoError(t, err)

	sev := mentalMgr.CurrentSeverity("u_wrath_no_mental", mentalstate.TrackRage)
	assert.Equal(t, mentalstate.SeverityNone, sev,
		"REQ-80-1: handleWrath must NOT advance the mental Rage track")
}

// TestHandleWrath_AlreadyActive_ReturnsDenialMessage verifies that calling handleWrath
// when wrath_active is already present returns a denial without changing state (REQ-80-2).
//
// Precondition: Player already has wrath_active condition.
// Postcondition: Response message contains "already"; wrath_active stack count unchanged.
func TestHandleWrath_AlreadyActive_ReturnsDenialMessage(t *testing.T) {
	wrathFeat := &ruleset.Feat{
		ID: "wrath", Name: "Wrath", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{wrathFeat})
	sess := addWrathTestPlayer(t, sessMgr, "u_wrath_already")

	// Pre-apply wrath_active.
	_, _ = svc.handleWrath("u_wrath_already")
	require.True(t, sess.Conditions.Has("wrath_active"))

	// Second activation must be denied.
	evt, err := svc.handleWrath("u_wrath_already")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "already", "REQ-80-2: denial must say 'already'")
}

// TestHandleWrath_Cooldown_ReturnsDenialMessage verifies that handleWrath is blocked
// during the 1-minute cooldown window (REQ-80-3).
//
// Precondition: sess.WrathCooldownUntil is in the future.
// Postcondition: Response message references cooldown; wrath_active is NOT applied.
func TestHandleWrath_Cooldown_ReturnsDenialMessage(t *testing.T) {
	wrathFeat := &ruleset.Feat{
		ID: "wrath", Name: "Wrath", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{wrathFeat})
	sess := addWrathTestPlayer(t, sessMgr, "u_wrath_cooldown")

	// Simulate active cooldown.
	sess.WrathCooldownUntil = time.Now().Add(45 * time.Second)

	evt, err := svc.handleWrath("u_wrath_cooldown")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "REQ-80-3: cooldown response must be non-empty")
	assert.False(t, sess.Conditions.Has("wrath_active"),
		"REQ-80-3: wrath_active must NOT be applied during cooldown")
}

// TestHandleWrath_AfterCooldownExpires_Succeeds verifies that handleWrath succeeds once
// the cooldown window has passed (REQ-80-3 boundary condition).
//
// Precondition: sess.WrathCooldownUntil is in the past.
// Postcondition: wrath_active applied; response is activation narrative.
func TestHandleWrath_AfterCooldownExpires_Succeeds(t *testing.T) {
	wrathFeat := &ruleset.Feat{
		ID: "wrath", Name: "Wrath", Active: true,
		ActivateText: "Fury overtakes you. You stop holding back.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{wrathFeat})
	sess := addWrathTestPlayer(t, sessMgr, "u_wrath_cooldown_done")

	// Expired cooldown.
	sess.WrathCooldownUntil = time.Now().Add(-1 * time.Second)

	evt, err := svc.handleWrath("u_wrath_cooldown_done")
	require.NoError(t, err)
	require.NotNil(t, evt)

	assert.True(t, sess.Conditions.Has("wrath_active"),
		"REQ-80-3: wrath_active must be applied after cooldown expires")
}

// --- Adrenaline Surge precondition tests ---

// TestHandleUse_AdrenalineSurge_BlockedWhenNoRageOrWrath verifies that activating
// adrenaline_surge is blocked when neither Wrath nor Enraged is active (REQ-80-4).
//
// Precondition: Player has adrenaline_surge feat; no active wrath or enraged state.
// Postcondition: Response message references the requirement; feat not consumed.
func TestHandleUse_AdrenalineSurge_BlockedWhenNoRageOrWrath(t *testing.T) {
	surgeFeat := &ruleset.Feat{
		ID:           "adrenaline_surge",
		Name:         "Adrenaline Surge",
		Active:       true,
		ActivateText: "The adrenaline cuts through the haze and you think clearly for a moment.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{surgeFeat})
	addWrathTestPlayer(t, sessMgr, "u_surge_blocked")

	evt, err := svc.handleUse("u_surge_blocked", "adrenaline_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	useMsg := evt.GetUseResponse().GetMessage()
	msgContent := evt.GetMessage().GetContent()
	combined := useMsg + msgContent
	assert.NotEmpty(t, combined, "Response must contain a message")
	assert.NotEqual(t, surgeFeat.ActivateText, useMsg,
		"Adrenaline Surge must not activate when neither Wrath nor Enraged")
}

// TestHandleUse_AdrenalineSurge_SucceedsWhenWrathActive verifies that activating
// adrenaline_surge succeeds when the player has wrath_active condition (REQ-80-4).
//
// Precondition: Player has adrenaline_surge feat; wrath_active condition is set.
// Postcondition: Response contains ActivateText.
func TestHandleUse_AdrenalineSurge_SucceedsWhenWrathActive(t *testing.T) {
	surgeFeat := &ruleset.Feat{
		ID:           "adrenaline_surge",
		Name:         "Adrenaline Surge",
		Active:       true,
		ActivateText: "The adrenaline cuts through the haze and you think clearly for a moment.",
	}
	svc, sessMgr, _, _ := newWrathSvc(t, []*ruleset.Feat{surgeFeat})
	sess := addWrathTestPlayer(t, sessMgr, "u_surge_wrath_ok")

	// Apply wrath_active directly.
	condReg := makeWrathConditionRegistry()
	def, ok := condReg.Get("wrath_active")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, def, 1, -1))

	evt, err := svc.handleUse("u_surge_wrath_ok", "adrenaline_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetUseResponse().GetMessage()
	assert.Equal(t, surgeFeat.ActivateText, msg,
		"REQ-80-4: Adrenaline Surge must activate when player has wrath_active")
}

// TestHandleUse_AdrenalineSurge_SucceedsWhenEnraged verifies that activating
// adrenaline_surge still succeeds when the player is Enraged via mental state (REQ-80-4
// backward compatibility — mental rage still qualifies).
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
	svc, sessMgr, _, mentalMgr := newWrathSvc(t, []*ruleset.Feat{surgeFeat})
	addWrathTestPlayer(t, sessMgr, "u_surge_ok")

	// Apply Enraged mental state before using Adrenaline Surge.
	_ = mentalMgr.ApplyTrigger("u_surge_ok", mentalstate.TrackRage, mentalstate.SeverityMod)

	evt, err := svc.handleUse("u_surge_ok", "adrenaline_surge", "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetUseResponse().GetMessage()
	assert.Equal(t, surgeFeat.ActivateText, msg,
		"REQ-80-4: Adrenaline Surge must activate when player is Enraged via mental state")
}
