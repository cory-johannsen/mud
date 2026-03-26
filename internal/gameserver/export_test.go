package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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

// RequireEditor exposes requireEditor for white-box testing.
var RequireEditor = func(sess *session.PlayerSession) *gamev1.ServerEvent {
	return requireEditor(sess)
}

// RequireAdmin exposes requireAdmin for white-box testing.
var RequireAdmin = func(sess *session.PlayerSession) *gamev1.ServerEvent {
	return requireAdmin(sess)
}

// ApplyExploreModeOnCombatStartForTest is an exported test shim for applyExploreModeOnCombatStart.
func ApplyExploreModeOnCombatStartForTest(sess *session.PlayerSession, playerCbt *combat.Combatant, h *CombatHandler) []string {
	return applyExploreModeOnCombatStart(sess, playerCbt, h)
}
