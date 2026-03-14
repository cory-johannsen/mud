package gameserver

import (
	"math"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"pgregory.net/rapid"
)

// newRobCombatHandler builds a CombatHandler with the given npc.Manager and session.Manager,
// sharing them with the caller for direct state inspection.
//
// Precondition: npcMgr and sessMgr must be non-nil.
// Postcondition: Returns a non-nil CombatHandler using the provided managers.
func newRobCombatHandler(t testing.TB, npcMgr *npc.Manager, sessMgr *session.Manager) *CombatHandler {
	t.Helper()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	return NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
}

// TestRob_PlayerDefeated_NPCReceivesCurrency verifies that when all players are defeated
// by a robbing NPC, robPlayersLocked transfers a fraction of each player's currency to
// the NPC's Currency wallet.
//
// Precondition: player.Currency == 100; inst.RobPercent == 20.
// Postcondition: inst.Currency == 20; player.Currency == 80; one narrative event returned.
func TestRob_PlayerDefeated_NPCReceivesCurrency(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_pdn"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-rob-pdn", Name: "Bandit", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2, RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)
	inst.RobPercent = 20.0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_pdn", Username: "Victim", CharName: "Victim",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 100

	_, err = h.Attack("u_rob_pdn", "Bandit")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok, "expected active combat")
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	robEvents := h.robPlayersLocked(cbt)
	h.combatMu.Unlock()

	assert.Greater(t, len(robEvents), 0, "expected at least one rob narrative event")
	assert.Equal(t, 20, inst.Currency, "NPC should have gained 20 currency")
	assert.Equal(t, 80, sess.Currency, "player currency should be 80 after rob")
}

// TestRob_BrokePlayer_NoRob verifies that a player with 0 currency generates no rob
// events and NPC currency remains 0.
//
// Precondition: player.Currency == 0; inst.RobPercent == 20.
// Postcondition: inst.Currency == 0; robEvents is empty.
func TestRob_BrokePlayer_NoRob(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_broke"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "bandit-broke", Name: "BrokeRobber", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2, RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)
	inst.RobPercent = 20.0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_broke", Username: "Poorman", CharName: "Poorman",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 0

	_, err = h.Attack("u_rob_broke", "BrokeRobber")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	robEvents := h.robPlayersLocked(cbt)
	h.combatMu.Unlock()

	assert.Empty(t, robEvents, "no rob events when player is broke")
	assert.Equal(t, 0, inst.Currency, "NPC should gain nothing from broke player")
}

// TestRob_MultipleNPCs_SequentialDeduction verifies that two robbing NPCs each deduct
// their fraction from the remaining player currency sequentially.
//
// Precondition: player.Currency == 100; NPC-A.RobPercent == 20; NPC-B.RobPercent == 20.
// Postcondition: player.Currency == 64; instA.Currency + instB.Currency == 36.
func TestRob_MultipleNPCs_SequentialDeduction(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_multi"
	instA, err := npcMgr.Spawn(&npc.Template{
		ID: "banditA-rob", Name: "BanditA", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2, RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)
	instA.RobPercent = 20.0

	instB, err := npcMgr.Spawn(&npc.Template{
		ID: "banditB-rob", Name: "BanditB", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2, RobMultiplier: 1.0,
	}, roomID)
	require.NoError(t, err)
	instB.RobPercent = 20.0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_multi", Username: "Victim2", CharName: "Victim2",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 100

	_, err = h.Attack("u_rob_multi", "BanditA")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	// Add BanditB as a living NPC combatant so robPlayersLocked can iterate it.
	cbt.Combatants = append(cbt.Combatants, &combat.Combatant{
		ID:        instB.ID,
		Kind:      combat.KindNPC,
		Name:      instB.Name(),
		MaxHP:     30,
		CurrentHP: 30,
		AC:        13,
		Level:     1,
	})
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	_ = h.robPlayersLocked(cbt)
	h.combatMu.Unlock()

	// Sequential floor deductions: floor(100*0.20)=20, floor(80*0.20)=16; total=36.
	assert.Equal(t, 64, sess.Currency, "player currency must decrease sequentially: 100→80→64")
	assert.Equal(t, 36, instA.Currency+instB.Currency, "total stolen must be 36")
}

// TestRob_NonRobNPC_NoEffect verifies that an NPC with RobPercent == 0 does not rob
// and does not generate any events.
//
// Precondition: NPC.RobPercent == 0; player.Currency == 50.
// Postcondition: player.Currency == 50; inst.Currency == 0; robEvents empty.
func TestRob_NonRobNPC_NoEffect(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_nonrob"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-nonrob", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13,
		Perception: 2, RobMultiplier: 0.0,
	}, roomID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, inst.RobPercent)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_nonrob", Username: "Safe", CharName: "Safe",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 50

	_, err = h.Attack("u_rob_nonrob", "Goblin")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	robEvents := h.robPlayersLocked(cbt)
	h.combatMu.Unlock()

	assert.Empty(t, robEvents, "no rob events for non-robbing NPC")
	assert.Equal(t, 50, sess.Currency, "player currency unchanged")
	assert.Equal(t, 0, inst.Currency, "NPC currency unchanged")
}

// TestProperty_Rob_CurrencyConserved is a property test verifying that total currency
// (player + NPC wallet) is conserved after robPlayersLocked, and stolen amount equals
// floor(playerCurrency * robPercent / 100).
//
// Precondition: arbitrary player currency in [0,1000]; NPC RobPercent in [5.0,30.0].
// Postcondition: player.Currency + inst.Currency == initial playerCurrency.
func TestProperty_Rob_CurrencyConserved(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		playerCurrency := rapid.IntRange(0, 1000).Draw(rt, "playerCurrency")
		robPercent := rapid.Float64Range(5.0, 30.0).Draw(rt, "robPercent")

		npcMgr := npc.NewManager()
		sessMgr := session.NewManager()
		h := newRobCombatHandler(t, npcMgr, sessMgr)

		roomID := rapid.StringMatching(`[a-z][a-z0-9]{4,8}`).Draw(rt, "roomID")
		uid := "u_prop_rob_" + roomID

		inst, err := npcMgr.Spawn(&npc.Template{
			ID: "robber-" + roomID, Name: "Robber", Level: 1, MaxHP: 30, AC: 13,
			Perception: 2, RobMultiplier: 1.0,
		}, roomID)
		require.NoError(rt, err)
		inst.RobPercent = robPercent

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: "V", CharName: "V",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		require.NoError(rt, err)
		sess.Currency = playerCurrency

		_, err = h.Attack(uid, "Robber")
		require.NoError(rt, err)
		h.cancelTimer(roomID)

		h.combatMu.Lock()
		cbt, ok := h.engine.GetCombat(roomID)
		require.True(rt, ok)
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindPlayer {
				c.CurrentHP = 0
				c.Dead = true
			}
		}
		h.robPlayersLocked(cbt)
		h.combatMu.Unlock()

		total := sess.Currency + inst.Currency
		assert.Equal(rt, playerCurrency, total,
			"total currency must be conserved: player(%d)+npc(%d) != initial(%d)",
			sess.Currency, inst.Currency, playerCurrency)

		expectedStolen := int(math.Floor(float64(playerCurrency) * robPercent / 100.0))
		assert.Equal(rt, expectedStolen, inst.Currency,
			"NPC currency must equal floor(%d * %.2f / 100) = %d",
			playerCurrency, robPercent, expectedStolen)
	})
}

// TestRob_LootPayoutIncludesRobCurrency verifies that when an NPC that has accumulated
// rob currency dies with a loot table, the killer receives both loot and rob wallet currency.
//
// Precondition: NPC Loot.Currency = {Min:10, Max:10}; inst.Currency == 25; player.Currency == 0.
// Postcondition: player.Currency == 35; inst.Currency == 0.
func TestRob_LootPayoutIncludesRobCurrency(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_loot"
	loot := npc.LootTable{
		Currency: &npc.CurrencyDrop{Min: 10, Max: 10},
	}
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "lootboss-rob", Name: "LootBoss", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2, Loot: &loot,
	}, roomID)
	require.NoError(t, err)
	inst.Currency = 25

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_loot", Username: "Winner", CharName: "Winner",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 0

	_, err = h.Attack("u_rob_loot", "LootBoss")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	h.removeDeadNPCsLocked(cbt)
	h.combatMu.Unlock()

	// Loot fixed at 10 (Min==Max==10) + rob wallet 25 = 35.
	assert.Equal(t, 35, sess.Currency, "player should receive loot currency + rob wallet")
	assert.Equal(t, 0, inst.Currency, "inst.Currency must be zeroed after payout")
}

// TestRob_NoLootTable_RobWalletPaidOut verifies that when an NPC has no loot table but
// has rob currency, that currency is paid out to the killer on NPC death.
//
// Precondition: NPC has no Loot; inst.Currency == 40; player.Currency == 0.
// Postcondition: player.Currency == 40; inst.Currency == 0.
func TestRob_NoLootTable_RobWalletPaidOut(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_noloot"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "mugger-noloot", Name: "Mugger", Level: 1, MaxHP: 30, AC: 13,
		Perception: 2,
	}, roomID)
	require.NoError(t, err)
	inst.Currency = 40

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_noloot", Username: "Avenger", CharName: "Avenger",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 0

	_, err = h.Attack("u_rob_noloot", "Mugger")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	h.removeDeadNPCsLocked(cbt)
	h.combatMu.Unlock()

	assert.Equal(t, 40, sess.Currency, "player should receive rob wallet when NPC has no loot table")
	assert.Equal(t, 0, inst.Currency, "inst.Currency must be zeroed after payout")
}

// TestRob_DeadNPC_DoesNotRob verifies that a dead NPC with RobPercent > 0
// does not rob the player (REQ-T7).
//
// Precondition: NPC has RobPercent=20 but is dead in combat; player has 50 currency.
// Postcondition: inst.Currency == 0; player currency unchanged.
func TestRob_DeadNPC_DoesNotRob(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := newRobCombatHandler(t, npcMgr, sessMgr)

	const roomID = "room_rob_dead_npc"
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "robber-dead", Name: "DeadRobber", Level: 3, MaxHP: 10, AC: 10,
		Perception: 0,
	}, roomID)
	require.NoError(t, err)
	inst.RobPercent = 20.0

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_rob_dead_npc", Username: "Victim", CharName: "Victim",
		RoomID: roomID, CurrentHP: 1, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)
	sess.Currency = 50

	_, err = h.Attack("u_rob_dead_npc", "DeadRobber")
	require.NoError(t, err)
	h.cancelTimer(roomID)

	h.combatMu.Lock()
	cbt, ok := h.engine.GetCombat(roomID)
	require.True(t, ok)
	// Mark the NPC as dead in this combat
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			c.CurrentHP = 0
			c.Dead = true
		}
	}
	robEvents := h.robPlayersLocked(cbt)
	h.combatMu.Unlock()

	assert.Empty(t, robEvents, "dead NPC must not generate rob events")
	assert.Equal(t, 0, inst.Currency, "dead NPC must not gain currency")
	assert.Equal(t, 50, sess.Currency, "player currency must be unchanged when NPC is dead")
}
