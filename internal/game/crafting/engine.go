package crafting

import "context"

// Outcome represents the degree of success for a crafting check.
type Outcome int

const (
	CritSuccess Outcome = iota
	Success
	Failure
	CritFailure
)

// CraftResult holds the outcome of a quick-craft attempt.
type CraftResult struct {
	OutputQuantity    int
	MaterialsDeducted map[string]int
	MaterialsConsumed bool // true when any materials are deducted
}

// DowntimeCraftStarter begins a downtime crafting activity for a character.
type DowntimeCraftStarter interface {
	BeginCraftActivity(ctx context.Context, characterID int64, recipeID string, daysRequired int) error
}

// CraftingEngine executes crafting logic as a pure functional core.
type CraftingEngine struct{}

// NewEngine returns a new CraftingEngine.
func NewEngine() *CraftingEngine { return &CraftingEngine{} }

// ExecuteQuickCraft applies PF2E quick-craft outcome rules.
// Precondition: recipe must not be nil.
// Postcondition: CritSuccess yields output_count+1; Success yields output_count;
// Failure yields 0 with half materials; CritFailure yields 0 with all materials consumed.
// ExecuteQuickCraft applies PF2E quick-craft outcome rules.
// MaterialsConsumed is true when any player materials are present (consumed on success/crit-success/crit-failure)
// or partially consumed (failure). For Failure and CritFailure with no recipe materials defined,
// MaterialsConsumed reflects whether playerMaterials were provided.
func (e *CraftingEngine) ExecuteQuickCraft(recipe *Recipe, playerMaterials map[string]int, outcome Outcome) CraftResult {
	switch outcome {
	case CritSuccess:
		d := fullDeduction(recipe)
		consumed := len(d) > 0 || len(playerMaterials) > 0
		return CraftResult{OutputQuantity: recipe.OutputCount + 1, MaterialsDeducted: d, MaterialsConsumed: consumed}
	case Success:
		d := fullDeduction(recipe)
		return CraftResult{OutputQuantity: recipe.OutputCount, MaterialsDeducted: d, MaterialsConsumed: len(d) > 0}
	case Failure:
		d := halfDeduction(recipe)
		return CraftResult{OutputQuantity: 0, MaterialsDeducted: d, MaterialsConsumed: len(d) > 0}
	default: // CritFailure
		d := fullDeduction(recipe)
		return CraftResult{OutputQuantity: 0, MaterialsDeducted: d, MaterialsConsumed: len(d) > 0}
	}
}

func fullDeduction(r *Recipe) map[string]int {
	d := make(map[string]int, len(r.Materials))
	for _, m := range r.Materials {
		d[m.ID] = m.Quantity
	}
	return d
}

func halfDeduction(r *Recipe) map[string]int {
	d := make(map[string]int, len(r.Materials))
	for _, m := range r.Materials {
		d[m.ID] = m.Quantity / 2
	}
	return d
}
