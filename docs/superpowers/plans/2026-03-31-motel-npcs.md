# Motel NPCs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `motel_keeper` NPC type and populate one lore-appropriate motel keeper per zone so players can use the `rest` command in every zone's hub safe room.

**Architecture:** New `MotelConfig` struct and `motel_keeper` NPC type in the template/instance layer; `RestCost` populated from YAML during spawn. Content-only changes for 17 zones: one NPC entry per non-combat YAML file, one spawn entry per hub room in each zone YAML. No changes to the rest handler.

**Tech Stack:** Go 1.26, GopherLua (not touched), YAML content files, `mise` toolchain.

---

### Task 1: Add `MotelConfig`, `motel_keeper` type, and `RestCost` population

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/instance.go`

- [ ] **Step 1: Add `MotelConfig` struct and `Motel` field to `Template`**

In `internal/game/npc/template.go`, add the struct immediately after the `ChipDocConfig` struct definition (search for `type ChipDocConfig struct`), and add the field to `Template` alongside the other type-specific config fields (the block ending with `ChipDoc *ChipDocConfig`):

```go
// MotelConfig holds configuration for a motel_keeper NPC (REQ-MOT-2).
type MotelConfig struct {
    RestCost int `yaml:"rest_cost"`
}
```

In the `Template` struct, add after `ChipDoc *ChipDocConfig \`yaml:"chip_doc,omitempty"\``:

```go
Motel *MotelConfig `yaml:"motel,omitempty"`
```

- [ ] **Step 2: Register `motel_keeper` in `validTypes` and add validation case**

In `Validate()`, find the `validTypes` map and add `"motel_keeper": true`.

Then add a new case to the `switch t.NPCType` block after the `"chip_doc"` case:

```go
case "motel_keeper":
    if t.Motel == nil {
        return fmt.Errorf("npc template %q: npc_type 'motel_keeper' requires a motel: config block", t.ID)
    }
    if t.Motel.RestCost <= 0 {
        return fmt.Errorf("npc template %q: npc_type 'motel_keeper' requires rest_cost > 0", t.ID)
    }
```

- [ ] **Step 3: Populate `RestCost` in `NewInstanceWithResolver`**

In `internal/game/npc/instance.go`, in `NewInstanceWithResolver`, add after the `inst := &Instance{...}` literal closes (search for `FactionID: tmpl.FactionID`):

```go
if tmpl.Motel != nil {
    inst.RestCost = tmpl.Motel.RestCost
}
```

- [ ] **Step 4: Run fast tests to verify no regressions**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/template.go internal/game/npc/instance.go
git commit -m "feat: add motel_keeper NPC type with MotelConfig and RestCost population"
```

---

### Task 2: Unit tests for `motel_keeper` type and `RestCost`

**Files:**
- Modify: `internal/game/npc/template_test.go`
- Modify: `internal/game/npc/instance_test.go`

- [ ] **Step 1: Write failing tests for template validation**

Add to `internal/game/npc/template_test.go`:

```go
func TestTemplate_MotelKeeperRequiresMotelConfig(t *testing.T) {
    data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
`)
    _, err := npc.LoadTemplateFromBytes(data)
    assert.Error(t, err, "motel_keeper without motel config must error")
    assert.Contains(t, err.Error(), "requires a motel: config block")
}

func TestTemplate_MotelKeeperRequiresPositiveRestCost(t *testing.T) {
    data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
motel:
  rest_cost: 0
`)
    _, err := npc.LoadTemplateFromBytes(data)
    assert.Error(t, err, "motel_keeper with rest_cost 0 must error")
    assert.Contains(t, err.Error(), "rest_cost > 0")
}

func TestTemplate_MotelKeeperWithValidConfigLoads(t *testing.T) {
    data := []byte(`id: test_motel
name: Test Motel Keeper
level: 2
max_hp: 20
ac: 10
npc_type: motel_keeper
motel:
  rest_cost: 50
`)
    tmpl, err := npc.LoadTemplateFromBytes(data)
    require.NoError(t, err)
    assert.Equal(t, "motel_keeper", tmpl.NPCType)
    require.NotNil(t, tmpl.Motel)
    assert.Equal(t, 50, tmpl.Motel.RestCost)
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test -run "TestTemplate_MotelKeeper" ./internal/game/npc/... -v
```

Expected: FAIL — `motel_keeper` is unknown type (Task 1 must be done first). If Task 1 is complete, these should pass immediately — proceed to Step 4.

- [ ] **Step 3: Write failing test for `RestCost` population on `Instance`**

Add to `internal/game/npc/instance_test.go`:

```go
func TestNewInstanceWithResolver_RestCostFromMotelConfig(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "test_motel",
        Name:    "Test Motel Keeper",
        Level:   2,
        MaxHP:   20,
        AC:      10,
        NPCType: "motel_keeper",
        Motel:   &npc.MotelConfig{RestCost: 75},
    }
    inst := npc.NewInstance("inst-motel", tmpl, "room-1")
    assert.Equal(t, 75, inst.RestCost, "RestCost must be populated from MotelConfig")
}

func TestNewInstanceWithResolver_RestCostZeroWhenNoMotelConfig(t *testing.T) {
    tmpl := &npc.Template{
        ID:      "test_combat",
        Name:    "Bandit",
        Level:   1,
        MaxHP:   20,
        AC:      12,
        NPCType: "combat",
    }
    inst := npc.NewInstance("inst-combat", tmpl, "room-1")
    assert.Equal(t, 0, inst.RestCost, "RestCost must be zero when Motel is nil")
}
```

- [ ] **Step 4: Run all new tests**

```bash
go test -run "TestTemplate_MotelKeeper|TestNewInstanceWithResolver_RestCost" ./internal/game/npc/... -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Run full fast suite**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/game/npc/template_test.go internal/game/npc/instance_test.go
git commit -m "test: motel_keeper template validation and RestCost instance population"
```

---

### Task 3: Motel keeper NPC entries — zones A–H (8 zones)

**Files:**
- Modify: `content/npcs/non_combat/felony_flats.yaml`
- Modify: `content/npcs/non_combat/rustbucket_ridge.yaml`
- Modify: `content/npcs/non_combat/se_industrial.yaml`
- Modify: `content/npcs/non_combat/ross_island.yaml`
- Modify: `content/npcs/non_combat/sauvie_island.yaml`
- Modify: `content/npcs/non_combat/the_couve.yaml`
- Modify: `content/npcs/non_combat/vantucky.yaml`
- Modify: `content/npcs/non_combat/battleground.yaml`

- [ ] **Step 1: Append motel keeper to `felony_flats.yaml`**

Append to end of `content/npcs/non_combat/felony_flats.yaml`:

```yaml

- id: felony_flats_motel_keeper
  name: "Crash Pad Runner"
  npc_type: motel_keeper
  type: human
  description: "Rents floor space in the safe house, three to a room, no questions asked. Bring your own bedroll."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 25
```

- [ ] **Step 2: Append motel keeper to `rustbucket_ridge.yaml`**

Append to end of `content/npcs/non_combat/rustbucket_ridge.yaml`:

```yaml

- id: rustbucket_ridge_motel_keeper
  name: "Scrap Inn Clerk"
  npc_type: motel_keeper
  type: human
  description: "Runs a row of converted shipping containers fitted with cots. Cold in winter, hot in summer, safe year-round."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 35
```

- [ ] **Step 3: Append motel keeper to `se_industrial.yaml`**

Append to end of `content/npcs/non_combat/se_industrial.yaml`:

```yaml

- id: se_industrial_motel_keeper
  name: "Shift Boss"
  npc_type: motel_keeper
  type: human
  description: "Manages the break room cots between shifts. The fluorescent lights never fully turn off. You get used to it."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 35
```

- [ ] **Step 4: Append motel keeper to `ross_island.yaml`**

Append to end of `content/npcs/non_combat/ross_island.yaml`:

```yaml

- id: ross_island_motel_keeper
  name: "Dock Keeper"
  npc_type: motel_keeper
  type: human
  description: "Rents a cot in the corner of the dock shack. The river noise keeps some people up. Others find it peaceful."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 40
```

- [ ] **Step 5: Append motel keeper to `sauvie_island.yaml`**

Append to end of `content/npcs/non_combat/sauvie_island.yaml`:

```yaml

- id: sauvie_island_motel_keeper
  name: "Farm Host"
  npc_type: motel_keeper
  type: human
  description: "Rents bunk space in the old farmhouse. Breakfast is not included, but the smell of coffee at dawn is free."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 40
```

- [ ] **Step 6: Append motel keeper to `the_couve.yaml`**

Append to end of `content/npcs/non_combat/the_couve.yaml`:

```yaml

- id: the_couve_motel_keeper
  name: "Crossing Keeper"
  npc_type: motel_keeper
  type: human
  description: "Runs the only beds at the river crossing. Charges by the night, cash only, no exceptions."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 45
```

- [ ] **Step 7: Append motel keeper to `vantucky.yaml`**

Append to end of `content/npcs/non_combat/vantucky.yaml`:

```yaml

- id: vantucky_motel_keeper
  name: "Compound Steward"
  npc_type: motel_keeper
  type: human
  description: "Manages sleeping quarters inside the compound walls. Strict check-in, strict lights-out. Worth it for the security."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 45
```

- [ ] **Step 8: Append motel keeper to `battleground.yaml`**

Append to end of `content/npcs/non_combat/battleground.yaml`:

```yaml

- id: battleground_motel_keeper
  name: "Commons Steward"
  npc_type: motel_keeper
  type: human
  description: "Manages sleeping quarters in the neutral commons. Militia affiliation checked at the door; inside, everyone's just tired."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 50
```

- [ ] **Step 9: Run fast tests**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add content/npcs/non_combat/felony_flats.yaml content/npcs/non_combat/rustbucket_ridge.yaml \
        content/npcs/non_combat/se_industrial.yaml content/npcs/non_combat/ross_island.yaml \
        content/npcs/non_combat/sauvie_island.yaml content/npcs/non_combat/the_couve.yaml \
        content/npcs/non_combat/vantucky.yaml content/npcs/non_combat/battleground.yaml
git commit -m "content: add motel keeper NPCs to zones felony_flats through battleground"
```

---

### Task 4: Motel keeper NPC entries — zones I–Q (9 zones, including new Wooklyn file)

**Files:**
- Modify: `content/npcs/non_combat/troutdale.yaml`
- Modify: `content/npcs/non_combat/ne_portland.yaml`
- Modify: `content/npcs/non_combat/aloha.yaml`
- Modify: `content/npcs/non_combat/beaverton.yaml`
- Modify: `content/npcs/non_combat/hillsboro.yaml`
- Modify: `content/npcs/non_combat/downtown.yaml`
- Modify: `content/npcs/non_combat/lake_oswego.yaml`
- Modify: `content/npcs/non_combat/pdx_international.yaml`
- Create: `content/npcs/non_combat/wooklyn.yaml`

- [ ] **Step 1: Append motel keeper to `troutdale.yaml`**

Append to end of `content/npcs/non_combat/troutdale.yaml`:

```yaml

- id: troutdale_motel_keeper
  name: "Motel Clerk"
  npc_type: motel_keeper
  type: human
  description: "Runs what's left of the highway motel — half the rooms still intact, the sign still lights up at night. Old habit."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 50
```

- [ ] **Step 2: Append motel keeper to `ne_portland.yaml`**

Append to end of `content/npcs/non_combat/ne_portland.yaml`:

```yaml

- id: ne_portland_motel_keeper
  name: "Bunker Host"
  npc_type: motel_keeper
  type: human
  description: "Rents bunks in the reinforced neutral bunker. Not fancy, but you'll wake up in the morning."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 55
```

- [ ] **Step 3: Append motel keeper to `aloha.yaml`**

Append to end of `content/npcs/non_combat/aloha.yaml`:

```yaml

- id: aloha_motel_keeper
  name: "Bazaar Host"
  npc_type: motel_keeper
  type: human
  description: "Rents back rooms above the market stalls. The beds are clean, the noise never fully stops, but the price is fair."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 60
```

- [ ] **Step 4: Append motel keeper to `beaverton.yaml`**

Append to end of `content/npcs/non_combat/beaverton.yaml`:

```yaml

- id: beaverton_motel_keeper
  name: "Extended Stay Clerk"
  npc_type: motel_keeper
  type: human
  description: "Works the front desk of a former corporate extended-stay hotel, still running on backup power. Hot water, sometimes."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 75
```

- [ ] **Step 5: Append motel keeper to `hillsboro.yaml`**

Append to end of `content/npcs/non_combat/hillsboro.yaml`:

```yaml

- id: hillsboro_motel_keeper
  name: "Waystation Keeper"
  npc_type: motel_keeper
  type: human
  description: "Manages the waystation's bunk room. Former tech corridor worker, now runs the most organized lodging west of Portland."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 75
```

- [ ] **Step 6: Append motel keeper to `downtown.yaml`**

Append to end of `content/npcs/non_combat/downtown.yaml`:

```yaml

- id: downtown_motel_keeper
  name: "Night Manager"
  npc_type: motel_keeper
  type: human
  description: "Runs the last functioning hotel in downtown Portland. The rates are steep, the sheets are clean, the door locks work."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 100
```

- [ ] **Step 7: Append motel keeper to `lake_oswego.yaml`**

Append to end of `content/npcs/non_combat/lake_oswego.yaml`:

```yaml

- id: lake_oswego_motel_keeper
  name: "Estate Concierge"
  npc_type: motel_keeper
  type: human
  description: "Manages guest quarters in a former lakeside estate. Speaks quietly, prices loudly. The linens are actual linen."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 120
```

- [ ] **Step 8: Append motel keeper to `pdx_international.yaml`**

Append to end of `content/npcs/non_combat/pdx_international.yaml`:

```yaml

- id: pdx_international_motel_keeper
  name: "Terminal Agent"
  npc_type: motel_keeper
  type: human
  description: "Manages the last operational terminal hotel. Prices reflect demand, not quality. Knows it."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 150
```

- [ ] **Step 9: Create `content/npcs/non_combat/wooklyn.yaml`**

Create `content/npcs/non_combat/wooklyn.yaml` with contents:

```yaml
- id: wooklyn_motel_keeper
  name: "Camp Steward"
  npc_type: motel_keeper
  type: human
  description: "Keeps the caravan bunk tent. Communal sleeping, communal rules. No violence, no exceptions, no refunds."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  motel:
    rest_cost: 45
```

- [ ] **Step 10: Run fast tests**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
git add content/npcs/non_combat/troutdale.yaml content/npcs/non_combat/ne_portland.yaml \
        content/npcs/non_combat/aloha.yaml content/npcs/non_combat/beaverton.yaml \
        content/npcs/non_combat/hillsboro.yaml content/npcs/non_combat/downtown.yaml \
        content/npcs/non_combat/lake_oswego.yaml content/npcs/non_combat/pdx_international.yaml \
        content/npcs/non_combat/wooklyn.yaml
git commit -m "content: add motel keeper NPCs to zones troutdale through pdx_international and wooklyn"
```

---

### Task 5: Zone spawn entries — zones A–H (8 zones)

Add the spawn entry for each motel keeper to its hub safe room. Search for the hub room ID in each zone YAML, then append the spawn entry to that room's `spawns:` list. If the room has no `spawns:` key yet, add one.

**Files:**
- Modify: `content/zones/felony_flats.yaml` — room `flats_safe_house`
- Modify: `content/zones/rustbucket_ridge.yaml` — room `grinders_row`
- Modify: `content/zones/se_industrial.yaml` — room `sei_break_room`
- Modify: `content/zones/ross_island.yaml` — room `ross_dock_shack`
- Modify: `content/zones/sauvie_island.yaml` — room `sauvie_farm_stand`
- Modify: `content/zones/the_couve.yaml` — room `couve_the_crossing`
- Modify: `content/zones/vantucky.yaml` — room `vantucky_the_compound`
- Modify: `content/zones/battleground.yaml` — room `battle_neutral_commons`

- [ ] **Step 1: Add spawn to `flats_safe_house` in `felony_flats.yaml`**

Find `id: flats_safe_house` in `content/zones/felony_flats.yaml`. Locate the `spawns:` list for that room and append:

```yaml
    - template: felony_flats_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 2: Add spawn to `grinders_row` in `rustbucket_ridge.yaml`**

Find `id: grinders_row` in `content/zones/rustbucket_ridge.yaml`. Append to its `spawns:`:

```yaml
    - template: rustbucket_ridge_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 3: Add spawn to `sei_break_room` in `se_industrial.yaml`**

Find `id: sei_break_room` in `content/zones/se_industrial.yaml`. Append to its `spawns:`:

```yaml
    - template: se_industrial_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 4: Add spawn to `ross_dock_shack` in `ross_island.yaml`**

Find `id: ross_dock_shack` in `content/zones/ross_island.yaml`. Append to its `spawns:`:

```yaml
    - template: ross_island_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 5: Add spawn to `sauvie_farm_stand` in `sauvie_island.yaml`**

Find `id: sauvie_farm_stand` in `content/zones/sauvie_island.yaml`. Append to its `spawns:`:

```yaml
    - template: sauvie_island_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 6: Add spawn to `couve_the_crossing` in `the_couve.yaml`**

Find `id: couve_the_crossing` in `content/zones/the_couve.yaml`. Append to its `spawns:`:

```yaml
    - template: the_couve_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 7: Add spawn to `vantucky_the_compound` in `vantucky.yaml`**

Find `id: vantucky_the_compound` in `content/zones/vantucky.yaml`. Append to its `spawns:`:

```yaml
    - template: vantucky_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 8: Add spawn to `battle_neutral_commons` in `battleground.yaml`**

Find `id: battle_neutral_commons` in `content/zones/battleground.yaml`. Append to its `spawns:`:

```yaml
    - template: battleground_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 9: Run fast tests**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add content/zones/felony_flats.yaml content/zones/rustbucket_ridge.yaml \
        content/zones/se_industrial.yaml content/zones/ross_island.yaml \
        content/zones/sauvie_island.yaml content/zones/the_couve.yaml \
        content/zones/vantucky.yaml content/zones/battleground.yaml
git commit -m "content: spawn motel keeper NPCs in hub safe rooms for zones A-H"
```

---

### Task 6: Zone spawn entries — zones I–Q (9 zones)

**Files:**
- Modify: `content/zones/troutdale.yaml` — room `trout_neutral_motel`
- Modify: `content/zones/ne_portland.yaml` — room `ne_neutral_bunker`
- Modify: `content/zones/aloha.yaml` — room `aloha_the_bazaar`
- Modify: `content/zones/beaverton.yaml` — room `beav_neutral_market`
- Modify: `content/zones/hillsboro.yaml` — room `hills_waystation`
- Modify: `content/zones/downtown.yaml` — room `market_district`
- Modify: `content/zones/lake_oswego.yaml` — room `lo_the_commons`
- Modify: `content/zones/pdx_international.yaml` — room `pdx_neutral_terminal`
- Modify: `content/zones/wooklyn.yaml` — room `tofteville_market`

- [ ] **Step 1: Add spawn to `trout_neutral_motel` in `troutdale.yaml`**

Find `id: trout_neutral_motel` in `content/zones/troutdale.yaml`. Append to its `spawns:`:

```yaml
    - template: troutdale_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 2: Add spawn to `ne_neutral_bunker` in `ne_portland.yaml`**

Find `id: ne_neutral_bunker` in `content/zones/ne_portland.yaml`. Append to its `spawns:`:

```yaml
    - template: ne_portland_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 3: Add spawn to `aloha_the_bazaar` in `aloha.yaml`**

Find `id: aloha_the_bazaar` in `content/zones/aloha.yaml`. Append to its `spawns:`:

```yaml
    - template: aloha_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 4: Add spawn to `beav_neutral_market` in `beaverton.yaml`**

Find `id: beav_neutral_market` in `content/zones/beaverton.yaml`. Append to its `spawns:`:

```yaml
    - template: beaverton_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 5: Add spawn to `hills_waystation` in `hillsboro.yaml`**

Find `id: hills_waystation` in `content/zones/hillsboro.yaml`. Append to its `spawns:`:

```yaml
    - template: hillsboro_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 6: Add spawn to `market_district` in `downtown.yaml`**

Find `id: market_district` in `content/zones/downtown.yaml`. Append to its `spawns:`:

```yaml
    - template: downtown_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 7: Add spawn to `lo_the_commons` in `lake_oswego.yaml`**

Find `id: lo_the_commons` in `content/zones/lake_oswego.yaml`. Append to its `spawns:`:

```yaml
    - template: lake_oswego_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 8: Add spawn to `pdx_neutral_terminal` in `pdx_international.yaml`**

Find `id: pdx_neutral_terminal` in `content/zones/pdx_international.yaml`. Append to its `spawns:`:

```yaml
    - template: pdx_international_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 9: Add spawn to `tofteville_market` in `wooklyn.yaml`**

Find `id: tofteville_market` in `content/zones/wooklyn.yaml`. Append to its `spawns:`:

```yaml
    - template: wooklyn_motel_keeper
      count: 1
      respawn_after: 0s
```

- [ ] **Step 10: Run full fast test suite**

```bash
make test-fast
```

Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
git add content/zones/troutdale.yaml content/zones/ne_portland.yaml \
        content/zones/aloha.yaml content/zones/beaverton.yaml \
        content/zones/hillsboro.yaml content/zones/downtown.yaml \
        content/zones/lake_oswego.yaml content/zones/pdx_international.yaml \
        content/zones/wooklyn.yaml
git commit -m "content: spawn motel keeper NPCs in hub safe rooms for zones I-Q and wooklyn"
```
