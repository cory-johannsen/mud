// Package substance implements substance definitions, a registry, and active substance tracking.
package substance

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ValidCategories is the set of accepted substance category values.
var ValidCategories = map[string]bool{
	"drug": true, "alcohol": true, "medicine": true, "poison": true, "toxin": true,
	"stimulant": true,
}

// SubstanceEffect describes one effect applied at onset.
// Exactly one action field must be non-zero.
type SubstanceEffect struct {
	ApplyCondition  string   `yaml:"apply_condition,omitempty"`
	Stacks          int      `yaml:"stacks,omitempty"`
	RemoveCondition string   `yaml:"remove_condition,omitempty"`
	HPRegen         int      `yaml:"hp_regen,omitempty"`
	CureConditions  []string `yaml:"cure_conditions,omitempty"`
	// Attribute-modifier effects adjust a named character attribute by Modifier.
	Attribute string `yaml:"attribute,omitempty"`
	Modifier  int    `yaml:"modifier,omitempty"`
}

// SubstanceDef is the static definition of a substance loaded from YAML.
type SubstanceDef struct {
	ID                   string            `yaml:"id"`
	Name                 string            `yaml:"name"`
	Category             string            `yaml:"category"`
	OnsetDelayStr        string            `yaml:"onset_delay"`
	DurationStr          string            `yaml:"duration"`
	Effects              []SubstanceEffect `yaml:"effects"`
	RemoveOnExpire       []string          `yaml:"remove_on_expire"`
	Addictive            bool              `yaml:"addictive"`
	AddictionPotential   string            `yaml:"addiction_potential,omitempty"`
	AddictionChance      float64           `yaml:"addiction_chance"`
	OverdoseThreshold    int               `yaml:"overdose_threshold"`
	OverdoseCondition    string            `yaml:"overdose_condition"`
	WithdrawalConditions []string          `yaml:"withdrawal_conditions"`
	WithdrawalEffects    []SubstanceEffect `yaml:"withdrawal_effects,omitempty"`
	RecoveryDurStr       string            `yaml:"recovery_duration"`

	// Parsed durations — populated by Validate().
	OnsetDelay       time.Duration `yaml:"-"`
	Duration         time.Duration `yaml:"-"`
	RecoveryDuration time.Duration `yaml:"-"`
}

// Validate parses duration strings and checks all invariants.
//
// Postcondition: returns nil iff all fields are valid; sets OnsetDelay, Duration, RecoveryDuration.
func (d *SubstanceDef) Validate() error {
	var errs []error
	if d.ID == "" {
		errs = append(errs, errors.New("id must not be empty"))
	}
	if d.Name == "" {
		errs = append(errs, errors.New("name must not be empty"))
	}
	if !ValidCategories[d.Category] {
		errs = append(errs, fmt.Errorf("category must be one of drug|alcohol|medicine|poison|toxin|stimulant, got %q", d.Category))
	}
	// REQ-AH-26: medicine may not be addictive.
	if d.Category == "medicine" && d.Addictive {
		errs = append(errs, errors.New("medicine substances must not be addictive"))
	}
	var err error
	if d.OnsetDelay, err = time.ParseDuration(d.OnsetDelayStr); err != nil {
		errs = append(errs, fmt.Errorf("invalid onset_delay %q: %w", d.OnsetDelayStr, err))
	}
	if d.Duration, err = time.ParseDuration(d.DurationStr); err != nil {
		errs = append(errs, fmt.Errorf("invalid duration %q: %w", d.DurationStr, err))
	}
	if d.RecoveryDuration, err = time.ParseDuration(d.RecoveryDurStr); err != nil {
		errs = append(errs, fmt.Errorf("invalid recovery_duration %q: %w", d.RecoveryDurStr, err))
	}
	if d.AddictionChance < 0.0 || d.AddictionChance > 1.0 {
		errs = append(errs, fmt.Errorf("addiction_chance must be in [0,1], got %v", d.AddictionChance))
	}
	if d.OverdoseThreshold < 1 {
		errs = append(errs, fmt.Errorf("overdose_threshold must be >= 1, got %d", d.OverdoseThreshold))
	}
	if len(errs) > 0 {
		return fmt.Errorf("substance %q validation failed: %v", d.ID, errs)
	}
	return nil
}

// Registry holds all known SubstanceDefs keyed by ID.
type Registry struct {
	defs map[string]*SubstanceDef
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]*SubstanceDef)}
}

// Get returns the SubstanceDef for id.
//
// Postcondition: Returns (def, true) if found, or (nil, false) otherwise.
func (r *Registry) Get(id string) (*SubstanceDef, bool) {
	d, ok := r.defs[id]
	return d, ok
}

// All returns a snapshot slice sorted by ID ascending.
//
// Postcondition: returned slice is sorted by ID ascending.
func (r *Registry) All() []*SubstanceDef {
	out := make([]*SubstanceDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Register adds def to the registry.
//
// Precondition: def must not be nil and def.ID must not be empty.
func (r *Registry) Register(def *SubstanceDef) {
	if def == nil || def.ID == "" {
		return
	}
	r.defs[def.ID] = def
}

// LoadDirectory reads every *.yaml in dir, parses as SubstanceDef, validates, and returns a Registry.
//
// Precondition: dir must be a readable directory.
// Postcondition: Returns a non-nil Registry or error if any file fails to parse or validate.
func LoadDirectory(dir string) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading substance dir %q: %w", dir, err)
	}
	reg := NewRegistry()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}
		var def SubstanceDef
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&def); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}
		if err := def.Validate(); err != nil {
			return nil, fmt.Errorf("invalid substance in %q: %w", path, err)
		}
		reg.Register(&def)
	}
	return reg, nil
}

// CrossValidate checks all condition ID references in every SubstanceDef against condIDs.
//
// REQ-AH-4A: any unknown condition ID causes a fatal startup error.
// Precondition: condIDs must be the complete set of known condition IDs.
// Postcondition: returns nil iff all referenced condition IDs are in condIDs.
func (r *Registry) CrossValidate(condIDs map[string]bool) error {
	var unknown []string
	for _, def := range r.defs {
		for _, eff := range def.Effects {
			if eff.ApplyCondition != "" && !condIDs[eff.ApplyCondition] {
				unknown = append(unknown, fmt.Sprintf("%s.effects.apply_condition:%s", def.ID, eff.ApplyCondition))
			}
			if eff.RemoveCondition != "" && !condIDs[eff.RemoveCondition] {
				unknown = append(unknown, fmt.Sprintf("%s.effects.remove_condition:%s", def.ID, eff.RemoveCondition))
			}
			for _, c := range eff.CureConditions {
				if !condIDs[c] {
					unknown = append(unknown, fmt.Sprintf("%s.effects.cure_conditions:%s", def.ID, c))
				}
			}
		}
		for _, c := range def.RemoveOnExpire {
			if !condIDs[c] {
				unknown = append(unknown, fmt.Sprintf("%s.remove_on_expire:%s", def.ID, c))
			}
		}
		if def.OverdoseCondition != "" && !condIDs[def.OverdoseCondition] {
			unknown = append(unknown, fmt.Sprintf("%s.overdose_condition:%s", def.ID, def.OverdoseCondition))
		}
		for _, c := range def.WithdrawalConditions {
			if !condIDs[c] {
				unknown = append(unknown, fmt.Sprintf("%s.withdrawal_conditions:%s", def.ID, c))
			}
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown condition IDs in substance definitions: %v", unknown)
	}
	return nil
}
