package gameserver

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleHotbar processes hotbar commands: set, clear, show.
// For "set" with kind+ref, creates a typed slot. For "set" with text only,
// creates a command slot (backward compatibility with telnet hotbar command).
//
// Precondition: uid identifies a connected player; req is non-nil.
// Postcondition: On "set"/"clear", sess.Hotbar updated, SaveHotbar called, HotbarUpdateEvent returned.
//
//	On "show", per-slot MessageEvents pushed to entity; nil returned.
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
		sess.Hotbar[idx] = slot
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbar(context.Background(), sess.CharacterID, sess.Hotbar); err != nil {
				s.logger.Warn("SaveHotbar failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		return s.hotbarUpdateEvent(sess), nil

	case "clear":
		if req.Slot < 1 || req.Slot > 10 {
			return messageEvent("Slot out of range (1-10)."), nil
		}
		idx := int(req.Slot) - 1
		sess.Hotbar[idx] = session.HotbarSlot{}
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbar(context.Background(), sess.CharacterID, sess.Hotbar); err != nil {
				s.logger.Warn("SaveHotbar failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		return s.hotbarUpdateEvent(sess), nil

	case "show":
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			display := "---"
			if cmd := sess.Hotbar[i].ActivationCommand(); cmd != "" {
				display = cmd
			}
			s.pushMessageToUID(uid, fmt.Sprintf("[%d] %s", slotNum, display))
		}
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

// hotbarUpdateEvent builds a HotbarUpdateEvent for the player's current hotbar.
// Resolves display_name, description, and use-count fields from registries and session state.
//
// Postcondition: Returns a non-nil ServerEvent with exactly 10 HotbarSlot entries.
func (s *GameServiceServer) hotbarUpdateEvent(sess *session.PlayerSession) *gamev1.ServerEvent {
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range sess.Hotbar {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
			ps.UsesRemaining, ps.MaxUses, ps.RechargeCondition = s.resolveHotbarSlotUseState(sess, sl)
		}
		protoSlots[i] = ps
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{Slots: protoSlots},
		},
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
		if s.techRegistry != nil {
			if techDef, ok := s.techRegistry.Get(slot.Ref); ok {
				rechargeCondition = techDef.RechargeCondition
			}
		}
		// Innate tech: look up by tech ID.
		if innate, ok := sess.InnateTechs[slot.Ref]; ok && innate.MaxUses > 0 {
			maxUses = int32(innate.MaxUses)
			usesRemaining = int32(innate.UsesRemaining)
		} else if sess.PreparedTechs != nil {
			// Prepared tech: count non-expended slots for this tech ID.
			var total, remaining int32
			for _, pslots := range sess.PreparedTechs {
				for _, ps := range pslots {
					if ps != nil && ps.TechID == slot.Ref {
						total++
						if !ps.Expended {
							remaining++
						}
					}
				}
			}
			if total > 0 {
				maxUses = total
				usesRemaining = remaining
			} else if sess.SpontaneousUsePools != nil {
				// Spontaneous tech: find pool level from SpontaneousTechs.
				for lvl, techIDs := range sess.SpontaneousTechs {
					for _, tid := range techIDs {
						if tid == slot.Ref {
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
			if tech, ok := s.techRegistry.Get(slot.Ref); ok {
				name := tech.Name
				if tech.ShortName != "" {
					name = tech.ShortName
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
