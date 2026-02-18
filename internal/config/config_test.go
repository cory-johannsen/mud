package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func validConfig() Config {
	return Config{
		Server: ServerConfig{
			Mode: "standalone",
			Type: "mud",
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "mud",
			Password:        "mud",
			Name:            "mud",
			SSLMode:         "disable",
			MaxConns:        10,
			MinConns:        2,
			MaxConnLifetime: time.Hour,
		},
		Telnet: TelnetConfig{
			Host:         "0.0.0.0",
			Port:         4000,
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		GameServer: GameServerConfig{
			GRPCHost: "127.0.0.1",
			GRPCPort: 50051,
		},
	}
}

func TestValidConfig(t *testing.T) {
	cfg := validConfig()
	assert.NoError(t, cfg.Validate())
}

func TestDatabaseDSN(t *testing.T) {
	cfg := validConfig()
	dsn := cfg.Database.DSN()
	assert.Equal(t, "postgres://mud:mud@localhost:5432/mud?sslmode=disable", dsn)
}

func TestTelnetAddr(t *testing.T) {
	cfg := validConfig()
	assert.Equal(t, "0.0.0.0:4000", cfg.Telnet.Addr())
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	err := os.WriteFile(path, []byte(`
server:
  mode: standalone
  type: mud
database:
  host: localhost
  port: 5432
  user: testuser
  password: testpass
  name: testdb
  sslmode: disable
  max_conns: 5
  min_conns: 1
  max_conn_lifetime: 30m
telnet:
  host: 127.0.0.1
  port: 4001
  read_timeout: 1m
  write_timeout: 10s
logging:
  level: debug
  format: console
`), 0644)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "standalone", cfg.Server.Mode)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, 4001, cfg.Telnet.Port)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestValidateServerMode(t *testing.T) {
	for _, mode := range []string{"standalone", "frontend", "backend"} {
		cfg := validConfig()
		cfg.Server.Mode = mode
		assert.NoError(t, cfg.Validate(), "mode %q should be valid", mode)
	}
	cfg := validConfig()
	cfg.Server.Mode = "invalid"
	assert.Error(t, cfg.Validate())
}

func TestValidateServerTypeEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Type = ""
	assert.Error(t, cfg.Validate())
}

func TestValidateLoggingLevel(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := validConfig()
		cfg.Logging.Level = level
		assert.NoError(t, cfg.Validate(), "level %q should be valid", level)
	}
	cfg := validConfig()
	cfg.Logging.Level = "trace"
	assert.Error(t, cfg.Validate())
}

func TestValidateLoggingFormat(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		cfg := validConfig()
		cfg.Logging.Format = format
		assert.NoError(t, cfg.Validate(), "format %q should be valid", format)
	}
	cfg := validConfig()
	cfg.Logging.Format = "xml"
	assert.Error(t, cfg.Validate())
}

func TestValidateDatabasePort(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Port = 0
	assert.Error(t, cfg.Validate())

	cfg = validConfig()
	cfg.Database.Port = 65536
	assert.Error(t, cfg.Validate())
}

func TestValidateDatabaseMaxConns(t *testing.T) {
	cfg := validConfig()
	cfg.Database.MaxConns = 0
	assert.Error(t, cfg.Validate())
}

func TestValidateDatabaseMinConnsExceedsMax(t *testing.T) {
	cfg := validConfig()
	cfg.Database.MinConns = 20
	cfg.Database.MaxConns = 10
	assert.Error(t, cfg.Validate())
}

func TestValidateTelnetPort(t *testing.T) {
	cfg := validConfig()
	cfg.Telnet.Port = 0
	assert.Error(t, cfg.Validate())
}

func TestGameServerAddr(t *testing.T) {
	cfg := validConfig()
	assert.Equal(t, "127.0.0.1:50051", cfg.GameServer.Addr())
}

func TestValidateGameServerPort(t *testing.T) {
	cfg := validConfig()
	cfg.GameServer.GRPCPort = 0
	assert.Error(t, cfg.Validate())

	cfg = validConfig()
	cfg.GameServer.GRPCPort = 65536
	assert.Error(t, cfg.Validate())
}

func TestValidateGameServerHostEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.GameServer.GRPCHost = ""
	assert.Error(t, cfg.Validate())
}

// Property-based tests

func TestPropertyValidPortRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		port := rapid.IntRange(1, 65535).Draw(t, "port")
		cfg := validConfig()
		cfg.Database.Port = port
		err := cfg.Validate()
		if err != nil {
			t.Fatalf("valid port %d rejected: %v", port, err)
		}
	})
}

func TestPropertyInvalidPortRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate ports outside valid range
		port := rapid.OneOf(
			rapid.IntRange(-1000, 0),
			rapid.IntRange(65536, 100000),
		).Draw(t, "port")
		cfg := validConfig()
		cfg.Database.Port = port
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("invalid port %d accepted", port)
		}
	})
}

func TestPropertyMaxConnsAlwaysPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxConns := rapid.Int32Range(1, 1000).Draw(t, "max_conns")
		minConns := rapid.Int32Range(0, maxConns).Draw(t, "min_conns")
		cfg := validConfig()
		cfg.Database.MaxConns = maxConns
		cfg.Database.MinConns = minConns
		err := cfg.Validate()
		if err != nil {
			t.Fatalf("valid conns max=%d min=%d rejected: %v", maxConns, minConns, err)
		}
	})
}

func TestPropertyMinConnsNeverExceedsMax(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxConns := rapid.Int32Range(1, 100).Draw(t, "max_conns")
		minConns := rapid.Int32Range(maxConns+1, maxConns+100).Draw(t, "min_conns")
		cfg := validConfig()
		cfg.Database.MaxConns = maxConns
		cfg.Database.MinConns = minConns
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("min_conns=%d > max_conns=%d accepted", minConns, maxConns)
		}
	})
}

func TestPropertyDSNContainsAllFields(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		host := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "host")
		port := rapid.IntRange(1, 65535).Draw(t, "port")
		user := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "user")
		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "name")

		db := DatabaseConfig{
			Host:    host,
			Port:    port,
			User:    user,
			Name:    name,
			SSLMode: "disable",
		}

		dsn := db.DSN()
		assert.Contains(t, dsn, host)
		assert.Contains(t, dsn, user)
		assert.Contains(t, dsn, name)
		assert.Contains(t, dsn, "disable")
	})
}
