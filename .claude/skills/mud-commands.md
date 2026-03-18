---
name: mud-commands
description: Command registration, bridge dispatch, and gRPC wiring — load when adding or modifying player commands, bridge handlers, or proto messages.
type: reference
---

## Trigger

Load when adding or modifying player commands, bridge handlers, or proto messages.

## Responsibility Boundary

This skill covers the full lifecycle of a player command: constant registration, `BuiltinCommands()` entry, frontend bridge dispatch, proto message encoding, gRPC stream transport, and gameserver handler routing. It does not cover game logic inside `handle<Name>` functions (see mud-combat, mud-character) or terminal rendering (see mud-ui).

## Key Files

- `/home/cjohannsen/src/mud/internal/game/command/commands.go` — Handler constants, `Command` struct, `BuiltinCommands()` slice, `RegisterShortcuts()`, and `IsMovementCommand()`
- `/home/cjohannsen/src/mud/internal/frontend/handlers/bridge_handlers.go` — `bridgeHandlerMap` (single source of dispatch truth), all `bridge<Name>` functions, `bridgeContext`, `bridgeResult`, and `BridgeHandlers()` test export
- `/home/cjohannsen/src/mud/internal/frontend/handlers/game_bridge.go` — `gameBridge()`, `commandLoop()`, `forwardServerEvents()`, `BuildPrompt()`, and `StartIdleMonitor()`
- `/home/cjohannsen/src/mud/internal/frontend/telnet/acceptor.go` — `Acceptor`, `ListenAndServe()`, per-connection `handleConn()`, telnet negotiation and NAWS await
- `/home/cjohannsen/src/mud/internal/gameserver/grpc_service.go` — `GameService`, `Session()` stream handler, `dispatch` type switch routing all `ClientMessage` oneofs to `handle<Name>` functions

## Core Data Structures

**`command.Command`** (commands.go)
```
Name     string   // canonical command name
Aliases  []string // alternate names
Help     string   // short description shown in help
Category string   // movement | world | combat | communication | system | admin | character | hidden
Handler  string   // maps to bridgeHandlerMap key and proto message type
```

**`handlers.bridgeContext`** (bridge_handlers.go)
```
reqID    string                          // monotonic request ID (req-N)
cmd      *command.Command                // resolved Command entry
parsed   command.ParseResult             // raw tokens from Parse()
conn     *telnet.Conn                    // player's telnet connection
charName string                          // character display name
role     string                          // account role (player | admin | editor)
stream   gamev1.GameService_SessionClient // bidirectional gRPC stream
helpFn   func()                          // renders help and re-prompts
promptFn func() string                   // builds current colored prompt string
```

**`handlers.bridgeResult`** (bridge_handlers.go)
```
msg             *gamev1.ClientMessage // proto message to send (nil = nothing)
done            bool                  // handler wrote output locally; continue loop
quit            bool                  // clean disconnect; return nil from commandLoop
switchCharacter bool                  // return ErrSwitchCharacter from commandLoop
```

**`gamev1.ClientMessage`** (generated proto)
- Oneof `payload` with one variant per command (e.g., `Move`, `Look`, `Attack`, `SelectTech`, …)

**`gamev1.ServerEvent`** (generated proto)
- Oneof `payload` with one variant per response type (e.g., `RoomView`, `CombatEvent`, `CharacterSheet`, …)

## Primary Data Flow

1. Player types command in telnet terminal
2. `internal/frontend/telnet/conn.go` reads input line
3. `internal/frontend/handlers/game_bridge.go` parses command string → looks up in `BuiltinCommands()`
4. Dispatches to matching `bridge<Name>` func in `internal/frontend/handlers/bridge_handlers.go`
5. Bridge func encodes proto `ClientMessage` oneof variant
6. Sends over bidirectional gRPC `Session` stream to gameserver
7. `internal/gameserver/grpc_service.go` `dispatch` type switch routes to `handle<Name>`
8. Handler executes game logic, builds `ServerEvent` oneof response
9. Response streams back to frontend → rendered to telnet terminal

### Unknown command fallback

If `registry.Resolve()` returns no match, `commandLoop` treats the input as a movement direction (custom exit name) and sends a `MoveRequest` directly, bypassing the bridge map.

### Server-event rendering

`forwardServerEvents()` runs in a dedicated goroutine. It reads `ServerEvent` from the stream and calls the appropriate `Render*` function in `internal/frontend/handlers/`. `RoomView` events go to `conn.WriteRoom()`; all others go to `conn.WriteConsole()` (split-screen) or `conn.WriteLine()`.

## Invariants & Contracts

- Every `HandlerXxx` constant MUST have a `Command{...}` entry in `BuiltinCommands()` (CMD-2)
- Every `BuiltinCommands()` entry MUST have a `bridge<Name>` in `bridgeHandlerMap` (CMD-5); `TestAllCommandHandlersAreWired` enforces this
- Every bridge func MUST have a corresponding `handle<Name>` in grpc_service.go (CMD-6)
- `bridgeResult.msg` may be nil only when `done`, `quit`, or `switchCharacter` is true
- `commandLoop` returns `nil` on clean quit, `ErrSwitchCharacter` on character switch, `ctx.Err()` on cancellation, or a wrapped error on I/O failure
- `RegisterShortcuts` panics at startup on any shortcut collision with an existing command name or alias (fail-fast)
- `CategoryHidden` commands appear in `BuiltinCommands()` and `bridgeHandlerMap` but are excluded from `showGameHelp()` output

## Extension Points

Adding a new command requires ALL of the following steps. Omitting any is a defect (CMD-7).

CMD-1: Add `HandlerFoo` constant to `internal/game/command/commands.go`
CMD-2: Add `Command{Handler: HandlerFoo, Name: "foo", ...}` to `BuiltinCommands()` in the same file
CMD-3: Implement `HandleFoo(char, args) (string, error)` in `internal/game/command/foo.go` with TDD coverage (SWENG-5, SWENG-5a)
CMD-4: Add `FooRequest` proto message to `api/proto/game/v1/game.proto` and add to `ClientMessage` oneof; run `make proto`
CMD-5: Add `bridgeFoo` func to `bridge_handlers.go` and register in `bridgeHandlerMap`; `TestAllCommandHandlersAreWired` MUST pass
CMD-6: Implement `handleFoo` in `internal/gameserver/grpc_service.go` and wire into `dispatch` type switch
CMD-7: All steps complete; all tests pass before command is considered done

## Common Pitfalls

- Adding a Handler constant but not a `BuiltinCommands()` entry causes the command to be unreachable — `registry.Resolve()` will never match it.
- Adding a `BuiltinCommands()` entry but not a `bridgeHandlerMap` entry causes the runtime fallback message "You don't know how to '...'" to appear — `TestAllCommandHandlersAreWired` will also catch this at test time.
- Adding a bridge func but not wiring `handle<Name>` in `grpc_service.go` causes the gRPC dispatch to silently drop the message.
- Forgetting `make proto` after editing `.proto` leaves the generated Go types stale; the build will fail.
- `RegisterShortcuts` is called at character-load time with the character's active class features. Shortcut names must not collide with any name or alias already in `BuiltinCommands()`.
- Movement commands (`north`, `south`, etc.) share the single `HandlerMove` constant; the `parsed.Command` field carries the direction string to `bridgeMove`.
- `CategoryHidden` commands (e.g., `archetype_selection`) are valid and fully wired but intentionally absent from player-facing help output.
