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

func TestUpdateFileReferences_ReplacesAllOccurrences(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "illusionist.yaml")
	content := `id: illusionist
technology_grants:
  prepared:
    pool:
      - { id: acid_arrow_technical, level: 1 }
      - { id: daze_neural, level: 1 }
      - { id: chrome_reflex, level: 1 }
level_up_grants:
  3:
    prepared:
      pool:
        - id: acid_arrow_technical
          level: 2
`
	require.NoError(t, os.WriteFile(jobFile, []byte(content), 0644))

	renames := map[string]string{
		"acid_arrow_technical": "corrosive_projectile",
		"daze_neural":          "cranial_shock",
	}
	err := updateFileReferences(jobFile, renames)
	require.NoError(t, err)

	data, err := os.ReadFile(jobFile)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "id: corrosive_projectile")
	assert.Contains(t, s, "id: cranial_shock")
	assert.Contains(t, s, "id: chrome_reflex", "unrenamed IDs must be untouched")
	assert.NotContains(t, s, "acid_arrow_technical")
	assert.NotContains(t, s, "daze_neural")
	// Job id line must not be renamed
	assert.Contains(t, s, "id: illusionist")
}

func TestUpdateFileReferences_Idempotent(t *testing.T) {
	dir := t.TempDir()
	jobFile := filepath.Join(dir, "job.yaml")
	content := "id: job\ntechnology_grants:\n  pool:\n    - { id: corrosive_projectile, level: 1 }\n"
	require.NoError(t, os.WriteFile(jobFile, []byte(content), 0644))

	renames := map[string]string{"acid_arrow_technical": "corrosive_projectile"}
	require.NoError(t, updateFileReferences(jobFile, renames))

	data, _ := os.ReadFile(jobFile)
	require.NoError(t, updateFileReferences(jobFile, renames))
	data2, _ := os.ReadFile(jobFile)
	assert.Equal(t, string(data), string(data2), "second pass must be a no-op")
}

func TestUpdateGoStringLiterals_BacktickMapKeys(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "static_localizer.go")
	content := "var m = map[string]x{\n\t`acid_arrow_technical`: {Name: `Corrosive Projectile`},\n\t`daze_neural`: {Name: `Cranial Shock`},\n\t`chrome_reflex`: {Name: `Chrome Reflex`},\n}\n"
	require.NoError(t, os.WriteFile(goFile, []byte(content), 0644))

	renames := map[string]string{
		"acid_arrow_technical": "corrosive_projectile",
		"daze_neural":          "cranial_shock",
	}
	err := updateGoStringLiterals(goFile, renames)
	require.NoError(t, err)

	data, err := os.ReadFile(goFile)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "`corrosive_projectile`:")
	assert.Contains(t, s, "`cranial_shock`:")
	assert.Contains(t, s, "`chrome_reflex`:", "unrenamed ID must be untouched")
	assert.NotContains(t, s, "`acid_arrow_technical`")
	assert.NotContains(t, s, "`daze_neural`")
	// Value backtick strings must be untouched
	assert.Contains(t, s, "`Corrosive Projectile`")
}

func TestUpdateGoStringLiterals_DoubleQuotedStrings(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "service_test.go")
	content := "techID := \"acid_arrow_technical\"\nother := \"chrome_reflex\"\n"
	require.NoError(t, os.WriteFile(goFile, []byte(content), 0644))

	renames := map[string]string{"acid_arrow_technical": "corrosive_projectile"}
	require.NoError(t, updateGoStringLiterals(goFile, renames))

	data, _ := os.ReadFile(goFile)
	s := string(data)
	assert.Contains(t, s, `"corrosive_projectile"`)
	assert.NotContains(t, s, `"acid_arrow_technical"`)
	assert.Contains(t, s, `"chrome_reflex"`)
}

func TestEmitMigration_UpAndDown(t *testing.T) {
	dir := t.TempDir()
	upFile := filepath.Join(dir, "058_rename_tech_ids.up.sql")
	downFile := filepath.Join(dir, "058_rename_tech_ids.down.sql")

	renames := []RenameEntry{
		{OldID: "acid_arrow_technical", NewID: "corrosive_projectile", Skip: false},
		{OldID: "daze_neural", NewID: "cranial_shock", Skip: false},
		{OldID: "chrome_reflex", NewID: "chrome_reflex", Skip: true},
	}

	err := emitMigration(renames, upFile, downFile)
	require.NoError(t, err)

	up, err := os.ReadFile(upFile)
	require.NoError(t, err)
	upStr := string(up)
	// Each non-skip entry gets UPDATE statements for all 4 tables
	assert.Contains(t, upStr, "SET tech_id = 'corrosive_projectile' WHERE tech_id = 'acid_arrow_technical'")
	assert.Contains(t, upStr, "SET tech_id = 'cranial_shock' WHERE tech_id = 'daze_neural'")
	assert.NotContains(t, upStr, "chrome_reflex", "skip entries must not appear in migration")
	// Must cover all 4 tables
	assert.Contains(t, upStr, "character_hardwired_technologies")
	assert.Contains(t, upStr, "character_innate_technologies")
	assert.Contains(t, upStr, "character_spontaneous_technologies")
	assert.Contains(t, upStr, "character_prepared_technologies")

	down, err := os.ReadFile(downFile)
	require.NoError(t, err)
	downStr := string(down)
	// Down is the inverse
	assert.Contains(t, downStr, "SET tech_id = 'acid_arrow_technical' WHERE tech_id = 'corrosive_projectile'")
	assert.Contains(t, downStr, "SET tech_id = 'daze_neural' WHERE tech_id = 'cranial_shock'")
}
