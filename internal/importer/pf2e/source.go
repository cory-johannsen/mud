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
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading source directory %s: %w", sourceDir, err)
	}

	var results []*importer.TechData
	var warnings []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}

		path := filepath.Join(sourceDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("reading %s: %v; skipping", name, err))
			continue
		}

		spell, err := ParseSpell(data)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: parse error: %v; skipping", name, err))
			continue
		}

		techData, convWarnings, err := ConvertSpell(spell)
		warnings = append(warnings, convWarnings...)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: conversion error: %v; skipping", name, err))
			continue
		}

		results = append(results, techData...)
	}

	return results, warnings, nil
}
