package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/trap"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// makeTrapDeploySvc builds on makeTrapSvc, adding an invRegistry with a deployable mine.
func makeTrapDeploySvc(t *testing.T) (*GameServiceServer, *trap.TrapManager, *inventory.Registry) {
	t.Helper()
	svc, trapMgr := makeTrapSvc(t)

	reg := inventory.NewRegistry()
	mineTmpl := &trap.TrapTemplate{
		ID: "mine", Name: "Mine", TriggerRangeFt: 5, BlastRadiusFt: 10,
		ResetMode: trap.ResetOneShot,
		Payload:   &trap.TrapPayload{Type: "mine", Damage: "4d6"},
	}
	svc.trapTemplates["mine"] = mineTmpl

	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID: "deployable_mine", Name: "Deployable Mine",
		Kind: inventory.KindTrap, TrapTemplateRef: "mine",
		Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
	}))
	svc.invRegistry = reg
	return svc, trapMgr, reg
}

func TestHandleDeployTrap_SessionNotFound(t *testing.T) {
	svc, _, _ := makeTrapDeploySvc(t)
	// No player added — uid does not exist.
	ev, err := svc.handleDeployTrap("no_such_uid", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "not in the game")
}

func TestHandleDeployTrap_OutOfCombat_Success(t *testing.T) {
	svc, trapMgr, reg := makeTrapDeploySvc(t)

	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "deployer", Username: "deployer", CharName: "deployer", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	_, err = sess.Backpack.Add("deployable_mine", 1, reg)
	require.NoError(t, err)

	ev, err := svc.handleDeployTrap("deployer", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "arm")

	// Backpack must be empty after deploy
	items := sess.Backpack.Items()
	assert.Empty(t, items)

	// TrapManager must have a new armed consumable trap in room_a
	traps := trapMgr.TrapsForRoom("test", "room_a")
	found := false
	for _, id := range traps {
		inst, ok := trapMgr.GetTrap(id)
		if ok && inst.IsConsumable && inst.Armed {
			assert.Equal(t, 0, inst.DeployPosition, "out-of-combat deploy must use position 0")
			found = true
			break
		}
	}
	assert.True(t, found, "a consumable trap must be armed after deploy")
}

func TestHandleDeployTrap_InCombat_CostsOneAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
	trapMgr := trap.NewTrapManager()
	mineTmpl := &trap.TrapTemplate{ID: "mine", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot}
	tmplMap := map[string]*trap.TrapTemplate{"mine": mineTmpl}
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID: "deployable_mine", Name: "Deployable Mine",
		Kind: inventory.KindTrap, TrapTemplateRef: "mine",
		Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
	}))

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		nil,
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, roller, nil, npcMgr, combatHandler, nil, // 7-12
		nil, nil, nil, nil, nil, nil,                  // 13-18
		nil, nil, nil, nil, nil, nil, nil, nil, "",    // 19-27
		nil, nil, nil,                                 // 28-30
		nil, nil, nil,                                 // 31-33
		nil, nil, nil, nil, nil, nil, nil,             // 34-40
		nil, nil,                                      // 41-42
		nil,                                           // 43
		nil,                                           // 44
		trapMgr, tmplMap,                              // 45-46
	)
	svc.invRegistry = reg

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-dep", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "dep_player", Username: "dep_player", CharName: "dep_player", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, err = combatHandler.Attack("dep_player", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer("room_a")

	_, err = sess.Backpack.Add("deployable_mine", 1, reg)
	require.NoError(t, err)

	apBefore := combatHandler.RemainingAP("dep_player")
	require.GreaterOrEqual(t, apBefore, 1)

	ev, deployErr := svc.handleDeployTrap("dep_player", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, deployErr)
	assert.Contains(t, ev.GetMessage().GetContent(), "arm")
	assert.Equal(t, apBefore-1, combatHandler.RemainingAP("dep_player"), "deploy must cost exactly 1 AP in combat")
}

func TestHandleDeployTrap_NotEnoughAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
	trapMgr := trap.NewTrapManager()
	tmplMap := map[string]*trap.TrapTemplate{
		"mine": {ID: "mine", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot},
	}
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID: "deployable_mine", Name: "Deployable Mine",
		Kind: inventory.KindTrap, TrapTemplateRef: "mine",
		Weight: 2.0, Stackable: true, MaxStack: 5, Value: 300,
	}))

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		nil,
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		trapMgr, tmplMap,
	)
	svc.invRegistry = reg

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-noap", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "noap", Username: "noap", CharName: "noap", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, err = combatHandler.Attack("noap", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer("room_a")

	// Exhaust all AP
	rem := combatHandler.RemainingAP("noap")
	if rem > 0 {
		require.NoError(t, combatHandler.SpendAP("noap", rem))
	}

	_, err = sess.Backpack.Add("deployable_mine", 1, reg)
	require.NoError(t, err)

	ev, deployErr := svc.handleDeployTrap("noap", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, deployErr)
	assert.Contains(t, ev.GetMessage().GetContent(), "Not enough AP")
	// Item must NOT be consumed
	items := sess.Backpack.Items()
	total := 0
	for _, it := range items {
		total += it.Quantity
	}
	assert.Equal(t, 1, total, "item must remain in backpack when AP denied")
}

func TestHandleDeployTrap_ItemNotFound(t *testing.T) {
	svc, _, _ := makeTrapDeploySvc(t)
	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "nf", Username: "nf", CharName: "nf", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	ev, err := svc.handleDeployTrap("nf", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "don't have")
}

func TestHandleDeployTrap_WrongKind(t *testing.T) {
	svc, _, reg := makeTrapDeploySvc(t)
	// Use KindConsumable — stackable items require MaxStack >= 1.
	require.NoError(t, reg.RegisterItem(&inventory.ItemDef{
		ID: "bandage", Name: "Bandage", Kind: inventory.KindConsumable,
		Stackable: true, MaxStack: 10,
	}))
	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "wk", Username: "wk", CharName: "wk", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	_, err = sess.Backpack.Add("bandage", 1, reg)
	require.NoError(t, err)

	ev, err := svc.handleDeployTrap("wk", &gamev1.DeployTrapRequest{ItemName: "Bandage"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "can't deploy")
}

func TestProperty_DeployTrap_BackpackDecrementsByOne(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, _, reg := makeTrapDeploySvc(t)
		count := rapid.IntRange(1, 5).Draw(rt, "count")
		uid := "prop_dep_" + rapid.StringN(3, 8, -1).Draw(rt, "uid")
		sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, Role: "player",
			RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
		})
		require.NoError(rt, err)
		sess.Conditions = condition.NewActiveSet()
		_, err = sess.Backpack.Add("deployable_mine", count, reg)
		require.NoError(rt, err)

		_, err = svc.handleDeployTrap(uid, &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
		require.NoError(rt, err)

		total := 0
		for _, it := range sess.Backpack.Items() {
			total += it.Quantity
		}
		assert.Equal(t, count-1, total, "exactly 1 item must be consumed per deploy")
	})
}

// REQ-CTR-5: A deployed trap must be position-anchored — subsequent movement by
// the deploying player must not change the trap's DeployPosition.
func TestHandleDeployTrap_PositionAnchoredAfterPlayerMoves(t *testing.T) {
	svc, trapMgr, reg := makeTrapDeploySvc(t)

	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "anchor_player", Username: "anchor_player", CharName: "anchor_player", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	_, err = sess.Backpack.Add("deployable_mine", 1, reg)
	require.NoError(t, err)

	// Deploy trap out-of-combat — position is 0.
	ev, err := svc.handleDeployTrap("anchor_player", &gamev1.DeployTrapRequest{ItemName: "Deployable Mine"})
	require.NoError(t, err)
	require.Contains(t, ev.GetMessage().GetContent(), "arm")

	// Find the deployed trap.
	traps := trapMgr.TrapsForRoom("test", "room_a")
	require.NotEmpty(t, traps)
	var deployedID string
	for _, id := range traps {
		inst, ok := trapMgr.GetTrap(id)
		if ok && inst.IsConsumable && inst.Armed {
			deployedID = id
			break
		}
	}
	require.NotEmpty(t, deployedID, "deployed trap must be found")

	origInst, ok := trapMgr.GetTrap(deployedID)
	require.True(t, ok)
	origPos := origInst.DeployPosition

	// Simulate player movement by changing their RoomID.
	sess.RoomID = "room_b"

	// The trap's DeployPosition must not have changed.
	laterInst, ok := trapMgr.GetTrap(deployedID)
	require.True(t, ok)
	assert.Equal(t, origPos, laterInst.DeployPosition,
		"trap DeployPosition must be immutable after player moves (REQ-CTR-5)")
}
