package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cory-johannsen/mud/internal/game/world"
	"gopkg.in/yaml.v3"
)

// Importer orchestrates content import from a Source to an output directory.
type Importer struct {
	source Source
}

// New constructs an Importer backed by the given Source.
//
// Precondition: source must be non-nil.
// Postcondition: returns a non-nil Importer.
func New(source Source) *Importer {
	return &Importer{source: source}
}

// Run loads zones from sourceDir, validates each, and writes them as YAML
// files to outputDir. Each output file is named <zone_id>.yaml.
//
// Precondition: sourceDir must satisfy the source's layout requirements;
// outputDir must exist or be creatable.
// Postcondition: one zone YAML per zone is written to outputDir, or an error
// is returned.
func (imp *Importer) Run(sourceDir, outputDir, startRoom string) error {
	overall := time.Now()

	t0 := time.Now()
	zones, err := imp.source.Load(sourceDir, startRoom)
	if err != nil {
		return fmt.Errorf("loading source: %w", err)
	}
	fmt.Printf("load    %d zone(s) in %s\n", len(zones), time.Since(t0).Round(time.Millisecond))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outputDir, err)
	}

	for _, zd := range zones {
		t1 := time.Now()

		data, err := yaml.Marshal(zd)
		if err != nil {
			return fmt.Errorf("serialising zone %q: %w", zd.Zone.ID, err)
		}

		// Validate output is loadable before writing.
		if _, err := world.LoadZoneFromBytes(data); err != nil {
			return fmt.Errorf("zone %q failed validation: %w", zd.Zone.ID, err)
		}

		outPath := filepath.Join(outputDir, zd.Zone.ID+".yaml")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("writing zone %q to %s: %w", zd.Zone.ID, outPath, err)
		}

		fmt.Printf("wrote   %s  (%d rooms)  in %s\n",
			outPath, len(zd.Zone.Rooms), time.Since(t1).Round(time.Millisecond))
	}

	fmt.Printf("total   %s\n", time.Since(overall).Round(time.Millisecond))
	return nil
}
