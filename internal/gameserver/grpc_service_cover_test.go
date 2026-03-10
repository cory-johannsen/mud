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
						CoverTier: combat.CoverTierStandard,
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
	return NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)
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
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil,
	)

	const roomID = "room_cover"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-cover-ap", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Perception: 5,
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
