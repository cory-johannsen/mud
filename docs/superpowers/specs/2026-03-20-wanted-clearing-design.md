# Wanted Level Clearing — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Feature:** `wanted-clearing` (priority 25)
**Dependencies:** `room-danger-levels`, `non-combat-npcs`

---

## Overview

Three active methods allow players to reduce their WantedLevel faster than passive daily decay: bribing a guard or fixer NPC, surrendering to a guard and serving detention, or completing a quest with a Wanted-reduction reward. A new `fixer` NPC type is introduced; its faction reputation services are deferred to the `factions` feature.

WantedLevel uses the five-tier enum defined in `room-danger-levels`: 0=None, 1=Flagged, 2=Burned, 3=Hunted, 4=D.O.S. All methods reduce WantedLevel by 1 per use. Passive decay (1 level per in-game day) is defined in `room-danger-levels` and is not modified here.

---

## 1. Fixer NPC Type

### Template Integration

`npc_type: "fixer"` is a new recognized value. The `Template` struct in `internal/game/npc/template.go` gains one additional field alongside the existing type-specific configs:

```go
Fixer *FixerConfig `yaml:"fixer,omitempty"`
```

`Template.Validate()` MUST be updated to recognize `"fixer"` and enforce that `Fixer` is non-nil when `NPCType == "fixer"`, consistent with REQ-NPC-2 in the `non-combat-npcs` spec. The fixer `crafter: {}` YAML requirement from REQ-NPC-2 applies analogously: an explicit `fixer: {}` block MUST be present in NPC YAML for fixers.

The personality default table in the `non-combat-npcs` spec is extended with:

| Type | Default combat response |
|---|---|
| fixer | flee |

### Config

```go
type FixerConfig struct {
    // BaseCosts maps WantedLevel (1–4) to base bribe credit cost at Sketchy danger level.
    // All four keys (1, 2, 3, 4) MUST be present and > 0.
    BaseCosts map[int]int `yaml:"base_costs"`

    // NPCVariance is a per-NPC multiplier applied on top of base × zone cost.
    // e.g. 1.2 = 20% markup above baseline. Must be > 0.
    NPCVariance float64 `yaml:"npc_variance"`

    // MaxWantedLevel is the highest WantedLevel this fixer will clear. Default: 4.
    // A fixer with MaxWantedLevel: 2 refuses to help players at Hunted or D.O.S.
    MaxWantedLevel int `yaml:"max_wanted_level"`

    // ClearRecordQuestID is the quest ID this fixer offers for record-clearing.
    // Empty until the `quests` feature is implemented.
    ClearRecordQuestID string `yaml:"clear_record_quest_id,omitempty"`
}
```

- REQ-WC-1: `FixerConfig.NPCVariance` MUST be greater than 0. A zero or negative value MUST be a fatal load error.
- REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4. Values outside this range MUST be a fatal load error.
- REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain entries for all keys 1–4 and all values MUST be greater than 0. Missing keys or non-positive values MUST be a fatal load error.
- REQ-WC-3: Fixers MUST default to `flee` on combat start. They MUST NOT be added to the initiative order.
- REQ-WC-4: The `change_rep` command MUST NOT be implemented in this feature; it is reserved for the `factions` feature.

### Named Rustbucket Ridge Fixer

One lore-appropriate fixer NPC in Rustbucket Ridge (name and room TBD during content authoring).

---

## 2. Bribe Mechanic

### Eligible NPCs

Two NPC types accept the `bribe [npc]` command:

- **Fixers** — always bribeable.
- **Guards** — bribeable only when `GuardConfig.Bribeable: true` (default: `false`) AND `GuardConfig.MaxBribeWantedLevel` is at or above the player's current WantedLevel.

`GuardConfig` gains two new fields:

```go
// Bribeable indicates this guard accepts bribes. Default: false.
Bribeable bool `yaml:"bribeable"`

// MaxBribeWantedLevel is the highest WantedLevel this guard will clear via bribe.
// Only meaningful when Bribeable is true. Default: 2 (Burned).
MaxBribeWantedLevel int `yaml:"max_bribe_wanted_level"`
```

When multiple bribeable NPCs are present in the same room, the player MUST specify the target: `bribe <npc-name>`. If only one bribeable NPC is present, `bribe` alone is sufficient.

### Cost Formula

```
cost = floor(BaseCosts[currentWantedLevel] × ZoneMultiplier × NPCVariance)
```

`BaseCosts` for guards is defined per-NPC in `GuardConfig` using the same map structure as `FixerConfig`. Guards with `Bribeable: true` MUST also define `BaseCosts` with all four keys.

**Zone multipliers** (global defaults; zone YAML MAY override):

| Danger Level | Multiplier |
|---|---|
| Safe | 0.8 |
| Sketchy | 1.0 |
| Dangerous | 1.5 |
| All Out War | 2.5 |

Safe-zone fixers and bribeable guards are intentionally cheaper (0.8×) — players in Safe zones have earned easier access to clearing services.

### Command Flow

1. Player types `bribe [npc-name]` in a room containing a bribeable NPC.
2. The game displays the calculated cost and prompts for confirmation.
3. Player types `bribe confirm` to proceed.
4. Credits are deducted; WantedLevel decrements by 1.

- REQ-WC-5: `bribe` MUST fail with a message if the player's WantedLevel is 0 (None).
- REQ-WC-6: `bribe` MUST fail with a message if the player has insufficient credits.
- REQ-WC-7: `bribe` MUST fail with a message if no bribeable NPC is present in the room.
- REQ-WC-8: `bribe` MUST fail with a message if the player's current WantedLevel exceeds the target NPC's `MaxWantedLevel` (fixer) or `MaxBribeWantedLevel` (guard).
- REQ-WC-9: The cost MUST be displayed and confirmed before credits are deducted. A two-step `bribe [npc]` → `bribe confirm` flow MUST be used.
- REQ-WC-9a: When multiple bribeable NPCs are present, `bribe` without a name argument MUST fail with a disambiguation message listing the bribeable NPCs in the room.

---

## 3. Surrender Mechanic

### Command

`surrender` — available only when at least one guard is present in the player's current room.

### Detained Condition

A new condition `detained` is added to `content/conditions/detained.yaml`:

```yaml
id: detained
name: Detained
description: You are restrained and cannot act.
prevents_movement: true
prevents_commands: true
prevents_targeting: true
```

The `detained` condition integrates with the existing condition system in `internal/game/condition/`. The `prevents_movement`, `prevents_commands`, and `prevents_targeting` flags require new boolean fields on the condition `Definition` struct. Existing conditions that use none of these flags are unaffected.

The detained player is displayed to all room occupants as a room description suffix:

```
<PlayerName> is detained here.
```

- REQ-WC-10: A detained player MUST NOT be able to move, use any command, or be targeted by attacks.
- REQ-WC-11: The detained player's presence MUST be visible to all players entering or already in the room.

### Detention Duration

Duration is determined by WantedLevel **at the time of surrender** and owned by the `PlayerSession` — not tied to any guard instance. Duration is stored as a `DetainedUntil time.Time` field on `PlayerSession` using game-clock time.

| WantedLevel | In-game duration | Real time |
|---|---|---|
| 1 (Flagged) | 30 in-game minutes | 30 real seconds |
| 2 (Burned) | 1 in-game hour | 1 real minute |
| 3 (Hunted) | 3 in-game hours | 3 real minutes |
| 4 (D.O.S.) | 8 in-game hours | 8 real minutes |

Time is measured against the game clock. Per `docs/architecture/overview.md`: 1 real minute = 1 in-game hour. A full in-game day is 24 real minutes.

On detention completion: WantedLevel decrements by 1; `detained` condition is removed; player resumes normal control.

**Guard killed during detention:** The detention timer runs on `PlayerSession` and is independent of any guard instance. If all guards leave or are killed, the timer continues. The player remains detained until the timer expires or another player releases them.

**Player disconnect while detained:** `DetainedUntil` is persisted to the database on the player session record. On reconnect, the game clock is compared against `DetainedUntil`. If time has elapsed, detention completes normally (WantedLevel decrements, `detained` removed). If time remains, the player reconnects still detained with the remaining duration. The timer continues advancing against the game clock whether the player is online or offline.

**Post-release WantedLevel at Hunted (3):** When D.O.S. detention completes and WantedLevel decrements to Hunted (3), any guard in the room will check WantedLevel per REQ-NPC-7 and re-initiate combat. This is by design — clearing D.O.S. via surrender does not grant safety; the player must escape the room before guards re-engage. A 5-second grace window MUST be applied after detention completion before guards re-evaluate WantedLevel, giving the player one opportunity to act.

- REQ-WC-12: `surrender` MUST fail with a message if no guard is present in the room.
- REQ-WC-13: `surrender` MUST fail with a message if the player's WantedLevel is 0 (None).

Note: `surrender` and `bribe` are blocked while the player is detained because `prevents_commands: true` on the `detained` condition prevents all command input. No additional guard is needed.
- REQ-WC-14: Detention duration MUST be evaluated against the in-game clock (1 real minute = 1 in-game hour per `docs/architecture/overview.md`), not wall-clock time.
- REQ-WC-14a: `DetainedUntil` MUST be persisted to the database and restored on player reconnect. Detention MUST continue advancing against the game clock while the player is offline.
- REQ-WC-14b: If detention expires while the player is offline, it MUST complete normally (WantedLevel decrements, `detained` removed) the next time the player connects.
- REQ-WC-14c: A 5-second grace window MUST be applied after detention completion before guards re-evaluate the player's WantedLevel.

### Release by Another Player

Any other player in the room may attempt `release <player>` to free a detained player.

- Skill check: Grift or Ghosting (player's choice) vs DC set by the room's **current** danger level at the time of the `release` attempt:

| Danger Level | Release DC |
|---|---|
| Safe | 12 |
| Sketchy | 16 |
| Dangerous | 20 |
| All Out War | 24 |

- **Success:** `detained` condition removed immediately. WantedLevel does NOT decrease — the player has escaped, not been cleared.
- **Failure:** No effect. The player may retry immediately.

- REQ-WC-15: A successful `release` MUST remove the `detained` condition but MUST NOT modify the detained player's WantedLevel.
- REQ-WC-16: `release` MUST be available to any player in the room, not just allies.
- REQ-WC-16a: The release skill check DC MUST use the room's danger level at the time of the `release` attempt.

---

## 4. Quest-Based Clearing

### Path A — Quest Reward Flag

Quest definitions gain a `wanted_reduction: N` field (integer). On quest completion, the completing player's WantedLevel decrements by N (minimum 0; cannot go below None).

This field is defined here for schema completeness. Full quest wiring is deferred to the `quests` feature (priority 380).

- REQ-WC-17: Quest completion with `wanted_reduction: N` MUST decrement the player's WantedLevel by N, clamped to a minimum of 0.

### Path B — Fixer Record-Clearing Quest

Fixers are valid quest-givers for record-clearing quests. The `talk <fixer>` command (per the `non-combat-npcs` spec) presents available quests including any record-clearing quest when `FixerConfig.ClearRecordQuestID` is non-empty. Full wiring is deferred to the `quests` feature; `ClearRecordQuestID` is empty in all fixer YAML until then.

---

## 5. Out of Scope

- Fixer faction reputation services (`change_rep`) → `factions` feature
- Full quest wiring for quest-based clearing → `quests` feature
- Per-zone fixer content → `non-combat-npcs-all-zones` feature
- Guard patrol and manual re-arm → `npc-behaviors` feature

---

## Requirements Summary

- REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; zero or negative MUST be a fatal load error.
- REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; out-of-range MUST be a fatal load error.
- REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain all keys 1–4 with positive values; violations MUST be fatal load errors.
- REQ-WC-2b: `GuardConfig.MaxBribeWantedLevel` MUST be in range 1–4 when `Bribeable` is true. Out-of-range values MUST be a fatal load error.
- REQ-WC-3: Fixers MUST default to `flee` on combat start and MUST NOT enter the initiative order.
- REQ-WC-4: The `change_rep` command MUST NOT be implemented in this feature; reserved for `factions`.
- REQ-WC-5: `bribe` MUST fail if the player's WantedLevel is 0.
- REQ-WC-6: `bribe` MUST fail if the player has insufficient credits.
- REQ-WC-7: `bribe` MUST fail if no bribeable NPC is present in the room.
- REQ-WC-8: `bribe` MUST fail if the player's WantedLevel exceeds the target NPC's bribe level cap.
- REQ-WC-9: `bribe` cost MUST be displayed and confirmed via a two-step flow before credits are deducted.
- REQ-WC-9a: `bribe` without a name when multiple bribeable NPCs are present MUST fail with a disambiguation message.
- REQ-WC-10: A detained player MUST NOT be able to move, use commands, or be targeted by attacks.
- REQ-WC-11: A detained player's presence MUST be visible to all players in the room.
- REQ-WC-12: `surrender` MUST fail if no guard is present in the room.
- REQ-WC-13: `surrender` MUST fail if the player's WantedLevel is 0.
- REQ-WC-14: Detention duration MUST be evaluated against the in-game clock (1 real minute = 1 in-game hour).
- REQ-WC-14a: `DetainedUntil` MUST be persisted and restored on reconnect; detention advances while offline.
- REQ-WC-14b: Detention that expires offline MUST complete normally on next player connect.
- REQ-WC-14c: A 5-second grace window MUST apply after detention completion before guards re-evaluate WantedLevel.
- REQ-WC-15: A successful `release` MUST remove `detained` but MUST NOT modify WantedLevel.
- REQ-WC-16: `release` MUST be available to any player in the room.
- REQ-WC-16a: The release skill check DC MUST use the room's danger level at the time of the `release` attempt.
- REQ-WC-17: Quest completion with `wanted_reduction: N` MUST decrement WantedLevel by N, clamped to 0.
