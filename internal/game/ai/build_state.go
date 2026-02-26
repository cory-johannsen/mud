package ai

import (
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// BuildCombatWorldState constructs a WorldState snapshot from an active Combat
// for the NPC identified by inst.
//
// Precondition: cbt and inst must not be nil.
// Postcondition: ws.NPC.UID == inst.ID; all combatants are represented.
func BuildCombatWorldState(cbt *combat.Combat, inst *npc.Instance, zoneID string) *WorldState {
	ws := &WorldState{
		NPC: &NPCState{
			UID:        inst.ID,
			Name:       inst.Name,
			Kind:       "npc",
			HP:         inst.CurrentHP,
			MaxHP:      inst.MaxHP,
			Perception: inst.Perception,
			ZoneID:     zoneID,
			RoomID:     cbt.RoomID,
		},
		Room: &RoomState{
			ID:     cbt.RoomID,
			ZoneID: zoneID,
		},
	}
	for _, c := range cbt.Combatants {
		kind := "npc"
		if c.Kind == combat.KindPlayer {
			kind = "player"
		}
		ws.Combatants = append(ws.Combatants, &CombatantState{
			UID:   c.ID,
			Name:  c.Name,
			Kind:  kind,
			HP:    c.CurrentHP,
			MaxHP: c.MaxHP,
			AC:    c.AC,
			Dead:  c.CurrentHP <= 0 || c.Dead,
		})
	}
	return ws
}
