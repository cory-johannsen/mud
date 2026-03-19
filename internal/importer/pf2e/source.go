package pf2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cory-johannsen/mud/internal/importer"
)

// PF2ETechSource implements importer.TechSource for PF2E compendium JSON files.
// Each .json file in sourceDir is expected to be a PF2E spell document.
type PF2ETechSource struct{}

// NewTechSource constructs a PF2ETechSource.
func NewTechSource() *PF2ETechSource { return &PF2ETechSource{} }

var _ importer.TechSource = (*PF2ETechSource)(nil)

// Load walks sourceDir, parses each .json file as a PF2E spell, and converts it.
// Non-JSON files are silently skipped. Files that fail to parse produce warnings.
// An empty source directory is not an error.
//
// Precondition: sourceDir must exist.
// Postcondition: returns all successfully converted TechData, accumulated warnings,
// and nil error.
func (s *PF2ETechSource) Load(sourceDir string) ([]*importer.TechData, []string, error) {
	var results []*importer.TechData
	var warnings []string

	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("walking %s: %v; skipping", path, err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("reading %s: %v; skipping", name, readErr))
			return nil
		}

		spell, parseErr := ParseSpell(data)
		if parseErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: parse error: %v; skipping", name, parseErr))
			return nil
		}

		techData, convWarnings, convErr := ConvertSpell(spell)
		warnings = append(warnings, convWarnings...)
		if convErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: conversion error: %v; skipping", name, convErr))
			return nil
		}

		results = append(results, techData...)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walking source directory %s: %w", sourceDir, err)
	}

	return results, warnings, nil
}
