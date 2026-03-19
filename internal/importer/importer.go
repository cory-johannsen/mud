package importer

import (
	"context"
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

// TechImporter orchestrates technology content import from a TechSource to output directories.
type TechImporter struct {
	source TechSource
}

// NewTech constructs a TechImporter backed by the given TechSource.
//
// Precondition: source must be non-nil.
// Postcondition: returns a non-nil TechImporter.
func NewTech(source TechSource) *TechImporter {
	return &TechImporter{source: source}
}

// RunTech loads TechData from sourceDir, calls localizer.Localize on each def,
// validates, and writes <outputDir>/<tradition>/<id>.yaml for each valid def.
// Invalid defs and localization errors are skipped with a warning (not an error).
//
// Precondition: sourceDir must satisfy the source's layout; outputDir must be
// writable; localizer must be non-nil.
// Postcondition: all valid defs are written; returns non-nil error only on load
// or directory-creation failure.
func (imp *TechImporter) RunTech(sourceDir, outputDir string, localizer Localizer) error {
	overall := time.Now()
	ctx := context.Background()

	t0 := time.Now()
	techData, warnings, err := imp.source.Load(sourceDir)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}
	if err != nil {
		return fmt.Errorf("loading tech source: %w", err)
	}
	fmt.Printf("load    %d tech(s) in %s\n", len(techData), time.Since(t0).Round(time.Millisecond))

	written := 0
	for _, td := range techData {
		t1 := time.Now()

		if err := localizer.Localize(ctx, td.Def); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: localize %q: %v; skipping\n", td.Def.ID, err)
			continue
		}

		if err := td.Def.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: invalid def %q: %v; skipping\n", td.Def.ID, err)
			continue
		}

		tradDir := filepath.Join(outputDir, td.Tradition)
		if err := os.MkdirAll(tradDir, 0755); err != nil {
			return fmt.Errorf("creating tradition directory %s: %w", tradDir, err)
		}

		data, err := yaml.Marshal(td.Def)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: marshalling %q: %v; skipping\n", td.Def.ID, err)
			continue
		}

		outPath := filepath.Join(tradDir, td.Def.ID+".yaml")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}

		fmt.Printf("wrote   %s  in %s\n", outPath, time.Since(t1).Round(time.Millisecond))
		written++
	}

	fmt.Printf("total   %d written in %s\n", written, time.Since(overall).Round(time.Millisecond))
	return nil
}
