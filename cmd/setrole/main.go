// Package main provides a CLI tool for setting account roles.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	username := flag.String("username", "", "target account username (required)")
	role := flag.String("role", "", "role to assign: player, editor, or admin (required)")
	flag.Parse()

	if *username == "" || *role == "" {
		flag.Usage()
		os.Exit(1)
	}

	if !postgres.ValidRole(*role) {
		log.Fatalf("invalid role %q: must be one of player, editor, admin", *role)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewAccountRepository(pool.DB())

	acct, err := repo.GetByUsername(ctx, *username)
	if err != nil {
		log.Fatalf("looking up account %q: %v", *username, err)
	}

	if err := repo.SetRole(ctx, acct.ID, *role); err != nil {
		log.Fatalf("setting role: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stdout, "set role for %s (#%d): %s -> %s [%s]\n",
		acct.Username, acct.ID, acct.Role, *role, elapsed)
}
