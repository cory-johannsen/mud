package gameserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// mockSaverForRegen is a minimal CharacterSaver that records SaveState calls.
type mockSaverForRegen struct {
	mockCharSaverFull
	mu        sync.Mutex
	savedHP   int
	saveCalls int
}

func (m *mockSaverForRegen) SaveState(_ context.Context, _ int64, _ string, hp int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedHP = hp
	m.saveCalls++
	return nil
}

// addIdlePlayer adds a player with Status=IDLE to sessMgr and returns the session.
func addIdlePlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string, currentHP, maxHP, grit int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   currentHP,
		MaxHP:       maxHP,
		Abilities:   character.AbilityScores{Grit: grit},
		Role:        "player",
	})
	require.NoError(t, err)
	return sess
}

// makeRegenCombatHandler creates a CombatHandler for regen tests.
func makeRegenCombatHandler(t *testing.T) *CombatHandler {
	t.Helper()
	return makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
}

// TestRegenManager_HealsIdlePlayer verifies that an idle player below max HP is healed.
func TestRegenManager_HealsIdlePlayer(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	const grit = 14 // AbilityMod = +2
	const initHP = 5
	const maxHP = 20
	sess := addIdlePlayer(t, sessMgr, "p1", "room1", initHP, maxHP, grit)

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	expectedRegen := combat.AbilityMod(grit) // 2
	assert.Equal(t, initHP+expectedRegen, sess.CurrentHP)
	assert.Equal(t, 1, saver.saveCalls)
	assert.Equal(t, initHP+expectedRegen, saver.savedHP)
}

// TestRegenManager_SkipsPlayerAtMaxHP verifies that a player at max HP is not healed.
func TestRegenManager_SkipsPlayerAtMaxHP(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	sess := addIdlePlayer(t, sessMgr, "p1", "room1", 20, 20, 10)
	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 20, sess.CurrentHP)
	assert.Equal(t, 0, saver.saveCalls)
}

// TestRegenManager_SkipsPlayerInCombat verifies that a player in combat is not healed.
func TestRegenManager_SkipsPlayerInCombat(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	sess := addIdlePlayer(t, sessMgr, "p1", "room1", 5, 20, 10)
	sess.Status = 2 // CombatStatus_IN_COMBAT

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 5, sess.CurrentHP, "combat player HP must not change")
	assert.Equal(t, 0, saver.saveCalls)
}

// TestRegenManager_CapsAtMaxHP verifies that regen does not exceed MaxHP.
func TestRegenManager_CapsAtMaxHP(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	const grit = 18 // AbilityMod = +4
	sess := addIdlePlayer(t, sessMgr, "p1", "room1", 19, 20, grit)

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 20, sess.CurrentHP, "HP must not exceed MaxHP")
	assert.Equal(t, 20, saver.savedHP)
}

// TestRegenManager_MinRegenIsOne verifies that negative GritMod still gives 1 HP regen.
func TestRegenManager_MinRegenIsOne(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	const grit = 6 // AbilityMod = -2
	sess := addIdlePlayer(t, sessMgr, "p1", "room1", 5, 20, grit)

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 6, sess.CurrentHP, "minimum regen must be 1")
}

// TestRegenManager_HealsNPC verifies that idle NPCs below max HP are healed.
func TestRegenManager_HealsNPC(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	tmpl := &npc.Template{
		ID:    "goblin",
		Name:  "Goblin",
		MaxHP: 10,
		Level: 1,
		AC:    12,
	}
	inst, err := npcMgr.Spawn(tmpl, "room1")
	require.NoError(t, err)
	inst.CurrentHP = 5

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 5+regenNPCRate, inst.CurrentHP)
}

// TestRegenManager_SkipsNPCInCombatRoom verifies that NPCs in combat rooms are not healed.
func TestRegenManager_SkipsNPCInCombatRoom(t *testing.T) {
	sessMgr := session.NewManager()
	npcMgr := npc.NewManager()
	combatH := makeRegenCombatHandler(t)

	tmpl := &npc.Template{
		ID:    "goblin",
		Name:  "Goblin",
		MaxHP: 10,
		Level: 1,
		AC:    12,
	}
	inst, err := npcMgr.Spawn(tmpl, "room1")
	require.NoError(t, err)
	inst.CurrentHP = 5

	// Simulate active combat in room1 by inserting a timer entry.
	combatH.timersMu.Lock()
	combatH.timers["room1"] = combat.NewRoundTimer(time.Hour, func() {})
	combatH.timersMu.Unlock()

	saver := &mockSaverForRegen{}
	mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
	mgr.tick(context.Background())

	assert.Equal(t, 5, inst.CurrentHP, "NPC in combat room must not regen")
}

// TestRegenManager_Property_RegenNeverExceedsMaxHP is a property test verifying
// that regen always caps at MaxHP regardless of grit score.
func TestRegenManager_Property_RegenNeverExceedsMaxHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		grit := rapid.IntRange(1, 20).Draw(rt, "grit")
		maxHP := rapid.IntRange(1, 100).Draw(rt, "maxHP")
		currentHP := rapid.IntRange(0, maxHP).Draw(rt, "currentHP")

		sessMgr := session.NewManager()
		npcMgr := npc.NewManager()
		combatH := makeRegenCombatHandler(t)

		sess := addIdlePlayer(t, sessMgr, "p1", "room1", currentHP, maxHP, grit)
		saver := &mockSaverForRegen{}
		mgr := NewRegenManager(sessMgr, npcMgr, combatH, saver, time.Second, zaptest.NewLogger(t))
		mgr.tick(context.Background())

		assert.LessOrEqual(t, sess.CurrentHP, sess.MaxHP, "HP must never exceed MaxHP")
		assert.GreaterOrEqual(t, sess.CurrentHP, currentHP, "HP must never decrease from regen")
	})
}
