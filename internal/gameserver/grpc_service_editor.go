package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// handleSpawnNPC spawns a runtime-only NPC instance from a template. (REQ-EC-8,9)
//
// Precondition: uid session must exist; template_id must be non-empty.
// Postcondition: NPC instance created in target room. No YAML written.
func (s *GameServiceServer) handleSpawnNPC(uid string, req *gamev1.SpawnNPCRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}

	tmpl, ok := s.respawnMgr.GetTemplate(req.GetTemplateId())
	if !ok {
		return errorEvent(fmt.Sprintf("Unknown NPC template: %s.", req.GetTemplateId())), nil
	}

	roomID := req.GetRoomId()
	if roomID == "" {
		roomID = sess.RoomID
	}
	room, roomOk := s.world.GetRoom(roomID)
	if !roomOk {
		return errorEvent(fmt.Sprintf("Unknown room: %s.", roomID)), nil
	}

	if _, err := s.npcMgr.Spawn(tmpl, roomID); err != nil {
		return errorEvent(fmt.Sprintf("Failed to spawn NPC: %v", err)), nil
	}

	return messageEvent(fmt.Sprintf("Spawned %s in %s.", tmpl.Name, room.Title)), nil
}

// handleAddRoom adds a new room to a zone. (REQ-EC-17,18)
//
// Precondition: worldEditor must be non-nil; zone_id and room_id must be non-empty.
// Postcondition: New room added to zone YAML and hot-reloaded.
func (s *GameServiceServer) handleAddRoom(uid string, req *gamev1.AddRoomRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.worldEditor == nil {
		return errorEvent("world-editing is not available on this server"), nil
	}

	if err := s.worldEditor.AddRoom(req.GetZoneId(), req.GetRoomId(), req.GetTitle()); err != nil {
		return errorEvent(err.Error()), nil
	}
	return messageEvent(fmt.Sprintf("Room %s added to zone %s.", req.GetRoomId(), req.GetZoneId())), nil
}

// handleAddLink adds a bidirectional exit between two rooms. (REQ-EC-19,20,21)
//
// Precondition: worldEditor must be non-nil; from_room_id, direction, to_room_id must be non-empty.
// Postcondition: Exit added in affected zone YAML(s) and hot-reloaded.
func (s *GameServiceServer) handleAddLink(uid string, req *gamev1.AddLinkRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.worldEditor == nil {
		return errorEvent("world-editing is not available on this server"), nil
	}

	if err := s.worldEditor.AddLink(req.GetFromRoomId(), req.GetDirection(), req.GetToRoomId()); err != nil {
		return errorEvent(err.Error()), nil
	}
	return messageEvent(fmt.Sprintf("Linked %s %s ↔ %s.", req.GetFromRoomId(), req.GetDirection(), req.GetToRoomId())), nil
}

// handleRemoveLink removes a directional exit from a room. (REQ-EC-22,23)
//
// Precondition: worldEditor must be non-nil; room_id and direction must be non-empty.
// Postcondition: Exit removed in zone YAML and hot-reloaded.
func (s *GameServiceServer) handleRemoveLink(uid string, req *gamev1.RemoveLinkRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.worldEditor == nil {
		return errorEvent("world-editing is not available on this server"), nil
	}

	if err := s.worldEditor.RemoveLink(req.GetRoomId(), req.GetDirection()); err != nil {
		return errorEvent(err.Error()), nil
	}
	return messageEvent(fmt.Sprintf("Removed %s exit from %s.", req.GetDirection(), req.GetRoomId())), nil
}

// handleSetRoom sets a field on the editor's current room. (REQ-EC-24,25,26)
//
// Precondition: worldEditor must be non-nil; field must be one of title/description/danger_level.
// Postcondition: Room field updated in zone YAML, hot-reloaded, and updated display pushed to
// all players in the affected room when field is title or description.
func (s *GameServiceServer) handleSetRoom(uid string, req *gamev1.SetRoomRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}
	if s.worldEditor == nil {
		return errorEvent("world-editing is not available on this server"), nil
	}

	roomID := sess.RoomID
	if err := s.worldEditor.SetRoomField(roomID, req.GetField(), req.GetValue()); err != nil {
		return errorEvent(err.Error()), nil
	}

	// REQ-EC-26: push updated room display to all players in the affected room.
	if req.GetField() == "title" || req.GetField() == "description" {
		s.pushRoomViewToAllInRoom(roomID)
	}

	return messageEvent(fmt.Sprintf("Room %s %s updated.", roomID, req.GetField())), nil
}

// handleEditorCmds lists all CategoryEditor commands sorted alphabetically. (REQ-EC-27,28)
//
// Precondition: caller must have editor or admin role.
// Postcondition: Returns sorted list of all CategoryEditor commands with descriptions.
func (s *GameServiceServer) handleEditorCmds(uid string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if evt := requireEditor(sess); evt != nil {
		return evt, nil
	}

	allCmds := s.commands.Commands()
	var editorCmds []*command.Command
	for _, cmd := range allCmds {
		if cmd.Category == command.CategoryEditor {
			editorCmds = append(editorCmds, cmd)
		}
	}
	sort.Slice(editorCmds, func(i, j int) bool {
		return editorCmds[i].Name < editorCmds[j].Name
	})

	var lines []string
	lines = append(lines, "Editor commands:")
	for _, cmd := range editorCmds {
		lines = append(lines, fmt.Sprintf("  %-14s %s", cmd.Name, cmd.Help))
	}
	return messageEvent(strings.Join(lines, "\r\n")), nil
}

// handleSpawnChar creates a test character for the claude_player account. (REQ-EC-28)
//
// Precondition: uid session must exist; req.Name must be non-empty.
// Postcondition: A new character is created and persisted for the claude_player account,
// or an error event is returned if any precondition fails.
func (s *GameServiceServer) handleSpawnChar(uid string, req *gamev1.SpawnCharRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}
	if deny := requireEditor(sess); deny != nil {
		return deny, nil
	}
	if req.GetName() == "" {
		return messageEvent("Usage: spawn_char <name>"), nil
	}
	if s.accountAdmin == nil || s.charSaver == nil {
		return messageEvent("spawn_char: account or character repository not configured"), nil
	}
	charRepo, ok := s.charSaver.(*postgres.CharacterRepository)
	if !ok {
		return messageEvent("spawn_char: character repository does not support creation"), nil
	}
	ctx := context.Background()
	claudeAcct, err := s.accountAdmin.GetAccountByUsername(ctx, "claude_player")
	if err != nil {
		return messageEvent(fmt.Sprintf("spawn_char: claude_player account not found: %v", err)), nil
	}
	c := &character.Character{
		AccountID:  claudeAcct.ID,
		Name:       req.GetName(),
		Region:     "northeast",
		Class:      "gunslinger",
		Team:       "",
		Level:      1,
		Experience: 0,
		Location:   "battle_infirmary",
		Abilities: character.AbilityScores{
			Brutality: 10, Quickness: 10, Grit: 10,
			Reasoning: 10, Savvy: 10, Flair: 10,
		},
		MaxHP:     20,
		CurrentHP: 20,
		Gender:    "they/them",
	}
	created, err := charRepo.Create(ctx, c)
	if err != nil {
		return messageEvent(fmt.Sprintf("spawn_char: failed to create character %q: %v", req.GetName(), err)), nil
	}
	return messageEvent(fmt.Sprintf("Character %q created (id=%d) for claude_player.", created.Name, created.ID)), nil
}

// handleDeleteChar removes a character by name from the claude_player account. (REQ-EC-29)
//
// Precondition: uid session must exist; req.Name must be non-empty; character must belong to claude_player.
// Postcondition: The named character is permanently removed, or an error event is returned.
func (s *GameServiceServer) handleDeleteChar(uid string, req *gamev1.DeleteCharRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}
	if deny := requireEditor(sess); deny != nil {
		return deny, nil
	}
	if req.GetName() == "" {
		return messageEvent("Usage: delete_char <name>"), nil
	}
	if s.accountAdmin == nil || s.charSaver == nil {
		return messageEvent("delete_char: account or character repository not configured"), nil
	}
	charRepo, ok := s.charSaver.(*postgres.CharacterRepository)
	if !ok {
		return messageEvent("delete_char: character repository does not support deletion"), nil
	}
	ctx := context.Background()
	claudeAcct, err := s.accountAdmin.GetAccountByUsername(ctx, "claude_player")
	if err != nil {
		return messageEvent(fmt.Sprintf("delete_char: claude_player account not found: %v", err)), nil
	}
	if err := charRepo.DeleteByAccountAndName(ctx, claudeAcct.ID, req.GetName()); err != nil {
		return messageEvent(fmt.Sprintf("delete_char: failed to delete character %q: %v", req.GetName(), err)), nil
	}
	return messageEvent(fmt.Sprintf("Character %q deleted from claude_player.", req.GetName())), nil
}
