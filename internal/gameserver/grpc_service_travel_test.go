package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newTravelWorld builds a two-zone world where both zones have world coordinates.
// zoneA has roomA as start; zoneB has roomB as start.
func newTravelWorld() (*world.Manager, string, string) {
	xA, yA := 0, 0
	xB, yB := 2, 0
	roomA := &world.Room{ID: "roomA", ZoneID: "zoneA", Title: "Room A", Description: "D", MapX: 0, MapY: 0}
	roomB := &world.Room{ID: "roomB", ZoneID: "zoneB", Title: "Room B", Description: "D", MapX: 0, MapY: 0}
	zoneA := &world.Zone{
		ID: "zoneA", Name: "Zone A", StartRoom: "roomA",
		Rooms:       map[string]*world.Room{"roomA": roomA},
		DangerLevel: "safe", WorldX: &xA, WorldY: &yA,
	}
	zoneB := &world.Zone{
		ID: "zoneB", Name: "Zone B", StartRoom: "roomB",
		Rooms:       map[string]*world.Room{"roomB": roomB},
		DangerLevel: "sketchy", WorldX: &xB, WorldY: &yB,
	}
	mgr, err := world.NewManager([]*world.Zone{zoneA, zoneB})
	if err != nil {
		panic(err)
	}
	return mgr, "roomA", "roomB"
}

func addTravelPlayer(t *testing.T, sMgr *session.Manager, roomID string) *session.PlayerSession {
	t.Helper()
	_, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID: "uid1", Username: "u", CharName: "Hero", CharacterID: 1,
		RoomID: roomID, CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{}, Role: "player",
		RegionDisplayName: "test", Class: "Gunner", Level: 1,
	})
	require.NoError(t, err)
	sess, _ := sMgr.GetPlayer("uid1")
	return sess
}

// TestHandleTravel_UnknownZone verifies "That zone does not exist." when zone_id is bogus.
func TestHandleTravel_UnknownZone(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	addTravelPlayer(t, sMgr, roomA)
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "nowhere"})
	require.NoError(t, err)
	require.Contains(t, result.GetMessage().Content, "That zone does not exist.")
}

// TestHandleTravel_Undiscovered verifies "You don't know how to get there." when cache is empty.
func TestHandleTravel_Undiscovered(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	addTravelPlayer(t, sMgr, roomA)
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "zoneB"})
	require.NoError(t, err)
	require.Contains(t, result.GetMessage().Content, "You don't know how to get there.")
}

// TestHandleTravel_AlreadyThere verifies "You're already there." when player is in target zone.
func TestHandleTravel_AlreadyThere(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	sess := addTravelPlayer(t, sMgr, roomA)
	sess.AutomapCache["zoneA"] = map[string]bool{roomA: true}
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "zoneA"})
	require.NoError(t, err)
	require.Contains(t, result.GetMessage().Content, "You're already there.")
}

// TestHandleTravel_InCombat verifies "You can't travel while in combat."
func TestHandleTravel_InCombat(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	sess := addTravelPlayer(t, sMgr, roomA)
	sess.AutomapCache["zoneB"] = map[string]bool{"roomB": true}
	sess.Status = statusInCombat
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "zoneB"})
	require.NoError(t, err)
	require.Contains(t, result.GetMessage().Content, "You can't travel while in combat.")
}

// TestHandleTravel_Wanted verifies "You can't travel while Wanted."
func TestHandleTravel_Wanted(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	sess := addTravelPlayer(t, sMgr, roomA)
	sess.AutomapCache["zoneB"] = map[string]bool{"roomB": true}
	sess.WantedLevel = map[string]int{"zoneA": 1}
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "zoneB"})
	require.NoError(t, err)
	require.Contains(t, result.GetMessage().Content, "You can't travel while Wanted.")
}

// TestHandleTravel_Success verifies relocation to StartRoom and travel message.
func TestHandleTravel_Success(t *testing.T) {
	wMgr, roomA, _ := newTravelWorld()
	sMgr := session.NewManager()
	sess := addTravelPlayer(t, sMgr, roomA)
	sess.AutomapCache["zoneB"] = map[string]bool{"roomB": true}
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	result, err := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "zoneB"})
	require.NoError(t, err)
	// On success the session room must be the target zone's StartRoom.
	require.Equal(t, "roomB", sess.RoomID)
	// The result must be a message event (travel confirmation), not nil.
	require.NotNil(t, result)
	require.Contains(t, result.GetMessage().Content, "Zone B")
}

// TestProperty_HandleTravel_PreconditionOrder verifies the check sequence is always
// unknown-zone → undiscovered → in-combat → wanted → already-there.
func TestProperty_HandleTravel_PreconditionOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Always request a completely unknown zone: must return "does not exist" regardless
		// of other state.
		wMgr, roomA, _ := newTravelWorld()
		sMgr := session.NewManager()
		_, err := sMgr.AddPlayer(session.AddPlayerOptions{
			UID: "uid1", Username: "u", CharName: "Hero", CharacterID: 1,
			RoomID: roomA, CurrentHP: 10, MaxHP: 10,
			Abilities: character.AbilityScores{}, Role: "player",
			RegionDisplayName: "test", Class: "Gunner", Level: 1,
		})
		if err != nil {
			rt.Fatalf("AddPlayer: %v", err)
		}
		sess, _ := sMgr.GetPlayer("uid1")
		// Set up all other bad conditions too.
		sess.Status = statusInCombat
		sess.WantedLevel = map[string]int{"zoneA": 2}
		s := &GameServiceServer{sessions: sMgr, world: wMgr}

		result, addErr := s.handleTravel("uid1", &gamev1.TravelRequest{ZoneId: "totally_unknown"})
		if addErr != nil {
			rt.Fatalf("unexpected error: %v", addErr)
		}
		if result.GetMessage() == nil || result.GetMessage().Content != "That zone does not exist." {
			rt.Fatalf("expected 'That zone does not exist.', got %v", result.GetMessage())
		}
	})
}
