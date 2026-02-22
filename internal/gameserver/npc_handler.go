package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// NPCHandler handles NPC-related commands.
type NPCHandler struct {
	npcMgr   *npc.Manager
	sessions *session.Manager
}

// NewNPCHandler creates an NPCHandler.
//
// Precondition: npcMgr and sessions must be non-nil.
func NewNPCHandler(npcMgr *npc.Manager, sessions *session.Manager) *NPCHandler {
	return &NPCHandler{npcMgr: npcMgr, sessions: sessions}
}

// Examine looks up an NPC by name prefix in the player's room and returns its detail view.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns NpcView or an error if the target is not found.
func (h *NPCHandler) Examine(uid, target string) (*gamev1.NpcView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}

	return &gamev1.NpcView{
		InstanceId:        inst.ID,
		Name:              inst.Name,
		Description:       inst.Description,
		HealthDescription: inst.HealthDescription(),
		Level:             int32(inst.Level),
	}, nil
}
