package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/importer"
	"github.com/cory-johannsen/mud/internal/importer/gomud"
)

func main() {
	format := flag.String("format", "", "source format: gomud")
	sourceDir := flag.String("source", "", "path to source asset directory")
	outputDir := flag.String("output", "", "path to output zone directory")
	startRoom := flag.String("start-room", "", "optional display-name override for zone start room")
	flag.Parse()

	if *format == "" || *sourceDir == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "usage: import-content -format <fmt> -source <dir> -output <dir> [-start-room <name>]")
		os.Exit(1)
	}

	var src importer.Source
	switch *format {
	case "gomud":
		src = gomud.NewSource()
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (supported: gomud)\n", *format)
		os.Exit(1)
	}

	start := time.Now()
	imp := importer.New(src)
	if err := imp.Run(*sourceDir, *outputDir, *startRoom); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("import complete in %s\n", time.Since(start).Round(time.Millisecond))
}
