# Gear Actions — Design Spec

**Date:** 2026-03-25
**Status:** Approved
**Feature:** `gear-actions` (priority 45)
**Dependencies:** `equipment-mechanics`

---

## Overview

Gear Actions implement the two remaining PF2E gear-category actions: **Repair** and **Affix a Precious Material**. Activate Item and Swap are already complete (see `docs/features/actions.md`).

**Repair** is fully implemented as part of `equipment-mechanics` (REQ-EM-13 through REQ-EM-16). This spec covers the `affix` command only.

---

## 1. Precious Materials

### 1.1 Item Kind

A new item kind `precious_material` is added to `ItemDef.Kind`. It MUST be added to the `validKinds` map in `internal/game/inventory/item.go` so that startup validation accepts precious material YAML files.

Precious material items are carried in the backpack and consumed (or returned on critical success) when affixed.

Each material exists at three grades. Each grade is a separate `ItemDef` with a distinct ID:

```
<material_id>_street_grade
<material_id>_mil_spec_grade
<material_id>_ghost_grade
```

Example: `carbide_alloy_street_grade`, `carbide_alloy_mil_spec_grade`, `carbide_alloy_ghost_grade`.

### 1.2 Grade Terminology

| PF2E Grade | Gunchete Grade | ID |
|---|---|---|
| Standard | Street Grade | `street_grade` |
| Higher Grade | Mil-Spec Grade | `mil_spec_grade` |
| Major | Ghost Grade | `ghost_grade` |

### 1.3 Material Registry

Fifteen materials are the canonical required set. Startup validation MUST check that all 45 YAML files (15 materials × 3 grades) exist in `content/items/precious_materials/`. A missing file MUST be a fatal load error.

| Gunchete Name | PF2E Source | Tier | ID | Applies To |
|---|---|---|---|---|
| Scrap Iron | Cold Iron | Common | `scrap_iron` | weapon |
| Hollow Point | Silver | Common | `hollow_point` | weapon |
| Carbide Alloy | Adamantine | Uncommon | `carbide_alloy` | weapon, armor |
| Carbon Weave | Mithral | Uncommon | `carbon_weave` | armor |
| Polymer Frame | Darkwood | Uncommon | `polymer_frame` | weapon, armor |
| Thermite Lace | Siccatite (hot) | Uncommon | `thermite_lace` | weapon |
| Cryo-Gel | Siccatite (cold) | Uncommon | `cryo_gel` | weapon |
| Quantum Alloy | Orichalcum | Rare | `quantum_alloy` | weapon, armor |
| Rad-Core | Abysium | Rare | `rad_core` | weapon, armor |
| Neural Gel | Djezet | Rare | `neural_gel` | weapon, armor |
| Ghost Steel | Inubrix | Rare | `ghost_steel` | weapon |
| Null-Weave | Noqual | Rare | `null_weave` | weapon, armor |
| Soul-Guard Alloy | Sovereign Steel | Rare | `soul_guard_alloy` | armor |
| Shadow Plate | Sisterstone (Dusk) | Rare | `shadow_plate` | weapon |
| Radiance Plate | Sisterstone (Dawn) | Rare | `radiance_plate` | weapon |

The `Applies To` column is enforced at affix time (see REQ-GA-10).

### 1.4 ItemDef additions

```go
const KindPreciousMaterial = "precious_material"

// Added to ItemDef:
MaterialID string `yaml:"material_id,omitempty"` // set when Kind == KindPreciousMaterial
GradeID    string `yaml:"grade_id,omitempty"`    // "street_grade" | "mil_spec_grade" | "ghost_grade"
```

---

## 2. Upgrade Slots

`RarityDef.FeatureSlots` (already defined in `internal/game/inventory/rarity.go`) is the source of truth for upgrade slot counts:

| Rarity | Upgrade Slots |
|---|---|
| Salvage | 0 |
| Street | 1 |
| Mil-Spec | 2 |
| Black Market | 3 |
| Ghost | 4 |

- REQ-GA-1: `WeaponDef` and `ArmorDef` MUST gain `UpgradeSlots int` tagged `yaml:"-"`. The loader MUST derive its value from `RarityDef.FeatureSlots` after the `Rarity` field is resolved, following the same pattern as `RarityStatMultiplier`.
- REQ-GA-2: `ItemInstance` MUST gain `AffixedMaterials []string`. Each entry is formatted `"<material_id>:<grade_id>"` (e.g., `"carbide_alloy:mil_spec_grade"`).
- REQ-GA-3: `EquippedWeapon` MUST gain `AffixedMaterials []string`. `SlottedItem` MUST gain `AffixedMaterials []string`. These are cached copies populated from the corresponding `ItemInstance.AffixedMaterials` at equip time and at session load time, following the same caching pattern used for `Durability` and `Modifier`.
- REQ-GA-4: The same material ID MUST NOT be affixed twice on the same item instance regardless of grade. Attempting to do so MUST fail with: `"<item name> already has <material name> affixed."`
- REQ-GA-5: `len(AffixedMaterials)` MUST NOT exceed `UpgradeSlots`. Attempting to affix when no slots remain MUST fail with: `"<item name> has no upgrade slots remaining."`
- REQ-GA-6: When a host item is destroyed (via `DeductDurability` returning `Destroyed: true`), all affixed materials on that item are permanently lost. No recovery is attempted. The player MUST receive the standard destruction message (REQ-EM-10) only; no additional message for lost materials is required.

---

## 3. Crafting DCs

| Material Tier | Street Grade DC | Mil-Spec Grade DC | Ghost Grade DC |
|---|---|---|---|
| Common | 16 | 21 | 26 |
| Uncommon | 18 | 23 | 28 |
| Rare | 20 | 25 | 30 |

- REQ-GA-7: Crafting check DCs MUST match Section 3 constants. These are immutable game constants, not loaded from YAML.

---

## 4. The `affix` Command

### 4.1 Syntax

```
affix <material_item> <target_item>
```

`<material_item>` matches against the item's ID or name (case-insensitive) in the player's backpack.
`<target_item>` matches against equipped weapon or armor slot item ID or name (case-insensitive).

### 4.2 Preconditions

All preconditions are checked in order before the Crafting roll is made. Any failure returns immediately.

- REQ-GA-8: `affix` MUST be rejected in combat with: `"You cannot affix materials during combat."`
- REQ-GA-9: `affix` MUST fail if the material item is not found in the player's backpack with: `"You don't have <material name> in your pack."`
- REQ-GA-10: `affix` MUST fail if the target item is not found in equipped weapon or armor slots with: `"<target> is not equipped."`
- REQ-GA-11: `affix` MUST fail if the target item is broken (`Durability == 0`) with: `"You cannot affix materials to broken equipment. Repair it first."`
- REQ-GA-12: `affix` MUST fail if the material's `Applies To` restriction (Section 1.3) does not include the target item's slot type (weapon vs armor) with: `"<material name> cannot be affixed to armor."` or `"<material name> cannot be affixed to weapons."` as appropriate.
- REQ-GA-13: REQ-GA-4 (duplicate material check) and REQ-GA-5 (slot count check) MUST be applied before the Crafting roll.

### 4.3 Crafting Check

- REQ-GA-14: The Crafting check total is computed as:

  ```
  total = d20 + Abilities.Modifier(Abilities.Reasoning) + skillcheck.ProficiencyBonus(Skills["crafting"])
  ```

  where `Abilities` is `PlayerSession.Abilities` and `Skills["crafting"]` is the character's crafting proficiency rank from `PlayerSession.Skills`.

- REQ-GA-15: The outcome is determined by `skillcheck.OutcomeFor(total, dc)` where `dc` is the grade DC from Section 3.

### 4.4 Resolution

- REQ-GA-16: **Critical success** (`total >= dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is returned to the player's backpack (not consumed). Message: `"Exceptional work. <material name> affixed to <item name> — material returned intact."`
- REQ-GA-17: **Success** (`dc <= total < dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is consumed from backpack. Message: `"<material name> affixed to <item name>."`
- REQ-GA-18: **Failure** (`dc - 10 <= total < dc`): nothing changes, material is not consumed. Message: `"Your hands slip. The material is undamaged but the affix fails."`
- REQ-GA-19: **Critical failure** (`total < dc - 10`): material item is destroyed (removed from backpack), slot is not consumed. Message: `"You ruin the material. <material name> is destroyed."`

### 4.5 Display

- REQ-GA-20: `inventory` and `loadout` output MUST show affixed materials as a sub-list under the item with used/total slot count:
  ```
  Mil-Spec Pistol [2/3 slots]
    ↳ Carbide Alloy (Street Grade)
    ↳ Hollow Point (Mil-Spec Grade)
  ```
- REQ-GA-21: Items with 0 upgrade slots (Salvage rarity) MUST NOT show slot indicators.

---

## 5. Material Effects

Effects are applied at combat resolution time alongside rarity multipliers and modifiers. All effect functions MUST be pure — no side effects, no DB writes. Armor-only effects are marked *(armor)*; weapon-only effects are unmarked.

### 5.1 Types

```go
// AttackContext carries the per-hit context needed to evaluate material effects.
type AttackContext struct {
    TargetIsCyberAugmented bool
    TargetIsSupernatural   bool
    TargetIsLightAspected  bool
    TargetIsShadowAspected bool
    TargetIsMetalArmored   bool // true when target's equipped armor includes a metal slot
    IsHit                  bool // false for miss (some effects only apply on hit)
    IsFirstHitThisCombat   bool // used by Ghost Steel street grade
}

// MaterialEffectResult holds all effect values produced by ApplyMaterialEffects.
// Callers aggregate results across all affixed materials before applying.
type MaterialEffectResult struct {
    DamageBonus            int
    PersistentFireDmg      int
    PersistentColdDmg      int
    PersistentRadDmg       int
    PersistentBleedDmg     int
    CarrierRadDmgPerHour   int
    TargetLosesAP          int  // AP penalty applied to target for 1 round
    TargetSpeedPenalty     int  // feet reduction for 1 round
    TargetFlatFooted       bool // 1 round
    TargetDazzled          bool // until next turn
    TargetBlinded          bool // 1 round
    TargetSlowed           bool // 1 round
    SuppressRegeneration   bool // 1 round
    IgnoreMetalArmorAC     bool
    IgnoreAllArmorAC       bool
    // Save bonuses and FP effects are applied outside combat resolution;
    // they are tracked on PlayerSession and evaluated at check/spend time.
}
```

- REQ-GA-22: `ApplyMaterialEffects(affixed []string, ctx AttackContext, reg *Registry) MaterialEffectResult` MUST be the single entry point for resolving all affixed material effects. It iterates `affixed`, looks up each material+grade definition, and accumulates a `MaterialEffectResult`. It MUST be a pure function in `internal/game/inventory/material.go`.
- REQ-GA-23: Bulk reduction from Polymer Frame MUST be floored at 0 — bulk MUST NOT go negative.

### 5.2 Common Materials

#### Scrap Iron — disrupts cyber-augmented enemies

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs cyber-augmented enemies |
| Mil-Spec Grade | +2 damage vs cyber-augmented; target loses 1 AP on hit (1 round) |
| Ghost Grade | +4 damage vs cyber-augmented; target is flat-footed on hit (1 round) |

#### Hollow Point — weakens supernatural entities

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs supernatural enemies |
| Mil-Spec Grade | +2 damage vs supernatural; persistent bleed 1 on hit |
| Ghost Grade | +4 damage vs supernatural; suppresses target regeneration for 1 round |

### 5.3 Uncommon Materials

#### Carbide Alloy — extreme hardness

| Grade | Weapon Effect | Armor Effect |
|---|---|---|
| Street Grade | +2 MaxDurability | *(armor)* +1 Hardness |
| Mil-Spec Grade | +4 MaxDurability; ignore target Hardness ≤ 5 | *(armor)* +2 Hardness |
| Ghost Grade | +6 MaxDurability; ignore target Hardness ≤ 10 | *(armor)* +3 Hardness |

#### Carbon Weave — lightweight composite *(armor only)*

| Grade | Effect |
|---|---|
| Street Grade | −1 check penalty |
| Mil-Spec Grade | −2 check penalty, +5 ft speed |
| Ghost Grade | No check penalty, +10 ft speed |

#### Polymer Frame — undetectable lightweight polymer

| Grade | Effect |
|---|---|
| Street Grade | −1 bulk (floor 0) |
| Mil-Spec Grade | −2 bulk (floor 0); +1 to Stealth checks |
| Ghost Grade | −3 bulk (floor 0); undetectable by metal scanners; +2 to Stealth checks |

#### Thermite Lace — incendiary alloy

| Grade | Effect |
|---|---|
| Street Grade | Persistent fire 1 on hit |
| Mil-Spec Grade | Persistent fire 2 on hit; +1 fire damage |
| Ghost Grade | Persistent fire 4 on hit; +2 fire damage; ignites flammable objects |

#### Cryo-Gel — cryogenic composite

| Grade | Effect |
|---|---|
| Street Grade | Persistent cold 1 on hit |
| Mil-Spec Grade | Persistent cold 2 on hit; target −5 ft speed for 1 round |
| Ghost Grade | Persistent cold 4 on hit; target gains slowed for 1 round |

### 5.4 Rare Materials

#### Quantum Alloy — time-reactive metal

| Grade | Effect |
|---|---|
| Street Grade | Once per combat: reroll 1 attack roll (keep second result) |
| Mil-Spec Grade | Once per combat: reroll 1 save (keep second result); +1 initiative |
| Ghost Grade | Once per combat: reroll any check (keep second result); +2 initiative |

#### Rad-Core — radioactive shard

| Grade | Effect |
|---|---|
| Street Grade | Persistent radiation 1 on hit; carrier takes 1 radiation damage/hour |
| Mil-Spec Grade | Persistent radiation 2 on hit; carrier takes 2 radiation damage/hour; *(armor)* +1 AC vs energy attacks |
| Ghost Grade | Persistent radiation 4 on hit; inflicts irradiated condition (1 round); carrier takes 3 radiation damage/hour |

#### Neural Gel — neuro-conductive liquid metal

| Grade | Effect |
|---|---|
| Street Grade | Reduce Focus Point cost of 1 technology per day by 1 (min 1) |
| Mil-Spec Grade | Reduce FP cost twice per day; +1 to technology attack rolls |
| Ghost Grade | Reduce FP cost three times per day; recover +1 FP on Recalibrate |

#### Ghost Steel — phase-shifted alloy *(weapon only)*

| Grade | Effect |
|---|---|
| Street Grade | Ignore target's metal armor AC bonus on first hit per combat |
| Mil-Spec Grade | Ignore metal armor AC bonus on every hit; +1 damage |
| Ghost Grade | Ignore all armor AC bonuses; +2 damage; once per combat make a touch attack |

#### Null-Weave — anti-tech composite

| Grade | Effect |
|---|---|
| Street Grade | +1 to saves vs technology effects |
| Mil-Spec Grade | +2 to saves vs technology effects; immune to 1 tech-applied condition per combat |
| Ghost Grade | +3 to saves vs technology effects; once per combat reflect a technology attack back at attacker (50% chance) |

#### Soul-Guard Alloy — resists mental domination *(armor only)*

| Grade | Effect |
|---|---|
| Street Grade | +1 to saves vs mental conditions; immune to frightened |
| Mil-Spec Grade | +2 to saves vs mental; immune to frightened and confused |
| Ghost Grade | +3 to saves vs mental; immune to all mental conditions; once per day negate a domination effect |

#### Shadow Plate — harms light-aspected entities *(weapon only)*

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs light-aspected entities |
| Mil-Spec Grade | +2 damage vs light-aspected; target dazzled on hit |
| Ghost Grade | +4 damage vs light-aspected; target blinded for 1 round on hit |

#### Radiance Plate — harms shadow-aspected entities *(weapon only)*

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs shadow-aspected entities |
| Mil-Spec Grade | +2 damage vs shadow-aspected; target dazzled on hit |
| Ghost Grade | +4 damage vs shadow-aspected; target blinded for 1 round on hit |

---

## 6. Data Model Changes

### 6.1 ItemDef (`internal/game/inventory/item.go`)

```go
const KindPreciousMaterial = "precious_material"
// validKinds must include KindPreciousMaterial.

// Added to ItemDef:
MaterialID string `yaml:"material_id,omitempty"`
GradeID    string `yaml:"grade_id,omitempty"`
```

### 6.2 WeaponDef and ArmorDef

```go
// Added to WeaponDef and ArmorDef:
UpgradeSlots int `yaml:"-"` // derived from RarityDef.FeatureSlots at load time
```

### 6.3 ItemInstance (`internal/game/inventory/backpack.go`)

```go
// Added to ItemInstance:
AffixedMaterials []string // each entry: "<material_id>:<grade_id>"
```

### 6.4 EquippedWeapon (`internal/game/inventory/preset.go`)

```go
// Added to EquippedWeapon:
AffixedMaterials []string // cached copy from ItemInstance; populated at equip/load time
```

### 6.5 SlottedItem (`internal/game/inventory/equipment.go`)

```go
// Added to SlottedItem:
AffixedMaterials []string // cached copy from ItemInstance; populated at equip/load time
```

### 6.6 AffixSession (`internal/game/command/affix.go`)

`AffixSession` follows the same struct-wrapper pattern as `RepairSession` — it wraps `*session.PlayerSession` so callers do not need a separate type in tests:

```go
type AffixSession struct {
    Session *session.PlayerSession
}
```

`HandleAffix` takes `*AffixSession` and MUST NOT import `internal/game/session` for anything beyond the struct field.

---

## 7. Database Migrations

Migration **046** adds `affixed_materials` to the three tables that store per-instance item state. The column type is `text[]` (PostgreSQL array of strings).

```sql
-- 046_affixed_materials.up.sql

ALTER TABLE character_inventory_instances
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';

ALTER TABLE character_equipment
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';

ALTER TABLE character_weapon_presets
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';
```

```sql
-- 046_affixed_materials.down.sql

ALTER TABLE character_inventory_instances  DROP COLUMN affixed_materials;
ALTER TABLE character_equipment            DROP COLUMN affixed_materials;
ALTER TABLE character_weapon_presets       DROP COLUMN affixed_materials;
```

`character_weapon_presets` stores per-instance weapon state (including `Durability` and `Modifier`) that is copied into `EquippedWeapon` at load time. `AffixedMaterials` is instance-level state and follows the same pattern.

---

## 8. Architecture

- REQ-GA-24: `HandleAffix` MUST be implemented in `internal/game/command/affix.go`. It uses `AffixSession` (Section 6.6) and MUST NOT import `internal/game/session` directly.
- REQ-GA-25: `ApplyMaterialEffects` MUST be a pure function in `internal/game/inventory/material.go`. No DB writes, no side effects.
- REQ-GA-26: Material YAML files MUST be loaded from `content/items/precious_materials/`. Missing files for any of the 45 required material+grade combinations MUST be a fatal load error.
- REQ-GA-27: `KindPreciousMaterial` MUST be added to `validKinds` in `internal/game/inventory/item.go` before any precious material YAML is loaded.
- REQ-GA-28: TDD with property-based tests MUST be used for all new code per SWENG-5/5a.
