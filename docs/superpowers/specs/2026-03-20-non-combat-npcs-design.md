# Non-Combat NPCs — Design Spec

**Date:** 2026-03-20
**Status:** Approved
**Feature:** `non-combat-npcs` (priority 20)
**Dependencies:** `room-danger-levels`, `wanted-clearing` (partial)

---

## Overview

Non-combat NPCs are NPC types that do not participate in the combat initiative order. They provide economic, social, and support services to players. Eight non-combat types are defined: merchant, guard, healer, quest giver, hireling, banker, job trainer, and crafter. Each type has a type-specific config sub-struct on the shared NPC Template. All non-combat NPCs have a personality field that drives an HTN-based flee/cower response when combat starts in their room.

The `crafter` type is fully stubbed here — detailed behavior is defined in the `crafting` feature spec.

---

## 1. Base Data Model

### Template Additions

Three new fields are added to the existing `Template` struct in `internal/game/npc/template.go`:

```go
// NPCType classifies the NPC's role.
// One of: "combat", "merchant", "guard", "healer", "quest_giver",
// "hireling", "banker", "job_trainer", "crafter"
// Defaults to "combat" if absent, preserving all existing NPC behavior.
NPCType string `yaml:"npc_type"`

// Personality names the HTN plan preset governing non-combat behavior.
// e.g., "cowardly", "brave", "neutral", "opportunistic"
// Defaults to the NPC type's default if absent (see table below).
Personality string `yaml:"personality"`

// Type-specific config — at most one will be non-nil for a given NPC.
Merchant   *MerchantConfig   `yaml:"merchant,omitempty"`
Guard      *GuardConfig      `yaml:"guard,omitempty"`
Healer     *HealerConfig     `yaml:"healer,omitempty"`
QuestGiver *QuestGiverConfig `yaml:"quest_giver,omitempty"`
Hireling   *HirelingConfig   `yaml:"hireling,omitempty"`
Banker     *BankerConfig     `yaml:"banker,omitempty"`
JobTrainer *JobTrainerConfig `yaml:"job_trainer,omitempty"`
Crafter    *CrafterConfig    `yaml:"crafter,omitempty"`
```

- REQ-NPC-1: Existing NPCs with no `npc_type` field MUST default to `"combat"` at load time. No existing YAML requires changes.
- REQ-NPC-2: At load time, the type-specific config sub-struct corresponding to `npc_type` MUST be present and non-nil. A mismatch MUST be a fatal load error. `Template.Validate()` in `internal/game/npc/template.go` MUST be updated to enforce this check. For `npc_type: "crafter"`, an explicit `crafter: {}` block MUST be present in the YAML; the loader MUST NOT auto-populate an empty `CrafterConfig`.
- REQ-NPC-2a: `Template.Validate()` MUST verify that any skill IDs referenced in NPC config (e.g., `smooth_talk`, `grift` used in negotiate) exist in the skill registry at load time. An unknown skill ID MUST be a fatal load error.

### Combat Exclusion

- REQ-NPC-3: Non-combat NPCs (`npc_type != "combat"`, with the explicit exception of guards when they are actively engaging per Section 3 behavior) MUST NOT be added to the combat initiative order. A guard enters the initiative order when their Section 3 behavior table evaluates to "engage" for the player's WantedLevel, or when REQ-NPC-6 triggers.
- REQ-NPC-4: Non-combat NPCs MUST NOT be valid targets for the `strike` command or any attack action, with the exception of guards when they are actively engaging per Section 3 behavior.

### Flee/Cower on Combat Start

When combat starts in a non-combat NPC's room, the HTN planner evaluates the NPC's personality plan, resolving to one of two behaviors:

- **Flee:** NPC moves to a randomly selected adjacent room that is not an All Out War room. If no valid exit exists, falls back to cower.
- **Cower:** NPC remains in the room, becomes non-interactive for the combat duration, and resumes normal behavior when combat ends.

Default behavior by type when `personality` is unset:

| Type | Default combat response |
|------|------------------------|
| merchant | cower |
| guard | engage (see Section 3) |
| healer | flee |
| quest_giver | cower |
| hireling | combat participant (see Section 6) |
| banker | flee |
| job_trainer | cower |
| crafter | flee |

---

## 2. Merchant

### Config

```go
type MerchantConfig struct {
    MerchantType   string          `yaml:"merchant_type"` // weapons | armor | rings_neck | consumables | maps | technology | drugs
    Inventory      []MerchantItem  `yaml:"inventory"`
    SellMargin     float64         `yaml:"sell_margin"`   // markup multiplier on base price for player purchases
    BuyMargin      float64         `yaml:"buy_margin"`    // fraction of base price paid to player on sale
    Budget         int             `yaml:"budget"`        // max credits available to buy from players
    ReplenishRate  ReplenishConfig `yaml:"replenish_rate"`
}

type MerchantItem struct {
    ItemID    string `yaml:"item_id"`
    BasePrice int    `yaml:"base_price"`
    InitStock int    `yaml:"init_stock"` // initial quantity loaded at zone initialization
    MaxStock  int    `yaml:"max_stock"`  // cap for replenishment
}

type ReplenishConfig struct {
    MinHours     int `yaml:"min_hours"`     // minimum in-game hours between replenishment cycles
    MaxHours     int `yaml:"max_hours"`     // actual interval = MinHours + rand(0, MaxHours-MinHours); capped at 24h
    StockRefill  int `yaml:"stock_refill"`  // units added per cycle per item; 0 = full reset to MaxStock
    BudgetRefill int `yaml:"budget_refill"` // credits added to Budget per cycle; 0 = full reset
}
```

**Runtime state:** `MerchantItem.InitStock` and `MerchantConfig.Budget` are YAML-defined initial values. Runtime stock quantities and current budget are tracked in a `MerchantRuntimeState` struct on the NPC instance and persisted to the database. On server restart, runtime state is loaded from the database — YAML initial values are only applied at first zone initialization, not on restart.

```go
type MerchantRuntimeState struct {
    Stock          map[string]int // itemID → current quantity
    CurrentBudget  int
    NextReplenishAt time.Time
}
```

- REQ-NPC-12: Merchant runtime state (stock quantities and current budget) MUST be persisted to the database and restored on server restart. YAML initial values MUST only be applied at first zone initialization.

**Replenishment:** Triggered by the in-game calendar tick subsystem (the same subsystem that fires daily NPC events). On each tick, the system compares `MerchantRuntimeState.NextReplenishAt` against the current in-game time; if elapsed, stock and budget are updated and `NextReplenishAt` is advanced by the next calculated interval. Dangerous and All Out War rooms replenish more frequently (higher turnover) than Safe rooms. The interval is never shorter than `MinHours` and never longer than `MaxHours`, capped at 24 in-game hours. `NextReplenishAt` MUST be included in `MerchantRuntimeState` and persisted to the database so the schedule survives server restarts.

- REQ-NPC-13: `ReplenishConfig` MUST satisfy `0 < MinHours <= MaxHours <= 24` at load time. Violation MUST be a fatal load error.

### Commands

- `browse` — lists merchant inventory with current stock and prices, adjusted by any active negotiate modifier
- `buy <item> [qty]` — deducts credits from player, decrements stock; fails if insufficient credits or stock is zero
- `sell <item> [qty]` — pays player `floor(buy_margin × base_price × qty)`; decrements merchant budget; fails if budget is exhausted
- `negotiate` — `smooth_talk` or `grift` skill (both defined in `content/skills.yaml`) vs merchant Perception DC; applies a session-scoped price modifier stored in the player's room session state and cleared on room exit. The modifier stacks multiplicatively with any WantedLevel-based surcharge (WantedLevel 1 surcharge from `room-danger-levels` is applied first, then the negotiate modifier on top):

| Outcome | Price modifier |
|---------|---------------|
| Critical success | ±20% (buy cheaper / sell higher) |
| Success | ±10% |
| Failure | No effect |
| Critical failure | +10% penalty on purchases only; sells are unaffected |

- REQ-NPC-5: `negotiate` MUST only be usable once per merchant room visit. Repeated attempts in the same room visit MUST be rejected with a message.
- REQ-NPC-5a: The negotiate price modifier MUST be stored on the player's room session state (not on the NPC instance) and MUST be cleared when the player transitions to a different room. On disconnect mid-visit, the modifier is discarded.
- REQ-NPC-5b: When a player with WantedLevel 1 executes `buy` or `sell`, the 10% WantedLevel surcharge (per `room-danger-levels`) MUST be applied first, then the negotiate modifier multiplied on top. The WantedLevel surcharge MUST NOT be applied to the negotiate roll itself.

### Named Rustbucket Ridge Merchants

| NPC | Merchant type | Location |
|-----|--------------|----------|
| Sergeant Mack | weapons | Last Stand Lodge |
| Slick Sally | consumables | Rusty Oasis |
| Whiskey Joe | consumables | The Bottle Shack |
| Old Rusty | consumables | The Heap |
| Herb | consumables | The Green Hell |

---

## 3. Guard

### Config

```go
type GuardConfig struct {
    // WantedThreshold is the minimum WantedLevel that triggers guard aggression.
    // Default: 2 (Burned).
    WantedThreshold int    `yaml:"wanted_threshold"`
    // PatrolRoom is the room ID this guard patrols; empty = stationary.
    PatrolRoom      string `yaml:"patrol_room,omitempty"`
}
```

### Behavior

Guards are the only non-combat NPC type that enters the initiative order — they engage rather than flee or cower.

The guard behavior table below shows the default behavior when `WantedThreshold == 2`. The `WantedThreshold` field shifts the "engage" boundary: levels below the threshold follow the "watch/warn" pattern; levels at or above the threshold trigger combat. Levels 1 through `WantedThreshold - 1` always produce the watch/warn response regardless of threshold setting; levels at or above `WantedThreshold` always produce the engage response.

| WantedLevel (default threshold=2) | Guard action |
|-----------------------------------|-------------|
| 0 (None) | Ignores player |
| 1 (Flagged) | Watches player; emits a warning message to the room |
| ≥ WantedThreshold (default: 2 Burned) | Initiates combat — detain at level 2, attack on sight at levels 3–4 |

- REQ-NPC-6: When the Safe room violation flow (defined in the `room-danger-levels` spec) escalates to the second violation, all guards present in the room MUST enter the initiative order and target the aggressor, regardless of the aggressor's WantedLevel.
- REQ-NPC-7: Guards MUST check the player's WantedLevel on room entry and on WantedLevel change events, not only on combat start.

**Named Rustbucket Ridge guard:** One lore-appropriate guard NPC in a Safe room in Rustbucket Ridge (name and specific room TBD during content authoring).

---

## 4. Healer

### Config

```go
type HealerConfig struct {
    PricePerHP    int `yaml:"price_per_hp"`   // credits per HP restored
    DailyCapacity int `yaml:"daily_capacity"` // max total HP restorable per in-game day
}
```

**Runtime state:** `CapacityUsed` is tracked in a `HealerRuntimeState` struct on the NPC instance, persisted to the database on every heal command and restored on server restart.

```go
type HealerRuntimeState struct {
    CapacityUsed int
}
```

### Commands

- `heal` — restores player to MaxHP; cost = `PricePerHP × (MaxHP - CurrentHP)`; fails if insufficient credits or capacity exhausted
- `heal <amount>` — restores specified HP; cost = `PricePerHP × amount`; capped at available capacity and player's missing HP

If a heal request would exceed remaining daily capacity, the healer offers to heal up to remaining capacity at the prorated cost.

- REQ-NPC-16: `CapacityUsed` MUST be reset to 0 on each daily calendar tick. On server restart, `CapacityUsed` MUST be restored from the database, not reset to 0.

### Named Rustbucket Ridge Healers

| NPC | Location |
|-----|----------|
| Clutch | The Tinker's Den |
| Tina Wires | Junker's Dream |

---

## 5. Quest Giver

### Config

```go
type QuestGiverConfig struct {
    // PlaceholderDialog is shown until the Quest system is implemented.
    PlaceholderDialog []string `yaml:"placeholder_dialog"`
    // QuestIDs is populated when the Quest system is implemented.
    QuestIDs []string `yaml:"quest_ids,omitempty"`
}
```

### Commands

- `talk <npc>` — displays one line of `PlaceholderDialog` selected at random; when the Quest system is implemented, this presents available quests filtered by player level and completion state

- REQ-NPC-18: `QuestGiverConfig.PlaceholderDialog` MUST contain at least one entry. An empty list MUST be a fatal load error.

### Stub Behavior

Quest givers are fully implemented as NPCs with `talk` command support. Until the Quest system lands (`quests` feature, priority 380), they respond with placeholder dialog only. No quest state is tracked. `QuestIDs` is empty in all YAML until the Quest system spec is written.

### Named Rustbucket Ridge Quest Giver

| NPC | Location |
|-----|----------|
| Gail "Grinder" Graves | Scrapshack 23 |

---

## 6. Hireling

### Config

```go
type HirelingConfig struct {
    DailyCost      int    `yaml:"daily_cost"`       // credits per in-game day while hired
    CombatRole     string `yaml:"combat_role"`       // "melee" | "ranged" | "support"
    MaxFollowZones int    `yaml:"max_follow_zones"`  // max zone transitions to follow; 0 = unlimited
}
```

### Commands

- `hire <npc>` — deducts `DailyCost` from player credits for the first day; binds hireling to player session; hireling begins following automatically; fails if insufficient credits or hireling is already hired by another player
- REQ-NPC-15: Hireling binding MUST be performed as an atomic check-and-set operation to prevent two concurrent `hire` commands from both succeeding on the same hireling instance.
- `dismiss` — releases hireling; hireling returns to their home room

### Runtime State

```go
type HirelingRuntimeState struct {
    HiredByPlayerID string // empty if not currently hired
    ZonesFollowed   int    // count of zone transitions since hire; compared against MaxFollowZones
}
```

`HirelingRuntimeState` is persisted to the database on every state change (hire, dismiss, zone transition).

### Follow Mechanics

On every room transition, the hireling instance is automatically moved to the player's new room. If the destination is in a different zone, `ZonesFollowed` is incremented. If `MaxFollowZones > 0` and `ZonesFollowed` would exceed `MaxFollowZones`, the hireling remains in the last room of the current zone, the player is warned, and they must dismiss or leave the hireling behind.

### Daily Cost

Charged once per in-game day on the daily calendar tick. If the player cannot pay, the hireling is automatically dismissed with a flavor message.

### Combat Participation

Hirelings join the initiative order as AI-controlled combatants when combat starts in their current room. `CombatRole` maps to an AI domain (`melee` → close-combat AI, `ranged` → ranged AI, `support` → defensive/healing AI). Hirelings are exempt from the flee/cower system — they always participate in combat as an ally.

- REQ-NPC-8: A hireling MUST be treated as a combat ally — it MUST NOT be targetable by the player's own attack commands.

**Named Rustbucket Ridge hireling:** One lore-appropriate hireling NPC in Rustbucket Ridge (name and room TBD during content authoring).

---

## 7. Banker

### Config

```go
type BankerConfig struct {
    ZoneID       string  `yaml:"zone_id"`       // zone this banker operates in
    BaseRate     float64 `yaml:"base_rate"`      // baseline exchange rate; 1.0 = no fee
    RateVariance float64 `yaml:"rate_variance"`  // max daily variance; e.g. 0.05 = ±5%
}
```

### Global Stash

Every player character gains a `StashBalance int` field, persisted to the database. All bankers access the same global stash regardless of zone.

### Exchange Rate

Each banker NPC instance holds a `CurrentRate float64`, recalculated once per in-game day by the calendar tick subsystem. `CurrentRate` MUST be persisted to the database on each recalculation and restored on server restart so that all players see the same rate for a given banker within the same in-game day regardless of restarts.

```
CurrentRate = clamp(BaseRate + rand(-RateVariance, +RateVariance), 0.5, 1.0)
```

- **Deposit:** `stash_added = floor(amount × CurrentRate)` — the banker takes a cut on deposit
- **Withdrawal:** `credits_received = floor(stash_amount / CurrentRate)` — the banker takes a cut on withdrawal

- REQ-NPC-14: Both deposit and withdrawal MUST use `CurrentRate` at the time the command is executed, not the rate at the time of a prior deposit. Rate changes between deposit and withdrawal are expected and by design.

### Commands

- `deposit <amount>` — deducts `amount` from carried credits; adds `floor(amount × CurrentRate)` to `StashBalance`; fails if insufficient carried credits
- `withdraw <amount>` — deducts `amount` from `StashBalance`; adds `floor(amount / CurrentRate)` to carried credits; fails if insufficient stash balance
- `balance` — displays current `StashBalance` and the banker's `CurrentRate`

**Named Rustbucket Ridge banker:** One lore-appropriate banker NPC in a Safe room in Rustbucket Ridge (name and room TBD during content authoring).

---

## 8. Job Trainer

### Config

```go
type JobTrainerConfig struct {
    OfferedJobs []TrainableJob `yaml:"offered_jobs"`
}

type TrainableJob struct {
    JobID         string           `yaml:"job_id"`
    TrainingCost  int              `yaml:"training_cost"`
    Prerequisites JobPrerequisites `yaml:"prerequisites"`
}

type JobPrerequisites struct {
    MinLevel      int               `yaml:"min_level,omitempty"`       // minimum player level
    MinJobLevel   map[string]int    `yaml:"min_job_level,omitempty"`   // minimum level in named jobs
    MinAttributes map[string]int    `yaml:"min_attributes,omitempty"`  // minimum attribute scores
    MinSkillRanks map[string]string `yaml:"min_skill_ranks,omitempty"` // minimum skill rank per skill
    RequiredJobs  []string          `yaml:"required_jobs,omitempty"`   // must currently hold all listed jobs
}
```

### Commands

- `train <job>` — checks all prerequisites; deducts `TrainingCost`; adds job to player's job list at level 1; if any prerequisite is unmet, fails with a message naming the specific unmet requirement
- `jobs` — lists all jobs the player holds with current level in each; marks the active job
- `setjob <job>` — sets the named job as active; player must already hold the job
- REQ-NPC-17: `setjob` MUST be available from any room, not only in job trainer rooms.

### Active Job Rules

- REQ-NPC-9: Players MUST have exactly one active job at all times after their first job is trained.
- REQ-NPC-10: The active job MUST earn XP on combat kills and milestones. Inactive jobs MUST NOT earn XP.
- REQ-NPC-11: Inactive jobs MUST still provide their feats and proficiencies to the player.

**Named Rustbucket Ridge job trainer:** One lore-appropriate job trainer NPC in Rustbucket Ridge (name and room TBD during content authoring).

---

## 9. Crafter (Stub)

The crafter NPC type is declared here as a valid `npc_type` value. Full behavior — item breakdown, enhancement, crafting, component sales — is defined in the `crafting` feature spec (priority 236).

```go
// CrafterConfig is intentionally empty until the crafting feature spec is written.
type CrafterConfig struct{}
```

Crafters default to `flee` on combat start.

---

## 10. Out of Scope

- Active Wanted clearing via guard interaction → `wanted-clearing` feature
- Quest system wiring for quest givers → `quests` feature
- Crafter behavior → `crafting` feature
- Per-zone NPC content (one of each type per zone) → `non-combat-npcs-all-zones` feature
- Per-NPC dialog and daily patterns → `npc-behaviors` feature
- Job Specialist/Expert advancement → `job-development` feature

---

## Requirements Summary

- REQ-NPC-1: NPCs with no `npc_type` MUST default to `"combat"` at load time.
- REQ-NPC-2: The type-specific config sub-struct for the declared `npc_type` MUST be non-nil at load time; mismatch MUST be a fatal load error. For `npc_type: "crafter"`, an explicit `crafter: {}` YAML block MUST be present.
- REQ-NPC-2a: `Template.Validate()` MUST verify all referenced skill IDs exist in the skill registry at load time; unknown skill IDs MUST be fatal load errors.
- REQ-NPC-3: Non-combat NPCs MUST NOT be added to the combat initiative order, except guards when actively engaging per Section 3 behavior.
- REQ-NPC-4: Non-combat NPCs MUST NOT be valid attack targets, except guards when actively engaging per Section 3 behavior.
- REQ-NPC-5: `negotiate` MUST only be usable once per merchant room visit.
- REQ-NPC-5a: The negotiate price modifier MUST be stored on player room session state and cleared on room exit.
- REQ-NPC-5b: WantedLevel 1 surcharge MUST be applied before the negotiate modifier; the surcharge MUST NOT apply to the negotiate roll itself.
- REQ-NPC-6: When the Safe room violation flow escalates to the second violation, all guards present MUST enter the initiative order and target the aggressor regardless of WantedLevel.
- REQ-NPC-7: Guards MUST check WantedLevel on room entry and on WantedLevel change events.
- REQ-NPC-8: Hirelings MUST be treated as combat allies and MUST NOT be targetable by the player's own attack commands.
- REQ-NPC-9: Players MUST have exactly one active job at all times after their first job is trained.
- REQ-NPC-10: The active job MUST earn XP; inactive jobs MUST NOT.
- REQ-NPC-11: Inactive jobs MUST still provide their feats and proficiencies.
- REQ-NPC-12: Merchant runtime state (stock quantities, current budget, `NextReplenishAt`) MUST be persisted to the database and restored on server restart. YAML initial values MUST only apply at first zone initialization.
- REQ-NPC-13: `ReplenishConfig` MUST satisfy `0 < MinHours <= MaxHours <= 24` at load time; violation MUST be a fatal load error.
- REQ-NPC-14: Deposit and withdrawal MUST use `CurrentRate` at the time the command is executed.
- REQ-NPC-15: Hireling binding MUST be an atomic check-and-set operation.
- REQ-NPC-16: `CapacityUsed` MUST reset to 0 on each daily calendar tick; MUST be restored from the database on server restart.
- REQ-NPC-17: `setjob` MUST be available from any room.
- REQ-NPC-18: `QuestGiverConfig.PlaceholderDialog` MUST contain at least one entry; empty list MUST be a fatal load error.
