# Gear Actions — Design Spec

**Date:** 2026-03-25
**Status:** Draft
**Feature:** `gear-actions` (priority 45)
**Dependencies:** `equipment-mechanics`

---

## Overview

Gear Actions implement the two remaining PF2E gear-category actions: **Repair** and **Affix a Precious Material**. Activate Item and Swap are already complete (see `docs/features/actions.md`).

**Repair** is fully implemented as part of `equipment-mechanics` (REQ-EM-13 through REQ-EM-16). This spec covers the `affix` command only.

---

## 1. Precious Materials

### 1.1 Item Kind

A new item kind `precious_material` is added to `ItemDef.Kind`. Precious material items are carried in the backpack and consumed (or returned on critical success) when affixed.

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

Fifteen materials adapted from PF2E precious materials into the Gunchete world:

| Gunchete Name | PF2E Source | Tier | ID |
|---|---|---|---|
| Scrap Iron | Cold Iron | Common | `scrap_iron` |
| Hollow Point | Silver | Common | `hollow_point` |
| Carbide Alloy | Adamantine | Uncommon | `carbide_alloy` |
| Carbon Weave | Mithral | Uncommon | `carbon_weave` |
| Polymer Frame | Darkwood | Uncommon | `polymer_frame` |
| Thermite Lace | Siccatite (hot) | Uncommon | `thermite_lace` |
| Cryo-Gel | Siccatite (cold) | Uncommon | `cryo_gel` |
| Quantum Alloy | Orichalcum | Rare | `quantum_alloy` |
| Rad-Core | Abysium | Rare | `rad_core` |
| Neural Gel | Djezet | Rare | `neural_gel` |
| Ghost Steel | Inubrix | Rare | `ghost_steel` |
| Null-Weave | Noqual | Rare | `null_weave` |
| Soul-Guard Alloy | Sovereign Steel | Rare | `soul_guard_alloy` |
| Shadow Plate | Sisterstone (Dusk) | Rare | `shadow_plate` |
| Radiance Plate | Sisterstone (Dawn) | Rare | `radiance_plate` |

---

## 2. Upgrade Slots

Upgrade slots are sourced from the existing rarity "feature slots" defined in `equipment-mechanics`:

| Rarity | Upgrade Slots |
|---|---|
| Salvage | 0 |
| Street | 1 |
| Mil-Spec | 2 |
| Black Market | 3 |
| Ghost | 4 |

- REQ-GA-1: `WeaponDef` and `ArmorDef` MUST expose `UpgradeSlots int` derived from their rarity's feature slot count at load time.
- REQ-GA-2: `ItemInstance` MUST gain `AffixedMaterials []string` persisted to the database. Each entry is formatted `"<material_id>:<grade_id>"` (e.g., `"carbide_alloy:mil_spec_grade"`).
- REQ-GA-3: The same material ID MUST NOT be affixed twice on the same item instance regardless of grade. Attempting to do so MUST fail with: `"<item name> already has <material name> affixed."`
- REQ-GA-4: `len(AffixedMaterials)` MUST NOT exceed `UpgradeSlots`. Attempting to affix when no slots remain MUST fail with: `"<item name> has no upgrade slots remaining."`

---

## 3. Crafting DCs

| Material Tier | Street Grade DC | Mil-Spec Grade DC | Ghost Grade DC |
|---|---|---|---|
| Common | 16 | 21 | 26 |
| Uncommon | 18 | 23 | 28 |
| Rare | 20 | 25 | 30 |

- REQ-GA-5: Crafting check DCs MUST match Section 3 constants. These are immutable game constants, not loaded from YAML.

---

## 4. The `affix` Command

### 4.1 Syntax

```
affix <material_item> <target_item>
```

`<material_item>` matches against the item's ID or name (case-insensitive) in the player's backpack.
`<target_item>` matches against equipped weapon or armor slot item ID or name (case-insensitive).

### 4.2 Preconditions

- REQ-GA-6: `affix` MUST be rejected in combat with: `"You cannot affix materials during combat."`
- REQ-GA-7: `affix` MUST fail if the material item is not found in the player's backpack with: `"You don't have <material name> in your pack."`
- REQ-GA-8: `affix` MUST fail if the target item is not found in equipped weapon or armor slots with: `"<item name> is not equipped."`
- REQ-GA-9: REQ-GA-3 and REQ-GA-4 slot checks MUST be applied before the Crafting roll is made.

### 4.3 Resolution

- REQ-GA-10: The Crafting check is `d20 + Crafting modifier` vs the grade DC (Section 3).
- REQ-GA-11: **Critical success** (beat DC by 10 or more): material is affixed, 1 upgrade slot is consumed, material item is returned to backpack (not consumed). Message: `"Exceptional work. <material name> affixed to <item name> — material returned intact."`
- REQ-GA-12: **Success** (meet or beat DC): material is affixed, 1 upgrade slot is consumed, material item is consumed from backpack. Message: `"<material name> affixed to <item name>."`
- REQ-GA-13: **Failure** (miss DC by 1–9): nothing changes, material is not consumed. Message: `"Your hands slip. The material is undamaged but the affix fails."`
- REQ-GA-14: **Critical failure** (miss DC by 10 or more): material item is destroyed (removed from backpack), slot is not consumed. Message: `"You ruin the material. <material name> is destroyed."`

### 4.4 Display

- REQ-GA-15: `inventory` and `loadout` output MUST show affixed materials as a sub-list under the item with used/total slot count:
  ```
  Mil-Spec Pistol [2/3 slots]
    ↳ Carbide Alloy (Street Grade)
    ↳ Hollow Point (Mil-Spec Grade)
  ```
- REQ-GA-16: Items with 0 upgrade slots (Salvage rarity) MUST NOT show slot indicators.

---

## 5. Material Effects

Effects are applied at combat resolution time alongside rarity multipliers and modifiers. All effect functions MUST be pure — no side effects, no DB writes. Armor effects are marked *(armor)*.

### 5.1 Common Materials

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

### 5.2 Uncommon Materials

#### Carbide Alloy — extreme hardness

| Grade | Weapon Effect | Armor Effect |
|---|---|---|
| Street Grade | +2 MaxDurability | +1 Hardness |
| Mil-Spec Grade | +4 MaxDurability; ignore target Hardness ≤ 5 | +2 Hardness |
| Ghost Grade | +6 MaxDurability; ignore target Hardness ≤ 10 | +3 Hardness |

#### Carbon Weave — lightweight composite

| Grade | Effect |
|---|---|
| Street Grade | *(armor)* −1 check penalty |
| Mil-Spec Grade | *(armor)* −2 check penalty, +5 ft speed |
| Ghost Grade | *(armor)* no check penalty, +10 ft speed; *(weapon)* +1 attack roll |

#### Polymer Frame — undetectable lightweight polymer

| Grade | Effect |
|---|---|
| Street Grade | −1 bulk |
| Mil-Spec Grade | −2 bulk; +1 to Stealth checks |
| Ghost Grade | −3 bulk; undetectable by metal scanners; +2 to Stealth checks |

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

### 5.3 Rare Materials

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

#### Ghost Steel — phase-shifted alloy

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

#### Soul-Guard Alloy — resists mental domination

| Grade | Effect |
|---|---|
| Street Grade | +1 to saves vs mental conditions; immune to frightened |
| Mil-Spec Grade | +2 to saves vs mental; immune to frightened and confused |
| Ghost Grade | +3 to saves vs mental; immune to all mental conditions; once per day negate a domination effect |

#### Shadow Plate — harms light-aspected entities

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs light-aspected entities |
| Mil-Spec Grade | +2 damage vs light-aspected; target dazzled on hit |
| Ghost Grade | +4 damage vs light-aspected; target blinded for 1 round on hit |

#### Radiance Plate — harms shadow-aspected entities

| Grade | Effect |
|---|---|
| Street Grade | +1 damage vs shadow-aspected entities |
| Mil-Spec Grade | +2 damage vs shadow-aspected; target dazzled on hit |
| Ghost Grade | +4 damage vs shadow-aspected; target blinded for 1 round on hit |

---

## 6. Data Model Changes

### 6.1 ItemDef

```go
// KindPreciousMaterial is the item kind for precious material items.
const KindPreciousMaterial = "precious_material"

// ItemDef gains:
MaterialID string `yaml:"material_id,omitempty"` // set when Kind == KindPreciousMaterial
GradeID    string `yaml:"grade_id,omitempty"`    // "street_grade" | "mil_spec_grade" | "ghost_grade"
```

### 6.2 ItemInstance

```go
// AffixedMaterials is the list of materials affixed to this item instance.
// Each entry is formatted "<material_id>:<grade_id>".
// len(AffixedMaterials) must not exceed the item's UpgradeSlots.
AffixedMaterials []string
```

### 6.3 Database

`character_inventory_instances` gains:

```sql
ALTER TABLE character_inventory_instances
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';
```

`character_equipment` gains:

```sql
ALTER TABLE character_equipment
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';
```

`character_weapon_presets` gains:

```sql
ALTER TABLE character_weapon_presets
    ADD COLUMN affixed_materials text[] NOT NULL DEFAULT '{}';
```

---

## 7. Architecture

- REQ-GA-17: `HandleAffix` MUST be a pure command handler in `internal/game/command/affix.go`. It MUST NOT import `internal/game/session` directly; it MUST use an `AffixSession` wrapper interface (same pattern as `RepairSession`).
- REQ-GA-18: Material effect resolution MUST be implemented as pure functions in `internal/game/inventory/material.go`. These functions take an `ItemInstance` and return effect values — no DB writes, no side effects.
- REQ-GA-19: `ApplyMaterialEffects(inst *ItemInstance, ctx AttackContext) MaterialEffectResult` MUST be the single entry point for resolving all affixed material effects at combat resolution time.
- REQ-GA-20: Material YAML files MUST be loaded from `content/items/precious_materials/` at startup. Missing files for any registered material ID MUST be a fatal load error.
- REQ-GA-21: TDD with property-based tests MUST be used for all new code per SWENG-5/5a.
