# Equipment Mechanics Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the item and equipment system with rarity tiers, per-instance durability, item modifiers (tuned/defective/cursed), equipment set bonuses, team-based consumable effectiveness, and six new consumable items (REQ-EM-1 through REQ-EM-45).

**Architecture:** Pure-function core (`durability.go`, `consumable.go`, `set_registry.go`, `rarity.go`) with imperative shell in combat resolvers and command handlers. All dice rolling passes through a `Roller` interface for testability. `ConsumableTarget` interface in `internal/game/inventory` avoids import cycles with `internal/game/session`.

**Tech Stack:** Go 1.22+, `pgregory.net/rapid` for property-based tests, PostgreSQL migrations (`db/migrations/`), YAML content files.

---

## File Map

**New files:**
- `internal/game/inventory/rarity.go` — `RarityDef` constants, `ModifierProbs`, `RarityColor`, `RarityRegistry`
- `internal/game/inventory/roller.go` — `Roller` interface + `DiceRoller` prod implementation
- `internal/game/inventory/durability.go` — `DeductDurability`, `RepairField`, `RepairFull`, `InitDurability`
- `internal/game/inventory/consumable.go` — `ConsumableEffect`, `ConsumableTarget`, `ApplyConsumable`
- `internal/game/inventory/set_registry.go` — `SetDef`, `SetRegistry`, `ActiveBonuses`
- `internal/game/inventory/rarity_test.go`
- `internal/game/inventory/durability_test.go`
- `internal/game/inventory/consumable_test.go`
- `internal/game/inventory/set_registry_test.go`
- `db/migrations/035_equipment_durability.sql`
- `db/migrations/036_weapon_preset_durability.sql`
- `db/migrations/037_inventory_instances.sql`
- `content/consumables/whores_pasta.yaml`
- `content/consumables/poontangesca.yaml`
- `content/consumables/four_loko.yaml`
- `content/consumables/old_english.yaml`
- `content/consumables/penjamin_franklin.yaml`
- `content/consumables/repair_kit.yaml`
- `internal/gameserver/grpc_service_repair.go` — `repair` command handler
- `internal/gameserver/grpc_service_use.go` — `use` command handler (replace stub)

**Modified files:**
- `internal/game/inventory/backpack.go` — add `Durability`, `MaxDurability`, `Modifier`, `CurseRevealed` to `ItemInstance`
- `internal/game/inventory/equipment.go` — add `InstanceID`, `Durability`, `Modifier`, `CurseRevealed` to `SlottedItem`; update `ComputedDefenses` for modifiers, broken items, set bonuses
- `internal/game/inventory/preset.go` — add `InstanceID`, `Durability`, `Modifier` to `EquippedWeapon`
- `internal/game/inventory/item.go` — add `Rarity string`, `Team string`, `Effect *ConsumableEffect` to `ItemDef`; add `RarityColorName()` helper
- `internal/game/inventory/weapon.go` — add `Rarity string` to `WeaponDef`; apply stat multiplier at load
- `internal/game/inventory/armor.go` — add `Rarity string` to `ArmorDef`; apply stat multiplier at load
- `internal/game/inventory/providers.go` — add `SetRegistry` and `ConsumableRegistry` providers
- `internal/game/combat/resolver.go` — weapon durability deduction per attack; armor durability deduction per hit; modifier damage adjustments
- `internal/game/session/player_session.go` — add `Team string`, `SetBonuses SetBonusSummary`
- `internal/gameserver/grpc_service.go` — wire `SetRegistry`, consumable use, `repair` handler

---

## Task 1: RarityDef Constants

**Files:**
- Create: `internal/game/inventory/rarity.go`
- Create: `internal/game/inventory/rarity_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/game/inventory/rarity_test.go
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestRarityRegistry_AllTiersPresent(t *testing.T) {
    for _, id := range []string{"salvage", "street", "mil_spec", "black_market", "ghost"} {
        def, ok := inventory.RarityRegistry[id]
        if !ok {
            t.Fatalf("rarity %q not found in registry", id)
        }
        if def.ID != id {
            t.Errorf("rarity %q has ID %q", id, def.ID)
        }
    }
}

func TestRarityRegistry_StatMultipliers(t *testing.T) {
    cases := map[string]float64{
        "salvage":      1.0,
        "street":       1.2,
        "mil_spec":     1.5,
        "black_market": 1.8,
        "ghost":        2.2,
    }
    for id, want := range cases {
        def := inventory.RarityRegistry[id]
        if def.StatMultiplier != want {
            t.Errorf("rarity %q StatMultiplier = %v, want %v", id, def.StatMultiplier, want)
        }
    }
}

func TestRarityRegistry_MaxDurability(t *testing.T) {
    cases := map[string]int{
        "salvage": 20, "street": 40, "mil_spec": 60, "black_market": 80, "ghost": 100,
    }
    for id, want := range cases {
        def := inventory.RarityRegistry[id]
        if def.MaxDurability != want {
            t.Errorf("rarity %q MaxDurability = %d, want %d", id, def.MaxDurability, want)
        }
    }
}

func TestRarityRegistry_DestructionChance(t *testing.T) {
    cases := map[string]float64{
        "salvage": 0.50, "street": 0.30, "mil_spec": 0.15, "black_market": 0.05, "ghost": 0.01,
    }
    for id, want := range cases {
        def := inventory.RarityRegistry[id]
        if def.DestructionChance != want {
            t.Errorf("rarity %q DestructionChance = %v, want %v", id, def.DestructionChance, want)
        }
    }
}

func TestRarityRegistry_ModifierProbs_SumToOne(t *testing.T) {
    for id, def := range inventory.RarityRegistry {
        sum := def.ModifierProbs.Tuned + def.ModifierProbs.Defective + def.ModifierProbs.Cursed
        if sum > 1.0 {
            t.Errorf("rarity %q modifier probs sum to %v > 1.0", id, sum)
        }
    }
}

func TestRarityColor_AllTiers(t *testing.T) {
    for _, id := range []string{"salvage", "street", "mil_spec", "black_market", "ghost"} {
        color := inventory.RarityColor(id)
        if color == "" {
            t.Errorf("RarityColor(%q) returned empty string", id)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestRarity -v 2>&1 | head -30
```

Expected: FAIL — `inventory.RarityRegistry` undefined

- [ ] **Step 3: Implement rarity.go**

```go
// internal/game/inventory/rarity.go
package inventory

// ModifierProbs holds spawn probability for each modifier type.
// The remainder (1 - Tuned - Defective - Cursed) is the probability of no modifier.
type ModifierProbs struct {
    Tuned     float64
    Defective float64
    Cursed    float64
}

// RarityDef holds immutable constants for a rarity tier.
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

// RarityRegistry maps rarity ID to its definition.
// All values are immutable game constants — not loaded from YAML.
var RarityRegistry = map[string]RarityDef{
    "salvage": {
        ID: "salvage", StatMultiplier: 1.0, FeatureSlots: 0, FeatureEffectiveness: 1.00,
        MinLevel: 0, MaxDurability: 20, DestructionChance: 0.50,
        ModifierProbs: ModifierProbs{Tuned: 0.00, Defective: 0.30, Cursed: 0.10},
    },
    "street": {
        ID: "street", StatMultiplier: 1.2, FeatureSlots: 1, FeatureEffectiveness: 1.10,
        MinLevel: 1, MaxDurability: 40, DestructionChance: 0.30,
        ModifierProbs: ModifierProbs{Tuned: 0.05, Defective: 0.15, Cursed: 0.05},
    },
    "mil_spec": {
        ID: "mil_spec", StatMultiplier: 1.5, FeatureSlots: 2, FeatureEffectiveness: 1.25,
        MinLevel: 5, MaxDurability: 60, DestructionChance: 0.15,
        ModifierProbs: ModifierProbs{Tuned: 0.10, Defective: 0.10, Cursed: 0.03},
    },
    "black_market": {
        ID: "black_market", StatMultiplier: 1.8, FeatureSlots: 3, FeatureEffectiveness: 1.40,
        MinLevel: 10, MaxDurability: 80, DestructionChance: 0.05,
        ModifierProbs: ModifierProbs{Tuned: 0.20, Defective: 0.05, Cursed: 0.02},
    },
    "ghost": {
        ID: "ghost", StatMultiplier: 2.2, FeatureSlots: 4, FeatureEffectiveness: 1.60,
        MinLevel: 15, MaxDurability: 100, DestructionChance: 0.01,
        ModifierProbs: ModifierProbs{Tuned: 0.30, Defective: 0.02, Cursed: 0.01},
    },
}

// ANSI escape codes for rarity colors.
const (
    ansiGray   = "\033[90m"
    ansiWhite  = "\033[97m"
    ansiGreen  = "\033[32m"
    ansiPurple = "\033[35m"
    ansiGold   = "\033[33m"
    ansiReset  = "\033[0m"
)

// RarityColor returns the ANSI opening escape for the given rarity ID.
// Returns ansiReset for unknown rarities.
func RarityColor(rarityID string) string {
    switch rarityID {
    case "salvage":
        return ansiGray
    case "street":
        return ansiWhite
    case "mil_spec":
        return ansiGreen
    case "black_market":
        return ansiPurple
    case "ghost":
        return ansiGold
    default:
        return ansiReset
    }
}

// RarityReset returns the ANSI reset escape code.
func RarityReset() string { return ansiReset }

// RollModifier selects a modifier ("tuned", "defective", "cursed", or "") based on rarity probabilities.
// rng must return a float in [0.0, 1.0).
func RollModifier(rarityID string, rng Roller) string {
    def, ok := RarityRegistry[rarityID]
    if !ok {
        return ""
    }
    p := rng.RollFloat()
    if p < def.ModifierProbs.Cursed {
        return "cursed"
    }
    p -= def.ModifierProbs.Cursed
    if p < def.ModifierProbs.Defective {
        return "defective"
    }
    p -= def.ModifierProbs.Defective
    if p < def.ModifierProbs.Tuned {
        return "tuned"
    }
    return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestRarity -v 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/rarity.go internal/game/inventory/rarity_test.go
git commit -m "feat(equipment-mechanics): add RarityDef constants and RarityRegistry"
```

---

## Task 2: Roller Interface

**Files:**
- Create: `internal/game/inventory/roller.go`

- [ ] **Step 1: Write the failing test**

```go
// Add to rarity_test.go
func TestDiceRoller_RollD20_InRange(t *testing.T) {
    roller := inventory.NewDiceRoller()
    rapid.Check(t, func(rt *rapid.T) {
        v := roller.RollD20()
        if v < 1 || v > 20 {
            rt.Fatalf("RollD20() = %d, want 1-20", v)
        }
    })
}

func TestDiceRoller_RollFloat_InRange(t *testing.T) {
    roller := inventory.NewDiceRoller()
    rapid.Check(t, func(rt *rapid.T) {
        v := roller.RollFloat()
        if v < 0.0 || v >= 1.0 {
            rt.Fatalf("RollFloat() = %v, want [0.0, 1.0)", v)
        }
    })
}
```

Also add `"pgregory.net/rapid"` import to rarity_test.go.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestDiceRoller -v 2>&1 | head -20
```

Expected: compile error — `NewDiceRoller` undefined

- [ ] **Step 3: Implement roller.go**

```go
// internal/game/inventory/roller.go
package inventory

import (
    "math/rand"
    "time"

    "github.com/cory-johannsen/mud/internal/game/dice"
)

// Roller abstracts dice rolling for testability.
type Roller interface {
    Roll(diceStr string) int   // e.g. "2d6+4", "1d6"
    RollD20() int
    RollFloat() float64        // [0.0, 1.0) for probability checks
}

// DiceRoller is the production Roller backed by the dice package.
type DiceRoller struct {
    rng *rand.Rand
}

// NewDiceRoller returns a DiceRoller seeded from the current time.
func NewDiceRoller() *DiceRoller {
    return &DiceRoller{rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (d *DiceRoller) Roll(diceStr string) int {
    return dice.Roll(diceStr)
}

func (d *DiceRoller) RollD20() int {
    return d.rng.Intn(20) + 1
}

func (d *DiceRoller) RollFloat() float64 {
    return d.rng.Float64()
}
```

> Note: Check `internal/game/dice/roller.go` for the actual exported function name. If it is not `dice.Roll(string) int`, adapt the call to match the actual API.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestDiceRoller -v 2>&1 | tail -20
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/roller.go internal/game/inventory/rarity_test.go
git commit -m "feat(equipment-mechanics): add Roller interface and DiceRoller implementation"
```

---

## Task 3: Database Migrations

**Files:**
- Create: `db/migrations/035_equipment_durability.sql`
- Create: `db/migrations/036_weapon_preset_durability.sql`
- Create: `db/migrations/037_inventory_instances.sql`

- [ ] **Step 1: Verify latest migration number**

```bash
ls db/migrations/ | sort | tail -5
```

Expected: `034_detained_until.sql` is the latest. Migrations 035, 036, 037 are unused.

- [ ] **Step 2: Verify characters.team already exists**

```bash
grep -r "team" db/migrations/ | grep -i "alter table characters"
```

Expected: migration 004 or similar already adds `team` column. We do NOT need a migration for it.

- [ ] **Step 3: Write migration 035 (equipment durability)**

```sql
-- db/migrations/035_equipment_durability.sql
ALTER TABLE character_equipment
    ADD COLUMN IF NOT EXISTS durability     int NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS max_durability int NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS instance_id    text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS modifier       text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS curse_revealed bool NOT NULL DEFAULT false;
```

- [ ] **Step 4: Write migration 036 (weapon preset durability)**

```sql
-- db/migrations/036_weapon_preset_durability.sql
ALTER TABLE character_weapon_presets
    ADD COLUMN IF NOT EXISTS durability     int NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS max_durability int NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS instance_id    text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS modifier       text NOT NULL DEFAULT '';
```

- [ ] **Step 5: Write migration 037 (inventory instances)**

```sql
-- db/migrations/037_inventory_instances.sql
CREATE TABLE IF NOT EXISTS character_inventory_instances (
    instance_id    text   PRIMARY KEY,
    character_id   bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    item_def_id    text   NOT NULL,
    durability     int    NOT NULL DEFAULT -1,
    max_durability int    NOT NULL DEFAULT -1,
    modifier       text   NOT NULL DEFAULT '',
    curse_revealed bool   NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_inventory_instances_character_id
    ON character_inventory_instances(character_id);
```

- [ ] **Step 6: Run migrations**

```bash
cd /home/cjohannsen/src/mud && make migrate 2>&1 | tail -20
```

Expected: migrations 035, 036, 037 applied successfully.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add db/migrations/035_equipment_durability.sql db/migrations/036_weapon_preset_durability.sql db/migrations/037_inventory_instances.sql
git commit -m "feat(equipment-mechanics): add durability + inventory_instances DB migrations"
```

---

## Task 4: Extend ItemInstance / SlottedItem / EquippedWeapon

**Files:**
- Modify: `internal/game/inventory/backpack.go`
- Modify: `internal/game/inventory/equipment.go`
- Modify: `internal/game/inventory/preset.go`

- [ ] **Step 1: Read the current structs**

```bash
cd /home/cjohannsen/src/mud && grep -n "type ItemInstance\|type SlottedItem\|type EquippedWeapon" internal/game/inventory/backpack.go internal/game/inventory/equipment.go internal/game/inventory/preset.go
```

- [ ] **Step 2: Write failing tests**

```go
// internal/game/inventory/backpack_test.go (create or add to existing)
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestItemInstance_HasDurabilityFields(t *testing.T) {
    inst := inventory.ItemInstance{
        InstanceID:    "uuid-1",
        ItemDefID:     "street_jacket",
        Quantity:      1,
        Durability:    40,
        MaxDurability: 40,
        Modifier:      "tuned",
        CurseRevealed: false,
    }
    if inst.Durability != 40 {
        t.Errorf("Durability = %d, want 40", inst.Durability)
    }
    if inst.Modifier != "tuned" {
        t.Errorf("Modifier = %q, want tuned", inst.Modifier)
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run TestItemInstance -v 2>&1 | head -20
```

Expected: compile error — `Durability` field undefined on `ItemInstance`

- [ ] **Step 4: Extend ItemInstance in backpack.go**

Add fields to the `ItemInstance` struct:
```go
Durability    int
MaxDurability int
Modifier      string // "" | "tuned" | "defective" | "cursed"
CurseRevealed bool
```

- [ ] **Step 5: Extend SlottedItem in equipment.go**

Add fields to the `SlottedItem` struct:
```go
InstanceID    string
Durability    int
Modifier      string
CurseRevealed bool
```

- [ ] **Step 6: Extend EquippedWeapon in preset.go**

Add fields to the `EquippedWeapon` struct:
```go
InstanceID string
Durability int
Modifier   string
```

- [ ] **Step 7: Verify project still compiles**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors (new fields have zero values, existing code unaffected)

- [ ] **Step 8: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all pass

- [ ] **Step 9: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/backpack.go internal/game/inventory/equipment.go internal/game/inventory/preset.go
git commit -m "feat(equipment-mechanics): add durability/modifier fields to item instance structs"
```

---

## Task 5: Rarity Field on WeaponDef / ArmorDef + Load Validation + Min Level

**Files:**
- Modify: `internal/game/inventory/weapon.go`
- Modify: `internal/game/inventory/armor.go`
- Modify: `internal/game/inventory/item.go`

- [ ] **Step 1: Read the WeaponDef and ArmorDef structs**

```bash
cd /home/cjohannsen/src/mud && grep -n "type WeaponDef\|type ArmorDef\|type ItemDef" internal/game/inventory/weapon.go internal/game/inventory/armor.go internal/game/inventory/item.go
```

Then read the full structs (10-20 lines around each).

- [ ] **Step 2: Write failing tests**

```go
// internal/game/inventory/weapon_rarity_test.go
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestWeaponDef_HasRarityField(t *testing.T) {
    def := inventory.WeaponDef{}
    def.Rarity = "street"
    if def.Rarity != "street" {
        t.Errorf("Rarity = %q, want street", def.Rarity)
    }
}

func TestArmorDef_HasRarityField(t *testing.T) {
    def := inventory.ArmorDef{}
    def.Rarity = "mil_spec"
    if def.Rarity != "mil_spec" {
        t.Errorf("Rarity = %q, want mil_spec", def.Rarity)
    }
}

func TestItemDef_HasRarityAndTeamFields(t *testing.T) {
    def := inventory.ItemDef{}
    def.Rarity = "salvage"
    def.Team = "gun"
    if def.Rarity != "salvage" || def.Team != "gun" {
        t.Errorf("fields not set: Rarity=%q Team=%q", def.Rarity, def.Team)
    }
}

func TestValidateWeaponRarity_MissingRarityFails(t *testing.T) {
    def := &inventory.WeaponDef{} // Rarity == ""
    if err := inventory.ValidateWeaponRarity(def); err == nil {
        t.Error("expected error for missing rarity, got nil")
    }
}

func TestValidateArmorRarity_MissingRarityFails(t *testing.T) {
    def := &inventory.ArmorDef{} // Rarity == ""
    if err := inventory.ValidateArmorRarity(def); err == nil {
        t.Error("expected error for missing rarity, got nil")
    }
}

func TestValidateItemTeam_InvalidTeamFails(t *testing.T) {
    def := &inventory.ItemDef{Team: "invalid_team"}
    if err := inventory.ValidateItemTeam(def); err == nil {
        t.Error("expected error for invalid team, got nil")
    }
}

func TestCheckMinLevel_TooLowFails(t *testing.T) {
    def := &inventory.WeaponDef{Rarity: "mil_spec"} // MinLevel 5
    err := inventory.CheckWeaponMinLevel(def, 3)     // player level 3
    if err == nil {
        t.Error("expected error for level too low, got nil")
    }
    want := "You need to be level 5 to equip"
    if !strings.Contains(err.Error(), want) {
        t.Errorf("error = %q, want substring %q", err.Error(), want)
    }
}
```

Add `"strings"` import.

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestWeaponDef_HasRarityField|TestArmorDef_HasRarityField|TestValidateWeaponRarity|TestValidateArmorRarity|TestValidateItemTeam|TestCheckMinLevel" -v 2>&1 | head -30
```

Expected: compile errors

- [ ] **Step 4: Add Rarity field to WeaponDef**

In `internal/game/inventory/weapon.go`, add to `WeaponDef`:
```go
Rarity string `yaml:"rarity"`
```

Also add validation and min-level functions to weapon.go (or a new `weapon_rarity.go`):

```go
// ValidateWeaponRarity returns an error if the WeaponDef has an unrecognized or missing rarity.
// Precondition: def is non-nil.
func ValidateWeaponRarity(def *WeaponDef) error {
    if def.Rarity == "" {
        return fmt.Errorf("weapon %q: missing required rarity field", def.ID)
    }
    if _, ok := RarityRegistry[def.Rarity]; !ok {
        return fmt.Errorf("weapon %q: unknown rarity %q", def.ID, def.Rarity)
    }
    return nil
}

// CheckWeaponMinLevel returns an error if playerLevel is below the rarity min level.
func CheckWeaponMinLevel(def *WeaponDef, playerLevel int) error {
    rd, ok := RarityRegistry[def.Rarity]
    if !ok {
        return nil
    }
    if playerLevel < rd.MinLevel {
        return fmt.Errorf("You need to be level %d to equip %s.", rd.MinLevel, def.Name)
    }
    return nil
}
```

- [ ] **Step 5: Add Rarity field to ArmorDef**

In `internal/game/inventory/armor.go`, add `Rarity string \`yaml:"rarity"\`` to `ArmorDef`.
Add `ValidateArmorRarity` and `CheckArmorMinLevel` analogous to weapon functions.

- [ ] **Step 6: Add Rarity, Team, Effect fields to ItemDef**

In `internal/game/inventory/item.go`, add to `ItemDef`:
```go
Rarity string           `yaml:"rarity,omitempty"`
Team   string           `yaml:"team,omitempty"`
Effect *ConsumableEffect `yaml:"effect,omitempty"`
```

Add `ValidateItemTeam`:
```go
func ValidateItemTeam(def *ItemDef) error {
    if def.Team == "" {
        return nil
    }
    if def.Team != "gun" && def.Team != "machete" {
        return fmt.Errorf("item %q: invalid team %q (must be gun, machete, or empty)", def.ID, def.Team)
    }
    return nil
}
```

- [ ] **Step 7: Apply stat multiplier at weapon/armor load**

Find where `WeaponDef` structs are populated from YAML (the registry/loader). After loading each `WeaponDef`, call the rarity stat multiplier. Example pattern to add in the loader after YAML unmarshal:

```go
if err := ValidateWeaponRarity(def); err != nil {
    log.Fatalf("fatal: %v", err)
}
rd := RarityRegistry[def.Rarity]
// Apply stat multiplier to base damage average.
// The damage dice string is not changed; only numeric bonus fields are scaled.
// (Consult the actual WeaponDef fields — scale whatever represents base damage bonus.)
def.BaseDamageBonus = int(math.Round(float64(def.BaseDamageBonus) * rd.StatMultiplier))
```

> Note: Read the actual `WeaponDef` fields for damage. If damage is expressed as a dice string (e.g., `"2d6"`), the multiplier applies to the average and the dice count must be adjusted. Check the actual field names and adapt accordingly. For armor, scale `ACBonus`:
```go
def.ACBonus = int(math.Round(float64(def.ACBonus) * rd.StatMultiplier))
```

- [ ] **Step 8: Validate all YAML on startup — add rarity to existing weapon/armor YAML files**

```bash
cd /home/cjohannsen/src/mud && grep -rL "rarity:" content/weapons/ content/armor/ 2>/dev/null | head -20
```

For each file that lacks `rarity:`, add `rarity: street` as a default (adjust appropriately per item). Do NOT leave any file without a rarity field — startup will fatal.

- [ ] **Step 9: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
```

Expected: all pass

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/ content/weapons/ content/armor/
git commit -m "feat(equipment-mechanics): add rarity field to WeaponDef/ArmorDef; stat multiplier at load; min level enforcement"
```

---

## Task 6: DurabilityManager Pure Functions

**Files:**
- Create: `internal/game/inventory/durability.go`
- Create: `internal/game/inventory/durability_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/game/inventory/durability_test.go
package inventory_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

// stubRoller is a deterministic Roller for tests.
type stubRoller struct {
    floatVal float64
    d6Val    int
    d20Val   int
}

func (s *stubRoller) Roll(_ string) int    { return s.d6Val }
func (s *stubRoller) RollD20() int         { return s.d20Val }
func (s *stubRoller) RollFloat() float64   { return s.floatVal }

func TestDeductDurability_ReducesByOne(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 10, MaxDurability: 40}
    inst.ItemDefID = "street_jacket"
    rng := &stubRoller{floatVal: 0.99} // won't destroy
    result := inventory.DeductDurability(inst, "street", rng)
    if result.NewDurability != 9 {
        t.Errorf("NewDurability = %d, want 9", result.NewDurability)
    }
    if result.BecameBroken {
        t.Error("BecameBroken should be false at durability 9")
    }
}

func TestDeductDurability_BecomesBroken_NoDestruction(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 1, MaxDurability: 40}
    rng := &stubRoller{floatVal: 0.99} // 0.99 > 0.30 destruction chance for street
    result := inventory.DeductDurability(inst, "street", rng)
    if !result.BecameBroken {
        t.Error("BecameBroken should be true")
    }
    if result.Destroyed {
        t.Error("Destroyed should be false when float > DestructionChance")
    }
    if inst.Durability != 0 {
        t.Errorf("inst.Durability = %d, want 0", inst.Durability)
    }
}

func TestDeductDurability_BecomesBroken_Destroyed(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 1, MaxDurability: 40}
    rng := &stubRoller{floatVal: 0.01} // 0.01 < 0.30 destruction chance for street
    result := inventory.DeductDurability(inst, "street", rng)
    if !result.BecameBroken {
        t.Error("BecameBroken should be true")
    }
    if !result.Destroyed {
        t.Error("Destroyed should be true when float < DestructionChance")
    }
}

func TestDeductDurability_AlreadyBroken_NoOp(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 0, MaxDurability: 40}
    rng := &stubRoller{floatVal: 0.0} // would destroy if roll made
    result := inventory.DeductDurability(inst, "street", rng)
    if result.BecameBroken || result.Destroyed {
        t.Error("no-op on already-broken item: BecameBroken/Destroyed must be false")
    }
    if inst.Durability != 0 {
        t.Errorf("inst.Durability = %d, want 0", inst.Durability)
    }
}

func TestRepairField_Restores1d6(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 5, MaxDurability: 40}
    rng := &stubRoller{d6Val: 4}
    restored := inventory.RepairField(inst, rng)
    if restored != 4 {
        t.Errorf("restored = %d, want 4", restored)
    }
    if inst.Durability != 9 {
        t.Errorf("Durability = %d, want 9", inst.Durability)
    }
}

func TestRepairField_CapsAtMaxDurability(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 38, MaxDurability: 40}
    rng := &stubRoller{d6Val: 6}
    restored := inventory.RepairField(inst, rng)
    if inst.Durability != 40 {
        t.Errorf("Durability = %d, want 40 (capped)", inst.Durability)
    }
    if restored != 2 {
        t.Errorf("restored = %d, want 2 (capped)", restored)
    }
}

func TestRepairFull_RestoresToMax(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 10, MaxDurability: 60}
    inventory.RepairFull(inst)
    if inst.Durability != 60 {
        t.Errorf("Durability = %d, want 60", inst.Durability)
    }
}

func TestInitDurability_SentinelInitialized(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: -1}
    inventory.InitDurability(inst, "street")
    if inst.Durability != 40 {
        t.Errorf("Durability = %d, want 40", inst.Durability)
    }
    if inst.MaxDurability != 40 {
        t.Errorf("MaxDurability = %d, want 40", inst.MaxDurability)
    }
}

func TestInitDurability_NonSentinel_NoChange(t *testing.T) {
    inst := &inventory.ItemInstance{Durability: 15, MaxDurability: 40}
    inventory.InitDurability(inst, "street")
    if inst.Durability != 15 {
        t.Errorf("Durability = %d, want 15 (unchanged)", inst.Durability)
    }
}

func TestDeductDurability_Property(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        initial := rapid.IntRange(1, 100).Draw(rt, "initial")
        maxDur := rapid.IntRange(initial, 100).Draw(rt, "maxDur")
        inst := &inventory.ItemInstance{Durability: initial, MaxDurability: maxDur}
        rng := &stubRoller{floatVal: 0.99}
        result := inventory.DeductDurability(inst, "ghost", rng)
        if result.NewDurability != initial-1 {
            rt.Fatalf("NewDurability = %d, want %d", result.NewDurability, initial-1)
        }
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestDeductDurability|TestRepairField|TestRepairFull|TestInitDurability" -v 2>&1 | head -20
```

Expected: compile error — `DeductDurability` etc. undefined

- [ ] **Step 3: Implement durability.go**

```go
// internal/game/inventory/durability.go
package inventory

// DeductResult holds the outcome of a single durability deduction.
type DeductResult struct {
    NewDurability int
    BecameBroken  bool
    Destroyed     bool
}

// DeductDurability reduces inst.Durability by 1 and makes a destruction roll if it reaches 0.
// Precondition: inst is non-nil; rarityID is a valid rarity key or "".
// Postcondition: inst.Durability is decremented (clamped to 0). If it was already 0, no-op.
// REQ-EM-44: This is a pure function w.r.t. ItemInstance. Persistence is the caller's responsibility.
func DeductDurability(inst *ItemInstance, rarityID string, rng Roller) DeductResult {
    if inst.Durability == 0 {
        return DeductResult{NewDurability: 0}
    }
    inst.Durability--
    if inst.Durability > 0 {
        return DeductResult{NewDurability: inst.Durability}
    }
    // Durability just reached 0 — make destruction roll.
    rd, ok := RarityRegistry[rarityID]
    if !ok {
        return DeductResult{NewDurability: 0, BecameBroken: true}
    }
    destroyed := rng.RollFloat() < rd.DestructionChance
    return DeductResult{NewDurability: 0, BecameBroken: true, Destroyed: destroyed}
}

// RepairField restores 1d6 durability (capped at MaxDurability).
// Caller MUST consume a repair_kit before calling this.
// Returns the number of durability points actually restored.
func RepairField(inst *ItemInstance, rng Roller) int {
    if inst.Durability >= inst.MaxDurability {
        return 0
    }
    roll := rng.Roll("1d6")
    before := inst.Durability
    inst.Durability += roll
    if inst.Durability > inst.MaxDurability {
        inst.Durability = inst.MaxDurability
    }
    return inst.Durability - before
}

// RepairFull restores inst to MaxDurability. Used by the Downtime Repair activity.
func RepairFull(inst *ItemInstance) {
    inst.Durability = inst.MaxDurability
}

// InitDurability sets Durability = MaxDurability for the item's rarity if Durability == -1 (sentinel).
// REQ-EM-17: Called at login for all equipped and backpack items.
func InitDurability(inst *ItemInstance, rarityID string) {
    if inst.Durability != -1 {
        return
    }
    rd, ok := RarityRegistry[rarityID]
    if !ok {
        return
    }
    inst.MaxDurability = rd.MaxDurability
    inst.Durability = rd.MaxDurability
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestDeductDurability|TestRepairField|TestRepairFull|TestInitDurability" -v 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/durability.go internal/game/inventory/durability_test.go
git commit -m "feat(equipment-mechanics): add DurabilityManager pure functions with property-based tests"
```

---

## Task 7: InitDurability at Login

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (lines ~604–636, the JoinWorld session load path)

- [ ] **Step 1: Locate the session load code**

The JoinWorld handler in `internal/gameserver/grpc_service.go` around lines 604–636 loads equipment and backpack items:
```go
eq, eqErr := s.charSaver.LoadEquipment(loadCtx2, characterID)
// ...
sess.Equipment = eq
// ...
sess.Backpack.Add(it.ItemDefID, it.Quantity, s.invRegistry)
```
After these lines is where `InitDurability` calls must be inserted.

- [ ] **Step 2: Write failing test**

In the session loading test file (or create `internal/gameserver/login_durability_test.go`):

```go
func TestLogin_InitsDurabilityForSentinelItems(t *testing.T) {
    t.Skip("integration test: requires real DB and JoinWorld flow — verify manually after Step 3")
    // When un-skipped: load a session via the JoinWorld handler with a test character
    // whose equipment rows have durability == -1. Assert that after load, no equipped
    // item or backpack item has Durability == -1.
}
```

The test skips by default and compiles cleanly. To verify the behavior, either:
- Add a unit test over `InitDurability` calls directly (these are covered by Task 6 tests), or
- Add assertions into the existing `grpc_service_test.go` integration tests after wiring `InitDurability` in Step 3.

- [ ] **Step 3: Add InitDurability calls in the session load path**

After loading equipped items from DB, for each `SlottedItem`:
```go
inst := // resolve to ItemInstance (lookup by InstanceID or create from slot data)
inventory.InitDurability(&inst, weaponOrArmorDef.Rarity)
slottedItem.Durability = inst.Durability
```

After loading backpack items from `character_inventory_instances`, for each `ItemInstance`:
```go
itemDef := reg.Get(item.ItemDefID)
inventory.InitDurability(&item, itemDef.Rarity)
```

Then persist the updated durability back to DB for any row that was `-1`.

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add -p
git commit -m "feat(equipment-mechanics): call InitDurability at login for sentinel items (REQ-EM-17)"
```

---

## Task 8: Weapon Durability Deduction in Combat

**Files:**
- Modify: `internal/game/combat/resolver.go`

- [ ] **Step 1: Read the resolver**

```bash
cd /home/cjohannsen/src/mud && grep -n "func Resolve\|func.*Attack" internal/game/combat/resolver.go | head -20
```

Read the full `ResolveAttack` and `ResolveFirearmAttack` functions (or equivalent).

- [ ] **Step 2: Write failing test**

```go
// internal/game/combat/resolver_durability_test.go
package combat_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/combat"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestResolveAttack_DeductsDurability(t *testing.T) {
    // Build a minimal AttackContext or equivalent with an EquippedWeapon
    // that has Durability = 10.
    // REPLACE_WITH_ACTUAL_ATTACK_CONTEXT_CONSTRUCTION
    weaponInst := &inventory.ItemInstance{
        InstanceID:    "test-uuid",
        ItemDefID:     "street_pistol",
        Durability:    10,
        MaxDurability: 40,
    }
    // Resolve an attack (hit or miss) and assert durability decremented to 9.
    // REPLACE_WITH_ACTUAL_RESOLVE_CALL
    _ = weaponInst
    t.Skip("REPLACE with actual resolver call using test infrastructure")
}
```

> Note: Replace the skeleton with the actual attack resolution API. The key invariant: after any `ResolveAttack` call (hit or miss), the weapon `ItemInstance.Durability` MUST be decremented by 1.

- [ ] **Step 3: Run test to verify it fails / skips**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestResolveAttack_DeductsDurability -v 2>&1
```

- [ ] **Step 4: Add durability deduction in resolver**

`ResolveAttack` in `internal/game/combat/resolver.go` takes `attacker, target *Combatant` and `src Source`. It does NOT have direct access to a `PlayerSession` or a persistence layer — it is a pure function. Durability deduction must be done by the caller of `ResolveAttack` in `internal/gameserver/combat_handler.go`.

Find the call site(s) of `ResolveAttack` in `combat_handler.go` and wrap them:

```go
// REQ-EM-5: Deduct weapon durability per attack (hit or miss).
attackResult := combat.ResolveAttack(attacker, target, src)

// Apply weapon durability deduction via the attacker's PlayerSession.
attackerSess, sessOk := h.sessions.GetPlayer(attacker.UID)
if sessOk && attackerSess.LoadoutSet != nil {
    activePreset := attackerSess.LoadoutSet.Active()
    if activePreset != nil {
        slot := activePreset.ActiveWeapon() // check actual method name
        if slot != nil && slot.InstanceID != "" {
            inst := attackerSess.Backpack.FindInstanceByID(slot.InstanceID)
            if inst != nil {
                weaponDef := h.invRegistry.GetWeapon(slot.Def.ID)
                dr := inventory.DeductDurability(inst, weaponDef.Rarity, h.rng)
                if dr.Destroyed {
                    attackerSess.Backpack.RemoveInstance(slot.InstanceID)
                    attackerSess.LoadoutSet.ClearWeaponSlot(slot)
                    h.appendMessage(attackerSess, fmt.Sprintf("Your %s has been destroyed.", slot.Def.Name))
                }
                if err := h.charSaver.SaveEquipment(ctx, attackerSess.CharacterID, attackerSess.Equipment); err != nil {
                    h.logger.Warn("save equipment after durability deduct", zap.Error(err))
                }
            }
        }
    }
}
```

> Note: Read `internal/gameserver/combat_handler.go` around the `ResolveAttack` call sites to find the actual method names for the loadout, weapon slot, and backpack instance lookup. Adapt accordingly — the pattern above shows the structural intent. The methods `FindInstanceByID`, `RemoveInstance`, `ClearWeaponSlot` may not exist yet; add them to their respective types if needed.

- [ ] **Step 5: Also handle broken weapon — 0 damage (REQ-EM-7)**

In `combat_handler.go`, before computing damage from the attack result, check weapon durability:

```go
// REQ-EM-7: Broken weapon contributes 0 damage beyond unarmed base.
if slot != nil && slot.Durability == 0 {
    attackResult.Damage = unarmedBaseDamage(attackerSess)
}
```

Where `unarmedBaseDamage` is a small helper returning 1 (or the session's unarmed damage attribute). Verify against the actual unarmed damage logic in the codebase.

- [ ] **Step 6: Apply modifier damage adjustment (REQ-EM-23)**

In `combat_handler.go`, after step 5, apply non-zero modifier to damage for non-broken weapons:

```go
// REQ-EM-23: Apply modifier damage adjustment for non-broken weapons.
if slot != nil && slot.Durability > 0 && slot.Modifier != "" {
    switch slot.Modifier {
    case "tuned":
        attackResult.Damage = int(math.Round(float64(attackResult.Damage) * 1.10))
    case "defective":
        attackResult.Damage = int(math.Round(float64(attackResult.Damage) * 0.90))
    case "cursed":
        attackResult.Damage = int(math.Round(float64(attackResult.Damage) * 0.85))
    }
}
```

- [ ] **Step 6: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/game/combat/resolver_durability_test.go
git commit -m "feat(equipment-mechanics): deduct weapon durability per attack; broken weapon → 0 damage; modifier damage adj (REQ-EM-5, REQ-EM-7, REQ-EM-23)"
```

---

## Task 9: Armor Durability Deduction + ComputedDefenses Updates

**Files:**
- Modify: `internal/gameserver/combat_handler.go` (armor durability deduction in the hit handler)
- Modify: `internal/game/inventory/equipment.go` (`ComputedDefenses` — broken armor + modifier AC)

- [ ] **Step 1: Read ApplyDamage and ComputedDefenses**

```bash
cd /home/cjohannsen/src/mud && grep -n "func.*ApplyDamage\|func.*ComputedDefenses" internal/game/combat/combat.go internal/game/inventory/equipment.go
```

Read both functions fully.

- [ ] **Step 2: Write failing tests**

```go
// internal/game/inventory/equipment_durability_test.go
package inventory_test

func TestComputedDefenses_BrokenArmorContributesZeroAC(t *testing.T) {
    // Create a Registry stub and a SlottedItem with Durability==0
    slotted := inventory.SlottedItem{
        ItemDefID:  "street_jacket",
        Durability: 0,  // broken
    }
    // Build equipment with this slotted item
    // Call ComputedDefenses and assert ACBonus from this slot is 0
    // REPLACE_WITH_ACTUAL_ComputedDefenses_CALL
    _ = slotted
    t.Skip("REPLACE with actual equipment test")
}

func TestComputedDefenses_ModifierACApplied(t *testing.T) {
    slotted := inventory.SlottedItem{
        ItemDefID:  "street_jacket",
        Durability: 40,
        Modifier:   "tuned",  // +1 AC
    }
    _ = slotted
    t.Skip("REPLACE with actual equipment test")
}
```

- [ ] **Step 3: Update ComputedDefenses in equipment.go**

In `ComputedDefenses`, when computing AC for each slot:
```go
ac := baseDef.ACBonus // rarity-multiplied at load time
if slot.Durability == 0 {
    ac = 0 // REQ-EM-7: broken armor contributes 0
} else {
    switch slot.Modifier {
    case "tuned":
        ac += 1  // REQ-EM-22
    case "defective":
        ac -= 1
    case "cursed":
        ac -= 2
    }
}
totalAC += ac
```

- [ ] **Step 4: Add armor durability deduction in the hit handler**

`Combatant.ApplyDamage` in `internal/game/combat/combat.go` is a pure method with no access to `PlayerSession`. Like weapon durability, armor durability deduction must be done by the caller in `internal/gameserver/combat_handler.go`, after `ResolveAttack` returns a hit.

In `combat_handler.go`, after confirming a hit, find the struck armor slot (use the existing slot-selection logic if any; otherwise pick the body slot). Then:

```go
// REQ-EM-6: Deduct durability from struck armor slot.
targetSess, targetOk := h.sessions.GetPlayer(target.UID)
if targetOk && targetSess.Equipment != nil {
    struckSlotName := selectStruckSlot(attackResult) // use existing slot logic
    slotted := targetSess.Equipment.GetSlot(struckSlotName)
    if slotted != nil && slotted.InstanceID != "" {
        inst := targetSess.Backpack.FindInstanceByID(slotted.InstanceID)
        if inst != nil {
            armorDef := h.invRegistry.GetArmor(slotted.ItemDefID)
            dr := inventory.DeductDurability(inst, armorDef.Rarity, h.rng)
            if dr.Destroyed {
                targetSess.Equipment.ClearSlot(struckSlotName)
                targetSess.Backpack.RemoveInstance(slotted.InstanceID)
                h.appendMessage(targetSess, fmt.Sprintf("Your %s has been destroyed.", slotted.Name))
            }
            if err := h.charSaver.SaveEquipment(ctx, targetSess.CharacterID, targetSess.Equipment); err != nil {
                h.logger.Warn("save equipment after armor durability deduct", zap.Error(err))
            }
        }
    }
}
```

> Note: `selectStruckSlot`, `GetSlot`, `ClearSlot`, `FindInstanceByID`, `RemoveInstance` may not all exist yet. Read `equipment.go` and `backpack.go` for the actual API and add any missing helpers.

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/game/inventory/equipment.go
git commit -m "feat(equipment-mechanics): armor durability deduction per hit; ComputedDefenses modifier+broken logic (REQ-EM-6, REQ-EM-7, REQ-EM-22)"
```

---

## Task 10: Repair Command

**Files:**
- Create: `internal/gameserver/grpc_service_repair.go`

- [ ] **Step 1: Write failing test**

```go
// internal/gameserver/grpc_service_repair_test.go
package gameserver_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/gameserver"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func TestHandleRepair_NoRepairKit_Fails(t *testing.T) {
    // Build a GameServiceServer with a session that has NO repair_kit in backpack.
    // Call handleRepair. Assert error message = "You need a repair kit to field-repair equipment."
    t.Skip("REPLACE with actual server test setup")
}

func TestHandleRepair_WithRepairKit_RestoresDurability(t *testing.T) {
    // Build session with repair_kit in backpack and a broken item in equipment.
    // Call handleRepair. Assert durability increased by 1d6.
    t.Skip("REPLACE with actual server test setup")
}
```

- [ ] **Step 2: Run test to verify it skips/fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleRepair -v 2>&1 | head -20
```

- [ ] **Step 3: Implement grpc_service_repair.go**

```go
// internal/gameserver/grpc_service_repair.go
package gameserver

import (
    "context"
    "fmt"

    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

// handleRepair processes a `repair <item>` command.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// REQ-EM-13: Requires repair_kit in backpack; consumes it before calling RepairField.
// REQ-EM-14: Restores 1d6 durability, capped at MaxDurability.
func (s *GameServiceServer) handleRepair(uid string, req *gamev1.RepairRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }

    // Check for repair_kit in backpack.
    kitIdx := sess.Backpack.FindItem("repair_kit")
    if kitIdx < 0 {
        return messageEvent("You need a repair kit to field-repair equipment."), nil
    }

    // Find the target item instance by name in equipment or backpack.
    itemName := req.GetItemName()
    inst, rarityID := findItemInstanceByName(sess, itemName)
    if inst == nil {
        return messageEvent(fmt.Sprintf("You don't have %q equipped or in your backpack.", itemName)), nil
    }
    if inst.Durability == inst.MaxDurability {
        return messageEvent(fmt.Sprintf("%s is already at full durability.", itemName)), nil
    }

    // Consume the repair_kit.
    sess.Backpack.RemoveItem("repair_kit", 1)

    // Apply field repair.
    rng := inventory.NewDiceRoller()
    restored := inventory.RepairField(inst, rng)

    // Persist updated durability.
    if s.charSaver != nil && sess.CharacterID > 0 {
        _ = s.charSaver.SaveItemInstance(context.Background(), inst)
    }

    return messageEvent(fmt.Sprintf(
        "You field-repair %s. Restored %d durability (%d/%d).",
        itemName, restored, inst.Durability, inst.MaxDurability,
    )), nil
}
```

> Note: `findItemInstanceByName`, `sess.Backpack.FindItem`, `sess.Backpack.RemoveItem`, and `s.charSaver.SaveItemInstance` must be implemented or wired to actual methods. Check the backpack and repository APIs and adapt accordingly.

- [ ] **Step 4: Wire repair command in the main gRPC handler**

Find where commands are dispatched in `internal/gameserver/grpc_service.go` (the `ClientMessage` switch, around line 1473). Add a `"repair"` case:

```go
case "repair":
    return s.handleRepair(uid, parseRepairRequest(input))
```

Also add the `RepairRequest` message to the proto if it does not exist, or use a generic text request. Check existing patterns in `gamev1/`.

- [ ] **Step 5: Add 1 AP cost in combat (REQ-EM-15)**

In `handleRepair`, check whether the player is in combat and deduct 1 AP:

```go
// REQ-EM-15: repair costs 1 AP in combat.
if sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT) {
    // Look up the player's current AP in the combat engine's action queue.
    // Use the same pattern as other 1-AP actions (e.g., ActionReload).
    // Find the combatant in s.combatH and check/deduct AP before proceeding.
    if !s.combatH.DeductAP(uid, 1) {
        return messageEvent("You don't have enough AP to repair in combat."), nil
    }
}
```

> Note: Check `internal/gameserver/combat_handler.go` for the actual method to deduct AP from a combatant's action queue (look at how ActionReload handles AP). Use the same mechanism.

- [ ] **Step 6: Add downtime RepairFull integration (REQ-EM-16)**

Find the existing downtime Repair activity handler in `internal/gameserver/action_handler.go` (search for `downtimeContext`). The Repair activity must call `inventory.RepairFull` and compute the downtime day cost:

```go
// REQ-EM-16: Downtime Repair restores MaxDurability. Cost = ceil((MaxDurability - Durability) / 10), min 1.
func (s *GameServiceServer) handleDowntimeRepair(uid string, itemName string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    inst, rarityID := findItemInstanceByName(sess, itemName)
    if inst == nil {
        return messageEvent(fmt.Sprintf("You don't have %q.", itemName)), nil
    }
    missing := inst.MaxDurability - inst.Durability
    if missing == 0 {
        return messageEvent(fmt.Sprintf("%s is already at full durability.", itemName)), nil
    }
    days := (missing + 9) / 10 // ceil(missing / 10)
    if days < 1 {
        days = 1
    }
    inventory.RepairFull(inst)
    // Persist.
    if s.charSaver != nil && sess.CharacterID > 0 {
        _ = s.charSaver.SaveEquipment(context.Background(), sess.CharacterID, sess.Equipment)
    }
    return messageEvent(fmt.Sprintf(
        "You spend %d downtime day(s) repairing %s. It is now fully restored (%d/%d).",
        days, itemName, inst.MaxDurability, inst.MaxDurability,
    )), nil
}
```

Wire this handler into the downtime activity dispatch (the `"repair"` downtime activity case). The actual downtime activity routing must be found in `action_handler.go` and adapted.

- [ ] **Step 7: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service_repair.go internal/gameserver/grpc_service_repair_test.go internal/gameserver/action_handler.go
git commit -m "feat(equipment-mechanics): add repair command; AP cost in combat; downtime RepairFull (REQ-EM-13-16)"
```

---

## Task 11: Modifier Spawn + Display + Curse Mechanics

**Files:**
- Modify: `internal/game/inventory/item.go` (display helpers)
- Modify: `internal/game/inventory/floor.go` (loot drop spawn — apply `RollModifier`)
- Modify: `internal/gameserver/grpc_service.go` — `handleEquip` (lines ~3449+) and `handleUnequip` (lines ~3824+)
- Modify: `internal/game/inventory/rarity.go` — add `RollModifierForMerchant`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/inventory/modifier_test.go
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestDisplayName_Tuned(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: "tuned"}
    name := inventory.DisplayName("Pistol", inst)
    if name != "Tuned Pistol" {
        t.Errorf("DisplayName = %q, want %q", name, "Tuned Pistol")
    }
}

func TestDisplayName_Defective(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: "defective"}
    name := inventory.DisplayName("Pistol", inst)
    if name != "Defective Pistol" {
        t.Errorf("DisplayName = %q, want %q", name, "Defective Pistol")
    }
}

func TestDisplayName_Cursed_NotRevealed(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: "cursed", CurseRevealed: false}
    name := inventory.DisplayName("Pistol", inst)
    if name != "Pistol" {
        t.Errorf("DisplayName = %q, want %q (hidden curse)", name, "Pistol")
    }
}

func TestDisplayName_Cursed_Revealed(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: "cursed", CurseRevealed: true}
    name := inventory.DisplayName("Pistol", inst)
    if name != "Cursed Pistol" {
        t.Errorf("DisplayName = %q, want %q", name, "Cursed Pistol")
    }
}

func TestDisplayName_NoModifier(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: ""}
    name := inventory.DisplayName("Pistol", inst)
    if name != "Pistol" {
        t.Errorf("DisplayName = %q, want %q", name, "Pistol")
    }
}

func TestRollModifier_MerchantsNeverCursed(t *testing.T) {
    rng := &stubRoller{floatVal: 0.0} // would be cursed for normal spawn
    mod := inventory.RollModifierForMerchant("salvage", rng)
    if mod == "cursed" {
        t.Error("merchant modifier must never be cursed (REQ-EM-27)")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestDisplayName|TestRollModifier" -v 2>&1 | head -20
```

Expected: compile error

- [ ] **Step 3: Add DisplayName helper to item.go**

```go
// DisplayName returns the display name for an item instance, applying modifier prefix rules.
// REQ-EM-18, REQ-EM-19, REQ-EM-20.
func DisplayName(baseName string, inst *ItemInstance) string {
    if inst == nil || inst.Modifier == "" {
        return baseName
    }
    switch inst.Modifier {
    case "tuned":
        return "Tuned " + baseName
    case "defective":
        return "Defective " + baseName
    case "cursed":
        if inst.CurseRevealed {
            return "Cursed " + baseName
        }
        return baseName
    }
    return baseName
}
```

Add `RollModifierForMerchant` to `rarity.go`:

```go
// RollModifierForMerchant is like RollModifier but never returns "cursed" (REQ-EM-27).
func RollModifierForMerchant(rarityID string, rng Roller) string {
    mod := RollModifier(rarityID, rng)
    if mod == "cursed" {
        return ""
    }
    return mod
}
```

- [ ] **Step 4: Add curse reveal on equip**

Find the `equip` command handler. When an item with `Modifier == "cursed"` is equipped:
```go
if inst.Modifier == "cursed" && !inst.CurseRevealed {
    inst.CurseRevealed = true
    // persist inst
}
```

- [ ] **Step 5: Block unequip for cursed + revealed items (REQ-EM-24)**

In `handleUnequip` in `internal/gameserver/grpc_service.go` (line ~3828), before removing the item from the slot, add:
```go
if slottedItem.Modifier == "cursed" && slottedItem.CurseRevealed {
    return messageEvent("This item is cursed and cannot be removed."), nil
}
```

- [ ] **Step 6: Implement curse-removal → defective transition (REQ-EM-25)**

When a cursed item is successfully uncursed (by the `curse-removal` feature via a chip_doc NPC), its modifier must become `"defective"` and `CurseRevealed` reset to `false`. Add this helper that the future `curse-removal` feature will call:

```go
// UncurseItem changes a cursed item to defective and clears CurseRevealed.
// REQ-EM-25: Called by the curse-removal feature's uncurse command handler.
// Precondition: inst is non-nil; inst.Modifier == "cursed".
// Postcondition: inst.Modifier == "defective"; inst.CurseRevealed == false.
func UncurseItem(inst *inventory.ItemInstance) {
    inst.Modifier = "defective"
    inst.CurseRevealed = false
}
```

Add this function to `internal/gameserver/grpc_service_repair.go` (or a new `internal/gameserver/grpc_service_curse.go`). Wire it with a test:

```go
func TestUncurseItem_BecomesDefective(t *testing.T) {
    inst := &inventory.ItemInstance{Modifier: "cursed", CurseRevealed: true}
    gameserver.UncurseItem(inst)
    if inst.Modifier != "defective" {
        t.Errorf("Modifier = %q, want defective", inst.Modifier)
    }
    if inst.CurseRevealed {
        t.Error("CurseRevealed should be false after uncurse")
    }
}
```

- [ ] **Step 7: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/ internal/gameserver/
git commit -m "feat(equipment-mechanics): modifier display/spawn; curse reveal on equip; unequip blocked; UncurseItem (REQ-EM-18-27)"
```

---

## Task 12: Rarity ANSI Color in Inventory Display

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` (inventory/equipment rendering)
- Modify: `internal/gameserver/grpc_service.go` — `handleEquipment` (~line 3840) and any inline item listing

- [ ] **Step 1: Find inventory display code**

```bash
cd /home/cjohannsen/src/mud && grep -rn "inventory\|backpack\|equipment" internal/frontend/ --include="*.go" -l | head -10
```

```bash
cd /home/cjohannsen/src/mud && grep -rn "RenderInventory\|renderInventory\|InventoryList\|inventoryList" internal/ --include="*.go" | head -10
```

- [ ] **Step 2: Write failing test**

```go
func TestRarityColoredName_ContainsANSI(t *testing.T) {
    name := inventory.RarityColoredName("street", "Street Pistol", nil)
    if !strings.Contains(name, "\033[") {
        t.Errorf("RarityColoredName = %q, want ANSI escape in output", name)
    }
}
```

- [ ] **Step 3: Add RarityColoredName helper to item.go**

```go
// RarityColoredName wraps name with the rarity ANSI color and reset codes.
// If inst is non-nil, applies modifier prefix first.
// REQ-EM-4.
func RarityColoredName(rarityID string, baseName string, inst *ItemInstance) string {
    displayedName := DisplayName(baseName, inst)
    return RarityColor(rarityID) + displayedName + RarityReset()
}
```

- [ ] **Step 4: Apply RarityColoredName in inventory display**

In each place items are listed (inventory command, equipment command, shop displays), replace plain name with:
```go
inventory.RarityColoredName(def.Rarity, def.Name, inst)
```

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add -p
git commit -m "feat(equipment-mechanics): rarity ANSI color in all inventory/equipment displays (REQ-EM-4)"
```

---

## Task 13: ConsumableEffect Engine

**Files:**
- Create: `internal/game/inventory/consumable.go`
- Create: `internal/game/inventory/consumable_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/inventory/consumable_test.go
package inventory_test

import (
    "testing"
    "time"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

// mockTarget implements ConsumableTarget for testing.
type mockTarget struct {
    team       string
    statMods   map[string]int // stat name → modifier
    healed     int
    conditions []string
    removed    []string
    diseases   []string
    toxins     []string
}

func (m *mockTarget) GetTeam() string                                 { return m.team }
func (m *mockTarget) GetStatMod(stat string) int                     { return m.statMods[stat] }
func (m *mockTarget) ApplyHeal(amount int)                           { m.healed += amount }
func (m *mockTarget) ApplyCondition(id string, d time.Duration)      { m.conditions = append(m.conditions, id) }
func (m *mockTarget) RemoveCondition(id string)                      { m.removed = append(m.removed, id) }
func (m *mockTarget) ApplyDisease(id string, severity int)           { m.diseases = append(m.diseases, id) }
func (m *mockTarget) ApplyToxin(id string, severity int)             { m.toxins = append(m.toxins, id) }

func TestApplyConsumable_HealNoTeam(t *testing.T) {
    target := &mockTarget{team: "gun"}
    def := &inventory.ItemDef{
        ID:   "penjamin_franklin",
        Team: "",
        Effect: &inventory.ConsumableEffect{
            Heal: "1d6",
        },
    }
    rng := &stubRoller{d6Val: 4}
    result := inventory.ApplyConsumable(target, def, rng)
    if result.HealApplied != 4 {
        t.Errorf("HealApplied = %d, want 4", result.HealApplied)
    }
    if result.TeamMultiplier != 1.0 {
        t.Errorf("TeamMultiplier = %v, want 1.0", result.TeamMultiplier)
    }
}

func TestApplyConsumable_HealSameTeamBonus(t *testing.T) {
    target := &mockTarget{team: "gun"}
    def := &inventory.ItemDef{
        ID:   "old_english",
        Team: "gun",
        Effect: &inventory.ConsumableEffect{
            Heal: "1d6",
        },
    }
    rng := &stubRoller{d6Val: 4}
    result := inventory.ApplyConsumable(target, def, rng)
    // 4 * 1.25 = 5.0 → floored to 5
    if result.HealApplied != 5 {
        t.Errorf("HealApplied = %d, want 5 (1.25× same-team)", result.HealApplied)
    }
}

func TestApplyConsumable_HealOpposingTeamPenalty(t *testing.T) {
    target := &mockTarget{team: "machete"}
    def := &inventory.ItemDef{
        ID:   "old_english",
        Team: "gun",
        Effect: &inventory.ConsumableEffect{
            Heal: "1d6",
        },
    }
    rng := &stubRoller{d6Val: 4}
    result := inventory.ApplyConsumable(target, def, rng)
    // 4 * 0.75 = 3.0 → floored to 3
    if result.HealApplied != 3 {
        t.Errorf("HealApplied = %d, want 3 (0.75× opposing team)", result.HealApplied)
    }
}

func TestApplyConsumable_RemoveConditionsBefore(t *testing.T) {
    target := &mockTarget{team: ""}
    def := &inventory.ItemDef{
        ID:   "poontangesca",
        Team: "machete",
        Effect: &inventory.ConsumableEffect{
            RemoveConditions: []string{"fatigued"},
        },
    }
    rng := &stubRoller{}
    inventory.ApplyConsumable(target, def, rng)
    if len(target.removed) != 1 || target.removed[0] != "fatigued" {
        t.Errorf("removed = %v, want [fatigued]", target.removed)
    }
}

func TestApplyConsumable_TeamMultiplierProperty(t *testing.T) {
    // Property: same-team always heals >= no-team; opposing-team always heals <= no-team.
    rapid.Check(t, func(rt *rapid.T) {
        rollVal := rapid.IntRange(1, 20).Draw(rt, "roll")
        rng := &stubRoller{d6Val: rollVal}
        def := &inventory.ItemDef{
            ID:   "test_item",
            Team: "gun",
            Effect: &inventory.ConsumableEffect{Heal: "1d6"},
        }
        sameTeamTarget := &mockTarget{team: "gun"}
        noTeamTarget := &mockTarget{team: ""}
        opposingTarget := &mockTarget{team: "machete"}

        inventory.ApplyConsumable(sameTeamTarget, def, &stubRoller{d6Val: rollVal})
        inventory.ApplyConsumable(noTeamTarget, def, &stubRoller{d6Val: rollVal})
        inventory.ApplyConsumable(opposingTarget, def, &stubRoller{d6Val: rollVal})

        if sameTeamTarget.healed < noTeamTarget.healed {
            rt.Fatalf("same-team heal (%d) < neutral heal (%d) for roll %d",
                sameTeamTarget.healed, noTeamTarget.healed, rollVal)
        }
        if opposingTarget.healed > noTeamTarget.healed {
            rt.Fatalf("opposing-team heal (%d) > neutral heal (%d) for roll %d",
                opposingTarget.healed, noTeamTarget.healed, rollVal)
        }
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestApplyConsumable" -v 2>&1 | head -20
```

Expected: compile error — `ConsumableEffect` etc. undefined

- [ ] **Step 3: Implement consumable.go**

```go
// internal/game/inventory/consumable.go
package inventory

import (
    "math"
    "time"
)

// ConsumableEffect describes what happens when a consumable item is used.
type ConsumableEffect struct {
    Heal             string            `yaml:"heal,omitempty"`
    Conditions       []ConditionEffect `yaml:"conditions,omitempty"`
    RemoveConditions []string          `yaml:"remove_conditions,omitempty"`
    ConsumeCheck     *ConsumeCheck     `yaml:"consume_check,omitempty"`
    RepairField      bool              `yaml:"repair_field,omitempty"`
}

// ConditionEffect applies a condition for a duration.
type ConditionEffect struct {
    ConditionID string `yaml:"condition_id"`
    Duration    string `yaml:"duration"`
}

// ConsumeCheck applies a d20 stat check with a possible critical failure effect.
type ConsumeCheck struct {
    Stat              string             `yaml:"stat"`
    DC                int                `yaml:"dc"`
    OnCriticalFailure *CritFailureEffect `yaml:"on_critical_failure,omitempty"`
}

// CritFailureEffect holds effects applied on a PF2E critical failure (≤ DC-10 or natural 1).
type CritFailureEffect struct {
    Conditions   []ConditionEffect `yaml:"conditions,omitempty"`
    ApplyDisease *DiseaseEffect    `yaml:"apply_disease,omitempty"`
    ApplyToxin   *ToxinEffect      `yaml:"apply_toxin,omitempty"`
}

// DiseaseEffect applies a disease at a given severity.
type DiseaseEffect struct {
    DiseaseID string `yaml:"disease_id"`
    Severity  int    `yaml:"severity"`
}

// ToxinEffect applies a toxin at a given severity.
type ToxinEffect struct {
    ToxinID  string `yaml:"toxin_id"`
    Severity int    `yaml:"severity"`
}

// ConsumableTarget is the minimal interface required to apply consumable effects.
// Implemented by *session.PlayerSession. REQ-EM-45: must NOT import session package.
type ConsumableTarget interface {
    GetTeam() string
    // GetStatMod returns the character's modifier for the named stat (e.g., "constitution").
    // Used by consume_check (REQ-EM-41). Returns 0 for unknown stat names.
    GetStatMod(stat string) int
    ApplyHeal(amount int)
    ApplyCondition(conditionID string, duration time.Duration)
    RemoveCondition(conditionID string)
    ApplyDisease(diseaseID string, severity int)
    ApplyToxin(toxinID string, severity int)
}

// ConsumableResult holds resolved effect data for display and audit.
type ConsumableResult struct {
    HealApplied        int
    ConditionsApplied  []string
    ConditionsRemoved  []string
    DiseaseApplied     string
    ToxinApplied       string
    TeamMultiplier     float64
    ConsumeCheckResult string // "success" | "failure" | "critical_failure" | "not_checked"
}

// teamMultiplier returns the effectiveness multiplier for the given player/item team pair.
// REQ-EM-37, REQ-EM-38, REQ-EM-39.
func teamMultiplier(playerTeam, itemTeam string) float64 {
    if itemTeam == "" {
        return 1.0
    }
    if playerTeam == itemTeam {
        return 1.25
    }
    // Opposing teams: gun vs machete.
    return 0.75
}

// ApplyConsumable applies all effects from def.Effect to target.
// Precondition: def is non-nil; def.Effect may be nil (no-op).
// Postcondition: returns ConsumableResult; all effect values floored after team multiplier.
// REQ-EM-45: pure w.r.t. target (calls only ConsumableTarget methods).
func ApplyConsumable(target ConsumableTarget, def *ItemDef, rng Roller) ConsumableResult {
    result := ConsumableResult{ConsumeCheckResult: "not_checked"}
    if def.Effect == nil {
        return result
    }
    mult := teamMultiplier(target.GetTeam(), def.Team)
    result.TeamMultiplier = mult

    // REQ-EM-43: Remove conditions before applying new ones.
    for _, condID := range def.Effect.RemoveConditions {
        target.RemoveCondition(condID)
        result.ConditionsRemoved = append(result.ConditionsRemoved, condID)
    }

    // Apply heal.
    if def.Effect.Heal != "" {
        base := rng.Roll(def.Effect.Heal)
        applied := int(math.Floor(float64(base) * mult))
        target.ApplyHeal(applied)
        result.HealApplied = applied
    }

    // Apply conditions.
    for _, ce := range def.Effect.Conditions {
        d, err := time.ParseDuration(ce.Duration)
        if err != nil {
            d = time.Hour // fallback; YAML validation at load should prevent this
        }
        scaledSec := int(math.Floor(d.Seconds() * mult))
        target.ApplyCondition(ce.ConditionID, time.Duration(scaledSec)*time.Second)
        result.ConditionsApplied = append(result.ConditionsApplied, ce.ConditionID)
    }

    // Consume check. REQ-EM-41: d20 + stat modifier vs DC.
    if def.Effect.ConsumeCheck != nil {
        statMod := target.GetStatMod(def.Effect.ConsumeCheck.Stat)
        result.ConsumeCheckResult = applyConsumeCheck(target, def.Effect.ConsumeCheck, statMod, mult, rng)
    }

    return result
}

// applyConsumeCheck performs the d20 + stat mod vs DC check (REQ-EM-41).
// statMod is the character's modifier for check.Stat (e.g., constitution mod).
// The caller is responsible for computing statMod from the character's ability scores.
func applyConsumeCheck(target ConsumableTarget, check *ConsumeCheck, statMod int, mult float64, rng Roller) string {
    roll := rng.RollD20()
    total := roll + statMod
    critFailDC := check.DC - 10
    if roll == 1 || total <= critFailDC {
        if check.OnCriticalFailure != nil {
            applyOnCritFailure(target, check.OnCriticalFailure, mult)
        }
        return "critical_failure"
    }
    if total < check.DC {
        return "failure"
    }
    return "success"
}

func applyOnCritFailure(target ConsumableTarget, eff *CritFailureEffect, mult float64) {
    for _, ce := range eff.Conditions {
        d, _ := time.ParseDuration(ce.Duration)
        scaledSec := int(math.Floor(d.Seconds() * mult))
        target.ApplyCondition(ce.ConditionID, time.Duration(scaledSec)*time.Second)
    }
    if eff.ApplyDisease != nil {
        target.ApplyDisease(eff.ApplyDisease.DiseaseID, eff.ApplyDisease.Severity)
    }
    if eff.ApplyToxin != nil {
        target.ApplyToxin(eff.ApplyToxin.ToxinID, eff.ApplyToxin.Severity)
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestApplyConsumable" -v 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/consumable.go internal/game/inventory/consumable_test.go
git commit -m "feat(equipment-mechanics): add ConsumableEffect engine + ConsumableTarget interface (REQ-EM-37-45)"
```

---

## Task 14: PlayerSession.Team + ConsumableTarget Implementation

**Files:**
- Modify: `internal/game/session/player_session.go`
- Modify: session loader (DB → PlayerSession)

- [ ] **Step 1: Read PlayerSession struct**

```bash
cd /home/cjohannsen/src/mud && grep -n "type PlayerSession\|Team\|SetBonus" internal/game/session/player_session.go | head -20
```

- [ ] **Step 2: Write failing test**

```go
// internal/game/session/player_session_team_test.go
package session_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestPlayerSession_ImplementsConsumableTarget(t *testing.T) {
    // This test verifies the interface is satisfied at compile time.
    var _ interface {
        GetTeam() string
    } = (*session.PlayerSession)(nil)
}
```

- [ ] **Step 3: Add Team to PlayerSession (SetBonuses added in Task 16)**

> **Ordering note:** `inventory.SetBonusSummary` is defined in Task 16. Add only `Team` to `PlayerSession` here. The `SetBonuses` field is added in Task 16, Step 1 after `SetBonusSummary` exists.

In `internal/game/session/manager.go`, add to `PlayerSession` struct:
```go
// Team is the player's faction affiliation ("gun" | "machete" | "").
// Loaded from characters.team at login. Empty = neutral (1.0× consumable multiplier).
Team string
```

Import `"github.com/cory-johannsen/mud/internal/game/inventory"`.

Add `ConsumableTarget` interface methods to `PlayerSession`. `PlayerSession` already has `Conditions *condition.ActiveSet` (line 80 of `manager.go`) which provides `Apply` and `Remove`. Use those directly:

```go
// GetTeam implements inventory.ConsumableTarget.
func (s *PlayerSession) GetTeam() string { return s.Team }

// GetStatMod implements inventory.ConsumableTarget.
// Returns the ability modifier (score-10)/2 for the named stat.
func (s *PlayerSession) GetStatMod(stat string) int {
    var score int
    switch stat {
    case "brutality":
        score = s.Abilities.Brutality
    case "grit":
        score = s.Abilities.Grit
    case "quickness":
        score = s.Abilities.Quickness
    case "reasoning":
        score = s.Abilities.Reasoning
    case "savvy":
        score = s.Abilities.Savvy
    case "flair":
        score = s.Abilities.Flair
    case "constitution":
        // constitution maps to grit in Gunchete world
        score = s.Abilities.Grit
    default:
        return 0
    }
    return (score - 10) / 2
}

// ApplyHeal implements inventory.ConsumableTarget.
func (s *PlayerSession) ApplyHeal(amount int) {
    s.CurrentHP += amount
    if s.CurrentHP > s.MaxHP {
        s.CurrentHP = s.MaxHP
    }
}

// ApplyCondition implements inventory.ConsumableTarget.
// Uses the existing condition.ActiveSet on the session.
func (s *PlayerSession) ApplyCondition(conditionID string, duration time.Duration) {
    if s.Conditions == nil {
        return
    }
    // duration in seconds as stacks for out-of-combat conditions
    durationSec := int(duration.Seconds())
    if durationSec < 1 {
        durationSec = 1
    }
    // condition.ActiveSet.Apply takes (uid string, def *ConditionDef, stacks, duration int).
    // The conditionDef must be looked up from the condition registry.
    // This method cannot look up the registry itself (no access). The caller (gameserver handler)
    // that wraps ApplyConsumable must call ApplyCondition via a shim that resolves the def.
    // For the ConsumableTarget interface, we store the request for the handler to process.
    // Use a pending-conditions list instead:
    s.pendingConditions = append(s.pendingConditions, pendingCondition{ID: conditionID, Duration: durationSec})
}

// RemoveCondition implements inventory.ConsumableTarget.
func (s *PlayerSession) RemoveCondition(conditionID string) {
    if s.Conditions != nil {
        s.Conditions.Remove(s.UID, conditionID)
    }
}

// ApplyDisease implements inventory.ConsumableTarget.
// Stores as a pending condition for the handler to apply via the condition registry.
func (s *PlayerSession) ApplyDisease(diseaseID string, severity int) {
    s.pendingConditions = append(s.pendingConditions, pendingCondition{ID: diseaseID, Severity: severity})
}

// ApplyToxin implements inventory.ConsumableTarget.
func (s *PlayerSession) ApplyToxin(toxinID string, severity int) {
    s.pendingConditions = append(s.pendingConditions, pendingCondition{ID: toxinID, Severity: severity})
}
```

Add the helper struct to `manager.go`:
```go
// pendingCondition holds a condition/disease/toxin application queued during ApplyConsumable.
// The gameserver handler drains this list after ApplyConsumable returns.
type pendingCondition struct {
    ID       string
    Duration int // seconds; 0 means use severity-based lookup
    Severity int
}
```

Add field to `PlayerSession`:
```go
// pendingConditions queues condition/disease/toxin applications from consumable use.
// Drained by the `use` command handler after ApplyConsumable returns.
pendingConditions []pendingCondition
```

In `handleUse` (Task 15), after calling `ApplyConsumable`, drain `sess.pendingConditions`:
```go
// Drain pending conditions through the condition registry.
for _, pc := range sess.pendingConditions {
    condDef := s.condRegistry.Get(pc.ID)
    if condDef == nil {
        continue
    }
    dur := pc.Duration
    if dur == 0 {
        dur = pc.Severity * 60 // severity-based fallback
    }
    _ = sess.Conditions.Apply(sess.UID, condDef, 1, dur)
}
sess.pendingConditions = nil
```

> Note: `s.condRegistry` uses the actual condition registry field name. Check `internal/gameserver/grpc_service.go` for the field name (likely `s.condRegistry` based on existing usage). Adapt accordingly.

- [ ] **Step 4: Load Team from DB at login**

Find the character load path. After loading character fields:
```go
sess.Team = characterRow.Team // from characters.team column
```

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/player_session.go
git commit -m "feat(equipment-mechanics): add Team + SetBonuses to PlayerSession; ConsumableTarget methods (REQ-EM-39)"
```

---

## Task 15: Wire `use <item>` Command to Consumable Engine

**Files:**
- Create: `internal/gameserver/grpc_service_use.go` (replace stub)

- [ ] **Step 1: Find the existing use command stub**

```bash
cd /home/cjohannsen/src/mud && grep -rn "use\|Activating" internal/gameserver/ --include="*.go" | grep -i "activating\|use" | head -10
```

- [ ] **Step 2: Write failing test**

```go
// internal/gameserver/grpc_service_use_test.go
package gameserver_test

import (
    "testing"
)

func TestHandleUse_NoSuchItem_Fails(t *testing.T) {
    // Session has no "penjamin_franklin" in backpack.
    // Assert: message = "You don't have penjamin_franklin."
    t.Skip("REPLACE with actual server test setup")
}

func TestHandleUse_HealItem_RestoresHP(t *testing.T) {
    // Session has penjamin_franklin in backpack; HP < MaxHP.
    // After use, HP increases.
    t.Skip("REPLACE with actual server test setup")
}
```

- [ ] **Step 3: Implement handleUse**

Create or replace `grpc_service_use.go`:

```go
// internal/gameserver/grpc_service_use.go
package gameserver

import (
    "context"
    "fmt"

    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

// handleUse processes a `use <item>` command.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleUse(uid string, req *gamev1.UseRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    itemID := req.GetItemId()

    // Find item in backpack.
    inst := sess.Backpack.FindItemInstance(itemID)
    if inst == nil {
        return messageEvent(fmt.Sprintf("You don't have %s.", itemID)), nil
    }

    def := s.itemRegistry.Get(itemID)
    if def == nil {
        return messageEvent(fmt.Sprintf("Unknown item: %s.", itemID)), nil
    }
    if def.Effect == nil || def.Effect.RepairField {
        return messageEvent(fmt.Sprintf("You can't use %s that way. Use 'repair <item>' for repair kits.", def.Name)), nil
    }

    rng := inventory.NewDiceRoller()
    result := inventory.ApplyConsumable(sess, def, rng)

    // Consume one unit.
    sess.Backpack.RemoveItem(itemID, 1)

    // Persist.
    if s.charSaver != nil && sess.CharacterID > 0 {
        _ = s.charSaver.SaveInventory(context.Background(), sess.CharacterID, sess.Backpack)
    }

    msg := buildUseMessage(def.Name, result)
    return messageEvent(msg), nil
}

// buildUseMessage constructs a human-readable result string from a ConsumableResult.
func buildUseMessage(name string, r inventory.ConsumableResult) string {
    msg := fmt.Sprintf("You use %s.", name)
    if r.HealApplied > 0 {
        msg += fmt.Sprintf(" Restored %d HP.", r.HealApplied)
    }
    for _, c := range r.ConditionsApplied {
        msg += fmt.Sprintf(" Applied: %s.", c)
    }
    for _, c := range r.ConditionsRemoved {
        msg += fmt.Sprintf(" Removed: %s.", c)
    }
    if r.ConsumeCheckResult == "critical_failure" {
        msg += " [Critical failure on consume check!]"
    }
    return msg
}
```

- [ ] **Step 4: Wire `use` command in the handler switch**

Replace the existing stub `"use"` case with a call to `handleUse`.

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service_use.go internal/gameserver/grpc_service_use_test.go
git commit -m "feat(equipment-mechanics): wire use command to consumable engine (REQ-EM-37-43)"
```

---

## Task 16: SetRegistry — Equipment Set Bonuses

**Files:**
- Create: `internal/game/inventory/set_registry.go`
- Create: `internal/game/inventory/set_registry_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/game/inventory/set_registry_test.go
package inventory_test

import (
    "testing"
    "pgregory.net/rapid"
    "github.com/cory-johannsen/mud/internal/game/inventory"
)

var testSetDef = inventory.SetDef{
    ID:   "street_rat_set",
    Name: "Street Rat's Outfit",
    Pieces: []inventory.SetPiece{
        {ItemDefID: "street_jacket"},
        {ItemDefID: "street_trousers"},
        {ItemDefID: "street_boots"},
    },
    Bonuses: []inventory.SetBonus{
        {
            Threshold:   inventory.SetThreshold{Count: 2},
            Description: "+1 Stealth",
            Effect:      inventory.SetEffect{Type: "skill_bonus", Skill: "stealth", Amount: 1},
        },
        {
            Threshold:   inventory.SetThreshold{IsFull: true},
            Description: "+1 AC",
            Effect:      inventory.SetEffect{Type: "ac_bonus", Amount: 1},
        },
    },
}

func TestActiveBonuses_NoPieces_NoBonus(t *testing.T) {
    reg := inventory.NewSetRegistryFromDefs([]inventory.SetDef{testSetDef})
    bonuses := reg.ActiveBonuses([]string{})
    if len(bonuses) != 0 {
        t.Errorf("expected 0 bonuses, got %d", len(bonuses))
    }
}

func TestActiveBonuses_TwoPieces_SkillBonus(t *testing.T) {
    reg := inventory.NewSetRegistryFromDefs([]inventory.SetDef{testSetDef})
    bonuses := reg.ActiveBonuses([]string{"street_jacket", "street_trousers"})
    if len(bonuses) != 1 {
        t.Fatalf("expected 1 bonus, got %d", len(bonuses))
    }
    if bonuses[0].Effect.Type != "skill_bonus" {
        t.Errorf("expected skill_bonus, got %q", bonuses[0].Effect.Type)
    }
}

func TestActiveBonuses_FullSet_BothBonuses(t *testing.T) {
    reg := inventory.NewSetRegistryFromDefs([]inventory.SetDef{testSetDef})
    bonuses := reg.ActiveBonuses([]string{"street_jacket", "street_trousers", "street_boots"})
    if len(bonuses) != 2 {
        t.Fatalf("expected 2 bonuses, got %d", len(bonuses))
    }
}

func TestActiveBonuses_IsPure(t *testing.T) {
    reg := inventory.NewSetRegistryFromDefs([]inventory.SetDef{testSetDef})
    rapid.Check(t, func(rt *rapid.T) {
        ids := rapid.SliceOfDistinct(
            rapid.SampledFrom([]string{"street_jacket", "street_trousers", "street_boots", "other_item"}),
            func(s string) string { return s },
        ).Draw(rt, "ids")
        b1 := reg.ActiveBonuses(ids)
        b2 := reg.ActiveBonuses(ids)
        if len(b1) != len(b2) {
            rt.Fatalf("ActiveBonuses not pure: %d != %d for same input", len(b1), len(b2))
        }
    })
}

func TestSetThreshold_FullResolvesToLen(t *testing.T) {
    def := inventory.SetDef{
        Pieces: []inventory.SetPiece{{ItemDefID: "a"}, {ItemDefID: "b"}, {ItemDefID: "c"}},
        Bonuses: []inventory.SetBonus{
            {Threshold: inventory.SetThreshold{IsFull: true}},
        },
    }
    reg := inventory.NewSetRegistryFromDefs([]inventory.SetDef{def})
    bonuses := reg.ActiveBonuses([]string{"a", "b", "c"})
    if len(bonuses) != 1 {
        t.Errorf("full threshold with 3 pieces: expected 1 bonus at 3 equipped, got %d", len(bonuses))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestActiveBonuses|TestSetThreshold" -v 2>&1 | head -20
```

Expected: compile error

- [ ] **Step 3: Implement set_registry.go**

```go
// internal/game/inventory/set_registry.go
package inventory

import (
    "fmt"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// SetThreshold represents either a numeric count or "full".
type SetThreshold struct {
    IsFull bool
    Count  int
}

// UnmarshalYAML implements yaml.Unmarshaler for SetThreshold.
// Accepts either an integer or the string "full".
func (s *SetThreshold) UnmarshalYAML(value *yaml.Node) error {
    var n int
    if err := value.Decode(&n); err == nil {
        s.Count = n
        return nil
    }
    var str string
    if err := value.Decode(&str); err == nil && str == "full" {
        s.IsFull = true
        return nil
    }
    return fmt.Errorf("set threshold must be an integer or \"full\", got %q", value.Value)
}

// SetPiece references an item def in a set.
type SetPiece struct {
    ItemDefID string `yaml:"item_def_id"`
}

// SetEffect describes what a set bonus does.
type SetEffect struct {
    Type        string `yaml:"type"`
    Skill       string `yaml:"skill,omitempty"`
    Stat        string `yaml:"stat,omitempty"`
    ConditionID string `yaml:"condition_id,omitempty"`
    Amount      int    `yaml:"amount,omitempty"`
}

// SetBonus represents a threshold bonus within a set.
type SetBonus struct {
    Threshold   SetThreshold `yaml:"threshold"`
    Description string       `yaml:"description"`
    Effect      SetEffect    `yaml:"effect"`
}

// SetDef is the full definition of an equipment set.
type SetDef struct {
    ID      string     `yaml:"id"`
    Name    string     `yaml:"name"`
    Pieces  []SetPiece `yaml:"pieces"`
    Bonuses []SetBonus `yaml:"bonuses"`
}

// SetRegistry holds loaded set definitions.
type SetRegistry struct {
    defs []resolvedSetDef
}

type resolvedSetDef struct {
    def      SetDef
    pieceIDs map[string]struct{} // set for O(1) lookup
}

// NewSetRegistryFromDefs builds a SetRegistry from pre-loaded SetDefs (used in tests).
// REQ-EM-28: "full" threshold resolves to len(pieces).
func NewSetRegistryFromDefs(defs []SetDef) *SetRegistry {
    reg := &SetRegistry{}
    for _, d := range defs {
        rsd := resolvedSetDef{def: d, pieceIDs: make(map[string]struct{}, len(d.Pieces))}
        for _, p := range d.Pieces {
            rsd.pieceIDs[p.ItemDefID] = struct{}{}
        }
        // Resolve "full" thresholds.
        for i := range rsd.def.Bonuses {
            if rsd.def.Bonuses[i].Threshold.IsFull {
                rsd.def.Bonuses[i].Threshold.Count = len(d.Pieces)
            }
        }
        reg.defs = append(reg.defs, rsd)
    }
    return reg
}

// NewSetRegistryFromDir loads all YAML files from dir and returns a SetRegistry.
// condExists is called to validate condition_immunity effect IDs (REQ-EM-33); pass nil to skip.
// Fatal if any file fails to load, has an unrecognized effect type (REQ-EM-32), or has an
// unresolvable condition_id (REQ-EM-33).
func NewSetRegistryFromDir(dir string, condExists ConditionExistsFn) (*SetRegistry, error) {
    files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
    if err != nil {
        return nil, err
    }
    var defs []SetDef
    for _, f := range files {
        data, err := os.ReadFile(f)
        if err != nil {
            return nil, fmt.Errorf("set registry: read %s: %w", f, err)
        }
        var d SetDef
        if err := yaml.Unmarshal(data, &d); err != nil {
            return nil, fmt.Errorf("set registry: parse %s: %w", f, err)
        }
        if err := validateSetDef(&d, condExists); err != nil {
            return nil, fmt.Errorf("set registry: validate %s: %w", f, err)
        }
        defs = append(defs, d)
    }
    return NewSetRegistryFromDefs(defs), nil
}

var validSetEffectTypes = map[string]struct{}{
    "skill_bonus": {}, "ac_bonus": {}, "speed_bonus": {}, "stat_bonus": {}, "condition_immunity": {},
}

// ConditionExistsFn is a function type for condition ID existence checks.
// Used by NewSetRegistryFromDir to validate condition_immunity references (REQ-EM-33).
type ConditionExistsFn func(conditionID string) bool

func validateSetDef(d *SetDef, condExists ConditionExistsFn) error {
    for _, b := range d.Bonuses {
        if _, ok := validSetEffectTypes[b.Effect.Type]; !ok {
            return fmt.Errorf("set %q: unrecognized effect type %q (REQ-EM-32)", d.ID, b.Effect.Type)
        }
        // REQ-EM-33: Validate condition_id for condition_immunity effects.
        if b.Effect.Type == "condition_immunity" && condExists != nil {
            if !condExists(b.Effect.ConditionID) {
                return fmt.Errorf("set %q: condition_immunity references unknown condition_id %q (REQ-EM-33)", d.ID, b.Effect.ConditionID)
            }
        }
    }
    return nil
}

// NewSetRegistryFromDir signature updated to accept a condition existence checker.
// Pass nil for condExists to skip REQ-EM-33 validation (e.g., in tests without a condition registry).

// ActiveBonuses returns all bonuses whose thresholds are met by equippedItemIDs.
// REQ-EM-34: pure function — no side effects.
func (r *SetRegistry) ActiveBonuses(equippedItemIDs []string) []SetBonus {
    equipped := make(map[string]struct{}, len(equippedItemIDs))
    for _, id := range equippedItemIDs {
        equipped[id] = struct{}{}
    }
    var result []SetBonus
    for _, rsd := range r.defs {
        count := 0
        for id := range rsd.pieceIDs {
            if _, ok := equipped[id]; ok {
                count++
            }
        }
        for _, b := range rsd.def.Bonuses {
            if count >= b.Threshold.Count {
                result = append(result, b)
            }
        }
    }
    return result
}

// SetBonusSummary aggregates all active set bonuses into discrete effect buckets.
type SetBonusSummary struct {
    SkillBonuses        map[string]int
    SpeedBonus          int
    StatBonuses         map[string]int
    ConditionImmunities []string
}

// BuildSetBonusSummary computes the summary from a slice of active bonuses.
// REQ-EM-35.
func BuildSetBonusSummary(bonuses []SetBonus) SetBonusSummary {
    s := SetBonusSummary{
        SkillBonuses: make(map[string]int),
        StatBonuses:  make(map[string]int),
    }
    for _, b := range bonuses {
        switch b.Effect.Type {
        case "skill_bonus":
            s.SkillBonuses[b.Effect.Skill] += b.Effect.Amount
        case "speed_bonus":
            s.SpeedBonus += b.Effect.Amount
        case "stat_bonus":
            s.StatBonuses[b.Effect.Stat] += b.Effect.Amount
        case "condition_immunity":
            s.ConditionImmunities = append(s.ConditionImmunities, b.Effect.ConditionID)
        }
    }
    return s
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -run "TestActiveBonuses|TestSetThreshold" -v 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/set_registry.go internal/game/inventory/set_registry_test.go
git commit -m "feat(equipment-mechanics): add SetRegistry with pure ActiveBonuses (REQ-EM-28-35)"
```

---

## Task 17: Wire Set Bonuses into ComputedDefenses and PlayerSession

**Files:**
- Modify: `internal/game/inventory/equipment.go`
- Modify: `internal/game/session/player_session.go`
- Modify: `internal/gameserver/` login path
- Modify: `internal/game/inventory/providers.go`

- [ ] **Step 1: Add SetBonuses field to PlayerSession + SetRegistry to providers.go**

Now that `inventory.SetBonusSummary` exists (defined in Task 16's `set_registry.go`), add it to `PlayerSession` in `internal/game/session/manager.go`:

```go
// SetBonuses holds aggregated active equipment set bonuses. Rebuilt at login and on equip/unequip.
SetBonuses inventory.SetBonusSummary
```

Import `"github.com/cory-johannsen/mud/internal/game/inventory"` in `manager.go` if not already present.

Then add `SetRegistry` to `internal/game/inventory/providers.go`:
```go
// ProvideSetRegistry loads set definitions from content/sets/.
// condExists is used to validate condition_immunity condition IDs at load (REQ-EM-33).
// Pass nil to skip validation (not recommended for production).
func ProvideSetRegistry(contentDir string, condExists ConditionExistsFn) (*SetRegistry, error) {
    return NewSetRegistryFromDir(filepath.Join(contentDir, "sets"), condExists)
}
```

> Note: `ConditionExistsFn` is `func(conditionID string) bool`. The production caller must pass a function that checks against the condition registry (e.g., `func(id string) bool { return condReg.Get(id) != nil }`). Wire this in `cmd/gameserver/wire.go` or the equivalent Wire provider set.

Add to `Providers` wire set:
```go
Providers = wire.NewSet(NewRegistryFromDirs, NewFloorManager, NewSeededRoomEquipmentManager, ProvideSetRegistry)
```

- [ ] **Step 2: Rebuild SetBonusSummary at login and equipment change**

In the session load path, after loading equipment:
```go
equippedIDs := sess.Equipment.EquippedItemDefIDs()
bonuses := setReg.ActiveBonuses(equippedIDs)
sess.SetBonuses = inventory.BuildSetBonusSummary(bonuses)
```

Add `EquippedItemDefIDs() []string` to `Equipment` in `equipment.go`:
```go
func (e *Equipment) EquippedItemDefIDs() []string {
    var ids []string
    for _, slot := range e.Slots {
        if slot.ItemDefID != "" {
            ids = append(ids, slot.ItemDefID)
        }
    }
    return ids
}
```

Similarly call `BuildSetBonusSummary` after equip/unequip command handlers.

- [ ] **Step 3: Apply AC bonus from sets in ComputedDefenses**

In `ComputedDefenses`, after summing per-slot AC:
```go
// REQ-EM-35: Apply set bonus AC.
// SetBonusSummary is passed in or accessed from session.
totalAC += setBonusSummary.ACBonus // sum ac_bonus bonuses
```

> Note: `ac_bonus` set bonuses are NOT stored in `SetBonusSummary`'s map — they contribute directly to `DefenseStats.ACBonus`. Accumulate them in `ComputedDefenses` by iterating the active bonuses or adding an `ACBonus int` field to `SetBonusSummary`.

Add `ACBonus int` to `SetBonusSummary` in `set_registry.go` and compute it in `BuildSetBonusSummary`:
```go
case "ac_bonus":
    s.ACBonus += b.Effect.Amount
```

- [ ] **Step 4: Create content/sets directory and sample set YAML**

```bash
mkdir -p content/sets
```

Create `content/sets/street_rat_set.yaml`:
```yaml
id: street_rat_set
name: "Street Rat's Outfit"
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

> Note: The item_def_ids must match actual items in the content directory. Verify `street_jacket`, `street_trousers`, `street_boots`, `street_gloves` exist in `content/armor/`. If they don't, adjust the set to use IDs that do exist, or create minimal stubs.

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/inventory/ internal/game/session/ internal/gameserver/ content/sets/
git commit -m "feat(equipment-mechanics): wire SetRegistry into ComputedDefenses + PlayerSession (REQ-EM-29, REQ-EM-35)"
```

---

## Task 18: New Consumable YAML Files

**Files:**
- Create: `content/consumables/whores_pasta.yaml`
- Create: `content/consumables/poontangesca.yaml`
- Create: `content/consumables/four_loko.yaml`
- Create: `content/consumables/old_english.yaml`
- Create: `content/consumables/penjamin_franklin.yaml`
- Create: `content/consumables/repair_kit.yaml`

- [ ] **Step 1: Create content/consumables directory and all six YAML files**

```bash
mkdir -p content/consumables
```

`content/consumables/whores_pasta.yaml`:
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

`content/consumables/poontangesca.yaml`:
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

`content/consumables/four_loko.yaml`:
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

`content/consumables/old_english.yaml`:
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

`content/consumables/penjamin_franklin.yaml`:
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

`content/consumables/repair_kit.yaml`:
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
  repair_field: true
```

- [ ] **Step 2: Wire consumables directory into ItemDef loader**

Find where item YAML files are loaded. Add `content/consumables/` to the scan directories. Validate referenced `condition_id`, `disease_id`, and `toxin_id` fields against `content/conditions/` at startup (REQ-EM-42):

```go
if def.Effect != nil {
    if err := validateConsumableEffect(def.Effect, condRegistry); err != nil {
        log.Fatalf("fatal: item %q: %v", def.ID, err)
    }
}
```

```go
func validateConsumableEffect(eff *inventory.ConsumableEffect, condReg ConditionRegistry) error {
    for _, ce := range eff.Conditions {
        if !condReg.Exists(ce.ConditionID) {
            return fmt.Errorf("unknown condition_id %q", ce.ConditionID)
        }
    }
    if eff.ConsumeCheck != nil && eff.ConsumeCheck.OnCriticalFailure != nil {
        cf := eff.ConsumeCheck.OnCriticalFailure
        if cf.ApplyDisease != nil && !condReg.Exists(cf.ApplyDisease.DiseaseID) {
            return fmt.Errorf("unknown disease_id %q", cf.ApplyDisease.DiseaseID)
        }
        if cf.ApplyToxin != nil && !condReg.Exists(cf.ApplyToxin.ToxinID) {
            return fmt.Errorf("unknown toxin_id %q", cf.ApplyToxin.ToxinID)
        }
    }
    return nil
}
```

> Note: The condition IDs referenced (`strength_boost_1`, `speed_boost_2`, `attack_bonus_1`, `fortitude_bonus_1`, `sickened`, `street_rot`, `gut_rot`) must exist in `content/conditions/`. Check what conditions exist and adjust YAML files or create stubs accordingly.

- [ ] **Step 3: Verify startup succeeds**

```bash
cd /home/cjohannsen/src/mud && go build ./cmd/gameserver/ 2>&1 && echo "BUILD OK"
```

Expected: `BUILD OK`

- [ ] **Step 4: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/consumables/
git commit -m "feat(equipment-mechanics): add 6 new consumable YAML files (REQ-EM-40)"
```

---

## Task 19: Character Sheet Display — Set Bonuses + Rarity/Modifier Color in Inventory

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go` — character sheet equipment section and inventory display
- Modify: `internal/gameserver/grpc_service.go` — `handleEquipment` (~line 3840) for equipment listing

- [ ] **Step 1: Find display code**

```bash
cd /home/cjohannsen/src/mud && grep -rn "RenderCharacterSheet\|renderCharacterSheet\|equipment.*display\|slot.*display" internal/ --include="*.go" | head -15
```

- [ ] **Step 2: Write failing test**

```go
func TestRenderEquipmentWithSetBonus_ContainsBonus(t *testing.T) {
    // Create a session with active set bonus (e.g., "+1 Stealth").
    // Render character sheet.
    // Assert output contains set bonus description.
    t.Skip("REPLACE with actual render call")
}
```

- [ ] **Step 3: Add set bonus section to character sheet**

In the equipment section of `RenderCharacterSheet` (or equivalent), after listing slots:

```go
if len(sess.SetBonuses.SkillBonuses) > 0 || sess.SetBonuses.SpeedBonus > 0 ||
    len(sess.SetBonuses.StatBonuses) > 0 || len(sess.SetBonuses.ConditionImmunities) > 0 {
    sb.WriteString("\nActive Set Bonuses:\n")
    for skill, amt := range sess.SetBonuses.SkillBonuses {
        sb.WriteString(fmt.Sprintf("  +%d %s\n", amt, skill))
    }
    if sess.SetBonuses.SpeedBonus > 0 {
        sb.WriteString(fmt.Sprintf("  +%d Speed\n", sess.SetBonuses.SpeedBonus))
    }
    for stat, amt := range sess.SetBonuses.StatBonuses {
        sb.WriteString(fmt.Sprintf("  +%d %s\n", amt, stat))
    }
    for _, condID := range sess.SetBonuses.ConditionImmunities {
        sb.WriteString(fmt.Sprintf("  Immune: %s\n", condID))
    }
}
```

- [ ] **Step 4: Apply rarity color to slot display**

In the equipment slot rendering, replace item name with:
```go
inventory.RarityColoredName(def.Rarity, def.Name, inst)
```

In the inventory listing, similarly use `RarityColoredName`.

- [ ] **Step 5: Run full tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add -p
git commit -m "feat(equipment-mechanics): character sheet set bonus display; rarity/modifier color in inventory (REQ-EM-31, REQ-EM-4)"
```

---

## Final Verification

- [ ] **Run all tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -30
```

Expected: all PASS

- [ ] **Build all binaries**

```bash
cd /home/cjohannsen/src/mud && go build ./cmd/... 2>&1 && echo "ALL BUILDS OK"
```

Expected: `ALL BUILDS OK`

- [ ] **Requirements coverage check**

Verify each REQ-EM requirement is implemented:
- REQ-EM-1–4: Task 5 (rarity validation, stat multiplier, min level, color)
- REQ-EM-5–6: Tasks 8–9 (weapon/armor durability deduction in combat_handler.go)
- REQ-EM-7–12: Task 6 (DeductDurability pure function, destruction roll, DurabilityRegistry constants)
- REQ-EM-13–14: Task 10 (repair command, repair_kit consumption, RepairField)
- REQ-EM-15: Task 10, Step 5 (1 AP deducted in combat)
- REQ-EM-16: Task 10, Step 6 (downtime RepairFull with day cost formula)
- REQ-EM-17: Task 7 (InitDurability sentinel at login)
- REQ-EM-18–20: Task 11 (modifier display: tuned/defective/cursed names)
- REQ-EM-21–24: Tasks 8–11 (modifier effects at resolution time, ComputedDefenses, unequip block)
- REQ-EM-23: Task 8, Step 6 (modifier damage adjustment ±10%/±15%)
- REQ-EM-25: Task 11, Step 6 (UncurseItem → defective + CurseRevealed=false)
- REQ-EM-26–27: Task 11 (RollModifier probabilities, RollModifierForMerchant)
- REQ-EM-28–31: Tasks 16–17 (SetRegistry, full threshold resolution, set bonuses at login, character sheet display)
- REQ-EM-32: Task 16 (validateSetDef effect type check)
- REQ-EM-33: Task 16 (validateSetDef condition_id validation via ConditionExistsFn)
- REQ-EM-34–35: Tasks 16–17 (ActiveBonuses pure function, derived stats application)
- REQ-EM-36–39: Tasks 13–14 (ConsumableTarget.GetTeam, team multiplier, PlayerSession.Team)
- REQ-EM-40–43: Tasks 15, 18 (consumable use command, YAML files, consume_check with stat mod)
- REQ-EM-41: Task 13 (GetStatMod on ConsumableTarget; applyConsumeCheck uses stat mod)
- REQ-EM-44–45: Tasks 6, 13 (pure functions, ConsumableTarget no session import)
