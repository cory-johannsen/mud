# Player Resistances & Weaknesses Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Players gain resistances and weaknesses from equipped armor. These are shown on the character sheet and applied when NPCs attack the player in combat.

**Architecture:** Add `Resistances`/`Weaknesses map[string]int` to `ArmorDef` (YAML). Aggregate across equipped slots in `ComputedDefenses` (resistances = max per type; weaknesses = additive). Store on `PlayerSession`. Wire into the player `Combatant` in `combat_handler.go` — NPC attacks already call `applyResistanceWeakness(target, ...)` so player resistance applies automatically. Expose via proto and render on character sheet.

**Tech Stack:** Go, existing `internal/game/inventory`, `internal/game/session`, `internal/gameserver`, `internal/frontend/handlers` packages. One proto change (`CharacterSheetView`).

**PF2E stacking rules:**
- Resistances: highest value per damage type wins (no stacking)
- Weaknesses: additive across sources

---

### Task 1: Add Resistances/Weaknesses to ArmorDef

**Files:**
- Modify: `internal/game/inventory/armor.go`

**Step 1: Write failing test**

In `internal/game/inventory/armor_test.go` (check if exists, else create), package `inventory`, add:

```go
func TestArmorDef_ResistancesWeaknesses_YAML(t *testing.T) {
    input := `
id: test_armor
name: Test Armor
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: leather
proficiency_category: light_armor
resistances:
  fire: 5
  piercing: 2
weaknesses:
  electricity: 3
`
    var a ArmorDef
    require.NoError(t, yaml.Unmarshal([]byte(input), &a))
    assert.Equal(t, 5, a.Resistances["fire"])
    assert.Equal(t, 2, a.Resistances["piercing"])
    assert.Equal(t, 3, a.Weaknesses["electricity"])
}
```

Run: `go test ./internal/game/inventory/... -run TestArmorDef_Resistances -v`
Expected: compile error.

**Step 2: Implement**

In `ArmorDef` struct, add after `CrossTeamEffect`:
```go
// Resistances maps damage type → flat damage reduction granted to the wearer.
// Stacking rule: only the highest value per type across all equipped armor applies.
Resistances map[string]int `yaml:"resistances"`
// Weaknesses maps damage type → flat damage addition applied to the wearer.
// Stacking rule: additive across all equipped armor.
Weaknesses  map[string]int `yaml:"weaknesses"`
```

**Step 3: Run all inventory tests**
```bash
go test ./internal/game/inventory/... -v 2>&1 | tail -20
```

**Step 4: Commit**
```bash
git add internal/game/inventory/armor.go internal/game/inventory/armor_test.go
git commit -m "feat: add Resistances/Weaknesses to ArmorDef (YAML)"
```

---

### Task 2: Aggregate resistances/weaknesses in ComputedDefenses

**Files:**
- Modify: `internal/game/inventory/equipment.go`

**Step 1: Write failing tests**

In `internal/game/inventory/equipment_resistance_test.go` (new file), package `inventory_test`:

```go
package inventory_test

import (
    "testing"
    "github.com/cory-johannsen/mud/internal/game/inventory"
    "github.com/stretchr/testify/assert"
)

func TestComputedDefenses_Resistances_MaxWins(t *testing.T) {
    reg := inventory.NewRegistry()
    // Two torso armor defs with overlapping resistances
    _ = reg.RegisterArmor(&inventory.ArmorDef{
        ID: "vest1", Name: "Vest1", Slot: inventory.SlotTorso, Group: "leather",
        ProficiencyCategory: "light_armor",
        Resistances: map[string]int{"fire": 3, "piercing": 2},
    })
    _ = reg.RegisterArmor(&inventory.ArmorDef{
        ID: "vest2", Name: "Vest2", Slot: inventory.SlotLeftArm, Group: "leather",
        ProficiencyCategory: "light_armor",
        Resistances: map[string]int{"fire": 5},
    })

    eq := inventory.NewEquipment()
    eq.Armor[inventory.SlotTorso]   = &inventory.InventoryItem{ItemDefID: "vest1", Name: "Vest1", Kind: "armor"}
    eq.Armor[inventory.SlotLeftArm] = &inventory.InventoryItem{ItemDefID: "vest2", Name: "Vest2", Kind: "armor"}

    def := eq.ComputedDefenses(reg, 2)
    // fire: max(3, 5) = 5; piercing: only vest1 = 2
    assert.Equal(t, 5, def.Resistances["fire"])
    assert.Equal(t, 2, def.Resistances["piercing"])
}

func TestComputedDefenses_Weaknesses_Additive(t *testing.T) {
    reg := inventory.NewRegistry()
    _ = reg.RegisterArmor(&inventory.ArmorDef{
        ID: "a1", Name: "A1", Slot: inventory.SlotTorso, Group: "leather",
        ProficiencyCategory: "light_armor",
        Weaknesses: map[string]int{"electricity": 2},
    })
    _ = reg.RegisterArmor(&inventory.ArmorDef{
        ID: "a2", Name: "A2", Slot: inventory.SlotLeftArm, Group: "leather",
        ProficiencyCategory: "light_armor",
        Weaknesses: map[string]int{"electricity": 3},
    })

    eq := inventory.NewEquipment()
    eq.Armor[inventory.SlotTorso]   = &inventory.InventoryItem{ItemDefID: "a1", Name: "A1", Kind: "armor"}
    eq.Armor[inventory.SlotLeftArm] = &inventory.InventoryItem{ItemDefID: "a2", Name: "A2", Kind: "armor"}

    def := eq.ComputedDefenses(reg, 2)
    // electricity: 2 + 3 = 5
    assert.Equal(t, 5, def.Weaknesses["electricity"])
}

func TestComputedDefenses_NoResistances_EmptyMap(t *testing.T) {
    reg := inventory.NewRegistry()
    eq := inventory.NewEquipment()
    def := eq.ComputedDefenses(reg, 2)
    assert.Empty(t, def.Resistances)
    assert.Empty(t, def.Weaknesses)
}
```

Run: `go test ./internal/game/inventory/... -run TestComputedDefenses_Resist -v`
Expected: compile error (fields don't exist on DefenseStats).

**Step 2: Add fields to DefenseStats**

In `equipment.go`, in `DefenseStats`:
```go
// Resistances maps damage type → effective flat reduction (highest single-source value per type).
Resistances map[string]int
// Weaknesses maps damage type → total flat addition (sum across all sources).
Weaknesses  map[string]int
```

**Step 3: Populate in ComputedDefenses**

In `ComputedDefenses`, initialize and populate after the existing loop:

```go
stats.Resistances = make(map[string]int)
stats.Weaknesses  = make(map[string]int)

for slot, item := range e.Armor {
    if item == nil || reg == nil {
        continue
    }
    armorDef, ok := reg.Armor(item.ItemDefID)
    if !ok {
        continue
    }
    _ = slot
    for dmgType, val := range armorDef.Resistances {
        if val > stats.Resistances[dmgType] {
            stats.Resistances[dmgType] = val
        }
    }
    for dmgType, val := range armorDef.Weaknesses {
        stats.Weaknesses[dmgType] += val
    }
}
```

Note: read the existing `ComputedDefenses` carefully — it already loops over `e.Armor`. You may combine the resistance/weakness loop with the existing loop or add a second loop. Either is fine.

**Step 4: Run all inventory tests**
```bash
go test ./internal/game/inventory/... -v 2>&1 | tail -20
```

**Step 5: Commit**
```bash
git add internal/game/inventory/equipment.go internal/game/inventory/equipment_resistance_test.go
git commit -m "feat: aggregate armor resistances/weaknesses in ComputedDefenses (max/additive)"
```

---

### Task 3: Add Resistances/Weaknesses to PlayerSession and populate at login

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Add fields to PlayerSession**

In `manager.go`, in `PlayerSession` struct, add:
```go
// Resistances maps damage type → effective flat reduction from equipped armor (highest per type).
Resistances map[string]int
// Weaknesses maps damage type → total flat bonus from equipped armor (additive).
Weaknesses  map[string]int
```

**Step 2: Populate at login in grpc_service.go**

Find where `ComputedDefenses` is called at login (search for `ComputedDefenses` in the login/stream handler). After computing `def`, add:
```go
sess.Resistances = def.Resistances
sess.Weaknesses  = def.Weaknesses
```

Also find where equipment is saved/loaded after equip/unequip commands — update resistances/weaknesses there too if `ComputedDefenses` is re-called.

**Step 3: Write a test**

In `internal/gameserver/grpc_service_resistance_test.go` (new), verify that after login with armor that has resistances, `sess.Resistances` is populated. Use existing test server helpers. If too complex, document gap pointing to equipment_resistance_test.go and add a minimal compile-time check instead.

**Step 4: Build check**
```bash
go build ./... 2>&1
```

**Step 5: Run full suite**
```bash
go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Step 6: Commit**
```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go
git commit -m "feat: add Resistances/Weaknesses to PlayerSession; populate from armor at login"
```

---

### Task 4: Wire player resistances into combat handler

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

**Step 1:** Find where `playerCbt` is built. After wiring `WeaponDamageType`, add:
```go
// Wire player resistances/weaknesses from session (from equipped armor).
playerCbt.Resistances = sess.Resistances
playerCbt.Weaknesses  = sess.Weaknesses
```

NPC attacks already call `applyResistanceWeakness(target, damageType, dmg)` in `round.go` — so player resistance applies automatically once `playerCbt.Resistances` is set.

**Step 2: Build and test**
```bash
go build ./... && go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Step 3: Commit**
```bash
git add internal/gameserver/combat_handler.go
git commit -m "feat: wire player resistances/weaknesses into combat handler"
```

---

### Task 5: Proto + character sheet display

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`
- Modify: `internal/gameserver/grpc_service.go` (populate proto fields)
- Modify: `internal/frontend/handlers/text_renderer.go` (render section)

**Step 1: Add proto message and fields**

In `game.proto`, add a new message before or after `ProficiencyEntry`:
```proto
// ResistanceEntry represents one damage-type resistance or weakness.
message ResistanceEntry {
  string damage_type = 1; // e.g. "fire", "piercing"
  int32  value       = 2; // flat amount
}
```

In `CharacterSheetView`, add after `total_ac`:
```proto
repeated ResistanceEntry player_resistances = 38;
repeated ResistanceEntry player_weaknesses  = 39;
```

Run: `make proto`

**Step 2: Populate in grpc_service.go**

In the character sheet handler, after populating `TotalAc`, add:
```go
for dmgType, val := range sess.Resistances {
    view.PlayerResistances = append(view.PlayerResistances, &gamev1.ResistanceEntry{
        DamageType: dmgType,
        Value:      int32(val),
    })
}
sort.Slice(view.PlayerResistances, func(i, j int) bool {
    return view.PlayerResistances[i].DamageType < view.PlayerResistances[j].DamageType
})
for dmgType, val := range sess.Weaknesses {
    view.PlayerWeaknesses = append(view.PlayerWeaknesses, &gamev1.ResistanceEntry{
        DamageType: dmgType,
        Value:      int32(val),
    })
}
sort.Slice(view.PlayerWeaknesses, func(i, j int) bool {
    return view.PlayerWeaknesses[i].DamageType < view.PlayerWeaknesses[j].DamageType
})
```

(`sort` is already imported or add it.)

**Step 3: Render on character sheet**

In `text_renderer.go`, in `RenderCharacterSheet`, after the Defense section (after the `acLine` append), add:

```go
if len(csv.GetPlayerResistances()) > 0 {
    parts := make([]string, 0, len(csv.GetPlayerResistances()))
    for _, r := range csv.GetPlayerResistances() {
        parts = append(parts, fmt.Sprintf("%s %d", r.GetDamageType(), r.GetValue()))
    }
    left = append(left, slPlain(fmt.Sprintf("Resist: %s", strings.Join(parts, "  "))))
}
if len(csv.GetPlayerWeaknesses()) > 0 {
    parts := make([]string, 0, len(csv.GetPlayerWeaknesses()))
    for _, r := range csv.GetPlayerWeaknesses() {
        parts = append(parts, fmt.Sprintf("%s %d", r.GetDamageType(), r.GetValue()))
    }
    left = append(left, slPlain(fmt.Sprintf("Weak:   %s", strings.Join(parts, "  "))))
}
```

**Step 4: Build and test**
```bash
go build ./... && go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Step 5: Commit**
```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go
git commit -m "feat: expose player resistances/weaknesses on character sheet"
```

---

### Task 6: Add resistances/weaknesses to armor content + deploy

**Files:**
- Modify: several files in `content/armor/`

**Step 1:** List all armor files and pick 4-6 where resistance/weakness makes lore sense:
- Fire-resistant materials (e.g. a flak jacket or nomex suit): `resistances: {fire: 3}`
- Heavy plate/ballistic armor: `resistances: {piercing: 2}`
- Rubber/insulated gear: `resistances: {electricity: 3}`
- Thin/light gear that's vulnerable to slashing: `weaknesses: {slashing: 1}`

Read each YAML before editing. Add the block after existing fields.

**Step 2: Verify YAML loads cleanly**
```bash
go test ./internal/game/inventory/... -v 2>&1 | tail -10
go test ./... 2>&1 | grep -E "FAIL|ok"
```

**Step 3: Full deploy**
```bash
make k8s-redeploy 2>&1 | tail -8
```

**Step 4: Mark feature complete in FEATURES.md**

Update:
```
- [x] Damage types
  - [x] Resistance (NPCs)
  - [x] Weakness (NPCs)
  - [x] Player resistances/weaknesses from armor; shown on character sheet; applied in combat
```

**Step 5: Commit and push**
```bash
git add content/armor/ docs/requirements/FEATURES.md
git commit -m "content+docs: add armor resistances/weaknesses; mark player resistance feature complete"
git push
```
