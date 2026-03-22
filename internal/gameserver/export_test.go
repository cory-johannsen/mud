package gameserver

import (
	"github.com/cory-johannsen/mud/internal/game/ai"
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
