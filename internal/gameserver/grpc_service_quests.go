package gameserver

import (
	"context"
	"fmt"
	"strings"

	sessionpkg "github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleQuestCommand dispatches quest subcommands: list, log, abandon, completed.
//
// Precondition: uid identifies an active player session; args is the subcommand string.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleQuestCommand(uid, args string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	if s.questSvc == nil {
		return messageEvent("Quest system not available."), nil
	}
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return messageEvent("Usage: quest list | quest log <id> | quest abandon <id> [confirm] | quest completed"), nil
	}
	switch parts[0] {
	case "list":
		return s.questList(sess), nil
	case "log":
		if len(parts) < 2 {
			return messageEvent("Usage: quest log <id>"), nil
		}
		return s.questLog(sess, parts[1]), nil
	case "abandon":
		if len(parts) < 2 {
			return messageEvent("Usage: quest abandon <id> [confirm]"), nil
		}
		questID := parts[1]
		confirm := len(parts) >= 3 && parts[2] == "confirm"
		msg, err := s.questSvc.Abandon(context.Background(), sess, sess.CharacterID, questID, confirm)
		if err != nil {
			return messageEvent(fmt.Sprintf("Error: %v", err)), nil
		}
		return messageEvent(msg), nil
	case "completed":
		return s.questCompleted(sess), nil
	default:
		return messageEvent("Unknown quest subcommand."), nil
	}
}

// questList formats active quests with objective completion ratios.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns a non-nil ServerEvent with active quest list or empty message.
func (s *GameServiceServer) questList(sess *sessionpkg.PlayerSession) *gamev1.ServerEvent {
	if len(sess.ActiveQuests) == 0 {
		return messageEvent("You have no active quests.")
	}
	reg := s.questSvc.Registry()
	var sb strings.Builder
	for qid, aq := range sess.ActiveQuests {
		def, ok := reg[qid]
		if !ok {
			continue
		}
		done := 0
		for _, obj := range def.Objectives {
			if aq.ObjectiveProgress[obj.ID] >= obj.Quantity {
				done++
			}
		}
		sb.WriteString(fmt.Sprintf("[%d/%d] %s — %s\n", done, len(def.Objectives), qid, def.Title))
	}
	return messageEvent(strings.TrimRight(sb.String(), "\n"))
}

// questLog shows full detail for an active or completed quest.
//
// Precondition: sess must be non-nil; questID must be non-empty.
// Postcondition: Returns a non-nil ServerEvent with quest detail or not-found message.
func (s *GameServiceServer) questLog(sess *sessionpkg.PlayerSession, questID string) *gamev1.ServerEvent {
	reg := s.questSvc.Registry()
	def, ok := reg[questID]
	if !ok {
		return messageEvent("No quest found with that ID.")
	}
	var sb strings.Builder
	if completedAt, done := sess.CompletedQuests[questID]; done && completedAt != nil {
		sb.WriteString(fmt.Sprintf("(Completed: %s)\n", completedAt.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("%s\n%s\n", def.Title, def.Description))
		for _, obj := range def.Objectives {
			sb.WriteString(fmt.Sprintf("[x] %s (%d/%d)\n", obj.Description, obj.Quantity, obj.Quantity))
		}
		return messageEvent(strings.TrimRight(sb.String(), "\n"))
	}
	if aq, active := sess.ActiveQuests[questID]; active {
		sb.WriteString(fmt.Sprintf("%s\n%s\n", def.Title, def.Description))
		for _, obj := range def.Objectives {
			prog := aq.ObjectiveProgress[obj.ID]
			check := "[ ]"
			if prog >= obj.Quantity {
				check = "[x]"
			}
			sb.WriteString(fmt.Sprintf("%s %s (%d/%d)\n", check, obj.Description, prog, obj.Quantity))
		}
		sb.WriteString(fmt.Sprintf("Rewards: %d XP", def.Rewards.XP))
		if def.Rewards.Credits > 0 {
			sb.WriteString(fmt.Sprintf(", %d credits", def.Rewards.Credits))
		}
		for _, ri := range def.Rewards.Items {
			name := ri.ItemID
			if s.invRegistry != nil {
				if d, ok2 := s.invRegistry.Item(ri.ItemID); ok2 {
					name = d.Name
				}
			}
			sb.WriteString(fmt.Sprintf(", %dx %s", ri.Quantity, name))
		}
		return messageEvent(strings.TrimRight(sb.String(), "\n"))
	}
	return messageEvent("No quest found with that ID.")
}

// questCompleted lists completed quests with timestamps.
//
// Precondition: sess must be non-nil.
// Postcondition: Returns a non-nil ServerEvent listing completed quests or empty message.
func (s *GameServiceServer) questCompleted(sess *sessionpkg.PlayerSession) *gamev1.ServerEvent {
	reg := s.questSvc.Registry()
	var sb strings.Builder
	found := false
	for qid, completedAt := range sess.CompletedQuests {
		if completedAt == nil {
			continue
		}
		def, ok := reg[qid]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s — %s (completed %s)\n", qid, def.Title, completedAt.Format("2006-01-02 15:04:05")))
		found = true
	}
	if !found {
		return messageEvent("You have not completed any quests.")
	}
	return messageEvent(strings.TrimRight(sb.String(), "\n"))
}
