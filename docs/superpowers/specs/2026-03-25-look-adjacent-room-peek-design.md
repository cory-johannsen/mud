# Spec: Look Into Adjacent Room (Peek)

**Date:** 2026-03-25
**Status:** Draft

---

## Overview

Players can use `look <direction>` to peek into an adjacent room. The result is gated by an Awareness-based skill check. Outcomes range from seeing nothing to a full room view. NPCs in the target room may notice the player on a critical failure.

---

## Command Syntax

- `look` (no argument) — existing behavior, renders current room; no change.
- `look <direction>` — triggers the peek flow for the exit in that direction.

Handled in `internal/gameserver/grpc_service.go` `handleLook()` (no dedicated command file exists for `look`).

### Exit Visibility Rules

- PEEK-1: A player MAY only peek through exits where `Exit.Hidden == false`. Revealed exits have `Hidden` set to `false` by `world.Manager.RevealExit()`; there is no per-player revealed-exit tracking.
- PEEK-2: Peeking toward an exit where `Exit.Hidden == true` MUST return the same "no exit" message as if the exit does not exist.
- PEEK-3: Peeking through a locked exit MUST be permitted.

---

## Skill Check

### DC Calculation

- PEEK-4: The base DC for a peek MUST come from the target room's `PeekDC int` field (YAML: `peek_dc`). If `PeekDC == 0`, the default base DC is 10.
- PEEK-5: The final DC MUST be computed as `base_dc + highest_awareness_modifier_among_npcs_in_target_room`, where each NPC's awareness modifier is `inst.Awareness` (used directly as an integer modifier, not converted via `AbilityMod`). If the target room has no NPCs, only `base_dc` applies.
- PEEK-6: The NPC with the highest `inst.Awareness` value serves as both the DC modifier source (PEEK-5) and the detection candidate (PEEK-13). When multiple NPCs share the highest value, the first in slice order MUST be selected.

### Roll

- PEEK-7: The peek check MUST use `skillcheck.Resolve(roll, abilityMod, rank, finalDC, skillcheck.TriggerDef{})` where:
  - `roll` is a d20 result (1–20)
  - `abilityMod` is `combat.AbilityMod(sess.Abilities.Savvy)` — i.e., `(Savvy - 10) / 2` using integer division with floor semantics
  - `rank` is `sess.Proficiencies["awareness"]`
  - `finalDC` is computed per PEEK-4/PEEK-5
  - `TriggerDef{}` is passed as an empty struct (consistent with the crafting skill check pattern)

### Outcomes

| Tier | Revealed to Player |
|------|--------------------|
| Crit Fail | "You can't make out anything." — NPC detection check fires (see §NPC Detection). |
| Fail | Room title only — e.g., `You glimpse: The Rusted Corridor` |
| Success | Room title + NPC names with health descriptions |
| Crit Success | Full room view: title, description, NPCs (with health), floor items |

- PEEK-8: On Crit Success, the peek result MUST include the room title, full description, all visible NPCs (`*gamev1.NpcInfo` with name and health description), and all floor items (`*gamev1.RoomEquipmentItem`).
- PEEK-9: On Success, the peek result MUST include the room title and all visible NPCs (`*gamev1.NpcInfo` with name and health description) only.
- PEEK-10: On Fail, the peek result MUST include the room title only.
- PEEK-11: On Crit Fail, the peek result MUST render "You can't make out anything." and trigger the NPC detection check.
- PEEK-12: The peek response MUST include a roll summary line showing roll, ability modifier, proficiency bonus, total, and outcome. Format: `Awareness check (DC 12): rolled 8 + 2 + 4 = 14 — success.` where the three addends are `Roll`, `AbilityMod`, and `ProfBonus` from `CheckResult`. When `AbilityMod` or `ProfBonus` is negative, a minus sign MUST be used (e.g., `rolled 8 - 2 + 4 = 10`). When either is zero, it MUST still be included as `+ 0`.

---

## NPC Detection on Crit Fail

- PEEK-13: On a Crit Fail, the NPC with the highest `Awareness` modifier in the target room (per PEEK-6) MUST be the detection candidate.
- PEEK-14: If the detection candidate's `inst.Awareness` value is ≥ 0, that NPC's `Disposition` field MUST be set to `"hostile"`.
- PEEK-15: On detection, `PeekResult.DetectedNPCID` MUST be set to the detecting NPC's `inst.ID`. The Lua hook `on_peek_detected(npc_instance_id, player_uid)` MUST be fired from `handleLook()` in `grpc_service.go` (where `scriptMgr` is accessible) after `Peek()` returns, using `PeekResult.DetectedNPCID`.
- PEEK-16: If the target room has no NPCs, NPC detection MUST be skipped entirely.

---

## Architecture

### New / Modified Components

**`internal/game/world/model.go`**
- PEEK-17: `Room` MUST gain an optional `PeekDC int` field (YAML tag: `peek_dc`; zero value means use default DC 10 per PEEK-4).

**`internal/gameserver/peek_result.go`** (new file in gameserver package)
- PEEK-18: A `PeekResult` struct MUST be defined with fields:
  - `CheckResult skillcheck.CheckResult` — outcome is accessed via `CheckResult.Outcome`
  - `RoomTitle string`
  - `Description string` — populated only on Crit Success
  - `NPCs []*gamev1.NpcInfo` — populated on Success and Crit Success
  - `Items []*gamev1.RoomEquipmentItem` — populated only on Crit Success
  - `Detected bool` — true when NPC detection fires
  - `DetectedNPCID string` — instance ID of the detecting NPC; empty when `Detected == false`

**`internal/gameserver/world_handler.go`**
- PEEK-19: A new `Peek(uid, direction string) (*PeekResult, error)` method MUST be added to `WorldHandler`.
  - Validates the exit is visible to the player (PEEK-1/PEEK-2).
  - Computes `finalDC` per PEEK-4/PEEK-5.
  - Invokes `skillcheck.Resolve()` per PEEK-7.
  - Populates and returns `PeekResult` per outcome tier (PEEK-8 through PEEK-11).
  - Mutates detecting NPC's `Disposition` and sets `PeekResult.DetectedNPCID` per PEEK-13 through PEEK-16. Does NOT fire the Lua hook (see PEEK-15).

**`internal/gameserver/grpc_service.go`**
- PEEK-20: The proto file `api/proto/game/v1/game.proto` MUST be updated to add an optional `string direction = 1` field to `message LookRequest {}`. After this change, `make proto` MUST be run to regenerate `gamev1/game.pb.go`.
- PEEK-20a: The dispatch site in `grpc_service.go` (`case *gamev1.ClientMessage_Look`) MUST be updated to pass `p.Look.Direction` to `handleLook(uid, direction string)`.
- PEEK-20b: `handleLook(uid, direction string)` MUST detect a non-empty direction argument. When present, it MUST call `worldH.Peek()`, render the result using `RenderPeekResult()`, send the output as console text, and fire the Lua hook per PEEK-15. When direction is empty, existing `worldH.Look()` behavior is unchanged.
- PEEK-20c: Peek MUST be blocked for detained players. The detained-player allowlist (type-switch guard in `grpc_service.go`) MUST continue to pass `LookRequest` only when `direction` is empty. A `LookRequest` with a non-empty `direction` while detained MUST be rejected with the same response as other blocked commands.

**`internal/gameserver/peek_result.go`** (rendering, same package as `PeekResult`)
- PEEK-21: A `RenderPeekResult(pr *PeekResult, width int) string` function MUST be defined in the `gameserver` package. It MUST format peek output as a console message block. The frontend layer does NOT render peek results; rendering lives in the gameserver package alongside `PeekResult` to avoid cross-layer imports.

### Unchanged Components

- `skillcheck.Resolve()` and `skillcheck.OutcomeFor()` — used as-is.
- `RenderRoomView()` — unchanged; peek output is console text, not a room repaint.
- Exit hiding logic (dark periods) — unchanged; if exits are hidden due to darkness they cannot be peeked through.
- NPC combat rendering helpers in the frontend — NOT reused by `RenderPeekResult` (rendering is in the gameserver package). `RenderPeekResult` renders NPC names and health descriptions directly from `*gamev1.NpcInfo` fields.

---

## Data Model Change

```yaml
# Zone YAML example
rooms:
  - id: the_rusted_corridor
    title: The Rusted Corridor
    peek_dc: 12          # optional; defaults to 10 if omitted (zero value)
    # ... rest of room definition
```

---

## Testing

- PEEK-22: Unit tests MUST cover all four outcome tiers for `Peek()` using property-based testing. Invariant: for a fixed DC, increasing the roll value MUST produce an equal or better outcome (monotonicity).
- PEEK-23: Unit tests MUST cover DC calculation: no NPCs present (base DC only), NPCs present (base DC + highest `inst.Awareness`).
- PEEK-24: Unit tests MUST cover NPC detection: Crit Fail with candidate `inst.Awareness` ≥ 0 (becomes hostile), Crit Fail with candidate `inst.Awareness` < 0 (no disposition change), no NPCs present (detection skipped).
- PEEK-25: Unit tests MUST cover exit visibility gating: exit with `Hidden == true` (blocked), exit with `Hidden == false` (allowed), locked exit with `Hidden == false` (allowed), nonexistent direction (error).
- PEEK-26: Unit tests MUST cover `PeekResult` field population per outcome tier: fields absent when not applicable, fields present when applicable.
- PEEK-27: Integration tests MUST verify `handleLook` with a direction argument dispatches to `Peek()`, and `handleLook` with no argument dispatches to the existing `Look()`.
