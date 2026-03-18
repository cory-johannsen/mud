package technology_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

// TestLoad_InnateSubdirLoads verifies that technology.Load reads all innate tech files
// from the real content directory.
func TestLoad_InnateSubdirLoads(t *testing.T) {
	reg, err := technology.Load("../../../content/technologies")
	require.NoError(t, err)
	innate := reg.ByUsageType(technology.UsageInnate)
	assert.Equal(t, 11, len(innate), "expected 11 innate tech files loaded")
}

// TestProperty_Load_InnateCount verifies that Load returns exactly as many innate
// entries as valid innate YAML files placed in the innate subdirectory.
func TestProperty_Load_InnateCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n")

		// Create a temp technologies dir with an innate subdir
		dir := t.TempDir()
		innateDir := filepath.Join(dir, "innate")
		require.NoError(t, os.MkdirAll(innateDir, 0755))

		for i := 0; i < n; i++ {
			id := fmt.Sprintf("test_innate_%d", i)
			content := fmt.Sprintf(`id: %s
name: Test Innate %d
description: A test innate technology.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: instant
resolution: none
effects:
  on_apply:
    - type: utility
`, id, i)
			path := filepath.Join(innateDir, id+".yaml")
			require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		}

		reg, err := technology.Load(dir)
		require.NoError(t, err)

		innate := reg.ByUsageType(technology.UsageInnate)
		if len(innate) != n {
			rt.Errorf("expected %d innate techs, got %d", n, len(innate))
		}
	})
}
