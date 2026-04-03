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
	byShortName map[string]*TechnologyDef
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:        make(map[string]*TechnologyDef),
		byTradition: make(map[Tradition][]*TechnologyDef),
		byLevel:     make(map[int][]*TechnologyDef),
		byUsage:     make(map[UsageType][]*TechnologyDef),
		byShortName: make(map[string]*TechnologyDef),
	}
}

// Load walks dir recursively, parses all YAML files, validates each def,
// and returns a populated Registry. Returns an error on the first invalid
// file; the error message includes the file path. Duplicate short names or
// short names that collide with an existing technology ID are also errors.
//
// Precondition: dir must be a valid, existing directory path.
// Postcondition: returned Registry has all secondary indexes fully populated;
// nil is returned on any error.
func Load(dir string) (*Registry, error) {
	// First pass: parse and validate all defs.
	var defs []*TechnologyDef
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
		defs = append(defs, &def)
		return nil
	})
	if err != nil {
		return nil, err
	}

	r := NewRegistry()
	// Build byID first so second pass can check short_name vs. all IDs.
	for _, def := range defs {
		r.byID[def.ID] = def
	}
	// Second pass: populate secondary indexes; enforce short_name uniqueness and ID collision.
	for _, def := range defs {
		r.byTradition[def.Tradition] = append(r.byTradition[def.Tradition], def)
		r.byLevel[def.Level] = append(r.byLevel[def.Level], def)
		r.byUsage[def.UsageType] = append(r.byUsage[def.UsageType], def)
		if def.ShortName == "" {
			continue
		}
		if existing, dup := r.byShortName[def.ShortName]; dup {
			return nil, fmt.Errorf("duplicate short_name %q on %q and %q", def.ShortName, existing.ID, def.ID)
		}
		// Validate() already enforces ShortName != ID, so this check never triggers on def itself.
		if other, col := r.byID[def.ShortName]; col {
			return nil, fmt.Errorf("short_name %q on %q collides with existing technology id %q", def.ShortName, def.ID, other.ID)
		}
		r.byShortName[def.ShortName] = def
	}
	return r, nil
}

// Get returns the TechnologyDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*TechnologyDef, bool) {
	d, ok := r.byID[id]
	return d, ok
}

// GetByShortName returns the TechnologyDef for the given short name, or (nil, false) if not found.
func (r *Registry) GetByShortName(short string) (*TechnologyDef, bool) {
	d, ok := r.byShortName[short]
	return d, ok
}

// Register adds or replaces a TechnologyDef in the registry by its ID.
// Intended for use in tests and programmatic registry population.
//
// Precondition: def must be non-nil.
// Postcondition: def is retrievable via Get(def.ID). If def.ShortName is non-empty,
// def is also retrievable via GetByShortName(def.ShortName).
func (r *Registry) Register(def *TechnologyDef) {
	r.byID[def.ID] = def
	if def.ShortName != "" {
		r.byShortName[def.ShortName] = def
	}
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
