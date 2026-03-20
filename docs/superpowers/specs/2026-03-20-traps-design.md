# Traps — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Feature:** `traps` (priority 15)
**Dependencies:** `room-danger-levels`

---

## Overview

Traps are room hazards that trigger on specific conditions, dealing damage or applying conditions to players. They can be placed statically in room YAML or generated procedurally at runtime based on zone danger level probabilities. Traps follow PF2E detection and disarm rules. Effect severity scales with room danger level.

Six trap types are supported: Mine, Pit, Bear Trap, Trip Wire, Pressure Plate (trigger only), and Honkeypot (region-targeted crowd control).

---

## 1. Data Model

### Trap Template YAML

Trap templates are stored in `content/traps/` as individual YAML files, one per template. They are referenced by ID from room YAML and zone procedural configurations.

```yaml
id: bear_trap
name: Bear Trap
description: A rusted steel jaw trap hidden under debris.
trigger: entry                # entry | interaction | pressure_plate | region
payload:
  type: bear_trap             # mine | pit | bear_trap | trip_wire | honkeypot
  damage: 2d6
  condition: grabbed
  save_type: ""               # empty = no save (bear_trap has no save)
  save_dc: 0
stealth_dc: 16                # Perception DC to detect (Search mode only)
disable_dc: 20                # Thievery DC to disarm
reset_mode: auto              # one_shot | auto | manual
reset_timer: 10m              # in-game duration; only for auto reset mode
```

**Honkeypot** adds `target_regions` and `trigger_action`:

```yaml
id: honkeypot_charmer
name: Honkeypot
description: A lure disguised as something appealing to a specific group.
trigger: region
target_regions: [lake_oswego, pearl_district]
trigger_action: entry         # entry | interaction
payload:
  type: honkeypot
  technology_effect: charm
  save_type: will
  save_dc: 20
stealth_dc: 18
disable_dc: 22
reset_mode: auto
reset_timer: 1h
```

**Pressure Plate** declares `payload_template` pointing to any non-Pressure-Plate template:

```yaml
id: pressure_plate_mine
name: Pressure Plate
description: A concealed plate connected to an explosive charge.
trigger: pressure_plate
payload_template: mine        # MUST NOT reference another pressure_plate template
stealth_dc: 14
disable_dc: 18
reset_mode: one_shot
```

- REQ-TR-11: A Pressure Plate template's `payload_template` MUST NOT reference another Pressure Plate template. This MUST be validated as a fatal load error.
- REQ-TR-16: The `reset_mode` on a Pressure Plate template governs the instance lifecycle. The `reset_mode` on the linked `payload_template` MUST be ignored.

### Room YAML

Rooms gain a `traps` list (parallel to `spawns`):

```yaml
traps:
  - template: mine
    position: room            # room-level trap
  - template: honkeypot_charmer
    position: metal_cabinet   # item-level: references a RoomEquipmentConfig Description or ItemID
```

### RoomEquipmentConfig

`RoomEquipmentConfig` gains one new field (does not currently exist — must be added):

```go
// TrapTemplate is the trap template ID assigned to this equipment item.
// "" means no trap. Set by static YAML authoring or procedural generation.
TrapTemplate string `yaml:"trap_template"`
```

At trap trigger time for equipment-level traps, the implementation MUST look up the trap template via `RoomEquipmentManager.configs[roomID][instance.configIdx].TrapTemplate`. The `TrapTemplate` value is NOT propagated into `EquipmentInstance` — it is always read from the config by `configIdx`.

### Zone YAML

Zones may override default trap probabilities and define a weighted trap pool:

```yaml
trap_probabilities:
  room_trap_chance: 0.4       # omit to use danger level default
  cover_trap_chance: 0.6
  trap_pool:
    - template: mine
      weight: 3
    - template: bear_trap
      weight: 2
    - template: honkeypot_charmer
      weight: 1
```

### Global Default Trap Pool

`content/traps/defaults.yaml` defines the fallback trap pool used when a zone specifies no pool:

```yaml
default_pool:
  - template: mine
    weight: 3
  - template: bear_trap
    weight: 2
  - template: trip_wire
    weight: 2
  - template: pit
    weight: 1
```

---

## 2. Trap Runtime State

Trap runtime state is managed by a new `TrapManager` in `internal/game/trap/`.

### TrapInstanceID

Each trap instance is identified by a string key:

- Room-level trap: `zoneID + "/" + roomID + "/room/" + uuid`
- Equipment-level trap: `zoneID + "/" + roomID + "/equip/" + instanceID`

The `uuid` component for room-level traps is a UUID generated at placement time (zone load for static traps, procedural generation for runtime traps) and stored in `TrapInstanceState.InstanceID`. This ensures stable identity for both static and procedural room traps regardless of YAML array position.

Key construction MUST use a named helper function (`trapInstanceID(...)`) that asserts all components are non-empty.

### TrapInstanceState

```go
type TrapInstanceState struct {
    TemplateID    string        // trap template ID
    Armed         bool          // true if trap will fire on trigger
    BeingDisarmed bool          // true if a disarm attempt is in flight
    ResetAt       *time.Time    // non-nil for auto mode; when to re-arm
}
```

### TrapManager

```go
type TrapManager struct {
    mu       sync.RWMutex
    traps    map[string]*TrapInstanceState  // keyed by TrapInstanceID
    sessions map[string]map[string]bool     // playerID → set of detected TrapInstanceIDs
}
```

- Initialized at zone load time from static room YAML and procedural placement.
- `Armed` starts `true` for all instances at initialization.
- Detection state (`sessions`) is per-player and per-session; it is NOT persisted.
- REQ-TR-12: Detection state for a room MUST be cleared for all players when a room resets (new procedural placement cycle). Detection state is per-session and does not persist across player disconnects.

### Lifecycle

- Zone load: `TrapManager` is initialized with all static traps; procedural placement runs immediately after.
- On trigger: `Armed` set to `false`; one-shot instances removed; auto instances schedule `ResetAt`.
- On re-arm (auto): timer fires, `Armed` set to `true` — UNLESS `BeingDisarmed` is `true`, in which case the re-arm is rescheduled by one `reset_timer` interval.
- On manual reset: an NPC action sets `Armed` to `true` (NPC behavior defined in `npc-behaviors` feature).

---

## 3. Trigger System

Four declarable trigger types in trap template YAML, plus one implicit runtime hook:

| Trigger | YAML value | Fires when | Applies to |
|---------|-----------|-----------|------------|
| Entry | `entry` | Player enters the room | Room-level traps |
| Interaction | `interaction` | Player executes `use`/`interact` on the equipment item | Item-level traps |
| Pressure Plate | `pressure_plate` | Player executes any move action (Stride, Step, Tumble Through) during combat | Room-level traps |
| Region | `region` | Player whose home region matches `target_regions` performs `trigger_action` | Honkeypot only |
| Cover crossfire | *(not declarable)* | Attack misses and crosses cover; the equipment item has `TrapTemplate` set | Item-level traps only |

**Cover crossfire is NOT a declarable trigger type.** It is an implicit hook on the existing crossfire mechanic: after a cover object absorbs a miss, if `RoomEquipmentConfig.TrapTemplate != ""` for that equipment item and the trap is armed, the trap fires. No new `trigger:` enum value is added for this.

### Multiple Traps

If multiple armed traps in a room share the same trigger type (e.g., two entry traps), all matching armed traps fire simultaneously on the triggering event.

### Armed State

Each trap instance in `TrapManager` carries `Armed bool`:

- **One-shot:** on trigger, instance is permanently disarmed and removed from `TrapManager`.
- **Auto:** on trigger, `Armed = false`; `ResetAt` is set to `now + reset_timer`.
- **Manual:** on trigger, `Armed = false`; requires an NPC action to re-arm.

### Region Trigger (Honkeypot)

- REQ-TR-1: A player whose home region is NOT in `target_regions` MUST NOT trigger a Honkeypot.
- REQ-TR-2: A Honkeypot MUST NOT appear in Search detection rolls for a player whose home region is not in `target_regions`.

### Triggering a Known Trap

Detection does not grant immunity. A player who has detected a trap and moves through the room anyway triggers it normally.

---

## 4. Detection & Disarm

Following PF2E rules.

### Detection

- REQ-TR-3: Trap detection is available ONLY during Search exploration mode. This requirement applies to traps specifically; other hidden entities (secret exits, hidden items) may have their own detection rules outside Search mode.
- REQ-TR-4: On room entry while in Search mode, the game MUST roll a secret Perception check for the player against each armed trap's `stealth_dc` (modified by danger level scaling) in the room, including traps on equipment items.
- REQ-TR-5: On success, the trap MUST be flagged as detected in the player's `TrapManager.sessions` entry for this session, and a message MUST reveal the trap's name and location.
- REQ-TR-6: On failure, no message is shown. The trap remains hidden.
- REQ-TR-7: Honkeypots MUST be excluded from Search detection rolls for non-targeted players (per REQ-TR-2).

### Disarm

The `disarm <trap-name>` command is available only for traps flagged as detected in the current session.

**Argument resolution:** The player uses the trap name as shown at detection time. If multiple detected traps in the room share the same name, a location qualifier is appended (e.g., `bear trap near metal cabinet`). The `disarm` command matches against name and optional location qualifier.

When a disarm attempt begins, `BeingDisarmed` is set to `true` on the `TrapInstanceState`. It is cleared to `false` on completion regardless of outcome.

Thievery check vs `disable_dc` (modified by danger level scaling):

| Outcome | Result |
|---------|--------|
| Critical success | Identical to Success — trap disarmed; no additional benefit beyond Success |
| Success | Trap disarmed; one-shot traps removed from `TrapManager`; auto/manual left in place disarmed |
| Failure | No effect; `BeingDisarmed` cleared; player may retry |
| Failure by 5+ | Trap fires against the disarming player only (see targeting rule below) |

**Disarm-triggered targeting:** When a trap fires due to a failed disarm attempt, the payload MUST target only the disarming player regardless of the normal targeting rules. This means AoE traps (Mine) do NOT hit all room occupants on disarm failure — only the disarming player is affected.

- REQ-TR-13: A trap triggered by a failed disarm attempt MUST apply its payload to the disarming player only.

---

## 5. Trap Payloads & Danger Level Scaling

### Payload Effects

All saves use the PF2E four-tier success scale. Trap damage resolves immediately on trigger, outside the initiative order.

| Type | Damage | Condition | Save | Notes |
|------|--------|-----------|------|-------|
| Mine | 4d6 piercing+fire | — | Reflex vs `save_dc` | AoE — affects all combatants in room (except on disarm failure — see REQ-TR-13) |
| Pit | 2d6 fall | `immobilized` (1 round) | Reflex vs `save_dc` | Critical success: no damage, no condition |
| Bear Trap | 2d6 piercing | `grabbed` (until escaped) | None | No save; uses existing `grabbed` condition |
| Trip Wire | 1d6 slashing | `prone` | Reflex vs `save_dc` | Uses existing `prone` condition |
| Honkeypot | — | Technology payload (charm/confused/etc.) | Will vs `save_dc` | `technology_effect` references existing Technology system; no damage |
| Pressure Plate | (from `payload_template`) | (from `payload_template`) | (from `payload_template`) | Trigger only — all payload fields resolved from the linked template |

- REQ-TR-14: Bear Trap payload MUST apply the `grabbed` condition with no save.
- REQ-TR-15: Danger level `damage_bonus` MUST be silently ignored for payload types with no damage field (Honkeypot).

### Danger Level Scaling

Scaling bonuses are additive and applied at trigger/detection time using the room's `DangerLevel`. Scaling applies to damage, save DC, stealth DC, and disable DC. Safe rooms have no traps. Sketchy is the baseline.

**Global defaults:**

| Danger Level | Damage Bonus | Save DC Bonus | Stealth DC Bonus | Disable DC Bonus |
|-------------|-------------|--------------|-----------------|-----------------|
| Sketchy | +0 | +0 | +0 | +0 |
| Dangerous | +1d6 | +2 | +2 | +2 |
| All Out War | +2d6 | +4 | +4 | +4 |

**Per-template overrides** via `danger_scaling` block (any tier omitted falls back to global default):

```yaml
danger_scaling:
  dangerous:
    damage_bonus: 2d6
    save_dc_bonus: 3
    stealth_dc_bonus: 3
    disable_dc_bonus: 3
  all_out_war:
    damage_bonus: 4d6
    save_dc_bonus: 6
    stealth_dc_bonus: 5
    disable_dc_bonus: 5
```

- REQ-TR-8: Scaling MUST be applied at trigger/detection time from the room's runtime `DangerLevel`, not baked into the template at load time.

---

## 6. Procedural Generation

Traps are procedurally placed at zone initialization and on room reset. Static YAML traps are always present and are not affected by procedural generation.

### Default Probabilities by Danger Level

Per the `room-danger-levels` spec: Sketchy rooms do NOT contain room-level traps. They may have equipment/cover traps only.

| Danger Level | Room trap chance | Cover trap chance |
|-------------|-----------------|------------------|
| Safe | 0% | 0% |
| Sketchy | 0% | 15% |
| Dangerous | 35% | 50% |
| All Out War | 60% | 75% |

Zone YAML `trap_probabilities` overrides these defaults per zone.

### Placement Algorithm

For each room at initialization/reset:

1. Roll against `room_trap_chance` — on success, select one trap template from the applicable `trap_pool` using weighted random selection and place it as a room-level trap.
2. For each cover item (`CoverTier != ""`) in the room's equipment list, roll against `cover_trap_chance` — on success, assign a trap template to `RoomEquipmentConfig.TrapTemplate`.
3. Pool resolution order: zone `trap_pool` → `content/traps/defaults.yaml` global pool.

- REQ-TR-9: Procedural placement MUST NOT overwrite statically defined traps in room YAML.
- REQ-TR-10: Procedurally placed traps MUST use `one_shot` or `auto` reset mode only.

---

## 7. Out of Scope

- NPC re-arm behavior for `manual` reset traps → `npc-behaviors` feature
- Trap crafting and placement by players → `crafting` feature
- Specific trap content for new zones → `zones-new` feature

---

## Requirements Summary

- REQ-TR-1: Non-targeted players MUST NOT trigger a Honkeypot.
- REQ-TR-2: Honkeypots MUST NOT appear in Search detection rolls for non-targeted players.
- REQ-TR-3: Trap detection MUST only be available during Search exploration mode (traps only; other hidden entity detection rules are defined separately).
- REQ-TR-4: On room entry in Search mode, a secret Perception check MUST be rolled against each armed trap's scaled `stealth_dc`.
- REQ-TR-5: A successful detection check MUST flag the trap as detected and reveal its name and location.
- REQ-TR-6: A failed detection check MUST produce no message.
- REQ-TR-7: Honkeypots MUST be excluded from Search detection rolls for non-targeted players.
- REQ-TR-8: Danger level scaling MUST be applied at trigger/detection time from the room's runtime `DangerLevel`.
- REQ-TR-9: Procedural placement MUST NOT overwrite statically defined room traps.
- REQ-TR-10: Procedurally placed traps MUST use `one_shot` or `auto` reset mode only.
- REQ-TR-11: A Pressure Plate's `payload_template` MUST NOT reference another Pressure Plate; this MUST be a fatal load error.
- REQ-TR-12: Detection state for a room MUST be cleared for all players on room reset.
- REQ-TR-13: A trap triggered by a failed disarm attempt MUST apply its payload to the disarming player only.
- REQ-TR-14: Bear Trap payload MUST apply the `grabbed` condition with no save.
- REQ-TR-15: Danger level `damage_bonus` MUST be silently ignored for payload types with no damage field.
- REQ-TR-16: The `reset_mode` on a Pressure Plate template governs its instance lifecycle; the linked payload template's `reset_mode` MUST be ignored.
