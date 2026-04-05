package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// TestBrutalSurge_AppliesConditionToCombatSet verifies that using a class feature
// with a condition_id during combat applies the condition to the combat condition set
// so that AC/damage modifiers take effect (regression for BUG-74).
func TestBrutalSurge_AppliesConditionToCombatSet(t *testing.T) {
	const uid = "bs-player"
	const roomID = "bs-room"
	const condID = "brutal_surge_active"
	const featureID = "brutal_surge"

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()

	// Condition registry with brutal_surge_active.
	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           condID,
		Name:         "Brutal Surge Active",
		DurationType: "encounter",
		ACPenalty:    2,
		DamageBonus:  2,
	})

	// Class feature registry with brutal_surge.
	cf := &ruleset.ClassFeature{
		ID:           featureID,
		Name:         "Brutal Surge",
		Active:       true,
		ActivateText: "The red haze drops and you move on pure instinct.",
		ConditionID:  condID,
	}
	cfReg := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{cf})
	cfRepo := &stubClassFeaturesRepo{data: map[int64][]string{0: {featureID}}}

	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, worldMgr, nil, nil, nil, nil, nil, nil,
	)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, cfReg, cfRepo, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	// Spawn an NPC in the room so combat can start.
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "grunt-bs", Name: "Grunt", Level: 1, MaxHP: 20, AC: 10, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	// Add player to room_a (testWorldAndSession uses room_a as the seed room;
	// we need the player's RoomID to match where the NPC is).
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Beserker", CharName: "Beserker",
		RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Start combat so ActiveCombatForPlayer returns non-nil.
	_, err = combatHandler.Attack(uid, "Grunt")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok, "expected active combat in room")

	// Ensure combat condition set exists for player.
	if cbt.Conditions[uid] == nil {
		cbt.Conditions[uid] = condition.NewActiveSet()
	}

	// Verify precondition: condition not yet applied.
	assert.False(t, cbt.Conditions[uid].Has(condID), "condition should not be present before use")

	// Use brutal_surge.
	resp, err := svc.handleUse(uid, featureID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, resp)

	useResp := resp.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Equal(t, cf.ActivateText, useResp.Message)

	// Postcondition: condition must be in the COMBAT set, not session-level.
	assert.True(t, cbt.Conditions[uid].Has(condID),
		"brutal_surge_active should be in combat condition set after use")
	assert.False(t, sess.Conditions.Has(condID),
		"brutal_surge_active should NOT be in session-level condition set during combat")
}

// TestBrutalSurge_OutOfCombat_FallsBackToSessionConditions verifies that
// using a class feature outside combat falls back to the session condition set.
func TestBrutalSurge_OutOfCombat_FallsBackToSessionConditions(t *testing.T) {
	const uid = "bs-player-oc"
	const condID = "brutal_surge_active"
	const featureID = "brutal_surge"

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:           condID,
		Name:         "Brutal Surge Active",
		DurationType: "encounter",
		ACPenalty:    2,
		DamageBonus:  2,
	})

	cf := &ruleset.ClassFeature{
		ID: featureID, Name: "Brutal Surge", Active: true,
		ActivateText: "The red haze drops and you move on pure instinct.",
		ConditionID:  condID,
	}
	cfReg := ruleset.NewClassFeatureRegistry([]*ruleset.ClassFeature{cf})
	cfRepo := &stubClassFeaturesRepo{data: map[int64][]string{0: {featureID}}}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, cfReg, cfRepo, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Brawler", CharName: "Brawler",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	// Not in combat — Status remains default.

	resp, err := svc.handleUse(uid, featureID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, resp)

	useResp := resp.GetUseResponse()
	require.NotNil(t, useResp)
	assert.Equal(t, cf.ActivateText, useResp.Message)

	assert.True(t, sess.Conditions.Has(condID),
		"brutal_surge_active should be in session conditions when used outside combat")
}
