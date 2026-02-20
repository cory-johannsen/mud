package gameserver

import (
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// ChatHandler handles say, emote, and who commands.
type ChatHandler struct {
	sessions *session.Manager
}

// NewChatHandler creates a ChatHandler with the given dependencies.
//
// Precondition: sessMgr must be non-nil.
func NewChatHandler(sessMgr *session.Manager) *ChatHandler {
	return &ChatHandler{
		sessions: sessMgr,
	}
}

// Say broadcasts a chat message to all players in the sender's room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a MessageEvent to broadcast, or an error.
func (h *ChatHandler) Say(uid string, message string) (*gamev1.MessageEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	return &gamev1.MessageEvent{
		Sender:  sess.CharName,
		Content: message,
		Type:    gamev1.MessageType_MESSAGE_TYPE_SAY,
	}, nil
}

// Emote broadcasts an emote action to all players in the sender's room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a MessageEvent to broadcast, or an error.
func (h *ChatHandler) Emote(uid string, action string) (*gamev1.MessageEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	return &gamev1.MessageEvent{
		Sender:  sess.CharName,
		Content: action,
		Type:    gamev1.MessageType_MESSAGE_TYPE_EMOTE,
	}, nil
}

// Who returns the list of players in the sender's room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a PlayerList or an error.
func (h *ChatHandler) Who(uid string) (*gamev1.PlayerList, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	players := h.sessions.PlayersInRoom(sess.RoomID)
	return &gamev1.PlayerList{
		RoomTitle: sess.RoomID,
		Players:   players,
	}, nil
}
