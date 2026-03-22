// Package main provides a CLI tool to seed three Claude agent accounts into
// the MUD PostgreSQL database. Run once (or repeatedly — it is idempotent).
//
// Usage:
//
//	CLAUDE_ACCOUNT_PASSWORD=<password> ./seed-claude-accounts [-config configs/dev.yaml]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// claudeAccounts defines the three accounts to seed.
var claudeAccounts = []struct {
	username string
	role     string
}{
	{"claude_player", postgres.RolePlayer},
	{"claude_editor", postgres.RoleEditor},
	{"claude_admin", postgres.RoleAdmin},
}

func main() {
	start := time.Now()

	configPath := flag.String("config", "configs/dev.yaml", "path to configuration file")
	flag.Parse()

	password := os.Getenv("CLAUDE_ACCOUNT_PASSWORD")
	if password == "" {
		log.Fatal("CLAUDE_ACCOUNT_PASSWORD environment variable must be set")
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

	for _, acc := range claudeAccounts {
		if err := upsertAccount(ctx, repo, acc.username, password, acc.role); err != nil {
			log.Fatalf("upserting account %q: %v", acc.username, err)
		}
		fmt.Fprintf(os.Stdout, "seeded account: %s (role: %s)\n", acc.username, acc.role)
	}

	fmt.Fprintf(os.Stdout, "done [%s]\n", time.Since(start))
}

// upsertAccount implements the three-step idempotent upsert:
//  1. Fetch by username.
//  2. If absent, create via AccountRepository.Create() (bcrypt path).
//  3. Set role via AccountRepository.SetRole() (ensures correct role even if account pre-existed).
//
// Precondition: repo must be non-nil; username, password, and role must be non-empty.
// Postcondition: Account exists in DB with the specified role; no duplicate is created.
func upsertAccount(ctx context.Context, repo *postgres.AccountRepository, username, password, role string) error {
	acct, err := repo.GetByUsername(ctx, username)
	if errors.Is(err, postgres.ErrAccountNotFound) {
		acct, err = repo.Create(ctx, username, password)
		if err != nil {
			return fmt.Errorf("creating account %q: %w", username, err)
		}
	} else if err != nil {
		return fmt.Errorf("fetching account %q: %w", username, err)
	}
	if err := repo.SetRole(ctx, acct.ID, role); err != nil {
		return fmt.Errorf("setting role for %q: %w", username, err)
	}
	return nil
}
