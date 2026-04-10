package condition

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConditionDef is the static definition of a condition, loaded from YAML.
type ConditionDef struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	DurationType    string   `yaml:"duration_type"` // "rounds" | "until_save" | "permanent"
	MaxStacks       int      `yaml:"max_stacks"`    // 0 = unstackable
	AttackPenalty   int      `yaml:"attack_penalty"`
	AttackBonus     int      `yaml:"attack_bonus"`   // positive = bonus to attack rolls
	ACPenalty       int      `yaml:"ac_penalty"`
	SpeedPenalty    int      `yaml:"speed_penalty"`
	DamageBonus     int      `yaml:"damage_bonus"`   // positive = bonus to damage rolls
	ReflexBonus     int      `yaml:"reflex_bonus"`   // positive = bonus to Reflex saves
	StealthBonus    int      `yaml:"stealth_bonus"`  // positive = bonus to Stealth checks
	// FlairBonus is a flat bonus applied to the bearer's Flair attribute while this condition is active.
	FlairBonus      int      `yaml:"flair_bonus,omitempty"`
	RestrictActions []string `yaml:"restrict_actions"`
	// APReduction is the number of AP removed from the combatant's action queue at round start.
	APReduction int `yaml:"ap_reduction"`
	// SkipTurn, if true, causes the combatant's entire turn to be skipped.
	SkipTurn bool `yaml:"skip_turn"`
	// SkillPenalty is a flat penalty applied to ALL skill checks while this condition is active.
	// See also SkillPenalties for per-skill penalties. Both are applied additively:
	// total_penalty = SkillPenalty + SkillPenalties[skillID].
	SkillPenalty int `yaml:"skill_penalty"`
	// MoveAPCost is the additional AP cost imposed on the bearer when they move.
	// Only meaningful for terrain conditions (ID prefix "terrain_"). Default 0 = no extra cost.
	MoveAPCost int `yaml:"move_ap_cost"`
	// SkillPenalties maps canonical skill IDs (lowercase, underscore-separated, e.g. "flair", "savvy")
	// to per-skill penalties applied while this condition is active. Applied in addition to SkillPenalty:
	// total_penalty = SkillPenalty + SkillPenalties[skillID].
	// nil and empty map are equivalent (no per-skill penalties).
	SkillPenalties map[string]int `yaml:"skill_penalties,omitempty"`
	// ForcedAction, if non-empty, forces a specific action type each combat round.
	// Valid values: "random_attack" (attack random alive combatant), "lowest_hp_attack" (attack lowest-HP alive combatant).
	ForcedAction string `yaml:"forced_action"`
	LuaOnApply      string   `yaml:"lua_on_apply"`  // stored; ignored until Stage 6
	LuaOnRemove     string   `yaml:"lua_on_remove"` // stored; ignored until Stage 6
	LuaOnTick       string   `yaml:"lua_on_tick"`   // stored; ignored until Stage 6
	// Enforcement flags
	PreventMovement  bool `yaml:"prevents_movement"`
	PreventCommands  bool `yaml:"prevents_commands"`
	PreventTargeting bool `yaml:"prevents_targeting"`
	// ExtraWeaponDice is the number of additional weapon damage dice to roll on a hit.
	// Each extra die uses the weapon's own die type (e.g. 1d10 weapon → extra d10 per die).
	// Used by feats like Overpower that add bonus weapon dice rather than a flat damage bonus.
	ExtraWeaponDice int `yaml:"extra_weapon_dice,omitempty"`
	// IsDomination indicates this condition represents magical domination (e.g. confused).
	// Precious material ghost_grade weapons and armor can suppress domination conditions.
	IsDomination bool `yaml:"is_domination"`
	// IsMentalCondition indicates this condition is mental in nature (e.g. frightened, confused).
	// Precious material ghost_grade items grant resistance to mental conditions.
	IsMentalCondition bool `yaml:"is_mental_condition"`
	Severity          int  `yaml:"severity,omitempty"`
	MaxSeverity       int  `yaml:"max_severity,omitempty"`
	Stage             int  `yaml:"stage,omitempty"`
	MaxStage          int  `yaml:"max_stage,omitempty"`
}

// Registry holds all known ConditionDefs keyed by ID.
type Registry struct {
	defs map[string]*ConditionDef
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]*ConditionDef)}
}

// Register adds def to the registry, overwriting any existing entry with the same ID.
// Precondition: def must not be nil and def.ID must not be empty.
func (r *Registry) Register(def *ConditionDef) {
	if def == nil || def.ID == "" {
		return // silently skip invalid entries; LoadDirectory validates at parse time
	}
	r.defs[def.ID] = def
}

// Get returns the ConditionDef for id.
// Postcondition: Returns (def, true) if id is registered, or (nil, false) otherwise.
func (r *Registry) Get(id string) (*ConditionDef, bool) {
	d, ok := r.defs[id]
	return d, ok
}

// Has returns true if id is registered in the registry.
// Postcondition: Returns true iff Get(id) would return (non-nil, true).
func (r *Registry) Has(id string) bool {
	_, ok := r.defs[id]
	return ok
}

// All returns a snapshot slice of all registered ConditionDefs.
// Postcondition: returned slice is sorted by ID ascending.
func (r *Registry) All() []*ConditionDef {
	out := make([]*ConditionDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LoadDirectory reads every *.yaml file in dir, parses each as a ConditionDef,
// and returns a populated Registry.
// Precondition: dir must be a readable directory.
// Postcondition: Returns a non-nil Registry, or an error if any file fails to parse.
func LoadDirectory(dir string) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading condition dir %q: %w", dir, err)
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
		var def ConditionDef
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&def); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}
		reg.Register(&def)
	}
	return reg, nil
}
