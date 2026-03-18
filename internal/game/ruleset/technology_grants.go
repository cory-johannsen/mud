package ruleset

import "fmt"

// TechnologyGrants defines all technology assignments a job provides at character creation.
//
// Precondition: nil TechnologyGrants is valid (job grants no technologies).
// Postcondition: Validate() returns nil iff pool+fixed entries are sufficient for all slots.
type TechnologyGrants struct {
	Hardwired   []string           `yaml:"hardwired,omitempty"`
	Prepared    *PreparedGrants    `yaml:"prepared,omitempty"`
	Spontaneous *SpontaneousGrants `yaml:"spontaneous,omitempty"`
}

// PreparedGrants defines prepared technology slot allocation for a job.
type PreparedGrants struct {
	// SlotsByLevel maps technology level to number of prepared slots at that level.
	SlotsByLevel map[int]int `yaml:"slots_by_level"`
	// Fixed lists job-mandated prepared technologies (pre-fills slots; no player choice).
	Fixed []PreparedEntry `yaml:"fixed,omitempty"`
	// Pool lists technologies the player may choose to fill remaining slots.
	Pool []PreparedEntry `yaml:"pool,omitempty"`
}

// PreparedEntry is a technology at a specific level for a prepared slot.
type PreparedEntry struct {
	ID    string `yaml:"id"`
	Level int    `yaml:"level"`
}

// SpontaneousGrants defines spontaneous technology known-slot allocation for a job.
// Uses are a shared daily pool per level (PF2E-faithful).
type SpontaneousGrants struct {
	// KnownByLevel maps technology level to number of techs known at that level.
	KnownByLevel map[int]int `yaml:"known_by_level"`
	// UsesByLevel maps technology level to shared daily uses at that level.
	UsesByLevel map[int]int `yaml:"uses_by_level"`
	// Fixed lists job-mandated spontaneous technologies (always known; no player choice).
	Fixed []SpontaneousEntry `yaml:"fixed,omitempty"`
	// Pool lists technologies the player may choose to fill remaining known slots.
	Pool []SpontaneousEntry `yaml:"pool,omitempty"`
}

// SpontaneousEntry is a technology at a specific level for a spontaneous known slot.
type SpontaneousEntry struct {
	ID    string `yaml:"id"`
	Level int    `yaml:"level"`
}

// InnateGrant defines a single innate technology granted by an archetype or region.
type InnateGrant struct {
	ID         string `yaml:"id"`
	UsesPerDay int    `yaml:"uses_per_day"` // 0 = unlimited
}

// Validate returns an error if pool+fixed entries are insufficient to fill any slot level.
// Precondition: g is not nil.
func (g *TechnologyGrants) Validate() error {
	if g.Prepared != nil {
		for lvl, slots := range g.Prepared.SlotsByLevel {
			fixed := countEntriesAtLevel(preparedToLeveled(g.Prepared.Fixed), lvl)
			pool := countEntriesAtLevel(preparedToLeveled(g.Prepared.Pool), lvl)
			if fixed+pool < slots {
				return fmt.Errorf("prepared: level %d requires %d slots but only %d fixed+pool entries available", lvl, slots, fixed+pool)
			}
		}
	}
	if g.Spontaneous != nil {
		for lvl, known := range g.Spontaneous.KnownByLevel {
			fixed := countEntriesAtLevel(spontaneousToLeveled(g.Spontaneous.Fixed), lvl)
			pool := countEntriesAtLevel(spontaneousToLeveled(g.Spontaneous.Pool), lvl)
			if fixed+pool < known {
				return fmt.Errorf("spontaneous: level %d requires %d known but only %d fixed+pool entries available", lvl, known, fixed+pool)
			}
		}
	}
	return nil
}

type leveledEntry struct{ level int }

func preparedToLeveled(entries []PreparedEntry) []leveledEntry {
	out := make([]leveledEntry, len(entries))
	for i, e := range entries {
		out[i] = leveledEntry{e.Level}
	}
	return out
}

func spontaneousToLeveled(entries []SpontaneousEntry) []leveledEntry {
	out := make([]leveledEntry, len(entries))
	for i, e := range entries {
		out[i] = leveledEntry{e.Level}
	}
	return out
}

func countEntriesAtLevel(entries []leveledEntry, level int) int {
	n := 0
	for _, e := range entries {
		if e.level == level {
			n++
		}
	}
	return n
}

// MergeGrants combines archetype-level grants (slot progression) with job-level grants
// (fixed techs, pool options, optional extra slots).
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned grant is the union of both; nil if both are nil.
// When one argument is nil, the returned pointer aliases the other argument — callers must not mutate it.
func MergeGrants(archetype, job *TechnologyGrants) *TechnologyGrants {
	if archetype == nil && job == nil {
		return nil
	}
	if archetype == nil {
		return job
	}
	if job == nil {
		return archetype
	}
	merged := &TechnologyGrants{}

	// Hardwired: union
	merged.Hardwired = append(append([]string(nil), archetype.Hardwired...), job.Hardwired...)

	// Prepared: sum slots, union fixed and pool
	if archetype.Prepared != nil || job.Prepared != nil {
		merged.Prepared = mergePreparedGrants(archetype.Prepared, job.Prepared)
	}

	// Spontaneous: sum known/uses, union fixed and pool
	if archetype.Spontaneous != nil || job.Spontaneous != nil {
		merged.Spontaneous = mergeSpontaneousGrants(archetype.Spontaneous, job.Spontaneous)
	}

	return merged
}

func mergePreparedGrants(a, b *PreparedGrants) *PreparedGrants {
	out := &PreparedGrants{SlotsByLevel: make(map[int]int)}
	if a != nil {
		for lvl, n := range a.SlotsByLevel {
			out.SlotsByLevel[lvl] += n
		}
		out.Fixed = append(out.Fixed, a.Fixed...)
		out.Pool = append(out.Pool, a.Pool...)
	}
	if b != nil {
		for lvl, n := range b.SlotsByLevel {
			out.SlotsByLevel[lvl] += n
		}
		out.Fixed = append(out.Fixed, b.Fixed...)
		out.Pool = append(out.Pool, b.Pool...)
	}
	return out
}

func mergeSpontaneousGrants(a, b *SpontaneousGrants) *SpontaneousGrants {
	out := &SpontaneousGrants{
		KnownByLevel: make(map[int]int),
		UsesByLevel:  make(map[int]int),
	}
	if a != nil {
		for lvl, n := range a.KnownByLevel {
			out.KnownByLevel[lvl] += n
		}
		for lvl, n := range a.UsesByLevel {
			out.UsesByLevel[lvl] += n
		}
		out.Fixed = append(out.Fixed, a.Fixed...)
		out.Pool = append(out.Pool, a.Pool...)
	}
	if b != nil {
		for lvl, n := range b.KnownByLevel {
			out.KnownByLevel[lvl] += n
		}
		for lvl, n := range b.UsesByLevel {
			out.UsesByLevel[lvl] += n
		}
		out.Fixed = append(out.Fixed, b.Fixed...)
		out.Pool = append(out.Pool, b.Pool...)
	}
	return out
}

// MergeLevelUpGrants merges two level-keyed grant maps key by key.
// Used by level-up technology processing (Phase 2) to combine archetype and job level-up grants.
//
// Precondition: either or both arguments may be nil.
// Postcondition: returned map contains all keys from both inputs;
// keys present in both are merged via MergeGrants.
func MergeLevelUpGrants(archetype, job map[int]*TechnologyGrants) map[int]*TechnologyGrants {
	if len(archetype) == 0 && len(job) == 0 {
		return nil
	}
	out := make(map[int]*TechnologyGrants)
	for lvl, g := range archetype {
		out[lvl] = g
	}
	for lvl, g := range job {
		if existing, ok := out[lvl]; ok {
			out[lvl] = MergeGrants(existing, g)
		} else {
			out[lvl] = g
		}
	}
	return out
}
