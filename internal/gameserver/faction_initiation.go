package gameserver

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// checkFactionInitiation examines the room for hostile-faction NPCs and initiates
// NPC-vs-NPC combat if any are found. Only fires in all_out_war rooms (REQ-CCF-3a,3b).
//
// Precondition: arrivalInst must be non-nil; roomID must be non-empty.
// Postcondition: initiate is called at most once with the lowest-HP hostile NPC found.
func checkFactionInitiation(
	arrivalInst *npc.Instance,
	roomID string,
	npcsInRoom func(roomID string) []*npc.Instance,
	getRoom func(roomID string) *world.Room,
	getHostiles func(factionID string) []string,
	initiate func(attacker, target *npc.Instance, room *world.Room),
) {
	if arrivalInst.FactionID == "" {
		return
	}
	room := getRoom(roomID)
	if room == nil || room.DangerLevel != string(danger.AllOutWar) {
		return
	}
	hostileSet := make(map[string]bool)
	for _, hf := range getHostiles(arrivalInst.FactionID) {
		hostileSet[hf] = true
	}
	if len(hostileSet) == 0 {
		return
	}

	existing := npcsInRoom(roomID)
	var target *npc.Instance
	for _, inst := range existing {
		if inst.ID == arrivalInst.ID || !hostileSet[inst.FactionID] {
			continue
		}
		if target == nil || inst.CurrentHP < target.CurrentHP {
			target = inst
		}
	}
	if target == nil {
		return
	}
	initiate(arrivalInst, target, room)
}

// initiateNPCFactionCombat sends a console message to players in the room announcing
// the NPC-vs-NPC faction engagement (REQ-CCF-3).
//
// Precondition: attacker and target must be non-nil; roomID must be non-empty.
// Postcondition: a message is broadcast to all players in roomID.
func (s *GameServiceServer) initiateNPCFactionCombat(attacker, target *npc.Instance, roomID string) {
	msg := fmt.Sprintf("A %s lunges at a %s!", attacker.Name(), target.Name())
	s.broadcastMessage(roomID, "", &gamev1.MessageEvent{Content: msg})
}
