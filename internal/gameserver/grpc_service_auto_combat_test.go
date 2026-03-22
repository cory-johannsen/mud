package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/cory-johannsen/mud/internal/game/command"
)

// newAutoCombatSvc mirrors newJoinSvc — uses the same constructor args.
func newAutoCombatSvc(t *testing.T) (*GameServiceServer, *session.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr, combatHandler
}

// REQ-T9: When a player starts combat, group members in the same room auto-join as combatants.
func TestAutoCombat_SameRoom_GroupMemberJoinsAsCombatant(t *testing.T) {
	_, sessMgr, combatHandler := newAutoCombatSvc(t)

	// Create two players in the same room.
	leader, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "ac-leader",
		Username:  "Leader",
		CharName:  "Leader",
		RoomID:    "room_a",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, leader)

	member, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "ac-member",
		Username:  "Member",
		CharName:  "Member",
		RoomID:    "room_a",
		CurrentHP: 15,
		MaxHP:     15,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, member)

	// Form a group.
	g := sessMgr.CreateGroup("ac-leader")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "ac-member"))

	// Start combat via startCombatLocked with the leader as initiator.
	npcInst := &npc.Instance{
		TemplateID: "t1",
		RoomID:     "room_a",
		MaxHP:      10,
		CurrentHP:  10,
		AC:         12,
		Level:      1,
	}

	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(leader, npcInst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	// Assert: both players appear in cbt.Combatants.
	var leaderFound, memberFound bool
	for _, c := range cbt.Combatants {
		if c.ID == "ac-leader" {
			leaderFound = true
		}
		if c.ID == "ac-member" {
			memberFound = true
		}
	}
	assert.True(t, leaderFound, "leader must appear in Combatants")
	assert.True(t, memberFound, "group member in same room must appear in Combatants")

	// Assert: member status is statusInCombat.
	assert.Equal(t, statusInCombat, member.Status, "group member status must be statusInCombat after auto-join")
}

// REQ-T10: When a player starts combat, group members in a different room are notified but not added as combatants.
func TestAutoCombat_DifferentRoom_MemberNotifiedOnly(t *testing.T) {
	_, sessMgr, combatHandler := newAutoCombatSvc(t)

	// Create two players in different rooms.
	leader, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "ac2-leader",
		Username:  "Leader2",
		CharName:  "Leader2",
		RoomID:    "room_a",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, leader)

	member, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "ac2-member",
		Username:  "Member2",
		CharName:  "Member2",
		RoomID:    "room_b",
		CurrentHP: 15,
		MaxHP:     15,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, member)

	// Form a group.
	g := sessMgr.CreateGroup("ac2-leader")
	require.NoError(t, sessMgr.AddGroupMember(g.ID, "ac2-member"))

	// Start combat via startCombatLocked with the leader as initiator.
	npcInst := &npc.Instance{
		TemplateID: "t2",
		RoomID:     "room_a",
		MaxHP:      10,
		CurrentHP:  10,
		AC:         12,
		Level:      1,
	}

	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(leader, npcInst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	// Assert: member NOT in cbt.Combatants.
	for _, c := range cbt.Combatants {
		assert.NotEqual(t, "ac2-member", c.ID, "group member in different room must NOT appear in Combatants")
	}

	// Assert: member status is NOT statusInCombat.
	assert.NotEqual(t, statusInCombat, member.Status, "group member in different room must NOT have statusInCombat")
}
