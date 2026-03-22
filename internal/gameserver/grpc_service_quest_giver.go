package gameserver

import (
	"fmt"
	"math/rand"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findQuestGiverInRoom returns the first quest_giver NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findQuestGiverInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	if inst.NPCType != "quest_giver" {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	return inst, ""
}

// handleTalk responds to a talk command by returning a random line from the NPC's
// PlaceholderDialog.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleTalk(uid string, req *gamev1.TalkRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findQuestGiverInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.QuestGiver == nil {
		return messageEvent("That NPC has no dialog configured."), nil
	}
	dialog := tmpl.QuestGiver.PlaceholderDialog
	line := dialog[rand.Intn(len(dialog))]
	return messageEvent(fmt.Sprintf("%s says: %q", inst.Name(), line)), nil
}
