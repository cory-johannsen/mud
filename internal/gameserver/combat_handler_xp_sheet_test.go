package gameserver

// REQ-BUG61-1: pushXPMessages MUST call pushCharacterSheetFn after awarding XP so that
// the web client Stats tab reflects the updated experience total.
// REQ-BUG61-2: pushCharacterSheetFn MUST be called for every session that receives XP,
// regardless of whether a level-up occurred.

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestPushXPMessages_CallsPushCharacterSheetFn verifies that pushXPMessages calls
// the registered pushCharacterSheetFn callback for the session that received XP
// (REQ-BUG61-1).
//
// Precondition: A CombatHandler with a registered pushCharacterSheetFn; a player session
// with a BridgeEntity; 1 XP awarded (no level-up).
// Postcondition: pushCharacterSheetFn is called exactly once with the player session.
func TestPushXPMessages_CallsPushCharacterSheetFn(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	var (
		mu          sync.Mutex
		calledSess  []*session.PlayerSession
	)
	h.SetPushCharacterSheetFn(func(sess *session.PlayerSession) {
		mu.Lock()
		defer mu.Unlock()
		calledSess = append(calledSess, sess)
	})

	sess := &session.PlayerSession{
		UID:        "test-player",
		Experience: 0,
		Level:      1,
		MaxHP:      20,
		CurrentHP:  20,
		Entity:     session.NewBridgeEntity("test-player", 64),
	}

	// No level-up messages (empty slice).
	h.pushXPMessages(sess, nil, 50, "Goblin", sess.Level)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calledSess, 1, "pushCharacterSheetFn must be called exactly once")
	assert.Equal(t, sess, calledSess[0], "pushCharacterSheetFn must receive the XP recipient session")
}

// TestPushXPMessages_CallsPushCharacterSheetFn_WithLevelUp verifies that
// pushXPMessages calls pushCharacterSheetFn even when level-up messages are present
// (REQ-BUG61-2).
//
// Precondition: A CombatHandler with pushCharacterSheetFn registered; level-up messages provided.
// Postcondition: pushCharacterSheetFn is called exactly once.
func TestPushXPMessages_CallsPushCharacterSheetFn_WithLevelUp(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	var callCount int32
	h.SetPushCharacterSheetFn(func(_ *session.PlayerSession) {
		atomic.AddInt32(&callCount, 1)
	})

	sess := &session.PlayerSession{
		UID:        "test-player-lvl",
		Experience: 0,
		Level:      1,
		MaxHP:      20,
		CurrentHP:  20,
		Entity:     session.NewBridgeEntity("test-player-lvl", 64),
	}

	levelUpMsgs := []string{"You leveled up to level 2!"}
	h.pushXPMessages(sess, levelUpMsgs, 200, "Boss", 1)

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount),
		"pushCharacterSheetFn must be called exactly once even with level-up messages")
}

// TestProperty_PushXPMessages_AlwaysCallsCharacterSheetFn verifies that
// pushXPMessages always calls pushCharacterSheetFn regardless of xpAmount or
// message count (REQ-BUG61-1, REQ-BUG61-2).
//
// Precondition: Any combination of xpAmount >= 0 and levelMsgs count 0–3.
// Postcondition: pushCharacterSheetFn is called exactly once per pushXPMessages call.
func TestProperty_PushXPMessages_AlwaysCallsCharacterSheetFn(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		xpAmount := rapid.IntRange(0, 1000).Draw(rt, "xpAmount")
		msgCount := rapid.IntRange(0, 3).Draw(rt, "msgCount")

		levelMsgs := make([]string, msgCount)
		for i := range levelMsgs {
			levelMsgs[i] = "level up"
		}

		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
		var callCount int32
		h.SetPushCharacterSheetFn(func(_ *session.PlayerSession) {
			atomic.AddInt32(&callCount, 1)
		})

		sess := &session.PlayerSession{
			UID:       "prop-player",
			Level:     1,
			MaxHP:     20,
			CurrentHP: 20,
			Entity:    session.NewBridgeEntity("prop-player", 64),
		}

		h.pushXPMessages(sess, levelMsgs, xpAmount, "NPC", sess.Level)

		assert.Equal(rt, int32(1), atomic.LoadInt32(&callCount),
			"pushCharacterSheetFn must be called exactly once per XP award")
	})
}

