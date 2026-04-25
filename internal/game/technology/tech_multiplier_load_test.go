package technology_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

func TestTechEffect_Multiplier_ZeroPointFive_IsHalver(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 0.5}
	err := e.ValidateMultiplier()
	assert.NoError(t, err)
	assert.True(t, e.IsHalver())
	assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_TwoPointZero_IsMultiplier(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 2.0}
	err := e.ValidateMultiplier()
	assert.NoError(t, err)
	assert.False(t, e.IsHalver())
	assert.True(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_PointThree_IsLoadError(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 0.3}
	err := e.ValidateMultiplier()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "illegal fractional")
}

func TestTechEffect_Multiplier_OnePointZero_IsNoOp(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: 1.0}
	err := e.ValidateMultiplier()
	assert.NoError(t, err)
	assert.False(t, e.IsHalver())
	assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_Unset_IsNoOp(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage}
	err := e.ValidateMultiplier()
	assert.NoError(t, err)
	assert.False(t, e.IsHalver())
	assert.False(t, e.IsMultiplier())
}

func TestTechEffect_Multiplier_NegativeValue_IsLoadError(t *testing.T) {
	e := technology.TechEffect{Type: technology.EffectDamage, Multiplier: -0.5}
	err := e.ValidateMultiplier()
	require.Error(t, err)
}
