package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// ExportedBuildOptions exposes buildOptions for white-box testing.
func ExportedBuildOptions(ids []string, levels []int, reg *technology.Registry) []string {
	return buildOptions(ids, levels, reg)
}

// ExportedParseTechID exposes parseTechID for white-box testing.
func ExportedParseTechID(option string) string {
	return parseTechID(option)
}

// ExportedFilterAnimalPlanActions exposes FilterAnimalPlanActions for white-box testing.
func ExportedFilterAnimalPlanActions(actions []ai.PlannedAction, isAnimal bool) []ai.PlannedAction {
	return FilterAnimalPlanActions(actions, isAnimal)
}

// ExportedFleeNPCImmobile calls fleeNPCLocked on an immobile NPC with a nil room
// (which would cause a panic if immobility is not checked first). Returns true iff
// the instance's RoomID changed (which must not happen for immobile NPCs).
func ExportedFleeNPCImmobile(inst *npc.Instance) bool {
	before := inst.RoomID
	h := &CombatHandler{}
	h.fleeNPCLocked(inst, nil)
	return inst.RoomID != before
}
