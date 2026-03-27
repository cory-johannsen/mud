# Gear Actions — Affix Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `affix <material> <item>` command, allowing players to permanently upgrade equipped weapons and armor with precious material bonuses, including per-hit effects, passive bonuses, and session-tracked stateful effects.

**Architecture:** Material effect logic is split into three pure/isolated layers: `ApplyMaterialEffects` (pure, per-hit), `ComputePassiveMaterials` (pure, session-level), and `MaterialSessionState` (in-memory, reset on combat end/daily rollover). The `HandleAffix` command follows the `HandleRepair` pattern: pure in-memory mutation, grpc_service handles persistence via `AffixResult.Outcome`.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based tests), PostgreSQL (migration 046), YAML content files, `skillcheck.Resolve`, `inventory.Roller`

**Spec:** `docs/superpowers/specs/2026-03-25-gear-actions-design.md`

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Create | `internal/game/inventory/material.go` | `MaterialDef`, `AttackContext`, `MaterialEffectResult`, `PassiveMaterialSummary`, `MaterialSessionState`, DC constants, `ApplyMaterialEffects`, `ComputePassiveMaterials`, `RegisterMaterial`/`Material` |
| Create | `internal/game/inventory/material_test.go` | Property-based tests for all pure material functions |
| Create | `internal/game/command/affix.go` | `AffixSession`, `AffixResult`, `AffixOutcome`, `HandleAffix` |
| Create | `internal/game/command/affix_test.go` | HandleAffix tests (all branches + preconditions) |
| Create | `content/items/precious_materials/*.yaml` | 45 precious material YAML files (15 materials × 3 grades) |
| Create | `content/conditions/on_fire.yaml` | New on_fire condition |
| Create | `content/conditions/irradiated.yaml` | New irradiated condition |
| Create | `migrations/046_affixed_materials.up.sql` | DB migration: add affixed_materials columns |
| Create | `migrations/046_affixed_materials.down.sql` | DB migration rollback |
| Modify | `internal/game/inventory/item.go` | Add `KindPreciousMaterial`, 5 new `ItemDef` fields, `Validate()` |
| Modify | `internal/game/inventory/registry.go` | Add `materials` map, `RegisterMaterial`, `Material`, init in `NewRegistry` |
| Modify | `internal/game/inventory/weapon.go` | Add `UpgradeSlots int yaml:"-"` to `WeaponDef`, derive in loader |
| Modify | `internal/game/inventory/armor.go` | Add `UpgradeSlots int yaml:"-"` and `IsMetal bool` to `ArmorDef`, derive `UpgradeSlots` in loader |
| Modify | `internal/game/inventory/backpack.go` | Add `AffixedMaterials []string` and `MaterialMaxDurabilityBonus int` to `ItemInstance` |
| Modify | `internal/game/inventory/preset.go` | Add `AffixedMaterials []string` and `MaterialMaxDurabilityBonus int` to `EquippedWeapon` |
| Modify | `internal/game/inventory/equipment.go` | Add `AffixedMaterials []string` and `MaterialMaxDurabilityBonus int` to `SlottedItem` |
| Modify | `internal/game/condition/definition.go` | Add `IsDomination bool yaml:"is_domination"` to `ConditionDef` |
| Modify | `internal/game/session/manager.go` | Add `MaterialState`, `PassiveMaterials`, `HasHitThisCombat` to `PlayerSession` |
| Modify | `internal/game/command/wear.go` | Copy `AffixedMaterials`/`MaterialMaxDurabilityBonus` from `ItemInstance` to `SlottedItem` |
| Modify | `internal/game/command/inventory.go` | Show `[N/M slots]` and affixed material sub-list |
| Modify | `internal/game/command/loadout.go` | Show `[N/M slots]` and affixed material sub-list |
| Modify | `internal/gameserver/grpc_service.go` | Add `handleAffix` dispatcher, compute passive materials at login, reset session state at combat end/daily rollover, `CarrierRadDmgPerHour` in zone tick |
| Modify | `internal/storage/postgres/character.go` | Update `LoadWeaponPresets`, `SaveWeaponPresets`, `SaveEquipment` for new columns |

---

## Task 0: Mark Feature In-Progress

**Files:**
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Set gear-actions status to in_progress**

In `docs/features/index.yaml`, change `gear-actions` status from `planned` to `in_progress`.

- [ ] **Step 2: Commit**

```bash
git add docs/features/index.yaml
git commit -m "feat(gear-actions): mark gear-actions as in_progress"
```

---

## Task 1: DB Migration

**Files:**
- Create: `migrations/046_affixed_materials.up.sql`
- Create: `migrations/046_affixed_materials.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/046_affixed_materials.up.sql

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

- [ ] **Step 2: Create down migration**

```sql
-- migrations/046_affixed_materials.down.sql

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

- [ ] **Step 3: Commit**

```bash
git add migrations/046_affixed_materials.up.sql migrations/046_affixed_materials.down.sql
git commit -m "feat(db): migration 046 — add affixed_materials and material_max_durability_bonus columns"
```

---

## Task 2: Data Model — ItemDef, WeaponDef, ArmorDef, ConditionDef

**Files:**
- Modify: `internal/game/inventory/item.go`
- Modify: `internal/game/inventory/weapon.go`
- Modify: `internal/game/inventory/armor.go`
- Modify: `internal/game/condition/definition.go`

- [ ] **Step 1: Write failing test for ItemDef.Validate() with precious_material kind**

In `internal/game/inventory/item_test.go` (look for existing validate tests and add after them):

```go
func TestItemDef_Validate_PreciousMaterial_Missing_Fields(t *testing.T) {
    // Missing material_id
    d := &ItemDef{ID: "x", Name: "X", Kind: KindPreciousMaterial}
    err := d.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "material_id")
}

func TestItemDef_Validate_PreciousMaterial_InvalidGradeID(t *testing.T) {
    d := &ItemDef{
        ID: "x_street_grade", Name: "X", Kind: KindPreciousMaterial,
        MaterialID: "x", GradeID: "not_valid", MaterialName: "X",
        MaterialTier: "common", AppliesTo: []string{"weapon"},
    }
    err := d.Validate()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "grade_id")
}

func TestItemDef_Validate_PreciousMaterial_Valid(t *testing.T) {
    d := &ItemDef{
        ID: "scrap_iron_street_grade", Name: "Scrap Iron (Street Grade)",
        Kind: KindPreciousMaterial,
        MaterialID: "scrap_iron", GradeID: "street_grade",
        MaterialName: "Scrap Iron", MaterialTier: "common",
        AppliesTo: []string{"weapon"},
    }
    assert.NoError(t, d.Validate())
}
```

Run: `mise run go test ./internal/game/inventory/... -run TestItemDef_Validate_Precious -v`
Expected: FAIL (KindPreciousMaterial undefined)

- [ ] **Step 2: Add KindPreciousMaterial and fields to ItemDef**

In `internal/game/inventory/item.go`:

```go
const KindPreciousMaterial = "precious_material"
```

Add to `validKinds` map:
```go
KindPreciousMaterial: true,
```

Add to `ItemDef` struct:
```go
MaterialID   string   `yaml:"material_id,omitempty"`
GradeID      string   `yaml:"grade_id,omitempty"`
MaterialName string   `yaml:"material_name,omitempty"`
MaterialTier string   `yaml:"material_tier,omitempty"`
AppliesTo    []string `yaml:"applies_to,omitempty"`
```

In `ItemDef.Validate()`, add after the kind check:
```go
if d.Kind == KindPreciousMaterial {
    if d.MaterialID == "" {
        errs = append(errs, fmt.Errorf("material_id is required for precious_material kind"))
    }
    if d.GradeID == "" {
        errs = append(errs, fmt.Errorf("grade_id is required for precious_material kind"))
    } else if d.GradeID != "street_grade" && d.GradeID != "mil_spec_grade" && d.GradeID != "ghost_grade" {
        errs = append(errs, fmt.Errorf("grade_id %q is invalid; must be street_grade, mil_spec_grade, or ghost_grade", d.GradeID))
    }
    if d.MaterialName == "" {
        errs = append(errs, fmt.Errorf("material_name is required for precious_material kind"))
    }
    if d.MaterialTier == "" {
        errs = append(errs, fmt.Errorf("material_tier is required for precious_material kind"))
    } else if d.MaterialTier != "common" && d.MaterialTier != "uncommon" && d.MaterialTier != "rare" {
        errs = append(errs, fmt.Errorf("material_tier %q is invalid; must be common, uncommon, or rare", d.MaterialTier))
    }
    if len(d.AppliesTo) == 0 {
        errs = append(errs, fmt.Errorf("applies_to is required for precious_material kind"))
    }
    for _, at := range d.AppliesTo {
        if at != "weapon" && at != "armor" {
            errs = append(errs, fmt.Errorf("applies_to value %q is invalid; must be weapon or armor", at))
        }
    }
}
```

Also update the Validate() kind error message to include `precious_material` in the list.

- [ ] **Step 3: Run ItemDef tests**

```bash
mise run go test ./internal/game/inventory/... -run TestItemDef -v
```
Expected: all pass

- [ ] **Step 4: Add UpgradeSlots + IsMetal to ArmorDef**

In `internal/game/inventory/armor.go`, add to `ArmorDef` struct:
```go
IsMetal      bool `yaml:"is_metal"`
UpgradeSlots int  `yaml:"-"` // derived from RarityDef.FeatureSlots at load time
```

In `ArmorDef`'s load/validate path (same location as `RarityStatMultiplier` derivation), derive `UpgradeSlots`:
```go
if def, ok := LookupRarity(a.Rarity); ok {
    a.UpgradeSlots = def.FeatureSlots
}
```

- [ ] **Step 5: Add UpgradeSlots to WeaponDef**

In `internal/game/inventory/weapon.go`, add to `WeaponDef` struct:
```go
UpgradeSlots int `yaml:"-"` // derived from RarityDef.FeatureSlots at load time
```

In `WeaponDef`'s loader (same location as `RarityStatMultiplier`):
```go
if def, ok := LookupRarity(w.Rarity); ok {
    w.UpgradeSlots = def.FeatureSlots
}
```

- [ ] **Step 6: Add IsDomination and IsMentalCondition to ConditionDef**

In `internal/game/condition/definition.go`, add to `ConditionDef` struct:
```go
IsDomination      bool `yaml:"is_domination"`
IsMentalCondition bool `yaml:"is_mental_condition"`
```

Also add `IsMentalCondition: true` to the YAML files for all mental conditions (frightened, confused, dominated, and any others) when they are created or updated. The `"*mental"` sentinel produced by `soul_guard_alloy:ghost_grade` in `PassiveMaterialSummary.ConditionImmunities` must be expanded at use time: in `grpc_service.go`, when computing active immunities for a session, replace `"*mental"` with all condition IDs where `IsMentalCondition == true`.

- [ ] **Step 7: Run all fast tests**

```bash
mise run go test -race ./internal/game/inventory/... ./internal/game/condition/... -v
```
Expected: all pass

- [ ] **Step 8: Commit**

```bash
git add internal/game/inventory/item.go internal/game/inventory/weapon.go \
        internal/game/inventory/armor.go internal/game/condition/definition.go
git commit -m "feat(model): add KindPreciousMaterial, UpgradeSlots, IsMetal, IsDomination fields"
```

---

## Task 3: Data Model — ItemInstance, EquippedWeapon, SlottedItem

**Files:**
- Modify: `internal/game/inventory/backpack.go`
- Modify: `internal/game/inventory/preset.go`
- Modify: `internal/game/inventory/equipment.go`

- [ ] **Step 1: Add new fields to ItemInstance**

In `internal/game/inventory/backpack.go`, add to `ItemInstance` struct:
```go
AffixedMaterials          []string // each entry: "<material_id>:<grade_id>"
MaterialMaxDurabilityBonus int     // sum of Carbide Alloy grade bonuses; 0 if none
```

- [ ] **Step 2: Add new fields to EquippedWeapon**

In `internal/game/inventory/preset.go`, add to `EquippedWeapon` struct:
```go
AffixedMaterials          []string // cached copy from DB; set by LoadWeaponPresets
MaterialMaxDurabilityBonus int     // cached copy; effective max = Def MaxDurability + this
```

- [ ] **Step 3: Add new fields to SlottedItem**

In `internal/game/inventory/equipment.go`, add to `SlottedItem` struct:
```go
AffixedMaterials          []string // cached copy from DB; set by armor wear path
MaterialMaxDurabilityBonus int     // cached copy; effective max = base MaxDurability + this
```

- [ ] **Step 4: Run compile check**

```bash
mise run go build ./internal/game/inventory/...
```
Expected: compiles without error

- [ ] **Step 5: Commit**

```bash
git add internal/game/inventory/backpack.go internal/game/inventory/preset.go \
        internal/game/inventory/equipment.go
git commit -m "feat(model): add AffixedMaterials and MaterialMaxDurabilityBonus to ItemInstance, EquippedWeapon, SlottedItem"
```

---

## Task 4: material.go — Core Types, DC Constants, and Registry Extension

**Files:**
- Create: `internal/game/inventory/material.go`
- Create: `internal/game/inventory/material_test.go`
- Modify: `internal/game/inventory/registry.go`

- [ ] **Step 1: Write failing test for DC lookup**

Create `internal/game/inventory/material_test.go`:

```go
package inventory_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestDCConstants_CommonStreetGrade(t *testing.T) {
    assert.Equal(t, 16, inventory.DCCommonStreetGrade)
    assert.Equal(t, 21, inventory.DCCommonMilSpecGrade)
    assert.Equal(t, 26, inventory.DCCommonGhostGrade)
}

func TestDCConstants_UncommonTier(t *testing.T) {
    assert.Equal(t, 18, inventory.DCUncommonStreetGrade)
    assert.Equal(t, 23, inventory.DCUncommonMilSpecGrade)
    assert.Equal(t, 28, inventory.DCUncommonGhostGrade)
}

func TestDCConstants_RareTier(t *testing.T) {
    assert.Equal(t, 20, inventory.DCRareStreetGrade)
    assert.Equal(t, 25, inventory.DCRareMilSpecGrade)
    assert.Equal(t, 30, inventory.DCRareGhostGrade)
}

func TestRegistry_RegisterMaterial_AndLookup(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.MaterialDef{
        MaterialID: "scrap_iron", Name: "Scrap Iron",
        GradeID: "street_grade", GradeName: "Street Grade",
        Tier: "common", AppliesTo: []string{"weapon"},
    }
    err := reg.RegisterMaterial(def)
    assert.NoError(t, err)
    got, ok := reg.Material("scrap_iron", "street_grade")
    assert.True(t, ok)
    assert.Equal(t, "Scrap Iron", got.Name)
}

func TestRegistry_RegisterMaterial_InvalidAppliesTo(t *testing.T) {
    reg := inventory.NewRegistry()
    def := &inventory.MaterialDef{
        MaterialID: "x", Name: "X", GradeID: "street_grade",
        GradeName: "Street Grade", Tier: "common",
        AppliesTo: []string{"shield"}, // invalid
    }
    assert.Error(t, reg.RegisterMaterial(def))
}

func TestRegistry_RegisterMaterial_Property(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        reg := inventory.NewRegistry()
        matID := rapid.StringMatching(`[a-z_]{1,15}`).Draw(rt, "matID")
        gradeID := rapid.SampledFrom([]string{"street_grade", "mil_spec_grade", "ghost_grade"}).Draw(rt, "grade")
        def := &inventory.MaterialDef{
            MaterialID: matID, Name: "X",
            GradeID: gradeID, GradeName: "Grade",
            Tier: "common", AppliesTo: []string{"weapon"},
        }
        err := reg.RegisterMaterial(def)
        rt.Log("registerMaterial err:", err)
        // second register of same key must fail
        err2 := reg.RegisterMaterial(def)
        if err == nil {
            assert.Error(rt, err2, "duplicate registration must fail")
        }
    })
}
```

Run: `mise run go test ./internal/game/inventory/... -run TestDC -v`
Expected: FAIL (undefined)

- [ ] **Step 2: Create material.go with types, DC constants, and empty functions**

Create `internal/game/inventory/material.go`:

```go
package inventory

import (
    "fmt"
    "strings"
)

// DC constants for the crafting check to affix a precious material (REQ-GA-9).
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

// MaterialAppliesToWeapon and MaterialAppliesToArmor are the valid AppliesTo values.
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
// Constructed at load time from the corresponding ItemDef YAML fields.
type MaterialDef struct {
    MaterialID string
    Name       string
    GradeID    string
    GradeName  string
    Tier       string
    AppliesTo  []string
}

// AttackContext carries per-hit context for evaluating material effects.
type AttackContext struct {
    TargetIsCyberAugmented bool
    TargetIsSupernatural   bool
    TargetIsLightAspected  bool
    TargetIsShadowAspected bool
    TargetIsMetalArmored   bool
    IsHit                  bool
    IsFirstHitThisCombat   bool
}

// MaterialEffectResult holds per-hit effect values produced by ApplyMaterialEffects.
type MaterialEffectResult struct {
    DamageBonus              int
    PersistentFireDmg        int
    PersistentColdDmg        int
    PersistentRadDmg         int
    PersistentBleedDmg       int
    TargetLosesAP            int
    TargetSpeedPenalty       int
    TargetFlatFooted         bool
    TargetDazzled            bool
    TargetBlinded            bool
    TargetSlowed             bool
    SuppressRegeneration     bool
    IgnoreMetalArmorAC       bool
    IgnoreAllArmorAC         bool
    IgnoreHardnessThreshold  int
    ApplyOnFireCondition     bool // set by thermite_lace:ghost_grade on hit
    ApplyIrradiatedCondition bool // set by rad_core:ghost_grade on hit
}

// PassiveMaterialSummary accumulates passive bonuses from affixed materials.
type PassiveMaterialSummary struct {
    CheckPenaltyReduction int
    NoCheckPenalty        bool // set by carbon_weave:ghost_grade; caller treats this as zero check penalty regardless of CheckPenaltyReduction
    SpeedBonus            int
    BulkReduction         int
    StealthBonus          int
    MetalDetectionImmune  bool
    SaveVsTechBonus       int
    SaveVsMentalBonus     int
    ConditionImmunities   []string
    InitiativeBonus       int
    TechAttackRollBonus   int
    FPOnRecalibrateBonus  int
    HardnessBonus         int
    ACVsEnergyBonus       int
    CarrierRadDmgPerHour  int
}

// MaterialSessionState tracks per-combat and per-day stateful material effect usage.
type MaterialSessionState struct {
    CombatUsed map[string]bool
    DailyUsed  map[string]int
}

// DCForMaterial returns the crafting DC for the given material tier and grade ID.
// Returns 0 for unknown tier/grade combinations.
func DCForMaterial(tier, gradeID string) int {
    switch tier {
    case "common":
        switch gradeID {
        case "street_grade":
            return DCCommonStreetGrade
        case "mil_spec_grade":
            return DCCommonMilSpecGrade
        case "ghost_grade":
            return DCCommonGhostGrade
        }
    case "uncommon":
        switch gradeID {
        case "street_grade":
            return DCUncommonStreetGrade
        case "mil_spec_grade":
            return DCUncommonMilSpecGrade
        case "ghost_grade":
            return DCUncommonGhostGrade
        }
    case "rare":
        switch gradeID {
        case "street_grade":
            return DCRareStreetGrade
        case "mil_spec_grade":
            return DCRareMilSpecGrade
        case "ghost_grade":
            return DCRareGhostGrade
        }
    }
    return 0
}

// ApplyMaterialEffects accumulates per-hit effects from all affixed materials.
// Pure function — no side effects. Stateful effects are NOT handled here.
//
// Precondition: affixed entries must be formatted "<material_id>:<grade_id>".
// Postcondition: returns the aggregated MaterialEffectResult.
func ApplyMaterialEffects(affixed []string, ctx AttackContext, reg *Registry) MaterialEffectResult {
    var result MaterialEffectResult
    for _, entry := range affixed {
        parts := strings.SplitN(entry, ":", 2)
        if len(parts) != 2 {
            continue
        }
        def, ok := reg.Material(parts[0], parts[1])
        if !ok {
            continue
        }
        applyMaterialEffect(def, ctx, &result)
    }
    return result
}

// applyMaterialEffect applies one material's per-hit effects into result.
func applyMaterialEffect(def *MaterialDef, ctx AttackContext, result *MaterialEffectResult) {
    key := def.MaterialID + ":" + def.GradeID
    switch key {
    // Scrap Iron — disrupts cyber-augmented enemies
    case "scrap_iron:street_grade":
        if ctx.TargetIsCyberAugmented && ctx.IsHit {
            result.DamageBonus += 1
        }
    case "scrap_iron:mil_spec_grade":
        if ctx.TargetIsCyberAugmented && ctx.IsHit {
            result.DamageBonus += 2
            result.TargetLosesAP += 1
        }
    case "scrap_iron:ghost_grade":
        if ctx.TargetIsCyberAugmented && ctx.IsHit {
            result.DamageBonus += 4
            result.TargetFlatFooted = true
        }
    // Hollow Point — weakens supernatural entities
    case "hollow_point:street_grade":
        if ctx.TargetIsSupernatural && ctx.IsHit {
            result.DamageBonus += 1
        }
    case "hollow_point:mil_spec_grade":
        if ctx.TargetIsSupernatural && ctx.IsHit {
            result.DamageBonus += 2
            result.PersistentBleedDmg += 1
        }
    case "hollow_point:ghost_grade":
        if ctx.TargetIsSupernatural && ctx.IsHit {
            result.DamageBonus += 4
            result.SuppressRegeneration = true
        }
    // Carbide Alloy — weapon effects (armor effects are passive, in ComputePassiveMaterials)
    case "carbide_alloy:street_grade":
        result.IgnoreHardnessThreshold = max(result.IgnoreHardnessThreshold, 0)
    case "carbide_alloy:mil_spec_grade":
        result.IgnoreHardnessThreshold = max(result.IgnoreHardnessThreshold, 5)
    case "carbide_alloy:ghost_grade":
        result.IgnoreHardnessThreshold = max(result.IgnoreHardnessThreshold, 10)
    // Thermite Lace — fire damage
    case "thermite_lace:street_grade":
        if ctx.IsHit {
            result.PersistentFireDmg += 1
        }
    case "thermite_lace:mil_spec_grade":
        if ctx.IsHit {
            result.PersistentFireDmg += 2
            result.DamageBonus += 1
        }
    case "thermite_lace:ghost_grade":
        if ctx.IsHit {
            result.PersistentFireDmg += 4
            result.DamageBonus += 2
            result.ApplyOnFireCondition = true
        }
    // Cryo-Gel — cold damage
    case "cryo_gel:street_grade":
        if ctx.IsHit {
            result.PersistentColdDmg += 1
        }
    case "cryo_gel:mil_spec_grade":
        if ctx.IsHit {
            result.PersistentColdDmg += 2
            result.TargetSpeedPenalty += 5
        }
    case "cryo_gel:ghost_grade":
        if ctx.IsHit {
            result.PersistentColdDmg += 4
            result.TargetSlowed = true
        }
    // Rad-Core — radiation damage (carrier damage is passive, handled by ComputePassiveMaterials)
    case "rad_core:street_grade":
        if ctx.IsHit {
            result.PersistentRadDmg += 1
        }
    case "rad_core:mil_spec_grade":
        if ctx.IsHit {
            result.PersistentRadDmg += 2
        }
    case "rad_core:ghost_grade":
        if ctx.IsHit {
            result.PersistentRadDmg += 4
            result.ApplyIrradiatedCondition = true
        }
    // Ghost Steel — ignores armor AC (stateful first-hit handled by caller via ctx)
    case "ghost_steel:street_grade":
        if ctx.IsHit && ctx.IsFirstHitThisCombat && ctx.TargetIsMetalArmored {
            result.IgnoreMetalArmorAC = true
        }
    case "ghost_steel:mil_spec_grade":
        if ctx.IsHit {
            result.IgnoreMetalArmorAC = true
            result.DamageBonus += 1
        }
    case "ghost_steel:ghost_grade":
        if ctx.IsHit {
            result.IgnoreAllArmorAC = true
            result.DamageBonus += 2
        }
    // Shadow Plate — harms light-aspected
    case "shadow_plate:street_grade":
        if ctx.TargetIsLightAspected && ctx.IsHit {
            result.DamageBonus += 1
        }
    case "shadow_plate:mil_spec_grade":
        if ctx.TargetIsLightAspected && ctx.IsHit {
            result.DamageBonus += 2
            result.TargetDazzled = true
        }
    case "shadow_plate:ghost_grade":
        if ctx.TargetIsLightAspected && ctx.IsHit {
            result.DamageBonus += 4
            result.TargetBlinded = true
        }
    // Radiance Plate — harms shadow-aspected
    case "radiance_plate:street_grade":
        if ctx.TargetIsShadowAspected && ctx.IsHit {
            result.DamageBonus += 1
        }
    case "radiance_plate:mil_spec_grade":
        if ctx.TargetIsShadowAspected && ctx.IsHit {
            result.DamageBonus += 2
            result.TargetDazzled = true
        }
    case "radiance_plate:ghost_grade":
        if ctx.TargetIsShadowAspected && ctx.IsHit {
            result.DamageBonus += 4
            result.TargetBlinded = true
        }
    }
}

// ComputePassiveMaterials accumulates passive bonuses from affixed materials on equipped items.
// Pure function. Called at login and whenever equipped items change.
//
// equipped: active preset weapons only ([]*EquippedWeapon{preset.MainHand, preset.OffHand})
// armor: all equipped armor slots (sess.Equipment.Armor)
func ComputePassiveMaterials(equipped []*EquippedWeapon, armor map[ArmorSlot]*SlottedItem, reg *Registry) PassiveMaterialSummary {
    var s PassiveMaterialSummary
    var immunities []string

    // Process weapon slots
    for _, ew := range equipped {
        if ew == nil {
            continue
        }
        for _, entry := range ew.AffixedMaterials {
            parts := strings.SplitN(entry, ":", 2)
            if len(parts) != 2 {
                continue
            }
            def, ok := reg.Material(parts[0], parts[1])
            if !ok {
                continue
            }
            applyPassiveWeapon(def, &s, &immunities)
        }
    }

    // Process armor slots
    for _, si := range armor {
        if si == nil {
            continue
        }
        for _, entry := range si.AffixedMaterials {
            parts := strings.SplitN(entry, ":", 2)
            if len(parts) != 2 {
                continue
            }
            def, ok := reg.Material(parts[0], parts[1])
            if !ok {
                continue
            }
            applyPassiveArmor(def, si, &s, &immunities)
        }
    }

    s.ConditionImmunities = immunities
    return s
}

func applyPassiveWeapon(def *MaterialDef, s *PassiveMaterialSummary, immunities *[]string) {
    key := def.MaterialID + ":" + def.GradeID
    switch key {
    case "null_weave:street_grade":
        s.SaveVsTechBonus += 1
    case "null_weave:mil_spec_grade":
        s.SaveVsTechBonus += 2
    case "null_weave:ghost_grade":
        s.SaveVsTechBonus += 3
    case "quantum_alloy:mil_spec_grade":
        s.InitiativeBonus += 1
    case "quantum_alloy:ghost_grade":
        s.InitiativeBonus += 2
    case "neural_gel:mil_spec_grade":
        s.TechAttackRollBonus = max(s.TechAttackRollBonus, 1)
    case "neural_gel:ghost_grade":
        s.TechAttackRollBonus = max(s.TechAttackRollBonus, 1)
        s.FPOnRecalibrateBonus += 1
    case "rad_core:street_grade":
        s.CarrierRadDmgPerHour += 1
    case "rad_core:mil_spec_grade":
        s.CarrierRadDmgPerHour += 2
    case "rad_core:ghost_grade":
        s.CarrierRadDmgPerHour += 3
    }
}

func applyPassiveArmor(def *MaterialDef, si *SlottedItem, s *PassiveMaterialSummary, immunities *[]string) {
    key := def.MaterialID + ":" + def.GradeID
    switch key {
    case "carbon_weave:street_grade":
        s.CheckPenaltyReduction += 1
    case "carbon_weave:mil_spec_grade":
        s.CheckPenaltyReduction += 2
        s.SpeedBonus += 5
    case "carbon_weave:ghost_grade":
        s.NoCheckPenalty = true
        s.SpeedBonus += 10
    case "polymer_frame:street_grade":
        s.BulkReduction += 1
    case "polymer_frame:mil_spec_grade":
        s.BulkReduction += 2
        s.StealthBonus += 1
    case "polymer_frame:ghost_grade":
        s.BulkReduction += 3
        s.StealthBonus += 2
        s.MetalDetectionImmune = true
    case "carbide_alloy:street_grade":
        s.HardnessBonus += 1
    case "carbide_alloy:mil_spec_grade":
        s.HardnessBonus += 2
    case "carbide_alloy:ghost_grade":
        s.HardnessBonus += 3
    case "null_weave:street_grade":
        s.SaveVsTechBonus += 1
    case "null_weave:mil_spec_grade":
        s.SaveVsTechBonus += 2
    case "null_weave:ghost_grade":
        s.SaveVsTechBonus += 3
    case "quantum_alloy:mil_spec_grade":
        s.InitiativeBonus += 1
    case "quantum_alloy:ghost_grade":
        s.InitiativeBonus += 2
    case "rad_core:street_grade":
        s.CarrierRadDmgPerHour += 1
    case "rad_core:mil_spec_grade":
        s.CarrierRadDmgPerHour += 2
        s.ACVsEnergyBonus += 1
    case "rad_core:ghost_grade":
        s.CarrierRadDmgPerHour += 3
        s.ACVsEnergyBonus += 1
    case "neural_gel:mil_spec_grade":
        s.TechAttackRollBonus = max(s.TechAttackRollBonus, 1)
    case "neural_gel:ghost_grade":
        s.TechAttackRollBonus = max(s.TechAttackRollBonus, 1)
        s.FPOnRecalibrateBonus += 1
    case "soul_guard_alloy:street_grade":
        s.SaveVsMentalBonus += 1
        *immunities = appendIfAbsent(*immunities, "frightened")
    case "soul_guard_alloy:mil_spec_grade":
        s.SaveVsMentalBonus += 2
        *immunities = appendIfAbsent(*immunities, "frightened")
        *immunities = appendIfAbsent(*immunities, "confused")
    case "soul_guard_alloy:ghost_grade":
        s.SaveVsMentalBonus += 3
        // "*mental" is a sentinel: grpc_service expands it to all ConditionDef with IsMentalCondition=true
        *immunities = appendIfAbsent(*immunities, "*mental")
    }
}

func appendIfAbsent(slice []string, val string) []string {
    for _, v := range slice {
        if v == val {
            return slice
        }
    }
    return append(slice, val)
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// MaterialDefFromItemDef constructs a MaterialDef from a precious-material ItemDef.
// Returns an error if the ItemDef is not of KindPreciousMaterial.
func MaterialDefFromItemDef(d *ItemDef) (*MaterialDef, error) {
    if d.Kind != KindPreciousMaterial {
        return nil, fmt.Errorf("inventory: MaterialDefFromItemDef: item %q is not precious_material kind", d.ID)
    }
    gradeName, ok := GradeDisplayNames[d.GradeID]
    if !ok {
        return nil, fmt.Errorf("inventory: MaterialDefFromItemDef: unknown grade_id %q", d.GradeID)
    }
    return &MaterialDef{
        MaterialID: d.MaterialID,
        Name:       d.MaterialName,
        GradeID:    d.GradeID,
        GradeName:  gradeName,
        Tier:       d.MaterialTier,
        AppliesTo:  d.AppliesTo,
    }, nil
}
```

- [ ] **Step 3: Add RegisterMaterial / Material to registry.go**

In `internal/game/inventory/registry.go`, add `materials` field to `Registry` struct:
```go
materials map[string]*MaterialDef // key: "<material_id>:<grade_id>"
```

In `NewRegistry()`, initialize it:
```go
materials: make(map[string]*MaterialDef),
```

Add methods:
```go
// RegisterMaterial adds a MaterialDef to the registry.
//
// Precondition: d must not be nil; d.AppliesTo must contain only "weapon" or "armor".
// Postcondition: returns error on duplicate or invalid AppliesTo; nil on success.
func (r *Registry) RegisterMaterial(d *MaterialDef) error {
    for _, at := range d.AppliesTo {
        if at != MaterialAppliesToWeapon && at != MaterialAppliesToArmor {
            return fmt.Errorf("inventory: Registry.RegisterMaterial: invalid applies_to value %q for material %q", at, d.MaterialID)
        }
    }
    key := d.MaterialID + ":" + d.GradeID
    if _, exists := r.materials[key]; exists {
        return fmt.Errorf("inventory: Registry.RegisterMaterial: material %q already registered", key)
    }
    r.materials[key] = d
    return nil
}

// Material looks up a MaterialDef by material ID and grade ID.
func (r *Registry) Material(materialID, gradeID string) (*MaterialDef, bool) {
    d, ok := r.materials[materialID+":"+gradeID]
    return d, ok
}
```

- [ ] **Step 4: Run material tests**

```bash
mise run go test ./internal/game/inventory/... -run "TestDC|TestRegistry_RegisterMaterial" -v
```
Expected: all pass

- [ ] **Step 5: Write property tests for ApplyMaterialEffects and ComputePassiveMaterials**

Add to `internal/game/inventory/material_test.go`:

```go
func TestApplyMaterialEffects_Property_NoEffectOnMiss(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        reg := buildTestRegistry(rt)
        affixed := rapid.SliceOf(rapid.SampledFrom(allMaterialKeys())).Draw(rt, "affixed")
        ctx := AttackContext{IsHit: false}
        result := inventory.ApplyMaterialEffects(affixed, ctx, reg)
        // On a miss, no damage-on-hit effects should apply
        // (fire/cold/rad/bleed only apply on hit; DamageBonus only on hit for conditional materials)
        assert.Equal(rt, 0, result.PersistentFireDmg)
        assert.Equal(rt, 0, result.PersistentColdDmg)
        assert.Equal(rt, 0, result.PersistentBleedDmg)
        assert.False(rt, result.TargetFlatFooted)
        assert.False(rt, result.TargetSlowed)
    })
}

func TestApplyMaterialEffects_ScrapIron_HitsCyberTarget(t *testing.T) {
    reg := inventory.NewRegistry()
    registerMaterial(t, reg, "scrap_iron", "street_grade", "common", []string{"weapon"})
    registerMaterial(t, reg, "scrap_iron", "mil_spec_grade", "common", []string{"weapon"})
    registerMaterial(t, reg, "scrap_iron", "ghost_grade", "common", []string{"weapon"})

    ctx := inventory.AttackContext{TargetIsCyberAugmented: true, IsHit: true}

    r1 := inventory.ApplyMaterialEffects([]string{"scrap_iron:street_grade"}, ctx, reg)
    assert.Equal(t, 1, r1.DamageBonus)

    r2 := inventory.ApplyMaterialEffects([]string{"scrap_iron:mil_spec_grade"}, ctx, reg)
    assert.Equal(t, 2, r2.DamageBonus)
    assert.Equal(t, 1, r2.TargetLosesAP)

    r3 := inventory.ApplyMaterialEffects([]string{"scrap_iron:ghost_grade"}, ctx, reg)
    assert.Equal(t, 4, r3.DamageBonus)
    assert.True(t, r3.TargetFlatFooted)
}

func TestComputePassiveMaterials_CarbonWeave(t *testing.T) {
    reg := inventory.NewRegistry()
    registerMaterial(t, reg, "carbon_weave", "mil_spec_grade", "uncommon", []string{"armor"})

    si := &inventory.SlottedItem{
        AffixedMaterials: []string{"carbon_weave:mil_spec_grade"},
    }
    armor := map[inventory.ArmorSlot]*inventory.SlottedItem{
        inventory.SlotTorso: si,
    }
    summary := inventory.ComputePassiveMaterials(nil, armor, reg)
    assert.Equal(t, 2, summary.CheckPenaltyReduction)
    assert.Equal(t, 5, summary.SpeedBonus)
}

func TestComputePassiveMaterials_NullWeave_WeaponAndArmor_Accumulate(t *testing.T) {
    reg := inventory.NewRegistry()
    registerMaterial(t, reg, "null_weave", "street_grade", "rare", []string{"weapon", "armor"})

    ew := &inventory.EquippedWeapon{
        AffixedMaterials: []string{"null_weave:street_grade"},
    }
    si := &inventory.SlottedItem{
        AffixedMaterials: []string{"null_weave:street_grade"},
    }
    armor := map[inventory.ArmorSlot]*inventory.SlottedItem{
        inventory.SlotTorso: si,
    }
    summary := inventory.ComputePassiveMaterials([]*inventory.EquippedWeapon{ew}, armor, reg)
    assert.Equal(t, 2, summary.SaveVsTechBonus, "weapon + armor null_weave should accumulate")
}
```

Add helpers at bottom of test file:
```go
func registerMaterial(t *testing.T, reg *inventory.Registry, matID, gradeID, tier string, appliesTo []string) {
    t.Helper()
    gradeName := inventory.GradeDisplayNames[gradeID]
    def := &inventory.MaterialDef{
        MaterialID: matID, Name: matID,
        GradeID: gradeID, GradeName: gradeName,
        Tier: tier, AppliesTo: appliesTo,
    }
    require.NoError(t, reg.RegisterMaterial(def))
}

func allMaterialKeys() []string {
    // Subset for property tests
    return []string{
        "scrap_iron:street_grade", "hollow_point:street_grade",
        "thermite_lace:mil_spec_grade", "cryo_gel:ghost_grade",
    }
}
```

- [ ] **Step 6: Run all material tests**

```bash
mise run go test ./internal/game/inventory/... -run "TestApply|TestCompute|TestDC|TestRegistry_Register" -v
```
Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add internal/game/inventory/material.go internal/game/inventory/material_test.go \
        internal/game/inventory/registry.go
git commit -m "feat(inventory): add material.go with types, DC constants, ApplyMaterialEffects, ComputePassiveMaterials, RegisterMaterial"
```

---

## Task 5: Content — 45 Precious Material YAML Files + Conditions

**Files:**
- Create: `content/items/precious_materials/<material_id>_<grade_id>.yaml` (45 files)
- Create: `content/conditions/on_fire.yaml`
- Create: `content/conditions/irradiated.yaml`

- [ ] **Step 1: Create content/items/precious_materials/ directory and write all 45 YAML files**

Each file follows this template. Write all 45 (15 materials × 3 grades):

```yaml
# content/items/precious_materials/scrap_iron_street_grade.yaml
id: scrap_iron_street_grade
name: "Scrap Iron (Street Grade)"
description: "Cold iron fragments salvaged from junked cyberware. Disrupts cyber-augmented systems on impact."
kind: precious_material
material_id: scrap_iron
grade_id: street_grade
material_name: "Scrap Iron"
material_tier: common
applies_to:
  - weapon
stackable: false
max_stack: 1
value: 50
```

Repeat for all materials. Key values per material per grade:
- `name`: `"<MaterialName> (<GradeName>)"` e.g. `"Carbide Alloy (Mil-Spec Grade)"`
- `material_id`: from the ID column in spec Table 1.3
- `grade_id`: `street_grade` / `mil_spec_grade` / `ghost_grade`
- `material_tier`: from the Tier column
- `applies_to`: from the Applies To column (weapon, armor, or both)
- `value`: 50 (street), 150 (mil-spec), 500 (ghost) across all materials

- [ ] **Step 2: Create on_fire condition**

```yaml
# content/conditions/on_fire.yaml
id: on_fire
name: On Fire
description: "The character is engulfed in flames. Takes 1 fire damage at the start of each turn. Spend 1 AP to extinguish."
duration_type: rounds
max_stacks: 0
ap_reduction: 0
```

- [ ] **Step 3: Create irradiated condition**

```yaml
# content/conditions/irradiated.yaml
id: irradiated
name: Irradiated
description: "Exposed to concentrated radiation. Takes 1 radiation damage at the start of each turn."
duration_type: rounds
max_stacks: 0
```

- [ ] **Step 4: Verify files load (compile + startup check)**

```bash
mise run go build ./...
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add content/items/precious_materials/ content/conditions/on_fire.yaml content/conditions/irradiated.yaml
git commit -m "feat(content): add 45 precious material YAML files and on_fire/irradiated conditions"
```

---

## Task 6: Registry Loader — Precious Materials Startup Validation

**Files:**
- Modify: `internal/game/inventory/item.go`

The `requiredConsumableIDs` slice and `LoadPreciousMaterials` function go in `internal/game/inventory/item.go`, which already defines `requiredConsumableIDs` for consumables (line 155).

- [ ] **Step 1: Add requiredMaterialIDs and loader**

In `internal/game/inventory/item.go`, add after the existing `requiredConsumableIDs` block:

```go
// requiredMaterialIDs lists the 15 required precious material base IDs.
// For each, there must be 3 YAML files (street_grade, mil_spec_grade, ghost_grade).
var requiredMaterialIDs = []string{
    "scrap_iron", "hollow_point", "carbide_alloy", "carbon_weave", "polymer_frame",
    "thermite_lace", "cryo_gel", "quantum_alloy", "rad_core", "neural_gel",
    "ghost_steel", "null_weave", "soul_guard_alloy", "shadow_plate", "radiance_plate",
}

var requiredGradeIDs = []string{"street_grade", "mil_spec_grade", "ghost_grade"}
```

In the precious_materials loader (called after `KindPreciousMaterial` is in `validKinds`):
```go
// Load precious material YAML files and register MaterialDefs.
// Missing files are fatal load errors.
func LoadPreciousMaterials(reg *Registry, dir string) error {
    for _, matID := range requiredMaterialIDs {
        for _, gradeID := range requiredGradeIDs {
            filename := filepath.Join(dir, matID+"_"+gradeID+".yaml")
            data, err := os.ReadFile(filename)
            if err != nil {
                return fmt.Errorf("inventory: LoadPreciousMaterials: required file %q missing: %w", filename, err)
            }
            var def ItemDef
            if err := yaml.Unmarshal(data, &def); err != nil {
                return fmt.Errorf("inventory: LoadPreciousMaterials: parsing %q: %w", filename, err)
            }
            if err := reg.RegisterItem(&def); err != nil {
                return fmt.Errorf("inventory: LoadPreciousMaterials: registering item %q: %w", def.ID, err)
            }
            matDef, err := MaterialDefFromItemDef(&def)
            if err != nil {
                return fmt.Errorf("inventory: LoadPreciousMaterials: building MaterialDef from %q: %w", def.ID, err)
            }
            if err := reg.RegisterMaterial(matDef); err != nil {
                return fmt.Errorf("inventory: LoadPreciousMaterials: registering material %q: %w", matDef.MaterialID+":"+matDef.GradeID, err)
            }
        }
    }
    return nil
}
```

Call `LoadPreciousMaterials(reg, "content/items/precious_materials")` in the startup sequence after items are loaded, before the game starts accepting connections.

- [ ] **Step 3: Run build and startup smoke test**

```bash
mise run go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/game/inventory/registry.go
git commit -m "feat(registry): add LoadPreciousMaterials with required-file validation for 45 material+grade combinations"
```

---

## Task 7: Wear Command — Copy AffixedMaterials to SlottedItem

**Files:**
- Modify: `internal/game/command/wear.go`

When a player equips armor, the `SlottedItem` is constructed. The `ItemInstance` from the backpack carries `AffixedMaterials` and `MaterialMaxDurabilityBonus`.

- [ ] **Step 1: Find where SlottedItem is constructed in wear.go**

```bash
grep -n "SlottedItem\|slotted\|Durability" /home/cjohannsen/src/mud/internal/game/command/wear.go | head -20
```

- [ ] **Step 2: Copy new fields from ItemInstance to SlottedItem**

After the line that sets `slotted.Durability = inst.MaxDurability` (or wherever durability is copied), add:
```go
slotted.AffixedMaterials = inst.AffixedMaterials
slotted.MaterialMaxDurabilityBonus = inst.MaterialMaxDurabilityBonus
```

- [ ] **Step 3: Run compile**

```bash
mise run go build ./internal/game/command/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/game/command/wear.go
git commit -m "feat(wear): copy AffixedMaterials and MaterialMaxDurabilityBonus from ItemInstance to SlottedItem"
```

---

## Task 8: Storage — LoadWeaponPresets and SaveWeaponPresets/SaveEquipment

**Files:**
- Modify: `internal/storage/postgres/character.go`

- [ ] **Step 1: Update LoadWeaponPresets to SELECT new columns**

In `LoadWeaponPresets`, update the query to also select `affixed_materials` and `material_max_durability_bonus`:

```go
rows, err := r.db.Query(ctx, `
    SELECT preset_index, slot, item_def_id, ammo_count,
           affixed_materials, material_max_durability_bonus
    FROM character_weapon_presets
    WHERE character_id = $1
    ORDER BY preset_index, slot`,
    characterID,
)
```

Update the `rows.Scan` call:
```go
var affixedMaterials []string
var materialMaxDurBonus int
if err := rows.Scan(&presetIdx, &slot, &itemDefID, &ammoCount,
    &affixedMaterials, &materialMaxDurBonus); err != nil {
    ...
}
```

After `preset.EquipMainHand(def)` or `preset.EquipOffHand(def)`, assign the fields:
```go
case "main_hand":
    if equipErr := preset.EquipMainHand(def); equipErr != nil { ... }
    preset.MainHand.AffixedMaterials = affixedMaterials
    preset.MainHand.MaterialMaxDurabilityBonus = materialMaxDurBonus
case "off_hand":
    if equipErr := preset.EquipOffHand(def); equipErr != nil { ... }
    preset.OffHand.AffixedMaterials = affixedMaterials
    preset.OffHand.MaterialMaxDurabilityBonus = materialMaxDurBonus
```

- [ ] **Step 2: Update SaveWeaponPresets to INSERT new columns**

Find the INSERT statement in `SaveWeaponPresets` and add the new columns:
```go
INSERT INTO character_weapon_presets
    (character_id, preset_index, slot, item_def_id, ammo_count,
     affixed_materials, material_max_durability_bonus)
VALUES ($1, $2, $3, $4, $5, $6, $7)
```

Pass `ew.AffixedMaterials` and `ew.MaterialMaxDurabilityBonus` as the additional args. (If `AffixedMaterials` is nil, use `[]string{}`.)

- [ ] **Step 3: Update SaveEquipment to write new columns**

Find the INSERT/UPDATE in `SaveEquipment` and add the new columns similarly:
```go
INSERT INTO character_equipment
    (..., affixed_materials, material_max_durability_bonus)
VALUES (..., $N, $N+1)
```

Also update `LoadEquipment` to read the new columns. Change the `SELECT` query from:
```sql
SELECT slot, item_def_id
FROM character_equipment
WHERE character_id = $1
```
to:
```sql
SELECT slot, item_def_id, affixed_materials, material_max_durability_bonus
FROM character_equipment
WHERE character_id = $1
```
Change the scan from `rows.Scan(&slot, &itemDefID)` to:
```go
var slot, itemDefID string
var affixedMaterials []string
var materialMaxDurBonus int
if err := rows.Scan(&slot, &itemDefID, &affixedMaterials, &materialMaxDurBonus); err != nil {
    return nil, fmt.Errorf("scanning equipment row: %w", err)
}
item := &inventory.SlottedItem{
    ItemDefID:                itemDefID,
    Name:                     itemDefID,
    AffixedMaterials:         affixedMaterials,
    MaterialMaxDurabilityBonus: materialMaxDurBonus,
}
```
Also update `SaveEquipment`: change the INSERT for armor slots from 3 columns to 5:
```sql
INSERT INTO character_equipment (character_id, slot, item_def_id, affixed_materials, material_max_durability_bonus)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (character_id, slot)
    DO UPDATE SET item_def_id = EXCLUDED.item_def_id,
                  affixed_materials = EXCLUDED.affixed_materials,
                  material_max_durability_bonus = EXCLUDED.material_max_durability_bonus
```
Pass `item.AffixedMaterials` (use `[]string{}` if nil) and `item.MaterialMaxDurabilityBonus` as the additional args. Apply the same change to the accessories loop.

- [ ] **Step 4: Run postgres tests**

```bash
make test-postgres
```
Expected: all pass

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/character.go
git commit -m "feat(storage): add affixed_materials and material_max_durability_bonus to weapon preset and equipment persistence"
```

---

## Task 9: PlayerSession — New Fields and Reset Hooks

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go` (combat end reset, daily reset, login passive compute)

- [ ] **Step 1: Add new fields to PlayerSession**

In `internal/game/session/manager.go`, add to `PlayerSession` struct:
```go
MaterialState    inventory.MaterialSessionState
PassiveMaterials inventory.PassiveMaterialSummary
HasHitThisCombat bool
```

- [ ] **Step 2: Initialize MaterialState on session creation**

In the `PlayerSession` factory/initializer (wherever `Status: 1` is set), add:
```go
MaterialState: inventory.MaterialSessionState{
    CombatUsed: make(map[string]bool),
    DailyUsed:  make(map[string]int),
},
```

- [ ] **Step 3: Reset MaterialState.CombatUsed and HasHitThisCombat at combat end**

In `grpc_service.go`, find where combat ends (search for `COMBAT_STATUS_IDLE` or equivalent). Add:
```go
sess.MaterialState.CombatUsed = make(map[string]bool)
sess.HasHitThisCombat = false
```

- [ ] **Step 4: Reset MaterialState.DailyUsed at daily rollover**

Find the daily calendar rollover hook (search for Focus Point refresh, likely in `calendar.go` or `zone_tick.go`). Add:
```go
sess.MaterialState.DailyUsed = make(map[string]int)
```

- [ ] **Step 5: Compute PassiveMaterials at login**

In `grpc_service.go`, near where other session state is computed at login (search for `SetBonusSummary` or the login flow), add after weapons/armor are loaded:
```go
if sess.LoadoutSet != nil && sess.Equipment != nil {
    if active := sess.LoadoutSet.ActivePreset(); active != nil {
        equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
        sess.PassiveMaterials = inventory.ComputePassiveMaterials(
            equipped, sess.Equipment.Armor, reg)
    }
}
```

- [ ] **Step 6: Compute PassiveMaterials when equipped items change**

Find all trigger points where `SetBonusSummary` (or equivalent) is recomputed. Add the same `ComputePassiveMaterials` call after each.

- [ ] **Step 7: CarrierRadDmgPerHour in hourly zone tick**

The zone tick machinery lives in `internal/gameserver/grpc_service.go` (the `StartZoneTicks` method). Find the hourly calendar hook — search for `GameHour` or the clock subscriber that handles hourly HP regen. Add rad carrier damage in the same hourly hook, for each session with `PassiveMaterials.CarrierRadDmgPerHour > 0`:

```go
// Inside the hourly tick callback, for each active session uid:
sess, ok := s.sessionMgr.Get(uid)
if !ok {
    continue
}
if sess.PassiveMaterials.CarrierRadDmgPerHour > 0 {
    dmg := sess.PassiveMaterials.CarrierRadDmgPerHour
    sess.CurrentHP -= dmg
    if sess.CurrentHP < 0 {
        sess.CurrentHP = 0
    }
    s.pushMessageToUID(uid, fmt.Sprintf(
        "Your Rad-Core implant irradiates you for %d radiation damage.", dmg))
}
```

(`pushMessageToUID` is defined in `grpc_service_trap.go` — same package, no import needed.)

- [ ] **Step 8: Run build and fast tests**

```bash
mise run go build ./... && make test-fast
```
Expected: all pass

- [ ] **Step 9: Commit**

```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go \
        internal/gameserver/zone_tick.go
git commit -m "feat(session): add MaterialState, PassiveMaterials, HasHitThisCombat to PlayerSession with reset hooks and login computation"
```

---

## Task 10: HandleAffix Command

**Files:**
- Create: `internal/game/command/affix.go`
- Create: `internal/game/command/affix_test.go`

- [ ] **Step 1: Write failing test for combat precondition**

Create `internal/game/command/affix_test.go`:

```go
package command_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestHandleAffix_InCombat(t *testing.T) {
    sess := &session.PlayerSession{Status: 2} // InCombat
    as := &command.AffixSession{Session: sess}
    reg := inventory.NewRegistry()
    result := command.HandleAffix(as, reg, "carbide_alloy_street_grade", "pistol", stubRoller{})
    assert.Contains(t, result.Message, "cannot affix materials during combat")
    assert.Equal(t, command.AffixOutcomeUnspecified, result.Outcome)
}

func TestHandleAffix_MaterialNotInBackpack(t *testing.T) {
    sess := &session.PlayerSession{Status: 1}
    sess.Backpack = inventory.NewBackpack()
    as := &command.AffixSession{Session: sess}
    reg := inventory.NewRegistry()
    result := command.HandleAffix(as, reg, "carbide_alloy_street_grade", "pistol", stubRoller{})
    assert.Contains(t, result.Message, "carbide_alloy_street_grade")
    assert.Contains(t, result.Message, "pack")
}

// stubRoller implements inventory.Roller for testing
type stubRoller struct{ d20 int; floatVal float64 }
func (s stubRoller) Roll(dice string) int { return s.d20 }
func (s stubRoller) RollD20() int        { return s.d20 }
func (s stubRoller) RollFloat() float64  { return s.floatVal }

// buildAffixTestRegistry constructs a registry with the minimal items needed for affix tests.
func buildAffixTestRegistry(t testing.TB) *inventory.Registry {
    t.Helper()
    reg := inventory.NewRegistry()
    // Register scrap_iron (common weapon material) and carbon_weave (common armor-only material)
    for _, def := range []*inventory.ItemDef{
        {
            ID: "scrap_iron_street_grade", Name: "Scrap Iron (Street Grade)",
            Kind: inventory.KindPreciousMaterial,
            MaterialID: "scrap_iron", GradeID: "street_grade",
            MaterialName: "Scrap Iron", MaterialTier: "common",
            AppliesTo: []string{"weapon"},
        },
        {
            ID: "carbon_weave_street_grade", Name: "Carbon Weave (Street Grade)",
            Kind: inventory.KindPreciousMaterial,
            MaterialID: "carbon_weave", GradeID: "street_grade",
            MaterialName: "Carbon Weave", MaterialTier: "common",
            AppliesTo: []string{"armor"},
        },
        {
            ID: "hollow_point_street_grade", Name: "Hollow Point (Street Grade)",
            Kind: inventory.KindPreciousMaterial,
            MaterialID: "hollow_point", GradeID: "street_grade",
            MaterialName: "Hollow Point", MaterialTier: "common",
            AppliesTo: []string{"weapon"},
        },
    } {
        if err := reg.RegisterItem(def); err != nil {
            t.Fatalf("buildAffixTestRegistry: RegisterItem %s: %v", def.ID, err)
        }
        matDef, err := inventory.MaterialDefFromItemDef(def)
        if err != nil {
            t.Fatalf("buildAffixTestRegistry: MaterialDefFromItemDef %s: %v", def.ID, err)
        }
        if err := reg.RegisterMaterial(matDef); err != nil {
            t.Fatalf("buildAffixTestRegistry: RegisterMaterial %s: %v", def.ID, err)
        }
    }
    // Register test weapons: street rarity (1 slot) and mil-spec rarity (2 slots)
    for _, wd := range []*inventory.WeaponDef{
        {ID: "test_pistol_street", Name: "Test Pistol", Rarity: "street", MaxDurability: 10},
        {ID: "test_pistol_mil_spec", Name: "Test Pistol Mil-Spec", Rarity: "mil_spec", MaxDurability: 10},
    } {
        wd.UpgradeSlots = rarityToSlots(wd.Rarity) // helper below
        if err := reg.RegisterWeapon(wd); err != nil {
            t.Fatalf("buildAffixTestRegistry: RegisterWeapon %s: %v", wd.ID, err)
        }
    }
    return reg
}

// rarityToSlots returns the number of upgrade slots for a rarity name used in tests.
func rarityToSlots(rarity string) int {
    switch rarity {
    case "street":
        return 1
    case "mil_spec":
        return 2
    default:
        return 0
    }
}

// makeTestSession returns a minimal PlayerSession suitable for HandleAffix tests.
func makeTestSession() *session.PlayerSession {
    sess := &session.PlayerSession{Status: 1}
    sess.Backpack = inventory.NewBackpack()
    sess.MaterialState = inventory.MaterialSessionState{
        CombatUsed: make(map[string]bool),
        DailyUsed:  make(map[string]int),
    }
    return sess
}

// addMaterialToBackpack adds one unit of the given item def ID to sess.Backpack.
func addMaterialToBackpack(t testing.TB, sess *session.PlayerSession, reg *inventory.Registry, itemDefID string) {
    t.Helper()
    item, ok := reg.Item(itemDefID)
    if !ok {
        t.Fatalf("addMaterialToBackpack: item %q not in registry", itemDefID)
    }
    sess.Backpack.Add(inventory.ItemInstance{ItemDefID: item.ID, Quantity: 1})
}

// equipWeapon adds the given weapon to the session's active preset main hand.
func equipWeapon(t testing.TB, sess *session.PlayerSession, reg *inventory.Registry, weaponDefID string) {
    t.Helper()
    wd := reg.Weapon(weaponDefID)
    if wd == nil {
        t.Fatalf("equipWeapon: weapon %q not in registry", weaponDefID)
    }
    ew := &inventory.EquippedWeapon{
        Def:        wd,
        Durability: wd.MaxDurability,
    }
    preset := inventory.NewWeaponPreset()
    preset.MainHand = ew
    ls := inventory.NewLoadoutSet()
    ls.SetActive(preset)
    sess.LoadoutSet = ls
}

// addMaterialToBackpackRT is addMaterialToBackpack for use inside rapid.Check callbacks.
func addMaterialToBackpackRT(rt *rapid.T, sess *session.PlayerSession, reg *inventory.Registry, itemDefID string) {
    item, ok := reg.Item(itemDefID)
    if !ok {
        rt.Fatalf("addMaterialToBackpackRT: item %q not in registry", itemDefID)
    }
    sess.Backpack.Add(inventory.ItemInstance{ItemDefID: item.ID, Quantity: 1})
}

// equipWeaponRT is equipWeapon for use inside rapid.Check callbacks.
func equipWeaponRT(rt *rapid.T, sess *session.PlayerSession, reg *inventory.Registry, weaponDefID string) {
    wd := reg.Weapon(weaponDefID)
    if wd == nil {
        rt.Fatalf("equipWeaponRT: weapon %q not in registry", weaponDefID)
    }
    ew := &inventory.EquippedWeapon{
        Def:        wd,
        Durability: wd.MaxDurability,
    }
    preset := inventory.NewWeaponPreset()
    preset.MainHand = ew
    ls := inventory.NewLoadoutSet()
    ls.SetActive(preset)
    sess.LoadoutSet = ls
}
```

Run: `mise run go test ./internal/game/command/... -run TestHandleAffix -v`
Expected: FAIL (HandleAffix undefined)

- [ ] **Step 2: Create affix.go with AffixSession, AffixResult, AffixOutcome**

Create `internal/game/command/affix.go`:

```go
package command

import (
    "fmt"
    "strings"

    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/cory-johannsen/mud/internal/game/session"
    "github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// AffixSession provides the player-session view needed by HandleAffix.
//
// Precondition: Session must not be nil.
type AffixSession struct {
    Session *session.PlayerSession
}

// AffixOutcome represents the four possible crafting check outcomes.
type AffixOutcome int

const (
    // AffixOutcomeUnspecified is the zero value; never returned by HandleAffix.
    AffixOutcomeUnspecified    AffixOutcome = iota
    AffixOutcomeCriticalFailure              // total < dc - 10
    AffixOutcomeFailure                      // dc - 10 <= total < dc
    AffixOutcomeSuccess                      // dc <= total < dc + 10
    AffixOutcomeCriticalSuccess              // total >= dc + 10
)

// AffixResult carries the outcome of an affix operation for persistence routing.
type AffixResult struct {
    Message          string
    Outcome          AffixOutcome
    MaterialConsumed bool
    MaterialReturned bool
    TargetIsWeapon   bool
}

// HandleAffix processes the "affix <material> <target>" command.
//
// Precondition: as, reg, rng must be non-nil; materialQuery and targetQuery non-empty.
// Postcondition: returns AffixResult with player-facing message and outcome.
// HandleAffix modifies in-memory state only — it performs no database writes.
func HandleAffix(as *AffixSession, reg *inventory.Registry, materialQuery, targetQuery string, rng inventory.Roller) AffixResult {
    sess := as.Session

    // REQ-GA-10: reject in combat (Status 2 = InCombat)
    if sess.Status == 2 {
        return AffixResult{Message: "You cannot affix materials during combat."}
    }

    // REQ-GA-11: find material in backpack
    matItems := sess.Backpack.FindByItemDefID(materialQuery)
    if len(matItems) == 0 {
        // Also try name match
        for _, inst := range sess.Backpack.Items() {
            item, ok := reg.Item(inst.ItemDefID)
            if ok && strings.EqualFold(item.Name, materialQuery) {
                matItems = append(matItems, inst)
                break
            }
        }
    }
    if len(matItems) == 0 {
        return AffixResult{Message: fmt.Sprintf("You don't have %s in your pack.", materialQuery)}
    }
    matInst := matItems[0]
    matItem, ok := reg.Item(matInst.ItemDefID)
    if !ok || matItem.Kind != inventory.KindPreciousMaterial {
        return AffixResult{Message: fmt.Sprintf("You don't have %s in your pack.", materialQuery)}
    }
    matDef, ok := reg.Material(matItem.MaterialID, matItem.GradeID)
    if !ok {
        return AffixResult{Message: fmt.Sprintf("You don't have %s in your pack.", materialQuery)}
    }

    // REQ-GA-12: find equipped target
    target := findRepairTarget(sess, targetQuery)
    if target == nil {
        return AffixResult{Message: fmt.Sprintf("%s is not equipped.", targetQuery)}
    }

    // Determine target type and current AffixedMaterials
    targetIsWeapon := target.weapon != nil
    var targetAffixed *[]string
    var targetUpgradeSlots int
    var targetName string
    var targetDurability int
    var targetMaxDurBonus *int

    if targetIsWeapon {
        targetAffixed = &target.weapon.AffixedMaterials
        targetUpgradeSlots = target.weapon.Def.UpgradeSlots
        targetName = target.weapon.Def.Name
        targetDurability = target.weapon.Durability
        targetMaxDurBonus = &target.weapon.MaterialMaxDurabilityBonus
    } else {
        armorDef := reg.Armor(target.armorItem.ItemDefID)
        if armorDef == nil {
            return AffixResult{Message: fmt.Sprintf("%s is not equipped.", targetQuery)}
        }
        targetAffixed = &target.armorItem.AffixedMaterials
        targetUpgradeSlots = armorDef.UpgradeSlots
        targetName = target.armorItem.Name
        targetDurability = target.armorItem.Durability
        targetMaxDurBonus = &target.armorItem.MaterialMaxDurabilityBonus
    }

    // REQ-GA-13: broken item check
    if targetDurability == 0 {
        return AffixResult{Message: fmt.Sprintf("You cannot affix materials to broken equipment. Repair it first.")}
    }

    // REQ-GA-14: applies_to restriction
    if targetIsWeapon {
        canApplyToWeapon := false
        for _, at := range matDef.AppliesTo {
            if at == inventory.MaterialAppliesToWeapon {
                canApplyToWeapon = true
            }
        }
        if !canApplyToWeapon {
            return AffixResult{Message: fmt.Sprintf("%s cannot be affixed to weapons.", matDef.Name)}
        }
    } else {
        canApplyToArmor := false
        for _, at := range matDef.AppliesTo {
            if at == inventory.MaterialAppliesToArmor {
                canApplyToArmor = true
            }
        }
        if !canApplyToArmor {
            return AffixResult{Message: fmt.Sprintf("%s cannot be affixed to armor.", matDef.Name)}
        }
    }

    // REQ-GA-7: duplicate material check
    for _, entry := range *targetAffixed {
        parts := strings.SplitN(entry, ":", 2)
        if len(parts) == 2 && parts[0] == matDef.MaterialID {
            return AffixResult{Message: fmt.Sprintf("%s already has %s affixed.", targetName, matDef.Name)}
        }
    }

    // REQ-GA-8: slot count check
    if len(*targetAffixed) >= targetUpgradeSlots {
        return AffixResult{Message: fmt.Sprintf("%s has no upgrade slots remaining.", targetName)}
    }

    // REQ-GA-16: crafting check
    dc := inventory.DCForMaterial(matDef.Tier, matDef.GradeID)
    roll := rng.RollD20()
    result := skillcheck.Resolve(
        roll,
        sess.Abilities.Modifier(sess.Abilities.Reasoning),
        sess.Skills["crafting"],
        dc,
        skillcheck.TriggerDef{},
    )

    affixEntry := matDef.MaterialID + ":" + matDef.GradeID

    switch result.Outcome {
    case skillcheck.OutcomeCriticalSuccess:
        // REQ-GA-18: affix, return material
        *targetAffixed = append(*targetAffixed, affixEntry)
        applyDirectBonuses(matDef, targetIsWeapon, targetMaxDurBonus)
        recomputePassiveMaterials(sess, reg)
        return AffixResult{
            Message:          fmt.Sprintf("Exceptional work. %s affixed to %s — material returned intact.", matDef.Name+" ("+matDef.GradeName+")", targetName),
            Outcome:          AffixOutcomeCriticalSuccess,
            MaterialReturned: true,
            TargetIsWeapon:   targetIsWeapon,
        }
    case skillcheck.OutcomeSuccess:
        // REQ-GA-19: affix, consume material
        *targetAffixed = append(*targetAffixed, affixEntry)
        applyDirectBonuses(matDef, targetIsWeapon, targetMaxDurBonus)
        _ = sess.Backpack.Remove(matInst.InstanceID, 1)
        recomputePassiveMaterials(sess, reg)
        return AffixResult{
            Message:          fmt.Sprintf("%s affixed to %s.", matDef.Name+" ("+matDef.GradeName+")", targetName),
            Outcome:          AffixOutcomeSuccess,
            MaterialConsumed: true,
            TargetIsWeapon:   targetIsWeapon,
        }
    case skillcheck.OutcomeFailure:
        // REQ-GA-20: nothing changes
        return AffixResult{
            Message: "Your hands slip. The material is undamaged but the affix fails.",
            Outcome: AffixOutcomeFailure,
        }
    default: // OutcomeCriticalFailure
        // REQ-GA-21: material destroyed
        _ = sess.Backpack.Remove(matInst.InstanceID, 1)
        return AffixResult{
            Message:          fmt.Sprintf("You ruin the material. %s is destroyed.", matDef.Name+" ("+matDef.GradeName+")"),
            Outcome:          AffixOutcomeCriticalFailure,
            MaterialConsumed: true,
            TargetIsWeapon:   targetIsWeapon,
        }
    }
}

// applyDirectBonuses applies non-passive, non-per-hit bonuses at affix time (e.g. Carbide Alloy MaxDurability).
func applyDirectBonuses(def *inventory.MaterialDef, targetIsWeapon bool, maxDurBonus *int) {
    if def.MaterialID == "carbide_alloy" && targetIsWeapon {
        switch def.GradeID {
        case "street_grade":
            *maxDurBonus += 2
        case "mil_spec_grade":
            *maxDurBonus += 4
        case "ghost_grade":
            *maxDurBonus += 6
        }
    }
}

// recomputePassiveMaterials updates sess.PassiveMaterials after a successful affix (REQ-GA-39).
func recomputePassiveMaterials(sess *session.PlayerSession, reg *inventory.Registry) {
    if sess.LoadoutSet == nil || sess.Equipment == nil {
        return
    }
    active := sess.LoadoutSet.ActivePreset()
    if active == nil {
        return
    }
    equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
    sess.PassiveMaterials = inventory.ComputePassiveMaterials(equipped, sess.Equipment.Armor, reg)
}
```

- [ ] **Step 3: Run combat precondition test**

```bash
mise run go test ./internal/game/command/... -run TestHandleAffix_InCombat -v
```
Expected: PASS

- [ ] **Step 4: Write comprehensive HandleAffix tests**

Add to `affix_test.go`:

```go
func TestHandleAffix_TargetNotEquipped(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "nonexistent_weapon", stubRoller{})
    assert.Contains(t, result.Message, "not equipped")
}

func TestHandleAffix_BrokenTarget(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_street") // durability == 0
    sess.LoadoutSet.ActivePreset().MainHand.Durability = 0
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubRoller{})
    assert.Contains(t, result.Message, "broken")
}

func TestHandleAffix_WrongAppliesTo(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    // carbon_weave is armor-only; try to affix to a weapon
    addMaterialToBackpack(t, sess, reg, "carbon_weave_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_street")
    result := command.HandleAffix(as, reg, "carbon_weave_street_grade", "test_pistol_street", stubRoller{})
    assert.Contains(t, result.Message, "cannot be affixed to weapons")
}

func TestHandleAffix_DuplicateMaterial(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_street")
    // Pre-load affixed materials to simulate already affixed
    sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubRoller{})
    assert.Contains(t, result.Message, "already has")
}

func TestHandleAffix_NoSlotsRemaining(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_street") // UpgradeSlots == 1 (Street rarity)
    // Fill the slot
    sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"hollow_point:street_grade"}
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_street", stubRoller{})
    assert.Contains(t, result.Message, "no upgrade slots remaining")
}

func TestHandleAffix_CriticalSuccess_ReturnsMaterial(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_mil_spec") // UpgradeSlots >= 1
    // Roll 20 → critical success (total = 20 + modifier >= dc + 10)
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubRoller{d20: 20})
    assert.Equal(t, command.AffixOutcomeCriticalSuccess, result.Outcome)
    assert.True(t, result.MaterialReturned)
    assert.Contains(t, result.Message, "returned intact")
    // Material still in backpack
    assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 1)
    // AffixedMaterials updated
    assert.Contains(t, sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials, "scrap_iron:street_grade")
}

func TestHandleAffix_Success_ConsumesMaterial(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_mil_spec")
    // Roll 15 → success against DC 16 (common street) — actually this is a failure.
    // Craft DC for common street_grade is 16. With roll 15 and 0 modifier → failure.
    // Use roll 16 for exact success.
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubRoller{d20: 16})
    assert.Equal(t, command.AffixOutcomeSuccess, result.Outcome)
    assert.True(t, result.MaterialConsumed)
    assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 0)
}

func TestHandleAffix_CriticalFailure_DestroysMaterial(t *testing.T) {
    sess := makeTestSession()
    as := &command.AffixSession{Session: sess}
    reg := buildAffixTestRegistry(t)
    addMaterialToBackpack(t, sess, reg, "scrap_iron_street_grade")
    equipWeapon(t, sess, reg, "test_pistol_mil_spec")
    // Roll 1 → total 1, dc=16, dc-10=6. 1 < 6 → critical failure.
    result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubRoller{d20: 1})
    assert.Equal(t, command.AffixOutcomeCriticalFailure, result.Outcome)
    assert.Len(t, sess.Backpack.FindByItemDefID("scrap_iron_street_grade"), 0)
    assert.Contains(t, result.Message, "destroyed")
}

func TestHandleAffix_Property_OutcomeMatchesDCBounds(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        sess := makeTestSession()
        as := &command.AffixSession{Session: sess}
        reg := buildAffixTestRegistry(rt)
        addMaterialToBackpackRT(rt, sess, reg, "scrap_iron_street_grade")
        equipWeaponRT(rt, sess, reg, "test_pistol_mil_spec")
        d20Roll := rapid.IntRange(1, 20).Draw(rt, "roll")
        result := command.HandleAffix(as, reg, "scrap_iron_street_grade", "test_pistol_mil_spec", stubRoller{d20: d20Roll})
        // Outcome must be one of the valid outcomes
        validOutcomes := map[command.AffixOutcome]bool{
            command.AffixOutcomeCriticalFailure: true,
            command.AffixOutcomeFailure:         true,
            command.AffixOutcomeSuccess:         true,
            command.AffixOutcomeCriticalSuccess: true,
        }
        assert.True(rt, validOutcomes[result.Outcome], "unexpected outcome %d", result.Outcome)
    })
}
```

- [ ] **Step 5: Run all affix tests**

```bash
mise run go test ./internal/game/command/... -run TestHandleAffix -v
```
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/game/command/affix.go internal/game/command/affix_test.go
git commit -m "feat(command): implement HandleAffix with AffixResult, AffixOutcome, all preconditions and resolution branches"
```

---

## Task 11: grpc_service — handleAffix Dispatcher

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

Wire up `handleAffix` as a command dispatcher following the `handleRepair` pattern.

- [ ] **Step 1: Find handleRepair in grpc_service.go**

```bash
grep -n "handleRepair\|case \"repair\"\|\"repair\":" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

- [ ] **Step 2: Add handleAffix method**

Add near the existing `handleRepair` implementation:

```go
// handleAffix processes the "affix <material> <target>" command.
func (s *GrpcService) handleAffix(ctx context.Context, uid string, args []string) (*gamev1.ServerEvent, error) {
    if len(args) < 2 {
        return messageEvent("Usage: affix <material> <item>"), nil
    }
    materialQuery := args[0]
    targetQuery := strings.Join(args[1:], " ")

    sess, ok := s.sessions.Get(uid)
    if !ok {
        return errorEvent("session not found"), nil
    }
    characterID := sess.CharacterID

    as := &command.AffixSession{Session: sess}
    result := command.HandleAffix(as, s.registry, materialQuery, targetQuery, &diceRoller{})

    // Persist based on outcome (REQ-GA-33)
    switch result.Outcome {
    case command.AffixOutcomeCriticalSuccess, command.AffixOutcomeSuccess:
        invItems := backpackToInventoryItems(sess)
        if err := s.charSaver.SaveInventory(ctx, characterID, invItems); err != nil {
            s.logger.Error("handleAffix: SaveInventory failed", zap.Error(err))
        }
        if result.TargetIsWeapon {
            if err := s.charSaver.SaveWeaponPresets(ctx, characterID, sess.LoadoutSet); err != nil {
                s.logger.Error("handleAffix: SaveWeaponPresets failed", zap.Error(err))
            }
        } else {
            if err := s.charSaver.SaveEquipment(ctx, characterID, sess.Equipment); err != nil {
                s.logger.Error("handleAffix: SaveEquipment failed", zap.Error(err))
            }
        }
    case command.AffixOutcomeCriticalFailure:
        invItems := backpackToInventoryItems(sess)
        if err := s.charSaver.SaveInventory(ctx, characterID, invItems); err != nil {
            s.logger.Error("handleAffix: SaveInventory failed", zap.Error(err))
        }
    // AffixOutcomeFailure: nothing changed, no saves needed
    }

    return messageEvent(result.Message), nil
}
```

Add `backpackToInventoryItems` helper (or find existing one):
```go
func backpackToInventoryItems(sess *session.PlayerSession) []inventory.InventoryItem {
    items := sess.Backpack.Items()
    out := make([]inventory.InventoryItem, 0, len(items))
    for _, it := range items {
        out = append(out, inventory.InventoryItem{ItemDefID: it.ItemDefID, Quantity: it.Quantity})
    }
    return out
}
```

- [ ] **Step 3: Register "affix" in the command dispatch switch**

Find the command dispatch (search for `case "repair":` or similar). Add:
```go
case "affix":
    return s.handleAffix(ctx, uid, args)
```

- [ ] **Step 4: Run build**

```bash
mise run go build ./internal/gameserver/...
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(grpc): add handleAffix dispatcher with persistence routing via AffixResult.Outcome"
```

---

## Task 12: Display — inventory and loadout Commands

**Files:**
- Modify: `internal/game/command/inventory.go`
- Modify: `internal/game/command/loadout.go`

- [ ] **Step 1: Find where weapon and armor items are rendered in inventory.go**

```bash
grep -n "Durability\|UpgradeSlots\|AffixedMaterials\|func.*Inventory" \
    /home/cjohannsen/src/mud/internal/game/command/inventory.go | head -20
```

- [ ] **Step 2: Add slot counter and affixed material sub-list for weapons**

In the weapon display section, after the item name/durability line, add:
```go
if ew.Def.UpgradeSlots > 0 {
    sb.WriteString(fmt.Sprintf(" [%d/%d slots]", len(ew.AffixedMaterials), ew.Def.UpgradeSlots))
}
// ... existing line break
for _, entry := range ew.AffixedMaterials {
    parts := strings.SplitN(entry, ":", 2)
    if len(parts) == 2 {
        if def, ok := reg.Material(parts[0], parts[1]); ok {
            sb.WriteString(fmt.Sprintf("\n    ↳ %s (%s)", def.Name, def.GradeName))
        }
    }
}
```

- [ ] **Step 3: Repeat for armor items and for loadout.go**

Apply the same pattern to the armor display section in `inventory.go` and the equivalent sections in `loadout.go`.

- [ ] **Step 4: Write display tests**

In `internal/game/command/inventory_test.go` (or create it if absent), add:

```go
func TestInventory_ShowsSlotCounter(t *testing.T) {
    sess := makeTestInventorySession()
    reg := buildInventoryTestRegistry(t)
    // Equip a mil-spec weapon with 1 material already affixed
    equipWeaponForDisplay(t, sess, reg, "test_pistol_mil_spec")
    sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}
    output := command.HandleInventory(&command.InventorySession{Session: sess}, reg)
    assert.Contains(t, output, "[1/2 slots]", "should show used/total slots for mil-spec weapon")
}

func TestInventory_ShowsAffixedMaterialSubList(t *testing.T) {
    sess := makeTestInventorySession()
    reg := buildInventoryTestRegistry(t)
    equipWeaponForDisplay(t, sess, reg, "test_pistol_mil_spec")
    sess.LoadoutSet.ActivePreset().MainHand.AffixedMaterials = []string{"scrap_iron:street_grade"}
    output := command.HandleInventory(&command.InventorySession{Session: sess}, reg)
    assert.Contains(t, output, "Scrap Iron", "should show material name in sub-list")
    assert.Contains(t, output, "Street Grade", "should show grade name in sub-list")
}

func TestInventory_NoSlotCounter_ForZeroUpgradeSlots(t *testing.T) {
    sess := makeTestInventorySession()
    reg := buildInventoryTestRegistry(t)
    equipWeaponForDisplay(t, sess, reg, "test_pistol_salvage") // Salvage has 0 UpgradeSlots
    output := command.HandleInventory(&command.InventorySession{Session: sess}, reg)
    assert.NotContains(t, output, "slots]", "Salvage items should not show a slot counter")
}
```

`makeTestInventorySession`, `buildInventoryTestRegistry`, and `equipWeaponForDisplay` follow the same pattern as the affix test helpers. `test_pistol_salvage` has `UpgradeSlots = 0` (Salvage rarity).

- [ ] **Step 5: Run fast tests**

```bash
make test-fast
```
Expected: all pass

- [ ] **Step 6: Commit**

```bash
git add internal/game/command/inventory.go internal/game/command/loadout.go
git commit -m "feat(display): show [N/M slots] and affixed material sub-list in inventory and loadout commands"
```

---

## Task 13: Full Test Suite

- [ ] **Step 1: Run full test suite**

```bash
make test
```
Expected: all pass

- [ ] **Step 2: Fix any failures**

If any tests fail, diagnose and fix. Do not mark complete until all tests pass.

- [ ] **Step 3: Update feature status**

In `docs/features/index.yaml`, change `gear-actions` status from `in_progress` to `done`.

- [ ] **Step 4: Final commit**

```bash
git add docs/features/index.yaml
git commit -m "feat(gear-actions): mark gear-actions as done in feature index"
```

---

## Task 14: Deploy

- [ ] **Step 1: Deploy to k8s**

```bash
make k8s-redeploy
```

- [ ] **Step 2: Smoke test**

Connect via telnet client, obtain a precious material item, equip a weapon, and run:
```
affix scrap_iron_street_grade <weapon name>
```
Verify success/failure message appears and `loadout` shows the affixed material.
