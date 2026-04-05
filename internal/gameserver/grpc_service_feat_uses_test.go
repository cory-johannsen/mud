package gameserver

import (
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
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildFeatUseService constructs a minimal GameServiceServer configured with the
// given feat and a stub feats repo mapping characterID 0 → {feat.ID}.
func buildFeatUseService(t *testing.T, sessMgr *session.Manager, feat *ruleset.Feat) *GameServiceServer {
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
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{feat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
}

// addPlayerForFeatTest adds a player with CharacterID=0 and an empty condition set.
func addPlayerForFeatTest(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      uid,
		Username: uid,
		CharName: uid,
		RoomID:   "room_a",
		Role:     "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	return sess
}

// TestHandleUse_LimitedFeat_UsesDecrement_REQ_BUG12a verifies that activating a feat
// with PreparedUses > 0 decrements the session use count and returns a success message.
//
// Precondition: sess.ActiveFeatUses["berserk"] == 2; feat has PreparedUses=2.
// Postcondition: sess.ActiveFeatUses["berserk"] == 1; UseResponse.Message contains ActivateText.
func TestHandleUse_LimitedFeat_UsesDecrement_REQ_BUG12a(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You go berserk!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_feat_decrement")

	// Simulate populated ActiveFeatUses as the login flow would produce.
	sess.ActiveFeatUses = map[string]int{"berserk": 2}

	event, err := svc.handleUse("u_feat_decrement", "berserk", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Contains(t, useResp.Message, "You go berserk!", "message must contain ActivateText")
	assert.Equal(t, 1, sess.ActiveFeatUses["berserk"], "use count must decrement to 1")
}

// TestHandleUse_LimitedFeat_ZeroUses_Rejected_REQ_BUG12b verifies that attempting to
// activate a limited feat with 0 uses remaining returns a failure message and does not
// decrement the counter below 0.
//
// Precondition: sess.ActiveFeatUses["berserk"] == 0.
// Postcondition: UseResponse.Message indicates no uses remaining; counter stays at 0.
func TestHandleUse_LimitedFeat_ZeroUses_Rejected_REQ_BUG12b(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You go berserk!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_feat_zero")

	sess.ActiveFeatUses = map[string]int{"berserk": 0}

	event, err := svc.handleUse("u_feat_zero", "berserk", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	msg := event.GetMessage()
	require.NotNil(t, msg, "expected a plain message event, not UseResponse")
	assert.Contains(t, msg.Content, "no uses of Berserk remaining", "message must indicate exhaustion")
	assert.Equal(t, 0, sess.ActiveFeatUses["berserk"], "counter must not go below 0")
}

// TestHandleRest_RestoresActiveFeatUses_REQ_BUG12c verifies that handleRest restores
// ActiveFeatUses for limited active feats to their PreparedUses maximum.
//
// Precondition: sess.ActiveFeatUses["berserk"] == 0 (exhausted).
// Postcondition: sess.ActiveFeatUses["berserk"] == 2 (restored to PreparedUses).
func TestHandleRest_RestoresActiveFeatUses_REQ_BUG12c(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You go berserk!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_feat_rest")

	// Exhaust the feat.
	sess.ActiveFeatUses = map[string]int{"berserk": 0}

	stream := &fakeSessionStream{}
	err := svc.handleRest("u_feat_rest", "req-rest", stream)
	require.NoError(t, err)

	assert.Equal(t, 2, sess.ActiveFeatUses["berserk"], "uses must be restored to PreparedUses after rest")
}

// TestProperty_LimitedFeat_UsesNeverNegative_REQ_BUG12d is a property test verifying
// that no matter how many times handleUse is called on an exhausted feat, the session
// use counter never goes below 0.
//
// Precondition: feat has PreparedUses=1; sess.ActiveFeatUses set to 0.
// Postcondition: sess.ActiveFeatUses["prop-feat"] >= 0 after any number of activation attempts.
func TestProperty_LimitedFeat_UsesNeverNegative_REQ_BUG12d(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		attempts := rapid.IntRange(1, 10).Draw(rt, "attempts")

		sessMgr := session.NewManager()
		feat := &ruleset.Feat{
			ID:           "prop-feat",
			Name:         "Prop Feat",
			Active:       true,
			PreparedUses: 1,
			ActivateText: "Activated.",
		}
		svc := buildFeatUseService(t, sessMgr, feat)
		sess := addPlayerForFeatTest(t, sessMgr, "u_prop_feat_never_neg")
		sess.ActiveFeatUses = map[string]int{"prop-feat": 0}

		for i := 0; i < attempts; i++ {
			_, err := svc.handleUse("u_prop_feat_never_neg", "prop-feat", "", 0, 0)
			if err != nil {
				rt.Fatalf("unexpected error on attempt %d: %v", i, err)
			}
		}

		if sess.ActiveFeatUses["prop-feat"] < 0 {
			rt.Fatalf("use count went negative: %d", sess.ActiveFeatUses["prop-feat"])
		}
	})
}

// TestHandleUse_UnlimitedFeat_NoUsesTracked_REQ_BUG12e verifies that an active feat
// with PreparedUses == 0 (unlimited) activates without requiring or checking a use count.
//
// Precondition: feat has PreparedUses=0; sess.ActiveFeatUses is nil.
// Postcondition: UseResponse.Message == ActivateText; no error; ActiveFeatUses still nil.
func TestHandleUse_UnlimitedFeat_NoUsesTracked_REQ_BUG12e(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "lightning-dash",
		Name:         "Lightning Dash",
		Active:       true,
		PreparedUses: 0, // unlimited
		ActivateText: "You dash at lightning speed!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_feat_unlimited")

	// ActiveFeatUses is nil — unlimited feats are not tracked.
	assert.Nil(t, sess.ActiveFeatUses)

	event, err := svc.handleUse("u_feat_unlimited", "lightning-dash", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp)
	assert.Equal(t, "You dash at lightning speed!", useResp.Message)
	assert.Nil(t, sess.ActiveFeatUses, "ActiveFeatUses must remain nil for unlimited feats")
}

// TestHandleUse_ListMode_OmitsExhaustedFeat_REQ_BUG12f verifies that when abilityID is
// empty (list mode), limited feats with 0 uses remaining are excluded from the choices list.
//
// Precondition: sess has two active feats: "berserk" (PreparedUses=2, 0 remaining) and
// "quick-draw" (PreparedUses=0, unlimited).
// Postcondition: UseResponse.Choices contains "quick-draw" but not "berserk".
func TestHandleUse_ListMode_OmitsExhaustedFeat_REQ_BUG12f(t *testing.T) {
	sessMgr := session.NewManager()
	berserk := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You go berserk!",
	}
	quickDraw := &ruleset.Feat{
		ID:           "quick-draw",
		Name:         "Quick Draw",
		Active:       true,
		PreparedUses: 0,
		ActivateText: "You draw fast!",
	}

	worldMgr, _ := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{berserk, quickDraw})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"berserk", "quick-draw"},
		},
	}
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{berserk, quickDraw}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	sess := addPlayerForFeatTest(t, sessMgr, "u_list_omit_exhausted")
	// berserk exhausted; quick-draw has no limit.
	sess.ActiveFeatUses = map[string]int{"berserk": 0}

	event, err := svc.handleUse("u_list_omit_exhausted", "", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp)

	ids := make([]string, 0, len(useResp.Choices))
	for _, c := range useResp.Choices {
		ids = append(ids, c.FeatId)
	}
	assert.NotContains(t, ids, "berserk", "exhausted feat must be omitted from list")
	assert.Contains(t, ids, "quick-draw", "unlimited feat must appear in list")
}

// TestHandleUse_ListMode_ShowsRemainingUses_REQ_BUG12g verifies that in list mode, limited
// feats with uses remaining show the use count in their Description.
//
// Precondition: sess.ActiveFeatUses["berserk"] == 1; feat has PreparedUses=2.
// Postcondition: FeatEntry.Description for "berserk" contains "1 uses remaining".
func TestHandleUse_ListMode_ShowsRemainingUses_REQ_BUG12g(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		Description:  "Rage with power.",
		ActivateText: "You go berserk!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_list_show_uses")
	sess.ActiveFeatUses = map[string]int{"berserk": 1}

	event, err := svc.handleUse("u_list_show_uses", "", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp)
	require.Len(t, useResp.Choices, 1)
	assert.Contains(t, useResp.Choices[0].Description, "1 uses remaining")
}

// TestHandleUse_LimitedFeat_MessageContainsRemainingCount_REQ_BUG12h verifies that after a
// successful activation the response message includes the remaining use count.
//
// Precondition: sess.ActiveFeatUses["berserk"] == 2.
// Postcondition: UseResponse.Message ends with "(1 uses remaining.)".
func TestHandleUse_LimitedFeat_MessageContainsRemainingCount_REQ_BUG12h(t *testing.T) {
	sessMgr := session.NewManager()
	feat := &ruleset.Feat{
		ID:           "berserk",
		Name:         "Berserk",
		Active:       true,
		PreparedUses: 2,
		ActivateText: "You go berserk!",
	}
	svc := buildFeatUseService(t, sessMgr, feat)
	sess := addPlayerForFeatTest(t, sessMgr, "u_feat_count_msg")
	sess.ActiveFeatUses = map[string]int{"berserk": 2}

	event, err := svc.handleUse("u_feat_count_msg", "berserk", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	useResp := event.GetUseResponse()
	require.NotNil(t, useResp)
	assert.Contains(t, useResp.Message, "(1 uses remaining.)")
}

// Ensure gamev1 import is used (it is already used via fakeSessionStream from rest test file,
// but the package-level var keeps the linter silent in case of import reordering).
var _ = gamev1.CombatStatus_COMBAT_STATUS_IDLE
