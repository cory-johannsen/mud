package gameserver

import (
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// WorldHandler handles movement, look, and exit commands.
type WorldHandler struct {
	world    *world.Manager
	sessions *session.Manager
}

// NewWorldHandler creates a WorldHandler with the given dependencies.
//
// Precondition: worldMgr and sessMgr must be non-nil.
func NewWorldHandler(worldMgr *world.Manager, sessMgr *session.Manager) *WorldHandler {
	return &WorldHandler{
		world:    worldMgr,
		sessions: sessMgr,
	}
}

// Move moves the player in the given direction and returns the new room view.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the new RoomView or an error if movement fails.
func (h *WorldHandler) Move(uid string, dir world.Direction) (*gamev1.RoomView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	dest, err := h.world.Navigate(sess.RoomID, dir)
	if err != nil {
		return nil, err
	}

	oldRoomID, err := h.sessions.MovePlayer(uid, dest.ID)
	if err != nil {
		return nil, fmt.Errorf("moving player: %w", err)
	}

	_ = oldRoomID // Used by caller for broadcasting departure/arrival

	return h.buildRoomView(uid, dest), nil
}

// MoveResult holds the result of a Move operation including the old room
// for broadcasting departure events.
type MoveResult struct {
	OldRoomID string
	View      *gamev1.RoomView
}

// MoveWithContext moves the player and returns both old room and new room view.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns MoveResult or an error if movement fails.
func (h *WorldHandler) MoveWithContext(uid string, dir world.Direction) (*MoveResult, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	dest, err := h.world.Navigate(sess.RoomID, dir)
	if err != nil {
		return nil, err
	}

	oldRoomID, err := h.sessions.MovePlayer(uid, dest.ID)
	if err != nil {
		return nil, fmt.Errorf("moving player: %w", err)
	}

	return &MoveResult{
		OldRoomID: oldRoomID,
		View:      h.buildRoomView(uid, dest),
	}, nil
}

// Look returns the current room view for the player.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the RoomView or an error if the player/room is not found.
func (h *WorldHandler) Look(uid string) (*gamev1.RoomView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	room, ok := h.world.GetRoom(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("room %q not found", sess.RoomID)
	}

	return h.buildRoomView(uid, room), nil
}

// Exits returns the list of exits from the player's current room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the ExitList or an error.
func (h *WorldHandler) Exits(uid string) (*gamev1.ExitList, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	room, ok := h.world.GetRoom(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("room %q not found", sess.RoomID)
	}

	exitInfos := make([]*gamev1.ExitInfo, 0, len(room.Exits))
	for _, e := range room.Exits {
		exitInfos = append(exitInfos, &gamev1.ExitInfo{
			Direction:    string(e.Direction),
			TargetRoomId: e.TargetRoom,
			Locked:       e.Locked,
			Hidden:       e.Hidden,
		})
	}

	return &gamev1.ExitList{Exits: exitInfos}, nil
}

// buildRoomView constructs a RoomView proto from a Room, excluding the player themselves
// from the players list.
func (h *WorldHandler) buildRoomView(uid string, room *world.Room) *gamev1.RoomView {
	players := h.sessions.PlayersInRoom(room.ID)
	// Get the current player's char name to filter from list
	sess, _ := h.sessions.GetPlayer(uid)
	var otherPlayers []string
	for _, p := range players {
		if sess != nil && p == sess.CharName {
			continue
		}
		otherPlayers = append(otherPlayers, p)
	}

	visibleExits := room.VisibleExits()
	exitInfos := make([]*gamev1.ExitInfo, 0, len(visibleExits))
	for _, e := range visibleExits {
		exitInfos = append(exitInfos, &gamev1.ExitInfo{
			Direction:    string(e.Direction),
			TargetRoomId: e.TargetRoom,
			Locked:       e.Locked,
			Hidden:       e.Hidden,
		})
	}

	return &gamev1.RoomView{
		RoomId:      room.ID,
		Title:       room.Title,
		Description: room.Description,
		Exits:       exitInfos,
		Players:     otherPlayers,
	}
}
