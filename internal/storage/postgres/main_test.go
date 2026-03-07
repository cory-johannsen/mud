package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cory-johannsen/mud/internal/config"
	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

// sharedPool is the single database pool shared across all tests in this package.
// It is initialized by TestMain before any test runs.
var sharedPool *pgxpool.Pool

// TestMain starts one Postgres container for the entire package, applies all
// migrations, runs the tests, and terminates the container.
//
// Precondition: Docker must be available.
// Postcondition: All tests run against a single, fully-migrated database.
func TestMain(m *testing.M) {
	ctx := context.Background()
	start := time.Now()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting postgres container: %v [%s]\n", err, time.Since(start))
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting container host: %v\n", err)
		os.Exit(1)
	}
	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "getting mapped port: %v\n", err)
		os.Exit(1)
	}

	dbCfg := config.DatabaseConfig{
		Host:            host,
		Port:            mappedPort.Int(),
		User:            "test",
		Password:        "test",
		Name:            "test",
		SSLMode:         "disable",
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: 5 * time.Minute,
	}

	pool, err := pgstore.NewPool(ctx, dbCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connecting to test postgres: %v [%s]\n", err, time.Since(start))
		os.Exit(1)
	}
	sharedPool = pool.DB()

	if err := applyAllMigrations(sharedPool); err != nil {
		fmt.Fprintf(os.Stderr, "applying migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "postgres container ready [%s]\n", time.Since(start))

	code := m.Run()

	pool.Close()
	termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = container.Terminate(termCtx)

	os.Exit(code)
}

// applyAllMigrations applies all schema migrations needed by the postgres_test package.
//
// Precondition: pool must be connected.
// Postcondition: All tables used by integration tests exist.
func applyAllMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id            BIGSERIAL    PRIMARY KEY,
			username      VARCHAR(64)  NOT NULL UNIQUE,
			password_hash TEXT         NOT NULL,
			role          VARCHAR(16)  NOT NULL DEFAULT 'player',
			created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_accounts_username ON accounts (username);

		CREATE TABLE IF NOT EXISTS characters (
			id              BIGSERIAL    PRIMARY KEY,
			account_id      BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			name            VARCHAR(64)  NOT NULL,
			region          TEXT         NOT NULL,
			class           TEXT         NOT NULL,
			team            TEXT         NOT NULL DEFAULT '',
			level           INT          NOT NULL DEFAULT 1,
			experience      INT          NOT NULL DEFAULT 0,
			location        TEXT         NOT NULL DEFAULT 'grinders_row',
			brutality       INT          NOT NULL DEFAULT 10,
			quickness       INT          NOT NULL DEFAULT 10,
			grit            INT          NOT NULL DEFAULT 10,
			reasoning       INT          NOT NULL DEFAULT 10,
			savvy           INT          NOT NULL DEFAULT 10,
			flair           INT          NOT NULL DEFAULT 10,
			max_hp          INT          NOT NULL DEFAULT 8,
			current_hp      INT          NOT NULL DEFAULT 8,
			created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_characters_account_name UNIQUE (account_id, name)
		);
		CREATE INDEX IF NOT EXISTS idx_characters_account_id ON characters (account_id);

		ALTER TABLE characters
			ADD COLUMN IF NOT EXISTS has_received_starting_inventory BOOLEAN NOT NULL DEFAULT FALSE;

		CREATE TABLE IF NOT EXISTS character_weapon_presets (
			id           BIGSERIAL PRIMARY KEY,
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			preset_index INT NOT NULL,
			slot         TEXT NOT NULL,
			item_def_id  TEXT NOT NULL,
			ammo_count   INT NOT NULL DEFAULT 0,
			CONSTRAINT uq_character_preset_slot UNIQUE (character_id, preset_index, slot)
		);

		CREATE TABLE IF NOT EXISTS character_equipment (
			id           BIGSERIAL PRIMARY KEY,
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			slot         TEXT NOT NULL,
			item_def_id  TEXT NOT NULL,
			CONSTRAINT uq_character_equipment_slot UNIQUE (character_id, slot)
		);

		CREATE TABLE IF NOT EXISTS character_inventory (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			item_def_id  TEXT   NOT NULL,
			quantity     INT    NOT NULL DEFAULT 1,
			PRIMARY KEY (character_id, item_def_id)
		);

		CREATE TABLE IF NOT EXISTS character_map_rooms (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			zone_id      TEXT   NOT NULL,
			room_id      TEXT   NOT NULL,
			PRIMARY KEY (character_id, zone_id, room_id)
		);

		CREATE TABLE IF NOT EXISTS character_skills (
			character_id BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			skill_id     TEXT    NOT NULL,
			proficiency  TEXT    NOT NULL DEFAULT 'untrained',
			PRIMARY KEY (character_id, skill_id)
		);

		CREATE TABLE IF NOT EXISTS character_feats (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			feat_id      TEXT NOT NULL,
			PRIMARY KEY (character_id, feat_id)
		);

		CREATE TABLE IF NOT EXISTS character_feature_choices (
			character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			feature_id    TEXT    NOT NULL,
			choice_key    TEXT    NOT NULL,
			value         TEXT    NOT NULL,
			PRIMARY KEY (character_id, feature_id, choice_key)
		);

		CREATE TABLE IF NOT EXISTS character_ability_boosts (
			character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			source        TEXT    NOT NULL,
			ability       TEXT    NOT NULL,
			PRIMARY KEY (character_id, source, ability)
		);

		CREATE TABLE IF NOT EXISTS character_proficiencies (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			category     TEXT   NOT NULL,
			rank         TEXT   NOT NULL DEFAULT 'untrained',
			PRIMARY KEY (character_id, category)
		);

		CREATE TABLE IF NOT EXISTS character_pending_boosts (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			count        INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id)
		);
	`)
	return err
}
