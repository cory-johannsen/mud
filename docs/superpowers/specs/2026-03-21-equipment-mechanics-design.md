# Equipment Mechanics Expansion — Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `equipment-mechanics` (priority 280)
**Dependencies:** none

---

## Overview

Extends the item and equipment system with six additions: rarity tiers, per-instance durability, item modifiers (tuned/defective/cursed), equipment set bonuses, team-based consumable effectiveness, and five new consumable items.

---

## 1. Rarity

### 1.1 Rarity Tiers

| Tier | ID | Display Color | Stat Multiplier | Feature Slots | Feature Effectiveness | Min Level |
|------|----|---------------|-----------------|---------------|-----------------------|-----------|
| 1 | `salvage` | Gray | 1.0× | 0 | 1.00× | 0 |
| 2 | `street` | White | 1.2× | 1 | 1.10× | 1 |
| 3 | `mil_spec` | Green | 1.5× | 2 | 1.25× | 5 |
| 4 | `black_market` | Purple | 1.8× | 3 | 1.40× | 10 |
| 5 | `ghost` | Gold | 2.2× | 4 | 1.60× | 15 |

- REQ-EM-1: Every weapon and armor YAML definition MUST include a `rarity` field (one of the five tier IDs). Absence MUST be a fatal load error at startup.
- REQ-EM-2: Weapon and armor base stats (damage dice average, AC bonus) MUST be multiplied by the rarity stat multiplier at load time. The multiplied values are stored as the effective base values on `WeaponDef` and `ArmorDef`. All per-instance modifier effects (Section 3.1) are applied on top of these already-multiplied base values.
- REQ-EM-3: Items with a minimum level requirement MUST NOT be equippable by characters below that level. Attempting to equip MUST produce: `"You need to be level <N> to equip <item name>."`.
- REQ-EM-4: Item name display MUST be color-coded by rarity tier using ANSI escape codes in all inventory and equipment listings.

### 1.2 YAML Schema Addition

All weapon and armor YAML definitions gain:

```yaml
rarity: street   # required; one of: salvage, street, mil_spec, black_market, ghost
```

---

## 2. Durability

### 2.1 Data Model

Durability is tracked per item instance. The canonical `ItemInstance` struct is in `internal/game/inventory/backpack.go`. That struct gains:

```go
type ItemInstance struct {
    InstanceID    string // UUID (existing)
    ItemDefID     string // (existing)
    Quantity      int    // (existing)
    Durability    int
    MaxDurability int
    Modifier      string // "" | "tuned" | "defective" | "cursed"
    CurseRevealed bool   // true once the cursed item has been equipped
}
```

`SlottedItem` in `internal/game/inventory/equipment.go` gains:

```go
type SlottedItem struct {
    ItemDefID     string // (existing)
    Name          string // (existing)
    InstanceID    string // UUID — links to ItemInstance
    Durability    int    // cached; source of truth is ItemInstance
    Modifier      string
    CurseRevealed bool
}
```

`EquippedWeapon` in `internal/game/inventory/preset.go` gains:

```go
type EquippedWeapon struct {
    Def        *WeaponDef // (existing)
    Magazine   *Magazine  // (existing)
    InstanceID string     // UUID — links to ItemInstance
    Durability int        // cached; source of truth is ItemInstance
    Modifier   string
}
```

### 2.2 Degradation Rules

- REQ-EM-5: The active weapon in `EquippedWeapon` MUST lose 1 durability point per attack roll (hit or miss). The attack handler resolves the instance via `EquippedWeapon.InstanceID`.
- REQ-EM-6: The armor `SlottedItem` in the struck slot MUST lose 1 durability point per hit received. The damage handler resolves the instance via `SlottedItem.InstanceID`.
- REQ-EM-7: An item with `Durability == 0` is broken. `ComputedDefenses` MUST return 0 AC bonus for any broken armor slot. The attack handler MUST treat a broken weapon as dealing 0 damage beyond the character's unarmed base damage.
- REQ-EM-8: Modifier adjustments (Section 3.1) MUST NOT be applied to broken items — a broken item contributes nothing regardless of its modifier.
- REQ-EM-9: When an item reaches 0 durability, a destruction roll MUST be made immediately using the destruction chance for its rarity (Section 2.3).
- REQ-EM-10: A destroyed item MUST be removed from the character's inventory or equipment permanently. The player MUST receive: `"Your <item name> has been destroyed."`.
- REQ-EM-11: An item that survives the destruction roll (REQ-EM-9) remains in inventory or equipped as broken (`Durability == 0`) and can be repaired.

### 2.3 Rarity Durability and Destruction

| Rarity | Max Durability | Destruction Chance at 0 |
|--------|----------------|--------------------------|
| Salvage | 20 | 50% |
| Street | 40 | 30% |
| Mil-Spec | 60 | 15% |
| Black Market | 80 | 5% |
| Ghost | 100 | 1% |

- REQ-EM-12: `MaxDurability` and `DestructionChance` for each rarity tier MUST match the values in Section 2.3. These are immutable game constants, not loaded from YAML.

### 2.4 Repair Commands

**`repair <item>`** (field repair):

- REQ-EM-13: `repair <item>` MUST require a `repair_kit` consumable in the player's backpack. If absent, the command MUST fail with: `"You need a repair kit to field-repair equipment."`. The command handler MUST consume the `repair_kit` before calling `RepairField`.
- REQ-EM-14: `repair <item>` MUST restore `1d6` durability points (floored). It MUST NOT restore durability above `MaxDurability`.
- REQ-EM-15: `repair <item>` costs 1 AP when used in combat.

**Downtime Repair** (via existing `downtime` feature Repair activity):

- REQ-EM-16: The Downtime Repair activity MUST restore the item to full `MaxDurability`. Cost: `ceil((MaxDurability - CurrentDurability) / 10)` downtime days, minimum 1.

### 2.5 Database

`character_equipment` table gains a durability column:

```sql
ALTER TABLE character_equipment
    ADD COLUMN durability    int NOT NULL DEFAULT -1,
    ADD COLUMN max_durability int NOT NULL DEFAULT -1;
```

`character_weapon_presets` table gains durability columns:

```sql
ALTER TABLE character_weapon_presets
    ADD COLUMN durability    int NOT NULL DEFAULT -1,
    ADD COLUMN max_durability int NOT NULL DEFAULT -1;
```

The `characters` table gains the team column if not already present:

```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS team text NOT NULL DEFAULT '';
```

A new table for backpack item instances:

```sql
CREATE TABLE character_inventory_instances (
    instance_id    text   PRIMARY KEY,
    character_id   bigint NOT NULL REFERENCES characters(id),
    item_def_id    text   NOT NULL,
    durability     int    NOT NULL DEFAULT -1,
    max_durability int    NOT NULL DEFAULT -1,
    modifier       text   NOT NULL DEFAULT '',
    curse_revealed bool   NOT NULL DEFAULT false
);
```

- REQ-EM-17: A `DEFAULT -1` sentinel value on durability columns indicates an un-initialized legacy row. On load, if `durability == -1`, the system MUST initialize `Durability = MaxDurability` for the item's rarity tier and persist the corrected values before use.

---

## 3. Modifiers

### 3.1 Modifier Types

Three modifier variants apply to weapon and armor instances:

| Modifier | Type | Weapon Effect | Armor Effect | Visibility |
|----------|------|---------------|--------------|------------|
| `tuned` | positive | +10% damage (applied on top of rarity-multiplied base) | +1 AC | Always visible |
| `defective` | negative | −10% damage | −1 AC | Always visible |
| `cursed` | negative | −15% damage | −2 AC; cannot unequip normally | Hidden until equipped |

- REQ-EM-18: A `tuned` item MUST display as `"Tuned <item name>"` in all inventory and equipment listings.
- REQ-EM-19: A `defective` item MUST display as `"Defective <item name>"`.
- REQ-EM-20: A `cursed` item MUST display as its unmodified name until `CurseRevealed == true`. After equipping, `CurseRevealed` is set to `true` and it MUST display as `"Cursed <item name>"`.
- REQ-EM-21: Modifier damage/AC adjustments are applied to the rarity-multiplied base value at combat resolution time; they are NOT baked into `WeaponDef`/`ArmorDef`.
- REQ-EM-22: `ComputedDefenses` MUST apply the `Modifier` adjustment from each `SlottedItem` when computing total AC. Broken items (Section 2.2 REQ-EM-7) contribute 0 regardless of modifier.
- REQ-EM-23: The attack handler MUST apply the `Modifier` adjustment from `EquippedWeapon` when computing damage. Broken weapons contribute 0 damage regardless of modifier.
- REQ-EM-24: The `unequip` command MUST fail for items with `Modifier == "cursed"` and `CurseRevealed == true` with: `"This item is cursed and cannot be removed."`.
- REQ-EM-25: When a cursed item is successfully uncursed (via `curse-removal` feature), its `Modifier` MUST be changed to `"defective"` and `CurseRevealed` reset to `false`.

### 3.2 Modifier Probability at Spawn

When a new item instance is created (loot drop, merchant spawn, chest), a modifier roll is made:

| Rarity | Tuned % | Defective % | Cursed % | Normal % |
|--------|---------|-------------|----------|----------|
| Salvage | 0% | 30% | 10% | 60% |
| Street | 5% | 15% | 5% | 75% |
| Mil-Spec | 10% | 10% | 3% | 77% |
| Black Market | 20% | 5% | 2% | 73% |
| Ghost | 30% | 2% | 1% | 67% |

- REQ-EM-26: Modifier assignment MUST use the probabilities in Section 3.2. These are immutable game constants.
- REQ-EM-27: Merchants MUST NOT stock `cursed` items. Only loot drops and chest spawns may yield cursed items.

---

## 4. Equipment Sets

### 4.1 Set Definition

Equipment sets are defined in `content/sets/*.yaml`. Example:

```yaml
id: street_rat_set
name: Street Rat's Outfit
pieces:
  - item_def_id: street_jacket
  - item_def_id: street_trousers
  - item_def_id: street_boots
  - item_def_id: street_gloves
bonuses:
  - threshold: 2
    description: "+1 to Stealth checks"
    effect:
      type: skill_bonus
      skill: stealth
      amount: 1
  - threshold: 3
    description: "+1 AC"
    effect:
      type: ac_bonus
      amount: 1
  - threshold: full
    description: "+2 Speed"
    effect:
      type: speed_bonus
      amount: 2
```

- REQ-EM-28: `threshold: full` MUST resolve to `len(pieces)` at load time.
- REQ-EM-29: Set bonuses MUST be evaluated and applied to derived stats at login and whenever equipped armor changes.
- REQ-EM-30: Set bonuses MUST be rarity-independent (a Salvage piece counts identically to a Ghost piece for threshold purposes).
- REQ-EM-31: Active set bonuses MUST be displayed on the character sheet equipment section.

### 4.2 Supported Effect Types

| Effect Type | Fields | Description |
|-------------|--------|-------------|
| `skill_bonus` | `skill string`, `amount int` | Bonus to named skill checks |
| `ac_bonus` | `amount int` | Flat AC bonus added to `DefenseStats.ACBonus` |
| `speed_bonus` | `amount int` | Flat Speed bonus |
| `stat_bonus` | `stat string`, `amount int` | Flat attribute bonus |
| `condition_immunity` | `condition_id string` | Immunity to named condition; condition_id MUST reference a known condition or load fails |

- REQ-EM-32: All effect types in Section 4.2 MUST be supported. Unrecognized `type` values MUST be a fatal load error.
- REQ-EM-33: A `condition_id` in a set bonus MUST reference a condition ID that exists in `content/conditions/`. An unresolvable `condition_id` MUST be a fatal load error.

### 4.3 SetBonus and Threshold Types

```go
type SetThreshold struct {
    IsFull bool
    Count  int
}

type SetDef struct {
    ID      string      `yaml:"id"`
    Name    string      `yaml:"name"`
    Pieces  []SetPiece  `yaml:"pieces"`
    Bonuses []SetBonus  `yaml:"bonuses"`
}

type SetPiece struct {
    ItemDefID string `yaml:"item_def_id"`
}

type SetBonus struct {
    Threshold   SetThreshold // parsed from YAML; "full" → IsFull=true; int → Count=N
    Description string       `yaml:"description"`
    Effect      SetEffect    `yaml:"effect"`
}

type SetEffect struct {
    Type        string `yaml:"type"`
    Skill       string `yaml:"skill,omitempty"`
    Stat        string `yaml:"stat,omitempty"`
    ConditionID string `yaml:"condition_id,omitempty"`
    Amount      int    `yaml:"amount,omitempty"`
}
```

`SetThreshold` implements `yaml.Unmarshaler` to parse either an integer or the string `"full"`.

### 4.4 SetRegistry and Application

`SetRegistry.ActiveBonuses(equippedItemIDs []string) []SetBonus` is a pure function returning all bonuses whose thresholds are met.

Set bonuses that produce `ac_bonus` are added into `DefenseStats.ACBonus` by the derived-stats computation path. Set bonuses that produce `skill_bonus`, `speed_bonus`, `stat_bonus`, or `condition_immunity` are accumulated into a `SetBonusSummary` struct on `PlayerSession` that is rebuilt at login and on equipment change:

```go
type SetBonusSummary struct {
    SkillBonuses        map[string]int // skill ID → total bonus
    SpeedBonus          int
    StatBonuses         map[string]int // stat ID → total bonus
    ConditionImmunities []string       // condition IDs
}
```

`PlayerSession.SetBonuses SetBonusSummary` is consulted at skill check resolution, speed computation, stat computation, and condition application.

- REQ-EM-34: `SetRegistry.ActiveBonuses` MUST be a pure function with no side effects.
- REQ-EM-35: The derived-stats computation path MUST apply all active set bonuses from `SetRegistry.ActiveBonuses` when computing `DefenseStats` and `SetBonusSummary`.

---

## 5. Team Mechanic for Consumables

### 5.1 Team Field on Consumables

`ItemDef` gains:

```go
Team string `yaml:"team,omitempty"` // "gun" | "machete" | ""
```

- REQ-EM-36: If `ItemDef.Team` is set and is not `"gun"` or `"machete"`, it MUST be a fatal load error.

Consumable YAML gains an optional field:

```yaml
team: gun   # optional; "gun" | "machete"
```

### 5.2 Effectiveness Multiplier

| Relationship | Multiplier |
|--------------|------------|
| Player team matches item team | 1.25× |
| Player team opposes item team | 0.75× |
| Item has no team | 1.00× |

- REQ-EM-37: Consumable effect values (HP restored, stat bonus amounts, condition durations in seconds) MUST be multiplied by the team effectiveness multiplier before application.
- REQ-EM-38: Multiplied values MUST be floored to the nearest integer.
- REQ-EM-39: `PlayerSession.Team` MUST be loaded from `characters.team` at login. If empty, the 1.0× neutral multiplier MUST be used.

---

## 6. New Consumables

Five new consumable items in `content/consumables/`. All base effect values are the neutral (1.0×) values; the team multiplier (Section 5.2) is applied at use time.

### 6.1 Whore's Pasta

```yaml
id: whores_pasta
name: "Whore's Pasta"
description: "A heaping plate of carb-loaded street cuisine favored by Gun-side enforcers."
kind: consumable
team: gun
weight: 0.5
stackable: true
max_stack: 5
value: 15
effect:
  heal: "2d6+4"
  conditions:
    - condition_id: strength_boost_1
      duration: "1h"
  consume_check:
    stat: constitution
    dc: 12
    on_critical_failure:
      apply_disease:
        disease_id: street_rot
        severity: 1
```

### 6.2 Poontangesca

```yaml
id: poontangesca
name: "Poontangesca"
description: "A thick, spicy machete-side stew. Clears the mind and the legs."
kind: consumable
team: machete
weight: 0.5
stackable: true
max_stack: 5
value: 15
effect:
  remove_conditions:
    - fatigued
  conditions:
    - condition_id: speed_boost_2
      duration: "30m"
  consume_check:
    stat: constitution
    dc: 12
    on_critical_failure:
      apply_toxin:
        toxin_id: gut_rot
        severity: 1
```

### 6.3 4Loko

```yaml
id: four_loko
name: "4Loko"
description: "The machete-side energy drink of champions. Results may vary."
kind: consumable
team: machete
weight: 0.3
stackable: true
max_stack: 10
value: 8
effect:
  heal: "1d8"
  conditions:
    - condition_id: attack_bonus_1
      duration: "1h"
  consume_check:
    stat: constitution
    dc: 10
    on_critical_failure:
      conditions:
        - condition_id: sickened
          duration: "15m"
```

### 6.4 Old English

```yaml
id: old_english
name: "Old English"
description: "A tall can of Gun-side fortification. Smooth."
kind: consumable
team: gun
weight: 0.3
stackable: true
max_stack: 10
value: 8
effect:
  heal: "1d6+2"
  conditions:
    - condition_id: fortitude_bonus_1
      duration: "1h"
```

### 6.5 Penjamin Franklin

```yaml
id: penjamin_franklin
name: "Penjamin Franklin"
description: "A premium healing ration. No allegiances. Just results."
kind: consumable
team: ""
weight: 0.4
stackable: true
max_stack: 5
value: 40
effect:
  heal: "3d6"
```

### 6.6 Repair Kit

```yaml
id: repair_kit
name: "Repair Kit"
description: "A bundle of solder, patches, and polymer adhesive for field repairs."
kind: consumable
team: ""
weight: 1.0
stackable: true
max_stack: 5
value: 20
effect:
  repair_field: true   # consumed by the `repair <item>` command handler; not a direct-use effect
```

- REQ-EM-40: All six items in Section 6 MUST be loaded at startup. Missing YAML files MUST be a fatal load error.
- REQ-EM-41: The `consume_check` roll MUST use a d20 + the named stat modifier vs the given DC. A critical failure (total ≤ DC − 10, or natural 1) applies the `on_critical_failure` effects per PF2E four-tier rules.
- REQ-EM-42: `apply_disease` and `apply_toxin` effects MUST apply the named disease or toxin at the specified starting severity via the condition system. If the referenced `disease_id` or `toxin_id` does not exist in `content/conditions/`, it MUST be a fatal load error.
- REQ-EM-43: The `remove_conditions` effect MUST remove all listed conditions from the player before applying any new conditions from the same use.

---

## 7. Architecture

### 7.1 Roller Interface

All functions requiring dice rolls accept a `Roller` interface defined in `internal/game/inventory/roller.go`:

```go
type Roller interface {
    Roll(dice string) int  // e.g. "2d6+4", "1d20"
    RollD20() int
    RollFloat() float64    // [0.0, 1.0) for probability checks
}
```

### 7.2 DurabilityManager

`internal/game/inventory/durability.go`:

```go
type DeductResult struct {
    NewDurability int
    BecameBroken  bool  // true if durability just reached 0
    Destroyed     bool  // true if BecameBroken AND destruction roll succeeded
}

// DeductDurability reduces instance durability by 1 and returns the result.
// Precondition: if inst.Durability == 0, this is a no-op; returns DeductResult{} with
//   BecameBroken=false and Destroyed=false. No destruction re-roll is made.
// Case 1 (durability > 1 after deduct): BecameBroken=false, Destroyed=false.
// Case 2 (durability reaches 0, destruction roll succeeds): BecameBroken=true, Destroyed=true.
//   Caller MUST remove the item from inventory/equipment permanently.
// Case 3 (durability reaches 0, destruction roll fails): BecameBroken=true, Destroyed=false.
//   Item remains in inventory/equipment as broken (Durability==0) and may be repaired.
func DeductDurability(inst *ItemInstance, rng Roller) DeductResult

// RepairField restores 1d6 durability (capped at MaxDurability). Returns points restored.
// Caller MUST consume a repair_kit before calling this.
func RepairField(inst *ItemInstance, rng Roller) int

// RepairFull restores item to MaxDurability.
func RepairFull(inst *ItemInstance)

// InitDurability sets Durability = MaxDurability for the item's rarity if Durability == -1.
func InitDurability(inst *ItemInstance, rarity string)
```

- REQ-EM-44: `DeductDurability` and `RepairField` MUST be pure functions with respect to `ItemInstance`. All persistence (DB writes) MUST happen in the caller.

### 7.3 RarityRegistry

`internal/game/inventory/rarity.go` — immutable constants (not YAML):

```go
type RarityDef struct {
    ID                   string
    StatMultiplier       float64
    FeatureSlots         int
    FeatureEffectiveness float64
    MinLevel             int
    MaxDurability        int
    DestructionChance    float64
    ModifierProbs        ModifierProbs
}

type ModifierProbs struct {
    Tuned     float64
    Defective float64
    Cursed    float64
}
```

### 7.4 ConsumableEffect Engine

`internal/game/inventory/consumable.go` defines the effect structs and a narrow interface to avoid import cycles with `internal/game/session`:

```go
type ConsumableEffect struct {
    Heal             string             `yaml:"heal,omitempty"`           // dice string e.g. "2d6+4"
    Conditions       []ConditionEffect  `yaml:"conditions,omitempty"`
    RemoveConditions []string           `yaml:"remove_conditions,omitempty"`
    ConsumeCheck     *ConsumeCheck      `yaml:"consume_check,omitempty"`
    RepairField      bool               `yaml:"repair_field,omitempty"`
}

type ConditionEffect struct {
    ConditionID string `yaml:"condition_id"`
    Duration    string `yaml:"duration"` // Go time.Duration string e.g. "1h", "30m"
}

type ConsumeCheck struct {
    Stat              string              `yaml:"stat"`    // attribute ID e.g. "constitution"
    DC                int                 `yaml:"dc"`
    OnCriticalFailure *CritFailureEffect  `yaml:"on_critical_failure,omitempty"`
}

type CritFailureEffect struct {
    Conditions   []ConditionEffect `yaml:"conditions,omitempty"`
    ApplyDisease *DiseaseEffect    `yaml:"apply_disease,omitempty"`
    ApplyToxin   *ToxinEffect      `yaml:"apply_toxin,omitempty"`
}

type DiseaseEffect struct {
    DiseaseID string `yaml:"disease_id"`
    Severity  int    `yaml:"severity"`
}

type ToxinEffect struct {
    ToxinID  string `yaml:"toxin_id"`
    Severity int    `yaml:"severity"`
}
```

`ItemDef` gains `Effect *ConsumableEffect \`yaml:"effect,omitempty"\`` for consumable items.

`internal/game/inventory/consumable.go` defines a narrow interface to avoid import cycles with `internal/game/session`:

```go
// ConsumableTarget is the minimal interface required to apply consumable effects.
// Implemented by *session.PlayerSession.
type ConsumableTarget interface {
    GetTeam() string
    ApplyHeal(amount int)
    ApplyCondition(conditionID string, duration time.Duration)
    RemoveCondition(conditionID string)
    ApplyDisease(diseaseID string, severity int)
    ApplyToxin(toxinID string, severity int)
}

// ConsumableResult holds the resolved effects for display and audit.
type ConsumableResult struct {
    HealApplied        int
    ConditionsApplied  []string
    ConditionsRemoved  []string
    DiseaseApplied     string
    ToxinApplied       string
    TeamMultiplier     float64
    ConsumeCheckResult string // "success" | "failure" | "critical_failure" | "not_checked"
}

// ApplyConsumable applies all effects from the consumable to the target.
// This is a pure function: it calls only methods on the ConsumableTarget interface
// and returns a ConsumableResult. No direct state mutation occurs here.
func ApplyConsumable(target ConsumableTarget, def *ItemDef, rng Roller) ConsumableResult
```

- REQ-EM-45: `ApplyConsumable` MUST use the `ConsumableTarget` interface. It MUST NOT import `internal/game/session`.

### 7.5 SetRegistry

`internal/game/inventory/set_registry.go` — loads `content/sets/*.yaml` at startup. `ActiveBonuses` is a pure function (REQ-EM-34).

### 7.6 Fire Points

| Event | Action |
|-------|--------|
| Weapon attack roll resolved (hit or miss) | `DeductDurability` on `EquippedWeapon.InstanceID`'s ItemInstance |
| Armor slot receives a hit | `DeductDurability` on `SlottedItem.InstanceID`'s ItemInstance for the struck slot |
| `repair <item>` command | Verify + consume `repair_kit`, then `RepairField` |
| Downtime Repair activity completes | `RepairFull` |
| Item equipped | If `Modifier == "cursed"`, set `CurseRevealed = true` and persist |
| Login with `durability == -1` | `InitDurability` for all equipped and backpack items |

---

## 8. Requirements Summary

- REQ-EM-1: `rarity` field required on weapon/armor YAML; absence fatal at startup.
- REQ-EM-2: Base stats multiplied by rarity stat multiplier at load time; modifiers applied on top at resolution time.
- REQ-EM-3: Min level enforced at equip time with specific message.
- REQ-EM-4: Item names color-coded by rarity in all display contexts.
- REQ-EM-5: Active weapon loses 1 durability per attack roll.
- REQ-EM-6: Struck armor slot loses 1 durability per hit received.
- REQ-EM-7: Broken armor contributes 0 AC; broken weapons contribute 0 damage beyond unarmed base.
- REQ-EM-8: Modifier adjustments not applied to broken items.
- REQ-EM-9: Destruction roll made when item reaches 0 durability.
- REQ-EM-10: Destroyed items removed permanently; player notified.
- REQ-EM-11: Broken items remain and can be repaired.
- REQ-EM-12: MaxDurability and DestructionChance MUST match Section 2.3 constants.
- REQ-EM-13: `repair` requires `repair_kit`; handler consumes kit before calling RepairField.
- REQ-EM-14: `repair` restores 1d6 durability, capped at MaxDurability.
- REQ-EM-15: `repair` costs 1 AP in combat.
- REQ-EM-16: Downtime Repair restores full durability; cost scales with damage.
- REQ-EM-17: `durability == -1` sentinel triggers InitDurability on load.
- REQ-EM-18: Tuned items display as "Tuned <name>".
- REQ-EM-19: Defective items display as "Defective <name>".
- REQ-EM-20: Cursed items display as normal name until equipped; then "Cursed <name>".
- REQ-EM-21: Modifier effects applied at resolution time on top of rarity-multiplied base.
- REQ-EM-22: ComputedDefenses applies modifier AC adjustment per SlottedItem; 0 if broken.
- REQ-EM-23: Attack handler applies modifier damage adjustment per EquippedWeapon; 0 if broken.
- REQ-EM-24: `unequip` fails for cursed items with specific message.
- REQ-EM-25: Uncursed items become defective.
- REQ-EM-26: Modifier spawn probabilities MUST match Section 3.2 constants.
- REQ-EM-27: Merchants MUST NOT stock cursed items.
- REQ-EM-28: `threshold: full` resolves to `len(pieces)` at load time.
- REQ-EM-29: Set bonuses evaluated at login and on equipment change.
- REQ-EM-30: Set bonuses rarity-independent.
- REQ-EM-31: Active set bonuses displayed on character sheet.
- REQ-EM-32: Unrecognized set effect types fatal at load.
- REQ-EM-33: Unresolvable `condition_id` in set bonus fatal at load.
- REQ-EM-34: `SetRegistry.ActiveBonuses` is a pure function.
- REQ-EM-35: Derived-stats path applies all active set bonuses.
- REQ-EM-36: Invalid `ItemDef.Team` value fatal at load.
- REQ-EM-37: Consumable effects multiplied by team effectiveness multiplier.
- REQ-EM-38: Multiplied values floored.
- REQ-EM-39: `PlayerSession.Team` loaded from `characters.team`; empty = 1.0× multiplier.
- REQ-EM-40: All six new items loaded at startup; missing files fatal.
- REQ-EM-41: `consume_check` uses d20 + stat modifier vs DC; PF2E four-tier critical failure.
- REQ-EM-42: `apply_disease` and `apply_toxin` condition IDs MUST be validated at load.
- REQ-EM-43: `remove_conditions` clears listed conditions before applying new ones.
- REQ-EM-44: `DeductDurability` and `RepairField` are pure functions; persistence in caller.
- REQ-EM-45: `ApplyConsumable` uses `ConsumableTarget` interface; MUST NOT import `session` package.
