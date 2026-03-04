// Package testutil provides test helpers including container management
// and test client utilities.
package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance.
type PostgresContainer struct {
	container testcontainers.Container
	Pool      *postgres.Pool
	RawPool   *pgxpool.Pool
	Config    config.DatabaseConfig
}

// NewPostgresContainer starts a PostgreSQL test container and returns
// a connected Pool.
//
// Precondition: Docker must be available.
// Postcondition: Returns a running container with a connected pool,
// or fails the test.
func NewPostgresContainer(t *testing.T) *PostgresContainer {
	t.Helper()
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
		t.Fatalf("starting postgres container: %v [%s]", err, time.Since(start))
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("getting container host: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("getting mapped port: %v", err)
	}

	dbCfg := config.DatabaseConfig{
		Host:            host,
		Port:            mappedPort.Int(),
		User:            "test",
		Password:        "test",
		Name:            "test",
		SSLMode:         "disable",
		MaxConns:        5,
		MinConns:        1,
		MaxConnLifetime: 5 * time.Minute,
	}

	pool, err := postgres.NewPool(ctx, dbCfg)
	if err != nil {
		t.Fatalf("connecting to test postgres: %v [%s]", err, time.Since(start))
	}

	t.Logf("postgres container started [%s]", time.Since(start))

	pc := &PostgresContainer{
		container: container,
		Pool:      pool,
		RawPool:   pool.DB(),
		Config:    dbCfg,
	}

	t.Cleanup(func() {
		pool.Close()
		termCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = container.Terminate(termCtx)
	})

	return pc
}

// ApplyMigrations runs the schema creation SQL directly for tests.
// This avoids requiring the migrate tool in the test environment.
//
// Precondition: Pool must be connected.
// Postcondition: The accounts and characters tables exist in the test database.
func (pc *PostgresContainer) ApplyMigrations(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	start := time.Now()

	schema := `
		CREATE TABLE IF NOT EXISTS accounts (
			id         BIGSERIAL    PRIMARY KEY,
			username   VARCHAR(64)  NOT NULL UNIQUE,
			password_hash TEXT      NOT NULL,
			role       VARCHAR(16)  NOT NULL DEFAULT 'player',
			created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
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
	`

	_, err := pc.RawPool.Exec(ctx, schema)
	if err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	t.Logf("migrations applied [%s]", time.Since(start))
}

// NewPool starts a PostgreSQL test container, applies all migrations, and returns
// the raw *pgxpool.Pool for use in repository tests.
//
// Precondition: Docker must be available.
// Postcondition: Returns a connected pool with schema applied, or fails the test.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pc := NewPostgresContainer(t)
	pc.ApplyMigrations(t)
	return pc.RawPool
}

// ApplySkillsMigration adds the character_skills table used by migration 011.
//
// Precondition: Pool must be connected and ApplyMigrations must have been called.
// Postcondition: The character_skills table exists in the test database.
func (pc *PostgresContainer) ApplySkillsMigration(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	start := time.Now()
	_, err := pc.RawPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS character_skills (
			character_id BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			skill_id     TEXT    NOT NULL,
			proficiency  TEXT    NOT NULL DEFAULT 'untrained',
			PRIMARY KEY (character_id, skill_id)
		);
	`)
	if err != nil {
		t.Fatalf("applying skills migration: %v", err)
	}
	t.Logf("skills migration applied [%s]", time.Since(start))
}

// DSN returns the connection string for the test database.
func (pc *PostgresContainer) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		pc.Config.User, pc.Config.Password,
		pc.Config.Host, pc.Config.Port,
		pc.Config.Name, pc.Config.SSLMode,
	)
}
