package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("rename-tech-ids", flag.ContinueOnError)

	generate := fs.Bool("generate", false, "scan tech YAMLs and write tools/rename_map.yaml")
	apply := fs.Bool("apply", false, "apply tools/rename_map.yaml to all files and emit DB migration")

	techDir := fs.String("tech-dir", "content/technologies", "path to technology YAML directory")
	jobDir := fs.String("job-dir", "content/jobs", "path to job YAML directory")
	archetypeDir := fs.String("archetype-dir", "content/archetypes", "path to archetype YAML directory")
	goSourceDir := fs.String("go-source-dir", "internal/importer", "path to Go source directory for string literal updates")
	migrationsDir := fs.String("migrations-dir", "migrations", "path to migrations directory")
	mapFile := fs.String("map-file", "tools/rename_map.yaml", "path to rename map file")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*generate && !*apply {
		return errors.New("usage: rename-tech-ids --generate | --apply  (see --help for flags)")
	}
	if *generate && *apply {
		return errors.New("--generate and --apply are mutually exclusive")
	}

	if *generate {
		if err := RunGenerate(*techDir, *mapFile); err != nil {
			return err
		}
		fmt.Printf("rename map written to %s\n", *mapFile)
		fmt.Println("Review the map (especially pf2e_flag=true entries), then run --apply.")
		return nil
	}

	// --apply
	if err := RunApply(*mapFile, *techDir, *jobDir, *archetypeDir, *goSourceDir, *migrationsDir); err != nil {
		return err
	}
	fmt.Println("Apply complete. Run the DB migration:")
	fmt.Printf("  %s/058_rename_tech_ids.up.sql\n", *migrationsDir)
	return nil
}
