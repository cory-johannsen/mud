package gameserver

import (
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

// makeCoverConditionRegistry returns a condition registry with the three cover tiers.
func makeCoverConditionRegistry() *condition.Registry {
	reg := makeTestConditionRegistry()
	reg.Register(&condition.ConditionDef{
		ID: "lesser_cover", Name: "Lesser Cover", DurationType: "encounter",
		MaxStacks: 0, ACPenalty: 1, ReflexBonus: 1, StealthBonus: 1,
	})
	reg.Register(&condition.ConditionDef{
		ID: "standard_cover", Name: "Standard Cover", DurationType: "encounter",
		MaxStacks: 0, ACPenalty: 2, ReflexBonus: 2, StealthBonus: 2,
	})
	reg.Register(&condition.ConditionDef{
		ID: "greater_cover", Name: "Greater Cover", DurationType: "encounter",
		MaxStacks: 0, ACPenalty: 4, ReflexBonus: 4, StealthBonus: 4,
	})
	return reg
}

// testWorldWithCoverRoom creates a world manager with a room containing a standard_cover equipment item.
func testWorldWithCoverRoom(t *testing.T) (*world.Manager, *session.Manager) {
	t.Helper()
	return testWorldWithCoverTier(t, combat.CoverTierStandard)
}

// testWorldWithCoverTier creates a world manager with a room containing cover equipment of the given tier.
func testWorldWithCoverTier(t *testing.T, tier string) (*world.Manager, *session.Manager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test_cover",
		Name:        "Cover Test Zone",
		Description: "A zone for cover tests.",
		StartRoom:   "room_cover",
		Rooms: map[string]*world.Room{
			"room_cover": {
				ID:          "room_cover",
				ZoneID:      "test_cover",
				Title:       "Cover Room",
				Description: "A room with cover objects.",
				Properties:  map[string]string{},
				Equipment: []world.RoomEquipmentConfig{
					{
						ItemID:    "barrel_01",
						CoverTier: tier,
						CoverHP:   0,
					},
				},
			},
			"room_no_cover": {
				ID:          "room_no_cover",
				ZoneID:      "test_cover",
				Title:       "Open Room",
				Description: "A room with no cover.",
				Properties:  map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)
	return worldMgr, session.NewManager()
}

// newCoverSvc builds a GameServiceServer suitable for handleTakeCover tests.
func newCoverSvc(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) *GameServiceServer {
	t.Helper()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeCoverConditionRegistry()
	return newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
}


// newCoverSvcWithCombat builds a GameServiceServer with a CombatHandler wired, using the
// cover condition registry and the provided world manager.
func newCoverSvcWithCombat(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) (*GameServiceServer, *npc.Manager, *CombatHandler) {
	t.Helper()
	return newCoverSvcWithCombatTB(t, worldMgr, sessMgr)
}

// newCoverSvcWithCombatTB builds a GameServiceServer with a CombatHandler from a *testing.T.
func newCoverSvcWithCombatTB(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) (*GameServiceServer, *npc.Manager, *CombatHandler) {
	t.Helper()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeCoverConditionRegistry()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, worldMgr, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, npcMgr, combatHandler
}

// TestMaxNPCStealthInRoomWithCoverBonus verifies that for any cover tier and any stealth base value,
// an NPC with that cover condition applied has an effective stealth DC of at least stealthBase + coverBonus.
func TestMaxNPCStealthInRoomWithCoverBonus(t *testing.T) {
	coverBonusByTier := map[string]int{
		combat.CoverTierLesser:   1,
		combat.CoverTierStandard: 2,
		combat.CoverTierGreater:  4,
	}
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{
			combat.CoverTierLesser, combat.CoverTierStandard, combat.CoverTierGreater,
		}).Draw(rt, "tier")
		stealthBase := rapid.IntRange(0, 20).Draw(rt, "stealthBase")
		expectedBonus := coverBonusByTier[tier]

		worldMgr, sessMgr := testWorldWithCoverTier(t, tier)
		svc, npcMgr, combatHandler := newCoverSvcWithCombatTB(t, worldMgr, sessMgr)

		const roomID = "room_cover"
		inst, err := npcMgr.Spawn(&npc.Template{
			ID: "stalker-cover-stealth", Name: "Stalker", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
		}, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}
		inst.Stealth = stealthBase

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_stl_cover", Username: "Scout", CharName: "Scout",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}
		sess.Conditions = condition.NewActiveSet()
		sess.Status = statusInCombat

		if _, err = combatHandler.Attack("u_stl_cover", "Stalker"); err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		combatHandler.cancelTimer(roomID)

		cbt, ok := combatHandler.GetCombatForRoom(roomID)
		if !ok {
			rt.Fatal("no combat found for room")
		}
		var npcCombatant *combat.Combatant
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC {
				npcCombatant = c
				break
			}
		}
		if npcCombatant == nil {
			rt.Fatal("NPC combatant not found")
		}

		condReg := makeCoverConditionRegistry()
		condID := tier + "_cover"
		def, ok := condReg.Get(condID)
		if !ok {
			rt.Fatalf("condition %q not found in registry", condID)
		}
		if cbt.Conditions[npcCombatant.ID] == nil {
			cbt.Conditions[npcCombatant.ID] = condition.NewActiveSet()
		}
		if err := cbt.Conditions[npcCombatant.ID].Apply(npcCombatant.ID, def, 1, -1); err != nil {
			rt.Fatalf("Apply condition: %v", err)
		}

		dc := svc.maxNPCStealthInRoom(roomID)
		if dc < stealthBase+expectedBonus {
			rt.Errorf("stealth DC: got %d, want >= %d (base=%d cover=%d tier=%s)",
				dc, stealthBase+expectedBonus, stealthBase, expectedBonus, tier)
		}
	})
}

// TestNPCWithUseCoverStrategyTakesCover verifies that for any cover tier available in the room,
// an NPC with UseCover=true ends up with CoverTier set and the correct cover condition applied.
func TestNPCWithUseCoverStrategyTakesCover(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{
			combat.CoverTierLesser, combat.CoverTierStandard, combat.CoverTierGreater,
		}).Draw(rt, "tier")
		condID := tier + "_cover"

		worldMgr, sessMgr := testWorldWithCoverTier(t, tier)
		_, npcMgr, combatHandler := newCoverSvcWithCombatTB(t, worldMgr, sessMgr)

		const roomID = "room_cover"
		_, err := npcMgr.Spawn(&npc.Template{
			ID:     "sniper-use-cover",
			Name:   "Sniper",
			Level:  1,
			MaxHP:  20,
			AC:     13,
			Combat: npc.CombatStrategy{UseCover: true},
		}, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_npc_uc_yes", Username: "Target", CharName: "Target",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}
		sess.Conditions = condition.NewActiveSet()
		sess.Status = statusInCombat

		if _, err = combatHandler.Attack("u_npc_uc_yes", "Sniper"); err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		combatHandler.cancelTimer(roomID)

		cbt, ok := combatHandler.GetCombatForRoom(roomID)
		if !ok {
			rt.Fatal("no combat found for room")
		}

		combatHandler.combatMu.Lock()
		combatHandler.autoQueueNPCsLocked(cbt)
		combatHandler.combatMu.Unlock()

		var npcCombatant *combat.Combatant
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC {
				npcCombatant = c
				break
			}
		}
		if npcCombatant == nil {
			rt.Fatal("NPC combatant not found")
		}
		if npcCombatant.CoverTier == "" {
			rt.Errorf("NPC with UseCover=true must have cover tier set (tier=%s)", tier)
		}
		if npcCombatant.CoverTier != tier {
			rt.Errorf("NPC cover tier: got %q, want %q", npcCombatant.CoverTier, tier)
		}
		if npcCombatant.CoverEquipmentID == "" {
			rt.Errorf("NPC with UseCover=true must have cover equipment ID set")
		}

		condSet, ok := cbt.Conditions[npcCombatant.ID]
		if !ok {
			rt.Errorf("NPC with UseCover=true must have conditions entry; tier=%s condID=%s", tier, condID)
		} else if !condSet.Has(condID) {
			rt.Errorf("NPC must have condition %q applied (tier=%s)", condID, tier)
		}
	})
}

// TestNPCWithoutUseCoverStrategyDoesNotTakeCover verifies that regardless of room cover tier,
// an NPC with UseCover=false (default) never has a cover condition applied.
func TestNPCWithoutUseCoverStrategyDoesNotTakeCover(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{
			combat.CoverTierLesser, combat.CoverTierStandard, combat.CoverTierGreater,
		}).Draw(rt, "tier")

		worldMgr, sessMgr := testWorldWithCoverTier(t, tier)
		_, npcMgr, combatHandler := newCoverSvcWithCombatTB(t, worldMgr, sessMgr)

		const roomID = "room_cover"
		_, err := npcMgr.Spawn(&npc.Template{
			ID:    "grunt-no-cover",
			Name:  "Grunt",
			Level: 1,
			MaxHP: 20,
			AC:    13,
			// Combat.UseCover defaults to false.
		}, roomID)
		if err != nil {
			rt.Fatalf("Spawn: %v", err)
		}

		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: "u_npc_uc_no", Username: "Target2", CharName: "Target2",
			RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}
		sess.Conditions = condition.NewActiveSet()
		sess.Status = statusInCombat

		if _, err = combatHandler.Attack("u_npc_uc_no", "Grunt"); err != nil {
			rt.Fatalf("Attack: %v", err)
		}
		combatHandler.cancelTimer(roomID)

		cbt, ok := combatHandler.GetCombatForRoom(roomID)
		if !ok {
			rt.Fatal("no combat found for room")
		}

		combatHandler.combatMu.Lock()
		combatHandler.autoQueueNPCsLocked(cbt)
		combatHandler.combatMu.Unlock()

		var npcCombatant *combat.Combatant
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindNPC {
				npcCombatant = c
				break
			}
		}
		if npcCombatant == nil {
			rt.Fatal("NPC combatant not found")
		}
		if npcCombatant.CoverTier != "" {
			rt.Errorf("NPC with UseCover=false must NOT have cover tier set; got %q (room tier=%s)",
				npcCombatant.CoverTier, tier)
		}
		// Verify no cover condition was applied.
		condID := tier + "_cover"
		if condSet, ok := cbt.Conditions[npcCombatant.ID]; ok {
			if condSet.Has(condID) {
				rt.Errorf("NPC with UseCover=false must NOT have condition %q applied (room tier=%s)", condID, tier)
			}
		}
	})
}

// TestHandleTakeCover_NoCoverInRoom verifies that a room with no equipment yields the "no cover" message.
func TestHandleTakeCover_NoCoverInRoom(t *testing.T) {
	worldMgr, sessMgr := testWorldWithCoverRoom(t)
	svc := newCoverSvc(t, worldMgr, sessMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_cover_nc", Username: "Fighter", CharName: "Fighter",
		RoomID: "room_no_cover", CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	event, err := svc.handleTakeCover("u_cover_nc")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "no cover available")
}

// TestHandleTakeCover_StandardCoverApplied verifies that standard cover is applied from room equipment.
func TestHandleTakeCover_StandardCoverApplied(t *testing.T) {
	worldMgr, sessMgr := testWorldWithCoverRoom(t)
	svc := newCoverSvc(t, worldMgr, sessMgr)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_cover_std", Username: "Ranger", CharName: "Ranger",
		RoomID: "room_cover", CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	event, err := svc.handleTakeCover("u_cover_std")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected message event, got: %v", event)
	assert.Contains(t, msgEvt.Content, "standard cover")

	// Condition must be applied.
	assert.True(t, sess.Conditions.Has("standard_cover"), "expected standard_cover condition to be set")
}

// TestHandleTakeCover_AlreadyInEqualCover verifies that re-taking equal cover yields "already in" message.
func TestHandleTakeCover_AlreadyInEqualCover(t *testing.T) {
	worldMgr, sessMgr := testWorldWithCoverRoom(t)
	svc := newCoverSvc(t, worldMgr, sessMgr)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_cover_eq", Username: "Rogue", CharName: "Rogue",
		RoomID: "room_cover", CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	reg := makeCoverConditionRegistry()
	def, ok := reg.Get("standard_cover")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply("u_cover_eq", def, 1, -1))

	event, err := svc.handleTakeCover("u_cover_eq")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt)
	assert.Contains(t, msgEvt.Content, "already in")
}

// TestHandleTakeCover_MissingSession verifies handleTakeCover returns error when session is missing.
func TestHandleTakeCover_MissingSession(t *testing.T) {
	worldMgr, sessMgr := testWorldWithCoverRoom(t)
	svc := newCoverSvc(t, worldMgr, sessMgr)

	event, err := svc.handleTakeCover("u_cover_missing")
	require.Error(t, err)
	assert.Nil(t, event)
}

// TestCoverTierRank_Ordering verifies that coverTierRank returns values in the expected order.
func TestCoverTierRank_Ordering(t *testing.T) {
	assert.Greater(t, coverTierRank(combat.CoverTierGreater), coverTierRank(combat.CoverTierStandard))
	assert.Greater(t, coverTierRank(combat.CoverTierStandard), coverTierRank(combat.CoverTierLesser))
	assert.Greater(t, coverTierRank(combat.CoverTierLesser), coverTierRank(combat.CoverTierNone))
	assert.Equal(t, 0, coverTierRank(""))
}

// TestHandleTakeCover_InCombat_SpendAP verifies that 1 AP is spent and cover fields are set in combat.
func TestHandleTakeCover_InCombat_SpendAP(t *testing.T) {
	worldMgr, sessMgr := testWorldWithCoverRoom(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeCoverConditionRegistry()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)

	const roomID = "room_cover"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-cover-ap", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_cover_ap", Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 10, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack("u_cover_ap", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Record remaining AP before taking cover.
	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	var playerCombatant *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.ID == "u_cover_ap" {
			playerCombatant = c
			break
		}
	}
	require.NotNil(t, playerCombatant, "player combatant must exist")

	apBefore := cbt.ActionQueues["u_cover_ap"].RemainingPoints()

	event, err := svc.handleTakeCover("u_cover_ap")
	require.NoError(t, err)
	require.NotNil(t, event)
	msgEvt := event.GetMessage()
	require.NotNil(t, msgEvt, "expected message event, got: %v", event)
	assert.Contains(t, msgEvt.Content, "standard cover")

	// AP must be reduced by 1.
	assert.Equal(t, apBefore-1, cbt.ActionQueues["u_cover_ap"].RemainingPoints())
	// Cover fields must be set.
	assert.Equal(t, combat.CoverTierStandard, playerCombatant.CoverTier)
	assert.Equal(t, "barrel_01", playerCombatant.CoverEquipmentID)
	// Condition must be applied.
	assert.True(t, sess.Conditions.Has("standard_cover"))
}

// newCombatSvcWithCover builds a GameServiceServer with a real CombatHandler for stride/step/tumble tests.
func newCombatSvcWithCover(t *testing.T, worldMgr *world.Manager, sessMgr *session.Manager) (*GameServiceServer, *CombatHandler) {
	t.Helper()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeCoverConditionRegistry()
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
	)
	return svc, combatHandler
}

// setupPlayerInCombatWithCover adds an NPC to the room, starts combat, and applies standard_cover to the player.
// Returns the player session and the player combatant.
func setupPlayerInCombatWithCover(
	t *testing.T,
	sessMgr *session.Manager,
	combatHandler *CombatHandler,
	condReg *condition.Registry,
	uid, roomID string,
) (*session.PlayerSession, *combat.Combatant) {
	t.Helper()

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: "Fighter", CharName: "Fighter",
		RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat

	_, err = combatHandler.Attack(uid, "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer(roomID)

	// Apply standard_cover condition to the player.
	def, ok := condReg.Get("standard_cover")
	require.True(t, ok)
	require.NoError(t, sess.Conditions.Apply(uid, def, 1, -1))

	// Set CoverTier on combatant.
	cbt, ok := combatHandler.GetCombatForRoom(roomID)
	require.True(t, ok)
	var playerCombatant *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			playerCombatant = c
			break
		}
	}
	require.NotNil(t, playerCombatant, "player combatant must exist")
	playerCombatant.CoverTier = combat.CoverTierStandard
	playerCombatant.CoverEquipmentID = "barrel_01"

	return sess, playerCombatant
}

// TestStrideRemovesCoverCondition verifies that Stride removes cover for any cover tier.
func TestStrideRemovesCoverCondition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{"lesser", "standard", "greater"}).Draw(rt, "tier")
		condID := tier + "_cover"

		worldMgr, sessMgr := testWorldWithCoverRoom(t)
		condReg := makeCoverConditionRegistry()
		npcMgr := npc.NewManager()
		_, err := npcMgr.Spawn(&npc.Template{
			ID: "guard-stride-cover", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
		}, "room_cover")
		require.NoError(t, err)

		logger := zaptest.NewLogger(t)
		roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
		combatHandler := NewCombatHandler(
			combat.NewEngine(), npcMgr, sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
		)
		svc := newTestGameServiceServer(
			worldMgr, sessMgr,
			command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
			NewChatHandler(sessMgr),
			logger,
			nil, nil, nil, npcMgr, combatHandler, nil,
			nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
			nil, nil,
			nil, nil,
		)

		uid := "u_stride_cover_" + tier
		sess, playerCombatant := setupPlayerInCombatWithCover(t, sessMgr, combatHandler, condReg, uid, "room_cover")
		// Override the cover tier applied by the helper to match the drawn tier.
		sess.Conditions.Remove(uid, "standard_cover")
		def, ok := condReg.Get(condID)
		require.True(t, ok)
		require.NoError(t, sess.Conditions.Apply(uid, def, 1, -1))
		playerCombatant.CoverTier = tier

		require.True(t, sess.Conditions.Has(condID), "precondition: %s must be active before stride", condID)
		require.Equal(t, tier, playerCombatant.CoverTier, "precondition: combatant cover tier must be set")

		event, err := svc.handleStride(uid, &gamev1.StrideRequest{Direction: "away"})
		require.NoError(t, err)
		require.NotNil(t, event)

		if sess.Conditions.Has(condID) {
			rt.Errorf("cover condition %s must be removed after stride", condID)
		}
		if playerCombatant.CoverTier != "" {
			rt.Errorf("combatant CoverTier must be cleared after stride, got %q", playerCombatant.CoverTier)
		}
	})
}

// TestStepRemovesCoverCondition verifies that Step removes cover for any cover tier.
func TestStepRemovesCoverCondition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{"lesser", "standard", "greater"}).Draw(rt, "tier")
		condID := tier + "_cover"

		worldMgr, sessMgr := testWorldWithCoverRoom(t)
		condReg := makeCoverConditionRegistry()
		npcMgr := npc.NewManager()
		_, err := npcMgr.Spawn(&npc.Template{
			ID: "guard-step-cover", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
		}, "room_cover")
		require.NoError(t, err)

		logger := zaptest.NewLogger(t)
		roller := dice.NewLoggedRoller(dice.NewCryptoSource(), logger)
		combatHandler := NewCombatHandler(
			combat.NewEngine(), npcMgr, sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
		)
		svc := newTestGameServiceServer(
			worldMgr, sessMgr,
			command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
			NewChatHandler(sessMgr),
			logger,
			nil, nil, nil, npcMgr, combatHandler, nil,
			nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
			nil, nil,
			nil, nil,
		)

		uid := "u_step_cover_" + tier
		sess, playerCombatant := setupPlayerInCombatWithCover(t, sessMgr, combatHandler, condReg, uid, "room_cover")
		// Override the cover tier applied by the helper to match the drawn tier.
		sess.Conditions.Remove(uid, "standard_cover")
		def, ok := condReg.Get(condID)
		require.True(t, ok)
		require.NoError(t, sess.Conditions.Apply(uid, def, 1, -1))
		playerCombatant.CoverTier = tier

		require.True(t, sess.Conditions.Has(condID), "precondition: %s must be active before step", condID)
		require.Equal(t, tier, playerCombatant.CoverTier, "precondition: combatant cover tier must be set")

		event, err := svc.handleStep(uid, &gamev1.StepRequest{Direction: "away"})
		require.NoError(t, err)
		require.NotNil(t, event)

		if sess.Conditions.Has(condID) {
			rt.Errorf("cover condition %s must be removed after step", condID)
		}
		if playerCombatant.CoverTier != "" {
			rt.Errorf("combatant CoverTier must be cleared after step, got %q", playerCombatant.CoverTier)
		}
	})
}

// TestTumbleSuccessRemovesCoverCondition verifies that a successful Tumble removes cover for any tier.
func TestTumbleSuccessRemovesCoverCondition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{"lesser", "standard", "greater"}).Draw(rt, "tier")
		condID := tier + "_cover"

		worldMgr, sessMgr := testWorldWithCoverRoom(t)
		condReg := makeCoverConditionRegistry()
		npcMgr := npc.NewManager()
		_, err := npcMgr.Spawn(&npc.Template{
			ID: "guard-tumble-cover", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
		}, "room_cover")
		require.NoError(t, err)

		logger := zaptest.NewLogger(t)
		// Use a deterministic roller that always returns 19 (d20 = 20) to guarantee Tumble success.
		roller := dice.NewLoggedRoller(&fixedDiceSource{val: 19}, logger)
		combatHandler := NewCombatHandler(
			combat.NewEngine(), npcMgr, sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, nil, nil,
		)
		svc := newTestGameServiceServer(
			worldMgr, sessMgr,
			command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
			NewChatHandler(sessMgr),
			logger,
			nil, nil, nil, npcMgr, combatHandler, nil,
			nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
			nil, nil,
			nil, nil,
		)
		// Wire the fixed-source roller into the service.
		svc.dice = roller

		uid := "u_tumble_cover_" + tier
		sess, playerCombatant := setupPlayerInCombatWithCover(t, sessMgr, combatHandler, condReg, uid, "room_cover")
		// Override the cover tier applied by the helper to match the drawn tier.
		sess.Conditions.Remove(uid, "standard_cover")
		def, ok := condReg.Get(condID)
		require.True(t, ok)
		require.NoError(t, sess.Conditions.Apply(uid, def, 1, -1))
		playerCombatant.CoverTier = tier

		require.True(t, sess.Conditions.Has(condID), "precondition: %s must be active before tumble", condID)
		require.Equal(t, tier, playerCombatant.CoverTier, "precondition: combatant cover tier must be set")

		event, err := svc.handleTumble(uid, &gamev1.TumbleRequest{Target: "Guard"})
		require.NoError(t, err)
		require.NotNil(t, event)
		msgEvt := event.GetMessage()
		require.NotNil(t, msgEvt)
		require.Contains(t, msgEvt.Content, "success", "expected tumble success narrative")

		if sess.Conditions.Has(condID) {
			rt.Errorf("cover condition %s must be removed after successful tumble", condID)
		}
		if playerCombatant.CoverTier != "" {
			rt.Errorf("combatant CoverTier must be cleared after successful tumble, got %q", playerCombatant.CoverTier)
		}
	})
}
