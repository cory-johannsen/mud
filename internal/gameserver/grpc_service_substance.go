package gameserver

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/substance"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// ambientDoseInterval is the minimum elapsed time between ambient substance doses.
//
// REQ-OCF-8: Players in a room with AmbientSubstance receive one dose per 60 seconds.
const ambientDoseInterval = 60 * time.Second

// ShouldApplyAmbientDose reports whether enough time has elapsed since lastDose
// to apply the next ambient substance dose.
//
// Precondition: now must be >= lastDose (or lastDose is zero).
// Postcondition: Returns true iff lastDose is zero OR now-lastDose >= ambientDoseInterval.
func ShouldApplyAmbientDose(lastDose, now time.Time) bool {
	if lastDose.IsZero() {
		return true
	}
	return now.Sub(lastDose) >= ambientDoseInterval
}

// tickAmbientSubstances iterates all active player sessions and applies one micro-dose
// of the room's AmbientSubstance to each player who has not been dosed in the last 60s.
//
// REQ-OCF-8: Called by the 5-second ticker goroutine; enforces 60-second minimum interval.
// Precondition: s.world and s.substanceReg must be non-nil for dosing to occur.
// Postcondition: sess.LastAmbientDose is updated for each player that receives a dose.
func (s *GameServiceServer) tickAmbientSubstances() {
	if s.world == nil || s.substanceReg == nil {
		return
	}
	now := time.Now()
	for _, uid := range s.sessions.AllUIDs() {
		sess, ok := s.sessions.GetPlayer(uid)
		if !ok {
			continue
		}
		room, ok := s.world.GetRoom(sess.RoomID)
		if !ok || room.AmbientSubstance == "" {
			continue
		}
		if !ShouldApplyAmbientDose(sess.LastAmbientDose, now) {
			continue
		}
		def, ok := s.substanceReg.Get(room.AmbientSubstance)
		if !ok {
			log.Printf("tickAmbientSubstances: room %q references unknown substance %q", sess.RoomID, room.AmbientSubstance)
			continue
		}
		sess.LastAmbientDose = now
		s.applySubstanceDose(uid, def)
	}
}

// applySubstanceDose applies one dose of def to the player session identified by uid.
//
// Preconditions: def must be non-nil.
// Postconditions:
//   - ActiveSubstances contains an entry for def.ID with updated DoseCount/ExpiresAt.
//   - Overdose condition applied if DoseCount > OverdoseThreshold.
//   - Addiction state advanced per state machine (REQ-AH-11/17/18).
func (s *GameServiceServer) applySubstanceDose(uid string, def *substance.SubstanceDef) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}

	now := time.Now()

	// Find or create entry in ActiveSubstances.
	var entry *substance.ActiveSubstance
	for i := range sess.ActiveSubstances {
		if sess.ActiveSubstances[i].SubstanceID == def.ID {
			entry = &sess.ActiveSubstances[i]
			break
		}
	}
	if entry == nil {
		sess.ActiveSubstances = append(sess.ActiveSubstances, substance.ActiveSubstance{
			SubstanceID: def.ID,
			DoseCount:   1,
			OnsetAt:     now.Add(def.OnsetDelay),
			ExpiresAt:   now.Add(def.OnsetDelay + def.Duration),
		})
		entry = &sess.ActiveSubstances[len(sess.ActiveSubstances)-1]
	} else {
		entry.DoseCount++
		entry.ExpiresAt = now.Add(def.OnsetDelay + def.Duration)
	}

	// REQ-AH-10: overdose check.
	if entry.DoseCount > def.OverdoseThreshold && def.OverdoseCondition != "" && s.condRegistry != nil {
		if condDef, ok := s.condRegistry.Get(def.OverdoseCondition); ok && sess.Conditions != nil {
			_ = sess.Conditions.Apply(uid, condDef, 1, -1)
		}
		s.pushMessageToUID(uid, "You've taken too much. Your body reacts violently.")
	}

	// REQ-AH-11 / REQ-AH-17 / REQ-AH-18: addiction state machine.
	if def.Addictive {
		if sess.AddictionState == nil {
			sess.AddictionState = make(map[string]substance.SubstanceAddiction)
		}
		addict := sess.AddictionState[def.ID]
		switch addict.Status {
		case "":
			addict.Status = "at_risk"
		case "at_risk":
			if rand.Float64() < def.AddictionChance { //nolint:gosec
				addict.Status = "addicted"
				s.pushMessageToUID(uid, "You feel a gnawing need for more.")
			}
		case "withdrawal":
			// REQ-AH-17: suppress symptoms, reset timer.
			s.removeWithdrawalConditions(uid, sess, def)
			addict.WithdrawalUntil = time.Time{}
			addict.Status = "addicted"
		case "addicted":
			// REQ-AH-18: re-roll.
			if rand.Float64() < def.AddictionChance { //nolint:gosec
				s.pushMessageToUID(uid, "Your dependency deepens.")
			}
		}
		sess.AddictionState[def.ID] = addict
	}
}

// removeWithdrawalConditions removes all withdrawal conditions for def from uid's session.
//
// Preconditions: sess must be non-nil.
// Postconditions: withdrawal conditions are removed from sess.Conditions;
// SubstanceConditionRefs are decremented.
func (s *GameServiceServer) removeWithdrawalConditions(uid string, sess *session.PlayerSession, def *substance.SubstanceDef) {
	if sess.Conditions == nil {
		return
	}
	for _, condID := range def.WithdrawalConditions {
		if sess.SubstanceConditionRefs != nil {
			sess.SubstanceConditionRefs[condID]--
			if sess.SubstanceConditionRefs[condID] <= 0 {
				delete(sess.SubstanceConditionRefs, condID)
				sess.Conditions.Remove(uid, condID)
			}
		} else {
			sess.Conditions.Remove(uid, condID)
		}
	}
}

// ApplySubstanceByID looks up substanceID and calls applySubstanceDose directly.
//
// REQ-AH-20: bypasses the use handler category guard.
// Postcondition: returns error if substanceID is not found in the registry.
func (s *GameServiceServer) ApplySubstanceByID(uid, substanceID string) error {
	if s.substanceReg == nil {
		return fmt.Errorf("substance registry not initialised")
	}
	def, ok := s.substanceReg.Get(substanceID)
	if !ok {
		return fmt.Errorf("substance %q not found", substanceID)
	}
	s.applySubstanceDose(uid, def)
	return nil
}

// handleConsumeSubstanceItem is the handler for using a KindConsumable inventory item with a substance ID.
//
// REQ-AH-8: dispatched from the item use command handler.
// REQ-AH-8A: blocks poison and toxin from voluntary direct use.
// Preconditions: uid must be a valid player session; itemDefID must reference a KindConsumable item.
// Postconditions: substance dose applied; one stack removed from backpack; message event returned.
func (s *GameServiceServer) handleConsumeSubstanceItem(uid, itemDefID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("Session not found."), nil
	}
	if s.invRegistry == nil {
		return messageEvent("Inventory system unavailable."), nil
	}
	itemDef, defOK := s.invRegistry.Item(itemDefID)
	if !defOK || itemDef.SubstanceID == "" {
		return messageEvent("You can't use that."), nil
	}
	if s.substanceReg == nil {
		return messageEvent("Substance system unavailable."), nil
	}
	def, substOK := s.substanceReg.Get(itemDef.SubstanceID)
	if !substOK {
		return messageEvent("Unknown substance."), nil
	}
	// REQ-AH-8A: block poison and toxin from voluntary use.
	if def.Category == "poison" || def.Category == "toxin" {
		return messageEvent("You can't use that directly."), nil
	}
	s.applySubstanceDose(uid, def)
	// Remove one stack from backpack.
	if sess.Backpack != nil {
		instances := sess.Backpack.FindByItemDefID(itemDefID)
		if len(instances) > 0 {
			_ = sess.Backpack.Remove(instances[0].InstanceID, 1)
		}
	}
	return messageEvent(fmt.Sprintf("You use the %s.", itemDef.Name)), nil
}

// tickSubstances processes onset and expiry for all active substances in uid's session.
//
// REQ-AH-12: called by the 5-second ticker goroutine.
// REQ-AH-13: fires onset (EffectsApplied=false, time.Now().After(OnsetAt)) and
//
//	expiry (EffectsApplied=true, time.Now().After(ExpiresAt)).
//
// REQ-AH-14: applies hp_regen per tick for active medicine substances.
// REQ-AH-16: checks withdrawal expiry for all substances.
func (s *GameServiceServer) tickSubstances(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	if s.substanceReg == nil {
		return
	}

	now := time.Now()

	// REQ-AH-16: Check withdrawal expiry for each substance in AddictionState.
	for substID, addict := range sess.AddictionState {
		if addict.Status == "withdrawal" && now.After(addict.WithdrawalUntil) {
			def, defOK := s.substanceReg.Get(substID)
			if defOK {
				s.removeWithdrawalConditions(uid, sess, def)
			}
			addict.Status = ""
			addict.WithdrawalUntil = time.Time{}
			sess.AddictionState[substID] = addict
			s.pushMessageToUID(uid, "You feel like yourself again.")
		}
	}

	// Process active substance entries (onset, expiry, hp_regen).
	var remaining []substance.ActiveSubstance
	for i := range sess.ActiveSubstances {
		entry := &sess.ActiveSubstances[i]
		def, defOK := s.substanceReg.Get(entry.SubstanceID)
		if !defOK {
			remaining = append(remaining, *entry)
			continue
		}

		// REQ-AH-13: Onset.
		if !entry.EffectsApplied && now.After(entry.OnsetAt) {
			s.applySubstanceEffectsOnOnset(uid, sess, def)
			entry.EffectsApplied = true
			s.pushMessageToUID(uid, fmt.Sprintf("The %s kicks in.", def.Name))
		}

		// REQ-AH-14: HP regen while active.
		if entry.EffectsApplied && now.Before(entry.ExpiresAt) {
			for _, eff := range def.Effects {
				if eff.HPRegen > 0 {
					sess.CurrentHP += eff.HPRegen
					if sess.CurrentHP > sess.MaxHP {
						sess.CurrentHP = sess.MaxHP
					}
				}
			}
		}

		// REQ-AH-13: Expiry.
		if entry.EffectsApplied && now.After(entry.ExpiresAt) {
			s.applySubstanceExpiry(uid, sess, def)
			s.onSubstanceExpired(uid, def)
			continue // do not keep in remaining
		}

		remaining = append(remaining, *entry)
	}
	sess.ActiveSubstances = remaining
}

// applySubstanceEffectsOnOnset applies all onset effects for def to uid's session.
//
// REQ-AH-13: ApplyCondition increments SubstanceConditionRefs; CureConditions decrements.
func (s *GameServiceServer) applySubstanceEffectsOnOnset(uid string, sess *session.PlayerSession, def *substance.SubstanceDef) {
	if sess.Conditions == nil {
		return
	}
	if sess.SubstanceConditionRefs == nil {
		sess.SubstanceConditionRefs = make(map[string]int)
	}
	for _, eff := range def.Effects {
		if eff.ApplyCondition != "" && s.condRegistry != nil {
			if condDef, ok := s.condRegistry.Get(eff.ApplyCondition); ok {
				stacks := eff.Stacks
				if stacks < 1 {
					stacks = 1
				}
				_ = sess.Conditions.Apply(uid, condDef, stacks, -1)
				sess.SubstanceConditionRefs[eff.ApplyCondition]++
			}
		}
		if eff.RemoveCondition != "" {
			sess.Conditions.Remove(uid, eff.RemoveCondition)
		}
		// REQ-AH-24: CureConditions removes listed conditions immediately on onset.
		for _, condID := range eff.CureConditions {
			if sess.SubstanceConditionRefs != nil {
				sess.SubstanceConditionRefs[condID]--
				if sess.SubstanceConditionRefs[condID] <= 0 {
					delete(sess.SubstanceConditionRefs, condID)
					sess.Conditions.Remove(uid, condID)
				}
			} else {
				sess.Conditions.Remove(uid, condID)
			}
		}
	}
}

// applySubstanceExpiry removes per-expiry conditions for def from uid's session.
//
// REQ-AH-13: decrements SubstanceConditionRefs; removes zero-ref conditions.
func (s *GameServiceServer) applySubstanceExpiry(uid string, sess *session.PlayerSession, def *substance.SubstanceDef) {
	if sess.Conditions == nil {
		return
	}
	for _, condID := range def.RemoveOnExpire {
		if sess.SubstanceConditionRefs != nil {
			sess.SubstanceConditionRefs[condID]--
			if sess.SubstanceConditionRefs[condID] <= 0 {
				delete(sess.SubstanceConditionRefs, condID)
				sess.Conditions.Remove(uid, condID)
			}
		} else {
			sess.Conditions.Remove(uid, condID)
		}
	}
}

// onSubstanceExpired handles post-expiry logic: triggers withdrawal when addicted.
//
// REQ-AH-15.
func (s *GameServiceServer) onSubstanceExpired(uid string, def *substance.SubstanceDef) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	if sess.AddictionState == nil {
		return
	}
	addict := sess.AddictionState[def.ID]
	if addict.Status != "addicted" {
		return
	}

	addict.Status = "withdrawal"
	addict.WithdrawalUntil = time.Now().Add(def.RecoveryDuration)
	sess.AddictionState[def.ID] = addict

	// Apply withdrawal conditions.
	if sess.SubstanceConditionRefs == nil {
		sess.SubstanceConditionRefs = make(map[string]int)
	}
	if sess.Conditions != nil && s.condRegistry != nil {
		for _, condID := range def.WithdrawalConditions {
			if condDef, condOK := s.condRegistry.Get(condID); condOK {
				_ = sess.Conditions.Apply(uid, condDef, 1, -1)
				sess.SubstanceConditionRefs[condID]++
			}
		}
	}
	s.pushMessageToUID(uid, "You feel sick without your fix.")
}
