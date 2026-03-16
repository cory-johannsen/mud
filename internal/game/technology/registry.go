package technology

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry holds all loaded TechnologyDefs, indexed for fast lookup.
//
// Precondition: Load must be called with a valid, existing directory path.
// Postcondition: Load is fail-fast — it returns on the first invalid or unreadable
// file, wrapping the file path in the error. An empty directory is not an error.
type Registry struct {
	byID        map[string]*TechnologyDef
	byTradition map[Tradition][]*TechnologyDef
	byLevel     map[int][]*TechnologyDef
	byUsage     map[UsageType][]*TechnologyDef
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:        make(map[string]*TechnologyDef),
		byTradition: make(map[Tradition][]*TechnologyDef),
		byLevel:     make(map[int][]*TechnologyDef),
		byUsage:     make(map[UsageType][]*TechnologyDef),
	}
}

// Load walks dir recursively, parses all YAML files, validates each def,
// and returns a populated Registry. Returns an error on the first invalid
// file; the error message includes the file path.
func Load(dir string) (*Registry, error) {
	r := NewRegistry()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %q: %w", path, err)
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %q: %w", path, err)
		}
		var def TechnologyDef
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&def); err != nil {
			return fmt.Errorf("parsing %q: %w", path, err)
		}
		if err := def.Validate(); err != nil {
			return fmt.Errorf("validating %q: %w", path, err)
		}
		r.byID[def.ID] = &def
		r.byTradition[def.Tradition] = append(r.byTradition[def.Tradition], &def)
		r.byLevel[def.Level] = append(r.byLevel[def.Level], &def)
		r.byUsage[def.UsageType] = append(r.byUsage[def.UsageType], &def)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Get returns the TechnologyDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*TechnologyDef, bool) {
	d, ok := r.byID[id]
	return d, ok
}

// All returns all loaded TechnologyDefs sorted by tradition ascending (lexicographic:
// bio_synthetic < fanatic_doctrine < neural < technical), then level ascending, then ID ascending.
func (r *Registry) All() []*TechnologyDef {
	out := make([]*TechnologyDef, 0, len(r.byID))
	for _, d := range r.byID {
		out = append(out, d)
	}
	sortTechDefs(out)
	return out
}

// ByTradition returns all technologies of the given tradition,
// sorted by level ascending, then ID ascending.
func (r *Registry) ByTradition(t Tradition) []*TechnologyDef {
	out := make([]*TechnologyDef, len(r.byTradition[t]))
	copy(out, r.byTradition[t])
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ByTraditionAndLevel returns all technologies of the given tradition at a specific level,
// sorted by ID ascending.
func (r *Registry) ByTraditionAndLevel(t Tradition, level int) []*TechnologyDef {
	var out []*TechnologyDef
	for _, d := range r.byTradition[t] {
		if d.Level == level {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByUsageType returns all technologies of the given usage type,
// sorted by tradition ascending (lexicographic: bio_synthetic < fanatic_doctrine < neural < technical),
// then level ascending, then ID ascending.
func (r *Registry) ByUsageType(u UsageType) []*TechnologyDef {
	out := make([]*TechnologyDef, len(r.byUsage[u]))
	copy(out, r.byUsage[u])
	sortTechDefs(out)
	return out
}

func sortTechDefs(defs []*TechnologyDef) {
	sort.Slice(defs, func(i, j int) bool {
		ti, tj := string(defs[i].Tradition), string(defs[j].Tradition)
		if ti != tj {
			return ti < tj
		}
		if defs[i].Level != defs[j].Level {
			return defs[i].Level < defs[j].Level
		}
		return defs[i].ID < defs[j].ID
	})
}
