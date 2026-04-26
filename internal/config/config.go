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
//
// As of the telnet-deprecation rollout (#325), the player-facing telnet
// surface is retired. The player port (Host:Port) defaults to a rejector
// that prints a redirect to the web client and disconnects. The headless
// debug port (HeadlessPort) is bound loopback-only and accepts only
// seed-authorized connections (accounts seeded by seed-claude-accounts).
//
// Operators may temporarily re-enable the legacy player flow for graceful
// sunset by setting AllowGameCommands=true; this MUST NOT be enabled in
// production.
type TelnetConfig struct {
	// Host is the bind address for the Telnet listener. Default: 127.0.0.1.
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
	// HeadlessPort is the TCP port for the headless plain-text telnet listener.
	// If 0 or absent, the headless listener is not started. The headless
	// listener is always bound to 127.0.0.1 regardless of Host.
	HeadlessPort int `mapstructure:"headless_port"`
	// AllowGameCommands re-enables the legacy player-facing telnet flow.
	// Default: false. When false, the player port emits a redirect message
	// and closes; the auth handler also refuses login attempts as a
	// belt-and-suspenders gate. Set to true only for time-bounded graceful
	// sunset operations. MUST NOT be true in production.
	AllowGameCommands bool `mapstructure:"allow_game_commands"`
	// WebClientURL is the URL announced in the rejector message when
	// AllowGameCommands is false. Default: https://gunchete.local.
	WebClientURL string `mapstructure:"web_client_url"`
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

const (
	// DefaultReactionPromptTimeout is the interactive reaction-prompt timeout per REACTION-13.
	DefaultReactionPromptTimeout = 3 * time.Second
	reactionPromptTimeoutMin     = 500 * time.Millisecond
	reactionPromptTimeoutMax     = 30 * time.Second
)

// GameServerConfig holds game server gRPC connection settings.
type GameServerConfig struct {
	// GRPCHost is the bind/connect address for the game server gRPC service.
	GRPCHost string `mapstructure:"grpc_host"`
	// GRPCPort is the TCP port for the game server gRPC service.
	GRPCPort int `mapstructure:"grpc_port"`
	// RoundDurationMs is the combat round timer duration in milliseconds.
	RoundDurationMs int `mapstructure:"round_duration_ms"`
	// GameClockStart is the game hour (0-23) at server startup.
	GameClockStart int `mapstructure:"game_clock_start"`
	// GameTickDuration is how long each game hour lasts in real time.
	GameTickDuration time.Duration `mapstructure:"game_tick_duration"`
	// AutoNavStepMs is the delay in milliseconds between auto-navigation steps in the web client.
	// Minimum 100. Default 1000. (REQ-CNT-2)
	AutoNavStepMs int `mapstructure:"auto_nav_step_ms"`
	// ReactionPromptTimeout is the maximum time a player has to respond to a reaction prompt.
	// Valid range [500ms, 30s]. Zero or out-of-range values default to DefaultReactionPromptTimeout.
	// Per REACTION-12 and REACTION-13.
	ReactionPromptTimeout time.Duration `mapstructure:"reaction_prompt_timeout"`
}

// ValidateReactionPromptTimeout clamps ReactionPromptTimeout to [500ms, 30s].
// Zero or out-of-range values are replaced with DefaultReactionPromptTimeout.
func (g *GameServerConfig) ValidateReactionPromptTimeout() {
	if g.ReactionPromptTimeout == 0 ||
		g.ReactionPromptTimeout < reactionPromptTimeoutMin ||
		g.ReactionPromptTimeout > reactionPromptTimeoutMax {
		g.ReactionPromptTimeout = DefaultReactionPromptTimeout
	}
}

// Addr returns the "host:port" gRPC address.
//
// Postcondition: Returns a non-empty string in "host:port" format.
func (g GameServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", g.GRPCHost, g.GRPCPort)
}

// WeatherConfig holds weather engine settings.
type WeatherConfig struct {
	// ChancePerTick is the probability [0,1] of a weather change occurring each game tick.
	ChancePerTick float64 `mapstructure:"chance_per_tick"`
	// ContentFile is the path to the weather type definitions YAML file.
	ContentFile string `mapstructure:"content_file"`
}

// HotbarConfig holds hotbar layout settings.
type HotbarConfig struct {
	// MaxHotbars is the maximum number of hotbars a player may configure.
	MaxHotbars int `mapstructure:"max_hotbars"`
}

// WebConfig holds HTTP web server settings.
type WebConfig struct {
	// Port is the TCP port for the web HTTP server. Default: 0 (disabled). Set to 0 to disable.
	Port int `mapstructure:"port"`
	// JWTSecret is the HS256 signing secret for JWT tokens. Required when Port > 0.
	JWTSecret string `mapstructure:"jwt_secret"`
}

// Validate checks WebConfig invariants.
//
// Postcondition: Returns nil if valid, or a non-nil error describing violations.
func (w WebConfig) Validate() error {
	var errs []string
	if w.Port != 0 && (w.Port < 1 || w.Port > 65535) {
		errs = append(errs, fmt.Sprintf("web.port must be 1-65535 or 0 (disabled), got %d", w.Port))
	}
	// JWTSecret is only required when the web server is enabled (Port > 0).
	if w.Port > 0 && w.JWTSecret == "" {
		errs = append(errs, "web.jwt_secret must not be empty when web server is enabled")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// Config is the top-level application configuration.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Telnet     TelnetConfig     `mapstructure:"telnet"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	GameServer GameServerConfig `mapstructure:"gameserver"`
	Web        WebConfig        `mapstructure:"web"`
	Weather    WeatherConfig    `mapstructure:"weather"`
	Hotbar     HotbarConfig     `mapstructure:"hotbar"`
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

	if err := validateWeb(c.Web); err != nil {
		errs = append(errs, err.Error())
	}

	if err := validateWeather(c.Weather); err != nil {
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
	// REQ-TD-1c: with the player flow retired by default, the public telnet
	// port runs the rejector that refuses player auth and disconnects with a
	// pointer to the web client. The bind address may be 0.0.0.0 in
	// containerised deployments so the K8s readiness probe (and any in-cluster
	// caller) can reach the rejector. The Service is ClusterIP so external
	// access is still blocked at the K8s edge. The headless port (Telnet.HeadlessPort)
	// is always bound to 127.0.0.1 regardless of Host.
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
	if g.GameClockStart < 0 || g.GameClockStart > 23 {
		errs = append(errs, fmt.Sprintf("gameserver.game_clock_start must be 0-23, got %d", g.GameClockStart))
	}
	if g.GameTickDuration <= 0 {
		errs = append(errs, fmt.Sprintf("gameserver.game_tick_duration must be positive, got %v", g.GameTickDuration))
	}
	if g.AutoNavStepMs != 0 && g.AutoNavStepMs < 100 {
		errs = append(errs, fmt.Sprintf("gameserver.auto_nav_step_ms must be >= 100, got %d", g.AutoNavStepMs))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validateWeb(w WebConfig) error {
	return w.Validate()
}

func validateWeather(w WeatherConfig) error {
	var errs []string
	if w.ChancePerTick <= 0 || w.ChancePerTick > 1.0 {
		errs = append(errs, fmt.Sprintf("weather.chance_per_tick must be in (0.0, 1.0], got %v", w.ChancePerTick))
	}
	if w.ContentFile == "" {
		errs = append(errs, "weather.content_file must not be empty")
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

	cfg.GameServer.ValidateReactionPromptTimeout()

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
	cfg.GameServer.ValidateReactionPromptTimeout()
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

	// REQ-TD-1c: default to loopback now that the player flow is retired.
	v.SetDefault("telnet.host", "127.0.0.1")
	v.SetDefault("telnet.port", 4000)
	v.SetDefault("telnet.read_timeout", "5m")
	v.SetDefault("telnet.write_timeout", "30s")
	v.SetDefault("telnet.idle_timeout", "4m")
	v.SetDefault("telnet.idle_grace_period", "30s")
	v.SetDefault("telnet.headless_port", 4002)
	v.SetDefault("telnet.allow_game_commands", false)
	v.SetDefault("telnet.web_client_url", "https://gunchete.local")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	v.SetDefault("gameserver.grpc_host", "127.0.0.1")
	v.SetDefault("gameserver.grpc_port", 50051)
	v.SetDefault("gameserver.game_clock_start", 6)
	v.SetDefault("gameserver.game_tick_duration", "1m")
	v.SetDefault("gameserver.auto_nav_step_ms", 1000)

	v.SetDefault("web.port", 0)

	v.SetDefault("weather.chance_per_tick", 0.05)
	v.SetDefault("weather.content_file", "content/weather.yaml")

	v.SetDefault("hotbar.max_hotbars", 4)
}
