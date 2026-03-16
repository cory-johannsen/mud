package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// makeImmobilizedConditionRegistry returns a registry with grabbed having restrict_actions: ["move"].
func makeImmobilizedConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	reg.Register(&condition.ConditionDef{ID: "grabbed", Name: "Grabbed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2, RestrictActions: []string{"move"}})
	reg.Register(&condition.ConditionDef{ID: "hidden", Name: "Hidden", DurationType: "permanent", MaxStacks: 0})
	return reg
}

// newImmobilizedMoveSvc builds a GameServiceServer with a condReg that has grabbed restricting move.
func newImmobilizedMoveSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeImmobilizedConditionRegistry()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newImmobilizedFleeSvc builds a GameServiceServer with a condReg that has grabbed restricting move,
// a full combat handler, and an NPC in combat with the player.
func newImmobilizedFleeSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeImmobilizedConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, worldMgr, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleMove_GrabbedBlocked verifies that a player with the grabbed condition
// cannot move to another room.
func TestHandleMove_GrabbedBlocked(t *testing.T) {
	svc, sessMgr := newImmobilizedMoveSvc(t)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_move",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Apply grabbed condition.
	sess.Conditions = condition.NewActiveSet()
	condReg := makeImmobilizedConditionRegistry()
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	evt, err := svc.handleMove("u_imm_move", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must return an error event containing "grabbed".
	errEvt := evt.GetError()
	require.NotNil(t, errEvt, "expected ErrorEvent, got: %T", evt.Payload)
	assert.Contains(t, errEvt.Message, "grabbed")

	// Player must still be in room_a.
	sess2, ok := sessMgr.GetPlayer("u_imm_move")
	require.True(t, ok)
	assert.Equal(t, "room_a", sess2.RoomID, "player must not have moved")
}

// TestHandleMove_NotGrabbed_MovesNormally verifies that a player without the grabbed condition
// can move normally.
func TestHandleMove_NotGrabbed_MovesNormally(t *testing.T) {
	svc, sessMgr := newImmobilizedMoveSvc(t)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_free",
		Username:  "Tester",
		CharName:  "Tester",
		RoomID:    "room_a",
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	evt, err := svc.handleMove("u_imm_free", &gamev1.MoveRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Must return a RoomView (successful move), not an error.
	assert.Nil(t, evt.GetError(), "expected no error event for un-grabbed player")

	sess2, ok := sessMgr.GetPlayer("u_imm_free")
	require.True(t, ok)
	assert.Equal(t, "room_b", sess2.RoomID, "player must have moved to room_b")
}

// TestHandleFlee_GrabbedBlocked verifies that a player with the grabbed condition
// cannot flee combat.
func TestHandleFlee_GrabbedBlocked(t *testing.T) {
	src := dice.NewDeterministicSource([]int{5, 3}) // initiative rolls only
	roller := dice.NewRoller(src)
	svc, sessMgr, npcMgr, _ := newImmobilizedFleeSvc(t, roller)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_imm_flee",
		Username:  "Runner",
		CharName:  "Runner",
		RoomID:    "room_a",
		CurrentHP: 20,
		MaxHP:     20,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Spawn NPC and start combat.
	tmpl := &npc.Template{
		ID: "ganger-imm", Name: "Ganger", Level: 1, MaxHP: 20, AC: 12, Perception: 10,
	}
	_, err = npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	_, err = svc.combatH.Attack("u_imm_flee", "Ganger")
	require.NoError(t, err)

	// Apply grabbed condition to the player session.
	sess.Conditions = condition.NewActiveSet()
	condReg := makeImmobilizedConditionRegistry()
	grabbedDef, ok := condReg.Get("grabbed")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

	evt, err := svc.handleFlee("u_imm_flee")
	require.NoError(t, err)
	require.NotNil(t, evt)

	combatEvt := evt.GetCombatEvent()
	require.NotNil(t, combatEvt, "expected CombatEvent")
	assert.Contains(t, combatEvt.Narrative, "grabbed")

	// Player must still be in room_a.
	sess2, ok := sessMgr.GetPlayer("u_imm_flee")
	require.True(t, ok)
	assert.Equal(t, "room_a", sess2.RoomID, "grabbed player must not have moved")
}

// TestProperty_GrabbedAlwaysBlocksMove verifies that whenever the grabbed condition
// (with restrict_actions: ["move"]) is active, handleMove always returns an error event
// and the player never changes rooms.
func TestProperty_GrabbedAlwaysBlocksMove(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newImmobilizedMoveSvc(t)
		uid := rapid.StringMatching(`u_prop_[a-z]{6}`).Draw(rt, "uid")

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "Prop",
			CharName:  "Prop",
			RoomID:    "room_a",
			CurrentHP: 10,
			MaxHP:     10,
			Abilities: character.AbilityScores{},
			Role:      "player",
		})
		require.NoError(rt, err)

		sess.Conditions = condition.NewActiveSet()
		condReg := makeImmobilizedConditionRegistry()
		grabbedDef, ok := condReg.Get("grabbed")
		require.True(rt, ok)
		require.NoError(rt, sess.Conditions.Apply(sess.UID, grabbedDef, 1, -1))

		evt, err := svc.handleMove(uid, &gamev1.MoveRequest{Direction: "north"})
		require.NoError(rt, err)
		require.NotNil(rt, evt)

		errEvt := evt.GetError()
		assert.NotNil(rt, errEvt, "grabbed player must always receive an error event")

		sess2, ok := sessMgr.GetPlayer(uid)
		require.True(rt, ok)
		assert.Equal(rt, "room_a", sess2.RoomID, "grabbed player must never move rooms")
	})
}
