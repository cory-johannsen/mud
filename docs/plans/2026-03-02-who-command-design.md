# Who Command Enhancement — Design

**Date:** 2026-03-02
**Feature:** Enhance `who` command output to include level, job, descriptive health, and status per player.

---

## Summary

The `who` command currently returns a bare list of character names. This design adds per-player detail: level, job display name, descriptive health label (5 tiers), and combat status (4 values). Scope remains the current room. The `PlayerList` proto message gains a `repeated PlayerInfo players` field replacing `repeated string players`.

---

## Architecture

**Proto layer** — Add `CombatStatus` enum and `PlayerInfo` message. Replace `repeated string players` in `PlayerList` with `repeated PlayerInfo players`. Run `make proto`.

**Session layer** — Add `Status CombatStatus` field to `PlayerSession`. Combat handlers set it to `IN_COMBAT` on attack and `IDLE` on flee/end. `PlayersInRoom` updated (or a new `PlayersInRoomDetails` method added) to return full session data.

**Command layer** — Add `HandleWho` in `internal/game/command/who.go` with pure helper functions `HealthLabel(current, max int) string` and `StatusLabel(status int32) string` for plain-text fallback and unit testing.

**Service layer** — `handleWho` in `grpc_service.go` builds `PlayerList` with `PlayerInfo` entries. `ChatHandler.Who` updated to return `*gamev1.PlayerList` with `PlayerInfo` fields. `RenderPlayerList` in `text_renderer.go` updated to format the richer entries.

---

## Data Schema

### Proto additions

```proto
enum CombatStatus {
    COMBAT_STATUS_UNSPECIFIED = 0;
    COMBAT_STATUS_IDLE        = 1;
    COMBAT_STATUS_IN_COMBAT   = 2;
    COMBAT_STATUS_RESTING     = 3;
    COMBAT_STATUS_UNCONSCIOUS = 4;
}

message PlayerInfo {
    string       name         = 1;
    int32        level        = 2;
    string       job          = 3;   // display name, e.g. "Striker (Gun)"
    string       health_label = 4;   // see thresholds below
    CombatStatus status       = 5;
}
```

`PlayerList` updated:
```proto
message PlayerList {
    string              room_title = 1;
    repeated PlayerInfo players    = 2;   // replaces repeated string
}
```

### Health label thresholds

| HP%        | Label           |
|-----------|-----------------|
| 100%      | Uninjured       |
| 75–99%    | Lightly Wounded |
| 50–74%    | Wounded         |
| 25–49%    | Badly Wounded   |
| < 25%     | Near Death      |

### Status values

| CombatStatus            | Display      |
|------------------------|--------------|
| COMBAT_STATUS_IDLE     | Idle         |
| COMBAT_STATUS_IN_COMBAT| In Combat    |
| COMBAT_STATUS_RESTING  | Resting      |
| COMBAT_STATUS_UNCONSCIOUS | Unconscious |

### PlayerSession addition

```go
Status int32  // maps to CombatStatus enum values; default 0 = IDLE
```

### Output format (per player)

```
  Raze — Lvl 3 Striker (Gun) — Lightly Wounded — Idle
```

---

## Key Files

| File | Change |
|---|---|
| `api/proto/game/v1/game.proto` | ADD `CombatStatus` enum + `PlayerInfo` message; UPDATE `PlayerList.players` field |
| `internal/game/session/manager.go` | ADD `Status int32` to `PlayerSession`; UPDATE `PlayersInRoom` → return session structs with all needed fields |
| `internal/game/command/who.go` | NEW — `HandleWho`, `HealthLabel`, `StatusLabel` pure functions |
| `internal/game/command/who_test.go` | NEW — unit + property tests |
| `internal/gameserver/chat_handler.go` | UPDATE `Who` to build `PlayerInfo` entries |
| `internal/gameserver/grpc_service.go` | UPDATE `handleWho` (already delegates to chatH.Who) |
| `internal/frontend/handlers/text_renderer.go` | UPDATE `RenderPlayerList` to format `PlayerInfo` fields |

---

## Testing

### Unit tests (`internal/game/command/who_test.go`)
- `TestHealthLabel_AllThresholds` — all 5 label boundaries
- `TestHealthLabel_ZeroMaxHP` — no divide-by-zero
- `TestHandleWho_ReturnsNonEmptyString`
- `TestProperty_HealthLabel_NeverPanics` — rapid property test

### Service tests (`internal/gameserver/grpc_service_who_test.go`)
- `TestHandleWho_ReturnsPlayerInfoFields` — level, job, health_label, status populated
- `TestHandleWho_UnknownSession` — error on missing session

### Renderer tests
- `TestRenderPlayerList_ShowsLevelAndJob`
- `TestRenderPlayerList_ShowsHealthLabel`
- `TestRenderPlayerList_EmptyList` — "Nobody else is here." preserved
