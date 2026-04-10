package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// buildFeatUseServiceWithCondReg constructs a GameServiceServer configured with the
// given feat, a stub feats repo, and the provided condition registry.
//
// Precondition: feat and condReg must not be nil.
func buildFeatUseServiceWithCondReg(
	t *testing.T,
	sessMgr *session.Manager,
	feat *ruleset.Feat,
	condReg *condition.Registry,
) *GameServiceServer {
	t.Helper()
	worldMgr, _ := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {feat.ID},
		},
	}
	return newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{feat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
}

// TestHandleUse_FeatWithCondition_MessageIsActivateText_REQ_BUG149_1 verifies
// that when a feat with a condition_id is activated and the condition is in the registry,
// the UseResponse.Message is exactly the feat's ActivateText (no condition name parenthetical).
//
// Precondition: feat has ConditionID="overpower_active"; registry contains that condition.
// Postcondition: UseResponse.Message equals ActivateText only.
func TestHandleUse_FeatWithCondition_MessageIsActivateText_REQ_BUG149_1(t *testing.T) {
	const condID = "overpower_active"
	const featID = "overpower"
	const uid = "bug149-user-1"

	sessMgr := session.NewManager()

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           condID,
		Name:         "Overpower Active",
		DurationType: "encounter",
		DamageBonus:  2,
		ACPenalty:    2,
	})

	feat := &ruleset.Feat{
		ID:           featID,
		Name:         "Overpower",
		Active:       true,
		ActivateText: "You put everything into it.",
		ConditionID:  condID,
	}

	svc := buildFeatUseServiceWithCondReg(t, sessMgr, feat, condReg)
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Conditions = condition.NewActiveSet()

	event, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Equal(t, feat.ActivateText, useResp.Message, "message must equal ActivateText; no condition name parenthetical (REQ-BUG149-1)")
}

// TestHandleUse_FeatWithUnknownCondition_FallsBackToActivateText_REQ_BUG149_2 verifies
// that when a feat has a condition_id that is NOT in the registry, the response message
// falls back to just ActivateText with no crash and no error visible to the player.
//
// Precondition: feat has ConditionID="ghost_condition"; registry does not contain it.
// Postcondition: UseResponse.Message equals ActivateText only; no error returned.
func TestHandleUse_FeatWithUnknownCondition_FallsBackToActivateText_REQ_BUG149_2(t *testing.T) {
	const condID = "ghost_condition"
	const featID = "phantom_strike"
	const uid = "bug149-user-2"

	sessMgr := session.NewManager()

	// Registry exists but does NOT contain "ghost_condition".
	condReg := condition.NewRegistry()

	feat := &ruleset.Feat{
		ID:           featID,
		Name:         "Phantom Strike",
		Active:       true,
		ActivateText: "You strike from the shadows.",
		ConditionID:  condID,
	}

	svc := buildFeatUseServiceWithCondReg(t, sessMgr, feat, condReg)
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Conditions = condition.NewActiveSet()

	event, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err, "must not return an error when condition is missing from registry")
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Equal(t, feat.ActivateText, useResp.Message, "message must fall back to ActivateText when condition not in registry (REQ-BUG149-2)")
}

// TestHandleUse_FeatWithPreparedUsesAndCondition_MessageContainsUseCount_REQ_BUG149_3 verifies
// that when a feat has both PreparedUses > 0 and a ConditionID, the message contains the
// remaining use count but NOT a condition name parenthetical.
//
// Precondition: feat has PreparedUses=2, ConditionID set; sess.ActiveFeatUses == 2.
// Postcondition: UseResponse.Message contains "(1 uses remaining.)" but not a condition name.
func TestHandleUse_FeatWithPreparedUsesAndCondition_MessageContainsUseCount_REQ_BUG149_3(t *testing.T) {
	const condID = "overpower_active"
	const featID = "overpower"
	const uid = "bug149-user-3"

	sessMgr := session.NewManager()

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           condID,
		Name:         "Overpower Active",
		DurationType: "encounter",
		DamageBonus:  2,
		ACPenalty:    2,
	})

	feat := &ruleset.Feat{
		ID:           featID,
		Name:         "Overpower",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You put everything into it.",
		ConditionID:  condID,
	}

	svc := buildFeatUseServiceWithCondReg(t, sessMgr, feat, condReg)
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Conditions = condition.NewActiveSet()
	sess.ActiveFeatUses = map[string]int{featID: 2}

	event, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Contains(t, useResp.Message, "(1 uses remaining.)", "message must include remaining uses")
	assert.NotContains(t, useResp.Message, "Overpower Active", "message must NOT include condition name parenthetical (REQ-BUG29)")
}

// TestProperty_FeatWithConditionInRegistry_MessageIsActivateText_REQ_BUG149_4
// is a property test verifying that for any feat with a non-empty ConditionID that IS in
// the registry, the UseResponse.Message is just the ActivateText (no condition name appended).
//
// Precondition: Any feat with ConditionID in registry; player has session.
// Postcondition: UseResponse.Message equals ActivateText and does NOT contain the condition name.
func TestProperty_FeatWithConditionInRegistry_MessageIsActivateText_REQ_BUG149_4(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		condID := rapid.StringMatching(`[a-z][a-z0-9_]{2,15}`).Draw(rt, "condID")
		condName := rapid.StringMatching(`[A-Z][a-zA-Z ]{2,30}`).Draw(rt, "condName")
		featID := rapid.StringMatching(`[a-z][a-z0-9_]{2,15}`).Draw(rt, "featID")
		activateText := rapid.StringMatching(`[A-Z][a-zA-Z .!]{5,40}`).Draw(rt, "activateText")
		uid := "prop-bug149-" + featID

		sessMgr := session.NewManager()

		condReg := condition.NewRegistry()
		condReg.Register(&condition.ConditionDef{
			ID:           condID,
			Name:         condName,
			DurationType: "encounter",
		})

		feat := &ruleset.Feat{
			ID:           featID,
			Name:         featID,
			Active:       true,
			ActivateText: activateText,
			ConditionID:  condID,
		}

		svc := buildFeatUseServiceWithCondReg(t, sessMgr, feat, condReg)
		sess := addPlayerForFeatTest(t, sessMgr, uid)
		sess.Conditions = condition.NewActiveSet()

		event, err := svc.handleUse(uid, featID, "", 0, 0)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if event == nil {
			rt.Fatalf("expected non-nil event")
		}
		useResp := event.GetUseResponse()
		if useResp == nil {
			rt.Fatalf("expected UseResponse, got nil")
		}
		if useResp.Message != activateText {
			rt.Fatalf("message %q does not equal activateText %q", useResp.Message, activateText)
		}
		if strings.Contains(useResp.Message, condName) {
			rt.Fatalf("message %q must NOT contain condition name %q (REQ-BUG29)", useResp.Message, condName)
		}
	})
}

// TestHandleUse_RequiresCombatFeat_OutsideCombat_ReturnsError_REQ_BUG29 verifies that
// a feat with RequiresCombat=true returns an error message when the player is not in combat.
//
// Precondition: feat has RequiresCombat=true; player is not in any active combat.
// Postcondition: UseResponse.Message contains "must be in combat"; no error returned to caller.
func TestHandleUse_RequiresCombatFeat_OutsideCombat_ReturnsError_REQ_BUG29(t *testing.T) {
	const condID = "brutal_surge_active"
	const featID = "overpower"
	const uid = "bug29-combat-check"

	sessMgr := session.NewManager()

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           condID,
		Name:         "Brutal Surge Active",
		DurationType: "encounter",
		ACPenalty:    2,
	})

	feat := &ruleset.Feat{
		ID:             featID,
		Name:           "Overpower",
		Active:         true,
		ActivateText:   "You put everything into it.",
		ConditionID:    condID,
		RequiresCombat: true,
	}

	svc := buildFeatUseServiceWithCondReg(t, sessMgr, feat, condReg)
	sess := addPlayerForFeatTest(t, sessMgr, uid)
	sess.Conditions = condition.NewActiveSet()

	// No combatH wired → not in combat.
	event, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err, "must not return a protocol error")
	require.NotNil(t, event)

	// RequiresCombat rejection returns a Message event (not UseResponse).
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected Message event (not UseResponse) when feat is blocked by combat check")
	assert.Contains(t, msgEvt.Content, "must be in combat", "message must tell player feat requires combat (REQ-BUG29)")
	assert.NotContains(t, msgEvt.Content, feat.ActivateText, "activate text must not appear when blocked (REQ-BUG29)")
}
