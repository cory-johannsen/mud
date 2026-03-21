# Downtime — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `downtime` (priority 237)
**Dependencies:** `actions`, `persistent-calendar`, `crafting`, `focus-points`

---

## Overview

Players declare a downtime activity while in a Safe room. Their character enters a busy state — movement and combat blocked — while the in-game clock advances. After the activity's duration elapses (all activities complete within 10 real minutes / 10 in-game hours), a single skill check resolves the outcome and the player regains full control. Activity state persists across disconnects: the clock keeps ticking offline and the activity completes on reconnect if time has elapsed.

15 activities are defined, grouped by location requirement.

---

## 1. Data Model

`PlayerSession` gains:

```go
DowntimeActivityID  string    // active activity ID; empty = no active downtime
DowntimeCompletesAt time.Time // game-clock time when activity completes
DowntimeBusy        bool      // true while activity is active; blocks movement and combat
DowntimeMetadata    string    // JSON blob: recipe_id, target_name, item_id, etc. per activity
```

Persisted to a new `character_downtime` table:

```sql
character_downtime (
    character_id      bigint PRIMARY KEY REFERENCES characters(id),
    activity_id       text        NOT NULL,
    completes_at      timestamptz NOT NULL,
    room_id           text        NOT NULL,  -- room ID where activity was started; used for reconnect tag validation
    activity_metadata jsonb
)
```

On reconnect: load `character_downtime` row if present. Compare `completes_at` to current game-clock time. If elapsed, resolve activity immediately before returning control to the player. If time remains, restore `DowntimeBusy = true` with the remaining duration.

- REQ-DT-6: `DowntimeCompletesAt` MUST be persisted to DB and checked on player reconnect. If elapsed, the activity MUST complete before the player regains full control.

---

## 2. Busy State

While `DowntimeBusy` is true:

- **Blocked:** movement (direction commands), all combat commands, `explore`, `downtime <activity>` (cannot start a new activity)
- **Permitted:** `chat`, `say`, `look`, `inventory`, `character`, `downtime` (status), `downtime cancel`, non-combat skill checks, non-combat technology activation, non-combat feat activation

The busy state is enforced at the command dispatch layer in `grpc_service.go` — not via the condition system. It does not appear as a condition on the character sheet.

- REQ-DT-5: Movement and combat commands MUST be blocked while `DowntimeBusy` is true.
- REQ-DT-9: Non-combat skills, technologies, feats, and social commands MUST remain available while `DowntimeBusy` is true.

---

## 3. Commands

```
downtime                        — show active activity, time remaining, and busy status
downtime list                   — list all 15 activities with duration, skill, and location requirement
downtime <alias> [args]         — begin an activity (see alias table below)
downtime cancel                 — cancel active activity immediately; no material refund for Craft
```

### 3.1 Activity Aliases

| Command | Activity | Required args |
|---|---|---|
| `downtime earn` | Earn Creds | — |
| `downtime craft <recipe>` | Craft | recipe ID or name |
| `downtime retrain` | Retrain | — |
| `downtime sickness` | Fight the Sickness | — |
| `downtime subsist` | Subsist | — |
| `downtime forge` | Forge Papers | — |
| `downtime recalibrate` | Recalibrate | — |
| `downtime patchup` | Patch Up | — |
| `downtime flushit` | Flush It | — |
| `downtime intel <target>` | Run Intel | target person or place name |
| `downtime analyze <item>` | Analyze Tech | item name in inventory |
| `downtime repair <item>` | Field Repair | item name in inventory |
| `downtime decode` | Crack the Code | — |
| `downtime cover` | Run a Cover | — |
| `downtime pressure <npc>` | Apply Pressure | NPC name in room |

- REQ-DT-1: `downtime <activity>` MUST fail if the player is not in a Safe room.
- REQ-DT-2: `downtime <activity>` MUST fail if the activity's room tag requirement is not met.
- REQ-DT-3: `downtime <activity>` MUST fail if another activity is already active.
- REQ-DT-4: `downtime cancel` MUST NOT refund materials consumed by Craft.

---

## 4. Activities

### 4.1 Location Requirements

All activities require a Safe room as the base. Additional tag requirements:

| Tag required | Activities |
|---|---|
| None (any Safe room) | Earn Creds, Retrain, Subsist, Forge Papers, Recalibrate, Run Intel, Crack the Code, Run a Cover, Apply Pressure |
| `clinic` | Fight the Sickness, Patch Up, Flush It |
| `workshop` | Craft, Field Repair |
| `workshop` or `archive` | Analyze Tech |

Room tags are stored in `world.Room.Properties["tags"]` as a comma-separated list (e.g., `"clinic,safe"`). Tag validation occurs at activity start time.

- REQ-DT-8: Room tags MUST be validated at activity start time, not at command parse time.

### 4.2 Durations

All activities complete within 10 in-game hours (10 real minutes). The game clock runs at 1 real minute = 1 in-game hour.

| Activity | In-game duration | Real time |
|---|---|---|
| Recalibrate | 30 min | 30 sec |
| Patch Up | 1 hour | 1 min |
| Flush It | 1 hour | 1 min |
| Subsist | 2 hours | 2 min |
| Analyze Tech | 2 hours | 2 min |
| Field Repair | 2 hours | 2 min |
| Crack the Code | 3 hours | 3 min |
| Fight the Sickness | 4 hours | 4 min |
| Run Intel | 4 hours | 4 min |
| Apply Pressure | 4 hours | 4 min |
| Craft (complexity 2) | 4 hours | 4 min |
| Forge Papers | 6 hours | 6 min |
| Run a Cover | 6 hours | 6 min |
| Earn Creds | 6 hours | 6 min |
| Craft (complexity 3) | 6 hours | 6 min |
| Retrain | 8 hours | 8 min |
| Craft (complexity 4) | 8 hours | 8 min |

### 4.3 Resolution Outcomes

A single skill check fires at completion. All outcomes use the PF2E four-tier scale.

**Earn Creds** — skill: Rigging, Intel, or Rep (player's highest); DC: `settlement_dc` zone property (integer, default 15). Rep maps to the Performance skill in the skill system.

Each zone YAML MUST define `settlement_dc` (integer). If absent, default 15 applies.

| Outcome | Result |
|---|---|
| Critical Success | 3× base pay in credits |
| Success | 2× base pay in credits |
| Failure | Base pay in credits |
| Critical Failure | No pay |

**Craft** — skill: Rigging; DC: recipe DC. Materials deducted at `downtime craft <recipe>` confirm time via the crafting feature's `DeductMany` transaction. On completion, the downtime resolver calls the crafting feature's item delivery logic; produced items are added directly to the character's inventory. The `DowntimeCraftStarter` interface (crafting spec Section 5.5) owns this handoff.

| Outcome | Result |
|---|---|
| Critical Success | `output_count + 1` items added to character inventory |
| Success | `output_count` items added to character inventory |
| Failure | No items; half materials refunded (rounded down per material) to character inventory |
| Critical Failure | No items; no material refund |

**Fight the Sickness** — skill: Patch Job; DC: disease DC (from condition definition). Disease conditions MUST expose a `Severity int` field in their YAML definition (range 0–max_severity). "Removed entirely" sets Severity to 0 and removes the condition. Severity cannot go below 0 or above max_severity.

| Outcome | Result |
|---|---|
| Critical Success | Disease removed entirely (Severity → 0, condition removed) |
| Success | Disease Severity reduced by 2 (minimum 0) |
| Failure | Disease Severity reduced by 1 (minimum 0) |
| Critical Failure | Disease Severity increased by 1 (maximum max_severity) |

**Subsist** — skill: Scavenging or Factions (player's choice); DC: zone DC. "Ally" means any other connected player in the same room. The `fatigued` condition MUST exist in the condition registry; if absent, this activity will fail at resolution time. Temporary HP is deferred to the `advanced-health` feature; critical success instead grants a regular heal of 1 × level HP to self and one ally.

| Outcome | Result |
|---|---|
| Critical Success | Self + 1 ally covered; both healed for 1 × level HP |
| Success | Self covered; no condition penalty |
| Failure | Self covered; `fatigued` condition applied |
| Critical Failure | Not covered; `fatigued` condition applied |

**Forge Papers** — skill: Hustle; DC: 15

| Outcome | Result |
|---|---|
| Critical Success | Undetectable forgery document item produced |
| Success | Convincing forgery document item produced |
| Failure | Poor forgery produced (easily detected on inspection) |
| Critical Failure | No document produced; materials lost |

**Recalibrate** — no skill check

| Outcome | Result |
|---|---|
| Critical Success | All Focus Points restored |
| Success | All Focus Points restored |
| Failure | 1 Focus Point restored |
| Critical Failure | No Focus Points restored |

Recalibrate rolls 1d20 with no modifier to determine tier: 20 = critical success, 11–19 = success, 2–10 = failure, 1 = critical failure. Both critical success and success restore all Focus Points — the distinction between these tiers is reserved for future expansion. `FocusPoints` on `PlayerSession` is set to `MaxFocusPoints` and persisted immediately.

**Patch Up** — skill: Patch Job; DC: 15

| Outcome | Result |
|---|---|
| Critical Success | Heal 4 × level HP |
| Success | Heal 2 × level HP |
| Failure | Heal 1 × level HP |
| Critical Failure | No healing |

**Flush It** — skill: Patch Job; DC: poison/drug DC (from condition definition). Poison/drug conditions MUST expose a `Stage int` field in their YAML definition (range 0–max_stage). "Removed entirely" sets Stage to 0 and removes the condition. Stage cannot go below 0.

| Outcome | Result |
|---|---|
| Critical Success | Poison/drug removed entirely (Stage → 0, condition removed) |
| Success | Stage reduced by 2 (minimum 0) |
| Failure | Stage reduced by 1 (minimum 0) |
| Critical Failure | No effect |

**Run Intel** — skill: Smooth Talk; DC: target DC (set by GM/zone for named targets, 15 default). Facts are strings authored at content time in a target-specific lore entry keyed by target name or NPC ID. Each fact is a 1–2 sentence string. A "vague fact" is the first fact in the list with a prefix tag indicating low confidence. A "false lead" is a pre-authored alternate string in the same lore entry. On any success, facts are displayed as console messages.

| Outcome | Result |
|---|---|
| Critical Success | 3 facts about target revealed |
| Success | 2 facts about target revealed |
| Failure | 1 vague fact revealed |
| Critical Failure | False lead delivered |

**Analyze Tech** — skill: Tech Lore; DC: item DC (from item definition). Items MAY define a `hidden_properties []string` array in their definition. These are only revealed on critical success; on regular success, hidden properties remain unknown. If an item has no `hidden_properties`, critical success and success are equivalent.

| Outcome | Result |
|---|---|
| Critical Success | Full identification including hidden properties |
| Success | Full identification (hidden properties not revealed) |
| Failure | Partial identification (type and rough function only) |
| Critical Failure | Misidentification |

**Field Repair** — skill: Rigging; DC: item DC (from item definition)

| Outcome | Result |
|---|---|
| Critical Success | Full repair + 1 bonus durability |
| Success | Full repair |
| Failure | Partial repair (50% durability restored) |
| Critical Failure | No repair; 1 durability lost |

**Crack the Code** — skill: Intel or Tech Lore (player's choice); DC: document DC

| Outcome | Result |
|---|---|
| Critical Success | Document decoded + contextual lore fact |
| Success | Document decoded |
| Failure | Partial decode (gist only) |
| Critical Failure | Garbled output; document flagged as undecipherable for this attempt |

**Run a Cover** — skill: Hustle; DC: 15. The +1 circumstance bonus is transient: stored as `PlayerSession.ZoneCircumstanceBonus map[string]int` (keyed by zone ID + skill ID) and persists only for the current session. It is not written to the DB. If the player disconnects, the bonus is lost.

| Outcome | Result |
|---|---|
| Critical Success | Cover holds for 2 in-game days; +1 circumstance bonus to Hustle checks in this zone (current session only) |
| Success | Cover holds for 1 in-game day |
| Failure | Cover holds but fragile (first failed check blows it) |
| Critical Failure | Cover blown immediately |

**Apply Pressure** — skill: Hard Look; DC: NPC-specific DC (`10 + npc.Awareness`)

| Outcome | Result |
|---|---|
| Critical Success | NPC compliant for 2 in-game days |
| Success | NPC compliant for 1 in-game day |
| Failure | No effect |
| Critical Failure | NPC becomes hostile |

**Retrain** — no skill check; the retrain always succeeds. The chosen change (Feat, Skill rank, or Job feature) is applied immediately when the activity completes. A 1d20 roll with no modifier is made at activity start to determine duration: natural 20 = 6 hours, any other result = 8 hours. All three retrain types share the same duration:
- Skill rank change: 6 or 8 hours (per roll)
- Feat swap: 6 or 8 hours (per roll)
- Job feature swap: 6 or 8 hours (per roll)

---

## 5. Architecture

### 5.1 New Package

```
internal/game/downtime/
  activity.go    — Activity interface, 15 activity definitions (ID, duration, location req, skill)
  engine.go      — DowntimeEngine: Start(), Cancel(), CheckCompletion(sess, gameClock)
  resolver.go    — per-activity resolution logic (skill checks, four-tier outcomes, side effects)
```

### 5.2 Game Clock Integration

`GameClock` broadcasts a tick every real minute. `GameServiceServer` listens on the tick channel and calls `DowntimeEngine.CheckCompletion()` for every session where `DowntimeActivityID != ""` and `DowntimeCompletesAt` has passed.

- REQ-DT-7: The game clock tick handler MUST call `CheckCompletion()` for all sessions with an active downtime activity on every tick.

### 5.3 Storage

New repository: `CharacterDowntimeRepository` in `internal/storage/postgres/` with:

```go
Save(ctx context.Context, characterID int64, state DowntimeState) error
Load(ctx context.Context, characterID int64) (*DowntimeState, error) // nil if no active downtime
Clear(ctx context.Context, characterID int64) error
```

### 5.4 Room Tags

Room tags stored in `world.Room.Properties["tags"]` as a comma-separated list. `DowntimeEngine.Start()` validates tags before setting `DowntimeBusy`. The `workshop`, `clinic`, and `archive` tags are the canonical values defined by this spec; rooms in content YAML are updated to include appropriate tags.

A "Safe room" is defined as any room whose tags list contains `"safe"`. REQ-DT-1 requires the `safe` tag to be present; activity-specific tags (REQ-DT-2) are validated in addition to `safe`. Room tags are validated only at activity start time; tag changes during an active activity do not interrupt or cancel the activity.

### 5.5 Skill Check Engine

All downtime skill checks MUST use the standard skill check engine (pass skill ID and DC; the engine applies proficiency, ability modifiers, and any circumstance bonuses from `PlayerSession.ZoneCircumstanceBonus`). Downtime resolvers MUST NOT manually compute modifiers.

### 5.6 Command Pattern

`downtime` follows CMD-1 through CMD-7: `HandlerDowntime` constant, `BuiltinCommands()` entry, `DowntimeRequest { subcommand string, args string }` proto message in `ClientMessage` oneof, bridge handler, `handleDowntime` case in `grpc_service.go`. All subcommands (`list`, `cancel`, activity aliases) are dispatched within `handleDowntime` via argument parsing.

### 5.7 Crafting Spec Update

The `crafting` feature spec (`docs/superpowers/specs/2026-03-20-crafting-design.md`) Section 5.5 defines downtime craft duration in days (1/2/4). Per this spec's ≤10 real minute constraint, those values are superseded: complexity 2 = 4 hours, complexity 3 = 6 hours, complexity 4 = 8 hours.

---

## 6. Requirements Summary

- REQ-DT-1: `downtime <activity>` MUST fail if the player is not in a Safe room (a room whose tags include `"safe"`).
- REQ-DT-2: `downtime <activity>` MUST fail if the activity's room tag requirement is not met.
- REQ-DT-3: `downtime <activity>` MUST fail if another activity is already active.
- REQ-DT-4: `downtime cancel` MUST NOT refund materials consumed by Craft.
- REQ-DT-5: Movement and combat commands MUST be blocked while `DowntimeBusy` is true.
- REQ-DT-6: `DowntimeCompletesAt` MUST be persisted to DB and checked on player reconnect; elapsed activities MUST complete before the player regains full control.
- REQ-DT-7: The game clock tick handler MUST call `CheckCompletion()` for all sessions with an active downtime activity on every tick.
- REQ-DT-8: Room tags MUST be validated at activity start time, not at command parse time. Tag changes during an active activity MUST NOT cancel the activity.
- REQ-DT-9: Non-combat skills, technologies, feats, and social commands MUST remain available while `DowntimeBusy` is true.
- REQ-DT-10: All downtime skill checks MUST use the standard skill check engine; resolvers MUST NOT manually compute modifiers.
- REQ-DT-11: Disease conditions MUST expose a `Severity int` field and `MaxSeverity int` in their definition. Poison/drug conditions MUST expose a `Stage int` field and `MaxStage int` in their definition. These fields are required for Fight the Sickness and Flush It resolution.
- REQ-DT-12: Each zone YAML MUST define `settlement_dc int` (default 15). The Earn Creds activity MUST read this field for its DC.
- REQ-DT-13: The `fatigued` condition MUST exist in the condition registry before the Subsist activity can be used.
- REQ-DT-14: `PlayerSession.ZoneCircumstanceBonus map[string]int` MUST be added to support transient per-zone skill circumstance bonuses; not persisted to DB.
