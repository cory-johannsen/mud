# Game Client (Ebiten) Phase 1: Binary Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the cmd/ebitenclient binary scaffold with config loading, logging, and --version flag.

**Architecture:** Standalone Go binary with YAML config in OS user config dir. No Wire DI — simple manual wiring. Logging via zap.

**Tech Stack:** Go, go.uber.org/zap, github.com/spf13/viper, Ebiten v2 (added to go.mod)

---

## Requirements Covered

- REQ-GCE-1: A new `cmd/ebitenclient/` binary separate from `cmd/frontend/` and `cmd/webclient/`.
- REQ-GCE-2: `config.yaml` at `os.UserConfigDir()/mud-ebiten/config.yaml` with defaults; write defaults on first run; log warning if write fails, continue with defaults.
- REQ-GCE-4: Window title `"Mud"` — wired at binary entry point (full Ebiten window deferred to Phase 2; this phase establishes the title constant).
- REQ-GCE-5: Structured log output to `os.UserCacheDir()/mud-ebiten/client.log`; log level controlled by `log_level` config field.

---

## Files

| File | Action | Description |
|------|--------|-------------|
| `cmd/ebitenclient/main.go` | Create | Entry point: parse `--config`/`--version` flags, load config, init zap logger |
| `cmd/ebitenclient/config/config.go` | Create | `ClientConfig` struct, `Load()`, `Validate()`, defaults, first-run write |
| `cmd/ebitenclient/config/config_test.go` | Create | Table-driven + property-based tests for `Load` and `Validate` |
| `Makefile` | Modify | Add `build-ebiten` target |

---

## Tasks

### Task 1 — Add Ebiten to go.mod

- [ ] Run `mise exec -- go get github.com/hajimehoshi/ebiten/v2` from the repository root.
- [ ] Run `mise exec -- go mod tidy` to prune the module graph.
- [ ] Verify `go.mod` contains `github.com/hajimehoshi/ebiten/v2` and `go.sum` is updated.

**Acceptance:** `mise exec -- go list -m github.com/hajimehoshi/ebiten/v2` exits 0.

---

### Task 2 — ClientConfig: failing tests first (TDD)

Create `cmd/ebitenclient/config/config_test.go` with the following tests **before** writing the implementation.

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/config"
)

// TestValidate_Defaults verifies that a zero-value ClientConfig fails validation
// only when required fields are empty (log_level must be non-empty).
func TestValidate_Defaults(t *testing.T) {
	cfg := config.ClientConfig{
		WebClientURL:      "http://localhost:8080",
		GameServerAddr:    "localhost:50051",
		GitHubReleasesURL: "https://api.github.com/repos/cory-johannsen/mud/releases/latest",
		LogLevel:          "info",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid default config, got error: %v", err)
	}
}

// TestValidate_InvalidLogLevel verifies that an unrecognised log level is rejected.
func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := config.ClientConfig{
		WebClientURL:      "http://localhost:8080",
		GameServerAddr:    "localhost:50051",
		GitHubReleasesURL: "https://api.github.com/repos/cory-johannsen/mud/releases/latest",
		LogLevel:          "verbose",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid log level")
	}
}

// TestValidate_EmptyWebClientURL verifies that an empty WebClientURL is rejected.
func TestValidate_EmptyWebClientURL(t *testing.T) {
	cfg := config.ClientConfig{
		WebClientURL:      "",
		GameServerAddr:    "localhost:50051",
		GitHubReleasesURL: "https://api.github.com/repos/cory-johannsen/mud/releases/latest",
		LogLevel:          "info",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty WebClientURL")
	}
}

// TestValidate_EmptyGameServerAddr verifies that an empty GameServerAddr is rejected.
func TestValidate_EmptyGameServerAddr(t *testing.T) {
	cfg := config.ClientConfig{
		WebClientURL:      "http://localhost:8080",
		GameServerAddr:    "",
		GitHubReleasesURL: "https://api.github.com/repos/cory-johannsen/mud/releases/latest",
		LogLevel:          "info",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty GameServerAddr")
	}
}

// TestValidate_EmptyGitHubReleasesURL verifies that an empty GitHubReleasesURL is rejected.
func TestValidate_EmptyGitHubReleasesURL(t *testing.T) {
	cfg := config.ClientConfig{
		WebClientURL:      "http://localhost:8080",
		GameServerAddr:    "localhost:50051",
		GitHubReleasesURL: "",
		LogLevel:          "info",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty GitHubReleasesURL")
	}
}

// TestLoad_FileNotFound verifies that Load writes defaults and returns a valid
// ClientConfig when the config file does not exist.
func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load with missing file returned error: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config from missing file failed validation: %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level=info, got %q", cfg.LogLevel)
	}
	if cfg.WebClientURL != "http://localhost:8080" {
		t.Errorf("expected default webclient_url, got %q", cfg.WebClientURL)
	}
	if cfg.GameServerAddr != "localhost:50051" {
		t.Errorf("expected default gameserver_addr, got %q", cfg.GameServerAddr)
	}
	if cfg.GitHubReleasesURL != "https://api.github.com/repos/cory-johannsen/mud/releases/latest" {
		t.Errorf("expected default github_releases_url, got %q", cfg.GitHubReleasesURL)
	}
	// Verify defaults were written to disk.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected defaults to be written to %s", path)
	}
}

// TestLoad_ExistingFile verifies that Load reads values from an existing YAML file.
func TestLoad_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `webclient_url: "http://game.example.com"
gameserver_addr: "game.example.com:50051"
github_releases_url: "https://api.github.com/repos/cory-johannsen/mud/releases/latest"
log_level: "debug"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebClientURL != "http://game.example.com" {
		t.Errorf("expected webclient_url from file, got %q", cfg.WebClientURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level=debug, got %q", cfg.LogLevel)
	}
}

// TestLoad_InvalidLogLevelInFile verifies that Load returns an error for an
// invalid log_level value in the YAML file.
func TestLoad_InvalidLogLevelInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `webclient_url: "http://localhost:8080"
gameserver_addr: "localhost:50051"
github_releases_url: "https://api.github.com/repos/cory-johannsen/mud/releases/latest"
log_level: "trace"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	if _, err := config.Load(path); err == nil {
		t.Fatal("expected error for invalid log_level in file")
	}
}
```

Run tests (expect compile failure since the package does not exist yet):

```
mise exec -- go test ./cmd/ebitenclient/config/... -v 2>&1 | head -20
```

**Acceptance:** Tests fail with `cannot find package` or compilation errors — confirming red phase.

---

### Task 3 — ClientConfig implementation

Create `cmd/ebitenclient/config/config.go`:

```go
// Package config provides configuration loading for the ebitenclient binary.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ClientConfig holds all configuration for the ebitenclient binary.
//
// Precondition: All fields are populated by Load() before use.
type ClientConfig struct {
	WebClientURL      string `mapstructure:"webclient_url"      yaml:"webclient_url"`
	GameServerAddr    string `mapstructure:"gameserver_addr"    yaml:"gameserver_addr"`
	GitHubReleasesURL string `mapstructure:"github_releases_url" yaml:"github_releases_url"`
	LogLevel          string `mapstructure:"log_level"           yaml:"log_level"`
}

// defaults returns a ClientConfig populated with production-safe defaults.
//
// Postcondition: Returned config satisfies Validate().
func defaults() ClientConfig {
	return ClientConfig{
		WebClientURL:      "http://localhost:8080",
		GameServerAddr:    "localhost:50051",
		GitHubReleasesURL: "https://api.github.com/repos/cory-johannsen/mud/releases/latest",
		LogLevel:          "info",
	}
}

// Validate checks all invariants of the ClientConfig.
//
// Postcondition: Returns nil only when all fields are populated and valid.
func (c ClientConfig) Validate() error {
	var errs []string
	if c.WebClientURL == "" {
		errs = append(errs, "webclient_url must not be empty")
	}
	if c.GameServerAddr == "" {
		errs = append(errs, "gameserver_addr must not be empty")
	}
	if c.GitHubReleasesURL == "" {
		errs = append(errs, "github_releases_url must not be empty")
	}
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.LogLevel] {
		errs = append(errs, fmt.Sprintf("log_level must be one of [debug, info, warn, error], got %q", c.LogLevel))
	}
	if len(errs) > 0 {
		return fmt.Errorf("client config validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Load reads ClientConfig from path.
//
// Precondition: path is an absolute or relative file path.
// Postcondition: If path does not exist, defaults are written to path (a warning
// is logged to stderr if the write fails) and the defaults are returned.
// If the file exists, its values override defaults. Validation is always applied.
func Load(path string) (ClientConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("MUD_EBITEN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	d := defaults()
	v.SetDefault("webclient_url", d.WebClientURL)
	v.SetDefault("gameserver_addr", d.GameServerAddr)
	v.SetDefault("github_releases_url", d.GitHubReleasesURL)
	v.SetDefault("log_level", d.LogLevel)

	if err := v.ReadInConfig(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// Viper wraps os.ErrNotExist in a ConfigFileNotFoundError; check by string for safety.
			if !isNotFound(err) {
				return ClientConfig{}, fmt.Errorf("reading config: %w", err)
			}
		}
		// First run: write defaults to disk.
		writeDefaults(path, d)
	}

	var cfg ClientConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return ClientConfig{}, fmt.Errorf("unmarshalling config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return ClientConfig{}, err
	}
	return cfg, nil
}

// isNotFound returns true when err represents a missing config file from Viper.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such file") ||
		strings.Contains(err.Error(), "Not Found") ||
		errors.Is(err, os.ErrNotExist)
}

// writeDefaults writes the default ClientConfig as YAML to path, creating
// parent directories as needed. On failure, a warning is written to stderr.
func writeDefaults(path string, cfg ClientConfig) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not create config dir %s: %v\n", filepath.Dir(path), err)
		return
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not marshal default config: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not write default config to %s: %v\n", path, err)
	}
}
```

Add `gopkg.in/yaml.v3` if not already present:

```
mise exec -- go get gopkg.in/yaml.v3
mise exec -- go mod tidy
```

Run tests (expect green):

```
mise exec -- go test ./cmd/ebitenclient/config/... -v
```

**Acceptance:** All tests in `config_test.go` pass.

---

### Task 4 — cmd/ebitenclient/main.go

Create `cmd/ebitenclient/main.go`:

```go
// Package main is the entry point for the ebitenclient native game client.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	clientconfig "github.com/cory-johannsen/mud/cmd/ebitenclient/config"
	"github.com/cory-johannsen/mud/internal/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// WindowTitle is the base window title used on the login and character-select screens (REQ-GCE-4).
const WindowTitle = "Mud"

func main() {
	var (
		configPath  string
		showVersion bool
	)

	flag.StringVar(&configPath, "config", defaultConfigPath(), "path to config.yaml")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	cfg, err := clientconfig.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: loading config: %v\n", err)
		os.Exit(1)
	}

	logger, err := buildLogger(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: building logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	logger.Info("ebitenclient starting",
		zap.String("version", version.Version),
		zap.String("config", configPath),
		zap.String("log_level", cfg.LogLevel),
		zap.String("webclient_url", cfg.WebClientURL),
		zap.String("gameserver_addr", cfg.GameServerAddr),
	)

	// Phase 2 will start the Ebiten window here.
	// For Phase 1 we verify the binary wires correctly and exits cleanly.
	logger.Info("Phase 1 scaffold complete — Ebiten window not yet started")
}

// defaultConfigPath returns os.UserConfigDir()/mud-ebiten/config.yaml.
//
// Postcondition: Returns a non-empty path; falls back to ./config.yaml if
// os.UserConfigDir() fails.
func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(dir, "mud-ebiten", "config.yaml")
}

// buildLogger constructs a zap.Logger that writes structured JSON to the log
// file at os.UserCacheDir()/mud-ebiten/client.log, respecting the given level.
//
// Precondition: level must be one of "debug", "info", "warn", "error".
// Postcondition: Returns a non-nil logger or a non-nil error.
func buildLogger(level string) (*zap.Logger, error) {
	logPath, err := logFilePath()
	if err != nil {
		return nil, fmt.Errorf("resolving log path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating log dir: %w", err)
	}

	zapLevel, err := parseZapLevel(level)
	if err != nil {
		return nil, err
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	fileCfg := zap.NewProductionConfig()
	fileCfg.Level = zap.NewAtomicLevelAt(zapLevel)
	fileCfg.EncoderConfig = encoderCfg
	fileCfg.OutputPaths = []string{logPath}
	fileCfg.ErrorOutputPaths = []string{logPath}

	logger, err := fileCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building zap logger: %w", err)
	}
	return logger, nil
}

// logFilePath returns os.UserCacheDir()/mud-ebiten/client.log.
func logFilePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("os.UserCacheDir: %w", err)
	}
	return filepath.Join(dir, "mud-ebiten", "client.log"), nil
}

// parseZapLevel converts a level string to a zapcore.Level.
func parseZapLevel(level string) (zapcore.Level, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	return l, nil
}
```

Build and smoke-test:

```
mise exec -- go build -trimpath -ldflags "-X github.com/cory-johannsen/mud/internal/version.Version=test-phase1" \
  -o bin/ebitenclient ./cmd/ebitenclient
./bin/ebitenclient --version
```

**Acceptance:** `--version` prints `test-phase1` and exits 0.

---

### Task 5 — Add build-ebiten to Makefile

Edit `Makefile`:

- [ ] Add `build-ebiten` to the `.PHONY` line.
- [ ] Add the following target after `build-seed-claude-accounts`:

```makefile
build-ebiten:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/ebitenclient ./cmd/ebitenclient
```

Verify:

```
mise exec -- make build-ebiten
./bin/ebitenclient --version
```

**Acceptance:** `make build-ebiten` succeeds; `--version` exits 0 and prints the version string.

---

### Task 6 — Full test suite

Run the full test suite to confirm no regressions:

```
mise exec -- go test ./cmd/ebitenclient/... -v
```

Then run the broader suite:

```
mise exec -- go build ./...
```

**Acceptance:** All tests pass; `go build ./...` exits 0.

---

## Definition of Done

- [ ] Task 1: `go.mod` contains `github.com/hajimehoshi/ebiten/v2`.
- [ ] Task 2: Test file exists with all listed test functions; tests compile and fail before implementation.
- [ ] Task 3: All `config_test.go` tests pass.
- [ ] Task 4: `./bin/ebitenclient --version` exits 0 and prints the injected version string; binary starts, logs Phase 1 message, and exits.
- [ ] Task 5: `make build-ebiten` target exists and succeeds.
- [ ] Task 6: `go test ./cmd/ebitenclient/...` and `go build ./...` exit 0.
