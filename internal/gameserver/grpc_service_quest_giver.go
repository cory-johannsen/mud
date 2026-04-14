package gameserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/npc"
	questpkg "github.com/cory-johannsen/mud/internal/game/quest"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// stubQuestGiverMessage is the message displayed by quest giver NPCs before
// the quests feature is fully implemented for their faction.
const stubQuestGiverMessage = "I've got work for you, but the time isn't right yet."

// HandleQuestGiverInteract is a no-op handler for quest giver NPC interactions.
// It returns the stub message for all quest giver types until the quests
// feature is fully implemented.
//
// Precondition: giverID must not be empty.
// Postcondition: Returns (stubQuestGiverMessage, nil) for any non-empty giverID;
// returns ("", error) for an empty giverID.
func HandleQuestGiverInteract(_ context.Context, giverID string, _ string) (string, error) {
	if giverID == "" {
		return "", fmt.Errorf("quest giver NPC ID must not be empty")
	}
	return stubQuestGiverMessage, nil
}

// findQuestGiverInRoom returns the first quest_giver NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findQuestGiverInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	if inst.NPCType != "quest_giver" {
		return nil, fmt.Sprintf("No one named %q here.", npcName)
	}
	return inst, ""
}

// handleTalkAccept accepts a quest on behalf of the player.
//
// Precondition: uid identifies an active player session; questID is non-empty.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleTalkAccept(uid, questID string) (*gamev1.ServerEvent, error) {
	if s.questSvc == nil {
		return messageEvent("Quest system not available."), nil
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	title, objDescs, err := s.questSvc.Accept(context.Background(), sess, sess.CharacterID, questID)
	if err != nil {
		return messageEvent(fmt.Sprintf("Cannot accept quest: %v", err)), nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Quest accepted: %s\n", title))
	for _, d := range objDescs {
		sb.WriteString(fmt.Sprintf("  - %s\n", d))
	}
	return messageEvent(strings.TrimRight(sb.String(), "\n")), nil
}

// handleTalk responds to a talk command, offering quests, handling deliver objectives,
// accepting quests, and falling back to placeholder dialog.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleTalk(uid string, req *gamev1.TalkRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findQuestGiverInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.QuestGiver == nil {
		return messageEvent("That NPC has no dialog configured."), nil
	}

	// Parse subcommand args.
	args := strings.TrimSpace(req.GetArgs())
	if args != "" {
		parts := strings.Fields(args)
		if len(parts) >= 2 && parts[0] == "accept" {
			return s.handleTalkAccept(uid, parts[1])
		}
	}

	// Process pending deliver objectives silently: remove items and record progress.
	// The updated quest state will be reflected in the QuestGiverView returned below.
	if s.questSvc != nil && sess.Backpack != nil {
		reg := s.questSvc.Registry()
		for qid, aq := range sess.ActiveQuests {
			def, ok := reg[qid]
			if !ok {
				continue
			}
			if def.GiverNPCID != inst.TemplateID {
				continue
			}
			for _, obj := range def.Objectives {
				if obj.Type != "deliver" {
					continue
				}
				if aq.ObjectiveProgress[obj.ID] >= obj.Quantity {
					continue
				}
				instances := sess.Backpack.FindByItemDefID(obj.ItemID)
				if len(instances) == 0 {
					continue
				}
				needed := obj.Quantity - aq.ObjectiveProgress[obj.ID]
				removed := 0
				for _, inst2 := range instances {
					if removed >= needed {
						break
					}
					take := inst2.Quantity
					if take > needed-removed {
						take = needed - removed
					}
					if err := sess.Backpack.Remove(inst2.InstanceID, take); err != nil {
						continue
					}
					removed += take
				}
				if removed > 0 {
					_, _ = s.questSvc.RecordDeliver(context.Background(), sess, sess.CharacterID, qid, obj.ID)
				}
			}
		}
	}

	// Return structured QuestGiverView so the frontend can display the quest modal.
	return s.buildQuestGiverView(uid, inst)
}

// handleQuestLog returns a QuestLogView containing all of the player's active quests.
//
// Precondition: uid identifies an active player session.
// Postcondition: Returns a non-nil ServerEvent wrapping QuestLogView; error is always nil.
func (s *GameServiceServer) handleQuestLog(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	reg := questpkg.QuestRegistry(nil)
	if s.questSvc != nil {
		reg = s.questSvc.Registry()
	}

	var entries []*gamev1.QuestEntryView
	for questID, aq := range sess.ActiveQuests {
		def, ok := reg[questID]
		if !ok {
			continue
		}
		entry := &gamev1.QuestEntryView{
			QuestId:       def.ID,
			Title:         def.Title,
			Description:   def.Description,
			XpReward:      int32(def.Rewards.XP),
			CreditsReward: int32(def.Rewards.Credits),
			Status:        "active",
		}
		for _, obj := range def.Objectives {
			progress := 0
			if aq.ObjectiveProgress != nil {
				progress = aq.ObjectiveProgress[obj.ID]
			}
			entry.Objectives = append(entry.Objectives, &gamev1.QuestObjectiveView{
				Id:          obj.ID,
				Description: obj.Description,
				Current:     int32(progress),
				Required:    int32(obj.Quantity),
			})
		}
		entries = append(entries, entry)
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_QuestLogView{
			QuestLogView: &gamev1.QuestLogView{
				Quests: entries,
			},
		},
	}, nil
}
