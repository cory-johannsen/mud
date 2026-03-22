package gameserver

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var hirelingRuntimeMu sync.RWMutex

// initHirelingRuntimeState initialises runtime state for a hireling instance if absent.
//
// Precondition: inst must be non-nil.
// Postcondition: hirelingRuntimeStates[inst.ID] is set iff inst.NPCType == "hireling".
func (s *GameServiceServer) initHirelingRuntimeState(inst *npc.Instance) {
	if inst.NPCType != "hireling" {
		return
	}
	hirelingRuntimeMu.Lock()
	defer hirelingRuntimeMu.Unlock()
	if _, ok := s.hirelingRuntimeStates[inst.ID]; !ok {
		s.hirelingRuntimeStates[inst.ID] = &npc.HirelingRuntimeState{}
	}
}

// hirelingStateFor returns the HirelingRuntimeState for instID, or nil if absent.
func (s *GameServiceServer) hirelingStateFor(instID string) *npc.HirelingRuntimeState {
	hirelingRuntimeMu.RLock()
	defer hirelingRuntimeMu.RUnlock()
	return s.hirelingRuntimeStates[instID]
}

// HirelingOwnerOf returns the UID of the player who has hired the given hireling instance,
// or empty string if not hired. Satisfies the CombatHandler.SetHirelingOwnerOf callback
// contract for REQ-NPC-8.
//
// Precondition: instID is non-empty.
// Postcondition: returns the owner UID or "".
func (s *GameServiceServer) HirelingOwnerOf(instID string) string {
	hirelingRuntimeMu.RLock()
	defer hirelingRuntimeMu.RUnlock()
	if state, ok := s.hirelingRuntimeStates[instID]; ok {
		return state.HiredByPlayerID
	}
	return ""
}

// findHiredHireling returns the Instance currently hired by uid, or nil if none.
func (s *GameServiceServer) findHiredHireling(uid string) *npc.Instance {
	hirelingRuntimeMu.RLock()
	defer hirelingRuntimeMu.RUnlock()
	for instID, state := range s.hirelingRuntimeStates {
		if state.HiredByPlayerID == uid {
			if inst := s.npcMgr.InstanceByID(instID); inst != nil {
				return inst
			}
		}
	}
	return nil
}

// handleHire processes a HireRequest: validates the hireling NPC exists in the
// player's current room, checks availability and sufficient credits, then
// deducts the daily cost and records the hire.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleHire(uid string, req *gamev1.HireRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst := s.npcMgr.FindInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(fmt.Sprintf("You don't see %q here.", req.GetNpcName())), nil
	}
	if inst.NPCType != "hireling" {
		return messageEvent(fmt.Sprintf("%s is not a hireling.", inst.Name())), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Hireling == nil {
		return messageEvent("This NPC has no hireling configuration."), nil
	}
	cfg := tmpl.Hireling
	if sess.Currency < cfg.DailyCost {
		return messageEvent(fmt.Sprintf("You need %d credits to hire %s (daily cost).", cfg.DailyCost, inst.Name())), nil
	}

	// Atomic check-and-set: initialise state if absent, then claim.
	s.initHirelingRuntimeState(inst)
	hirelingRuntimeMu.Lock()
	state := s.hirelingRuntimeStates[inst.ID]
	if state.HiredByPlayerID != "" {
		hirelingRuntimeMu.Unlock()
		return messageEvent(fmt.Sprintf("%s is already hired by someone else.", inst.Name())), nil
	}
	state.HiredByPlayerID = uid
	state.ZonesFollowed = 0
	hirelingRuntimeMu.Unlock()

	sess.Currency -= cfg.DailyCost
	return messageEvent(fmt.Sprintf("%s agrees to work with you for %d credits per day.", inst.Name(), cfg.DailyCost)), nil
}

// handleDismiss releases the hireling currently employed by uid.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleDismiss(uid string, req *gamev1.DismissRequest) (*gamev1.ServerEvent, error) {
	if _, ok := s.sessions.GetPlayer(uid); !ok {
		return messageEvent("player not found"), nil
	}
	inst := s.findHiredHireling(uid)
	if inst == nil {
		return messageEvent("You have no hireling to dismiss."), nil
	}
	hirelingRuntimeMu.Lock()
	if state, ok := s.hirelingRuntimeStates[inst.ID]; ok {
		state.HiredByPlayerID = ""
		state.ZonesFollowed = 0
	}
	hirelingRuntimeMu.Unlock()
	return messageEvent(fmt.Sprintf("You dismiss %s. They head back to their post.", inst.Name())), nil
}

// tickHirelingDailyCost deducts the daily cost from each hiring player.
// Hirelings whose employer cannot pay are automatically dismissed.
// Intended to be called once per in-game day.
//
// Precondition: s.hirelingRuntimeStates MUST NOT be nil.
// Postcondition: all hired hirelings with solvent employers have had DailyCost deducted;
// insolvent or orphaned hirelings are released.
func (s *GameServiceServer) tickHirelingDailyCost() {
	hirelingRuntimeMu.Lock()
	defer hirelingRuntimeMu.Unlock()
	for instID, state := range s.hirelingRuntimeStates {
		if state.HiredByPlayerID == "" {
			continue
		}
		sess, ok := s.sessions.GetPlayer(state.HiredByPlayerID)
		if !ok {
			state.HiredByPlayerID = ""
			state.ZonesFollowed = 0
			continue
		}
		inst := s.npcMgr.InstanceByID(instID)
		if inst == nil {
			state.HiredByPlayerID = ""
			continue
		}
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil || tmpl.Hireling == nil {
			continue
		}
		if sess.Currency < tmpl.Hireling.DailyCost {
			state.HiredByPlayerID = ""
			state.ZonesFollowed = 0
		} else {
			sess.Currency -= tmpl.Hireling.DailyCost
		}
	}
}
