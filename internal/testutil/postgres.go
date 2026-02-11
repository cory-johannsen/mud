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
		_ = container.Terminate(ctx)
	})

	return pc
}

// ApplyMigrations runs the schema creation SQL directly for tests.
// This avoids requiring the migrate tool in the test environment.
//
// Precondition: Pool must be connected.
// Postcondition: The accounts table exists in the test database.
func (pc *PostgresContainer) ApplyMigrations(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	start := time.Now()

	schema := `
		CREATE TABLE IF NOT EXISTS accounts (
			id         BIGSERIAL    PRIMARY KEY,
			username   VARCHAR(64)  NOT NULL UNIQUE,
			password_hash TEXT      NOT NULL,
			created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_accounts_username ON accounts (username);
	`

	_, err := pc.RawPool.Exec(ctx, schema)
	if err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	t.Logf("migrations applied [%s]", time.Since(start))
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
