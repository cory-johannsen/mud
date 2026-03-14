package gameserver

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// makeForcedActionHandler builds a CombatHandler with a MentalStateManager for forced-action tests.
func makeForcedActionHandler(t *testing.T, mentalMgr *mentalstate.Manager) (*CombatHandler, *npc.Manager, *session.Manager) {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := NewCombatHandler(
		engine, npcMgr, sessMgr, roller, broadcastFn,
		10*time.Second, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, mentalMgr,
	)
	return h, npcMgr, sessMgr
}

// setupForcedActionCombat creates a combat scenario with one player and two NPCs.
// npc2 (Orc) has lower HP (3) than npc1 (Goblin, HP=20) for lowest-HP tests.
func setupForcedActionCombat(t *testing.T, mentalMgr *mentalstate.Manager) (*CombatHandler, *session.Manager, *combat.Combat, string, string) {
	t.Helper()
	h, npcMgr, sessMgr := makeForcedActionHandler(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_forced", Username: "T", CharName: "T", RoomID: "room_a",
		CurrentHP: 10, MaxHP: 10, Role: "player",
		DefaultCombatAction: "attack",
	})
	require.NoError(t, err)

	tmpl1 := &npc.Template{ID: "goblin", Name: "Goblin", Level: 1, MaxHP: 20, AC: 12}
	inst1, err := npcMgr.Spawn(tmpl1, "room_a")
	require.NoError(t, err)
	_ = inst1

	tmpl2 := &npc.Template{ID: "orc", Name: "Orc", Level: 1, MaxHP: 20, AC: 12}
	inst2, err := npcMgr.Spawn(tmpl2, "room_a")
	require.NoError(t, err)
	inst2.CurrentHP = 3

	_, err = h.Attack("u_forced", "Goblin")
	require.NoError(t, err)
	h.cancelTimer("room_a")

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat("room_a")
	require.True(t, ok, "combat must exist after Attack")
	// Reset round so all action queues are cleared — tests start from a clean state.
	_ = cbt.StartRound(3)
	// Manually inject Orc into the active combat so lowest-HP targeting tests work.
	orcCbt := &combat.Combatant{
		ID:        inst2.ID,
		Kind:      combat.KindNPC,
		Name:      inst2.Name(),
		MaxHP:     inst2.MaxHP,
		CurrentHP: inst2.CurrentHP, // 3
		AC:        inst2.AC,
		Level:     inst2.Level,
	}
	cbt.Combatants = append(cbt.Combatants, orcCbt)
	cbt.ActionQueues[inst2.ID] = combat.NewActionQueue(inst2.ID, 3)
	h.combatMu.Unlock()

	return h, sessMgr, cbt, "Goblin", "Orc"
}

// applyForcedCondition applies a condition with the given forced_action value to the session.
func applyForcedCondition(t *testing.T, sess *session.PlayerSession, uid, condID, forcedAction string) {
	t.Helper()
	if sess.Conditions == nil {
		sess.Conditions = condition.NewActiveSet()
	}
	def := &condition.ConditionDef{
		ID:           condID,
		Name:         condID,
		ForcedAction: forcedAction,
		DurationType: "rounds",
	}
	require.NoError(t, sess.Conditions.Apply(uid, def, 1, 5))
}

func TestForcedAction_RandomAttack_Panicked(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	assert.True(t, actions[0].Target == npc1Name || actions[0].Target == npc2Name || actions[0].Target == "T",
		"target must be an alive combatant, got %q", actions[0].Target)
}

func TestForcedAction_LowHPAttack_Berserker(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, _, npc2Name := setupForcedActionCombat(t, mentalMgr)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "rage_berserker", "lowest_hp_attack")

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
	assert.Equal(t, npc2Name, actions[0].Target)
}

func TestForcedAction_OverridesPreSubmitted(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, sessMgr, cbt, npc1Name, _ := setupForcedActionCombat(t, mentalMgr)

	combatH.combatMu.Lock()
	err := cbt.QueueAction("u_forced", combat.QueuedAction{Type: combat.ActionAttack, Target: npc1Name})
	combatH.combatMu.Unlock()
	require.NoError(t, err)

	q := cbt.ActionQueues["u_forced"]
	require.Len(t, q.QueuedActions(), 1)

	sess, ok := sessMgr.GetPlayer("u_forced")
	require.True(t, ok)
	applyForcedCondition(t, sess, "u_forced", "fear_panicked", "random_attack")

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, combat.ActionAttack, actions[0].Type)
}

func TestForcedAction_NoCondition_NormalBehavior(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	combatH, _, cbt, npc1Name, _ := setupForcedActionCombat(t, mentalMgr)

	combatH.combatMu.Lock()
	combatH.autoQueuePlayersLocked(cbt)
	combatH.combatMu.Unlock()

	q := cbt.ActionQueues["u_forced"]
	require.NotNil(t, q)
	actions := q.QueuedActions()
	require.Len(t, actions, 1)
	assert.Equal(t, npc1Name, actions[0].Target)
}

func TestProperty_ForcedAction_AlwaysTargetsAliveCombatant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		forcedType := rapid.SampledFrom([]string{"random_attack", "lowest_hp_attack"}).Draw(rt, "forced_type")

		mentalMgr := mentalstate.NewManager()
		combatH, sessMgr, cbt, npc1Name, npc2Name := setupForcedActionCombat(t, mentalMgr)

		sess, ok := sessMgr.GetPlayer("u_forced")
		require.True(t, ok)
		applyForcedCondition(t, sess, "u_forced", "test_forced", forcedType)

		combatH.combatMu.Lock()
		combatH.autoQueuePlayersLocked(cbt)
		combatH.combatMu.Unlock()

		q := cbt.ActionQueues["u_forced"]
		require.NotNil(t, q)
		actions := q.QueuedActions()
		require.Len(t, actions, 1, "forced action must produce exactly one action")
		assert.Equal(t, combat.ActionAttack, actions[0].Type, "forced action must be an attack")

		aliveCombatants := []string{npc1Name, npc2Name, "T"}
		found := false
		for _, name := range aliveCombatants {
			if actions[0].Target == name {
				found = true
				break
			}
		}
		assert.True(t, found, "target %q must be an alive combatant", actions[0].Target)
	})
}
