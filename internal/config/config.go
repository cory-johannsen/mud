// Package config provides Viper-based configuration loading for the MUD server.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ServerConfig holds top-level server settings.
type ServerConfig struct {
	// Mode is the server operation mode: "standalone", "frontend", or "backend".
	Mode string `mapstructure:"mode"`
	// Type is the Pitaya server type identifier.
	Type string `mapstructure:"type"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxConns        int32         `mapstructure:"max_conns"`
	MinConns        int32         `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
}

// DSN returns the PostgreSQL connection string.
//
// Precondition: Host, Port, User, and Name must be non-empty.
// Postcondition: Returns a valid PostgreSQL DSN string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// TelnetConfig holds Telnet acceptor settings.
type TelnetConfig struct {
	// Host is the bind address for the Telnet listener.
	Host string `mapstructure:"host"`
	// Port is the TCP port for the Telnet listener.
	Port int `mapstructure:"port"`
	// ReadTimeout is the per-read timeout for Telnet connections.
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	// WriteTimeout is the per-write timeout for Telnet connections.
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	// IdleTimeout is the duration of inactivity after which a warning is sent.
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`
	// IdleGracePeriod is the additional duration after IdleTimeout before disconnecting.
	IdleGracePeriod time.Duration `mapstructure:"idle_grace_period"`
}

// Addr returns the "host:port" listen address.
//
// Postcondition: Returns a non-empty string in "host:port" format.
func (t TelnetConfig) Addr() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	// Level is the minimum log level: "debug", "info", "warn", "error".
	Level string `mapstructure:"level"`
	// Format is the log output format: "json" or "console".
	Format string `mapstructure:"format"`
}

// GameServerConfig holds game server gRPC connection settings.
type GameServerConfig struct {
	// GRPCHost is the bind/connect address for the game server gRPC service.
	GRPCHost string `mapstructure:"grpc_host"`
	// GRPCPort is the TCP port for the game server gRPC service.
	GRPCPort int `mapstructure:"grpc_port"`
	// RoundDurationMs is the combat round timer duration in milliseconds.
	RoundDurationMs int `mapstructure:"round_duration_ms"`
}

// Addr returns the "host:port" gRPC address.
//
// Postcondition: Returns a non-empty string in "host:port" format.
func (g GameServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", g.GRPCHost, g.GRPCPort)
}

// Config is the top-level application configuration.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Telnet     TelnetConfig     `mapstructure:"telnet"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	GameServer GameServerConfig `mapstructure:"gameserver"`
}

// Validate checks all configuration invariants.
//
// Postcondition: Returns nil if configuration is valid, or an error describing all violations.
func (c Config) Validate() error {
	var errs []string

	if err := validateServer(c.Server); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateDatabase(c.Database); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateTelnet(c.Telnet); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateLogging(c.Logging); err != nil {
		errs = append(errs, err.Error())
	}

	if err := validateGameServer(c.GameServer); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func validateServer(s ServerConfig) error {
	validModes := map[string]bool{"standalone": true, "frontend": true, "backend": true}
	if !validModes[s.Mode] {
		return fmt.Errorf("server.mode must be one of [standalone, frontend, backend], got %q", s.Mode)
	}
	if s.Type == "" {
		return errors.New("server.type must not be empty")
	}
	return nil
}

func validateDatabase(d DatabaseConfig) error {
	var errs []string
	if d.Host == "" {
		errs = append(errs, "database.host must not be empty")
	}
	if d.Port < 1 || d.Port > 65535 {
		errs = append(errs, fmt.Sprintf("database.port must be 1-65535, got %d", d.Port))
	}
	if d.User == "" {
		errs = append(errs, "database.user must not be empty")
	}
	if d.Name == "" {
		errs = append(errs, "database.name must not be empty")
	}
	validSSL := map[string]bool{"disable": true, "require": true, "verify-ca": true, "verify-full": true}
	if !validSSL[d.SSLMode] {
		errs = append(errs, fmt.Sprintf("database.sslmode must be one of [disable, require, verify-ca, verify-full], got %q", d.SSLMode))
	}
	if d.MaxConns < 1 {
		errs = append(errs, fmt.Sprintf("database.max_conns must be >= 1, got %d", d.MaxConns))
	}
	if d.MinConns < 0 {
		errs = append(errs, fmt.Sprintf("database.min_conns must be >= 0, got %d", d.MinConns))
	}
	if d.MinConns > d.MaxConns {
		errs = append(errs, "database.min_conns must not exceed database.max_conns")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateTelnet(t TelnetConfig) error {
	var errs []string
	if t.Port < 1 || t.Port > 65535 {
		errs = append(errs, fmt.Sprintf("telnet.port must be 1-65535, got %d", t.Port))
	}
	if t.ReadTimeout < 0 {
		errs = append(errs, "telnet.read_timeout must not be negative")
	}
	if t.WriteTimeout < 0 {
		errs = append(errs, "telnet.write_timeout must not be negative")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateGameServer(g GameServerConfig) error {
	var errs []string
	if g.GRPCHost == "" {
		errs = append(errs, "gameserver.grpc_host must not be empty")
	}
	if g.GRPCPort < 1 || g.GRPCPort > 65535 {
		errs = append(errs, fmt.Sprintf("gameserver.grpc_port must be 1-65535, got %d", g.GRPCPort))
	}
	if g.RoundDurationMs < 0 {
		errs = append(errs, fmt.Sprintf("gameserver.round_duration_ms must be >= 0 (got %d)", g.RoundDurationMs))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateLogging(l LoggingConfig) error {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[l.Level] {
		return fmt.Errorf("logging.level must be one of [debug, info, warn, error], got %q", l.Level)
	}
	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[l.Format] {
		return fmt.Errorf("logging.format must be one of [json, console], got %q", l.Format)
	}
	return nil
}

// Load reads configuration from the given file path, applies environment variable
// overrides, and validates the result.
//
// Precondition: path must be a valid file path to a YAML configuration file.
// Postcondition: Returns a valid Config or a non-nil error.
func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	// Environment variable overrides with MUD_ prefix
	v.SetEnvPrefix("MUD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// LoadFromViper builds a Config from an already-configured Viper instance.
//
// Precondition: v must be non-nil and have configuration values set.
// Postcondition: Returns a valid Config or a non-nil error.
func LoadFromViper(v *viper.Viper) (Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshalling config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.mode", "standalone")
	v.SetDefault("server.type", "mud")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "mud")
	v.SetDefault("database.password", "mud")
	v.SetDefault("database.name", "mud")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_conns", 10)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("database.max_conn_lifetime", "1h")

	v.SetDefault("telnet.host", "0.0.0.0")
	v.SetDefault("telnet.port", 4000)
	v.SetDefault("telnet.read_timeout", "5m")
	v.SetDefault("telnet.write_timeout", "30s")
	v.SetDefault("telnet.idle_timeout", "5m")
	v.SetDefault("telnet.idle_grace_period", "1m")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	v.SetDefault("gameserver.grpc_host", "127.0.0.1")
	v.SetDefault("gameserver.grpc_port", 50051)
}
