package gameserver

import (
	"sync"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOnPlayerDeath_FiredWhenPlayerDowned verifies that SetOnPlayerDeath callback
// is invoked with the player's uid when they are downed (HP=0) and no other players
// are alive in the room.
//
// Precondition: one player (HP=10), one NPC; player HP set to 0 before resolve.
// Postcondition: onPlayerDeath callback receives the player's uid.
func TestOnPlayerDeath_FiredWhenPlayerDowned(t *testing.T) {
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-death-cb"

	var (
		mu        sync.Mutex
		calledUID string
		deathDone = make(chan struct{})
	)
	h.SetOnPlayerDeath(func(uid string) {
		mu.Lock()
		calledUID = uid
		mu.Unlock()
		close(deathDone)
	})

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	_ = inst
	sess := addTestPlayer(t, h.sessions, "player-death", roomID)
	sess.MaxHP = 10
	sess.CurrentHP = 10

	// Start combat.
	_, err := h.Attack("player-death", inst.Name())
	require.NoError(t, err)
	h.cancelTimer(roomID)

	// Down the player before resolve.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 0
		}
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()

	<-deathDone

	mu.Lock()
	got := calledUID
	mu.Unlock()

	assert.Equal(t, "player-death", got, "onPlayerDeath must receive the downed player's uid")
}

// TestOnPlayerDeath_NotFiredOnVictory verifies that SetOnPlayerDeath callback is NOT
// invoked when the player wins combat (all NPCs dead).
//
// Precondition: one player (HP=10), one NPC; NPC HP set to 0 before resolve.
// Postcondition: onPlayerDeath callback is never called.
func TestOnPlayerDeath_NotFiredOnVictory(t *testing.T) {
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-victory-no-death"

	called := false
	h.SetOnPlayerDeath(func(_ string) {
		called = true
	})

	combatEndDone := make(chan struct{})
	h.SetOnCombatEnd(func(_ string) { close(combatEndDone) })

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-win", roomID)

	_, err := h.Attack("player-win", inst.Name())
	require.NoError(t, err)
	h.cancelTimer(roomID)

	// Kill the NPC.
	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
		}
	}
	h.resolveAndAdvanceLocked(roomID, cbt)
	h.combatMu.Unlock()

	<-combatEndDone

	assert.False(t, called, "onPlayerDeath must not fire when all NPCs are dead (victory)")
}
