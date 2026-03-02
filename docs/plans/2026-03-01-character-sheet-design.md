# Character Sheet — Design

**Date:** 2026-03-01
**Feature:** `char` command — displays a full character sheet (identity, abilities, combat stats, equipped gear, currency).

---

## Summary

A new `char` command (alias `sheet`) sends a `CharacterSheetView` proto message to the client containing all character data. The sheet is assembled in `grpc_service.go` from `PlayerSession`, `JobRegistry`, and `Equipment.DefenseStats()`. No new DB queries are needed at sheet-view time.

---

## Architecture

Three layers following CMD rules:

1. **Proto layer** — `CharacterSheetRequest` added to `ClientMessage` oneof (field 21 or next available); `CharacterSheetView` added to `ServerEvent` oneof.

2. **Command layer** — `HandlerChar` constant + `Command{Name:"char", Aliases:["sheet"]}` in `commands.go`. `HandleChar(sess *session.PlayerSession, jobRegistry *ruleset.JobRegistry, invRegistry *inventory.Registry) string` in `command/char.go` returns a plain-text fallback string (for telnet/dev server).

3. **Service layer** — `handleChar` in `grpc_service.go` builds `CharacterSheetView` from session data, sends `ServerEvent{Payload: &gamev1.ServerEvent_CharacterSheet{...}}`. `bridgeChar` in `bridge_handlers.go` wires the frontend.

---

## Data Schema

### Proto: `CharacterSheetView`

```proto
message CharacterSheetRequest {}

message CharacterSheetView {
    // Identity
    string name      = 1;
    string job       = 2;   // job display name, e.g. "Boot (Gun)"
    string archetype = 3;   // e.g. "aggressor"
    string team      = 4;   // "gun" or "machete"
    int32  level     = 5;

    // Ability scores
    int32 brutality = 6;
    int32 grit      = 7;
    int32 quickness = 8;
    int32 reasoning = 9;
    int32 savvy     = 10;
    int32 flair     = 11;

    // Combat
    int32 current_hp    = 12;
    int32 max_hp        = 13;
    int32 ac_bonus      = 14;
    int32 check_penalty = 15;
    int32 speed_penalty = 16;

    // Currency
    string currency = 17;   // formatted via inventory.FormatRounds

    // Equipped gear (slot → item display name; empty string = unequipped)
    map<string, string> armor       = 18;
    map<string, string> accessories = 19;
    string main_hand = 20;
    string off_hand  = 21;
}
```

### Session data sources

| Field | Source |
|---|---|
| name | `sess.CharName` |
| job (display name) | `jobRegistry.Job(sess.Class).Name` |
| archetype | `jobRegistry.Job(sess.Class).Archetype` |
| team | `sess.Team` (or `jobRegistry.TeamFor(sess.Class)`) |
| level | `sess.Level` |
| ability scores | `character.Abilities` — loaded at login onto session OR fetched from DB |
| current_hp | `sess.CurrentHP` |
| max_hp | `sess.MaxHP` — add to `PlayerSession` if missing |
| defense stats | `sess.Equipment.DefenseStats(invRegistry)` |
| currency | `inventory.FormatRounds(sess.Currency)` |
| armor | `sess.Equipment.Armor` slot map |
| accessories | `sess.Equipment.Accessories` slot map |
| main_hand / off_hand | `sess.LoadoutSet.Active()` weapon pair |

### PlayerSession additions

If `MaxHP` and `Abilities` are not already on `PlayerSession`, add them and populate at login alongside existing character loads.

---

## Key Files

| File | Change |
|---|---|
| `api/proto/game/v1/game.proto` | ADD `CharacterSheetRequest` + `CharacterSheetView`; wire into oneofs |
| `internal/game/command/commands.go` | ADD `HandlerChar` constant + `Command{Name:"char"}` entry |
| `internal/game/command/char.go` | NEW — `HandleChar` returning plain-text fallback |
| `internal/game/command/char_test.go` | NEW — unit + property tests |
| `internal/game/session/manager.go` | MODIFY if `MaxHP`/`Abilities` missing from `PlayerSession` |
| `internal/gameserver/grpc_service.go` | ADD `handleChar`; populate `MaxHP`/`Abilities` at login if needed |
| `internal/frontend/handlers/bridge_handlers.go` | ADD `bridgeChar` + register in `bridgeHandlerMap` |

---

## Testing

### Unit tests (`internal/game/command/char_test.go`)

- `TestHandleChar_ReturnsNonEmptyString` — non-nil session returns non-empty output
- `TestHandleChar_ShowsJobName` — output contains job display name
- `TestHandleChar_UnknownJobGraceful` — unknown class ID returns output without panic

### Property test

- `TestProperty_HandleChar_NeverPanics` — random valid/invalid class IDs never panic (rapid)

### Wiring test

- `TestAllCommandHandlersAreWired` must pass (already enforces bridge completeness)
