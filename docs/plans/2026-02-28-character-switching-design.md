# Character Switching Design

**Date:** 2026-02-28

## Goal

Allow a player to switch to a different character on their account without disconnecting their Telnet session, using an in-game `switch` command.

## Constraints

- A character already logged in on another session cannot be selected (server enforces this).
- Switching behaves identically to `quit` for the current character: state is saved, the character is removed from the world, and a departure broadcast is sent.
- Follows CMD-1 through CMD-7 rules from AGENTS.md.

## Architecture

### Command & Server (CMD-1 through CMD-7)

- `HandlerSwitch` constant + `Command{...}` entry in `internal/game/command/commands.go`.
- `HandleSwitch` function in `internal/game/command/switch.go` — same shape as `HandleQuit`.
- `SwitchCharacterRequest {}` proto message added to `ClientMessage` oneof in `api/proto/game/v1/game.proto`; `make proto` regenerates.
- `handleSwitch` in `internal/gameserver/grpc_service.go` is identical to `handleQuit`: saves state, broadcasts `"<name> has left"`, returns `errQuit`.
- `bridgeSwitch` in `internal/frontend/handlers/bridge_handlers.go` sends `SwitchCharacterRequest` and returns `bridgeResult{switchCharacter: true}` (new boolean field on `bridgeResult`).
- `gameBridge` detects `result.switchCharacter` and returns a package-level sentinel `errSwitchCharacter`.

### Character Selection Loop

`characterFlow` in `internal/frontend/handlers/character_flow.go` currently does `return h.gameBridge(...)` at all call sites. Each becomes:

```go
err := h.gameBridge(ctx, conn, acct, selected)
if errors.Is(err, errSwitchCharacter) {
    continue // loop back to character selection screen
}
return err
```

**Duplicate character protection:** The server already rejects a `JoinWorldRequest` if the character UID is already registered in the session manager (`AddPlayer` returns an error). `gameBridge` wraps this as a recognisable error; `characterFlow` catches it, displays `"That character is already logged in."`, and loops back to the selection screen. The selection list itself does not attempt to filter logged-in characters.

## Data Flow

```
Player types "switch"
  → bridgeSwitch sends SwitchCharacterRequest
  → server handleSwitch: saves state, broadcasts departure, returns errQuit
  → gRPC stream closes
  → gameBridge sees switchCharacter=true, returns errSwitchCharacter
  → characterFlow continues loop → shows character selection screen
  → player picks new character
  → gameBridge opens new gRPC stream with new character
```

## Error Handling

- `errSwitchCharacter` is a package-level sentinel in `internal/frontend/handlers/`.
- Duplicate character login → server returns error on `JoinWorldRequest` → `gameBridge` returns wrapped error → `characterFlow` prints message and loops.
- All other errors from `gameBridge` propagate up and terminate the session normally.

## Testing

- TDD + property-based testing (SWENG-5, SWENG-5a) for each new function.
- `TestAllCommandHandlersAreWired` must pass (CMD-5 enforcement).
- Unit test: `handleSwitch` produces same result as `handleQuit`.
- Unit test: `characterFlow` loops back on `errSwitchCharacter`.
- Unit test: duplicate character login shows error and loops.
