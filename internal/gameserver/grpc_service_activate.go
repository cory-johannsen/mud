package gameserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// diceRollerAdapter adapts *dice.Roller to the inventory.Roller interface.
//
// Precondition: r must not be nil.
type diceRollerAdapter struct{ r *dice.Roller }

func (a *diceRollerAdapter) Roll(expr string) int {
	res, err := a.r.RollExpr(expr)
	if err != nil {
		return 0
	}
	return res.Total()
}

func (a *diceRollerAdapter) RollD20() int {
	res, err := a.r.RollExpr("1d20")
	if err != nil {
		return 10
	}
	return res.Total()
}

func (a *diceRollerAdapter) RollFloat() float64 {
	res, err := a.r.RollExpr("1d100")
	if err != nil {
		return 0.5
	}
	return float64(res.Total()-1) / 100.0
}

// playerActivateAdapter bridges *session.PlayerSession to inventory.ActivateSession.
// Disease and toxin application paths are logged as warnings when the substance
// registry is unavailable (those systems are wired in production).
//
// Precondition: sess and svc must not be nil.
type playerActivateAdapter struct {
	sess *session.PlayerSession
	svc  *GameServiceServer
}

// GetTeam returns the player's team via the job registry.
func (a *playerActivateAdapter) GetTeam() string {
	if a.svc.jobRegistry == nil {
		return ""
	}
	return a.svc.jobRegistry.TeamFor(a.sess.Class)
}

// GetStatModifier maps PF2E-style stat names to Gunchete ability scores.
func (a *playerActivateAdapter) GetStatModifier(stat string) int {
	ab := a.sess.Abilities
	switch strings.ToLower(stat) {
	case "brutality", "strength":
		return (ab.Brutality - 10) / 2
	case "grit", "constitution":
		return (ab.Grit - 10) / 2
	case "quickness", "dexterity":
		return (ab.Quickness - 10) / 2
	case "reasoning", "intelligence":
		return (ab.Reasoning - 10) / 2
	case "savvy", "wisdom":
		return (ab.Savvy - 10) / 2
	case "flair", "charisma":
		return (ab.Flair - 10) / 2
	default:
		return 0
	}
}

// ApplyHeal adds heal to the player's CurrentHP, capped at MaxHP.
func (a *playerActivateAdapter) ApplyHeal(amount int) {
	a.sess.CurrentHP += amount
	if a.sess.CurrentHP > a.sess.MaxHP {
		a.sess.CurrentHP = a.sess.MaxHP
	}
}

// ApplyCondition applies a condition with the given duration to the player.
// Duration <= 0 is treated as a permanent/encounter-scoped condition (stacks=1, duration=-1).
func (a *playerActivateAdapter) ApplyCondition(conditionID string, duration time.Duration) {
	if a.svc.condRegistry == nil || a.sess.Conditions == nil {
		return
	}
	def, ok := a.svc.condRegistry.Get(conditionID)
	if !ok {
		return
	}
	durSec := -1
	if duration > 0 {
		durSec = int(duration.Seconds())
	}
	_ = a.sess.Conditions.Apply(a.sess.UID, def, 1, durSec)
}

// RemoveCondition removes a condition from the player.
func (a *playerActivateAdapter) RemoveCondition(conditionID string) {
	if a.sess.Conditions == nil {
		return
	}
	a.sess.Conditions.Remove(a.sess.UID, conditionID)
}

// ApplyDisease applies a disease substance by ID (severity is advisory; substance def governs).
func (a *playerActivateAdapter) ApplyDisease(diseaseID string, _ int) {
	if err := a.svc.ApplySubstanceByID(a.sess.UID, diseaseID); err != nil {
		a.svc.logger.Warn("playerActivateAdapter.ApplyDisease failed",
			zap.String("uid", a.sess.UID),
			zap.String("disease_id", diseaseID),
			zap.Error(err),
		)
	}
}

// ApplyToxin applies a toxin substance by ID (severity is advisory; substance def governs).
func (a *playerActivateAdapter) ApplyToxin(toxinID string, _ int) {
	if err := a.svc.ApplySubstanceByID(a.sess.UID, toxinID); err != nil {
		a.svc.logger.Warn("playerActivateAdapter.ApplyToxin failed",
			zap.String("uid", a.sess.UID),
			zap.String("toxin_id", toxinID),
			zap.Error(err),
		)
	}
}

// EquippedInstances returns the player's currently equipped item instances.
func (a *playerActivateAdapter) EquippedInstances() []*inventory.ItemInstance {
	return a.sess.EquippedInstances()
}

// handleActivate processes the "activate <item>" command.
//
// Precondition: uid non-empty; req non-nil.
// Postcondition: on success, charge state is persisted and a narrative event is returned.
func (s *GameServiceServer) handleActivate(uid string, req *gamev1.ActivateItemRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}

	inCombat := sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)
	ap := 0
	if inCombat && s.combatH != nil {
		ap = s.combatH.RemainingAP(uid)
	}

	adapter := &playerActivateAdapter{sess: sess, svc: s}
	result, errMsg := inventory.HandleActivate(adapter, s.invRegistry, req.ItemQuery, inCombat, ap)
	if errMsg != "" {
		return errorEvent(errMsg), nil
	}

	// REQ-ACT-4: deduct AP in combat.
	if inCombat && s.combatH != nil {
		if err := s.combatH.SpendAP(uid, result.AP); err != nil {
			return errorEvent(err.Error()), nil
		}
	}

	// Apply effects.
	var scriptMsg string
	if result.Script != "" {
		var zoneID string
		if s.world != nil {
			if room, ok := s.world.GetRoom(sess.RoomID); ok {
				zoneID = room.ZoneID
			}
		}
		if zoneID != "" && s.scriptMgr != nil {
			// REQ-BUG15: uid must be forwarded to the Lua hook so scripts such as
			// zone_map_use(uid) can call engine.map.reveal_zone(uid, zoneID).
			if luaResult, callErr := s.scriptMgr.CallHook(zoneID, result.Script, lua.LString(uid)); callErr == nil && luaResult != lua.LNil {
				scriptMsg = luaResult.String()
			}
		}
	} else if result.ActivationEffect != nil {
		def, _ := s.invRegistry.Item(result.ItemDefID)
		team := ""
		if def != nil {
			team = def.Team
		}
		syntheticDef := &inventory.ItemDef{
			Effect: result.ActivationEffect,
			Team:   team,
		}
		var rng inventory.Roller
		if s.dice != nil {
			rng = &diceRollerAdapter{r: s.dice}
		} else {
			rng = &DurabilityRoller{}
		}
		inventory.ApplyConsumable(adapter, syntheticDef, rng)
	}

	// REQ-ACT-17: persist charge state BEFORE removing item from slot.
	if s.charSaver != nil {
		if err := s.persistChargeState(context.Background(), sess, result.ItemDefID); err != nil {
			s.logger.Warn("handleActivate: failed to persist charge state",
				zap.String("uid", uid),
				zap.String("item", result.ItemDefID),
				zap.Error(err),
			)
		}
	}

	// REQ-ACT-18: handle destruction (after persist).
	if result.Destroyed {
		s.removeEquippedItem(sess, result.ItemDefID)
	}

	def, _ := s.invRegistry.Item(result.ItemDefID)
	name := result.ItemDefID
	if def != nil {
		name = def.Name
	}
	if scriptMsg != "" {
		return messageEvent(scriptMsg), nil
	}
	if result.Destroyed {
		return messageEvent(fmt.Sprintf("You activate %s — it crumbles to dust.", name)), nil
	}
	return messageEvent(fmt.Sprintf("You activate %s.", name)), nil
}

// RechargeOnRest fires the "rest" recharge trigger for the given player's equipped instances.
// Called by the Resting feature when a player completes a rest (REQ-ACT-22).
//
// Precondition: uid non-empty.
// Postcondition: ChargesRemaining updated for matching items and persisted to DB.
func (s *GameServiceServer) RechargeOnRest(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	s.runRechargeForSession(context.Background(), sess, "rest")
}

// persistChargeState writes ChargesRemaining and Expended for the ItemInstance matching itemDefID.
//
// Precondition: sess non-nil; itemDefID non-empty.
// Postcondition: DB updated for the first matching instance found; returns nil on success.
func (s *GameServiceServer) persistChargeState(ctx context.Context, sess *session.PlayerSession, itemDefID string) error {
	for _, inst := range sess.EquippedInstances() {
		if inst.ItemDefID == itemDefID {
			return s.charSaver.SaveInstanceCharges(ctx, sess.CharacterID, inst.InstanceID, inst.ItemDefID, inst.ChargesRemaining, inst.Expended)
		}
	}
	return nil
}

// removeEquippedItem removes a destroyed item from equipped slots (active preset only) and persists.
// REQ-ACT-18/19: only the ACTIVE weapon preset slot is cleared; other presets are untouched.
func (s *GameServiceServer) removeEquippedItem(sess *session.PlayerSession, itemDefID string) {
	ctx := context.Background()
	if sess.LoadoutSet != nil && sess.LoadoutSet.Active < len(sess.LoadoutSet.Presets) {
		active := sess.LoadoutSet.Presets[sess.LoadoutSet.Active]
		if active != nil {
			if active.MainHand != nil && active.MainHand.Def != nil && active.MainHand.Def.ID == itemDefID {
				active.MainHand = nil
			}
			if active.OffHand != nil && active.OffHand.Def != nil && active.OffHand.Def.ID == itemDefID {
				active.OffHand = nil
			}
		}
	}
	if sess.Equipment != nil {
		for slot, si := range sess.Equipment.Armor {
			if si != nil && si.ItemDefID == itemDefID {
				sess.Equipment.Armor[slot] = nil
			}
		}
		for slot, si := range sess.Equipment.Accessories {
			if si != nil && si.ItemDefID == itemDefID {
				sess.Equipment.Accessories[slot] = nil
			}
		}
	}
	if s.charSaver != nil {
		_ = s.charSaver.SaveWeaponPresets(ctx, sess.CharacterID, sess.LoadoutSet)
		_ = s.charSaver.SaveEquipment(ctx, sess.CharacterID, sess.Equipment)
	}
}

// runRechargeForSession calls TickRecharge for a player and persists/notifies on changes.
func (s *GameServiceServer) runRechargeForSession(ctx context.Context, sess *session.PlayerSession, trigger string) {
	instances := sess.EquippedInstances()
	modified := inventory.TickRecharge(instances, s.invRegistry, trigger)
	for _, inst := range modified {
		if s.charSaver != nil {
			if err := s.persistChargeState(ctx, sess, inst.ItemDefID); err != nil {
				s.logger.Warn("runRechargeForSession: failed to persist",
					zap.String("uid", sess.UID),
					zap.String("item", inst.ItemDefID),
					zap.Error(err),
				)
			}
		}
		def, _ := s.invRegistry.Item(inst.ItemDefID)
		name := inst.ItemDefID
		if def != nil {
			name = def.Name
		}
		evt := messageEvent(name + " has recharged.")
		if data, err := proto.Marshal(evt); err == nil {
			_ = sess.Entity.Push(data)
		}
	}
}
