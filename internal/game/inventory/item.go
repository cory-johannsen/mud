package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Kind constants for ItemDef.Kind.
const (
	KindWeapon           = "weapon"
	KindExplosive        = "explosive"
	KindConsumable       = "consumable"
	KindJunk             = "junk"
	KindArmor            = "armor"
	KindTrap             = "trap"
	KindPreciousMaterial = "precious_material"
)

// validKinds is the set of valid ItemDef kinds.
var validKinds = map[string]bool{
	KindWeapon:           true,
	KindExplosive:        true,
	KindConsumable:       true,
	KindJunk:             true,
	KindArmor:            true,
	KindTrap:             true,
	KindPreciousMaterial: true,
}

// ItemDef defines the static properties of an inventory item loaded from YAML.
type ItemDef struct {
	ID           string  `yaml:"id"`
	Name         string  `yaml:"name"`
	Description  string  `yaml:"description"`
	Kind         string  `yaml:"kind"`
	Weight       float64 `yaml:"weight"`
	WeaponRef    string  `yaml:"weapon_ref"`
	ArmorRef     string  `yaml:"armor_ref"`    // references an ArmorDef ID; set when Kind == "armor"
	ExplosiveRef    string  `yaml:"explosive_ref"`
	TrapTemplateRef string  `yaml:"trap_template_ref"` // references a TrapTemplate ID; set when Kind == "trap"
	Stackable       bool    `yaml:"stackable"`
	MaxStack     int     `yaml:"max_stack"`
	Value        int     `yaml:"value"`
	// Team is optional; "gun" | "machete" | "". Applies a team effectiveness multiplier
	// when used as a consumable (REQ-EM-36 through REQ-EM-39).
	Team   string           `yaml:"team,omitempty"`
	// Effect holds consumable effect data; nil for non-consumable items.
	Effect *ConsumableEffect `yaml:"effect,omitempty"`
	// SubstanceID is the substance ID applied when this consumable item is used.
	// Only meaningful when Kind == KindConsumable. Empty for non-substance consumables.
	SubstanceID string `yaml:"substance_id,omitempty"`
	// PoisonSubstanceID is the substance ID applied to a target on a successful weapon hit.
	// Only meaningful when Kind == KindWeapon. Empty for non-poisoned weapons.
	// REQ-AH-21: attack pipeline calls ApplySubstanceByID when this is non-empty.
	PoisonSubstanceID string `yaml:"poison_substance_id,omitempty"`
	// ActivationCost is the AP cost to activate this item (1–3). 0 = not activatable (default).
	ActivationCost int `yaml:"activation_cost,omitempty"`
	// Charges is the initial and maximum charge count. Must be > 0 when ActivationCost > 0.
	Charges int `yaml:"charges,omitempty"`
	// OnDeplete controls item fate when ChargesRemaining reaches 0.
	// "destroy" removes the item permanently; "expend" leaves it in slot as inactive.
	// Ignored when Recharge is non-empty (expend semantics always apply).
	OnDeplete string `yaml:"on_deplete,omitempty"`
	// ActivationScript is the Lua hook name to invoke on activation.
	// Mutually exclusive with ActivationEffect.
	ActivationScript string `yaml:"activation_script,omitempty"`
	// ActivationEffect holds consumable-style effects for non-scripted activation.
	// Mutually exclusive with ActivationScript.
	ActivationEffect *ConsumableEffect `yaml:"activation_effect,omitempty"`
	// Recharge lists triggers that restore charges to this item.
	Recharge []RechargeEntry `yaml:"recharge,omitempty"`
	// MaterialID is the base material identifier. Required when Kind == KindPreciousMaterial.
	MaterialID string `yaml:"material_id,omitempty"`
	// GradeID is the material grade. Required when Kind == KindPreciousMaterial.
	// Valid values: street_grade, mil_spec_grade, ghost_grade.
	GradeID string `yaml:"grade_id,omitempty"`
	// MaterialName is the human-readable material name. Required when Kind == KindPreciousMaterial.
	MaterialName string `yaml:"material_name,omitempty"`
	// MaterialTier is the rarity tier of the material. Required when Kind == KindPreciousMaterial.
	// Valid values: common, uncommon, rare.
	MaterialTier string `yaml:"material_tier,omitempty"`
	// AppliesTo lists the item categories this material can be applied to.
	// Required when Kind == KindPreciousMaterial. Valid values: weapon, armor.
	AppliesTo []string `yaml:"applies_to,omitempty"`
}

// RechargeEntry defines one recharge trigger for an activatable item.
type RechargeEntry struct {
	// Trigger is when this recharge fires.
	// Valid values: "daily", "midnight", "dawn", "rest".
	Trigger string `yaml:"trigger"`
	// Amount is the number of charges restored. Must be > 0.
	Amount int `yaml:"amount"`
}

// Validate checks that the ItemDef satisfies its invariants.
//
// Precondition: d is non-nil.
// Postcondition: returns nil iff all fields are valid.
func (d *ItemDef) Validate() error {
	var errs []error
	if d.ID == "" {
		errs = append(errs, errors.New("ID must not be empty"))
	}
	if d.Name == "" {
		errs = append(errs, errors.New("Name must not be empty"))
	}
	if !validKinds[d.Kind] {
		errs = append(errs, fmt.Errorf("Kind must be one of weapon, explosive, consumable, junk, armor, trap, precious_material; got %q", d.Kind))
	}
	if d.MaxStack < 1 {
		errs = append(errs, errors.New("MaxStack must be >= 1"))
	}
	if d.Weight < 0 {
		errs = append(errs, errors.New("Weight must be >= 0"))
	}
	if d.Kind == KindWeapon && d.WeaponRef == "" {
		errs = append(errs, errors.New("WeaponRef is required when Kind is weapon"))
	}
	if d.Kind == KindExplosive && d.ExplosiveRef == "" {
		errs = append(errs, errors.New("ExplosiveRef is required when Kind is explosive"))
	}
	if d.Kind == KindArmor && d.ArmorRef == "" {
		errs = append(errs, errors.New("ArmorRef is required when Kind is armor"))
	}
	if d.Kind == KindTrap && d.TrapTemplateRef == "" {
		errs = append(errs, fmt.Errorf("item %q: TrapTemplateRef is required when Kind is %q", d.ID, KindTrap))
	}
	if d.Kind == KindPreciousMaterial {
		if d.MaterialID == "" {
			errs = append(errs, fmt.Errorf("material_id is required for precious_material kind"))
		}
		if d.GradeID == "" {
			errs = append(errs, fmt.Errorf("grade_id is required for precious_material kind"))
		} else if d.GradeID != "street_grade" && d.GradeID != "mil_spec_grade" && d.GradeID != "ghost_grade" {
			errs = append(errs, fmt.Errorf("grade_id %q is invalid; must be street_grade, mil_spec_grade, or ghost_grade", d.GradeID))
		}
		if d.MaterialName == "" {
			errs = append(errs, fmt.Errorf("material_name is required for precious_material kind"))
		}
		if d.MaterialTier == "" {
			errs = append(errs, fmt.Errorf("material_tier is required for precious_material kind"))
		} else if d.MaterialTier != "common" && d.MaterialTier != "uncommon" && d.MaterialTier != "rare" {
			errs = append(errs, fmt.Errorf("material_tier %q is invalid; must be common, uncommon, or rare", d.MaterialTier))
		}
		if len(d.AppliesTo) == 0 {
			errs = append(errs, fmt.Errorf("applies_to is required for precious_material kind"))
		}
		for _, at := range d.AppliesTo {
			if at != "weapon" && at != "armor" {
				errs = append(errs, fmt.Errorf("applies_to value %q is invalid; must be weapon or armor", at))
			}
		}
	}
	// REQ-EM-36: Team must be "" | "gun" | "machete".
	if d.Team != "" && d.Team != "gun" && d.Team != "machete" {
		errs = append(errs, fmt.Errorf("Team %q is not valid; must be \"gun\", \"machete\", or \"\"", d.Team))
	}
	// REQ-ACT-6: ActivationCost must be in [0, 3].
	if d.ActivationCost < 0 || d.ActivationCost > 3 {
		errs = append(errs, fmt.Errorf("activation_cost %d is out of range [0, 3]", d.ActivationCost))
	}
	// REQ-ACT-7: Charges must be > 0 when ActivationCost > 0.
	if d.ActivationCost > 0 && d.Charges <= 0 {
		errs = append(errs, fmt.Errorf("charges must be > 0 when activation_cost > 0 (got %d)", d.Charges))
	}
	// REQ-ACT-8: OnDeplete must be "", "destroy", or "expend".
	if d.OnDeplete != "" && d.OnDeplete != "destroy" && d.OnDeplete != "expend" {
		errs = append(errs, fmt.Errorf("on_deplete %q is invalid; must be \"destroy\" or \"expend\"", d.OnDeplete))
	}
	// REQ-ACT-9: ActivationScript and ActivationEffect are mutually exclusive.
	if d.ActivationScript != "" && d.ActivationEffect != nil {
		errs = append(errs, fmt.Errorf("activation_script and activation_effect are mutually exclusive"))
	}
	validTriggers := map[string]bool{"daily": true, "midnight": true, "dawn": true, "rest": true}
	for i, re := range d.Recharge {
		// REQ-ACT-10: Recharge trigger must be one of daily|midnight|dawn|rest.
		if !validTriggers[re.Trigger] {
			errs = append(errs, fmt.Errorf("recharge[%d].trigger %q is invalid; must be daily|midnight|dawn|rest", i, re.Trigger))
		}
		// REQ-ACT-11: Recharge amount must be > 0.
		if re.Amount <= 0 {
			errs = append(errs, fmt.Errorf("recharge[%d].amount must be > 0 (got %d)", i, re.Amount))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("item validation failed: %v", errs)
	}
	return nil
}

// requiredConsumableIDs lists the six consumable item IDs that MUST be present
// at startup (REQ-EM-40).
var requiredConsumableIDs = []string{
	"whores_pasta",
	"poontangesca",
	"four_loko",
	"old_english",
	"penjamin_franklin",
	"repair_kit",
}

// ValidateRequiredConsumables checks that all six required consumable item IDs
// are present in items (REQ-EM-40). Missing IDs produce a fatal error.
//
// Precondition: items may be nil (treated as empty).
// Postcondition: returns nil iff all required IDs are present.
func ValidateRequiredConsumables(items []*ItemDef) error {
	present := make(map[string]bool, len(items))
	for _, it := range items {
		present[it.ID] = true
	}
	var missing []string
	for _, id := range requiredConsumableIDs {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("LoadItems: required consumable item(s) missing: %v", missing)
	}
	return nil
}

// ValidateConsumableEffects checks that all disease_id and toxin_id values
// referenced in consumable effects appear in knownConditions (REQ-EM-42).
// An unresolvable ID is a fatal load error.
//
// Precondition: knownConditions may be nil (treated as empty — all IDs unknown).
// Postcondition: returns nil iff all referenced IDs are present in knownConditions.
func ValidateConsumableEffects(items []*ItemDef, knownConditions map[string]bool) error {
	for _, it := range items {
		if it.Effect == nil {
			continue
		}
		cc := it.Effect.ConsumeCheck
		if cc == nil || cc.OnCriticalFailure == nil {
			continue
		}
		cf := cc.OnCriticalFailure
		if cf.ApplyDisease != nil {
			id := cf.ApplyDisease.DiseaseID
			if !knownConditions[id] {
				return fmt.Errorf("item %q: disease_id %q is not a known condition ID (REQ-EM-42)", it.ID, id)
			}
		}
		if cf.ApplyToxin != nil {
			id := cf.ApplyToxin.ToxinID
			if !knownConditions[id] {
				return fmt.Errorf("item %q: toxin_id %q is not a known condition ID (REQ-EM-42)", it.ID, id)
			}
		}
	}
	return nil
}

// LoadItems reads all *.yaml and *.yml files from dir, parses each as an
// ItemDef, validates it, and returns the collected slice.
//
// Precondition: dir is a readable directory path.
// Postcondition: returns all valid ItemDefs or the first encountered error.
func LoadItems(dir string) ([]*ItemDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadItems: cannot read directory %q: %w", dir, err)
	}

	var items []*ItemDef
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadItems: cannot read file %q: %w", path, err)
		}
		var d ItemDef
		if err := yaml.Unmarshal(data, &d); err != nil {
			return nil, fmt.Errorf("LoadItems: cannot parse file %q: %w", path, err)
		}
		if err := d.Validate(); err != nil {
			return nil, fmt.Errorf("LoadItems: invalid item in %q: %w", path, err)
		}
		items = append(items, &d)
	}
	return items, nil
}
