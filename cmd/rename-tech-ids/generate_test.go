package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRunGenerate_WritesMapFile(t *testing.T) {
	techDir := t.TempDir()
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "rename_map.yaml")

	writeTechYAML(t, techDir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Corrosive Projectile")
	writeTechYAML(t, techDir, "neural", "chrome_reflex.yaml", "chrome_reflex", "Chrome Reflex")

	err := RunGenerate(techDir, outFile)
	require.NoError(t, err)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)

	var rm RenameMap
	require.NoError(t, yaml.Unmarshal(data, &rm))
	require.Len(t, rm.Renames, 2)

	byOld := make(map[string]RenameEntry)
	for _, e := range rm.Renames {
		byOld[e.OldID] = e
	}
	assert.Equal(t, "corrosive_projectile", byOld["acid_arrow_technical"].NewID)
	assert.True(t, byOld["chrome_reflex"].Skip)
}

func TestRunGenerate_CreatesParentDir(t *testing.T) {
	techDir := t.TempDir()
	outFile := filepath.Join(t.TempDir(), "nested", "dir", "rename_map.yaml")
	writeTechYAML(t, techDir, "technical", "x.yaml", "x_technical", "X Thing")

	err := RunGenerate(techDir, outFile)
	require.NoError(t, err)
	_, err = os.Stat(outFile)
	assert.NoError(t, err, "output file must exist")
}
