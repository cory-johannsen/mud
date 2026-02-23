package condition

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
	ACPenalty       int      `yaml:"ac_penalty"`
	SpeedPenalty    int      `yaml:"speed_penalty"`
	RestrictActions []string `yaml:"restrict_actions"`
	LuaOnApply      string   `yaml:"lua_on_apply"`  // stored; ignored until Stage 6
	LuaOnRemove     string   `yaml:"lua_on_remove"` // stored; ignored until Stage 6
	LuaOnTick       string   `yaml:"lua_on_tick"`   // stored; ignored until Stage 6
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
	r.defs[def.ID] = def
}

// Get returns the ConditionDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*ConditionDef, bool) {
	d, ok := r.defs[id]
	return d, ok
}

// All returns a snapshot slice of all registered ConditionDefs.
func (r *Registry) All() []*ConditionDef {
	out := make([]*ConditionDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
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
