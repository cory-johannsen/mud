package gameserver_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const respawnTestRoundDuration = 200 * time.Millisecond

// makeRespawnTestConditionRegistry constructs a minimal condition.Registry for respawn tests.
func makeRespawnTestConditionRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	return reg
}

// makeBroadcastCapture returns a broadcastFn that records all broadcast events,
// and a function to retrieve the captured slices under the mutex.
func makeBroadcastCapture() (func(string, []*gamev1.CombatEvent), func() [][]*gamev1.CombatEvent) {
	var mu sync.Mutex
	var captured [][]*gamev1.CombatEvent
	fn := func(_ string, events []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, events)
	}
	get := func() [][]*gamev1.CombatEvent {
		mu.Lock()
		defer mu.Unlock()
		return captured
	}
	return fn, get
}

// makeRespawnHandler constructs a CombatHandler with the given npcMgr and respawnMgr.
func makeRespawnHandler(
	t *testing.T,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	respawnMgr *npc.RespawnManager,
	broadcastFn func(string, []*gamev1.CombatEvent),
) *gameserver.CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	return gameserver.NewCombatHandler(
		engine, npcMgr, sessMgr, roller, broadcastFn,
		respawnTestRoundDuration,
		makeRespawnTestConditionRegistry(),
		nil, nil, nil, nil,
		respawnMgr,
	)
}

// spawnHighHPNPC creates an NPC with high HP so the player can die before killing it when needed.
func spawnRespawnTestNPC(t *testing.T, npcMgr *npc.Manager, roomID, templateID string, maxHP int) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:           templateID,
		Name:         "Ganger",
		Level:        1,
		MaxHP:        maxHP,
		AC:           1, // very low AC so player attacks always hit
		Perception:   2,
		RespawnDelay: "1m",
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)
	return inst
}

// addRespawnTestPlayer registers a player with enough HP to survive and high enough
// stats to reliably kill the NPC (via repeated rounds).
func addRespawnTestPlayer(t *testing.T, sessMgr *session.Manager, uid, roomID string, hp int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(uid, "testuser", "Hero", 1, roomID, hp)
	require.NoError(t, err)
	return sess
}

// waitForCombatEnd polls broadcast events until an END event is observed or the
// timeout elapses. Returns true if an END event was seen.
func waitForCombatEnd(t *testing.T, getEvents func() [][]*gamev1.CombatEvent, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, batch := range getEvents() {
			for _, e := range batch {
				if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_END {
					return true
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestCombatHandler_NPCDiedInCombat_RemovedFromManager verifies that when a
// player wins combat (NPC HP drops to 0), the NPC instance is removed from
// npcMgr by the time combat ends.
//
// Postcondition: npcMgr.Get(inst.ID) returns false after NPC dies in combat.
func TestCombatHandler_NPCDiedInCombat_RemovedFromManager(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()

	// Build a RespawnManager so removeDeadNPCsLocked can schedule a respawn
	// (exercises the full code path).
	tmpl := &npc.Template{
		ID: "ganger", Name: "Ganger", Level: 1,
		MaxHP: 1, AC: 1, Perception: 2, RespawnDelay: "1m",
	}
	spawns := map[string][]npc.RoomSpawn{
		"room-respawn-1": {{TemplateID: "ganger", Max: 1, RespawnDelay: time.Minute}},
	}
	templates := map[string]*npc.Template{"ganger": tmpl}
	respawnMgr := npc.NewRespawnManager(spawns, templates)

	broadcastFn, getEvents := makeBroadcastCapture()
	h := makeRespawnHandler(t, npcMgr, sessMgr, respawnMgr, broadcastFn)

	const roomID = "room-respawn-1"
	// Spawn NPC with 1 HP so any single attack kills it.
	inst := spawnRespawnTestNPC(t, npcMgr, roomID, "ganger", 1)
	_ = inst

	addRespawnTestPlayer(t, sessMgr, "player-respawn-1", roomID, 100)

	// Start combat. Player attacks NPC (1 HP) — should die this or next round.
	_, err := h.Attack("player-respawn-1", "Ganger")
	require.NoError(t, err)

	// Wait for the timer to fire and resolve the round, ending combat.
	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	// The NPC instance must have been removed from npcMgr.
	_, stillExists := npcMgr.Get(inst.ID)
	assert.False(t, stillExists, "expected dead NPC instance to be removed from npcMgr after combat end")
}

// TestCombatHandler_NilRespawnMgr_NoPanic verifies that removeDeadNPCsLocked does
// not panic when respawnMgr is nil and the NPC dies in combat.
//
// Postcondition: No panic; NPC instance is removed from npcMgr.
func TestCombatHandler_NilRespawnMgr_NoPanic(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()

	broadcastFn, getEvents := makeBroadcastCapture()
	// Pass nil respawnMgr to exercise the nil guard in removeDeadNPCsLocked.
	h := makeRespawnHandler(t, npcMgr, sessMgr, nil, broadcastFn)

	const roomID = "room-respawn-nil"
	// Spawn NPC with 1 HP so any attack kills it.
	inst := spawnRespawnTestNPC(t, npcMgr, roomID, "ganger", 1)
	addRespawnTestPlayer(t, sessMgr, "player-respawn-nil", roomID, 100)

	_, err := h.Attack("player-respawn-nil", "Ganger")
	require.NoError(t, err)

	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	_, stillExists := npcMgr.Get(inst.ID)
	assert.False(t, stillExists, "expected dead NPC instance to be removed from npcMgr (nil respawnMgr path)")
}

// TestCombatHandler_LivingNPC_RetainedInManager verifies that an NPC that
// survives combat (player flees or player dies) is not removed from npcMgr.
//
// Postcondition: npcMgr.Get(inst.ID) returns true when the NPC remains alive.
func TestCombatHandler_LivingNPC_RetainedInManager(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()

	broadcastFn, getEvents := makeBroadcastCapture()
	h := makeRespawnHandler(t, npcMgr, sessMgr, nil, broadcastFn)

	const roomID = "room-retain-1"
	// Spawn NPC with very high HP so it survives the player dying.
	inst := spawnRespawnTestNPC(t, npcMgr, roomID, "ganger", 10000)
	// Player with 1 HP will die from any NPC attack.
	addRespawnTestPlayer(t, sessMgr, "player-retain-1", roomID, 1)

	_, err := h.Attack("player-retain-1", "Ganger")
	require.NoError(t, err)

	ended := waitForCombatEnd(t, getEvents, 3*time.Second)
	require.True(t, ended, "expected combat END event within timeout")

	_, stillExists := npcMgr.Get(inst.ID)
	assert.True(t, stillExists, "expected surviving NPC instance to remain in npcMgr after player death")
}
