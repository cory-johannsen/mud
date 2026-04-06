package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renameYAMLFile renames the YAML file for entry from old_id stem to new_id stem,
// and rewrites the id: field in the file content.
// Returns the new file path (== entry.File if skip=true).
//
// Precondition: entry.File exists on disk.
// Postcondition: if !skip, old file is gone, new file exists with updated id field.
func renameYAMLFile(entry RenameEntry) (string, error) {
	if entry.Skip {
		return entry.File, nil
	}

	data, err := os.ReadFile(entry.File)
	if err != nil {
		return "", fmt.Errorf("reading %q: %w", entry.File, err)
	}

	// Rewrite id: field — replace "id: <old_id>\n" with "id: <new_id>\n"
	oldLine := []byte("id: " + entry.OldID + "\n")
	newLine := []byte("id: " + entry.NewID + "\n")
	updated := bytes.Replace(data, oldLine, newLine, 1)
	if bytes.Equal(data, updated) {
		return "", fmt.Errorf("id line %q not found in %q", string(oldLine), entry.File)
	}

	// Compute new file path: same directory, new stem
	dir := filepath.Dir(entry.File)
	newPath := filepath.Join(dir, entry.NewID+".yaml")

	if err := os.WriteFile(entry.File, updated, 0644); err != nil {
		return "", fmt.Errorf("writing updated content to %q: %w", entry.File, err)
	}
	if err := os.Rename(entry.File, newPath); err != nil {
		return "", fmt.Errorf("renaming %q to %q: %w", entry.File, newPath, err)
	}

	return newPath, nil
}

// updateFileReferences replaces all occurrences of "id: <old>" with "id: <new>"
// in the named file for every entry in the renames map.
// The replacement is performed as a plain string substitution; it is safe
// because tech IDs consist only of [a-z0-9_] and always follow "id: ".
//
// Precondition: file exists; renames maps old_id → new_id.
// Postcondition: file content updated in-place; idempotent (second call is a no-op).
func updateFileReferences(file string, renames map[string]string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading %q: %w", file, err)
	}
	content := string(data)
	for oldID, newID := range renames {
		content = strings.ReplaceAll(content, "id: "+oldID, "id: "+newID)
	}
	return os.WriteFile(file, []byte(content), 0644)
}

// RunApply reads the rename map at mapFile and applies all non-skip, non-collision renames.
// Placeholder — full implementation added in subsequent tasks.
func RunApply(mapFile, techDir, jobDir, archetypeDir, goSourceDir, migrationsDir string) error {
	_ = mapFile
	_ = techDir
	_ = jobDir
	_ = archetypeDir
	_ = goSourceDir
	_ = migrationsDir
	return fmt.Errorf("RunApply: not yet fully implemented")
}
