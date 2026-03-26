package downtime_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestActivity_HasRequiredFields(t *testing.T) {
	for _, a := range downtime.AllActivities() {
		assert.NotEmpty(t, a.ID, "activity missing ID")
		assert.NotEmpty(t, a.Alias, "activity %s missing alias", a.ID)
		assert.NotEmpty(t, a.Name, "activity %s missing name", a.ID)
	}
}

func TestActivity_Count(t *testing.T) {
	assert.Len(t, downtime.AllActivities(), 15)
}

func TestTagRequirement_Safe(t *testing.T) {
	a, ok := downtime.ActivityByAlias("earn")
	assert.True(t, ok)
	assert.Equal(t, []string{"safe"}, a.RequiredTags)
}

func TestTagRequirement_Workshop(t *testing.T) {
	a, ok := downtime.ActivityByAlias("craft")
	assert.True(t, ok)
	assert.Contains(t, a.RequiredTags, "safe")
	assert.Contains(t, a.RequiredTags, "workshop")
}

func TestCanStart_RequiresSafeRoom(t *testing.T) {
	errMsg := downtime.CanStart("earn", "", false)
	assert.NotEmpty(t, errMsg)
}

func TestCanStart_BlocksIfAlreadyBusy(t *testing.T) {
	errMsg := downtime.CanStart("earn", "safe", true)
	assert.NotEmpty(t, errMsg)
}

func TestCanStart_SucceedsForSafeRoom(t *testing.T) {
	errMsg := downtime.CanStart("earn", "safe", false)
	assert.Empty(t, errMsg)
}

func TestCanStart_WorkshopRequiredForCraft(t *testing.T) {
	// safe only — no workshop
	errMsg := downtime.CanStart("craft", "safe", false)
	assert.NotEmpty(t, errMsg)

	// safe + workshop
	errMsg = downtime.CanStart("craft", "safe,workshop", false)
	assert.Empty(t, errMsg)
}

func TestCanStart_AnalyzeTechRequiresWorkshopOrArchive(t *testing.T) {
	// safe only — fails
	assert.NotEmpty(t, downtime.CanStart("analyze", "safe", false))
	// safe + workshop — passes
	assert.Empty(t, downtime.CanStart("analyze", "safe,workshop", false))
	// safe + archive — passes
	assert.Empty(t, downtime.CanStart("analyze", "safe,archive", false))
}

func TestActivityByID_Unknown(t *testing.T) {
	_, ok := downtime.ActivityByID("nonexistent")
	assert.False(t, ok)
}

func TestActivityByAlias_Unknown(t *testing.T) {
	_, ok := downtime.ActivityByAlias("nonexistent")
	assert.False(t, ok)
}

// Property: every activity has a non-empty ID, Name, and Alias
func TestProperty_AllActivities_HaveRequiredFields(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		activities := downtime.AllActivities()
		idx := rapid.IntRange(0, len(activities)-1).Draw(rt, "idx")
		a := activities[idx]
		if a.ID == "" {
			rt.Fatalf("activity at index %d has empty ID", idx)
		}
		if a.Name == "" {
			rt.Fatalf("activity %s has empty Name", a.ID)
		}
		if a.Alias == "" {
			rt.Fatalf("activity %s has empty Alias", a.ID)
		}
	})
}

// Property: every activity's RequiredTags always contains "safe"
func TestProperty_AllActivities_AlwaysRequireSafe(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		activities := downtime.AllActivities()
		idx := rapid.IntRange(0, len(activities)-1).Draw(rt, "idx")
		a := activities[idx]
		hasSafe := false
		for _, tag := range a.RequiredTags {
			if tag == "safe" {
				hasSafe = true
				break
			}
		}
		if !hasSafe {
			rt.Fatalf("activity %s does not require 'safe' tag", a.ID)
		}
	})
}

// Property: TagsContain is consistent — if a tag is in a comma-separated string,
// TagsContain must return true
func TestProperty_TagsContain_Consistent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tag := rapid.StringMatching(`[a-z_]+`).Draw(rt, "tag")
		other := rapid.StringMatching(`[a-z_]+`).Draw(rt, "other")
		// Single tag string
		if !downtime.TagsContain(tag, tag) {
			rt.Fatalf("TagsContain(%q, %q) returned false for exact match", tag, tag)
		}
		// Multi-tag string: "other,tag" must contain tag
		combined := other + "," + tag
		if !downtime.TagsContain(combined, tag) {
			rt.Fatalf("TagsContain(%q, %q) returned false when tag is present", combined, tag)
		}
	})
}

// Property: CanStart with no safe tag always returns non-empty error regardless of alias
func TestProperty_CanStart_NoSafe_AlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		alias := rapid.StringMatching(`[a-z]+`).Draw(rt, "alias")
		tags := rapid.StringMatching(`[a-z_,]*`).Filter(func(s string) bool {
			return !downtime.TagsContain(s, "safe")
		}).Draw(rt, "tags")
		result := downtime.CanStart(alias, tags, false)
		if result == "" {
			rt.Fatalf("CanStart(%q, %q, false) returned empty error without safe tag", alias, tags)
		}
	})
}

// Property: CanStart with busy=true always returns non-empty error
func TestProperty_CanStart_Busy_AlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		alias := rapid.StringMatching(`[a-z]+`).Draw(rt, "alias")
		tags := rapid.StringMatching(`[a-z_,]*`).Draw(rt, "tags")
		result := downtime.CanStart(alias, tags, true)
		if result == "" {
			rt.Fatalf("CanStart(%q, %q, true) returned empty error when busy", alias, tags)
		}
	})
}
