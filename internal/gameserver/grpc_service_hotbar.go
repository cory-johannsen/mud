package gameserver

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleHotbar processes hotbar commands: set, clear, show.
// For "set" and "clear", it returns a single MessageEvent and persists via charSaver.
// For "show", it returns nil and sends individual MessageEvent lines to the player's entity.
//
// Precondition: uid identifies a connected player; req is non-nil.
// Postcondition: On "set"/"clear", sess.Hotbar updated, SaveHotbar called, MessageEvent returned.
//
//	On "show", per-slot MessageEvents pushed to entity; nil returned.
//	On out-of-range slot, error MessageEvent returned with no side effects.
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
		sess.Hotbar[idx] = session.CommandSlot(req.Text)
		if s.charSaver != nil && sess.CharacterID > 0 {
			if err := s.charSaver.SaveHotbar(context.Background(), sess.CharacterID, sess.Hotbar); err != nil {
				s.logger.Warn("SaveHotbar failed", zap.String("uid", uid), zap.Error(err))
			}
		}
		return messageEvent(fmt.Sprintf("Slot %d set.", req.Slot)), nil

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
		return messageEvent(fmt.Sprintf("Slot %d cleared.", req.Slot)), nil

	case "show":
		for i := 0; i < 10; i++ {
			slotNum := i + 1
			display := "---"
			if cmd := sess.Hotbar[i].ActivationCommand(); cmd != "" {
				display = cmd
			}
			line := fmt.Sprintf("[%d] %s", slotNum, display)
			s.pushMessageToUID(uid, line)
		}
		return nil, nil

	default:
		return messageEvent(fmt.Sprintf("Unknown hotbar action '%s'. Usage: hotbar [<slot> <text>] | clear <slot>", req.Action)), nil
	}
}

// hotbarUpdateEvent builds a HotbarUpdateEvent from a [10]HotbarSlot array.
//
// Postcondition: Returns a non-nil ServerEvent with exactly 10 proto HotbarSlot entries.
func hotbarUpdateEvent(slots [10]session.HotbarSlot) *gamev1.ServerEvent {
	protoSlots := make([]*gamev1.HotbarSlot, 10)
	for i, v := range slots {
		protoSlots[i] = &gamev1.HotbarSlot{
			Kind: v.Kind,
			Ref:  v.Ref,
		}
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HotbarUpdate{
			HotbarUpdate: &gamev1.HotbarUpdateEvent{Slots: protoSlots},
		},
	}
}
