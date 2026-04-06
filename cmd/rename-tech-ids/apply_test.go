package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeEntry builds a RenameEntry from a slice of (old,new,name,file,skip) tuples.
func makeEntry(oldID, newID, name, file string, skip bool) RenameEntry {
	return RenameEntry{OldID: oldID, NewID: newID, Name: name, File: file, Skip: skip}
}

func TestRenameYAMLFile_RenamesFileAndRewritesID(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "acid_arrow_technical.yaml")
	content := "id: acid_arrow_technical\nname: Corrosive Projectile\ntradition: technical\nlevel: 1\nusage_type: prepared\naction_cost: 2\nrange: ranged\ntargets: single\nduration: instant\nresolution: none\neffects: {}\n"
	require.NoError(t, os.WriteFile(oldPath, []byte(content), 0644))

	entry := makeEntry("acid_arrow_technical", "corrosive_projectile", "Corrosive Projectile", oldPath, false)
	newPath, err := renameYAMLFile(entry)
	require.NoError(t, err)

	// Old file must not exist
	_, err = os.Stat(oldPath)
	assert.True(t, os.IsNotExist(err), "old file must be gone")

	// New file must exist at new path
	expected := filepath.Join(dir, "corrosive_projectile.yaml")
	assert.Equal(t, expected, newPath)
	data, err := os.ReadFile(newPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: corrosive_projectile")
	assert.NotContains(t, string(data), "id: acid_arrow_technical")
}

func TestRenameYAMLFile_SkipEntry_NoOp(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "chrome_reflex.yaml")
	content := "id: chrome_reflex\nname: Chrome Reflex\n"
	require.NoError(t, os.WriteFile(oldPath, []byte(content), 0644))

	entry := makeEntry("chrome_reflex", "chrome_reflex", "Chrome Reflex", oldPath, true)
	newPath, err := renameYAMLFile(entry)
	require.NoError(t, err)
	assert.Equal(t, oldPath, newPath, "skip entries must not be renamed")

	// File must still exist unchanged
	data, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: chrome_reflex")
}
