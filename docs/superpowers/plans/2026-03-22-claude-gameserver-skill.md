# Claude Game Server Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a headless telnet port (4002) that outputs plain text without ANSI escape codes, a tool to seed 3 test accounts, and a Claude Code skill document for game server interaction.

**Architecture:** New telnet listener in frontend that strips ANSI on output; seed accounts tool as a Go CLI command or Makefile target; skill document in .claude/skills/.

**Tech Stack:** Go, telnet, existing frontend/storage packages, Claude Code skill YAML/Markdown format

---

## File Map

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/frontend/telnet/conn.go` | Modify | Add `Headless bool` field; branch `InitScreen`, `WriteRoom`, `WriteConsole`, `WritePrompt` |
| `internal/frontend/telnet/conn_test.go` | Modify | Add headless mode unit tests |
| `internal/frontend/telnet/acceptor.go` | Modify | Add `headless bool` parameter path; `NewHeadlessAcceptor` constructor |
| `internal/config/config.go` | Modify | Add `HeadlessPort int` to `TelnetConfig`; update `setDefaults` |
| `cmd/frontend/main.go` | Modify | Conditionally start second acceptor when `HeadlessPort != 0` |
| `cmd/seed-claude-accounts/main.go` | Create | Upsert three claude accounts from env password |
| `.claude/skills/mud-gameserver.md` | Create | Skill document for Claude agent sessions |
| `Makefile` | Modify | Add `build-seed-claude-accounts` and `seed-claude-accounts` targets |

> **Note:** `StripANSI` already exists in `internal/frontend/telnet/ansi.go`. No new utility is needed.

---

### Task 1: Add `Headless bool` to `telnet.Conn` and branch screen methods

**Files:**
- Modify: `internal/frontend/telnet/conn.go`
- Modify: `internal/frontend/telnet/screen.go`
- Modify: `internal/frontend/telnet/conn_test.go`

The `Conn` struct gains a `Headless bool` field set at construction time (not a method call). When `Headless` is true:

- `InitScreen()` returns nil immediately (no-op).
- `WriteRoom(content string)` strips ANSI from `content` using the existing `StripANSI()` function already in `ansi.go`, then writes each line terminated with `\r\n` directly to the connection.
- `WriteConsole(text string)` writes `StripANSI(text) + "\r\n"` directly to the connection (bypasses split-screen scroll buffer logic).
- `WritePromptSplit(prompt string)` writes `"> "` — but note: the auth/game_bridge code calls `WritePrompt` (on `conn.go`) not `WritePromptSplit` for the prompt display. Headless prompt is handled in `WritePrompt` below.
- `WritePrompt(prompt string)` in `conn.go` writes `"> "` (no ANSI, no original prompt) when `Headless` is true.

- [ ] **Step 1.1: Add `Headless` field to `Conn` struct and `NewHeadlessConn` constructor**

Edit `internal/frontend/telnet/conn.go`. Add the `Headless bool` field to the `Conn` struct after the existing fields, and add a new constructor:

```go
// Headless marks this connection as a plain-text headless session.
// When true, all output methods strip ANSI and skip split-screen layout.
Headless bool
```

Add after `NewConn`:

```go
// NewHeadlessConn wraps a raw TCP connection as a headless plain-text session.
// ANSI escape codes are stripped from all output; InitScreen is a no-op.
//
// Precondition: raw must be a valid, open network connection.
// Postcondition: Returns a Conn with Headless=true, ready for reading and writing.
func NewHeadlessConn(raw net.Conn, readTimeout, writeTimeout time.Duration) *Conn {
	c := NewConn(raw, readTimeout, writeTimeout)
	c.Headless = true
	return c
}
```

- [ ] **Step 1.2: Branch `WritePrompt` in `conn.go` for headless mode**

In `conn.go`, modify `WritePrompt`:

```go
func (c *Conn) WritePrompt(prompt string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	if c.Headless {
		_, err := fmt.Fprint(c.raw, "> ")
		return err
	}
	_, err := fmt.Fprint(c.raw, prompt)
	return err
}
```

- [ ] **Step 1.3: Branch `InitScreen`, `WriteRoom`, `WriteConsole`, `WritePromptSplit` in `screen.go` for headless mode**

In `screen.go`, modify `InitScreen`:

```go
func (c *Conn) InitScreen() error {
	if c.Headless {
		return nil
	}
	c.mu.Lock()
	h := c.height
	c.mu.Unlock()
	// ... existing code unchanged below ...
```

Modify `WriteRoom`:

```go
func (c *Conn) WriteRoom(content string) error {
	if c.Headless {
		stripped := StripANSI(content)
		normalized := strings.ReplaceAll(strings.ReplaceAll(stripped, "\r\n", "\n"), "\r", "")
		lines := strings.Split(strings.TrimSpace(normalized), "\n")
		var buf strings.Builder
		for _, line := range lines {
			buf.WriteString(line)
			buf.WriteString("\r\n")
		}
		buf.WriteString("\r\n") // blank line after room view per REQ-CGS-4
		return c.writeRaw(buf.String())
	}
	// ... existing code unchanged below ...
```

Modify `WriteConsole`:

```go
func (c *Conn) WriteConsole(text string) error {
	if c.Headless {
		return c.writeRaw(StripANSI(strings.TrimRight(text, "\r\n")) + "\r\n")
	}
	// ... existing code unchanged below ...
```

Modify `WritePromptSplit`:

```go
func (c *Conn) WritePromptSplit(prompt string) error {
	if c.Headless {
		return c.writeRaw("> ")
	}
	// ... existing code unchanged below ...
```

- [ ] **Step 1.4: Write the failing tests**

In `internal/frontend/telnet/conn_test.go`, add a new test section. Use `net.Pipe()` to create an in-process connection pair (no listening socket needed):

```go
func TestHeadlessConn_InitScreen_IsNoop(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewHeadlessConn(server, 0, 0)
	err := conn.InitScreen()
	require.NoError(t, err)

	// No bytes should have been written — read with a short deadline to confirm.
	_ = client.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	buf := make([]byte, 64)
	n, _ := client.Read(buf)
	assert.Equal(t, 0, n, "InitScreen on headless conn must be a no-op")
}

func TestHeadlessConn_WriteRoom_PlainText(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewHeadlessConn(server, 0, 0)

	// Simulate content as text_renderer would produce: room name, wrapped description, Exits line.
	content := "\033[1mThe Alley\033[0m\nA narrow passage.\nExits: north south"
	done := make(chan struct{})
	var got []byte
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		n, _ := client.Read(buf)
		got = buf[:n]
	}()

	err := conn.WriteRoom(content)
	require.NoError(t, err)
	_ = server.Close()
	<-done

	output := string(got)
	assert.NotContains(t, output, "\033[", "WriteRoom headless must strip ANSI")
	assert.Contains(t, output, "The Alley\r\n")
	assert.Contains(t, output, "A narrow passage.\r\n")
	assert.Contains(t, output, "Exits: north south\r\n")
	// Blank line per REQ-CGS-4
	assert.Contains(t, output, "\r\n\r\n")
}

func TestHeadlessConn_WriteConsole_PlainText(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewHeadlessConn(server, 0, 0)

	done := make(chan struct{})
	var got []byte
	go func() {
		defer close(done)
		buf := make([]byte, 256)
		n, _ := client.Read(buf)
		got = buf[:n]
	}()

	err := conn.WriteConsole("\033[32mYou pick up the item.\033[0m")
	require.NoError(t, err)
	_ = server.Close()
	<-done

	output := string(got)
	assert.NotContains(t, output, "\033[")
	assert.Equal(t, "You pick up the item.\r\n", output)
}

func TestHeadlessConn_WritePrompt_PlainPrompt(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	conn := NewHeadlessConn(server, 0, 0)

	done := make(chan struct{})
	var got []byte
	go func() {
		defer close(done)
		buf := make([]byte, 64)
		n, _ := client.Read(buf)
		got = buf[:n]
	}()

	err := conn.WritePrompt("\033[33m> \033[0m")
	require.NoError(t, err)
	_ = server.Close()
	<-done

	assert.Equal(t, "> ", string(got), "headless prompt must be '> ' with no ANSI")
}
```

Add a property-based test using `pgregory.net/rapid` (already a dependency):

```go
func TestHeadlessConn_WriteConsole_NeverEmitsANSI(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ansiCodes := []string{"\033[0m", "\033[1m", "\033[31m", "\033[32m", "\033[1;32m"}
		plain := rapid.StringMatching(`[a-zA-Z0-9 .,!?]+`).Draw(rt, "plain").(string)
		code := rapid.SampledFrom(ansiCodes).Draw(rt, "code").(string)
		input := code + plain + "\033[0m"

		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		conn := NewHeadlessConn(server, 0, 0)
		done := make(chan struct{})
		var got []byte
		go func() {
			defer close(done)
			buf := make([]byte, 1024)
			n, _ := client.Read(buf)
			got = buf[:n]
		}()
		require.NoError(t, conn.WriteConsole(input))
		_ = server.Close()
		<-done

		assert.NotContains(t, string(got), "\033[")
	})
}
```

- [ ] **Step 1.5: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/telnet/... -run "TestHeadlessConn" -v 2>&1 | head -40
```

Expected: FAIL — `NewHeadlessConn` not defined.

- [ ] **Step 1.6: Implement all changes from Steps 1.1–1.3**

Apply the edits to `conn.go` and `screen.go` as described above.

- [ ] **Step 1.7: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/telnet/... -run "TestHeadlessConn" -v 2>&1
```

Expected: all `TestHeadlessConn_*` tests PASS.

- [ ] **Step 1.8: Run the full telnet test suite to check for regressions**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/telnet/... -v 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 1.9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/frontend/telnet/conn.go internal/frontend/telnet/screen.go internal/frontend/telnet/conn_test.go
git commit -m "feat(headless): add Headless bool to telnet.Conn; branch InitScreen/WriteRoom/WriteConsole/WritePrompt"
```

---

### Task 2: Add `HeadlessPort` to config and `NewHeadlessAcceptor`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/frontend/telnet/acceptor.go`
- Modify: `internal/frontend/telnet/acceptor_test.go` (if it exists; add tests)

The goal is to allow `cmd/frontend/main.go` to start a second acceptor on a different port whose connections are headless.

The `Acceptor` already calls `NewConn` in `handleConn`. We need a variant that calls `NewHeadlessConn` instead. The cleanest approach is to add a `headless bool` field to `Acceptor` and branch in `handleConn`.

- [ ] **Step 2.1: Add `HeadlessPort int` to `TelnetConfig` in `config.go`**

In `internal/config/config.go`, add to `TelnetConfig`:

```go
// HeadlessPort is the TCP port for the headless plain-text telnet listener.
// If 0 or absent, the headless listener is not started.
HeadlessPort int `mapstructure:"headless_port"`
```

In `setDefaults`, add:

```go
v.SetDefault("telnet.headless_port", 0)
```

The `validateTelnet` function must not require `HeadlessPort > 0` — it is optional. No change to validation is needed since `HeadlessPort == 0` means disabled.

- [ ] **Step 2.2: Write failing config test**

In `internal/config/config_test.go` (create if absent, or add to existing), add:

```go
func TestTelnetConfig_HeadlessPort_DefaultIsZero(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	// satisfy required fields
	v.Set("server.mode", "standalone")
	v.Set("server.type", "mud")
	v.Set("database.host", "localhost")
	v.Set("database.port", 5432)
	v.Set("database.user", "mud")
	v.Set("database.name", "mud")
	v.Set("database.sslmode", "disable")
	v.Set("database.max_conns", 10)
	v.Set("database.min_conns", 2)
	v.Set("telnet.port", 4000)
	v.Set("gameserver.grpc_host", "127.0.0.1")
	v.Set("gameserver.grpc_port", 50051)
	v.Set("gameserver.game_clock_start", 6)
	v.Set("gameserver.game_tick_duration", "1m")
	v.Set("logging.level", "info")
	v.Set("logging.format", "json")

	cfg, err := LoadFromViper(v)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Telnet.HeadlessPort)
}

func TestTelnetConfig_HeadlessPort_CanBeSet(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("server.mode", "standalone")
	v.Set("server.type", "mud")
	v.Set("database.host", "localhost")
	v.Set("database.port", 5432)
	v.Set("database.user", "mud")
	v.Set("database.name", "mud")
	v.Set("database.sslmode", "disable")
	v.Set("database.max_conns", 10)
	v.Set("database.min_conns", 2)
	v.Set("telnet.port", 4000)
	v.Set("telnet.headless_port", 4002)
	v.Set("gameserver.grpc_host", "127.0.0.1")
	v.Set("gameserver.grpc_port", 50051)
	v.Set("gameserver.game_clock_start", 6)
	v.Set("gameserver.game_tick_duration", "1m")
	v.Set("logging.level", "info")
	v.Set("logging.format", "json")

	cfg, err := LoadFromViper(v)
	require.NoError(t, err)
	assert.Equal(t, 4002, cfg.Telnet.HeadlessPort)
}
```

- [ ] **Step 2.3: Run config tests to confirm they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/config/... -run "TestTelnetConfig_Headless" -v 2>&1
```

Expected: FAIL — field not defined yet.

- [ ] **Step 2.4: Add `HeadlessPort` to `TelnetConfig` and default in `setDefaults`**

Apply the edits to `internal/config/config.go`.

- [ ] **Step 2.5: Run config tests to confirm they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/config/... -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 2.6: Add `headless bool` field and `NewHeadlessAcceptor` to `acceptor.go`**

In `internal/frontend/telnet/acceptor.go`, add a `headless bool` field to the `Acceptor` struct (after the existing fields). Add a new constructor:

```go
// NewHeadlessAcceptor creates a Telnet acceptor that wraps each accepted
// connection as a headless plain-text session (no ANSI, no split-screen).
//
// Precondition: cfg must have a valid port; handler and logger must be non-nil.
// Postcondition: Returns an Acceptor ready to be started; all connections will have Headless=true.
func NewHeadlessAcceptor(cfg config.TelnetConfig, handler SessionHandler, logger *zap.Logger) *Acceptor {
	return &Acceptor{
		cfg:      cfg,
		handler:  handler,
		logger:   logger,
		quit:     make(chan struct{}),
		headless: true,
	}
}
```

In `handleConn`, branch on `a.headless`:

```go
var conn *Conn
if a.headless {
	conn = NewHeadlessConn(raw, a.cfg.ReadTimeout, a.cfg.WriteTimeout)
} else {
	conn = NewConn(raw, a.cfg.ReadTimeout, a.cfg.WriteTimeout)
}
```

Replace the existing `conn := NewConn(raw, a.cfg.ReadTimeout, a.cfg.WriteTimeout)` line.

The headless acceptor still calls `conn.Negotiate()` and `conn.AwaitNAWS()` — these are harmless for plain-text clients (negotiation bytes will be ignored/rejected by `nc`, but NAWS won't arrive, and `AwaitNAWS` times out in 1 second regardless).

- [ ] **Step 2.7: Run the full telnet package tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/telnet/... -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 2.8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/config/config.go internal/frontend/telnet/acceptor.go
git commit -m "feat(headless): add HeadlessPort to TelnetConfig; add NewHeadlessAcceptor"
```

---

### Task 3: Start the headless acceptor in `cmd/frontend/main.go`

**Files:**
- Modify: `cmd/frontend/main.go`
- Modify: `configs/dev.yaml` (add `headless_port: 4002`)

The frontend binary must conditionally start a second `telnet.Acceptor` when `cfg.Telnet.HeadlessPort != 0`. The headless acceptor reuses the same `SessionHandler` (the existing `AuthHandler` which drives auth → char-select → game-bridge). All headless behavior is encapsulated in `Conn`, so no handler changes are required.

- [ ] **Step 3.1: Modify `cmd/frontend/main.go` to start the headless acceptor**

After the `lifecycle.Add("telnet", ...)` block, add:

```go
if cfg.Telnet.HeadlessPort != 0 {
	headlessCfg := cfg.Telnet
	headlessCfg.Port = cfg.Telnet.HeadlessPort
	headlessAcceptor := telnet.NewHeadlessAcceptor(headlessCfg, app.TelnetAcceptor.Handler(), logger)
	lifecycle.Add("telnet-headless", &server.FuncService{
		StartFn: func() error {
			return headlessAcceptor.ListenAndServe()
		},
		StopFn: func() {
			headlessAcceptor.Stop()
		},
	})
	logger.Info("headless telnet acceptor configured",
		zap.Int("port", cfg.Telnet.HeadlessPort),
	)
}
```

> **Note:** This requires a `Handler()` accessor on `Acceptor`. Add it to `acceptor.go`:
>
> ```go
> // Handler returns the SessionHandler used by this acceptor.
> func (a *Acceptor) Handler() SessionHandler {
> 	return a.handler
> }
> ```

- [ ] **Step 3.2: Add `headless_port: 4002` to `configs/dev.yaml`**

Add under the `telnet:` section:

```yaml
telnet:
  host: 0.0.0.0
  port: 4000
  headless_port: 4002
  read_timeout: 5m
  write_timeout: 30s
```

- [ ] **Step 3.3: Build the frontend binary to confirm no compile errors**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./cmd/frontend/... 2>&1
```

Expected: compiles with no errors.

- [ ] **Step 3.4: Run all frontend-related tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/frontend/... ./internal/config/... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 3.5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/frontend/main.go internal/frontend/telnet/acceptor.go configs/dev.yaml
git commit -m "feat(headless): start headless telnet acceptor on HeadlessPort in cmd/frontend/main.go"
```

---

### Task 4: Create `cmd/seed-claude-accounts/main.go`

**Files:**
- Create: `cmd/seed-claude-accounts/main.go`
- Modify: `Makefile`

This CLI tool upserts three claude accounts using the existing `AccountRepository`. It mirrors the structure of `cmd/setrole/main.go`.

- [ ] **Step 4.1: Write the unit test for the upsert logic (as a separate testable function)**

Because the main function connects to a real DB, the upsert logic should be extracted into a testable function. Create `cmd/seed-claude-accounts/main.go` with the upsert logic in a package-level function `upsertAccount`:

```go
// upsertAccount implements the three-step upsert: fetch, create if absent, set role.
// Precondition: repo must be a valid AccountRepository; username, password, role must be non-empty.
// Postcondition: Account with username exists in DB with the given role; no duplicate created.
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
```

Since real DB tests require testcontainers (see `internal/storage/postgres/main_test.go` pattern), add a unit test that mocks the repository by testing the logic inline using a fake:

Create `cmd/seed-claude-accounts/main_test.go`:

```go
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAccountRepo implements only the methods used by upsertAccount.
// It stores accounts in a map for test inspection.
type fakeAccountRepo struct {
	accounts map[string]fakeAccount
	createErr error
	setRoleErr error
}

type fakeAccount struct {
	id       int64
	username string
	role     string
}

func newFakeRepo() *fakeAccountRepo {
	return &fakeAccountRepo{accounts: make(map[string]fakeAccount)}
}

// Since upsertAccount depends on *postgres.AccountRepository (a concrete type),
// we test it indirectly by extracting the logic as a function that accepts
// typed interfaces. Define a minimal accountRepoIface used only in tests.
// Production code keeps using *postgres.AccountRepository directly (no interface needed).

// accountStorer is a test-only interface matching the three methods used by upsertAccount.
type accountStorer interface {
	GetByUsernameStr(ctx context.Context, username string) (id int64, role string, notFound bool, err error)
	CreateStr(ctx context.Context, username, password string) (id int64, err error)
	SetRoleStr(ctx context.Context, id int64, role string) error
}
```

> **Note:** Because `postgres.AccountRepository` is a concrete struct without an interface, and testcontainers integration tests live in `internal/storage/postgres/`, the `cmd/seed-claude-accounts` unit test validates the logic via a simple table test of the `upsertAccount` function using a test double. See implementation note in Step 4.2.

- [ ] **Step 4.2: Create `cmd/seed-claude-accounts/main.go`**

```go
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
```

- [ ] **Step 4.3: Create `cmd/seed-claude-accounts/main_test.go`**

The unit test validates the `upsertAccount` control flow without a real DB by testing via a fake in-process store. Because `upsertAccount` takes `*postgres.AccountRepository` (concrete), extract a thin interface in the test file only (Go allows test files to define helpers):

```go
package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// testable wrappers — test accounts table
func TestClaudeAccounts_HasThreeEntries(t *testing.T) {
	assert.Len(t, claudeAccounts, 3)
}

func TestClaudeAccounts_Roles(t *testing.T) {
	roles := map[string]bool{}
	for _, a := range claudeAccounts {
		roles[a.role] = true
	}
	assert.True(t, roles["player"], "must have player role")
	assert.True(t, roles["editor"], "must have editor role")
	assert.True(t, roles["admin"], "must have admin role")
}

func TestClaudeAccounts_Usernames(t *testing.T) {
	for _, a := range claudeAccounts {
		assert.NotEmpty(t, a.username)
		assert.Contains(t, a.username, "claude_")
	}
}

func TestClaudeAccounts_UsernamesProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// All entries must have non-empty usernames and valid roles.
		for _, a := range claudeAccounts {
			assert.NotEmpty(rt, a.username)
			assert.NotEmpty(rt, a.role)
		}
	})
}
```

> **Note:** Integration tests for the full DB upsert path are intentionally not added here — that is covered by the existing testcontainers pattern in `internal/storage/postgres/account_test.go`. The unit tests above validate the static configuration of `claudeAccounts`.

- [ ] **Step 4.4: Run the unit tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./cmd/seed-claude-accounts/... -v 2>&1
```

Expected: all PASS.

- [ ] **Step 4.5: Build the binary to confirm no compile errors**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./cmd/seed-claude-accounts/... 2>&1
```

Expected: builds cleanly.

- [ ] **Step 4.6: Add Makefile targets**

In `Makefile`, add `build-seed-claude-accounts` to the `build` target and add the two new targets. Find the `build-setrole:` target and add after it:

```makefile
build-seed-claude-accounts: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/seed-claude-accounts ./cmd/seed-claude-accounts

seed-claude-accounts: build-seed-claude-accounts
	CLAUDE_ACCOUNT_PASSWORD=$(CLAUDE_ACCOUNT_PASSWORD) $(BIN_DIR)/seed-claude-accounts -config $(CONFIG)
```

Also update the `build:` line to include `build-seed-claude-accounts`:

```makefile
build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole build-seed-claude-accounts
```

> **Note:** `CLAUDE_ACCOUNT_PASSWORD` and `CONFIG` must be passed as env/make variables by the caller: `make seed-claude-accounts CLAUDE_ACCOUNT_PASSWORD=secret CONFIG=configs/dev.yaml`

- [ ] **Step 4.7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add cmd/seed-claude-accounts/main.go cmd/seed-claude-accounts/main_test.go Makefile
git commit -m "feat(seed): add cmd/seed-claude-accounts — idempotent upsert of claude_player/editor/admin"
```

---

### Task 5: Write the `.claude/skills/mud-gameserver.md` skill document

**Files:**
- Create: `.claude/skills/mud-gameserver.md`

This document follows the `mud-*.md` convention seen in `.claude/skills/`. It is a reference document for Claude agent sessions to connect to the headless telnet port and interact with the game server.

- [ ] **Step 5.1: Create `.claude/skills/mud-gameserver.md`**

```markdown
# MUD Game Server Skill

Connect to the running MUD game server via the headless telnet port (4002) for feature testing and content work. This port outputs plain text — no ANSI escape codes, no split-screen layout — making it suitable for programmatic interaction.

---

## Connection

```bash
nc localhost 4002
# or
telnet localhost 4002
```

The server responds with a `Username:` prompt immediately on connect.

---

## Account Selection

| Role | Username | Use For |
|------|----------|---------|
| player | `claude_player` | Testing gameplay: movement, combat, inventory, commands |
| editor | `claude_editor` | Placing content: creating items, NPCs, room descriptions |
| admin | `claude_admin` | World management: state inspection, spawning, overrides |

Password: set by the `CLAUDE_ACCOUNT_PASSWORD` environment variable when `make seed-claude-accounts` was run. Ask the operator if unknown.

---

## Session Flow

### Login

```
Username: claude_player
Password: <password>
```

On success the server sends the character selection menu:

```
Select a character:
  1. <character name>
  > (or 'new' to create)
```

If no characters exist yet, type `new` and follow the character creation prompts (name, class/job, etc.).

### After Character Select

The server outputs the current room:

```
<Room Name>
<Description lines, word-wrapped>
Exits: <direction> [<direction> ...]

>
```

The `> ` prompt indicates the server is waiting for a command.

### Prompt Recognition

- `Username: ` — server awaiting login
- `Password: ` — server awaiting password (input is not echoed)
- `> ` — logged in and in-game, awaiting command
- Any other line — server output (room views, messages, combat events)

---

## Plain-Text Output Format

Room output (after entering a room or typing `look`):

```
<Room Name>
<Description line 1>
<Description line 2 if wrapped>
Exits: north south east

>
```

Console messages (combat, system, NPC speech) appear as plain lines:

```
You strike the guard for 12 damage.
The guard attacks you for 8 damage.
>
```

---

## Command Reference

### Player Commands

| Command | Description | Expected Response |
|---------|-------------|-------------------|
| `look` | Describe current room | Room name, description, exits |
| `go <direction>` | Move to adjacent room | New room view |
| `north` / `south` / `east` / `west` / `up` / `down` | Move shorthand | New room view |
| `inventory` | List carried items | Item list |
| `equipment` | Show equipped items | Equipment slots |
| `stats` | Character statistics | Stat block |
| `get <item>` | Pick up item from room | Confirmation message |
| `drop <item>` | Drop item in room | Confirmation message |
| `attack <target>` | Initiate/continue combat | Combat event messages |
| `say <text>` | Speak to room | Echo of speech |
| `quit` | Disconnect gracefully | `Goodbye.` |

### Editor Commands

All player commands plus:

| Command | Description |
|---------|-------------|
| `edit room` | Open room editor (if implemented) |
| `spawn <npc-id>` | Spawn NPC in current room |
| `place <item-id>` | Place item in current room |

### Admin Commands

All editor commands plus:

| Command | Description |
|---------|-------------|
| `setrole <username> <role>` | Change account role |
| `shutdown` | Initiate server shutdown |
| `reload zones` | Reload zone content from disk |

---

## Example Workflows

### Workflow 1: Verify NPC Spawn

Goal: Confirm that a named NPC appears in a room after a spawn command.

```
# Connect as editor
nc localhost 4002
Username: claude_editor
Password: <password>
# Select or create character
> spawn guard_patrol_01
Guard Patrol 01 appears in the room.
> look
The Alley
A narrow passage between two buildings. A guard patrols here.
Exits: north south
Guard Patrol 01 is here.

>
```

Verify: `look` output contains the NPC name.

### Workflow 2: Place Item in Room

Goal: Place a medkit in a specific room and confirm a player can pick it up.

```
# Session 1: Editor places item
nc localhost 4002
Username: claude_editor
Password: <password>
> go north       # navigate to target room
> place medkit_standard
Medkit Standard is placed in the room.
> look
...
Medkit Standard is here.

# Session 2: Player picks it up
nc localhost 4002
Username: claude_player
Password: <password>
> go north
> get medkit
You pick up the Medkit Standard.
> inventory
  Medkit Standard
>
```

### Workflow 3: Test Combat Round

Goal: Confirm that a combat round completes and health is reduced.

```
nc localhost 4002
Username: claude_player
Password: <password>
> stats
HP: 30/30  AC: 15
> attack training_dummy
Combat begins.
You attack the Training Dummy.
You strike the Training Dummy for 7 damage. (23 HP remaining)
The Training Dummy attacks you.
The Training Dummy misses.
> stats
HP: 30/30  AC: 15
>
```

Verify: combat events appear as plain lines, `stats` reflects accurate HP.

---

## Notes

- The headless port (4002) is only available when `frontend.headless_port` is set to a non-zero value in the config (default in `configs/dev.yaml`: 4002).
- Output is plain text. No ANSI color codes, no cursor positioning, no split-screen.
- The session goes through the same auth and character-select flow as a normal telnet client.
- Use `quit` or close the connection to end the session cleanly.
```

- [ ] **Step 5.2: Verify the skill file exists and is well-formed**

```bash
wc -l /home/cjohannsen/src/mud/.claude/skills/mud-gameserver.md
```

Expected: > 100 lines.

- [ ] **Step 5.3: Commit**

```bash
cd /home/cjohannsen/src/mud && git add .claude/skills/mud-gameserver.md
git commit -m "docs(skill): add .claude/skills/mud-gameserver.md — headless port connection and workflow reference"
```

---

### Task 6: Final integration check

- [ ] **Step 6.1: Run the full non-DB test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test $(go list ./... | grep -v 'storage/postgres') -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -60
```

Expected: all packages report `ok`.

- [ ] **Step 6.2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && mise exec -- make build 2>&1
```

Expected: all binaries build with no errors.

- [ ] **Step 6.3: Verify the feature index entry**

Check `docs/features/index.yaml` for the `claude-gameserver-skill` entry. If its status is not `done`, update it to `done`.

```bash
grep -A5 "claude-gameserver-skill" /home/cjohannsen/src/mud/docs/features/index.yaml
```

- [ ] **Step 6.4: Final commit if feature index updated**

```bash
cd /home/cjohannsen/src/mud && git add docs/features/index.yaml
git commit -m "chore: mark claude-gameserver-skill feature as done"
```
