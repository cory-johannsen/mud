package downtime

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// QueueLimitEntry maps a job tier + level range to a max queue size.
//
// Precondition: JobTier in [1,3]; LevelMin >= 1; LevelMax >= LevelMin; MaxQueue >= 1.
type QueueLimitEntry struct {
	JobTier  int
	LevelMin int
	LevelMax int
	MaxQueue int
}

type queueLimitsYAML struct {
	Limits []struct {
		JobTier    int    `yaml:"job_tier"`
		LevelRange [2]int `yaml:"level_range"`
		MaxQueue   int    `yaml:"max_queue"`
	} `yaml:"limits"`
}

// DowntimeQueueLimitRegistry computes the downtime queue limit for a given job tier + level.
//
// Precondition: non-nil (use NewDowntimeQueueLimitRegistryFromEntries or LoadDowntimeQueueLimitRegistry).
// Postcondition: Lookup always returns >= 1.
type DowntimeQueueLimitRegistry struct {
	entries []QueueLimitEntry
}

// NewDowntimeQueueLimitRegistryFromEntries builds a registry from a slice of entries.
//
// Precondition: entries may be nil (yields empty registry; Lookup returns default).
// Postcondition: Returns non-nil registry.
func NewDowntimeQueueLimitRegistryFromEntries(entries []QueueLimitEntry) *DowntimeQueueLimitRegistry {
	return &DowntimeQueueLimitRegistry{entries: entries}
}

// LoadDowntimeQueueLimitRegistry loads the registry from a YAML file.
// Returns error if the file is missing or malformed (REQ-DTQ-15: caller must treat as fatal).
//
// Precondition: path is a non-empty file path.
// Postcondition: On success, returns non-nil registry; on error, returns nil.
func LoadDowntimeQueueLimitRegistry(path string) (*DowntimeQueueLimitRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading queue limits YAML %q: %w", path, err)
	}
	var raw queueLimitsYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing queue limits YAML %q: %w", path, err)
	}
	entries := make([]QueueLimitEntry, 0, len(raw.Limits))
	for _, l := range raw.Limits {
		entries = append(entries, QueueLimitEntry{
			JobTier:  l.JobTier,
			LevelMin: l.LevelRange[0],
			LevelMax: l.LevelRange[1],
			MaxQueue: l.MaxQueue,
		})
	}
	return &DowntimeQueueLimitRegistry{entries: entries}, nil
}

// Lookup returns the max queue size for the given job tier and level.
// Returns 3 (default) if no matching entry is found (REQ-DTQ-14).
//
// Precondition: jobTier and level may be any int (gracefully handles out-of-range).
// Postcondition: Returns >= 1.
func (r *DowntimeQueueLimitRegistry) Lookup(jobTier int, level int) int {
	for _, e := range r.entries {
		if e.JobTier == jobTier && level >= e.LevelMin && level <= e.LevelMax {
			return e.MaxQueue
		}
	}
	return 3 // REQ-DTQ-14: default
}
