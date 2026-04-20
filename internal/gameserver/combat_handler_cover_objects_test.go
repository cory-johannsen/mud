package gameserver_test

import (
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// makeCoverObjectsHandler builds a CombatHandler with a real world.Manager
// pre-populated with a room that contains the given equipment.
//
// Precondition: t must be non-nil; roomID must be non-empty.
// Postcondition: Returns handler, npcMgr, sessMgr, and a function that returns
// all captured RoundStartEvents under a mutex.
func makeCoverObjectsHandler(
	t *testing.T,
	roomID string,
	equipment []world.RoomEquipmentConfig,
) (*gameserver.CombatHandler, *npc.Manager, *session.Manager, func() []*gamev1.RoundStartEvent) {
	t.Helper()

	room := &world.Room{
		ID:        roomID,
		ZoneID:    "cover-zone",
		Equipment: equipment,
	}
	zone := &world.Zone{
		ID:        "cover-zone",
		StartRoom: roomID,
		Rooms:     map[string]*world.Room{roomID: room},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})

	broadcastFn, _ := makeBroadcastCapture()
	h := gameserver.NewCombatHandler(
		engine, npcMgr, sessMgr, roller, broadcastFn,
		200*time.Millisecond,
		reg,
		wm, nil, nil, nil, nil, nil, nil, nil,
	)

	var mu sync.Mutex
	var captured []*gamev1.RoundStartEvent

	h.SetRoundStartBroadcastFn(func(_ string, evt *gamev1.RoundStartEvent) {
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, evt)
	})

	get := func() []*gamev1.RoundStartEvent {
		mu.Lock()
		defer mu.Unlock()
		result := make([]*gamev1.RoundStartEvent, len(captured))
		copy(result, captured)
		return result
	}

	return h, npcMgr, sessMgr, get
}

// waitForRoundStartEvent polls until at least one RoundStartEvent is captured,
// or the timeout elapses.
//
// Precondition: getEvents must be non-nil.
// Postcondition: Returns captured events; may be empty if timeout elapsed.
func waitForRoundStartEvent(t *testing.T, getEvents func() []*gamev1.RoundStartEvent, timeout time.Duration) []*gamev1.RoundStartEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		evts := getEvents()
		if len(evts) > 0 {
			return evts
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// TestCoverObjects_NoCoverEquipment verifies that a room with no cover equipment
// yields zero CoverObjects in the RoundStartEvent.
//
// REQ-CO-1: RoundStartEvent.CoverObjects MUST contain exactly one entry per
// room equipment item with a non-empty CoverTier.
func TestCoverObjects_NoCoverEquipment(t *testing.T) {
	const roomID = "cover-room-none"
	// Equipment with no CoverTier set.
	equipment := []world.RoomEquipmentConfig{
		{ItemID: "table", Description: "A table", MaxCount: 1, Immovable: true},
	}
	h, npcMgr, sessMgr, getRoundStartEvents := makeCoverObjectsHandler(t, roomID, equipment)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-cover-none", Name: "GuardNoCover", Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "player-cover-none",
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 1000,
		MaxHP:     1000,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	equipTestPistol(t, h, "player-cover-none")

	_, err = h.Attack("player-cover-none", "GuardNoCover")
	require.NoError(t, err)

	evts := waitForRoundStartEvent(t, getRoundStartEvents, 2*time.Second)
	require.NotEmpty(t, evts, "expected at least one RoundStartEvent to be broadcast")

	for _, evt := range evts {
		assert.Empty(t, evt.CoverObjects,
			"RoundStartEvent.CoverObjects must be empty when room has no cover equipment")
	}
}

// TestCoverObjects_SingleStandardCover verifies that a room with one standard
// cover item yields exactly one CoverObjects entry with the correct tier and
// valid grid coordinates.
//
// REQ-CO-2: Each CoverObjectPosition MUST have a non-empty ItemId, a valid
// CoverTier, and X/Y within the 20×20 grid bounds [0, 19].
func TestCoverObjects_SingleStandardCover(t *testing.T) {
	const roomID = "cover-room-single"
	equipment := []world.RoomEquipmentConfig{
		{ItemID: "barrier-01", Description: "A concrete barrier", MaxCount: 1, Immovable: true, CoverTier: "standard"},
	}
	h, npcMgr, sessMgr, getRoundStartEvents := makeCoverObjectsHandler(t, roomID, equipment)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-cover-single", Name: "GuardSingle", Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "player-cover-single",
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 1000,
		MaxHP:     1000,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	equipTestPistol(t, h, "player-cover-single")

	_, err = h.Attack("player-cover-single", "GuardSingle")
	require.NoError(t, err)

	evts := waitForRoundStartEvent(t, getRoundStartEvents, 2*time.Second)
	require.NotEmpty(t, evts, "expected at least one RoundStartEvent to be broadcast")

	evt := evts[0]
	require.Len(t, evt.CoverObjects, 1,
		"RoundStartEvent.CoverObjects must contain exactly one entry for one cover item")

	co := evt.CoverObjects[0]
	assert.Equal(t, "barrier-01", co.ItemId, "CoverObjectPosition.ItemId must match the equipment item ID")
	assert.Equal(t, "standard", co.CoverTier, "CoverObjectPosition.CoverTier must equal 'standard'")
	assert.GreaterOrEqual(t, co.X, int32(0), "CoverObjectPosition.X must be >= 0")
	assert.Less(t, co.X, int32(20), "CoverObjectPosition.X must be < 20")
	assert.GreaterOrEqual(t, co.Y, int32(0), "CoverObjectPosition.Y must be >= 0")
	assert.Less(t, co.Y, int32(20), "CoverObjectPosition.Y must be < 20")
}

// TestProperty_CoverObjects_CountMatchesEquipment is a property-based test
// verifying that for any N cover items in the room (1..5), the RoundStartEvent
// contains exactly N CoverObjects entries, each with a non-empty ItemId, valid
// CoverTier, and X/Y within 20×20 grid bounds.
//
// REQ-CO-1: RoundStartEvent.CoverObjects MUST contain exactly one entry per
// room equipment item with a non-empty CoverTier.
// REQ-CO-2: Each CoverObjectPosition MUST have a non-empty ItemId, a valid
// CoverTier, and X/Y within the 20×20 grid bounds [0, 19].
func TestProperty_CoverObjects_CountMatchesEquipment(t *testing.T) {
	validTiers := []string{"lesser", "standard", "greater"}

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n_cover_items")

		equipment := make([]world.RoomEquipmentConfig, n)
		for i := range equipment {
			tier := validTiers[rapid.IntRange(0, 2).Draw(rt, "tier")]
			equipment[i] = world.RoomEquipmentConfig{
				ItemID:    rapid.StringMatching(`[a-z][a-z0-9\-]{2,8}`).Draw(rt, "item_id"),
				Immovable: true,
				CoverTier: tier,
			}
		}

		roomID := rapid.StringMatching(`room-prop-[a-z]{4}`).Draw(rt, "room_id")

		h, npcMgr, sessMgr, getRoundStartEvents := makeCoverObjectsHandler(t, roomID, equipment)

		npcID := rapid.StringMatching(`guard-[a-z]{4}`).Draw(rt, "npc_id")
		npcName := rapid.StringMatching(`Guard[A-Z][a-z]{3}`).Draw(rt, "npc_name")
		_, err := npcMgr.Spawn(&npc.Template{
			ID: npcID, Name: npcName, Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
		}, roomID)
		if err != nil {
			rt.Skip("npc spawn failed:", err)
		}

		playerUID := rapid.StringMatching(`player-[a-z]{4}`).Draw(rt, "player_uid")
		_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:       playerUID,
			Username:  "Hero",
			CharName:  "Hero",
			RoomID:    roomID,
			CurrentHP: 1000,
			MaxHP:     1000,
			Abilities: character.AbilityScores{},
			Role:      "player",
		})
		if err != nil {
			rt.Skip("session add player failed:", err)
		}

		equipTestPistol(t, h, playerUID)

		_, err = h.Attack(playerUID, npcName)
		if err != nil {
			rt.Skip("attack failed:", err)
		}

		evts := waitForRoundStartEvent(t, getRoundStartEvents, 2*time.Second)
		if len(evts) == 0 {
			rt.Fatal("no RoundStartEvent captured within 2 seconds")
		}

		evt := evts[0]

		// REQ-CO-1: Count must exactly match number of cover equipment items.
		if len(evt.CoverObjects) != n {
			rt.Fatalf("expected %d CoverObjects, got %d", n, len(evt.CoverObjects))
		}

		// REQ-CO-2: Each entry must have valid fields and grid bounds.
		for i, co := range evt.CoverObjects {
			if co.ItemId == "" {
				rt.Fatalf("CoverObjects[%d].ItemId must be non-empty", i)
			}
			if co.CoverTier == "" {
				rt.Fatalf("CoverObjects[%d].CoverTier must be non-empty", i)
			}
			if co.X < 0 || co.X >= 20 {
				rt.Fatalf("CoverObjects[%d].X = %d out of [0, 19]", i, co.X)
			}
			if co.Y < 0 || co.Y >= 20 {
				rt.Fatalf("CoverObjects[%d].Y = %d out of [0, 19]", i, co.Y)
			}
		}
	})
}
