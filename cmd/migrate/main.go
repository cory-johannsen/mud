// Package main provides a database migration runner.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/viper"

	"github.com/cory-johannsen/mud/internal/config"
)

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	direction := flag.String("direction", "up", "migration direction: up or down")
	steps := flag.Int("steps", 0, "number of steps (0 = all)")
	flag.Parse()

	v := viper.New()
	v.SetConfigFile(*configPath)
	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("reading config: %v", err)
	}

	var dbCfg config.DatabaseConfig
	if err := v.Sub("database").Unmarshal(&dbCfg); err != nil {
		log.Fatalf("parsing database config: %v", err)
	}

	dsn := dbCfg.DSN()
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatalf("creating migrator: %v", err)
	}
	defer m.Close()

	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	default:
		log.Fatalf("invalid direction %q: must be 'up' or 'down'", *direction)
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("migration failed: %v", err)
	}

	version, dirty, _ := m.Version()
	elapsed := time.Since(start)

	if err == migrate.ErrNoChange {
		fmt.Fprintf(os.Stdout, "no changes (version=%d dirty=%v) [%s]\n", version, dirty, elapsed)
	} else {
		fmt.Fprintf(os.Stdout, "migrated %s to version=%d dirty=%v [%s]\n", *direction, version, dirty, elapsed)
	}
}
