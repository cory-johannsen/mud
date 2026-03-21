package gameserver

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var healerRuntimeMu sync.RWMutex

// initHealerRuntimeState initialises runtime state for a healer instance if absent.
//
// Precondition: inst must be non-nil.
// Postcondition: healerRuntimeStates[inst.ID] is set iff inst.NPCType == "healer".
func (s *GameServiceServer) initHealerRuntimeState(inst *npc.Instance) {
	if inst.NPCType != "healer" {
		return
	}
	healerRuntimeMu.Lock()
	defer healerRuntimeMu.Unlock()
	if _, ok := s.healerRuntimeStates[inst.ID]; !ok {
		s.healerRuntimeStates[inst.ID] = &npc.HealerRuntimeState{}
	}
}

// healerStateFor returns the HealerRuntimeState for instID, or nil if absent.
func (s *GameServiceServer) healerStateFor(instID string) *npc.HealerRuntimeState {
	healerRuntimeMu.RLock()
	defer healerRuntimeMu.RUnlock()
	return s.healerRuntimeStates[instID]
}

// findHealerInRoom returns the first healer NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findHealerInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "healer" {
		return nil, fmt.Sprintf("%s is not a healer.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// tickHealerCapacity resets CapacityUsed to zero for all healer instances.
// Intended to be called once per in-game day.
//
// Precondition: s.healerRuntimeStates MUST NOT be nil.
// Postcondition: every healer state's CapacityUsed is set to 0.
func (s *GameServiceServer) tickHealerCapacity() {
	healerRuntimeMu.Lock()
	defer healerRuntimeMu.Unlock()
	for _, state := range s.healerRuntimeStates {
		state.CapacityUsed = 0
	}
}

// handleHeal restores the player to full HP via a healer NPC.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleHeal(uid string, req *gamev1.HealRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findHealerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Healer == nil {
		return messageEvent("This healer has no configuration."), nil
	}
	state := s.healerStateFor(inst.ID)
	if state == nil {
		s.initHealerRuntimeState(inst)
		state = s.healerStateFor(inst.ID)
	}
	if err := npc.CheckHealPrerequisites(tmpl.Healer, state, sess.CurrentHP, sess.MaxHP, sess.Currency); err != nil {
		remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
		missing := sess.MaxHP - sess.CurrentHP
		if remaining > 0 && remaining < missing {
			cost := tmpl.Healer.PricePerHP * remaining
			if sess.Currency >= cost {
				return messageEvent(fmt.Sprintf(
					"%s can only heal %d HP today. That would cost %d credits. Use 'heal %d %s' to accept.",
					inst.Name(), remaining, cost, remaining, inst.Name(),
				)), nil
			}
		}
		return messageEvent(err.Error()), nil
	}
	remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
	newHP, cost, newUsed := npc.ApplyHeal(tmpl.Healer, state, sess.CurrentHP, sess.MaxHP, remaining)
	healerRuntimeMu.Lock()
	state.CapacityUsed = newUsed
	healerRuntimeMu.Unlock()
	sess.CurrentHP = newHP
	sess.Currency -= cost
	return messageEvent(fmt.Sprintf(
		"%s patches you up. HP restored to %d/%d. Cost: %d credits.",
		inst.Name(), sess.CurrentHP, sess.MaxHP, cost,
	)), nil
}

// handleHealAmount restores the player by a specific HP amount via a healer NPC.
//
// Precondition: uid identifies an active player session; req is non-nil; req.Amount > 0.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleHealAmount(uid string, req *gamev1.HealAmountRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	amount := int(req.GetAmount())
	if amount <= 0 {
		return messageEvent("Specify a positive amount of HP to heal."), nil
	}
	inst, errMsg := s.findHealerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Healer == nil {
		return messageEvent("This healer has no configuration."), nil
	}
	state := s.healerStateFor(inst.ID)
	if state == nil {
		s.initHealerRuntimeState(inst)
		state = s.healerStateFor(inst.ID)
	}
	if sess.CurrentHP >= sess.MaxHP {
		return messageEvent("You are already at full health."), nil
	}
	remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
	if remaining <= 0 {
		return messageEvent(fmt.Sprintf("%s has exhausted their daily healing capacity.", inst.Name())), nil
	}
	missing := sess.MaxHP - sess.CurrentHP
	if amount > missing {
		amount = missing
	}
	if amount > remaining {
		amount = remaining
	}
	cost := npc.ComputeHealAmountCost(tmpl.Healer, amount)
	if sess.Currency < cost {
		return messageEvent(fmt.Sprintf("You need %d credits to heal %d HP but only have %d.", cost, amount, sess.Currency)), nil
	}
	healerRuntimeMu.Lock()
	state.CapacityUsed += amount
	healerRuntimeMu.Unlock()
	sess.CurrentHP += amount
	sess.Currency -= cost
	return messageEvent(fmt.Sprintf(
		"%s heals you for %d HP (%d/%d). Cost: %d credits.",
		inst.Name(), amount, sess.CurrentHP, sess.MaxHP, cost,
	)), nil
}
