# Wanted Level Clearing — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `wanted-clearing` (priority 25)
**Dependencies:** `room-danger-levels`, `non-combat-npcs`

---

## Overview

Three active methods allow players to reduce their WantedLevel faster than passive daily decay: bribing a guard or fixer NPC, surrendering to a guard and serving detention, or completing a quest with a Wanted-reduction reward. A new `fixer` NPC type is introduced; its faction reputation services are deferred to the `factions` feature.

WantedLevel uses the five-tier enum defined in `room-danger-levels`: 0=None, 1=Flagged, 2=Burned, 3=Hunted, 4=D.O.S. All methods reduce WantedLevel by 1 per use. Passive decay (1 level per in-game day) is defined in `room-danger-levels` and is not modified here.

---

## 1. Fixer NPC Type

### Config

A new `npc_type: "fixer"` is added to the NPC Template, parallel to the existing non-combat types.

```go
type FixerConfig struct {
    // BaseCosts maps WantedLevel (1–4) to base bribe credit cost at Sketchy danger level.
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
- REQ-WC-3: Fixers MUST default to `flee` on combat start. They MUST NOT be added to the initiative order.
- REQ-WC-4: Fixer faction reputation services (change_rep command) MUST be deferred to the `factions` feature.

### Named Rustbucket Ridge Fixer

One lore-appropriate fixer NPC in Rustbucket Ridge (name and room TBD during content authoring).

---

## 2. Bribe Mechanic

### Eligible NPCs

Two NPC types accept the `bribe` command:

- **Fixers** — always bribeable.
- **Guards** — bribeable only when `GuardConfig.Bribeable: true`. Default: `false`.

`GuardConfig` gains one new field:

```go
// Bribeable indicates this guard accepts bribes. Default: false.
// Most guards are not bribeable; only specifically authored guards have this flag.
Bribeable bool `yaml:"bribeable"`
```

### Cost Formula

```
cost = floor(BaseCosts[currentWantedLevel] × ZoneMultiplier × NPCVariance)
```

**Zone multipliers** (global defaults; zone YAML MAY override per the `room-danger-levels` spec):

| Danger Level | Multiplier |
|---|---|
| Safe | 0.8 |
| Sketchy | 1.0 |
| Dangerous | 1.5 |
| All Out War | 2.5 |

### Command Flow

1. Player types `bribe` in a room containing a bribeable guard or fixer.
2. The game displays the calculated cost and prompts for confirmation.
3. Player types `bribe confirm` to proceed.
4. Credits are deducted; WantedLevel decrements by 1.

- REQ-WC-5: `bribe` MUST fail with a message if the player's WantedLevel is 0 (None).
- REQ-WC-6: `bribe` MUST fail with a message if the player has insufficient credits.
- REQ-WC-7: `bribe` MUST fail with a message if no bribeable NPC is present in the room.
- REQ-WC-8: `bribe` MUST fail with a message if the player's current WantedLevel exceeds the NPC's `MaxWantedLevel`.
- REQ-WC-9: The cost MUST be displayed and confirmed before credits are deducted. A two-step `bribe` → `bribe confirm` flow MUST be used.

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

The `detained` condition is displayed to all room occupants as a room description suffix:

```
<PlayerName> is detained here.
```

- REQ-WC-10: A detained player MUST NOT be able to move, use any command, or be targeted by attacks.
- REQ-WC-11: The detained player's presence MUST be visible to all players entering or already in the room.

### Detention Duration

Duration scales with WantedLevel at the time of surrender:

| WantedLevel | In-game duration | Real time |
|---|---|---|
| 1 (Flagged) | 30 in-game minutes | 30 real seconds |
| 2 (Burned) | 1 in-game hour | 1 real minute |
| 3 (Hunted) | 3 in-game hours | 3 real minutes |
| 4 (D.O.S.) | 8 in-game hours | 8 real minutes |

Time is measured against the game clock (1 real minute = 1 in-game hour per the system time ratio documented in `docs/architecture/overview.md`).

On detention completion: WantedLevel decrements by 1; `detained` condition is removed; player resumes normal control.

- REQ-WC-12: `surrender` MUST fail with a message if no guard is present in the room.
- REQ-WC-13: `surrender` MUST fail with a message if the player's WantedLevel is 0 (None).
- REQ-WC-14: Detention duration MUST be evaluated against the in-game clock, not wall-clock time.

### Release by Another Player

Any other player in the room may attempt `release <player>` to free a detained player.

- Skill check: Grift or Ghosting (player's choice) vs DC set by room's danger level:

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

---

## 4. Quest-Based Clearing

### Path A — Quest Reward Flag

Quest definitions gain a `wanted_reduction: N` field (integer). On quest completion, the completing player's WantedLevel decrements by N (minimum 0; cannot go below None).

This field is defined here for schema completeness. Full quest wiring is deferred to the `quests` feature (priority 380).

- REQ-WC-17: Quest completion with `wanted_reduction: N` MUST decrement the player's WantedLevel by N, clamped to a minimum of 0.

### Path B — Fixer Record-Clearing Quest

Fixers are valid quest-givers for record-clearing quests. The `talk <fixer>` command (per the non-combat-npcs spec) presents available quests including any record-clearing quest when `FixerConfig.ClearRecordQuestID` is non-empty.

Full quest wiring is deferred to the `quests` feature. Until then, `ClearRecordQuestID` is empty in all fixer YAML.

- REQ-WC-18: When `FixerConfig.ClearRecordQuestID` is non-empty and the `quests` feature is implemented, `talk <fixer>` MUST offer the record-clearing quest.

---

## 5. Out of Scope

- Fixer faction reputation services (`change_rep`) → `factions` feature
- Full quest wiring for quest-based clearing → `quests` feature
- Per-zone fixer content (one per zone) → `non-combat-npcs-all-zones` feature
- Guard patrol and manual re-arm of traps → `npc-behaviors` feature

---

## Requirements Summary

- REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; zero or negative MUST be a fatal load error.
- REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; out-of-range MUST be a fatal load error.
- REQ-WC-3: Fixers MUST default to `flee` on combat start and MUST NOT enter the initiative order.
- REQ-WC-4: Fixer faction reputation services MUST be deferred to the `factions` feature.
- REQ-WC-5: `bribe` MUST fail if the player's WantedLevel is 0.
- REQ-WC-6: `bribe` MUST fail if the player has insufficient credits.
- REQ-WC-7: `bribe` MUST fail if no bribeable NPC is present in the room.
- REQ-WC-8: `bribe` MUST fail if the player's WantedLevel exceeds the NPC's `MaxWantedLevel`.
- REQ-WC-9: `bribe` cost MUST be displayed and confirmed via a two-step flow before credits are deducted.
- REQ-WC-10: A detained player MUST NOT be able to move, use commands, or be targeted by attacks.
- REQ-WC-11: A detained player's presence MUST be visible to all players in the room.
- REQ-WC-12: `surrender` MUST fail if no guard is present in the room.
- REQ-WC-13: `surrender` MUST fail if the player's WantedLevel is 0.
- REQ-WC-14: Detention duration MUST be evaluated against the in-game clock.
- REQ-WC-15: A successful `release` MUST remove `detained` but MUST NOT modify WantedLevel.
- REQ-WC-16: `release` MUST be available to any player in the room.
- REQ-WC-17: Quest completion with `wanted_reduction: N` MUST decrement WantedLevel by N, clamped to 0.
- REQ-WC-18: When `ClearRecordQuestID` is non-empty, `talk <fixer>` MUST offer the record-clearing quest.
