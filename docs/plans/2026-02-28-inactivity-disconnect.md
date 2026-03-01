# Inactivity Warning + Disconnect Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Warn idle players after 5 minutes of no input, then disconnect them 1 minute later, logging the disconnect reason, player, account, session duration, and room.

**Architecture:** Add `IdleTimeout` and `IdleGracePeriod` fields to `TelnetConfig`. In `commandLoop`, update a `sync/atomic int64` timestamp on every successful `ReadLine()`. A companion `idleMonitor` goroutine ticks every 30 seconds, sends a warning at `IdleTimeout`, and cancels the context at `IdleTimeout + IdleGracePeriod`. `gameBridge` logs all disconnects on return.

**Tech Stack:** Go, `sync/atomic`, `go.uber.org/zap`, `pgregory.net/rapid`

---

## Context for Implementer

### Repo layout (relevant paths)
- `internal/config/config.go` — `TelnetConfig` struct (lines 45-55), defaults at lines 279-282
- `internal/frontend/handlers/game_bridge.go` — `gameBridge()` (line 30), `commandLoop()` (line 114)
- `internal/frontend/handlers/game_bridge_test.go` — does not exist yet; create it

### How gameBridge works
1. Opens gRPC stream to game server
2. Sends `JoinWorldRequest` with character info
3. Spawns `forwardServerEvents` goroutine
4. Calls `commandLoop` which blocks reading Telnet input
5. On return, calls `cancel()` and `wg.Wait()`

### commandLoop structure
```
for {
    select { case <-ctx.Done(): return ctx.Err(); default: }
    line, err := conn.ReadLine()   // blocking — returns on input or timeout
    // parse and dispatch
}
```

### Logging
Uses `go.uber.org/zap`. The `AuthHandler` has a `h.logger *zap.Logger` field. Log with:
```go
h.logger.Info("player disconnected",
    zap.String("reason", reason),
    zap.String("player", charName),
    zap.String("account", acct.Username),
    zap.Duration("session_duration", time.Since(start)),
    zap.String("room_id", roomID),
)
```

### Run tests with
```bash
go test ./internal/config/... ./internal/frontend/handlers/... -v
```

---

## Task 1: Add IdleTimeout and IdleGracePeriod to TelnetConfig

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Read the TelnetConfig struct**

Open `internal/config/config.go`. Find the `TelnetConfig` struct (around line 45) and the defaults (around line 279).

**Step 2: Add fields to TelnetConfig**

Change:
```go
type TelnetConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}
```

To:
```go
type TelnetConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	IdleGracePeriod time.Duration `mapstructure:"idle_grace_period"`
}
```

**Step 3: Add defaults**

After the existing `v.SetDefault("telnet.write_timeout", "30s")` line, add:
```go
v.SetDefault("telnet.idle_timeout", "5m")
v.SetDefault("telnet.idle_grace_period", "1m")
```

**Step 4: Verify it compiles**
```bash
go build ./internal/config/...
```
Expected: no errors

**Step 5: Commit**
```bash
git add internal/config/config.go
git commit -m "feat: add IdleTimeout and IdleGracePeriod to TelnetConfig"
```

---

## Task 2: Implement idleMonitor and disconnect logging in gameBridge

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Create: `internal/frontend/handlers/game_bridge_test.go`

### Background

`gameBridge` signature:
```go
func (h *AuthHandler) gameBridge(ctx context.Context, conn *telnet.Conn, acct postgres.Account, char *character.Character) error
```

`commandLoop` signature:
```go
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string) error
```

The idle monitor needs:
- `idleTimeout` and `idleGracePeriod` from config (passed in or read from handler)
- `lastInput` atomic int64 (Unix nanos) shared between commandLoop and idleMonitor
- A cancel function to terminate the session on inactivity
- The player's room ID for logging — read from `sess.RoomID` but that requires the PlayerSession from the game server. Since the frontend doesn't directly have RoomID after join, pass `char.Location` as the room ID for logging (it's the room the player started in; good enough for disconnect logging).

**Step 1: Write the failing tests**

Create `internal/frontend/handlers/game_bridge_test.go`:

```go
package handlers_test

import (
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

// TestIdleMonitor_WarningAfterIdleTimeout verifies that the idle monitor sends
// a warning callback after the idle timeout and a disconnect callback after
// the grace period.
func TestIdleMonitor_WarningAfterIdleTimeout(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())

	warningCalled := make(chan struct{}, 1)
	disconnectCalled := make(chan struct{}, 1)

	idleTimeout := 100 * time.Millisecond
	gracePeriod := 50 * time.Millisecond
	tickInterval := 20 * time.Millisecond

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  idleTimeout,
		GracePeriod:  gracePeriod,
		TickInterval: tickInterval,
		OnWarning: func() {
			select {
			case warningCalled <- struct{}{}:
			default:
			}
		},
		OnDisconnect: func() {
			select {
			case disconnectCalled <- struct{}{}:
			default:
			}
		},
	})
	defer stop()

	select {
	case <-warningCalled:
		// good
	case <-time.After(idleTimeout + 3*tickInterval):
		t.Fatal("warning not called within expected time")
	}

	select {
	case <-disconnectCalled:
		// good
	case <-time.After(gracePeriod + 3*tickInterval):
		t.Fatal("disconnect not called within expected time after warning")
	}
}

// TestIdleMonitor_InputResetsTimer verifies that input before the idle timeout
// prevents the warning from being sent.
func TestIdleMonitor_InputResetsTimer(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())

	warningCalled := make(chan struct{}, 1)

	idleTimeout := 150 * time.Millisecond
	tickInterval := 20 * time.Millisecond

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  idleTimeout,
		GracePeriod:  500 * time.Millisecond,
		TickInterval: tickInterval,
		OnWarning: func() {
			select {
			case warningCalled <- struct{}{}:
			default:
			}
		},
		OnDisconnect: func() {},
	})
	defer stop()

	// Simulate input every 50ms — well within the 150ms idle timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(idleTimeout + 3*tickInterval)
		for {
			select {
			case <-ticker.C:
				lastInput.Store(time.Now().UnixNano())
			case <-deadline:
				return
			}
		}
	}()
	<-done

	select {
	case <-warningCalled:
		t.Fatal("warning should not have been called while player was active")
	default:
		// good — no warning
	}
}

// TestIdleMonitor_StopPreventsCallbacks verifies that calling the stop function
// prevents any callbacks from firing.
func TestIdleMonitor_StopPreventsCallbacks(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().Add(-10 * time.Second).UnixNano()) // already idle

	warningCalled := make(chan struct{}, 1)

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  10 * time.Millisecond,
		GracePeriod:  10 * time.Millisecond,
		TickInterval: 5 * time.Millisecond,
		OnWarning: func() {
			select {
			case warningCalled <- struct{}{}:
			default:
			}
		},
		OnDisconnect: func() {},
	})

	// Stop immediately before monitor can fire
	stop()

	select {
	case <-warningCalled:
		t.Fatal("warning should not fire after stop()")
	case <-time.After(50 * time.Millisecond):
		// good
	}
}

// TestIdleMonitor_WarningOnlyOnce verifies that the warning callback is called
// exactly once even if the monitor keeps ticking.
func TestIdleMonitor_WarningOnlyOnce(t *testing.T) {
	var lastInput atomic.Int64
	lastInput.Store(time.Now().Add(-10 * time.Second).UnixNano()) // already idle

	var warningCount atomic.Int64

	stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  10 * time.Millisecond,
		GracePeriod:  200 * time.Millisecond, // long grace so monitor keeps ticking
		TickInterval: 5 * time.Millisecond,
		OnWarning: func() {
			warningCount.Add(1)
		},
		OnDisconnect: func() {},
	})
	defer stop()

	time.Sleep(100 * time.Millisecond)

	if n := warningCount.Load(); n != 1 {
		t.Fatalf("expected warning called exactly once, got %d", n)
	}
}

// TestProperty_IdleMonitor_ActivePlayerNeverDisconnected verifies that
// a player who inputs at least once per half-idle-timeout is never disconnected.
func TestProperty_IdleMonitor_ActivePlayerNeverDisconnected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		var lastInput atomic.Int64
		lastInput.Store(time.Now().UnixNano())

		disconnected := make(chan struct{}, 1)

		idleTimeout := 200 * time.Millisecond
		tickInterval := 20 * time.Millisecond

		stop := handlers.StartIdleMonitor(handlers.IdleMonitorConfig{
			LastInput:    &lastInput,
			IdleTimeout:  idleTimeout,
			GracePeriod:  50 * time.Millisecond,
			TickInterval: tickInterval,
			OnWarning:    func() {},
			OnDisconnect: func() {
				select {
				case disconnected <- struct{}{}:
				default:
				}
			},
		})
		defer stop()

		// Simulate input every 80ms (well within 200ms idle timeout)
		inputCount := rapid.IntRange(3, 8).Draw(rt, "inputCount")
		for i := 0; i < inputCount; i++ {
			time.Sleep(80 * time.Millisecond)
			lastInput.Store(time.Now().UnixNano())
		}

		select {
		case <-disconnected:
			rt.Fatal("active player should never be disconnected")
		default:
			// good
		}
	})
}
```

**Step 2: Run to verify they fail**
```bash
go test ./internal/frontend/handlers/... -v -run TestIdleMonitor 2>&1 | head -20
```
Expected: FAIL with `undefined: handlers.StartIdleMonitor` or `undefined: handlers.IdleMonitorConfig`

**Step 3: Add imports to game_bridge.go**

At the top of `internal/frontend/handlers/game_bridge.go`, add these imports (merge with existing):
```go
"sync/atomic"
"time"
```

**Step 4: Add IdleMonitorConfig and StartIdleMonitor to game_bridge.go**

Add this block after the `ErrSwitchCharacter` declaration (after line 24):

```go
// IdleMonitorConfig configures the idle monitor goroutine.
type IdleMonitorConfig struct {
	// LastInput is the shared atomic timestamp (UnixNano) of the most recent player input.
	LastInput *atomic.Int64
	// IdleTimeout is the duration of silence before the warning callback fires.
	IdleTimeout time.Duration
	// GracePeriod is the duration after the warning before the disconnect callback fires.
	GracePeriod time.Duration
	// TickInterval controls how often the monitor checks for idleness.
	TickInterval time.Duration
	// OnWarning is called once when the player has been idle for IdleTimeout.
	OnWarning func()
	// OnDisconnect is called once when the player has been idle for IdleTimeout + GracePeriod.
	OnDisconnect func()
}

// StartIdleMonitor launches a goroutine that monitors player inactivity.
// It returns a stop function that terminates the goroutine cleanly.
//
// Precondition: cfg.LastInput must be non-nil; cfg.OnWarning and cfg.OnDisconnect must be non-nil.
// Postcondition: The goroutine exits when stop() is called or OnDisconnect fires.
func StartIdleMonitor(cfg IdleMonitorConfig) (stop func()) {
	done := make(chan struct{})
	stop = func() { close(done) }

	go func() {
		ticker := time.NewTicker(cfg.TickInterval)
		defer ticker.Stop()
		warningSent := false
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				idle := time.Duration(time.Now().UnixNano() - cfg.LastInput.Load())
				if !warningSent && idle >= cfg.IdleTimeout {
					warningSent = true
					cfg.OnWarning()
				}
				if warningSent && idle >= cfg.IdleTimeout+cfg.GracePeriod {
					cfg.OnDisconnect()
					return
				}
			}
		}
	}()

	return stop
}
```

**Step 5: Update gameBridge to launch idleMonitor and log disconnects**

Replace the `gameBridge` function body. The full updated function:

```go
func (h *AuthHandler) gameBridge(ctx context.Context, conn *telnet.Conn, acct postgres.Account, char *character.Character) error {
	sessionStart := time.Now()

	// Connect to gameserver
	grpcConn, err := grpc.NewClient(h.gameServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		h.logger.Error("connecting to game server", zap.Error(err))
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Failed to connect to game server. Please try again later."))
		return fmt.Errorf("dialing game server: %w", err)
	}
	defer grpcConn.Close()

	client := gamev1.NewGameServiceClient(grpcConn)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := client.Session(streamCtx)
	if err != nil {
		h.logger.Error("opening game session", zap.Error(err))
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Failed to start game session."))
		return fmt.Errorf("opening session stream: %w", err)
	}

	// Send JoinWorldRequest
	uid := fmt.Sprintf("%d", char.ID)
	if err := stream.Send(&gamev1.ClientMessage{
		RequestId: "join",
		Payload: &gamev1.ClientMessage_JoinWorld{
			JoinWorld: &gamev1.JoinWorldRequest{
				Uid:           uid,
				Username:      acct.Username,
				CharacterId:   char.ID,
				CharacterName: char.Name,
				CurrentHp:     int32(char.CurrentHP),
				Location:      char.Location,
				Role:          acct.Role,
				RegionDisplay: h.regionDisplayName(char.Region),
				Class:         char.Class,
				Level:         int32(char.Level),
			},
		},
	}); err != nil {
		return fmt.Errorf("sending join request: %w", err)
	}

	// Receive initial room view
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving initial room view: %w", err)
	}
	if rv := resp.GetRoomView(); rv != nil {
		_ = conn.Write([]byte(RenderRoomView(rv)))
	}

	// Write initial prompt
	prompt := telnet.Colorf(telnet.BrightCyan, "[%s]> ", char.Name)
	if err := conn.WritePrompt(prompt); err != nil {
		return fmt.Errorf("writing initial prompt: %w", err)
	}

	// Idle monitor: warn at IdleTimeout, disconnect at IdleTimeout + IdleGracePeriod.
	var lastInput atomic.Int64
	lastInput.Store(time.Now().UnixNano())
	disconnectReason := "quit"
	stopIdle := StartIdleMonitor(IdleMonitorConfig{
		LastInput:    &lastInput,
		IdleTimeout:  h.telnetCfg.IdleTimeout,
		GracePeriod:  h.telnetCfg.IdleGracePeriod,
		TickInterval: 30 * time.Second,
		OnWarning: func() {
			_ = conn.WriteLine(telnet.Colorize(telnet.Yellow,
				"Warning: You have been idle for 5 minutes. You will be disconnected in 1 minute."))
		},
		OnDisconnect: func() {
			disconnectReason = "inactivity"
			cancel()
		},
	})
	defer stopIdle()

	// Spawn goroutine to forward server events to Telnet.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.forwardServerEvents(streamCtx, stream, conn, char.Name)
	}()

	// Command loop: read Telnet → update lastInput → parse → send gRPC
	err = h.commandLoop(streamCtx, stream, conn, char.Name, acct.Role, &lastInput)

	cancel()
	wg.Wait()
	stopIdle()

	// Classify disconnect reason
	if err != nil && !errors.Is(err, context.Canceled) && disconnectReason == "quit" {
		disconnectReason = "connection_error"
	}
	if errors.Is(err, ErrSwitchCharacter) {
		disconnectReason = "switch_character"
	}

	h.logger.Info("player disconnected",
		zap.String("reason", disconnectReason),
		zap.String("player", char.Name),
		zap.String("account", acct.Username),
		zap.Duration("session_duration", time.Since(sessionStart)),
		zap.String("room_id", char.Location),
	)

	if errors.Is(err, ErrSwitchCharacter) {
		return ErrSwitchCharacter
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
```

**Step 6: Update commandLoop signature to accept lastInput**

Change the `commandLoop` signature from:
```go
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string) error {
```
To:
```go
func (h *AuthHandler) commandLoop(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, role string, lastInput *atomic.Int64) error {
```

After the successful `conn.ReadLine()` call (after `line, err := conn.ReadLine()` succeeds), add:
```go
lastInput.Store(time.Now().UnixNano())
```

Place it right after this block:
```go
line, err := conn.ReadLine()
if err != nil {
    return fmt.Errorf("reading input: %w", err)
}
lastInput.Store(time.Now().UnixNano())  // ← add this line
```

**Step 7: Verify AuthHandler has telnetCfg field**

Check that `AuthHandler` has access to the telnet config. Read `internal/frontend/handlers/auth.go` and look for the struct definition. If `telnetCfg` is not a field, add it:

```go
// In the AuthHandler struct, add:
telnetCfg config.TelnetConfig
```

And update the constructor to accept and store it. If the struct already has the config embedded differently, adapt accordingly — the key requirement is that `h.telnetCfg.IdleTimeout` and `h.telnetCfg.IdleGracePeriod` are accessible.

**Step 8: Run all handler tests**
```bash
go test ./internal/frontend/handlers/... -v
```
Expected: all PASS

**Step 9: Run the full build**
```bash
go build ./...
```
Expected: no errors

**Step 10: Commit**
```bash
git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/game_bridge_test.go
git commit -m "feat: add idle monitor with warning and disconnect, log all disconnects"
```

---

## Task 3: Update FEATURES.md

**Files:**
- Modify: `docs/requirements/FEATURES.md`

**Step 1: Mark feature complete**

Change:
```
- [ ] Inactivity warning before automatic disconnect.  Serverside logging of disconnect and reason.
```
To:
```
- [x] Inactivity warning before automatic disconnect.  Serverside logging of disconnect and reason.
```

**Step 2: Commit**
```bash
git add docs/requirements/FEATURES.md
git commit -m "feat: mark inactivity disconnect feature complete"
```
