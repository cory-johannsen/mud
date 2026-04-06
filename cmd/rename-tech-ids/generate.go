package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RunGenerate scans techDir for technology YAML files, builds a RenameMap,
// and writes it to outFile (creating parent directories as needed).
//
// Precondition: techDir exists.
// Postcondition: outFile contains valid YAML representing the RenameMap.
func RunGenerate(techDir, outFile string) error {
	rm, err := BuildRenameMap(techDir)
	if err != nil {
		return fmt.Errorf("building rename map: %w", err)
	}

	data, err := yaml.Marshal(rm)
	if err != nil {
		return fmt.Errorf("marshalling rename map: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", outFile, err)
	}

	return nil
}
