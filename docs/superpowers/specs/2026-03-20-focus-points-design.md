# Focus Points — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `focus-points` (priority 233)
**Dependencies:** `actions`

---

## Overview

A Focus Point pool per character powers focus technologies (technologies with `activation_cost: focus`). Players spend Focus Points to activate these technologies and restore them via the Recalibrate downtime activity. Max pool size is derived from class features and feats at login, capped at 3.

---

## 1. Data Model

### 1.1 DB Schema

`characters` table gains:

```sql
focus_points int NOT NULL DEFAULT 0
```

`MaxFocusPoints` is a derived value and is NOT persisted to the DB — it is recomputed at login and on every feat swap or level-up.

### 1.2 PlayerSession Fields

`PlayerSession` gains:

```go
FocusPoints    int  // current pool; loaded from DB at login; persisted on spend/restore
MaxFocusPoints int  // derived at login from active feats + class features; not persisted
```

### 1.3 MaxFocusPoints Computation

At login (and on feat swap or level-up), `MaxFocusPoints` is computed by counting all active class features and feats where `grants_focus_point: true`, capped at 3. Characters with no focus-granting features start with `MaxFocusPoints = 0` and cannot spend Focus Points.

- REQ-FP-1: `MaxFocusPoints` MUST be computed at login from all active class features and feats with `grants_focus_point: true`, capped at 3.
- REQ-FP-2: `MaxFocusPoints` MUST be recomputed after any feat swap or level-up before the player's next action.

### 1.4 Technology YAML

`TechnologyDef` in `internal/game/technology/model.go` gains a new boolean field:

```go
FocusCost bool `yaml:"focus_cost,omitempty"`
```

Technology YAML files use:

```yaml
focus_cost: true
```

`FocusCost: true` marks a technology as a focus technology; it may coexist with `action_cost` (which retains its existing `int` type for AP cost). A technology may have both an AP cost and a focus cost.

- REQ-FP-3: Technology definitions with `focus_cost: true` MUST require 1 Focus Point to activate, in addition to any AP cost.

### 1.5 Class Feature and Feat YAML

`ClassFeature` in `internal/game/ruleset/class_feature.go` gains:

```go
GrantsFocusPoint bool `yaml:"grants_focus_point,omitempty"`
```

Feat definitions (wherever feats are structured in the codebase) gain the same field:

```go
GrantsFocusPoint bool `yaml:"grants_focus_point,omitempty"`
```

Both fields default to `false` if absent. No validation error for absence — `grants_focus_point: true` is optional on any feature or feat.

---

## 2. Spending Focus Points

When a player activates a technology with `FocusCost: true`:

1. Check `FocusPoints > 0`. If not: fail with message "Not enough Focus Points. (N/M)" where N = current, M = max.
2. Decrement `FocusPoints` by 1.
3. Persist `focus_points` to the `characters` DB row immediately (within the activation handler, before sending the activation result to the client).
4. Proceed with technology activation.

- REQ-FP-4: Focus technology activation MUST fail with "Not enough Focus Points. (N/M)" if `FocusPoints == 0`.
- REQ-FP-5: `FocusPoints` MUST be decremented and persisted immediately on activation, before the activation result is sent to the client.
- REQ-FP-6: Each focus technology activation costs exactly 1 Focus Point regardless of technology level or complexity.

---

## 3. Restoring Focus Points

### 3.1 Recalibrate (Downtime)

The Recalibrate downtime activity restores Focus Points. Recalibrate has no skill check — it rolls 1d20 with no modifier to determine the tier (per downtime spec: 20 = critical success, 11–19 = success, 2–10 = failure, 1 = critical failure). The downtime resolver calls into the Focus Points system after the roll:

- Critical Success / Success: `FocusPoints = MaxFocusPoints`
- Failure: `FocusPoints = min(FocusPoints + 1, MaxFocusPoints)`
- Critical Failure: no change

- REQ-FP-7: The Recalibrate downtime resolver MUST set `FocusPoints = MaxFocusPoints` on critical success or success and persist immediately.
- REQ-FP-8: The Recalibrate downtime resolver MUST increment `FocusPoints` by 1 (capped at `MaxFocusPoints`) on failure and persist immediately. The 1d20 roll mechanic is owned by the downtime engine; the Focus Points system only handles the side effect.

### 3.2 Long Rest

Full Focus Point restoration on long rest is deferred to the `resting` feature.

---

## 4. Display

### 4.1 Prompt

If `MaxFocusPoints > 0`, the prompt includes `FP: N/M` alongside HP. If `MaxFocusPoints == 0`, the FP section is omitted entirely.

- REQ-FP-9: The player prompt MUST display `FP: N/M` if `MaxFocusPoints > 0`. If `MaxFocusPoints == 0`, Focus Points MUST NOT appear in the prompt.

### 4.2 Character Sheet

The character sheet includes a Focus Points row showing current and max. If `MaxFocusPoints == 0`, the row is omitted.

- REQ-FP-10: The character sheet MUST display Focus Points if `MaxFocusPoints > 0`. If `MaxFocusPoints == 0`, the row MUST be omitted.

---

## 5. Architecture

### 5.1 Technology Activation Handler

In the technology activation handler (existing code path for `use`/`activate`):

1. Check `technology.FocusCost == true`.
2. If so, apply Focus Point spend logic (REQ-FP-4, REQ-FP-5) before proceeding with AP cost deduction and effect resolution.

No new handler needed — this is an extension of the existing technology activation flow.

### 5.2 Rendering and Proto

The frontend receives game state via gRPC proto messages — it does not have access to `PlayerSession` directly. Focus Point data must be carried in proto messages.

**Proto changes (`gamev1` package):**
- `CharacterSheetView` message gains `int32 focus_points` and `int32 max_focus_points` fields.
- `HpUpdateEvent` (or an equivalent lightweight stat update event) gains `int32 focus_points` and `int32 max_focus_points` so the prompt can be updated after FP spend without a full character sheet refresh.

**Character sheet (`internal/frontend/text_renderer.go`, `RenderCharacterSheet`):** Add a "Focus Points" row in the stats section, formatted as `Focus Points: N/M`. Placement: after the HP row. Omit the row entirely if `max_focus_points == 0`. `CharacterSheetView` carries both values.

**Prompt (frontend prompt builder):** If `max_focus_points > 0` in the most recent stat event, append `FP: N/M` to the prompt stats line. If `max_focus_points == 0`, omit.

**Data flow:** `PlayerSession.FocusPoints` / `MaxFocusPoints` (gameserver in-memory) → serialized into proto messages → streamed to frontend → rendered in prompt and character sheet.

### 5.3 Character Repository

`CharacterRepository.Save()` (or equivalent update method) persists `focus_points`. If a targeted `SaveFocusPoints(ctx, characterID, focusPoints int) error` method does not exist, it MUST be added to avoid full-character writes on every FP spend.

### 5.4 Login Flow

At login, after loading the character from DB:
1. Load `FocusPoints` from `characters.focus_points`.
2. Compute `MaxFocusPoints` from active class features + feats.
3. Clamp: if `FocusPoints > MaxFocusPoints`, set `FocusPoints = MaxFocusPoints` (handles cases where max decreased since last login).

- REQ-FP-11: On login, `FocusPoints` MUST be clamped to `MaxFocusPoints` if it exceeds it.

---

## 6. Requirements Summary

- REQ-FP-1: `MaxFocusPoints` MUST be computed at login from all active class features and feats with `grants_focus_point: true`, capped at 3.
- REQ-FP-2: `MaxFocusPoints` MUST be recomputed after any feat swap or level-up before the player's next action.
- REQ-FP-3: Technology definitions with `focus_cost: true` (`TechnologyDef.FocusCost == true`) MUST require 1 Focus Point to activate.
- REQ-FP-4: Focus technology activation MUST fail with "Not enough Focus Points. (N/M)" if `FocusPoints == 0`.
- REQ-FP-5: `FocusPoints` MUST be decremented and persisted immediately on activation, before the activation result is sent to the client.
- REQ-FP-6: Each focus technology activation costs exactly 1 Focus Point regardless of technology level or complexity.
- REQ-FP-7: Recalibrate success/critical success MUST set `FocusPoints = MaxFocusPoints` and persist immediately.
- REQ-FP-8: Recalibrate failure MUST increment `FocusPoints` by 1 (capped at `MaxFocusPoints`) and persist immediately.
- REQ-FP-9: The prompt MUST display `FP: N/M` if `MaxFocusPoints > 0`; omit if 0.
- REQ-FP-10: The character sheet MUST display Focus Points if `MaxFocusPoints > 0`; omit if 0.
- REQ-FP-11: On login, `FocusPoints` MUST be clamped to `MaxFocusPoints` if it exceeds it.
- REQ-FP-12: `TechnologyDef.Validate()` MUST return an error if both `FocusCost == true` and `Passive == true`.
- REQ-FP-13: `CharacterSheetView` proto message MUST include `int32 focus_points` and `int32 max_focus_points` fields.
- REQ-FP-14: The stat update event sent after FP spend MUST include current `focus_points` and `max_focus_points` so the frontend prompt updates without a full character sheet refresh.
