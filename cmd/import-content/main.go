package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/importer"
	"github.com/cory-johannsen/mud/internal/importer/gomud"
	ipf2e "github.com/cory-johannsen/mud/internal/importer/pf2e"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the testable entry point, accepting CLI args directly.
func run(args []string) error {
	fs := flag.NewFlagSet("import-content", flag.ContinueOnError)

	format := fs.String("format", "", "source format: gomud | pf2e")
	sourceDir := fs.String("source", "", "path to source asset directory")
	outputDir := fs.String("output", "", "path to output directory (pf2e default: content/technologies/)")
	startRoom := fs.String("start-room", "", "optional display-name override for zone start room (gomud only)")
	localize := fs.Bool("localize", false, "enable Claude API lore localization (pf2e only)")
	anthropicKey := fs.String("anthropic-key", "", "Anthropic API key (also read from ANTHROPIC_API_KEY env var)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *format == "" {
		return errors.New("usage: import-content -format <fmt> -source <dir> [-output <dir>]")
	}
	if *sourceDir == "" {
		return errors.New("-source is required")
	}

	start := time.Now()

	switch *format {
	case "gomud":
		if *outputDir == "" {
			return errors.New("-output is required for format gomud")
		}
		src := gomud.NewSource()
		imp := importer.New(src)
		if err := imp.Run(*sourceDir, *outputDir, *startRoom); err != nil {
			return err
		}

	case "pf2e":
		out := *outputDir
		if out == "" {
			out = "content/technologies/"
		}

		var loc importer.Localizer = importer.NoopLocalizer{}
		if *localize {
			key := *anthropicKey
			if key == "" {
				key = os.Getenv("ANTHROPIC_API_KEY")
			}
			if key == "" {
				return errors.New("-localize requires an API key: set -anthropic-key or ANTHROPIC_API_KEY")
			}
			repoRoot := findRepoRoot()
			cl, err := importer.NewClaudeLocalizer(key, repoRoot)
			if err != nil {
				return fmt.Errorf("creating Claude localizer: %w", err)
			}
			loc = cl
		}

		src := ipf2e.NewTechSource()
		imp := importer.NewTech(src)
		if err := imp.RunTech(*sourceDir, out, loc); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown format %q (supported: gomud, pf2e)", *format)
	}

	fmt.Printf("import complete in %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}

// findRepoRoot returns the current working directory as the repo root.
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
