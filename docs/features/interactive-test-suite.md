# Interactive Automated Test Suite

## Overview

An end-to-end test suite that connects to the MUD through the headless telnet port and exercises complete game scenarios. A `TestMain` harness spins up an ephemeral PostgreSQL container, starts the gameserver and telnet frontend as subprocesses, seeds the `claude_*` accounts, and then runs scenario-based tests through a thin `HeadlessClient`. Tests cover authentication, navigation, combat, inventory, NPCs, crafting, downtime, and UI features from the player's perspective.

---

## Requirements

- REQ-ITS-1: The suite MUST launch a PostgreSQL container via `testcontainers-go`, apply all migrations, and start the `gameserver` and `telnet-frontend` binaries as subprocesses inside `TestMain` — no external running server MUST be required.
- REQ-ITS-2: `TestMain` MUST poll the headless telnet port until it accepts connections, timing out with a descriptive error after 30 seconds.
- REQ-ITS-3: `TestMain` MUST seed the `claude_player`, `claude_editor`, and `claude_admin` accounts by invoking the existing `seed-claude-accounts` binary against the ephemeral DB.
- REQ-ITS-4: The suite MUST provide a `HeadlessClient` type with `Send(cmd string) error` and `Expect(pattern string, timeout time.Duration) error` primitives; `Expect` MUST match against received lines by substring; `ExpectRegex` MUST match by compiled regular expression.
- REQ-ITS-5: `Expect` and `ExpectRegex` MUST default to a 5-second timeout when called with `timeout == 0`.
- REQ-ITS-6: A `NewClientForTest(t *testing.T) *HeadlessClient` helper MUST dial the headless port, register `t.Cleanup(c.Close)`, and return a ready-to-use client; callers MUST NOT manage connection lifecycle manually.
- REQ-ITS-7: Each test scenario MUST operate on its own test character; characters MUST be created via editor commands through the `claude_editor` session during scenario setup and deleted during cleanup.
- REQ-ITS-8: Cleanup MUST run regardless of pass/fail; `t.Cleanup` or `defer` MUST be used, never placed only in the happy path.
- REQ-ITS-9: The suite MUST be runnable via `make test-e2e` and MUST NOT be included in `make test`, `make test-fast`, or `make test-postgres`.
- REQ-ITS-10: Each scenario MUST record its elapsed time; `TestMain` MUST print a summary table (scenario name, elapsed ms, pass/fail) after `m.Run()` returns.
- REQ-ITS-11: The test configuration MUST be rendered from a template (`testdata/e2e/config.yaml.tmpl`) with the ephemeral DB URL and port assignments injected at runtime.
- REQ-ITS-12: The suite MUST cover the following scenario categories:
  - REQ-ITS-12a: Authentication — login with valid and invalid credentials; account lockout after repeated failures.
  - REQ-ITS-12b: Character lifecycle — character creation via the `create` command; character selection; re-login restores character state.
  - REQ-ITS-12c: Room navigation — `look`, `move <direction>`, locked exits, invalid direction messages.
  - REQ-ITS-12d: Combat — initiate combat, submit a combat action, receive round-resolution output, loot a defeated enemy.
  - REQ-ITS-12e: Inventory — pick up item, drop item, equip/unequip, inspect.
  - REQ-ITS-12f: NPC interaction — `buy`, `sell`, `browse` at a merchant NPC.
  - REQ-ITS-12g: Crafting and downtime — queue a downtime activity; verify completion message.
  - REQ-ITS-12h: Hotbar — assign a slot via `hotbar <slot> <cmd>`, activate via single-digit input, clear a slot.
  - REQ-ITS-12i: Editor commands — `spawn_npc`, `grant_item`, `grant_money` executed as `claude_editor`; verify effects visible to `claude_player` in the same room.

---

## Architecture

### Package Layout

```
internal/e2e/
  suite_test.go          # TestMain: container, subprocesses, seed, m.Run(), summary
  client.go              # HeadlessClient: Dial, Send, Expect, ExpectRegex, Close
  helpers_test.go        # NewClientForTest, loginAs, createCharacter, deleteCharacter
  scenarios/
    auth_test.go         # REQ-ITS-12a
    character_test.go    # REQ-ITS-12b
    navigation_test.go   # REQ-ITS-12c
    combat_test.go       # REQ-ITS-12d
    inventory_test.go    # REQ-ITS-12e
    npc_test.go          # REQ-ITS-12f
    crafting_test.go     # REQ-ITS-12g
    hotbar_test.go       # REQ-ITS-12h
    editor_test.go       # REQ-ITS-12i
testdata/e2e/
  config.yaml.tmpl       # Config template with {{.DBURL}}, {{.GameserverPort}}, {{.HeadlessPort}}
```

### HeadlessClient

```go
type HeadlessClient struct {
    conn    net.Conn
    reader  *bufio.Reader
    timeout time.Duration // default per-call timeout; overridden by Expect/ExpectRegex args
}

func Dial(addr string) (*HeadlessClient, error)
func (c *HeadlessClient) Send(cmd string) error
func (c *HeadlessClient) Expect(pattern string, timeout time.Duration) error
func (c *HeadlessClient) ExpectRegex(pattern string, timeout time.Duration) error
func (c *HeadlessClient) ReadLine(timeout time.Duration) (string, error)
func (c *HeadlessClient) Close() error
```

`Send` writes `cmd + "\r\n"` to the TCP connection. `Expect` reads lines in a loop until a line contains `pattern` or `timeout` elapses, returning a descriptive error on timeout. `ExpectRegex` compiles the pattern once and matches each line.

### TestMain Lifecycle

1. Start PostgreSQL 16-Alpine container (shared pool pattern from `internal/storage/postgres`).
2. Render `testdata/e2e/config.yaml.tmpl` with ephemeral DB URL and assigned ports into a temp file.
3. Build `cmd/gameserver` and `cmd/telnet-frontend` via `go build` into a temp directory.
4. Start `gameserver` subprocess with `-config <tempfile>`; capture stderr for diagnostics.
5. Start `telnet-frontend` subprocess with `-config <tempfile>`; capture stderr.
6. Poll `frontend.headless_port` via TCP dial until accepted (max 30 s, 200 ms interval).
7. Run `seed-claude-accounts -config <tempfile>` (or re-implement inline via repository calls).
8. Call `m.Run()` and record result.
9. Print per-scenario timing summary (populated by each test via shared `timingRecorder`).
10. Kill subprocesses (SIGTERM, 5 s grace, SIGKILL); stop container.
11. Exit with `m.Run()` result.

### Scenario Authoring Pattern

```go
func TestNavigation_MoveNorth(t *testing.T) {
    editor := NewClientForTest(t)
    loginAs(t, editor, "claude_editor")
    charName := createCharacter(t, editor, "TestChar_Nav")

    player := NewClientForTest(t)
    loginAs(t, player, "claude_player")
    selectCharacter(t, player, charName)

    player.Send("look")
    require.NoError(t, player.Expect("Exits:", 5*time.Second))

    player.Send("move north")
    require.NoError(t, player.Expect("You move", 5*time.Second))

    t.Cleanup(func() { deleteCharacter(t, editor, charName) })
}
```

### Configuration Template

`testdata/e2e/config.yaml.tmpl` mirrors `configs/dev.yaml` with substitution tokens:

```yaml
database:
  url: "{{.DBURL}}"
gameserver:
  port: {{.GameserverPort}}
frontend:
  port: {{.FrontendPort}}
  headless_port: {{.HeadlessPort}}
```

Ports are assigned by binding to `:0` and recording the OS-assigned port before releasing.

---

## File Map

| File | Change |
|------|--------|
| `internal/e2e/suite_test.go` | New: TestMain harness (container, subprocesses, seed, summary) |
| `internal/e2e/client.go` | New: HeadlessClient (Dial, Send, Expect, ExpectRegex, Close) |
| `internal/e2e/helpers_test.go` | New: NewClientForTest, loginAs, selectCharacter, createCharacter, deleteCharacter |
| `internal/e2e/scenarios/auth_test.go` | New: REQ-ITS-12a scenarios |
| `internal/e2e/scenarios/character_test.go` | New: REQ-ITS-12b scenarios |
| `internal/e2e/scenarios/navigation_test.go` | New: REQ-ITS-12c scenarios |
| `internal/e2e/scenarios/combat_test.go` | New: REQ-ITS-12d scenarios |
| `internal/e2e/scenarios/inventory_test.go` | New: REQ-ITS-12e scenarios |
| `internal/e2e/scenarios/npc_test.go` | New: REQ-ITS-12f scenarios |
| `internal/e2e/scenarios/crafting_test.go` | New: REQ-ITS-12g scenarios |
| `internal/e2e/scenarios/hotbar_test.go` | New: REQ-ITS-12h scenarios |
| `internal/e2e/scenarios/editor_test.go` | New: REQ-ITS-12i scenarios |
| `testdata/e2e/config.yaml.tmpl` | New: config template for ephemeral test stack |
| `Makefile` | Add `test-e2e` target: `go test -v -count=1 -timeout 300s ./internal/e2e/...` |

---

## Non-Goals

- No load testing or concurrent-user stress scenarios.
- No fuzzing of the telnet protocol.
- No mocking of the gameserver or database — this suite is strictly full-stack.
- No CI pipeline integration in this phase — `test-e2e` is developer-run only.
- No coverage instrumentation for the tested binaries.
