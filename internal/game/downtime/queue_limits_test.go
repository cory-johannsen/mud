package downtime_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// TestDowntimeQueueLimitRegistry_Lookup_MatchFound verifies that Lookup returns
// the correct MaxQueue when a matching tier+level entry exists.
//
// Precondition: registry has entries for tiers 1-2 with known ranges.
// Postcondition: Lookup returns MaxQueue for matching entry.
func TestDowntimeQueueLimitRegistry_Lookup_MatchFound(t *testing.T) {
	reg := downtime.NewDowntimeQueueLimitRegistryFromEntries([]downtime.QueueLimitEntry{
		{JobTier: 1, LevelMin: 1, LevelMax: 4, MaxQueue: 3},
		{JobTier: 1, LevelMin: 5, LevelMax: 8, MaxQueue: 5},
		{JobTier: 2, LevelMin: 1, LevelMax: 4, MaxQueue: 10},
	})
	assert.Equal(t, 3, reg.Lookup(1, 1))
	assert.Equal(t, 3, reg.Lookup(1, 4))
	assert.Equal(t, 5, reg.Lookup(1, 5))
	assert.Equal(t, 10, reg.Lookup(2, 1))
}

// TestDowntimeQueueLimitRegistry_Lookup_DefaultOnNoMatch verifies that Lookup
// returns 3 (the default) when no entry matches. REQ-DTQ-14.
//
// Precondition: empty registry.
// Postcondition: Lookup(99, 99) returns 3.
func TestDowntimeQueueLimitRegistry_Lookup_DefaultOnNoMatch(t *testing.T) {
	reg := downtime.NewDowntimeQueueLimitRegistryFromEntries(nil)
	assert.Equal(t, 3, reg.Lookup(99, 99))
}

// TestDowntimeQueueLimitRegistry_Property_LookupAlwaysPositive verifies that
// Lookup always returns >= 1 for any tier and level inputs.
//
// Precondition: empty registry; any tier in [0,10], any level in [0,20].
// Postcondition: result >= 1.
func TestDowntimeQueueLimitRegistry_Property_LookupAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tier := rapid.IntRange(0, 10).Draw(t, "tier")
		level := rapid.IntRange(0, 20).Draw(t, "level")
		reg := downtime.NewDowntimeQueueLimitRegistryFromEntries(nil)
		result := reg.Lookup(tier, level)
		assert.GreaterOrEqual(t, result, 1)
	})
}

// TestDowntimeQueueLimitRegistry_LoadFromYAML verifies that a valid YAML file
// loads without error and returns a non-nil registry.
//
// Precondition: testdata/queue_limits_test.yaml exists and is valid.
// Postcondition: no error, non-nil registry.
func TestDowntimeQueueLimitRegistry_LoadFromYAML(t *testing.T) {
	reg, err := downtime.LoadDowntimeQueueLimitRegistry("testdata/queue_limits_test.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, reg)
}

// TestDowntimeQueueLimitRegistry_MissingFile_Error verifies that LoadDowntimeQueueLimitRegistry
// returns an error when the file does not exist.
//
// Precondition: testdata/nonexistent.yaml does not exist.
// Postcondition: error is non-nil.
func TestDowntimeQueueLimitRegistry_MissingFile_Error(t *testing.T) {
	_, err := downtime.LoadDowntimeQueueLimitRegistry("testdata/nonexistent.yaml")
	assert.Error(t, err)
}
