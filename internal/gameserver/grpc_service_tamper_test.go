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

// TestTamper_AppliesConditionToFoeInCombat verifies that the tamper feat (condition_target=foe)
// applies its condition to the enemy combatant's condition set, not the player's.
func TestTamper_AppliesConditionToFoeInCombat(t *testing.T) {
	const uid = "tamper-player"
	const roomID = "tamper-room"
	const condID = "tamper_debuff"
	const featID = "tamper"

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{
		ID:            condID,
		Name:          "Tampered",
		DurationType:  "encounter",
		AttackPenalty: 2,
	})

	feat := &ruleset.Feat{
		ID:              featID,
		Name:            "Tamper",
		Active:          true,
		ActivateText:    "You make a small, invisible adjustment to their gear.",
		ConditionID:     condID,
		ConditionTarget: "foe",
	}
	featReg := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{0: {featID}}}

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
		nil, featReg, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	// Spawn NPC in room so combat can start.
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "target-nerd", Name: "Target", Level: 1, MaxHP: 20, AC: 10, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Saboteur", CharName: "Saboteur",
		RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	// Start combat and record the last target (normally set by the gRPC handler).
	_, err = combatHandler.Attack(uid, "Target")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)
	sess.LastCombatTarget = "Target"

	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok, "expected active combat in room")

	// Find the NPC combatant ID.
	var npcCombatantID string
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			npcCombatantID = c.ID
			break
		}
	}
	require.NotEmpty(t, npcCombatantID, "NPC combatant must be present")

	// Ensure condition sets exist.
	if cbt.Conditions[uid] == nil {
		cbt.Conditions[uid] = condition.NewActiveSet()
	}
	if cbt.Conditions[npcCombatantID] == nil {
		cbt.Conditions[npcCombatantID] = condition.NewActiveSet()
	}

	// Precondition: debuff not yet on foe.
	assert.False(t, cbt.Conditions[npcCombatantID].Has(condID), "tamper_debuff should not be on foe before use")

	// Use tamper (target resolves from LastCombatTarget set during Attack).
	resp, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, resp)

	useResp := resp.GetUseResponse()
	require.NotNil(t, useResp, "expected UseResponse")
	assert.Contains(t, useResp.Message, feat.ActivateText, "message must contain ActivateText")
	assert.Contains(t, useResp.Message, "Tampered", "message must include condition name (REQ-BUG149-1)")

	// Postcondition: debuff must be on the FOE, not the player.
	assert.True(t, cbt.Conditions[npcCombatantID].Has(condID),
		"tamper_debuff must be applied to the foe's combat condition set")
	assert.False(t, cbt.Conditions[uid].Has(condID),
		"tamper_debuff must NOT be applied to the player")
	assert.False(t, sess.Conditions.Has(condID),
		"tamper_debuff must NOT be in the player's session conditions")
}

// TestTamper_OutOfCombat_ReturnsError verifies that tamper requires combat.
func TestTamper_OutOfCombat_ReturnsError(t *testing.T) {
	const uid = "tamper-ooc"
	const featID = "tamper"

	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	condReg := condition.NewRegistry()
	condReg.Register(&condition.ConditionDef{ID: "tamper_debuff", Name: "Tampered", DurationType: "encounter", AttackPenalty: 2})

	feat := &ruleset.Feat{
		ID: featID, Name: "Tamper", Active: true,
		ActivateText:    "You make a small, invisible adjustment to their gear.",
		ConditionID:     "tamper_debuff",
		ConditionTarget: "foe",
	}
	featReg := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{0: {featID}}}

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
		nil, featReg, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Saboteur2", CharName: "Saboteur2",
		RoomID: "room_a", CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	// No combat, no last target.

	resp, err := svc.handleUse(uid, featID, "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should return a message requiring a target or combat, not a UseResponse.
	msg := resp.GetMessage()
	require.NotNil(t, msg, "expected Message event (error) when no target")
	assert.NotEmpty(t, msg.Content)
}
