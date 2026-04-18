package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// setupMartyrsCombat initialises a CombatHandler, spawns a high-HP NPC, adds a player
// with Martyr's Resolve available, starts combat, cancels the timer, and returns the
// handler and player session for further manipulation.
//
// The NPC has MaxHP=1000 so the player's attack won't end the fight.
// The dice source returns 19 for every call (attack always hits, 6 damage).
func setupMartyrsCombat(t *testing.T, uidSuffix string, martyrsSlot *session.InnateSlot) (*CombatHandler, *session.PlayerSession, string) {
	t.Helper()

	roomID := "room_martyrs_" + uidSuffix
	uid := "p_martyrs_" + uidSuffix

	src := newSeqSource(19) // 19 cycles: every d20 → 20, every d6 → min(19,5)+1 = 6
	h := makeCombatHandlerWithDice(t, src, func(_ string, _ []*gamev1.CombatEvent) {})

	// Spawn a tank NPC: MaxHP=1000 so the player can't kill it in one hit.
	_, err := h.npcMgr.Spawn(&npc.Template{
		ID: "tank_" + uidSuffix, Name: "Tank", Level: 1, MaxHP: 1000, AC: 5, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := h.sessions.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Zealot", CharName: "Zealot",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	if martyrsSlot != nil {
		sess.InnateTechs = map[string]*session.InnateSlot{
			"martyrs_resolve": martyrsSlot,
		}
	}

	// Start combat (player attacks Tank; this queues the player's attack and rolls initiative).
	_, err = h.Attack(uid, "Tank")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	return h, sess, roomID
}

// TestMartryrsResolve_StabilizesPlayerAt1HP is the regression test for GitHub issue #156:
// "Martyr's Resolve feat not implemented."
//
// Precondition: player (HP=1) has martyrs_resolve with MaxUses=1, UsesRemaining=1.
// NPC queues an attack that deals 6 damage (deterministic dice source returns 19 always).
// Postcondition: After round resolution the player has HP=1 (not dead), use slot consumed.
func TestMartryrsResolve_StabilizesPlayerAt1HP(t *testing.T) {
	t.Parallel()

	h, sess, roomID := setupMartyrsCombat(t, "active", &session.InnateSlot{MaxUses: 1, UsesRemaining: 1})
	uid := sess.UID

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)

	npcInsts := h.npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	npcID := npcInsts[0].ID

	// Move combatants adjacent (player at 0,10; NPC at 1,10) so melee attacks connect.
	// Default positions are 0,10 (player) and 19,10 (NPC) — 95 ft apart, out of melee range.
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 1
			c.GridX = 0
			c.GridY = 10
		} else {
			c.GridX = 1
			c.GridY = 10
		}
	}
	sess.CurrentHP = 1

	// Reset the NPC's action queue to clear the auto-queued stride+attack from setup,
	// then queue exactly one attack so the feat can fire exactly once per round.
	cbt.ActionQueues[npcID] = combat.NewActionQueue(npcID, 3)
	// Target must be the player's CharName (ResolveRound uses findCombatantByName).
	err := cbt.QueueAction(npcID, combat.QueuedAction{
		Type: combat.ActionAttack, Target: "Zealot", AbilityCost: 1,
	})
	require.NoError(t, err)

	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	h.cancelTimer(roomID)

	// Verify: player HP is 1, use consumed.
	sess, ok = h.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 1, sess.CurrentHP, "Martyr's Resolve must stabilize player at 1 HP")

	slot, ok := sess.InnateTechs["martyrs_resolve"]
	require.True(t, ok)
	assert.Equal(t, 0, slot.UsesRemaining, "Martyr's Resolve use must be consumed after activation")
}

// TestMartryrsResolve_ExpiredSlot_DoesNotActivate verifies that Martyr's Resolve
// does not trigger when UsesRemaining == 0.
//
// Precondition: player (HP=1) has martyrs_resolve with MaxUses=1, UsesRemaining=0.
// Postcondition: Player HP drops to 0 or below (feat does nothing).
func TestMartryrsResolve_ExpiredSlot_DoesNotActivate(t *testing.T) {
	t.Parallel()

	h, sess, roomID := setupMartyrsCombat(t, "expired", &session.InnateSlot{MaxUses: 1, UsesRemaining: 0})
	uid := sess.UID

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)

	npcInsts := h.npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	npcID := npcInsts[0].ID

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 1
			c.GridX = 0
			c.GridY = 10
		} else {
			c.GridX = 1
			c.GridY = 10
		}
	}
	sess.CurrentHP = 1

	// Reset the NPC's action queue to ensure exactly one attack fires this round.
	cbt.ActionQueues[npcID] = combat.NewActionQueue(npcID, 3)
	// Target must be the player's CharName (ResolveRound uses findCombatantByName).
	err := cbt.QueueAction(npcID, combat.QueuedAction{
		Type: combat.ActionAttack, Target: "Zealot", AbilityCost: 1,
	})
	require.NoError(t, err)

	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	h.cancelTimer(roomID)

	// With expired slot, player should be at 0 or below.
	sess, ok = h.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.LessOrEqual(t, sess.CurrentHP, 0,
		"expired Martyr's Resolve must not stabilize player; HP must be <= 0")
}

// TestMartryrsResolve_UnlimitedSlot_ActivatesWithoutDecrement verifies that
// MaxUses=0 (unlimited) activates without decrementing UsesRemaining.
//
// Precondition: player (HP=1) has martyrs_resolve with MaxUses=0 (unlimited).
// Postcondition: Player is stabilized at 1 HP; UsesRemaining remains 0.
func TestMartryrsResolve_UnlimitedSlot_ActivatesWithoutDecrement(t *testing.T) {
	t.Parallel()

	h, sess, roomID := setupMartyrsCombat(t, "unlimited", &session.InnateSlot{MaxUses: 0, UsesRemaining: 0})
	uid := sess.UID

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)

	npcInsts := h.npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	npcID := npcInsts[0].ID

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 1
			c.GridX = 0
			c.GridY = 10
		} else {
			c.GridX = 1
			c.GridY = 10
		}
	}
	sess.CurrentHP = 1

	// Reset the NPC's action queue to ensure exactly one attack fires this round.
	cbt.ActionQueues[npcID] = combat.NewActionQueue(npcID, 3)
	// Target must be the player's CharName (ResolveRound uses findCombatantByName).
	err := cbt.QueueAction(npcID, combat.QueuedAction{
		Type: combat.ActionAttack, Target: "Zealot", AbilityCost: 1,
	})
	require.NoError(t, err)

	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	h.cancelTimer(roomID)

	sess, ok = h.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.Equal(t, 1, sess.CurrentHP,
		"unlimited Martyr's Resolve must stabilize player at 1 HP")
	slot := sess.InnateTechs["martyrs_resolve"]
	assert.Equal(t, 0, slot.UsesRemaining,
		"unlimited slot UsesRemaining must not be decremented")
}

// TestMartryrsResolve_NoSlot_PlayerDies verifies that without Martyr's Resolve,
// the player is reduced to 0 HP normally.
//
// Precondition: player (HP=1) has no InnateTechs.
// Postcondition: Player HP is <= 0.
func TestMartryrsResolve_NoSlot_PlayerDies(t *testing.T) {
	t.Parallel()

	h, sess, roomID := setupMartyrsCombat(t, "noslot", nil)
	uid := sess.UID

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)

	npcInsts := h.npcMgr.InstancesInRoom(roomID)
	require.NotEmpty(t, npcInsts)
	npcID := npcInsts[0].ID

	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 1
			c.GridX = 0
			c.GridY = 10
		} else {
			c.GridX = 1
			c.GridY = 10
		}
	}
	sess.CurrentHP = 1

	// Reset the NPC's action queue to ensure exactly one attack fires this round.
	cbt.ActionQueues[npcID] = combat.NewActionQueue(npcID, 3)
	// Target must be the player's CharName (ResolveRound uses findCombatantByName).
	err := cbt.QueueAction(npcID, combat.QueuedAction{
		Type: combat.ActionAttack, Target: "Zealot", AbilityCost: 1,
	})
	require.NoError(t, err)

	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()
	h.cancelTimer(roomID)

	sess, ok = h.sessions.GetPlayer(uid)
	require.True(t, ok)
	assert.LessOrEqual(t, sess.CurrentHP, 0,
		"player without Martyr's Resolve must be reduced to 0 HP")
}
