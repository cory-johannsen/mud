package pf2e_test

import (
	"os"
	"path/filepath"
	"testing"

	ipf2e "github.com/cory-johannsen/mud/internal/importer/pf2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTechSource_Load_CountsResults(t *testing.T) {
	srcDir := t.TempDir()
	for _, name := range []string{"fireball.json", "divine_single.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, name), data, 0644))
	}
	src := ipf2e.NewTechSource()
	results, _, err := src.Load(srcDir)
	require.NoError(t, err)
	// fireball: arcane+primal = 2 TechData, divine_single: divine = 1
	assert.Len(t, results, 3)
}

func TestTechSource_Load_SkipsNonJSON(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "readme.txt"), []byte("ignore"), 0644))
	data, err := os.ReadFile(filepath.Join("testdata", "divine_single.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "divine_single.json"), data, 0644))
	src := ipf2e.NewTechSource()
	results, _, err := src.Load(srcDir)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestTechSource_Load_SkipsUnparseable(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "bad.json"), []byte(`{not json`), 0644))
	data, err := os.ReadFile(filepath.Join("testdata", "divine_single.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "ok.json"), data, 0644))
	src := ipf2e.NewTechSource()
	results, warnings, err := src.Load(srcDir)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "bad.json")
}

func TestTechSource_Load_WarnsForNoTradition(t *testing.T) {
	srcDir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "no_tradition.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "no_tradition.json"), data, 0644))
	src := ipf2e.NewTechSource()
	results, warnings, err := src.Load(srcDir)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.NotEmpty(t, warnings)
}

func TestTechSource_Load_EmptyDir(t *testing.T) {
	src := ipf2e.NewTechSource()
	results, warnings, err := src.Load(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Empty(t, warnings)
}
