package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
)

// newWorldWithSafeRoom creates a world.Manager with a room that has a "safe" tag in its Properties.
//
// Precondition: zoneID and roomID must be non-empty strings.
// Postcondition: The room at roomID has Properties["tags"] = "safe".
func newWorldWithSafeRoom(zoneID, roomID string) *world.Manager {
	r := &world.Room{
		ID:          roomID,
		ZoneID:      zoneID,
		Title:       "Safe Room",
		Description: "A safe room for downtime.",
		MapX:        0,
		MapY:        0,
		Properties:  map[string]string{"tags": "safe"},
	}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: roomID,
		Rooms:     map[string]*world.Room{roomID: r},
	}
	mgr, err := world.NewManager([]*world.Zone{z})
	if err != nil {
		panic("newWorldWithSafeRoom: " + err.Error())
	}
	return mgr
}

// addPlayerToSession adds a player with the given uid and roomID to a new session.Manager.
//
// Precondition: uid and roomID are non-empty.
// Postcondition: Returns a Manager where GetPlayer(uid) succeeds.
func addPlayerToSession(uid, roomID string) *session.Manager {
	sMgr := session.NewManager()
	_, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               uid,
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            roomID,
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the Northeast",
		Class:             "Gunner",
		Level:             1,
	})
	if err != nil {
		panic("addPlayerToSession: " + err.Error())
	}
	return sMgr
}

// TestHandleDowntime_RequiresSafeRoom verifies that starting an activity in a non-safe room
// returns an error message containing "Safe room".
//
// Precondition: Room has no "safe" tag; player sends DowntimeRequest{Subcommand:"earn"}.
// Postcondition: Response message contains "Safe room".
func TestHandleDowntime_RequiresSafeRoom(t *testing.T) {
	wMgr := newWorldWithRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "earn"})
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "Safe room")
}

// TestHandleDowntime_BlocksIfBusy verifies that starting an activity while one is already active
// returns an error message containing "busy".
//
// Precondition: sess.DowntimeBusy=true; player sends DowntimeRequest{Subcommand:"earn"}.
// Postcondition: Response message contains "busy".
func TestHandleDowntime_BlocksIfBusy(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")
	sess, ok := sMgr.GetPlayer("uid1")
	require.True(t, ok)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "earn_creds"

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "earn"})
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "active downtime activity")
}

// TestHandleDowntime_Cancel_NoBusy_Noop verifies that cancelling when not busy returns a
// message without panicking.
//
// Precondition: sess.DowntimeBusy=false; player sends DowntimeRequest{Subcommand:"cancel"}.
// Postcondition: Returns a non-nil ServerEvent with no error.
func TestHandleDowntime_Cancel_NoBusy_Noop(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "cancel"})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestHandleDowntime_Cancel_ClearsBusy verifies that cancelling an active activity clears
// the session busy state.
//
// Precondition: sess.DowntimeBusy=true, sess.DowntimeActivityID="earn_creds".
// Postcondition: sess.DowntimeBusy==false, sess.DowntimeActivityID=="".
func TestHandleDowntime_Cancel_ClearsBusy(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")
	sess, ok := sMgr.GetPlayer("uid1")
	require.True(t, ok)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "earn_creds"

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "cancel"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, sess.DowntimeBusy)
	require.Empty(t, sess.DowntimeActivityID)
}

// TestHandleDowntime_Start_SetsActivity verifies that starting an activity in a safe room
// sets the session busy state and activity ID.
//
// Precondition: Room has "safe" tag; not busy; player sends DowntimeRequest{Subcommand:"earn"}.
// Postcondition: sess.DowntimeBusy==true, sess.DowntimeActivityID=="earn_creds".
func TestHandleDowntime_Start_SetsActivity(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "earn"})
	require.NoError(t, err)
	require.NotNil(t, result)

	sess, ok := sMgr.GetPlayer("uid1")
	require.True(t, ok)
	require.True(t, sess.DowntimeBusy)
	require.Equal(t, "earn_creds", sess.DowntimeActivityID)
}

// TestHandleDowntime_Status_ShowsActivity verifies that the status subcommand (empty string)
// returns the current activity name when busy.
//
// Precondition: sess.DowntimeBusy=true, DowntimeActivityID="earn_creds", DowntimeCompletesAt is in the future.
// Postcondition: Response message contains "Earn Creds".
func TestHandleDowntime_Status_ShowsActivity(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")
	sess, ok := sMgr.GetPlayer("uid1")
	require.True(t, ok)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = "earn_creds"
	sess.DowntimeCompletesAt = time.Now().Add(5 * time.Minute)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: ""})
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "Earn Creds")
}

// TestHandleDowntime_List_ShowsActivities verifies that the list subcommand returns all activities.
//
// Precondition: Player sends DowntimeRequest{Subcommand:"list"}.
// Postcondition: Response message contains "earn" and "craft".
func TestHandleDowntime_List_ShowsActivities(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr := addPlayerToSession("uid1", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleDowntime("uid1", &gamev1.DowntimeRequest{Subcommand: "list"})
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "earn")
	require.Contains(t, msg.Content, "craft")
}
