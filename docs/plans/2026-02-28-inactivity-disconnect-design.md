# Inactivity Warning + Disconnect Design

**Date:** 2026-02-28

## Goal

Warn idle players after 5 minutes of no input, then disconnect them 1 minute later. Log every disconnect server-side with reason, player name, account, session duration, and room.

## Constraints

- Inactivity is measured as time since last player input only — server-generated messages do not reset the timer.
- No proto changes, no server-side changes, no new commands.
- Timeouts are configurable in `TelnetConfig`.

## Architecture

The idle monitor lives entirely in the frontend. `commandLoop` in `game_bridge.go` gets two additions:
1. A shared `lastInput` atomic timestamp (`sync/atomic int64`, Unix nanos) updated on every successful `ReadLine()`.
2. An `idleMonitor` goroutine launched at the start of `commandLoop`, ticking every 30 seconds.

When `gameBridge` returns (for any reason), it logs the disconnect with all required fields. The disconnect reason distinguishes inactivity, quit, and connection error.

## Components

- `internal/frontend/handlers/game_bridge.go` — add `idleMonitor` goroutine; update `lastInput` on each `ReadLine()`; add `disconnectReason` tracking; log disconnect on `gameBridge` return
- `internal/frontend/handlers/game_bridge_test.go` — tests for idle monitor (warning at idle timeout, disconnect after grace period, timer reset on input, no double-warning)
- `internal/config/config.go` — add `IdleTimeout time.Duration` and `IdleGracePeriod time.Duration` to `TelnetConfig` (defaults: 5m and 1m)

## Data Flow

```
Player connects → gameBridge starts → idleMonitor goroutine launched
                                           │
                  Player types anything    │  tick every 30s
                  → ReadLine() succeeds    │  check time.Since(lastInput)
                  → lastInput updated      │
                                           │  >= IdleTimeout: send warning
                                           │  >= IdleTimeout+GracePeriod: cancel(errInactivity)
                                                      └─> commandLoop exits
                                                      └─> gameBridge returns
                                                      └─> log disconnect:
                                                            reason: "inactivity"
                                                            player: CharName
                                                            account: Username
                                                            duration: time.Since(start)
                                                            room: sess.RoomID
```

## Disconnect Log Fields

Logged at `Info` level via `go.uber.org/zap` on every `gameBridge` return:

| Field | Value |
|---|---|
| `reason` | `"inactivity"` / `"quit"` / `"connection_error"` |
| `player` | `character.Name` |
| `account` | `account.Username` |
| `session_duration` | `time.Since(sessionStart)` |
| `room_id` | `sess.RoomID` (from PlayerSession) |

## Error Handling

- `idleMonitor` exits cleanly on context cancellation — no goroutine leak.
- `lastInput` uses `sync/atomic` for lock-free concurrent access.
- Warning is sent once only — tracked with a boolean flag in the goroutine.
- Write failure on warning send is logged and the goroutine continues.

## Testing

- Unit tests use short configurable timeouts (e.g. 100ms idle, 50ms grace) to verify warning and disconnect timing without sleeping for minutes.
- Property-based test: any sequence of inputs within the idle window never triggers a disconnect.
- Test: warning sent exactly once per idle period.
- Test: input resets the idle timer (no disconnect if player is active).
- Test: disconnect fires after idle + grace period with no further input.
