package gameserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// parseTechRef decodes a hotbar slot ref for technology slots.
// Refs are plain tech IDs ("frost_bolt") or level-encoded ("frost_bolt:2").
// Returns techID and level; level==0 means any level (backward compatibility).
//
// Precondition: ref is non-empty.
// Postcondition: techID is non-empty; level>=0.
func parseTechRef(ref string) (techID string, level int) {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		if lvl, err := strconv.Atoi(ref[i+1:]); err == nil && lvl > 0 {
			return ref[:i], lvl
		}
	}
	return ref, 0
}

// handleHotbar processes hotbar commands: set, clear, show, create, switch.
// For "set" with kind+ref, creates a typed slot. For "set" with text only,
// creates a command slot (backward compatibility with telnet hotbar command).
//
// Precondition: uid identifies a connected player; req is non-nil.
// Postcondition: On "set"/"clear", sess.Hotbars[ActiveHotbarIndex] updated, SaveHotbars called,
//
//	HotbarUpdateEvent pushed via entity (for web client refresh), and a confirmation
//	MessageEvent returned (for telnet feedback).
//	On "show", per-slot MessageEvents pushed to entity; nil returned.
//	On "create", a new empty bar is appended and ActiveHotbarIndex updated; nil returned at limit.
//	On "switch", ActiveHotbarIndex updated; nil returned on success, message on out-of-range.
//	On out-of-range slot or empty set payload, MessageEvent returned with no side effects.
func (s *GameServiceServer) handleHotbar(uid string, req *gamev1.HotbarRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	switch req.Action {
	case "set":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		slot := buildHotbarSlot(req)
		if slot.IsEmpty() {
			return messageEvent("Nothing to set: provide text or kind+ref."), nil
		}
		sess.Hotbars[sess.ActiveHotbarIndex][idx] = slot
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		// Push HotbarUpdateEvent so web client hotbar refreshes; return confirmation message
		// so telnet players see text feedback (REQ-HB-12).
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return messageEvent(fmt.Sprintf("Slot %d set.", req.Slot)), nil

	case "clear":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		sess.Hotbars[sess.ActiveHotbarIndex][idx] = session.HotbarSlot{}
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		// Push HotbarUpdateEvent so web client hotbar refreshes; return confirmation message
		// so telnet players see text feedback (REQ-HB-12).
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return messageEvent(fmt.Sprintf("Slot %d cleared.", req.Slot)), nil

	case "show":
		activeBar := sess.Hotbars[sess.ActiveHotbarIndex]
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			display := "---"
			if cmd := activeBar[i].ActivationCommand(); cmd != "" {
				display = cmd
			}
			s.pushMessageToUID(uid, fmt.Sprintf("[%d] %s", slotNum, display))
		}
		return nil, nil

	case "create":
		maxHotbars := s.maxHotbars
		if maxHotbars <= 0 {
			maxHotbars = 4
		}
		if len(sess.Hotbars) >= maxHotbars {
			return messageEvent(fmt.Sprintf("Hotbar limit reached (max %d).", maxHotbars)), nil
		}
		sess.Hotbars = append(sess.Hotbars, [10]session.HotbarSlot{})
		sess.ActiveHotbarIndex = len(sess.Hotbars) - 1
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return nil, nil

	case "switch":
		// HotbarIndex is 1-based; 0 means "current" (no-op on index, just refresh).
		targetIdx := int(req.HotbarIndex) - 1
		if targetIdx < 0 || targetIdx >= len(sess.Hotbars) {
			return messageEvent("Invalid hotbar index."), nil
		}
		sess.ActiveHotbarIndex = targetIdx
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbars(context.Background(), sess.CharacterID, sess.Hotbars, sess.ActiveHotbarIndex); err != nil {
				s.logger.Warn("SaveHotbars failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		s.pushEventToUID(uid, s.hotbarUpdateEvent(sess))
		return nil, nil

	default:
		return messageEvent(fmt.Sprintf("Unknown hotbar action '%s'. Usage: hotbar [<slot> <text>] | clear <slot>", req.Action)), nil
	}
}

// buildHotbarSlot converts a HotbarRequest into a domain HotbarSlot.
// Prefers kind+ref over text for typed assignment.
//
// Precondition: req is non-nil.
// Postcondition: Returns a non-empty HotbarSlot if req has valid kind+ref or text; otherwise empty.
func buildHotbarSlot(req *gamev1.HotbarRequest) session.HotbarSlot {
	if req.Kind != "" && req.Ref != "" {
		return session.HotbarSlot{Kind: req.Kind, Ref: req.Ref}
	}
	if req.Text != "" {
		return session.CommandSlot(req.Text)
	}
	return session.HotbarSlot{}
}

// hotbarUpdateEvent builds a HotbarUpdateEvent for the player's current active hotbar.
// Resolves display_name, description, and use-count fields from registries and session state.
//
// Postcondition: Returns a non-nil ServerEvent with exactly 10 HotbarSlot entries and
//
//	multi-bar metadata (ActiveHotbarIndex, HotbarCount, MaxHotbars).
func (s *GameServiceServer) hotbarUpdateEvent(sess *session.PlayerSession) *gamev1.ServerEvent {
	activeBar := sess.Hotbars[sess.ActiveHotbarIndex]
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range activeBar {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
			ps.UsesRemaining, ps.MaxUses, ps.RechargeCondition = s.resolveHotbarSlotUseState(sess, sl)
			ps.ApCost, ps.DamageSummary = s.resolveHotbarSlotTechInfo(sl)
		}
		protoSlots[i] = ps
	}
	maxHotbars := int32(s.maxHotbars)
	if maxHotbars <= 0 {
		maxHotbars = 4
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{
				Slots:             protoSlots,
				ActiveHotbarIndex: int32(sess.ActiveHotbarIndex + 1), // 1-based for client
				HotbarCount:       int32(len(sess.Hotbars)),
				MaxHotbars:        maxHotbars,
			},
		},
	}
}

// resolveHotbarSlotTechInfo returns the AP cost and a compact damage/heal summary
// for technology and feat hotbar slots. Returns (0, "") for command slots and
// slots not found in any registry.
//
// Precondition: slot.Ref is non-empty.
// Postcondition: ApCost > 0 only for technology/feat slots with a defined action cost.
// DamageSummary is a compact string like "2d6 fire" or "1d8 healing", or "" if none.
func (s *GameServiceServer) resolveHotbarSlotTechInfo(slot session.HotbarSlot) (apCost int32, damageSummary string) {
	switch slot.Kind {
	case session.HotbarSlotKindTechnology:
		if s.techRegistry == nil {
			return 0, ""
		}
		techID, _ := parseTechRef(slot.Ref)
		tech, ok := s.techRegistry.Get(techID)
		if !ok {
			return 0, ""
		}
		return int32(tech.ActionCost), primaryDamageSummary(tech)
	case session.HotbarSlotKindFeat:
		if s.featRegistry == nil {
			return 0, ""
		}
		feat, ok := s.featRegistry.Feat(slot.Ref)
		if !ok {
			return 0, ""
		}
		return int32(feat.ActionCost), ""
	default:
		return 0, ""
	}
}

// primaryDamageSummary extracts a compact damage or heal summary from the primary
// tier of effects for a technology definition.
// Returns "" if no damage or heal effect is found.
//
// Precondition: tech is non-nil.
func primaryDamageSummary(tech *technology.TechnologyDef) string {
	var candidates []technology.TechEffect
	switch tech.Resolution {
	case "attack":
		candidates = append(candidates, tech.Effects.OnHit...)
		candidates = append(candidates, tech.Effects.OnCritHit...)
	case "save":
		candidates = append(candidates, tech.Effects.OnFailure...)
		candidates = append(candidates, tech.Effects.OnCritFailure...)
		candidates = append(candidates, tech.Effects.OnSuccess...)
	default:
		candidates = append(candidates, tech.Effects.OnApply...)
	}
	for _, e := range candidates {
		switch e.Type {
		case technology.EffectDamage:
			s := formatDiceAmount(e.Dice, e.Amount)
			if e.DamageType != "" {
				s += " " + e.DamageType
			}
			return s
		case technology.EffectHeal:
			return formatDiceAmount(e.Dice, e.Amount) + " healing"
		}
	}
	return ""
}

func formatDiceAmount(dice string, amount int) string {
	switch {
	case dice != "" && amount != 0:
		return fmt.Sprintf("%s+%d", dice, amount)
	case dice != "":
		return dice
	case amount != 0:
		return fmt.Sprintf("%d", amount)
	default:
		return ""
	}
}

// resolveHotbarSlotUseState returns the uses-remaining, max-uses, and recharge condition
// for a hotbar slot by querying session state and registries.
//
// Precondition: sess and slot are valid; slot.Kind is a recognized kind.
// Postcondition: Returns (0, 0, "") for unlimited or command slots.
func (s *GameServiceServer) resolveHotbarSlotUseState(sess *session.PlayerSession, slot session.HotbarSlot) (usesRemaining, maxUses int32, rechargeCondition string) {
	switch slot.Kind {
	case session.HotbarSlotKindFeat:
		if s.featRegistry != nil {
			if feat, ok := s.featRegistry.Feat(slot.Ref); ok {
				if feat.PreparedUses > 0 {
					maxUses = int32(feat.PreparedUses)
					if sess.ActiveFeatUses != nil {
						usesRemaining = int32(sess.ActiveFeatUses[slot.Ref])
					}
					rechargeCondition = feat.RechargeCondition
				}
			}
		}
	case session.HotbarSlotKindTechnology:
		techID, techLevel := parseTechRef(slot.Ref)
		if s.techRegistry != nil {
			if techDef, ok := s.techRegistry.Get(techID); ok {
				rechargeCondition = techDef.RechargeCondition
			}
		}
		// Innate tech: look up by tech ID.
		// MaxUses == 0 means unlimited; sentinel -1 signals the client to show ∞.
		if innate, ok := sess.InnateTechs[techID]; ok {
			if innate.MaxUses == 0 {
				maxUses = -1
				usesRemaining = -1
			} else {
				maxUses = int32(innate.MaxUses)
				usesRemaining = int32(innate.UsesRemaining)
			}
		} else if sess.PreparedTechs != nil {
			// Prepared tech: count non-expended slots for this tech ID at the specific level.
			// If techLevel==0 (legacy slot without level), count across all levels.
			var total, remaining int32
			if techLevel > 0 {
				// Level-encoded ref: count only slots at the specific level.
				for _, ps := range sess.PreparedTechs[techLevel] {
					if ps != nil && ps.TechID == techID {
						total++
						if !ps.Expended {
							remaining++
						}
					}
				}
			} else {
				// Legacy ref (no level): count across all levels.
				for _, pslots := range sess.PreparedTechs {
					for _, ps := range pslots {
						if ps != nil && ps.TechID == techID {
							total++
							if !ps.Expended {
								remaining++
							}
						}
					}
				}
			}
			if total > 0 {
				maxUses = total
				usesRemaining = remaining
			} else if sess.SpontaneousUsePools != nil {
				// Spontaneous tech: find pool level from KnownTechs.
				for lvl, techIDs := range sess.KnownTechs {
					for _, tid := range techIDs {
						if tid == techID {
							if pool, ok := sess.SpontaneousUsePools[lvl]; ok && pool.Max > 0 {
								maxUses = int32(pool.Max)
								usesRemaining = int32(pool.Remaining)
							}
							break
						}
					}
				}
			}
		}
	}
	return
}

// resolveHotbarSlotDisplay returns the display_name and description for a slot
// by querying the appropriate registry.
//
// Precondition: slot.Ref is non-empty.
// Postcondition: Returns ("", "") when the ID is not found in any registry.
func (s *GameServiceServer) resolveHotbarSlotDisplay(slot session.HotbarSlot) (displayName, description string) {
	switch slot.Kind {
	case session.HotbarSlotKindFeat:
		if s.featRegistry != nil {
			if feat, ok := s.featRegistry.Feat(slot.Ref); ok {
				return feat.Name, feat.Description
			}
		}
	case session.HotbarSlotKindTechnology:
		if s.techRegistry != nil {
			techID, level := parseTechRef(slot.Ref)
			if tech, ok := s.techRegistry.Get(techID); ok {
				name := tech.Name
				if tech.ShortName != "" {
					name = tech.ShortName
				}
				if level > 0 {
					name = fmt.Sprintf("%s (L%d)", name, level)
				}
				return name, tech.Description
			}
		}
	case session.HotbarSlotKindThrowable, session.HotbarSlotKindConsumable:
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(slot.Ref); ok {
				return def.Name, def.Description
			}
		}
	}
	return "", ""
}
