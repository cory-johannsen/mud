package downtime_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/stretchr/testify/assert"
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
