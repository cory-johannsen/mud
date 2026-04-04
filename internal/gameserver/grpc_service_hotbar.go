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
		return s.hotbarUpdateEvent(sess.Hotbar), nil

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
		return s.hotbarUpdateEvent(sess.Hotbar), nil

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
// Resolves display_name and description from registries for typed slots.
//
// Postcondition: Returns a non-nil ServerEvent with exactly 10 HotbarSlot entries.
func (s *GameServiceServer) hotbarUpdateEvent(slots [10]session.HotbarSlot) *gamev1.ServerEvent {
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, sl := range slots {
		ps := &gamev1.HotbarSlot{Kind: sl.Kind, Ref: sl.Ref}
		if !sl.IsEmpty() {
			ps.DisplayName, ps.Description = s.resolveHotbarSlotDisplay(sl)
		}
		protoSlots[i] = ps
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{Slots: protoSlots},
		},
	}
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
