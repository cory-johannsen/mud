package gameserver

import (
	"context"
	"fmt"
	"time"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

// startRovingNPCs initialises the RovingManager, registers all instances
// from loaded templates with Roving != nil, and starts the background goroutine.
//
// Precondition: s.npcMgr and s.world must be non-nil.
// Postcondition: s.rovingMgr is non-nil; background goroutine runs until stopRovingNPCs is called.
func (s *GameServiceServer) startRovingNPCs(ctx context.Context) {
	rovingCtx, cancel := context.WithCancel(ctx)
	s.rovingMgr = npc.NewRovingManager(s.npcMgr, s.world, s.onRovingMove)

	// Register all currently-loaded roving NPC instances.
	for _, inst := range s.npcMgr.AllInstances() {
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil || tmpl.Roving == nil {
			continue
		}
		s.rovingMgr.Register(inst, tmpl)
	}

	go s.rovingMgr.Start(rovingCtx)
	s.stopRovingNPCs = cancel
}

// StopRovingNPCs stops the roving goroutine. Safe to call if never started.
//
// Postcondition: background goroutine is stopped.
func (s *GameServiceServer) StopRovingNPCs() {
	if s.stopRovingNPCs != nil {
		s.stopRovingNPCs()
		s.stopRovingNPCs = nil
	}
}

// StartRovingNPCs is the public lifecycle method called alongside StartZoneTicks.
//
// Precondition: s.npcMgr and s.world must be non-nil; ctx must not be nil.
// Postcondition: roving NPC background goroutine is running.
func (s *GameServiceServer) StartRovingNPCs(ctx context.Context) {
	if s.npcMgr == nil {
		return
	}
	s.startRovingNPCs(ctx)
}

// onRovingMove is the RovingMoveFunc callback. It:
//  1. Finds players in fromRoomID and sends "<name> leaves to the <direction>." MessageEvent
//  2. Finds players in toRoomID and sends "<name> arrives from the <direction>." MessageEvent
//
// Precondition: instID, fromRoomID, toRoomID must all be non-empty.
func (s *GameServiceServer) onRovingMove(instID, fromRoomID, toRoomID string) {
	inst, ok := s.npcMgr.Get(instID)
	if !ok {
		return
	}
	name := inst.Name()

	// Determine direction by inspecting the exits of fromRoom.
	leaveDir := ""
	arriveDir := ""
	if fromRoom, ok := s.world.GetRoom(fromRoomID); ok {
		for _, exit := range fromRoom.Exits {
			if exit.TargetRoom == toRoomID {
				leaveDir = string(exit.Direction)
				arriveDir = string(exit.Direction.Opposite())
				break
			}
		}
	}

	// Notify players in fromRoom.
	var leaveMsg string
	if leaveDir != "" {
		leaveMsg = fmt.Sprintf("%s leaves to the %s.", name, leaveDir)
	} else {
		leaveMsg = fmt.Sprintf("%s leaves.", name)
	}
	s.broadcastMessage(fromRoomID, "", &gamev1.MessageEvent{Content: leaveMsg})

	// Notify players in toRoom.
	var arriveMsg string
	if arriveDir != "" {
		arriveMsg = fmt.Sprintf("%s arrives from the %s.", name, arriveDir)
	} else {
		arriveMsg = fmt.Sprintf("%s appears in the room.", name)
	}
	s.broadcastMessage(toRoomID, "", &gamev1.MessageEvent{Content: arriveMsg})
}

// pauseRovingOnCombat pauses the roving NPC's movement for 10 minutes.
// Called via the CombatHandler.SetOnNPCDamageTaken callback.
//
// Precondition: instID must be non-empty.
// Postcondition: NPC movement is paused for 10 minutes if it is a tracked roving NPC.
func (s *GameServiceServer) pauseRovingOnCombat(instID string) {
	if s.rovingMgr == nil {
		return
	}
	s.rovingMgr.PauseFor(instID, 10*time.Minute)
}
