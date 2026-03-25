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
MaterialID   string   `yaml:"material_id,omitempty"`    // required when Kind == KindPreciousMaterial
GradeID      string   `yaml:"grade_id,omitempty"`       // required when Kind == KindPreciousMaterial; one of the three grade IDs
MaterialName string   `yaml:"material_name,omitempty"`  // display name shared across all grades of this material (e.g. "Carbide Alloy"); required when Kind == KindPreciousMaterial
MaterialTier string   `yaml:"material_tier,omitempty"`  // "common" | "uncommon" | "rare"; required when Kind == KindPreciousMaterial
AppliesTo    []string `yaml:"applies_to,omitempty"`     // ["weapon"], ["armor"], or ["weapon","armor"]; required when Kind == KindPreciousMaterial
```

`ItemDef.Validate()` MUST require non-empty `MaterialID`, `GradeID`, `MaterialName`, and `MaterialTier` when `Kind == KindPreciousMaterial`, and MUST return an error if any is missing, if `GradeID` is not one of `street_grade`, `mil_spec_grade`, `ghost_grade`, if `MaterialTier` is not one of `common`, `uncommon`, `rare`, or if `AppliesTo` is empty or contains a value other than `MaterialAppliesToWeapon` or `MaterialAppliesToArmor`.

### 1.5 MaterialDef and Registry Extension

`ApplyMaterialEffects` requires material effect definitions at runtime. The existing `inventory.Registry` is extended with a `materials` map:

```go
// AppliesTo constants for MaterialDef.AppliesTo values.
const (
    MaterialAppliesToWeapon = "weapon"
    MaterialAppliesToArmor  = "armor"
)

// GradeDisplayNames maps GradeID to the player-facing grade name.
var GradeDisplayNames = map[string]string{
    "street_grade":   "Street Grade",
    "mil_spec_grade": "Mil-Spec Grade",
    "ghost_grade":    "Ghost Grade",
}

// MaterialDef holds the static definition of one material at one grade.
// It is constructed at load time from the corresponding ItemDef YAML fields.
type MaterialDef struct {
    MaterialID string   // from ItemDef.MaterialID, e.g. "carbide_alloy"
    Name       string   // from ItemDef.MaterialName, e.g. "Carbide Alloy"
    GradeID    string   // from ItemDef.GradeID, e.g. "mil_spec_grade"
    GradeName  string   // derived: GradeDisplayNames[GradeID], e.g. "Mil-Spec Grade"
    Tier       string   // from ItemDef.MaterialTier, e.g. "uncommon"
    AppliesTo  []string // from ItemDef.AppliesTo; values MUST be MaterialAppliesToWeapon or MaterialAppliesToArmor
}
```

`MaterialDef` is constructed from `ItemDef` during startup loading: the loader iterates all precious material `ItemDef` entries, constructs a `MaterialDef` by copying `MaterialID`, `MaterialName`→`Name`, `GradeID`, `GradeDisplayNames[GradeID]`→`GradeName`, `MaterialTier`→`Tier`, and `AppliesTo`, then calls `Registry.RegisterMaterial`. `Registry.RegisterMaterial` MUST validate that each `AppliesTo` entry is one of the two constants above; an invalid value MUST be a fatal load error.

`MaterialDef.Name` and `GradeName` are the display names used in all player-facing messages (REQ-GA-7, REQ-GA-18 through REQ-GA-21).

- REQ-GA-1: `Registry` MUST gain `RegisterMaterial(d *MaterialDef) error` and `Material(materialID, gradeID string) (*MaterialDef, bool)` methods, following the existing `RegisterItem`/`Item` pattern. `NewRegistry()` MUST initialize the `materials` map.
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

- REQ-GA-3: `WeaponDef` and `ArmorDef` MUST gain `UpgradeSlots int` tagged `yaml:"-"`. `UpgradeSlots` is exactly equal to `RarityDef.FeatureSlots` for the item's rarity — they are not distinct values. The loader MUST derive its value from `RarityDef.FeatureSlots` after the `Rarity` field is resolved, following the same pattern as `RarityStatMultiplier`. Note: `WeaponDef` already has a `Rarity string` field (confirmed in `internal/game/inventory/weapon.go`) — no new field is needed on `WeaponDef` for rarity.
- REQ-GA-4: `ItemInstance` (defined in `internal/game/inventory/backpack.go`) MUST gain `AffixedMaterials []string`. Each entry is formatted `"<material_id>:<grade_id>"` (e.g., `"carbide_alloy:mil_spec_grade"`). `DeductDurability` in `durability.go` requires no changes — on destruction the caller removes the item, and `AffixedMaterials` is permanently lost with it (REQ-GA-6).
- REQ-GA-5: `EquippedWeapon` (in `preset.go`) MUST gain `AffixedMaterials []string` and `MaterialMaxDurabilityBonus int`. `SlottedItem` (in `equipment.go`) MUST gain `AffixedMaterials []string` and `MaterialMaxDurabilityBonus int`. These are cached copies populated from the DB at load time following the same pattern as `Durability`: `LoadWeaponPresets` in `internal/storage/postgres/character.go` MUST SELECT `affixed_materials` and `material_max_durability_bonus` from `character_weapon_presets` and, after calling `preset.EquipMainHand`/`EquipOffHand`, directly assign the values to `preset.MainHand.AffixedMaterials`, `preset.MainHand.MaterialMaxDurabilityBonus`, etc. The armor wear path in `wear.go` that constructs `SlottedItem` MUST copy these fields from the corresponding `ItemInstance` record. `newEquippedWeapon` itself is NOT modified — the copy happens at the call site in the loader, not inside the constructor.
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
- REQ-GA-11: `affix` MUST fail if the material item is not found in the player's backpack with: `"You don't have <query> in your pack."` where `<query>` is the raw `materialQuery` string passed to `HandleAffix` (no `ItemDef` match exists at this point, so the raw query is used as the item name in the message).
- REQ-GA-12: `affix` MUST search only the **active preset's** main-hand and off-hand weapon slots and the armor slots for the target item (same scope as `findRepairTarget` in `repair.go`). If the target is not found in either slot type, the command MUST fail with: `"<target> is not equipped."`
- REQ-GA-13: `affix` MUST fail if the target item is broken (`Durability == 0`) with: `"You cannot affix materials to broken equipment. Repair it first."`
- REQ-GA-14: `affix` MUST fail if the material's `Applies To` restriction (Section 1.3) does not include the target item's slot type (weapon vs armor) with: `"<material name> cannot be affixed to armor."` or `"<material name> cannot be affixed to weapons."` as appropriate.
- REQ-GA-15: REQ-GA-7 (duplicate material check) and REQ-GA-8 (slot count check) MUST be applied before the Crafting roll.

### 4.3 Crafting Check

- REQ-GA-16: The Crafting check MUST use `skillcheck.Resolve`:

  ```go
  roll := rng.RollD20() // rng is the inventory.Roller passed to HandleAffix
  result := skillcheck.Resolve(
      roll,
      sess.Abilities.Modifier(sess.Abilities.Reasoning), // computes (Reasoning - 10) / 2
      sess.Skills["crafting"],
      dc,
      skillcheck.TriggerDef{},
  )
  ```

  `PlayerSession.Abilities` is of type `character.AbilityScores` (defined in `internal/game/character/model.go`). `Abilities.Modifier(score int) int` returns `(score - 10) / 2`. `Abilities.Reasoning` is the raw Reasoning score (`int`). `sess.Skills["crafting"]` is the proficiency rank string from `PlayerSession.Skills map[string]string` (loaded from `character.Character.Skills`). `dc` is the grade DC constant from Section 3.

- REQ-GA-17: The outcome is `result.Outcome` from `skillcheck.Resolve`; `skillcheck.OutcomeFor` MUST NOT be called directly.

### 4.4 Resolution

- REQ-GA-18: **Critical success** (`total >= dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is returned to the player's backpack (not consumed). Message: `"Exceptional work. <material name> affixed to <item name> — material returned intact."`
- REQ-GA-19: **Success** (`dc <= total < dc + 10`): material is affixed, 1 upgrade slot is consumed, material item is consumed from backpack. Message: `"<material name> affixed to <item name>."`
- REQ-GA-20: **Failure** (`dc - 10 <= total < dc`): nothing changes, material is not consumed. Message: `"Your hands slip. The material is undamaged but the affix fails."`
- REQ-GA-21: **Critical failure** (`total < dc - 10`): material item is destroyed (removed from backpack), slot is not consumed. Message: `"You ruin the material. <material name> is destroyed."`

### 4.5 Display

- REQ-GA-22: `inventory` and `loadout` output MUST show affixed materials as a sub-list under the item with a slot counter for all non-Salvage items (UpgradeSlots > 0), even when no materials are affixed. The counter format is `[N/M slots]` where `N = len(AffixedMaterials)` (slots used) and `M = UpgradeSlots` (slots available):
  ```
  Mil-Spec Pistol [2/3 slots]
    ↳ Carbide Alloy (Street Grade)
    ↳ Hollow Point (Mil-Spec Grade)

  Street Jacket [0/1 slots]
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
    // TargetIsMetalArmored is true when any of the target NPC's equipped armor has IsMetal == true
    // in its ArmorDef. ArmorDef gains IsMetal bool `yaml:"is_metal"` (new optional field, default false).
    // Existing armor YAML files that represent metal armors (chain, plate, etc.) MUST set is_metal: true.
    TargetIsMetalArmored   bool
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
    TargetLosesAP        int  // AP penalty applied to target for 1 round
    TargetSpeedPenalty   int  // feet reduction for 1 round
    TargetFlatFooted     bool // 1 round
    TargetDazzled        bool // until next turn
    TargetBlinded        bool // 1 round
    TargetSlowed         bool // 1 round
    SuppressRegeneration bool // 1 round
    IgnoreMetalArmorAC      bool // true when Ghost Steel mil-spec or ghost grade is affixed to weapon
    IgnoreAllArmorAC        bool // true when Ghost Steel ghost grade is affixed to weapon
    IgnoreHardnessThreshold int  // from Carbide Alloy weapon: ignore target Hardness up to this value
}
```

**Terminology used in this section:**

- **technology attack**: any combat action whose `TechnologyDef.Resolution == "attack"` (see `internal/game/technology/model.go`). The attack roll for such actions is a "technology attack roll."
- **tech-applied condition**: any condition applied as an effect of a `TechnologyDef`-based action (i.e., the condition arises from a technology, not from a Strike or other non-tech source).
- **`on_fire` condition**: a **new** condition to be created at `content/conditions/on_fire.yaml` as part of this implementation. While `on_fire`, the character takes 1 fire damage per round at the start of their turn. A character may spend 1 AP to extinguish the flames and remove the condition. No new DB migration is required — conditions are content YAML files only.
- **`irradiated` condition**: a **new** condition to be created at `content/conditions/irradiated.yaml`. While `irradiated`, the character takes 1 radiation damage per round at the start of their turn. The condition expires at the end of its duration (1 round for Rad-Core ghost grade) or can be removed by applicable medical treatment. No new DB migration is required.

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
  - **Quantum Alloy** (all grades): once-per-combat reroll. Key: `"quantum_alloy:<grade>"`. The reroll is **automatic** — the combat system checks `MaterialState.CombatUsed` before each applicable roll; if the key is absent, the reroll is granted automatically (no player command needed) and the key is marked used. Street grade rerolls attack rolls; mil-spec grade rerolls saves; ghost grade rerolls any check.
  - **Null-Weave mil-spec grade**: once-per-combat condition immunity (immune to 1 tech-applied condition). Key: `"null_weave:mil_spec_grade"`. When a tech-applied condition would be applied and the key is not in `CombatUsed`, the condition is suppressed and the key is marked used. "Tech-applied condition" means any condition applied as an effect of a `TechnologyDef`-based action (Section 5, Terminology).
  - **Null-Weave ghost grade**: once-per-combat reflect. Key: `"null_weave:ghost_grade"`. When a technology attack targets the character and the key is not in `CombatUsed`: call `rng.RollFloat()`. If `< 0.5`, the attack reflects — the attacker takes the **damage that would have been dealt to the defending player** (i.e., the damage already computed for that attack). The attack is treated as a miss for the defending player and a hit for the attacker. Mark used. Message to attacker: `"Your attack reflects back at you!"`.
  - **Ghost Steel ghost grade**: once-per-combat touch attack. Key: `"ghost_steel:ghost_grade"`. The player declares a touch attack by prefixing the strike command: `touch strike <target>`. On a touch attack, the target's AC is computed as `10 + target.DexModifier` (all armor and shield bonuses ignored). Costs no additional AP. Mark `CombatUsed` on use. If the key is already in `CombatUsed`, the `touch` prefix is ignored and a normal strike is performed with message: `"Your Ghost Steel phase-shift is spent for this combat."`.
  - **Ghost Steel street grade**: `AttackContext.IsFirstHitThisCombat` is derived from `PlayerSession.HasHitThisCombat bool` — a new boolean field on `PlayerSession` that is `false` at combat start and set to `true` after the first resolved attack. `AttackContext.IsFirstHitThisCombat` is set to `!sess.HasHitThisCombat` before each attack. The grpc combat handler sets `sess.HasHitThisCombat = true` after the first attack resolves. `HasHitThisCombat` is reset to `false` at combat end. No `CombatUsed` entry is needed.
  - **Neural Gel** (all grades): N-per-day FP cost reduction (1/2/3 per day for street/mil-spec/ghost). Key: `"neural_gel:<grade>"`. The FP spend handler checks `DailyUsed[key] < maxUses` before applying the discount, then increments. The discount is applied to the FP cost of the technology: `newCost = max(1, originalCost - 1)`.
  - **Soul-Guard Alloy ghost grade**: once-per-day domination negation. Key: `"soul_guard_alloy:ghost_grade"`. When a domination effect would be applied to the character and `DailyUsed[key] == 0`, the domination is negated and the key is incremented to 1. "Domination" is defined as any condition with `IsDomination: true` in its condition YAML definition.
- REQ-GA-28: `MaterialSessionState` is in-memory only — it is NOT persisted to the database. State resets naturally on server restart (combat state) and at daily rollover (daily state).

### 5.3 Passive Effects (Session-Computed)

Several materials have passive effects (armor check penalty reduction, speed bonuses, save bonuses, immunities, initiative bonuses, bulk reduction, Stealth bonuses) that do not fit the per-hit `MaterialEffectResult` model. These are computed into a `PassiveMaterialSummary` at login and whenever equipped items change, using the same pattern as `SetBonusSummary`.

```go
// PassiveMaterialSummary accumulates all passive bonuses from affixed materials
// across all currently equipped items. Stored on PlayerSession.
type PassiveMaterialSummary struct {
    CheckPenaltyReduction int      // from Carbon Weave
    SpeedBonus            int      // from Carbon Weave
    BulkReduction         int      // from Polymer Frame (floor 0 per item, summed across items)
    StealthBonus          int      // from Polymer Frame
    MetalDetectionImmune  bool     // from Polymer Frame ghost grade
    SaveVsTechBonus       int      // from Null-Weave (applies from weapon or armor slot)
    SaveVsMentalBonus     int      // from Soul-Guard Alloy
    ConditionImmunities   []string // from Soul-Guard Alloy (frightened, confused, all mental conditions)
    InitiativeBonus       int      // from Quantum Alloy mil-spec/ghost grade
    TechAttackRollBonus   int      // from Neural Gel mil-spec and ghost grade (+1 for either)
    FPOnRecalibrateBonus  int      // from Neural Gel ghost grade: +1 FP restored on Recalibrate
    HardnessBonus         int      // from Carbide Alloy affixed to armor; added to armor Hardness in damage reduction
    ACVsEnergyBonus       int      // from Rad-Core mil-spec/ghost grade affixed to armor; added to AC vs energy attacks
    CarrierRadDmgPerHour  int      // from Rad-Core affixed to any item: radiation damage dealt to carrier per hour (summed)
}
```

- REQ-GA-29: `PlayerSession` MUST gain `PassiveMaterials PassiveMaterialSummary`. It MUST be recomputed by calling `ComputePassiveMaterials(equipped []*EquippedWeapon, armor map[ArmorSlot]*SlottedItem, reg *Registry) PassiveMaterialSummary` at login and whenever equipped items change (same trigger points as `SetBonusSummary`). `ComputePassiveMaterials` MUST be a pure function in `internal/game/inventory/material.go`. The `equipped` parameter MUST be populated from the **active preset only** — `[]*EquippedWeapon{preset.MainHand, preset.OffHand}` where `preset = sess.LoadoutSet.ActivePreset()` — matching the same scope used by `findRepairTarget`.
- REQ-GA-30: Passive bonuses from `PassiveMaterialSummary` MUST be applied at the following integration points: check penalty in equipment checks, speed in movement computation, save bonuses in save resolution, immunities in condition application, initiative bonus in initiative roll, tech attack roll bonus in technology attack resolution (`TechnologyDef.Resolution == "attack"`), Hardness bonus in damage reduction, ACVsEnergyBonus in energy-damage AC computation. `FPOnRecalibrateBonus` applies to the **Recalibrate action** (the Gunchete equivalent of PF2E Refocus — spending 10 in-game minutes to restore 1 Focus Point): when `FPOnRecalibrateBonus > 0`, the character restores `1 + FPOnRecalibrateBonus` Focus Points instead of 1. The Recalibrate action is not yet implemented; `FPOnRecalibrateBonus` is a placeholder that will be wired up when Recalibrate is added as a downtime action. `CarrierRadDmgPerHour` MUST be applied in the existing hourly zone tick handler (`internal/gameserver/zone_tick.go`) — for each player session with `PassiveMaterials.CarrierRadDmgPerHour > 0`, deal that many radiation damage points to the player per tick.
- REQ-GA-38: `PassiveMaterialSummary.SaveVsTechBonus` accumulates from all equipped items with Null-Weave affixed regardless of whether the item is a weapon or armor. The save bonus is character-level (not slot-restricted).
- REQ-GA-39: After a successful affix (success or critical success), `HandleAffix` MUST recompute `sess.PassiveMaterials` by calling `inventory.ComputePassiveMaterials(...)` with the active preset weapons and all armor slots, then writing the result back to `sess.PassiveMaterials`. `HandleAffix` performs this recomputation directly (not `handleAffix` in `grpc_service.go`), ensuring passive effects are active immediately without requiring a re-login.

### 5.4 Carbide Alloy MaxDurability

Carbide Alloy adds `+N MaxDurability` to weapons at affix time. To avoid permanently mutating the base `MaxDurability` and to support future accounting, the bonus is stored separately:

```go
// Added to ItemInstance:
MaterialMaxDurabilityBonus int // sum of all Carbide Alloy grade bonuses affixed to this item
```

The effective maximum durability is `MaxDurability + MaterialMaxDurabilityBonus`. On success or critical success, `ItemInstance.MaterialMaxDurabilityBonus += carbideGradeBonus` (the grade-specific bonus from the table in Section 5.6). REQ-GA-7 prevents affixing the same material ID twice, so the sum reflects at most one Carbide Alloy grade per item. On destruction, the entire item is removed and both fields are lost — no data integrity issue. The DB migration (Section 7) adds this column.

### 5.5 Common Materials

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

### 5.6 Uncommon Materials

#### Carbide Alloy — extreme hardness

MaxDurability bonus stored in `MaterialMaxDurabilityBonus` (see Section 5.4).

| Grade | Weapon Effect | Armor Effect |
|---|---|---|
| Street Grade | +2 MaterialMaxDurabilityBonus; `IgnoreHardnessThreshold=0` | *(armor)* +1 Hardness (`HardnessBonus+=1` in PassiveMaterials) |
| Mil-Spec Grade | +4 MaterialMaxDurabilityBonus; `IgnoreHardnessThreshold=5` | *(armor)* +2 Hardness (`HardnessBonus+=2`) |
| Ghost Grade | +6 MaterialMaxDurabilityBonus; `IgnoreHardnessThreshold=10` | *(armor)* +3 Hardness (`HardnessBonus+=3`) |

#### Carbon Weave — lightweight composite *(armor only)*

| Grade | Effect |
|---|---|
| Street Grade | −1 check penalty |
| Mil-Spec Grade | −2 check penalty, +5 ft speed |
| Ghost Grade | No check penalty, +10 ft speed |

#### Polymer Frame — undetectable lightweight polymer

- REQ-GA-40: Bulk reduction from Polymer Frame MUST be floored at 0 per item — bulk MUST NOT go negative.

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
| Ghost Grade | Persistent fire 4 on hit; +2 fire damage; target gains `on_fire` condition (1 fire damage per round; removed by spending 1 AP to extinguish) |

#### Cryo-Gel — cryogenic composite

| Grade | Effect |
|---|---|
| Street Grade | Persistent cold 1 on hit |
| Mil-Spec Grade | Persistent cold 2 on hit; target −5 ft speed for 1 round |
| Ghost Grade | Persistent cold 4 on hit; target gains slowed for 1 round |

### 5.7 Rare Materials

#### Quantum Alloy — time-reactive metal *(stateful, see Section 5.2)*

| Grade | Effect |
|---|---|
| Street Grade | Once per combat: reroll 1 attack roll (keep second result) |
| Mil-Spec Grade | Once per combat: reroll 1 save (keep second result); +1 initiative |
| Ghost Grade | Once per combat: reroll any check (keep second result); +2 initiative |

#### Rad-Core — radioactive shard

| Grade | Effect |
|---|---|
| Street Grade | Persistent radiation 1 on hit; `CarrierRadDmgPerHour+=1` (passive — applied by hourly zone tick) |
| Mil-Spec Grade | Persistent radiation 2 on hit; `CarrierRadDmgPerHour+=2`; *(armor)* +1 AC vs energy attacks (`ACVsEnergyBonus+=1` in PassiveMaterials) |
| Ghost Grade | Persistent radiation 4 on hit; inflicts irradiated condition (1 round); `CarrierRadDmgPerHour+=3` |

#### Neural Gel — neuro-conductive liquid metal *(stateful, see Section 5.2)*

| Grade | Effect |
|---|---|
| Street Grade | Reduce Focus Point cost of 1 technology per day by 1 (min 1) |
| Mil-Spec Grade | Reduce FP cost twice per day; +1 to technology attack rolls |
| Ghost Grade | Reduce FP cost three times per day; +1 to technology attack rolls; recover +1 FP on Recalibrate |

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
// Validate() must require non-empty MaterialID, GradeID, MaterialName, MaterialTier, and AppliesTo when Kind == KindPreciousMaterial.

// Added to ItemDef:
MaterialID   string   `yaml:"material_id,omitempty"`    // required when Kind == KindPreciousMaterial
GradeID      string   `yaml:"grade_id,omitempty"`       // required when Kind == KindPreciousMaterial
MaterialName string   `yaml:"material_name,omitempty"`  // display name shared across all grades; required when Kind == KindPreciousMaterial
MaterialTier string   `yaml:"material_tier,omitempty"`  // "common" | "uncommon" | "rare"; required when Kind == KindPreciousMaterial
AppliesTo    []string `yaml:"applies_to,omitempty"`     // required when Kind == KindPreciousMaterial
```

### 6.2 WeaponDef and ArmorDef

```go
// Added to WeaponDef and ArmorDef:
UpgradeSlots int `yaml:"-"` // derived from RarityDef.FeatureSlots at load time

// Added to ArmorDef only:
IsMetal bool `yaml:"is_metal"` // true for metal armor (chain, plate, etc.); used to set AttackContext.TargetIsMetalArmored
```

### 6.3 ItemInstance (`internal/game/inventory/backpack.go`)

```go
// Added to ItemInstance:
AffixedMaterials          []string // each entry: "<material_id>:<grade_id>"
MaterialMaxDurabilityBonus int     // sum of Carbide Alloy grade bonuses affixed; 0 if none
```

### 6.4 EquippedWeapon (`internal/game/inventory/preset.go`)

```go
// Added to EquippedWeapon:
AffixedMaterials          []string // cached copy; set by LoadWeaponPresets after EquipMainHand/EquipOffHand
MaterialMaxDurabilityBonus int     // cached copy; set by LoadWeaponPresets; effective max = MaxDurability + MaterialMaxDurabilityBonus
```

### 6.5 SlottedItem (`internal/game/inventory/equipment.go`)

```go
// Added to SlottedItem:
AffixedMaterials          []string // cached copy; set by armor wear path from ItemInstance
MaterialMaxDurabilityBonus int     // cached copy; set by armor wear path; effective max = MaxDurability + MaterialMaxDurabilityBonus
```

### 6.6 PlayerSession

```go
// Added to PlayerSession:
MaterialState    inventory.MaterialSessionState
PassiveMaterials inventory.PassiveMaterialSummary
HasHitThisCombat bool // Ghost Steel street grade: false at combat start; set true after first resolved attack; reset at combat end
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

Migration **046** adds `affixed_materials` and `material_max_durability_bonus` to the three tables that store per-instance item state. As of spec authoring the current highest migration in `migrations/` is 045.

```sql
-- 046_affixed_materials.up.sql

ALTER TABLE character_inventory_instances
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;

ALTER TABLE character_equipment
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;

ALTER TABLE character_weapon_presets
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;
```

```sql
-- 046_affixed_materials.down.sql

ALTER TABLE character_inventory_instances
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;

ALTER TABLE character_equipment
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;

ALTER TABLE character_weapon_presets
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;
```

`character_weapon_presets` stores per-instance weapon state (including `Durability` and `Modifier`) copied into `EquippedWeapon` at load time. `AffixedMaterials` and `MaterialMaxDurabilityBonus` follow the same pattern.

---

## 8. Architecture

- REQ-GA-31: `HandleAffix(as *AffixSession, reg *Registry, materialQuery, targetQuery string, rng inventory.Roller) string` MUST be implemented in `internal/game/command/affix.go`. `inventory.Roller` is the existing interface defined in `internal/game/inventory/roller.go` (same type used by `HandleRepair`). The return type is `string` (the player-facing message), following the `HandleRepair` pattern. `HandleAffix` modifies in-memory state only — it MUST NOT perform any database writes. Target lookup MUST reuse `findRepairTarget` (defined in `repair.go`) — the same active-preset weapons + all armor slots scope applies to `affix`. The `repairTarget` struct already exposes `weapon *EquippedWeapon` and `armorItem *SlottedItem`, which carry the `AffixedMaterials` field (REQ-GA-5).
- REQ-GA-32: After a successful affix, `HandleAffix` MUST update the in-memory cached `AffixedMaterials` on the live-equipped `EquippedWeapon` or `SlottedItem` directly (same as `HandleRepair` writes back `Durability` to the in-memory struct). This ensures the cache is consistent without requiring a reload.
- REQ-GA-33: The `handleAffix` method in `internal/gameserver/grpc_service.go` MUST persist state after `HandleAffix` returns according to outcome. All three persistence methods are on `s.charSaver` (type `CharacterSaver`, defined at line 92 of `grpc_service.go`):
  - **Critical success**: `s.charSaver.SaveInventory(ctx, characterID, ...)` (material returned to backpack) AND `s.charSaver.SaveWeaponPresets(ctx, characterID, sess.LoadoutSet)` or `s.charSaver.SaveEquipment(ctx, characterID, sess.Equipment)` (target item's `AffixedMaterials` updated).
  - **Success**: `s.charSaver.SaveInventory(ctx, characterID, ...)` AND `s.charSaver.SaveWeaponPresets` or `s.charSaver.SaveEquipment`.
  - **Failure**: no save calls required (nothing changed).
  - **Critical failure**: `s.charSaver.SaveInventory(ctx, characterID, ...)` only (material destroyed from backpack).
  `SaveWeaponPresets` is called when the target is a weapon; `SaveEquipment` when the target is armor. See `internal/storage/postgres/character.go` for the concrete implementations.
- REQ-GA-34: `ApplyMaterialEffects` MUST be a pure function in `internal/game/inventory/material.go` covering per-hit effects only (Section 5.1). Passive effects are handled by `ComputePassiveMaterials` (Section 5.3). Stateful effects (Section 5.2) are handled at their respective call sites.
- REQ-GA-35: Material YAML files MUST be loaded from `content/items/precious_materials/`. Missing files for any of the 45 required material+grade combinations MUST be a fatal load error.
- REQ-GA-36: `KindPreciousMaterial` MUST be added to `validKinds` before any precious material YAML is loaded.
- REQ-GA-37: TDD with property-based tests (SWENG-5/5a) MUST be used for all new code. Required test surfaces include: `ApplyMaterialEffects` (all 15 materials × 3 grades × relevant `AttackContext` combinations); `ComputePassiveMaterials` (all passive-effect materials); `HandleAffix` (all four outcome branches: critical success, success, failure, critical failure; precondition failures from REQ-GA-10 through REQ-GA-15); DC constant lookup (correct DC per tier × grade); `ItemDef.Validate()` precious material validation; `MaterialSessionState` reset paths (combat end, daily rollover); and `material.go` new file creation. `material.go` is a **new file** to be created at `internal/game/inventory/material.go`; it does not yet exist.
