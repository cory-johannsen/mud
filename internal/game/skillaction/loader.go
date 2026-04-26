package skillaction

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

// rawActionDef is the on-disk YAML shape for an ActionDef. The wire format
// uses string keys for the outcome map; we project to typed enums in Load.
type rawActionDef struct {
	ID          string                `yaml:"id"`
	DisplayName string                `yaml:"display_name"`
	Description string                `yaml:"description"`
	APCost      int                   `yaml:"ap_cost"`
	Skill       string                `yaml:"skill"`
	DC          DC                    `yaml:"dc"`
	Range       Range                 `yaml:"range"`
	TargetKinds []TargetKind          `yaml:"target_kinds"`
	Outcomes    map[string]rawOutcome `yaml:"outcomes"`
}

// Load parses a single ActionDef from a YAML byte slice. When condReg is non-nil
// every apply_condition.id referenced is validated against it. Numeric fields
// are checked for non-negativity per NCA-3.
func Load(data []byte, condReg *condition.Registry) (*ActionDef, error) {
	var raw rawActionDef
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("action def: id is required")
	}
	if raw.APCost < 0 {
		return nil, fmt.Errorf("action %q: ap_cost must be non-negative (got %d)", raw.ID, raw.APCost)
	}
	if raw.Skill == "" {
		return nil, fmt.Errorf("action %q: skill is required", raw.ID)
	}
	if raw.DC.Kind == "" {
		return nil, fmt.Errorf("action %q: dc.kind is required", raw.ID)
	}
	if raw.DC.Value < 0 {
		return nil, fmt.Errorf("action %q: dc.value must be non-negative (got %d)", raw.ID, raw.DC.Value)
	}
	if raw.Range.Kind == "" {
		return nil, fmt.Errorf("action %q: range.kind is required", raw.ID)
	}
	if raw.Range.Feet < 0 {
		return nil, fmt.Errorf("action %q: range.feet must be non-negative (got %d)", raw.ID, raw.Range.Feet)
	}
	if len(raw.TargetKinds) == 0 {
		return nil, fmt.Errorf("action %q: target_kinds must be non-empty", raw.ID)
	}

	def := &ActionDef{
		ID:          raw.ID,
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		APCost:      raw.APCost,
		Skill:       raw.Skill,
		DC:          raw.DC,
		Range:       raw.Range,
		TargetKinds: raw.TargetKinds,
		Outcomes:    make(map[DegreeOfSuccess]*OutcomeDef),
	}
	for k, v := range raw.Outcomes {
		dos, err := parseDegree(k)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", raw.ID, err)
		}
		od := &OutcomeDef{APRefund: v.APRefund}
		for i, e := range v.Effects {
			eff, err := e.ToEffect()
			if err != nil {
				return nil, fmt.Errorf("action %q outcome %s effect %d: %w", raw.ID, k, i, err)
			}
			if ac, ok := eff.(ApplyCondition); ok {
				if ac.ID == "" {
					return nil, fmt.Errorf("action %q outcome %s: apply_condition.id is required", raw.ID, k)
				}
				if ac.Stacks < 0 {
					return nil, fmt.Errorf("action %q outcome %s: apply_condition.stacks must be non-negative", raw.ID, k)
				}
				if condReg != nil && !condReg.Has(ac.ID) {
					return nil, fmt.Errorf("action %q outcome %s: unknown condition %q (not_a_condition guard)", raw.ID, k, ac.ID)
				}
			}
			if d, ok := eff.(Damage); ok && d.Expr == "" {
				return nil, fmt.Errorf("action %q outcome %s: damage.expr is required", raw.ID, k)
			}
			if r, ok := eff.(Reveal); ok && r.Count <= 0 {
				return nil, fmt.Errorf("action %q outcome %s: reveal.count must be positive", raw.ID, k)
			}
			od.Effects = append(od.Effects, eff)
			if n, ok := eff.(Narrative); ok && od.Narrative == "" {
				od.Narrative = n.Text
			}
		}
		def.Outcomes[dos] = od
	}
	return def, nil
}

// LoadDirectory scans dir for *.yaml skill-action defs and returns them keyed by ID.
// condReg, when non-nil, is used to validate every apply_condition.id reference.
func LoadDirectory(dir string, condReg *condition.Registry) (map[string]*ActionDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}
	out := make(map[string]*ActionDef)
	// Sort for deterministic load order — surfacing errors early on the first conflict.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		def, err := Load(data, condReg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if _, dup := out[def.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate action id %q", name, def.ID)
		}
		out[def.ID] = def
	}
	return out, nil
}
