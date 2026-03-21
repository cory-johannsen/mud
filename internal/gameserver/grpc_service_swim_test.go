package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// makeTestConditionRegistryWithSubmerged returns a condition registry that includes
// the standard test conditions plus an explicit "submerged" entry.
//
// Postcondition: Registry contains at least a "submerged" definition.
func makeTestConditionRegistryWithSubmerged() *condition.Registry {
	reg := makeTestConditionRegistry()
	reg.Register(&condition.ConditionDef{ID: "submerged", Name: "Submerged", DurationType: "permanent", MaxStacks: 1})
	return reg
}

// initSessionConditions initialises a player session's condition set if nil.
//
// Precondition: sess is non-nil.
// Postcondition: sess.Conditions is a non-nil *condition.ActiveSet.
func initSessionConditions(sess *session.PlayerSession) {
	if sess.Conditions == nil {
		sess.Conditions = condition.NewActiveSet()
	}
}

// newSwimWorld creates a world suitable for swim tests.
//
// Room "room_water" has Terrain="river" and a north exit (SwimDC=15) to "room_bank".
// Room "room_plain" has no water terrain and no swim exits.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a world.Manager and session.Manager containing the above rooms.
func newSwimWorld(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_swim",
		Name:        "TestSwim",
		Description: "Test zone for swim",
		StartRoom:   "room_water",
		Rooms: map[string]*world.Room{
			"room_water": {
				ID:          "room_water",
				ZoneID:      "test_swim",
				Title:       "River",
				Description: "A rushing river.",
				Terrain:     "river",
				Exits: []world.Exit{
					{Direction: world.North, TargetRoom: "room_bank", SwimDC: 15},
				},
				Properties: map[string]string{},
			},
			"room_bank": {
				ID:          "room_bank",
				ZoneID:      "test_swim",
				Title:       "River Bank",
				Description: "The river bank.",
				Exits:       []world.Exit{{Direction: world.South, TargetRoom: "room_water"}},
				Properties:  map[string]string{},
			},
			"room_plain": {
				ID:          "room_plain",
				ZoneID:      "test_swim",
				Title:       "Plain Room",
				Description: "No water here.",
				Exits:       []world.Exit{},
				Properties:  map[string]string{},
			},
		},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return wm, session.NewManager()
}

// newSwimSvc builds a minimal GameServiceServer using the swim world.
//
// Precondition: t must be non-nil.
// Postcondition: Returns a non-nil svc and sessMgr sharing the swim world.
func newSwimSvc(t *testing.T, roller *dice.Roller, condReg *condition.Registry) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := newSwimWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// newSwimSvcWithCombat builds a GameServiceServer with a real CombatHandler for swim tests
// that require in-combat state.
//
// Precondition: t must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler.
func newSwimSvcWithCombat(t *testing.T, roller *dice.Roller, condReg *condition.Registry) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := newSwimWorld(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// TestHandleSwim_NoDirection verifies that handleSwim with empty direction returns a usage error.
//
// Precondition: Player exists; req.Direction == "".
// Postcondition: Message event contains "direction".
func TestHandleSwim_NoDirection(t *testing.T) {
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr := newSwimSvc(t, nil, condReg)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_nodir",
		Username:  "Alice",
		CharName:  "Alice",
		Role:      "player",
		RoomID:    "room_water",
		CurrentHP: 10,
		MaxHP:     10,
	})
	require.NoError(t, err)

	ev, err := svc.handleSwim("u_swim_nodir", &gamev1.SwimRequest{Direction: ""})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msgEvt := ev.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "direction")
}

// TestHandleSwim_NoSwimmableExit verifies message when direction has no swim exit.
//
// Precondition: Player in room_water; direction "south" (no exit in that direction from room_water).
// Postcondition: Message event contains "no water".
func TestHandleSwim_NoSwimmableExit(t *testing.T) {
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr := newSwimSvc(t, nil, condReg)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_noexit",
		Username:  "Alice",
		CharName:  "Alice",
		Role:      "player",
		RoomID:    "room_water",
		CurrentHP: 10,
		MaxHP:     10,
	})
	require.NoError(t, err)

	ev, err := svc.handleSwim("u_swim_noexit", &gamev1.SwimRequest{Direction: "south"})
	require.NoError(t, err)
	require.NotNil(t, ev)
	msgEvt := ev.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "no water")
}

// TestHandleSwim_RoomNotWater_NotSubmerged verifies that handleSwim in a plain room with no
// swim exit and no submerged condition returns "no water".
//
// Precondition: Player is in "room_plain" with no water terrain; direction "north" (no exit).
// Postcondition: Message event contains "no water".
func TestHandleSwim_RoomNotWater_NotSubmerged(t *testing.T) {
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr := newSwimSvc(t, nil, condReg)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:      "u_swim_nowater",
		Username: "Swimmer",
		CharName: "Swimmer",
		RoomID:   "room_plain",
		Role:     "player",
	})
	require.NoError(t, err)

	event, err := svc.handleSwim("u_swim_nowater", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event")
	assert.Contains(t, msgEvt.Content, "no water")
}

// TestHandleSwim_Success verifies player moves to destination on high roll.
//
// Precondition: Player in room_water; direction "north"; SwimDC=15; roll=18 → success.
// Postcondition: Player RoomID == "room_bank".
func TestHandleSwim_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// val=17: Intn(20)=17 → roll=18; total=18 >= DC=15 → Success.
	src := &fixedDiceSource{val: 17}
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr := newSwimSvc(t, roller, condReg)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_success",
		Username:  "Alice",
		CharName:  "Alice",
		Role:      "player",
		RoomID:    "room_water",
		CurrentHP: 10,
		MaxHP:     10,
	})
	require.NoError(t, err)

	_, err = svc.handleSwim("u_swim_success", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_swim_success")
	require.True(t, ok)
	assert.Equal(t, "room_bank", sess.RoomID)
}

// TestHandleSwim_CritFailure verifies that a critical failure on the muscle check
// applies the submerged condition and reduces player HP.
//
// Precondition: Player is in "room_water" (Terrain="river", SwimDC=15).
// Dice fixed at val=0: roll=1, bonus=0, total=1; 1 < DC-10=5 → CritFailure.
// Postcondition: Player HP decreased; submerged condition applied; message contains "pulled under".
func TestHandleSwim_CritFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // Intn(20)=0 → roll=1; 1 < DC-10=5 → CritFailure
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()

	svc, sessMgr := newSwimSvc(t, roller, condReg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_cf",
		Username:  "Swimmer",
		CharName:  "Swimmer",
		RoomID:    "room_water",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	initSessionConditions(sess)

	hpBefore := sess.CurrentHP

	event, err := svc.handleSwim("u_swim_cf", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on crit failure")
	assert.Contains(t, msgEvt.Content, "pulled under")

	assert.Less(t, sess.CurrentHP, hpBefore, "player HP must decrease on crit failure")

	require.NotNil(t, sess.Conditions, "conditions must be initialised")
	assert.True(t, sess.Conditions.Has("submerged"), "player must have submerged condition after crit failure")
}

// TestHandleSwim_CritFailure_DrowningDamage verifies 1d6 drowning damage on crit fail.
//
// Precondition: Player in room_water; roll=1 (crit fail via val=0); d6 with val=0 → damage=1.
// Postcondition: HP = 20 - 1 = 19.
func TestHandleSwim_CritFailure_DrowningDamage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 0} // d20=1 (crit fail), d6=1 (damage)
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr := newSwimSvc(t, roller, condReg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_dmg",
		Username:  "Alice",
		CharName:  "Alice",
		Role:      "player",
		RoomID:    "room_water",
		CurrentHP: 20,
		MaxHP:     20,
	})
	require.NoError(t, err)
	initSessionConditions(sess)

	_, err = svc.handleSwim("u_swim_dmg", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)

	assert.Equal(t, 19, sess.CurrentHP) // 20 - 1 = 19
}

// TestHandleSwim_SubmergedSurface verifies that a player with the submerged condition
// who succeeds the muscle check loses the submerged condition.
//
// Precondition: Player is in "room_plain" with submerged condition; direction "north" (no exit → dc=12 fallback).
// Dice fixed at val=19: roll=20, total=20, dc=12 → Success. Submerged clears.
// Postcondition: submerged condition removed; message contains "surface".
func TestHandleSwim_SubmergedSurface(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 19} // Intn(20)=19 → roll=20; total=20 >= DC=12 → Success
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()

	svc, sessMgr := newSwimSvc(t, roller, condReg)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_surface",
		Username:  "Swimmer",
		CharName:  "Swimmer",
		RoomID:    "room_plain",
		CurrentHP: 10,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	initSessionConditions(sess)

	// Apply submerged condition.
	def, ok := condReg.Get("submerged")
	require.True(t, ok, "submerged condition must be in registry")
	err = sess.Conditions.Apply("u_swim_surface", def, 1, -1)
	require.NoError(t, err)
	require.True(t, sess.Conditions.Has("submerged"), "precondition: session must have submerged condition")

	event, err := svc.handleSwim("u_swim_surface", &gamev1.SwimRequest{Direction: "north"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event on success")
	assert.Contains(t, msgEvt.Content, "surface")

	assert.False(t, sess.Conditions.Has("submerged"), "submerged condition must be removed on success")
}

// TestProperty_SwimDC_TerrainDefaults verifies terrain default DC table.
//
// Precondition: exit.SwimDC == 0; terrain in {sewer, river, ocean, flooded}.
// Postcondition: swimDCForExit returns the expected DC for that terrain.
func TestProperty_SwimDC_TerrainDefaults(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		terrain := rapid.SampledFrom([]string{"sewer", "river", "ocean", "flooded"}).Draw(rt, "terrain")
		expected := map[string]int{"sewer": 10, "river": 15, "ocean": 20, "flooded": 12}[terrain]
		got := swimDCForExit(world.Exit{SwimDC: 0}, terrain)
		assert.Equal(rt, expected, got)
	})
}

// TestSubmergedDrowning verifies that resolveAndAdvanceLocked applies 1d6 drowning damage
// to a player with the submerged condition at the start of each combat round (TERRAIN-13).
//
// Precondition: Player is in combat with the submerged condition applied.
// Postcondition: Player HP is strictly less than before after resolveAndAdvanceLocked.
func TestSubmergedDrowning(t *testing.T) {
	logger := zaptest.NewLogger(t)
	// Use fixedDiceSource val=5 so Intn(6)=5 → d6 roll=6 (guaranteed non-zero damage).
	src := &fixedDiceSource{val: 5}
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()

	_, sessMgr, npcMgr, combatHandler := newSwimSvcWithCombat(t, roller, condReg)

	const roomID = "room_water"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-drown", Name: "Bandit", Level: 1, MaxHP: 30, AC: 10, Awareness: 0,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_drown",
		Username:  "Diver",
		CharName:  "Diver",
		RoomID:    roomID,
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	initSessionConditions(sess)

	// Apply submerged condition.
	def, ok := condReg.Get("submerged")
	require.True(t, ok, "submerged condition must be in registry")
	err = sess.Conditions.Apply("u_drown", def, 1, -1)
	require.NoError(t, err)
	require.True(t, sess.Conditions.Has("submerged"), "precondition: session must have submerged condition")

	// Start combat — Attack registers the player and NPC into the engine.
	_, err = combatHandler.Attack("u_drown", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	hpBefore := sess.CurrentHP

	// Trigger round resolution directly, which resolves current round and starts the next.
	combatHandler.combatMu.Lock()
	cbt, ok := combatHandler.engine.GetCombat(roomID)
	require.True(t, ok, "active combat must exist after Attack")
	combatHandler.resolveAndAdvanceLocked(roomID, cbt)
	combatHandler.combatMu.Unlock()
	combatHandler.cancelTimer(roomID)

	assert.Less(t, sess.CurrentHP, hpBefore, "player HP must decrease due to drowning damage at round-start when submerged")
}

// TestProperty_SubmergedDrowning_HPAlwaysDecreases is a property-based test verifying
// that a submerged player always loses HP at the start of each combat round.
//
// Precondition: Player is in combat with submerged condition; HP is drawn from [2, 100].
// Postcondition: HP after resolveAndAdvanceLocked is always strictly less than HP before.
func TestProperty_SubmergedDrowning_HPAlwaysDecreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		initialHP := rapid.IntRange(2, 100).Draw(rt, "initialHP")

		logger := zaptest.NewLogger(t)
		// val=5 → Intn(6)=5 → d6=6, guaranteed non-zero damage regardless of HP.
		src := &fixedDiceSource{val: 5}
		roller := dice.NewLoggedRoller(src, logger)
		condReg := makeTestConditionRegistryWithSubmerged()

		_, sessMgr, npcMgr, combatHandler := newSwimSvcWithCombat(t, roller, condReg)

		const roomID = "room_water"
		_, err := npcMgr.Spawn(&npc.Template{
			ID: "bandit-prop-drown", Name: "Bandit", Level: 1, MaxHP: 30, AC: 10, Awareness: 0,
		}, roomID)
		require.NoError(t, err)

		uid := fmt.Sprintf("u_drown_prop_%d", initialHP)
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       uid,
			Username:  "Diver",
			CharName:  "Diver",
			RoomID:    roomID,
			CurrentHP: initialHP,
			MaxHP:     initialHP,
			Role:      "player",
		})
		require.NoError(t, err)
		initSessionConditions(sess)

		def, ok := condReg.Get("submerged")
		require.True(t, ok)
		err = sess.Conditions.Apply(uid, def, 1, -1)
		require.NoError(t, err)

		_, err = combatHandler.Attack(uid, "Bandit")
		require.NoError(t, err)
		combatHandler.cancelTimer(roomID)

		hpBefore := sess.CurrentHP

		combatHandler.combatMu.Lock()
		cbt, ok := combatHandler.engine.GetCombat(roomID)
		require.True(t, ok)
		combatHandler.resolveAndAdvanceLocked(roomID, cbt)
		combatHandler.combatMu.Unlock()
		combatHandler.cancelTimer(roomID)

		if sess.CurrentHP >= hpBefore {
			rt.Fatalf("expected HP to decrease from drowning: before=%d after=%d", hpBefore, sess.CurrentHP)
		}
	})
}

// TestHandleSwim_BlocksAttackWhenSubmerged verifies that handleAttack returns a blocked
// message when the player has the submerged condition active.
//
// Precondition: Player has submerged condition applied.
// Postcondition: handleAttack returns a message event containing "submerged" and does not proceed to combat.
func TestHandleSwim_BlocksAttackWhenSubmerged(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 10} // mid-range roll, success in attack
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr, npcMgr, combatHandler := newSwimSvcWithCombat(t, roller, condReg)

	const roomID = "room_water"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-swim-block", Name: "Bandit", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_swim_block",
		Username:  "Swimmer",
		CharName:  "Swimmer",
		RoomID:    roomID,
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat
	initSessionConditions(sess)

	_, err = combatHandler.Attack("u_swim_block", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Apply submerged condition.
	def, ok := condReg.Get("submerged")
	require.True(t, ok)
	err = sess.Conditions.Apply("u_swim_block", def, 1, -1)
	require.NoError(t, err)

	event, err := svc.handleAttack("u_swim_block", &gamev1.AttackRequest{Target: "Bandit"})
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected a message event blocking attack")
	assert.Contains(t, msgEvt.Content, "submerged")
}

// TestHandleSwim_BlocksAllAttacksWhenSubmerged verifies that attack, burst, auto, and reload
// are all blocked with a "submerged" message when the player has the submerged condition.
//
// Precondition: Player has submerged condition applied and is in combat.
// Postcondition: Each blocked command returns a message event containing "submerged".
func TestHandleSwim_BlocksAllAttacksWhenSubmerged(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := &fixedDiceSource{val: 10}
	roller := dice.NewLoggedRoller(src, logger)
	condReg := makeTestConditionRegistryWithSubmerged()
	svc, sessMgr, npcMgr, combatHandler := newSwimSvcWithCombat(t, roller, condReg)

	const roomID = "room_water"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-all-block", Name: "Bandit", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "u_all_block",
		Username:  "Blocker",
		CharName:  "Blocker",
		RoomID:    roomID,
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	sess.Status = statusInCombat
	initSessionConditions(sess)

	_, err = combatHandler.Attack("u_all_block", "Bandit")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Apply submerged condition.
	def, ok := condReg.Get("submerged")
	require.True(t, ok)
	err = sess.Conditions.Apply("u_all_block", def, 1, -1)
	require.NoError(t, err)

	cases := []struct {
		name string
		call func() (*gamev1.ServerEvent, error)
	}{
		{
			name: "attack",
			call: func() (*gamev1.ServerEvent, error) {
				return svc.handleAttack("u_all_block", &gamev1.AttackRequest{Target: "Bandit"})
			},
		},
		{
			name: "fire burst",
			call: func() (*gamev1.ServerEvent, error) {
				return svc.handleFireBurst("u_all_block", &gamev1.FireBurstRequest{Target: "Bandit"})
			},
		},
		{
			name: "fire automatic",
			call: func() (*gamev1.ServerEvent, error) {
				return svc.handleFireAutomatic("u_all_block", &gamev1.FireAutomaticRequest{Target: "Bandit"})
			},
		},
		{
			name: "reload",
			call: func() (*gamev1.ServerEvent, error) {
				return svc.handleReload("u_all_block", &gamev1.ReloadRequest{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := tc.call()
			require.NoError(t, err)
			require.NotNil(t, event)
			msgEvt := event.GetMessage()
			require.NotNil(t, msgEvt, fmt.Sprintf("expected a message event blocking %s", tc.name))
			assert.Contains(t, msgEvt.Content, "submerged")
		})
	}
}
