package crafting

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RecipeMaterial struct {
	ID       string `yaml:"id"`
	Quantity int    `yaml:"quantity"`
}

type Recipe struct {
	ID                string           `yaml:"id"`
	Name              string           `yaml:"name"`
	OutputItemID      string           `yaml:"output_item_id"`
	OutputCount       int              `yaml:"output_count"`
	Category          string           `yaml:"category"`
	Complexity        int              `yaml:"complexity"`
	DC                int              `yaml:"dc"`
	QuickCraftMinRank string           `yaml:"quick_craft_min_rank"`
	Materials         []RecipeMaterial `yaml:"materials"`
	Description       string           `yaml:"description"`
}

// EffectiveMinRank returns the minimum proficiency rank required for quick crafting.
// Postcondition: returns a non-empty string in {"untrained","trained","expert","master"}.
func (r *Recipe) EffectiveMinRank() string {
	if r.QuickCraftMinRank != "" {
		return r.QuickCraftMinRank
	}
	switch r.Complexity {
	case 1:
		return "untrained"
	case 2:
		return "trained"
	case 3:
		return "expert"
	default:
		return "master"
	}
}

// DowntimeDays returns the number of downtime days required to craft this recipe.
func (r *Recipe) DowntimeDays() int {
	switch r.Complexity {
	case 2:
		return 1
	case 3:
		return 2
	default:
		return 4
	}
}

// RecipeRegistry provides O(1) lookup of recipes by ID.
type RecipeRegistry struct {
	recipes map[string]*Recipe
}

// OutputValidator validates that an output item ID is valid for a given category.
type OutputValidator interface {
	ValidateOutput(category, itemID string) error
}

// LoadRecipeRegistry reads all YAML files in dir, validates material IDs against matReg,
// and optionally validates output items against invReg.
// Precondition: dir must be a readable directory; matReg must not be nil.
// Postcondition: all recipe material IDs exist in matReg, or an error is returned (REQ-CRAFT-6).
func LoadRecipeRegistry(dir string, matReg *MaterialRegistry, invReg OutputValidator) (*RecipeRegistry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read recipes dir: %w", err)
	}
	reg := &RecipeRegistry{recipes: make(map[string]*Recipe)}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r Recipe
		if err := yaml.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parse recipe %s: %w", e.Name(), err)
		}
		for _, m := range r.Materials {
			if !matReg.HasID(m.ID) {
				return nil, fmt.Errorf("recipe %s: unknown material ID %q (REQ-CRAFT-6)", r.ID, m.ID)
			}
		}
		if invReg != nil {
			if err := invReg.ValidateOutput(r.Category, r.OutputItemID); err != nil {
				return nil, fmt.Errorf("recipe %s: %w (REQ-CRAFT-10)", r.ID, err)
			}
		}
		reg.recipes[r.ID] = &r
	}
	return reg, nil
}

// Recipe returns the Recipe with the given id and true, or nil and false if not found.
func (r *RecipeRegistry) Recipe(id string) (*Recipe, bool) {
	rec, ok := r.recipes[id]
	return rec, ok
}

// All returns all recipes in the registry in unspecified order.
func (r *RecipeRegistry) All() []*Recipe {
	out := make([]*Recipe, 0, len(r.recipes))
	for _, rec := range r.recipes {
		out = append(out, rec)
	}
	return out
}
