# Telnet Interface Deprecation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retire the player-facing telnet surface. Retain the telnet port for plain-text system debugging (status, log, ping, who, raw RPC inspection) and for the existing `HeadlessClient` test automation. Bind the headless port loopback-only; reject regular telnet connections with a redirect message; remove the telnet port from external Kubernetes exposure; isolate the `internal/frontend/handlers/` game-bridge code paths so the canonical `cmd/frontend/` build no longer carries them. Add a `telnet.allow_game_commands` config flag to enable a graceful sunset rather than a hard remove.

**Spec:** [docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md)

**Architecture:** Five surgical changes. (1) Acceptor split — the existing player-facing `TelnetAcceptor` becomes a *rejector* that prints a redirect message and closes; the `HeadlessAcceptor` (already exists; bound separately on `headless_port`) keeps the test-automation path. The two-acceptor pattern is already partially in place (`cmd/frontend/main.go:97`); this plan finalises it. (2) Debug command set — a small allowlist of operator-friendly commands (`status`, `log`, `ping`, `who`, `rpc`) is exposed only on the headless / debug acceptor; game-bridge handlers remain importable but are no longer wired by default in production builds. (3) Config — `telnet.host` defaults to `127.0.0.1` for both ports; `telnet.allow_game_commands bool` (default `false`) re-enables the legacy player flow for graceful sunset operations. Production configs (`prod.yaml`, `docker.yaml`) drop external host bindings. (4) Helm — the `frontend` `Service` is downgraded from `LoadBalancer:30400` to `ClusterIP` (loopback within the cluster) so the telnet port is no longer reachable from outside the namespace. (5) Documentation — `docs/architecture/` diagrams updated; `docs/features/claude-gameserver-skill.md` documents the headless port as the sole telnet access; in-repo telnet command docs redirect to the web client. The interactive test suite (`internal/e2e/`) continues to use `HeadlessClient.Dial` against the headless port unchanged.

**Tech Stack:** Go (`internal/frontend/telnet/`, `internal/frontend/handlers/`, `cmd/frontend/`, `cmd/devserver/`), Helm (`deployments/k8s/mud/templates/frontend/`), config YAML (`configs/`), docs.

**Prerequisite:** None. The web client is the supported player surface; #268 (telnet reaction UI) was withdrawn in this re-raise. No active telnet expansion remains.

**User confirmation checkpoints:**

- **Task 4 (REQ-TD-2c)**: confirm the Helm `Service` downgrade strategy — `ClusterIP` (default) vs delete the service entirely vs keep it but on a non-routable port. Plan default = `ClusterIP` per spec text.
- **Task 5 (debug command allowlist)**: confirm the operator-facing command set (`status`, `log`, `ping`, `who`, `rpc`) before authoring the debug-mode handler.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/frontend/telnet/acceptor.go` (`TelnetAcceptor` rejection mode; gate by `allow_game_commands`) |
| Modify | `internal/frontend/telnet/acceptor_test.go` |
| Create | `internal/frontend/telnet/rejector.go` (rejection-message handler) |
| Create | `internal/frontend/telnet/rejector_test.go` |
| Modify | `internal/frontend/telnet/conn.go` (headless connection path stays, player-flow gating) |
| Create | `internal/frontend/handlers/debug_handler.go` (operator command set: `status` / `log` / `ping` / `who` / `rpc`) |
| Create | `internal/frontend/handlers/debug_handler_test.go` |
| Modify | `internal/frontend/handlers/auth.go` (gate behind `allow_game_commands`; otherwise reject) |
| Modify | `cmd/frontend/main.go` (acceptor wiring; default rejection mode; debug acceptor hookup) |
| Modify | `cmd/devserver/main.go` (parallel wiring for the local dev path) |
| Modify | `internal/config/config.go` (or wherever `Telnet` config struct lives — `AllowGameCommands bool`, default `false`; `Host` default `127.0.0.1`) |
| Modify | `internal/config/config_test.go` |
| Modify | `configs/prod.yaml` (host → 127.0.0.1; document graceful-sunset flag) |
| Modify | `configs/docker.yaml` (host → 127.0.0.1) |
| Modify | `configs/dev.yaml` (host → 127.0.0.1; keep headless_port) |
| Modify | `deployments/k8s/mud/templates/frontend/service.yaml` (LoadBalancer → ClusterIP; drop nodePort) |
| Modify | `deployments/k8s/mud/values.yaml`, `values-prod.yaml` (any `frontend.service.type` overrides) |
| Modify | `internal/e2e/client.go` (verify HeadlessClient unchanged — REQ-TD-5b confirmation) |
| Modify | `docs/features/claude-gameserver-skill.md` (document headless port as sole access) |
| Modify | `docs/architecture/*.md` (diagrams referencing player-facing telnet) |
| Modify | `docs/features/telnet-deprecation.md` (mark complete) |
| Modify | `internal/frontend/doc.go` (or `internal/frontend/handlers/doc.go`) — package summary "Telnet frontend — retained for headless test/debug access only." |

---

### Task 1: Config — `Telnet.AllowGameCommands` + loopback-default host

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `configs/prod.yaml`, `configs/docker.yaml`, `configs/dev.yaml`

- [ ] **Step 1: Failing tests** (REQ-TD-1c + new sunset flag):

```go
func TestTelnetConfig_DefaultsLoopback(t *testing.T) {
    cfg := config.Default().Telnet
    require.Equal(t, "127.0.0.1", cfg.Host)
}

func TestTelnetConfig_AllowGameCommandsDefaultsFalse(t *testing.T) {
    cfg := config.Default().Telnet
    require.False(t, cfg.AllowGameCommands)
}

func TestTelnetConfig_HeadlessPortDistinctFromMain(t *testing.T) {
    cfg := mustLoadConfig(t, "configs/dev.yaml")
    require.NotEqual(t, cfg.Telnet.Port, cfg.Telnet.HeadlessPort)
    require.NotZero(t, cfg.Telnet.HeadlessPort)
}

func TestTelnetConfig_RejectsExternalHostInProd(t *testing.T) {
    // The prod loader MUST refuse a non-loopback host unless allow_game_commands is true.
    _, err := config.LoadProd(yamlWith("0.0.0.0", false))
    require.Error(t, err)
    require.Contains(t, err.Error(), "telnet.host must be 127.0.0.1")
}

func TestTelnetConfig_AllowsExternalHostWhenSunsetFlagSet(t *testing.T) {
    // Operator can opt back in for a graceful sunset.
    _, err := config.LoadProd(yamlWith("0.0.0.0", true))
    require.NoError(t, err)
}
```

- [ ] **Step 2: Implement**:

```go
type Telnet struct {
    Host              string        `yaml:"host"`
    Port              int           `yaml:"port"`
    HeadlessPort      int           `yaml:"headless_port"`
    ReadTimeout       time.Duration `yaml:"read_timeout"`
    WriteTimeout      time.Duration `yaml:"write_timeout"`
    AllowGameCommands bool          `yaml:"allow_game_commands"` // graceful sunset; default false
}

func Default() Config {
    return Config{
        // ... existing ...
        Telnet: Telnet{
            Host: "127.0.0.1",
            Port: 4000,
            HeadlessPort: 4002,
            ReadTimeout: 5 * time.Minute,
            WriteTimeout: 30 * time.Second,
            AllowGameCommands: false,
        },
    }
}
```

The prod-load path validates `Host == "127.0.0.1" || AllowGameCommands` and rejects otherwise.

- [ ] **Step 3: Update YAML configs**:

```yaml
# configs/prod.yaml
telnet:
  host: 127.0.0.1
  port: 4000
  headless_port: 4002
  read_timeout: 5m
  write_timeout: 30s
  allow_game_commands: false  # graceful sunset; flip to true to re-enable legacy player flow
```

Same for `dev.yaml` and `docker.yaml`. The `headless_port` stays present in all three.

---

### Task 2: Rejector — `TelnetAcceptor` rejects player flow by default

**Files:**
- Create: `internal/frontend/telnet/rejector.go`
- Create: `internal/frontend/telnet/rejector_test.go`
- Modify: `internal/frontend/telnet/acceptor.go`
- Modify: `internal/frontend/telnet/acceptor_test.go`

- [ ] **Step 1: Failing tests** (REQ-TD-2a, REQ-TD-2b):

```go
func TestRejector_PrintsRedirectAndCloses(t *testing.T) {
    srv := startRejectorOnRandomPort(t)
    conn, _ := net.Dial("tcp", srv.Addr())
    defer conn.Close()
    buf, _ := io.ReadAll(io.LimitReader(conn, 1024))
    require.Contains(t, string(buf), "The web client is the supported player surface")
    require.Contains(t, string(buf), "https://")
    // Connection MUST be closed after the message.
    _, err := conn.Read(make([]byte, 1))
    require.Error(t, err) // EOF
}

func TestAcceptor_DefaultsToRejectorWhenAllowGameCommandsFalse(t *testing.T) {
    cfg := config.Telnet{Host: "127.0.0.1", Port: 0, AllowGameCommands: false}
    a := telnet.NewAcceptor(cfg, gameHandler, logger)
    require.IsType(t, &telnet.Rejector{}, a.Handler(), "default handler must be the rejector when game commands are disabled")
}

func TestAcceptor_AllowsGameCommandsWhenSunsetFlagSet(t *testing.T) {
    cfg := config.Telnet{Host: "127.0.0.1", Port: 0, AllowGameCommands: true}
    a := telnet.NewAcceptor(cfg, gameHandler, logger)
    require.NotEqual(t, &telnet.Rejector{}, a.Handler(), "with sunset flag, original game handler is wired")
}
```

- [ ] **Step 2: Implement** the rejector:

```go
package telnet

import (
    "fmt"
    "io"
    "net"
)

type Rejector struct {
    webClientURL string
    logger       *zap.Logger
}

func NewRejector(webClientURL string, logger *zap.Logger) *Rejector {
    return &Rejector{webClientURL: webClientURL, logger: logger}
}

const rejectorMessage = `
                Gunchete — Telnet Player Surface Retired

The web client is the supported player surface for new gameplay.
Telnet is retained only for plain-text system debugging.

  Web client: %s

Press any key to disconnect.
`

func (r *Rejector) HandleConnection(conn net.Conn) {
    defer conn.Close()
    fmt.Fprintf(conn, rejectorMessage, r.webClientURL)
    _, _ = io.Copy(io.Discard, io.LimitReader(conn, 1)) // wait one byte then close
}
```

The rejector implements the same `Handler` interface the existing acceptor consumes; `NewAcceptor` chooses which handler to wire based on `AllowGameCommands`.

- [ ] **Step 3:** Existing `acceptor_test.go` cases that exercise the full game flow get a guard requiring `AllowGameCommands: true`; default-config tests verify the rejector path.

---

### Task 3: HeadlessAcceptor — loopback bind + seed-authorized only

**Files:**
- Modify: `internal/frontend/telnet/acceptor.go` (HeadlessAcceptor portion)
- Modify: `cmd/frontend/main.go` (wire headless acceptor unconditionally)
- Modify: `cmd/devserver/main.go` (parallel wiring)

- [ ] **Step 1: Failing tests** (REQ-TD-1a, REQ-TD-1c, REQ-TD-3a, REQ-TD-3b):

```go
func TestHeadlessAcceptor_BindsLoopbackOnly(t *testing.T) {
    cfg := config.Telnet{HeadlessPort: 0, Host: "127.0.0.1"}
    a := telnet.NewHeadlessAcceptor(cfg, gameHandler, logger)
    go a.ListenAndServe()
    defer a.Stop()
    addr := a.Addr().(*net.TCPAddr)
    require.True(t, addr.IP.IsLoopback(), "headless listener must bind loopback only")
}

func TestHeadlessAcceptor_RejectsUnseededConnection(t *testing.T) {
    a := telnet.NewHeadlessAcceptor(cfg, gameHandler, logger)
    go a.ListenAndServe()
    defer a.Stop()
    conn, _ := net.Dial("tcp", a.Addr().String())
    defer conn.Close()
    fmt.Fprintln(conn, `{"login":"unseeded_user"}`)
    buf, _ := io.ReadAll(io.LimitReader(conn, 1024))
    require.Contains(t, string(buf), "not seed-authorized")
    _, err := conn.Read(make([]byte, 1))
    require.Error(t, err) // closed
}

func TestHeadlessAcceptor_AcceptsSeededConnection(t *testing.T) {
    seeded := seed.MustSeed(t, "claude-test-account")
    a := telnet.NewHeadlessAcceptor(cfg, gameHandler, logger)
    go a.ListenAndServe()
    defer a.Stop()
    conn, _ := net.Dial("tcp", a.Addr().String())
    defer conn.Close()
    fmt.Fprintf(conn, `{"login":"%s","seed_token":"%s"}`+"\n", seeded.Login, seeded.Token)
    buf, _ := io.ReadAll(io.LimitReader(conn, 1024))
    require.NotContains(t, string(buf), "not seed-authorized")
}
```

- [ ] **Step 2: Implement**:

```go
func NewHeadlessAcceptor(cfg config.Telnet, gameHandler Handler, logger *zap.Logger) *HeadlessAcceptor {
    return &HeadlessAcceptor{
        addr:         net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.HeadlessPort)), // force loopback
        handler:      seedAuthHandler{inner: gameHandler, seedStore: seed.DefaultStore()},
        readTimeout:  cfg.ReadTimeout,
        writeTimeout: cfg.WriteTimeout,
        logger:       logger,
    }
}

type seedAuthHandler struct {
    inner     Handler
    seedStore seed.Store
}

func (h seedAuthHandler) HandleConnection(conn net.Conn) {
    // Read first line as seed-token JSON; verify against seed store.
    line, err := readLine(conn)
    if err != nil {
        conn.Close()
        return
    }
    var auth struct{ Login, SeedToken string }
    if err := json.Unmarshal([]byte(line), &auth); err != nil || !h.seedStore.Verify(auth.Login, auth.SeedToken) {
        fmt.Fprintln(conn, "not seed-authorized")
        conn.Close()
        return
    }
    // Inject the resolved login into the inner handler's session bootstrap.
    h.inner.HandleSeededConnection(conn, auth.Login)
}
```

`seed.Store` is the existing seed-claude-accounts seeding mechanism; if no public `Verify` exists, add one alongside the seed creation path.

- [ ] **Step 3: Wire** in `cmd/frontend/main.go`:

```go
// Always start the headless acceptor (REQ-TD-1a).
headlessAcceptor := telnet.NewHeadlessAcceptor(cfg.Telnet, app.GameHandler, logger)
lifecycle.Add("telnet-headless", &server.FuncService{
    StartFn: func() error { return headlessAcceptor.ListenAndServe() },
    StopFn:  func()       { headlessAcceptor.Stop() },
})

// Player port: rejector by default; full game handler only when sunset flag is set.
var playerHandler telnet.Handler
if cfg.Telnet.AllowGameCommands {
    playerHandler = app.GameHandler
} else {
    playerHandler = telnet.NewRejector(cfg.WebClientURL, logger)
}
playerAcceptor := telnet.NewAcceptor(cfg.Telnet, playerHandler, logger)
lifecycle.Add("telnet", &server.FuncService{...})
```

- [ ] **Step 4:** `cmd/devserver/main.go` mirrors the same pattern.

---

### Task 4: Helm/K8s — frontend service no longer external

**Files:**
- Modify: `deployments/k8s/mud/templates/frontend/service.yaml`
- Modify: `deployments/k8s/mud/values.yaml`
- Modify: `deployments/k8s/mud/values-prod.yaml`

- [ ] **Step 1: Checkpoint.** Confirm with user the downgrade target:
  - Option A (plan default per REQ-TD-2c): `ClusterIP`. The service is reachable inside the cluster only. Useful if any other in-cluster process needs it (none today).
  - Option B: delete the `Service` entirely. Telnet only reachable via `kubectl exec`/`port-forward`.

- [ ] **Step 2: Rewrite the service** (Option A default):

```yaml
# deployments/k8s/mud/templates/frontend/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: frontend
  namespace: mud
  annotations:
    deprecated.mud/telnet: "telnet is loopback-only; this Service is for in-cluster debug only"
spec:
  type: ClusterIP
  selector:
    app: frontend
  ports:
    - name: telnet-debug
      port: 4002
      targetPort: 4002
      protocol: TCP
```

Note: `port: 4002` (headless port) replaces `port: 4000`. The player port 4000 is no longer reachable in any form from outside the pod.

- [ ] **Step 3: Failing test (Helm template smoke)**:

```bash
helm template deployments/k8s/mud > /tmp/rendered.yaml
! grep -q "type: LoadBalancer" /tmp/rendered.yaml
! grep -q "nodePort: 30400" /tmp/rendered.yaml
! grep -q "port: 4000" /tmp/rendered.yaml
grep -q "port: 4002" /tmp/rendered.yaml
```

A small CI check (or a `make k8s-lint` target if one exists) runs `helm template` and greps for the forbidden lines.

- [ ] **Step 4: Verify** the deployment readiness check uses the gRPC port (or the webclient port), not telnet, so the readiness probe doesn't break.

---

### Task 5: Debug command allowlist for the headless surface

**Files:**
- Create: `internal/frontend/handlers/debug_handler.go`
- Create: `internal/frontend/handlers/debug_handler_test.go`

- [ ] **Step 1: Checkpoint.** Confirm with user the operator command set per the spec re-raise refinement:
  - `status` — server uptime / version / build / connection count.
  - `log <tail|grep <pattern>>` — recent log entries.
  - `ping` — round-trip echo.
  - `who` — connected accounts (loopback-only; safe to expose).
  - `rpc <method> <json-args>` — direct gRPC call inspection (auth-gated).

  Plan default: ship all five. Reversible.

- [ ] **Step 2: Failing tests**:

```go
func TestDebugHandler_StatusReportsUptimeAndBuild(t *testing.T) {
    h := newDebugHandler(t)
    out := h.Run("status")
    require.Regexp(t, `uptime: \d+`, out)
    require.Regexp(t, `build: \w+`, out)
    require.Regexp(t, `connections: \d+`, out)
}

func TestDebugHandler_PingEchoes(t *testing.T) {
    h := newDebugHandler(t)
    require.Equal(t, "pong\n", h.Run("ping"))
}

func TestDebugHandler_WhoListsConnectedAccounts(t *testing.T) {
    h := newDebugHandlerWithConnections(t, "alice", "bob")
    out := h.Run("who")
    require.Contains(t, out, "alice")
    require.Contains(t, out, "bob")
}

func TestDebugHandler_LogTail(t *testing.T) {
    h := newDebugHandler(t, withLogLine("level=info msg=hello"))
    require.Contains(t, h.Run("log tail"), "hello")
}

func TestDebugHandler_LogGrep(t *testing.T) {
    h := newDebugHandler(t, withLogLines("hello", "world", "boom"))
    require.Contains(t, h.Run("log grep boom"), "boom")
    require.NotContains(t, h.Run("log grep boom"), "hello")
}

func TestDebugHandler_RPCInvokesGRPC(t *testing.T) {
    h := newDebugHandlerWithGRPC(t)
    out := h.Run(`rpc GetServerStatus {}`)
    require.Contains(t, out, "version")
}

func TestDebugHandler_RejectsUnknownCommand(t *testing.T) {
    h := newDebugHandler(t)
    require.Contains(t, h.Run("attack"), "unknown debug command")
}
```

- [ ] **Step 3: Implement** the handler. Each command is a small pure function over the gameserver's introspection APIs:

```go
type DebugHandler struct {
    server  *gameserver.Server
    log     LogTail
    started time.Time
    grpc    grpcClient
}

func (h *DebugHandler) HandleSeededConnection(conn net.Conn, login string) {
    h.run(conn, login)
}

func (h *DebugHandler) run(conn net.Conn, login string) {
    fmt.Fprintln(conn, "Gunchete debug shell. Type `help` for commands.")
    sc := bufio.NewScanner(conn)
    for sc.Scan() {
        line := strings.TrimSpace(sc.Text())
        if line == "" { continue }
        out, err := h.dispatch(login, line)
        if err != nil { fmt.Fprintln(conn, "error:", err) ; continue }
        fmt.Fprint(conn, out)
    }
}

func (h *DebugHandler) dispatch(login, line string) (string, error) {
    fields := strings.Fields(line)
    switch fields[0] {
    case "status":  return h.handleStatus()
    case "ping":    return "pong\n", nil
    case "who":     return h.handleWho()
    case "log":     return h.handleLog(fields[1:])
    case "rpc":     return h.handleRPC(fields[1:])
    case "help":    return h.handleHelp()
    case "quit":    return "", io.EOF
    default:
        return "", fmt.Errorf("unknown debug command %q (try `help`)", fields[0])
    }
}
```

- [ ] **Step 4:** When the headless acceptor is wired (Task 3), the seed-authorized handler routes to `DebugHandler` by default. The legacy game-bridge handler stays importable but is wired only when `AllowGameCommands == true` for the *player* port.

---

### Task 6: Player-flow gating in `auth.go` / handlers

**Files:**
- Modify: `internal/frontend/handlers/auth.go`
- Modify: `internal/frontend/handlers/auth_test.go`

- [ ] **Step 1: Failing tests** (REQ-TD-2a, REQ-TD-2b):

```go
func TestAuth_RejectsLoginWhenAllowGameCommandsFalse(t *testing.T) {
    h := newAuthHandler(t, withConfig(allowGameCommands: false))
    out := connectAndAttemptLogin(t, h, "alice", "password")
    require.Contains(t, out, "telnet player surface retired")
    require.Contains(t, out, "web client")
}

func TestAuth_AllowsLoginWhenAllowGameCommandsTrue(t *testing.T) {
    h := newAuthHandler(t, withConfig(allowGameCommands: true))
    out := connectAndAttemptLogin(t, h, "alice", "password")
    require.NotContains(t, out, "retired")
}
```

- [ ] **Step 2:** Auth handler sees the config flag injected at construction time; when `false`, it returns a friendly rejection narrative instead of running the existing login state machine. (Belt-and-suspenders: even if the rejector at the acceptor layer is somehow bypassed, this layer also refuses.)

---

### Task 7: HeadlessClient back-compat verification

**Files:**
- Modify: `internal/e2e/client.go` (verify only — likely no changes)
- Modify: `internal/e2e/helpers_test.go` (port wiring)

- [ ] **Step 1: Failing test** (REQ-TD-5a, REQ-TD-5b):

```go
func TestHeadlessClient_DialsHeadlessPort(t *testing.T) {
    e2e := newE2EHarness(t)
    c, err := e2e.HeadlessClient.Dial(e2e.HeadlessAddr())
    require.NoError(t, err)
    defer c.Close()
    // Issue any real RPC-equivalent and verify response.
    res, err := c.Status()
    require.NoError(t, err)
    require.NotZero(t, res.Uptime)
}

func TestHeadlessClient_DoesNotDialPlayerPort(t *testing.T) {
    e2e := newE2EHarness(t)
    require.NotEqual(t, e2e.HeadlessAddr(), e2e.PlayerAddr())
}
```

- [ ] **Step 2:** Audit `internal/e2e/client.go` for any hard-coded port `4000`. Replace with `cfg.Telnet.HeadlessPort`. Run the full `interactive-test-suite` (`go test ./internal/e2e/...`) to confirm green.

- [ ] **Step 3:** If any test fixture started the player port and expected to dial it, retarget to the headless port. The seed-claude-accounts mechanism MUST seed before the test acquires its connection.

---

### Task 8: Documentation + diagrams

**Files:**
- Modify: `docs/features/claude-gameserver-skill.md`
- Modify: `docs/features/telnet-deprecation.md`
- Modify: `docs/architecture/*.md` (any diagram referencing telnet as a player path)
- Modify: `internal/frontend/doc.go` (or `internal/frontend/handlers/doc.go`) — package summary

- [ ] **Step 1: Author** the updated claude-gameserver-skill doc:

```markdown
## Direct telnet access (debug only)

The telnet port is retained for plain-text system debugging only. The web
client is the sole player-facing surface.

- Headless port: `127.0.0.1:4002` (loopback only).
- Authorization: seed-claude-accounts seeded login + token.
- Available commands: `status`, `log [tail|grep <pat>]`, `ping`, `who`, `rpc <method> <json>`, `help`, `quit`.
- Game commands (`look`, `attack`, `move`, etc.) are NOT available on the
  debug surface. The rejector on port 4000 emits a redirect to the web
  client URL.

For graceful sunset operations only, the legacy player flow can be
re-enabled by setting `telnet.allow_game_commands: true` in the config.
This MUST NOT be enabled in production.
```

- [ ] **Step 2: Update** the package summary comment per REQ-TD-4c:

```go
// Package frontend (telnet) — retained for headless test/debug access only.
// The web client is the supported player surface; the player-facing telnet
// flow has been retired. See docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md
// and docs/features/telnet-deprecation.md.
package telnet
```

- [ ] **Step 3:** Sweep `docs/` for player-facing telnet command documentation; replace each instance with a redirect to the web client docs.

- [ ] **Step 4:** Update architecture diagrams (`docs/architecture/mud-overview.md`, `docs/architecture/mud-gameserver.md`, `docs/architecture/mud-commands.md`) to remove the player→telnet→gameserver path and leave only web→gameserver and (debug-only) operator→telnet→gameserver.

- [ ] **Step 5:** Mark `docs/features/telnet-deprecation.md` as `status: done` in the features index when the implementation lands.

---

### Task 9: Integration smoke test

**Files:**
- Create: `internal/e2e/telnet_deprecation_smoke_test.go`

- [ ] **Step 1: End-to-end test** verifying the cross-task contract:

```go
func TestTelnetDeprecation_SmokeEndToEnd(t *testing.T) {
    e2e := newE2EHarness(t)

    // 1. Player port 4000 prints redirect and closes.
    conn, err := net.DialTimeout("tcp", e2e.PlayerAddr(), 2*time.Second)
    require.NoError(t, err)
    buf := readUpToWithDeadline(t, conn, 1024, 1*time.Second)
    require.Contains(t, string(buf), "web client")
    require.Eventually(t, func() bool {
        _, err := conn.Read(make([]byte, 1))
        return err != nil
    }, 1*time.Second, 50*time.Millisecond)

    // 2. Headless port 4002 (loopback only).
    require.True(t, isLoopback(e2e.HeadlessAddr()))

    // 3. Headless without seed → reject.
    hConn, _ := net.Dial("tcp", e2e.HeadlessAddr())
    fmt.Fprintln(hConn, `{"login":"nobody"}`)
    rej := readUpToWithDeadline(t, hConn, 1024, 1*time.Second)
    require.Contains(t, string(rej), "not seed-authorized")
    hConn.Close()

    // 4. Headless with seed → debug shell, status command works.
    seeded := seed.MustSeed(t, "claude-test-account")
    hConn2, _ := net.Dial("tcp", e2e.HeadlessAddr())
    fmt.Fprintf(hConn2, `{"login":"%s","seed_token":"%s"}`+"\n", seeded.Login, seeded.Token)
    fmt.Fprintln(hConn2, "status")
    out := readUpToWithDeadline(t, hConn2, 4096, 2*time.Second)
    require.Contains(t, string(out), "uptime")
    require.Contains(t, string(out), "build")

    // 5. Debug shell rejects game commands.
    fmt.Fprintln(hConn2, "look")
    out2 := readUpToWithDeadline(t, hConn2, 4096, 1*time.Second)
    require.Contains(t, string(out2), "unknown debug command")
}
```

- [ ] **Step 2:** Confirm `interactive-test-suite` passes end-to-end (REQ-TD-5a).

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
helm template deployments/k8s/mud > /tmp/rendered.yaml
! grep -E "type: LoadBalancer|nodePort: 30400|port: 4000$" /tmp/rendered.yaml
```

Additional sanity:

- `go vet ./...` clean.
- Manual test: deploy locally with `make k8s-redeploy`; verify `kubectl get svc -n mud frontend` shows `ClusterIP`; `kubectl port-forward -n mud svc/frontend 4002:4002` then `nc 127.0.0.1 4002` and run `status` after providing seed-token.
- Manual test: `nc <prod-host> 4000` from outside the cluster fails to connect (or hangs because there's no external route).

---

## Rollout / Open Questions Resolved at Plan Time

- **Helm strategy**: `ClusterIP` for in-cluster debug access. Confirmable at Task 4 checkpoint.
- **Debug commands**: `status`, `log`, `ping`, `who`, `rpc`, `help`, `quit`. Confirmable at Task 5 checkpoint.
- **Sunset flag default**: `false`. Operators may flip to `true` for time-bounded migration assistance, MUST flip back before the migration ticket closes.
- **Headless port stays distinct**: 4002 as already configured in `dev.yaml`; aligned to `prod.yaml` and `docker.yaml` in Task 1.

## Non-Goals

Per spec REQ-TD-1..5:

- No changes to the web client's UX; it is the supported surface and stays as-is.
- No changes to the gameserver's gRPC API.
- No removal of the `internal/frontend/handlers/` package — the package stays importable for the graceful-sunset path.
- No removal of telnet rendering primitives from `internal/frontend/telnet/` — they may serve future debug rendering needs.
- No persistence schema changes.
