package gameserver

import (
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// WorldHandler handles movement, look, and exit commands.
type WorldHandler struct {
	world        *world.Manager
	sessions     *session.Manager
	npcMgr       *npc.Manager
	clock        *GameClock
	roomEquipMgr *inventory.RoomEquipmentManager
	invRegistry  *inventory.Registry
	combatH      *CombatHandler
}

// SetCombatHandler wires in the CombatHandler for NPC combat status lookup.
//
// Precondition: none (may be nil to disable combat status).
// Postcondition: h.combatH is set to combatH.
func (h *WorldHandler) SetCombatHandler(combatH *CombatHandler) {
	h.combatH = combatH
}

// NewWorldHandler creates a WorldHandler with the given dependencies.
//
// Precondition: worldMgr, sessMgr, and npcMgr must be non-nil. clock, roomEquipMgr, and invRegistry may be nil.
// Postcondition: Returns a non-nil *WorldHandler.
func NewWorldHandler(worldMgr *world.Manager, sessMgr *session.Manager, npcMgr *npc.Manager, clock *GameClock, roomEquipMgr *inventory.RoomEquipmentManager, invRegistry *inventory.Registry) *WorldHandler {
	return &WorldHandler{
		world:        worldMgr,
		sessions:     sessMgr,
		npcMgr:       npcMgr,
		clock:        clock,
		roomEquipMgr: roomEquipMgr,
		invRegistry:  invRegistry,
	}
}

// Move moves the player in the given direction and returns the new room view.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the new RoomView or an error if movement fails.
func (h *WorldHandler) Move(uid string, dir world.Direction) (*gamev1.RoomView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	dest, err := h.world.Navigate(sess.RoomID, dir)
	if err != nil {
		return nil, err
	}

	oldRoomID, err := h.sessions.MovePlayer(uid, dest.ID)
	if err != nil {
		return nil, fmt.Errorf("moving player: %w", err)
	}

	_ = oldRoomID // Used by caller for broadcasting departure/arrival

	return h.buildRoomView(uid, dest), nil
}

// MoveResult holds the result of a Move operation including the old room
// for broadcasting departure events.
type MoveResult struct {
	OldRoomID string
	View      *gamev1.RoomView
}

// MoveWithContext moves the player and returns both old room and new room view.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns MoveResult or an error if movement fails.
func (h *WorldHandler) MoveWithContext(uid string, dir world.Direction) (*MoveResult, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	dest, err := h.world.Navigate(sess.RoomID, dir)
	if err != nil {
		return nil, err
	}

	oldRoomID, err := h.sessions.MovePlayer(uid, dest.ID)
	if err != nil {
		return nil, fmt.Errorf("moving player: %w", err)
	}

	return &MoveResult{
		OldRoomID: oldRoomID,
		View:      h.buildRoomView(uid, dest),
	}, nil
}

// Look returns the current room view for the player.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the RoomView or an error if the player/room is not found.
func (h *WorldHandler) Look(uid string) (*gamev1.RoomView, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	room, ok := h.world.GetRoom(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("room %q not found", sess.RoomID)
	}

	return h.buildRoomView(uid, room), nil
}

// Exits returns the list of exits from the player's current room.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns the ExitList or an error.
func (h *WorldHandler) Exits(uid string) (*gamev1.ExitList, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	room, ok := h.world.GetRoom(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("room %q not found", sess.RoomID)
	}

	exitInfos := make([]*gamev1.ExitInfo, 0, len(room.Exits))
	for _, e := range room.Exits {
		info := &gamev1.ExitInfo{
			Direction:    string(e.Direction),
			TargetRoomId: e.TargetRoom,
			Locked:       e.Locked,
			Hidden:       e.Hidden,
		}
		if targetRoom, ok := h.world.GetRoom(e.TargetRoom); ok {
			info.TargetTitle = targetRoom.Title
		}
		exitInfos = append(exitInfos, info)
	}

	return &gamev1.ExitList{Exits: exitInfos}, nil
}

// buildRoomView constructs a RoomView proto from a Room, excluding the player themselves
// from the players list and including live NPC instances in the room.
func (h *WorldHandler) buildRoomView(uid string, room *world.Room) *gamev1.RoomView {
	sess, _ := h.sessions.GetPlayer(uid)
	var otherPlayers []string
	for _, p := range h.sessions.PlayersInRoomDetails(room.ID) {
		if sess != nil && p.CharName == sess.CharName {
			continue
		}
		name := p.CharName
		if p.Conditions != nil && condition.IsTargetingPrevented(p.Conditions) {
			name = name + " (detained)"
		}
		otherPlayers = append(otherPlayers, name)
	}

	visibleExits := room.VisibleExits()
	exitInfos := make([]*gamev1.ExitInfo, 0, len(visibleExits))
	for _, e := range visibleExits {
		info := &gamev1.ExitInfo{
			Direction:    string(e.Direction),
			TargetRoomId: e.TargetRoom,
			Locked:       e.Locked,
			Hidden:       e.Hidden,
		}
		if targetRoom, ok := h.world.GetRoom(e.TargetRoom); ok {
			info.TargetTitle = targetRoom.Title
		}
		exitInfos = append(exitInfos, info)
	}

	instances := h.npcMgr.InstancesInRoom(room.ID)
	npcInfos := make([]*gamev1.NpcInfo, 0, len(instances))
	for _, inst := range instances {
		if !inst.IsDead() {
			fightingTarget := ""
			var condNames []string
			if h.combatH != nil {
				fightingTarget = h.combatH.FightingTargetName(inst.ID)
				// Precondition: uid is the viewing player's UID (same room).
				if activeSet, ok := h.combatH.GetCombatConditionSet(uid, inst.ID); ok {
					for _, ac := range activeSet.All() {
						condNames = append(condNames, ac.Def.Name)
					}
				}
			}
			npcInfos = append(npcInfos, &gamev1.NpcInfo{
				InstanceId:        inst.ID,
				Name:              inst.Name(),
				HealthDescription: inst.HealthDescription(),
				FightingTarget:    fightingTarget,
				Conditions:        condNames,
				NpcType:           inst.NPCType,
			})
		}
	}

	// Time of day
	hour := GameHour(6) // default to dawn if no clock
	if h.clock != nil {
		hour = h.clock.CurrentHour()
	}
	isOutdoor := room.Properties["outdoor"] == "true"
	period := hour.Period()

	// In dark periods, outdoor rooms hide exits
	if IsDarkPeriod(period) && isOutdoor {
		exitInfos = nil
	}

	description := room.Description
	if flavor := FlavorText(period, isOutdoor); flavor != "" {
		description = description + " " + flavor
	}

	var equipInfos []*gamev1.RoomEquipmentItem
	if h.roomEquipMgr != nil {
		for _, eq := range h.roomEquipMgr.EquipmentInRoom(room.ID) {
			name := eq.ItemDefID
			if h.invRegistry != nil {
				if def, ok := h.invRegistry.Item(eq.ItemDefID); ok {
					name = def.Name
				}
			}
			equipInfos = append(equipInfos, &gamev1.RoomEquipmentItem{
				InstanceId: eq.InstanceID,
				Name:       name,
				Quantity:   1,
				Immovable:  eq.Immovable,
				Usable:     eq.Script != "",
			})
		}
	}

	var zoneName string
	if zone, ok := h.world.GetZone(room.ZoneID); ok {
		zoneName = zone.Name
	}

	return &gamev1.RoomView{
		RoomId:      room.ID,
		Title:       room.Title,
		Description: description,
		Exits:       exitInfos,
		Players:     otherPlayers,
		Npcs:        npcInfos,
		Hour:        int32(hour),
		Period:      string(period),
		Equipment:   equipInfos,
		ZoneName:    zoneName,
	}
}
