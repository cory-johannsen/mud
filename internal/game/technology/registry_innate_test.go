package technology_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

func TestLoad_InnateSubdirLoads(t *testing.T) {
	reg, err := technology.Load("../../../content/technologies")
	require.NoError(t, err)
	innate := reg.ByUsageType(technology.UsageInnate)
	assert.Equal(t, 11, len(innate), "expected 11 innate tech files loaded")
}
