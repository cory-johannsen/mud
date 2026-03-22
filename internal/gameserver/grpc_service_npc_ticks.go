package gameserver

import (
	"math/rand"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

// tickMerchantReplenish advances all overdue merchant runtime states by one replenishment cycle.
//
// Precondition: s.merchantRuntimeStates MUST NOT be nil.
// Postcondition: every state whose NextReplenishAt is not after now has been advanced via npc.ApplyReplenish.
func (s *GameServiceServer) tickMerchantReplenish(now time.Time) {
	merchantRuntimeMu.Lock()
	defer merchantRuntimeMu.Unlock()
	for instID, state := range s.merchantRuntimeStates {
		if now.Before(state.NextReplenishAt) {
			continue
		}
		inst := s.npcMgr.InstanceByID(instID)
		if inst == nil {
			continue
		}
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil || tmpl.Merchant == nil {
			continue
		}
		s.merchantRuntimeStates[instID] = npc.ApplyReplenish(tmpl.Merchant, state, 0)
	}
}

// tickBankerRates recalculates CurrentRate for all banker instances.
// Intended to be called once per in-game day.
//
// Precondition: s.bankerRuntimeStates MUST NOT be nil.
// Postcondition: every banker state's CurrentRate is updated within the configured variance bounds.
func (s *GameServiceServer) tickBankerRates() {
	bankerRuntimeMu.Lock()
	defer bankerRuntimeMu.Unlock()
	for instID, state := range s.bankerRuntimeStates {
		inst := s.npcMgr.InstanceByID(instID)
		if inst == nil {
			continue
		}
		tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
		if tmpl == nil || tmpl.Banker == nil {
			continue
		}
		variance := tmpl.Banker.RateVariance
		delta := (rand.Float64()*2 - 1) * variance
		state.CurrentRate = npc.NewCurrentRateFromDelta(tmpl.Banker, delta)
	}
}

// StartNPCTickHook subscribes to the calendar and drives periodic NPC state updates.
// tickMerchantReplenish is called on every calendar tick; tickBankerRates is called
// once per in-game day (when dt.Hour == 0).
//
// Precondition: MUST be called after GameServiceServer is fully initialized.
// Precondition: s.calendar MUST NOT be nil.
// Postcondition: returns a stop function; call it to unsubscribe and stop the goroutine.
func (s *GameServiceServer) StartNPCTickHook() func() {
	if s.calendar == nil {
		return func() {}
	}
	ch := make(chan GameDateTime, 4)
	s.calendar.Subscribe(ch)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case dt := <-ch:
				s.tickMerchantReplenish(time.Now())
				if dt.Hour == 0 {
					s.tickBankerRates()
					s.tickHealerCapacity()
					s.tickHirelingDailyCost()
				}
			case <-stop:
				s.calendar.Unsubscribe(ch)
				return
			}
		}
	}()
	return func() { close(stop) }
}
