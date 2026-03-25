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

A new item kind `precious_material` is added to `ItemDef.Kind`. It MUST be added to the `validKinds` map in `internal/game/inventory/item.go` so that startup validation accepts precious material YAML files. The `Validate()` error message for an invalid kind MUST be updated to include `precious_material` in the listed valid kinds.

Precious material items are carried in the backpack and consumed (or returned on critical success) when affixed. They are non-stackable (`Stackable: false`, `MaxStack: 1`).

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

Fifteen materials are the canonical required set. Startup validation MUST check that all 45 YAML files (15 materials × 3 grades) exist in `content/items/precious_materials/`. A missing file MUST be a fatal load error. The required material IDs are hardcoded in the loader (same pattern as `requiredConsumableIDs`).

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

The `Applies To` column is enforced at affix time (see REQ-GA-12).

### 1.4 ItemDef Additions

```go
const KindPreciousMaterial = "precious_material"

// Added to ItemDef:
MaterialID string `yaml:"material_id,omitempty"` // required when Kind == KindPreciousMaterial
GradeID    string `yaml:"grade_id,omitempty"`    // required when Kind == KindPreciousMaterial; one of the three grade IDs
```

`ItemDef.Validate()` MUST require non-empty `MaterialID` and `GradeID` when `Kind == KindPreciousMaterial`, and MUST return an error if either is missing or if `GradeID` is not one of `street_grade`, `mil_spec_grade`, `ghost_grade`.

### 1.5 MaterialDef and Registry Extension

`ApplyMaterialEffects` requires material effect definitions at runtime. The existing `inventory.Registry` is extended with a `materials` map:

```go
// MaterialDef holds the static definition of one material at one grade.
type MaterialDef struct {
    MaterialID string // e.g. "carbide_alloy"
    GradeID    string // "street_grade" | "mil_spec_grade" | "ghost_grade"
    Tier       string // "common" | "uncommon" | "rare"
    AppliesTo  []string // "weapon" | "armor"
}
```

- REQ-GA-1: `Registry` MUST gain `RegisterMaterial(d *MaterialDef) error` and `Material(materialID, gradeID string) (*MaterialDef, bool)` methods, following the existing `RegisterItem`/`Item` pattern.
- REQ-GA-2: All 45 `MaterialDef` entries MUST be registered at startup from the YAML files in `content/items/precious_materials/`.

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

- REQ-GA-3: `WeaponDef` and `ArmorDef` MUST gain `UpgradeSlots int` tagged `yaml:"-"`. The loader MUST derive its value from `RarityDef.FeatureSlots` after the `Rarity` field is resolved, following the same pattern as `RarityStatMultiplier`.
- REQ-GA-4: `ItemInstance` (defined in `internal/game/inventory/backpack.go`) MUST gain `AffixedMaterials []string`. Each entry is formatted `"<material_id>:<grade_id>"` (e.g., `"carbide_alloy:mil_spec_grade"`). `DeductDurability` in `durability.go` requires no changes — on destruction the caller removes the item, and `AffixedMaterials` is permanently lost with it (REQ-GA-6).
- REQ-GA-5: `EquippedWeapon` (in `preset.go`) MUST gain `AffixedMaterials []string`. `SlottedItem` (in `equipment.go`) MUST gain `AffixedMaterials []string`. These are cached copies from `ItemInstance.AffixedMaterials`, populated at the same point that `Durability` and `Modifier` are copied: in `newEquippedWeapon` for weapons and in the armor wear path that constructs `SlottedItem`. Both paths MUST copy `AffixedMaterials` from the corresponding `ItemInstance` record.
- REQ-GA-6: When a host item is destroyed (`DeductDurability` returns `Destroyed: true`), all affixed materials are permanently lost. The player receives the standard REQ-EM-10 destruction message only; no additional message for lost materials is required.
- REQ-GA-7: The same material ID MUST NOT be affixed twice on the same item instance regardless of grade. Attempting to do so MUST fail with: `"<item name> already has <material name> affixed."`
- REQ-GA-8: `len(AffixedMaterials)` MUST NOT exceed `UpgradeSlots`. Attempting to affix when no slots remain MUST fail with: `"<item name> has no upgrade slots remaining."`

---

## 3. Crafting DCs

```go
// DC constants — immutable game constants, not loaded from YAML.
const (
    DCCommonStreetGrade    = 16
    DCCommonMilSpecGrade   = 21
    DCCommonGhostGrade     = 26
    DCUncommonStreetGrade  = 18
    DCUncommonMilSpecGrade = 23
    DCUncommonGhostGrade   = 28
    DCRareStreetGrade      = 20
    DCRareMilSpecGrade     = 25
    DCRareGhostGrade       = 30
)
```

| Material Tier | Street Grade DC | Mil-Spec Grade DC | Ghost Grade DC |
|---|---|---|---|
| Common | 16 | 21 | 26 |
| Uncommon | 18 | 23 | 28 |
| Rare | 20 | 25 | 30 |

- REQ-GA-9: Crafting check DCs MUST match the constants above. These MUST be defined as named constants in `internal/game/inventory/material.go`.

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

- REQ-GA-10: `affix` MUST be rejected in combat with: `"You cannot affix materials during combat."`
- REQ-GA-11: `affix` MUST fail if the material item is not found in the player's backpack with: `"You don't have <material name> in your pack."`
- REQ-GA-12: `affix` MUST fail if the target item is not found in equipped weapon or armor slots with: `"<target> is not equipped."`
- REQ-GA-13: `affix` MUST fail if the target item is broken (`Durability == 0`) with: `"You cannot affix materials to broken equipment. Repair it first."`
- REQ-GA-14: `affix` MUST fail if the material's `Applies To` restriction (Section 1.3) does not include the target item's slot type (weapon vs armor) with: `"<material name> cannot be affixed to armor."` or `"<material name> cannot be affixed to weapons."` as appropriate.
- REQ-GA-15: REQ-GA-7 (duplicate material check) and REQ-GA-8 (slot count check) MUST be applied before the Crafting roll.

### 4.3 Crafting Check

- REQ-GA-16: The Crafting check total is computed as:

  ```
  total = d20 + Abilities.Modifier(Abilities.Reasoning) + skillcheck.ProficiencyBonus(Skills["crafting"])
  ```

  where `Abilities` is `PlayerSession.Abilities` (`character.AbilityScores`) and `Skills["crafting"]` is the character's crafting proficiency rank from `PlayerSession.Skills`.

- REQ-GA-17: The outcome is determined by `skillcheck.OutcomeFor(total, dc)` where `dc` is the grade DC constant from Section 3.

### 4.4 Resolution

- REQ-GA-18: **Critical success** (`total >= dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is returned to the player's backpack (not consumed). Message: `"Exceptional work. <material name> affixed to <item name> — material returned intact."`
- REQ-GA-19: **Success** (`dc <= total < dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is consumed from backpack. Message: `"<material name> affixed to <item name>."`
- REQ-GA-20: **Failure** (`dc - 10 <= total < dc`): nothing changes, material is not consumed. Message: `"Your hands slip. The material is undamaged but the affix fails."`
- REQ-GA-21: **Critical failure** (`total < dc - 10`): material item is destroyed (removed from backpack), slot is not consumed. Message: `"You ruin the material. <material name> is destroyed."`

### 4.5 Display

- REQ-GA-22: `inventory` and `loadout` output MUST show affixed materials as a sub-list under the item with used/total slot count:
  ```
  Mil-Spec Pistol [2/3 slots]
    ↳ Carbide Alloy (Street Grade)
    ↳ Hollow Point (Mil-Spec Grade)
  ```
- REQ-GA-23: Items with 0 upgrade slots (Salvage rarity) MUST NOT show slot indicators.
- REQ-GA-24: The `examine` command (which targets room objects and NPCs) is explicitly out of scope for affix display. Only `inventory` and `loadout` are required to show affix information.

---

## 5. Material Effects

### 5.1 Per-Hit Effects (Pure)

Per-hit effects are resolved via a pure function. Armor-only effects are marked *(armor)*; weapon-only effects are unmarked.

```go
// AttackContext carries the per-hit context needed to evaluate per-hit material effects.
type AttackContext struct {
    TargetIsCyberAugmented bool
    TargetIsSupernatural   bool
    TargetIsLightAspected  bool
    TargetIsShadowAspected bool
    TargetIsMetalArmored   bool // true when target's equipped armor includes a metal slot
    IsHit                  bool // false for miss (some effects only apply on hit)
    IsFirstHitThisCombat   bool // used by Ghost Steel street grade
}

// MaterialEffectResult holds all per-hit effect values produced by ApplyMaterialEffects.
// Callers aggregate results across all affixed materials before applying.
type MaterialEffectResult struct {
    DamageBonus          int
    PersistentFireDmg    int
    PersistentColdDmg    int
    PersistentRadDmg     int
    PersistentBleedDmg   int
    CarrierRadDmgPerHour int
    TargetLosesAP        int  // AP penalty applied to target for 1 round
    TargetSpeedPenalty   int  // feet reduction for 1 round
    TargetFlatFooted     bool // 1 round
    TargetDazzled        bool // until next turn
    TargetBlinded        bool // 1 round
    TargetSlowed         bool // 1 round
    SuppressRegeneration bool // 1 round
    IgnoreMetalArmorAC   bool
    IgnoreAllArmorAC     bool
}
```

- REQ-GA-25: `ApplyMaterialEffects(affixed []string, ctx AttackContext, reg *Registry) MaterialEffectResult` MUST be a pure function in `internal/game/inventory/material.go`. It iterates `affixed`, looks up each material+grade via `reg.Material(materialID, gradeID)`, and accumulates a `MaterialEffectResult`. It covers all per-hit effects. Stateful effects (Section 5.2) are NOT handled here.

### 5.2 Stateful Effects (Session-Tracked)

Several materials have once-per-combat or N-per-day effects that require session state. These are NOT part of `MaterialEffectResult` and are NOT handled by `ApplyMaterialEffects`.

```go
// MaterialSessionState tracks per-combat and per-day usage of stateful material effects.
// Stored on PlayerSession as MaterialState MaterialSessionState.
type MaterialSessionState struct {
    // CombatUsed is the set of "<material_id>:<grade_id>" keys whose once-per-combat
    // effect has been used in the current combat. Reset at combat end.
    CombatUsed map[string]bool
    // DailyUsed maps "<material_id>:<grade_id>" to the number of times
    // the once-per-day / N-per-day effect has been used today. Reset at daily rollover.
    DailyUsed map[string]int
}
```

- REQ-GA-26: `PlayerSession` MUST gain `MaterialState MaterialSessionState`. `CombatUsed` MUST be reset (set to empty map) at combat end. `DailyUsed` MUST be reset at the daily calendar rollover (same hook as Focus Point refresh).
- REQ-GA-27: The following materials have stateful effects consumed via `MaterialSessionState`:
  - **Quantum Alloy** (all grades): once-per-combat reroll. Key: `"quantum_alloy:<grade>"`. The command handler checks `MaterialState.CombatUsed` before granting the reroll, then marks it used.
  - **Null-Weave ghost grade**: once-per-combat reflect. Key: `"null_weave:ghost_grade"`.
  - **Neural Gel** (all grades): N-per-day FP cost reduction (1/2/3 per day for street/mil-spec/ghost). Key: `"neural_gel:<grade>"`. The FP spend handler checks `DailyUsed[key] < maxUses` before applying the discount, then increments.
  - **Soul-Guard Alloy ghost grade**: once-per-day domination negation. Key: `"soul_guard_alloy:ghost_grade"`.
- REQ-GA-28: `MaterialSessionState` is in-memory only — it is NOT persisted to the database. State resets naturally on server restart (combat state) and at daily rollover (daily state).

### 5.3 Carbide Alloy MaxDurability

Carbide Alloy adds `+N MaxDurability` to weapons at affix time. This bonus is applied persistently: `ItemInstance.MaxDurability` is updated in the database when the material is successfully affixed. The bonus is not recomputed at load time.

### 5.4 Common Materials

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

### 5.5 Uncommon Materials

#### Carbide Alloy — extreme hardness

| Grade | Weapon Effect | Armor Effect |
|---|---|---|
| Street Grade | +2 MaxDurability (persistent) | *(armor)* +1 Hardness |
| Mil-Spec Grade | +4 MaxDurability (persistent); ignore target Hardness ≤ 5 | *(armor)* +2 Hardness |
| Ghost Grade | +6 MaxDurability (persistent); ignore target Hardness ≤ 10 | *(armor)* +3 Hardness |

#### Carbon Weave — lightweight composite *(armor only)*

| Grade | Effect |
|---|---|
| Street Grade | −1 check penalty |
| Mil-Spec Grade | −2 check penalty, +5 ft speed |
| Ghost Grade | No check penalty, +10 ft speed |

#### Polymer Frame — undetectable lightweight polymer

- REQ-GA-29: Bulk reduction from Polymer Frame MUST be floored at 0 — bulk MUST NOT go negative.

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

### 5.6 Rare Materials

#### Quantum Alloy — time-reactive metal *(stateful, see Section 5.2)*

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

#### Neural Gel — neuro-conductive liquid metal *(stateful, see Section 5.2)*

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

#### Null-Weave — anti-tech composite *(ghost grade stateful, see Section 5.2)*

| Grade | Effect |
|---|---|
| Street Grade | +1 to saves vs technology effects |
| Mil-Spec Grade | +2 to saves vs technology effects; immune to 1 tech-applied condition per combat |
| Ghost Grade | +3 to saves vs technology effects; once per combat reflect a technology attack back at attacker (50% chance) |

#### Soul-Guard Alloy — resists mental domination *(armor only; ghost grade stateful, see Section 5.2)*

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
// Validate() error message must include "precious_material" in the kind list.
// Validate() must require non-empty MaterialID and GradeID when Kind == KindPreciousMaterial.

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
AffixedMaterials []string // cached copy; set in newEquippedWeapon from ItemInstance
```

### 6.5 SlottedItem (`internal/game/inventory/equipment.go`)

```go
// Added to SlottedItem:
AffixedMaterials []string // cached copy; set in armor wear path from ItemInstance
```

### 6.6 PlayerSession

```go
// Added to PlayerSession:
MaterialState inventory.MaterialSessionState
```

### 6.7 AffixSession (`internal/game/command/affix.go`)

`AffixSession` follows the exact same struct-wrapper pattern as `RepairSession` in `repair.go`:

```go
type AffixSession struct {
    Session *session.PlayerSession
}
```

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

`character_weapon_presets` stores per-instance weapon state (including `Durability` and `Modifier`) copied into `EquippedWeapon` at load time. `AffixedMaterials` follows the same pattern.

---

## 8. Architecture

- REQ-GA-30: `HandleAffix` MUST be implemented in `internal/game/command/affix.go` using the `AffixSession` struct wrapper (Section 6.7).
- REQ-GA-31: `ApplyMaterialEffects` MUST be a pure function in `internal/game/inventory/material.go` covering per-hit effects only (Section 5.1). Stateful effects (Section 5.2) are handled at their respective call sites (FP spend, save resolution, combat end).
- REQ-GA-32: Material YAML files MUST be loaded from `content/items/precious_materials/`. Missing files for any of the 45 required material+grade combinations MUST be a fatal load error.
- REQ-GA-33: `KindPreciousMaterial` MUST be added to `validKinds` before any precious material YAML is loaded.
- REQ-GA-34: TDD with property-based tests MUST be used for all new code per SWENG-5/5a.
