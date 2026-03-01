// Package inventory provides definitions and loaders for weapons, armor, and related gear
// used in the MUD game engine.
package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CrossTeamEffect describes the mechanical consequence of equipping rival-team gear.
type CrossTeamEffect struct {
	Kind  string `yaml:"kind"`  // "condition" or "penalty"
	Value string `yaml:"value"` // condition ID or penalty magnitude
}

// ArmorDef defines the static properties of an armor piece loaded from YAML.
type ArmorDef struct {
	ID              string           `yaml:"id"`
	Name            string           `yaml:"name"`
	Description     string           `yaml:"description"`
	Slot            ArmorSlot        `yaml:"slot"`
	ACBonus         int              `yaml:"ac_bonus"`
	DexCap          int              `yaml:"dex_cap"`
	CheckPenalty    int              `yaml:"check_penalty"`    // non-positive; 0 = none
	SpeedPenalty    int              `yaml:"speed_penalty"`    // non-negative feet reduction
	StrengthReq     int              `yaml:"strength_req"`
	Bulk            int              `yaml:"bulk"`
	Group           string           `yaml:"group"`
	Traits          []string         `yaml:"traits"`
	TeamAffinity    string           `yaml:"team_affinity"`    // "gun", "machete", or ""
	CrossTeamEffect *CrossTeamEffect `yaml:"cross_team_effect"`
}

// validArmorSlots is the set of all legal ArmorSlot values.
var validArmorSlots = map[ArmorSlot]struct{}{
	SlotHead:     {},
	SlotTorso:    {},
	SlotLeftArm:  {},
	SlotRightArm: {},
	SlotHands:    {},
	SlotLeftLeg:  {},
	SlotRightLeg: {},
	SlotFeet:     {},
}

// ValidArmorSlots returns the set of all legal ArmorSlot values.
// Postcondition: Returns a non-nil map containing all 8 valid slot constants.
func ValidArmorSlots() map[ArmorSlot]struct{} { return validArmorSlots }

// Validate reports an error if the ArmorDef is missing required fields or contains illegal values.
// Precondition: def is non-nil.
// Postcondition: Returns nil iff the def is well-formed.
func (a *ArmorDef) Validate() error {
	var errs []error
	if a.ID == "" {
		errs = append(errs, errors.New("id must not be empty"))
	}
	if a.Name == "" {
		errs = append(errs, errors.New("name must not be empty"))
	}
	if _, ok := validArmorSlots[a.Slot]; !ok {
		errs = append(errs, fmt.Errorf("slot %q is not a valid armor slot", a.Slot))
	}
	if a.ACBonus < 0 {
		errs = append(errs, errors.New("ac_bonus must be >= 0"))
	}
	if a.CheckPenalty > 0 {
		errs = append(errs, errors.New("check_penalty must be <= 0"))
	}
	if a.SpeedPenalty < 0 {
		errs = append(errs, errors.New("speed_penalty must be >= 0"))
	}
	if a.Group == "" {
		errs = append(errs, errors.New("group must not be empty"))
	}
	if a.CrossTeamEffect != nil {
		if a.CrossTeamEffect.Kind != "condition" && a.CrossTeamEffect.Kind != "penalty" {
			errs = append(errs, fmt.Errorf("cross_team_effect.kind %q must be \"condition\" or \"penalty\"", a.CrossTeamEffect.Kind))
		}
		if a.CrossTeamEffect.Value == "" {
			errs = append(errs, errors.New("cross_team_effect.value must not be empty"))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("armor validation failed: %v", errs)
	}
	return nil
}

// LoadArmors reads all .yaml files in dir and returns parsed ArmorDef slice.
// Precondition: dir must be a readable directory.
// Postcondition: Returns non-nil slice and nil error on success; all returned defs pass Validate.
func LoadArmors(dir string) ([]*ArmorDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadArmors: cannot read directory %q: %w", dir, err)
	}

	var armors []*ArmorDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadArmors: cannot read file %q: %w", path, err)
		}
		var a ArmorDef
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("LoadArmors: cannot parse file %q: %w", path, err)
		}
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("LoadArmors: invalid armor in %q: %w", path, err)
		}
		armors = append(armors, &a)
	}
	if armors == nil {
		armors = []*ArmorDef{}
	}
	return armors, nil
}
