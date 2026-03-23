package substance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/substance"
)

func TestActiveSubstance_ZeroValue(t *testing.T) {
	var a substance.ActiveSubstance
	assert.Equal(t, "", a.SubstanceID)
	assert.Equal(t, 0, a.DoseCount)
	assert.False(t, a.EffectsApplied)
}

func TestSubstanceAddiction_ZeroValue(t *testing.T) {
	var a substance.SubstanceAddiction
	assert.Equal(t, "", a.Status)
	assert.True(t, a.WithdrawalUntil.IsZero())
}
