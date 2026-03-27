# Non-Combat NPCs — All Zones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Place one lore-appropriate instance of each non-combat NPC type (merchant, healer, job_trainer, banker) in a safe room in every zone via YAML content only.

**Architecture:** Pure YAML content feature — no Go code changes. Each zone gets new NPC YAML files referencing safe rooms already defined in that zone's room YAML.

**Tech Stack:** YAML content files, existing NPC template loader, existing zone loader

---

## Prerequisites

- REQ-NCNAZ-0 blocks this feature: the `banker` npc_role MUST be implemented in `non-combat-npcs` before this feature can be implemented. Verify `content/npcs/vera_coldcoin.yaml` exists and the loader accepts `npc_type: banker` before proceeding.

---

## Phase 0: TDD — Write Failing Tests First

All tests MUST be written and confirmed failing before any YAML content is created.

### Task 0.1 — Create `internal/game/world/noncombat_coverage_test.go`

- [ ] Create `/home/cjohannsen/src/mud/internal/game/world/noncombat_coverage_test.go` in package `world`.
- [ ] The file MUST use module path `github.com/cory-johannsen/mud`.
- [ ] The file MUST import `pgregory.net/rapid` for property-based tests.
- [ ] Implement the following tests:

**Test `TestAllZonesHaveAtLeastOneSafeRoom`**

Loads all 16 zone YAML files from `content/zones/*.yaml` (relative to the repo root resolved via `testhelper` or `os.Getwd()`). For each zone, verifies at least one room has `danger_level: safe`. Uses `LoadZoneFromBytes`.

```go
package world

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)

func TestAllZonesHaveAtLeastOneSafeRoom(t *testing.T) {
    zonesDir := filepath.Join(repoRoot(t), "content", "zones")
    entries, err := os.ReadDir(zonesDir)
    require.NoError(t, err)
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
            continue
        }
        data, err := os.ReadFile(filepath.Join(zonesDir, entry.Name()))
        require.NoError(t, err)
        zone, err := LoadZoneFromBytes(data)
        require.NoError(t, err, "zone file: %s", entry.Name())
        hasSafe := false
        for _, room := range zone.Rooms {
            if room.DangerLevel == "safe" {
                hasSafe = true
                break
            }
        }
        require.True(t, hasSafe, "zone %q must have at least one safe room (REQ-NCNAZ-1)", zone.ID)
    }
}
```

**Note:** `repoRoot(t)` MUST be implemented as a helper that walks up from `os.Getwd()` until it finds `go.mod`. Add it to a new `testhelpers_test.go` file in the same package, or inline it.

**Test `TestAllZonesHaveRequiredNPCTypes`**

Loads all `content/npcs/non_combat/<zone_id>.yaml` files. For each zone, verifies the NPC template list contains at minimum one template for each of: `merchant`, `healer`, `job_trainer`, `banker`. Uses the `npc` package's `LoadTemplates` or raw YAML decode.

```go
func TestAllZonesHaveRequiredNPCTypes(t *testing.T) {
    zones := []string{
        "aloha", "beaverton", "battleground", "downtown", "felony_flats",
        "hillsboro", "lake_oswego", "ne_portland", "pdx_international",
        "ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
        "the_couve", "troutdale", "vantucky",
    }
    required := []string{"merchant", "healer", "job_trainer", "banker"}
    root := repoRoot(t)
    for _, zoneID := range zones {
        path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
        data, err := os.ReadFile(path)
        require.NoError(t, err, "non_combat NPC file missing for zone %q", zoneID)
        var templates []struct {
            ID      string `yaml:"id"`
            NPCType string `yaml:"npc_type"`
        }
        require.NoError(t, yaml.Unmarshal(data, &templates))
        typeSet := make(map[string]bool)
        for _, tmpl := range templates {
            typeSet[tmpl.NPCType] = true
        }
        for _, req := range required {
            require.True(t, typeSet[req],
                "zone %q missing required npc_type %q (REQ-NCNAZ-4)", zoneID, req)
        }
    }
}
```

**Test `TestNonCombatNPCTemplateIDs`**

Verifies every template ID in each `content/npcs/non_combat/<zone_id>.yaml` follows the `<zone_id>_<npc_role>` pattern (REQ-NCNAZ-7).

```go
func TestNonCombatNPCTemplateIDs(t *testing.T) {
    zones := []string{
        "aloha", "beaverton", "battleground", "downtown", "felony_flats",
        "hillsboro", "lake_oswego", "ne_portland", "pdx_international",
        "ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
        "the_couve", "troutdale", "vantucky",
    }
    root := repoRoot(t)
    for _, zoneID := range zones {
        path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
        data, err := os.ReadFile(path)
        require.NoError(t, err)
        var templates []struct {
            ID      string `yaml:"id"`
            NPCType string `yaml:"npc_type"`
        }
        require.NoError(t, yaml.Unmarshal(data, &templates))
        for _, tmpl := range templates {
            expected := zoneID + "_" + tmpl.NPCType
            require.Equal(t, expected, tmpl.ID,
                "zone %q template ID must be %q (REQ-NCNAZ-7)", zoneID, expected)
        }
    }
}
```

**Test `TestNonCombatNPCsNoQuestGiverOrCrafter`**

Verifies no `quest_giver` or `crafter` templates exist in any `content/npcs/non_combat/` file (REQ-NCNAZ-6).

```go
func TestNonCombatNPCsNoQuestGiverOrCrafter(t *testing.T) {
    root := repoRoot(t)
    dir := filepath.Join(root, "content", "npcs", "non_combat")
    entries, err := os.ReadDir(dir)
    require.NoError(t, err)
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
            continue
        }
        data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
        require.NoError(t, err)
        var templates []struct {
            ID      string `yaml:"id"`
            NPCType string `yaml:"npc_type"`
        }
        require.NoError(t, yaml.Unmarshal(data, &templates))
        for _, tmpl := range templates {
            require.NotEqual(t, "quest_giver", tmpl.NPCType,
                "quest_giver template %q MUST NOT be placed (REQ-NCNAZ-6)", tmpl.ID)
            require.NotEqual(t, "crafter", tmpl.NPCType,
                "crafter template %q MUST NOT be placed (REQ-NCNAZ-6)", tmpl.ID)
        }
    }
}
```

**Property test `TestProperty_AllNonCombatTemplatesHaveNeutralDisposition`**

Uses `pgregory.net/rapid` to sample templates from a loaded pool and verify `disposition: neutral` and that `respawn_delay` is empty (non-combat NPCs are permanent — they MUST NOT have a `respawn_delay` set; REQ-NCNAZ-8 refers to the zone spawn entry `respawn_after: 0s`, not the template).

```go
func TestProperty_AllNonCombatTemplatesHaveNeutralDisposition(t *testing.T) {
    root := repoRoot(t)
    dir := filepath.Join(root, "content", "npcs", "non_combat")
    entries, err := os.ReadDir(dir)
    require.NoError(t, err)
    type minTemplate struct {
        ID           string `yaml:"id"`
        Disposition  string `yaml:"disposition"`
        RespawnDelay string `yaml:"respawn_delay"`
    }
    var allTemplates []minTemplate
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
            continue
        }
        data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
        require.NoError(t, err)
        var templates []minTemplate
        require.NoError(t, yaml.Unmarshal(data, &templates))
        allTemplates = append(allTemplates, templates...)
    }
    require.NotEmpty(t, allTemplates)
    rapid.Check(t, func(rt *rapid.T) {
        idx := rapid.IntRange(0, len(allTemplates)-1).Draw(rt, "idx")
        tmpl := allTemplates[idx]
        if tmpl.Disposition != "neutral" {
            rt.Fatalf("template %q: disposition must be 'neutral', got %q (REQ-NCNAZ-9)", tmpl.ID, tmpl.Disposition)
        }
        if tmpl.RespawnDelay != "" {
            rt.Fatalf("template %q: respawn_delay must be empty for permanent non-combat NPCs, got %q (REQ-NCNAZ-8)", tmpl.ID, tmpl.RespawnDelay)
        }
    })
}
```

**Test `TestOptionalNPCTypesOnlyInAuthorizedZones`**

Verifies `guard`, `hireling`, and `fixer` NPC types appear only in the zones listed in spec Section 2.2.

```go
func TestOptionalNPCTypesOnlyInAuthorizedZones(t *testing.T) {
    guardZones   := map[string]bool{"aloha": true, "battleground": true, "beaverton": true, "downtown": true, "hillsboro": true, "pdx_international": true, "se_industrial": true, "the_couve": true}
    hirelingZones := map[string]bool{"beaverton": true, "hillsboro": true, "lake_oswego": true, "ross_island": true, "rustbucket_ridge": true, "se_industrial": true, "vantucky": true}
    fixerZones   := map[string]bool{"aloha": true, "downtown": true, "felony_flats": true, "the_couve": true}

    zones := []string{
        "aloha", "beaverton", "battleground", "downtown", "felony_flats",
        "hillsboro", "lake_oswego", "ne_portland", "pdx_international",
        "ross_island", "rustbucket_ridge", "sauvie_island", "se_industrial",
        "the_couve", "troutdale", "vantucky",
    }
    root := repoRoot(t)
    for _, zoneID := range zones {
        path := filepath.Join(root, "content", "npcs", "non_combat", zoneID+".yaml")
        data, err := os.ReadFile(path)
        require.NoError(t, err)
        var templates []struct {
            ID      string `yaml:"id"`
            NPCType string `yaml:"npc_type"`
        }
        require.NoError(t, yaml.Unmarshal(data, &templates))
        for _, tmpl := range templates {
            switch tmpl.NPCType {
            case "guard":
                require.True(t, guardZones[zoneID], "guard %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
            case "hireling":
                require.True(t, hirelingZones[zoneID], "hireling %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
            case "fixer":
                require.True(t, fixerZones[zoneID], "fixer %q in unauthorized zone %q (REQ-NCNAZ-5)", tmpl.ID, zoneID)
            }
        }
    }
}
```

**Test `TestNewSafeRoomsConnectedBidirectionally`**

For each zone that required a new safe room (spec Section 1.2), loads the zone YAML and verifies:
- The new safe room exists with correct ID and `danger_level: safe`.
- The new safe room has an exit in the reverse direction to the anchor room.
- The anchor room has an exit in the specified direction to the new safe room.

```go
func TestNewSafeRoomsConnectedBidirectionally(t *testing.T) {
    type safeRoomSpec struct {
        zoneFile    string
        newRoomID   string
        anchorID    string
        anchorToNew string // direction from anchor → new
        newToAnchor string // reverse direction
    }
    specs := []safeRoomSpec{
        {"beaverton", "beav_free_market", "beav_canyon_road_east", "north", "south"},
        {"downtown", "downtown_underground", "morrison_bridge", "north", "south"},
        {"hillsboro", "hills_the_keep", "hills_tv_highway_east", "south", "north"},
        {"ne_portland", "ne_corner_store", "ne_alberta_street", "north", "south"},
        {"pdx_international", "pdx_terminal_b", "pdx_airport_way_west", "south", "north"},
        {"ross_island", "ross_dock_shack", "ross_bridge_east", "east", "west"},
        {"rustbucket_ridge", "rust_scrap_office", "last_stand_lodge", "east", "west"},
        {"sauvie_island", "sauvie_farm_stand", "sauvie_bridge_south", "south", "north"},
        {"se_industrial", "sei_break_room", "sei_holgate_blvd", "east", "west"},
        {"the_couve", "couve_the_crossing", "couve_interstate_bridge_south", "west", "east"},
        {"troutdale", "trout_truck_stop", "trout_i84_west", "north", "south"},
        {"vantucky", "vantucky_the_compound", "vantucky_fourth_plain_west", "north", "south"},
    }
    root := repoRoot(t)
    for _, s := range specs {
        data, err := os.ReadFile(filepath.Join(root, "content", "zones", s.zoneFile+".yaml"))
        require.NoError(t, err)
        zone, err := LoadZoneFromBytes(data)
        require.NoError(t, err)

        roomMap := make(map[string]*Room)
        for i := range zone.Rooms {
            roomMap[zone.Rooms[i].ID] = &zone.Rooms[i]
        }

        newRoom, ok := roomMap[s.newRoomID]
        require.True(t, ok, "zone %q missing new safe room %q (REQ-NCNAZ-1)", s.zoneFile, s.newRoomID)
        require.Equal(t, "safe", newRoom.DangerLevel, "new room %q must have danger_level: safe (REQ-NCNAZ-1)", s.newRoomID)

        // new room must have reverse exit to anchor
        hasReverseExit := false
        for _, exit := range newRoom.Exits {
            if exit.Direction == s.newToAnchor && exit.Target == s.anchorID {
                hasReverseExit = true
                break
            }
        }
        require.True(t, hasReverseExit, "room %q must have %s exit to %q (REQ-NCNAZ-13)", s.newRoomID, s.newToAnchor, s.anchorID)

        anchor, ok := roomMap[s.anchorID]
        require.True(t, ok, "zone %q missing anchor room %q", s.zoneFile, s.anchorID)
        hasForwardExit := false
        for _, exit := range anchor.Exits {
            if exit.Direction == s.anchorToNew && exit.Target == s.newRoomID {
                hasForwardExit = true
                break
            }
        }
        require.True(t, hasForwardExit, "anchor %q must have %s exit to %q (REQ-NCNAZ-13)", s.anchorID, s.anchorToNew, s.newRoomID)
    }
}
```

**Test `TestNewSafeRoomDescriptions`**

Verifies each new safe room's description matches the spec exactly (REQ-NCNAZ-3).

```go
func TestNewSafeRoomDescriptions(t *testing.T) {
    type roomDesc struct {
        zoneFile string
        roomID   string
        desc     string
    }
    cases := []roomDesc{
        {"beaverton", "beav_free_market", "An open-air block of vendor stalls under corrugated aluminum roofing. The smell of hot food and machine oil. People come here to trade, not fight."},
        {"downtown", "downtown_underground", "A repurposed parking garage two levels below street level. Strip lighting, folding tables, and the low hum of people who need things and people who have them."},
        {"hillsboro", "hills_the_keep", "A fortified community hall at the edge of the Hillsboro enclave. Stone walls and firelight. A place of order, or something close to it."},
        {"ne_portland", "ne_corner_store", "A converted convenience store with the shelving pushed to the walls. Locals come here to restock, get patched up, and hear what's going on in the neighborhood."},
        {"pdx_international", "pdx_terminal_b", "A section of the airport terminal cordoned off from the main concourse. Chairs bolted to the floor, vending machines that still work, and people who've learned to wait."},
        {"ross_island", "ross_dock_shack", "A weathered shack at the island's main landing. Nets hang on the walls, a woodstove burns in the corner, and someone is always willing to do business."},
        {"rustbucket_ridge", "rust_scrap_office", "A repurposed foreman's office at the edge of the ridge. Metal desk, fluorescent light, and a corkboard full of job postings nobody's taken down."},
        {"sauvie_island", "sauvie_farm_stand", "A roadside stand that evolved into a community hub. Folding tables with produce, herbs, and handmade goods. Calm enough that people leave their weapons at the door."},
        {"se_industrial", "sei_break_room", "A cinder-block room with folding chairs and a microwave that runs off a generator. Shift workers and traders share the same coffee and the same fatigue."},
        {"the_couve", "couve_the_crossing", "A checkpoint building at the Washington end of the bridge. The Couve faction controls it, but they're practical: trade is welcome, trouble is not."},
        {"troutdale", "trout_truck_stop", "A diesel-soaked rest stop with a diner counter, a parts wall, and a back room where deals get made. Everyone passes through Troutdale eventually."},
        {"vantucky", "vantucky_the_compound", "The Vantucky militia's main compound. Spare and functional. They'll trade, train, and bank here — loyalty is assumed, not enforced."},
    }
    root := repoRoot(t)
    for _, c := range cases {
        data, err := os.ReadFile(filepath.Join(root, "content", "zones", c.zoneFile+".yaml"))
        require.NoError(t, err)
        zone, err := LoadZoneFromBytes(data)
        require.NoError(t, err)
        var found *Room
        for i := range zone.Rooms {
            if zone.Rooms[i].ID == c.roomID {
                found = &zone.Rooms[i]
                break
            }
        }
        require.NotNil(t, found, "room %q not found in zone %q", c.roomID, c.zoneFile)
        require.Equal(t, c.desc, strings.TrimSpace(found.Description),
            "room %q description mismatch (REQ-NCNAZ-3)", c.roomID)
    }
}
```

**Note on Room/Exit struct access:** The `world` package's `Room` and `Exit` types must be accessible within the test (same package). Verify the exported fields `DangerLevel`, `Exits`, `Direction`, `Target`, `Description` are present in `loader.go`'s `yamlRoom`/`yamlRoomExit` structs (or their exported counterparts used by `LoadZoneFromBytes`). If the loader returns unexported structs, the test MUST use the public `Zone`, `Room` API. Adjust field access accordingly after reading `loader.go` lines 60+.

- [ ] Run tests: `cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run 'TestAllZones|TestNonCombat|TestOptional|TestNew|TestProperty_AllNonCombat' 2>&1 | tail -30`. All tests MUST fail with file-not-found or assertion errors (not compile errors). Fix any compile errors before proceeding.

---

## Phase 1: Create `content/npcs/non_combat/` Directory

- [ ] Create directory `/home/cjohannsen/src/mud/content/npcs/non_combat/`.

---

## Phase 2: NPC Template YAML Files

One file per zone. Each file is a YAML list. All templates MUST have:
- `disposition: neutral`
- `respawn_after: 0s`
- `npc_type` matching the role
- Appropriate type-specific config block
- `level: 2`, `max_hp: 20`, `ac: 10`, `awareness: 3` (minimum viable non-combat stat block)
- `personality: neutral` (except `fixer` which MUST use `cowardly`)

### NPC Config Blocks Reference

All config blocks are derived from existing named NPCs in `content/npcs/`:

**merchant:**
```yaml
merchant:
  merchant_type: general
  sell_margin: 1.3
  buy_margin: 0.45
  budget: 1000
  inventory:
    - item_id: stim_pack
      base_price: 50
      init_stock: 5
      max_stock: 10
  replenish_rate:
    min_hours: 6
    max_hours: 12
    stock_refill: 1
    budget_refill: 200
```

**healer:**
```yaml
healer:
  price_per_hp: 4
  daily_capacity: 100
```

**job_trainer:**
```yaml
job_trainer:
  offered_jobs:
    - job_id: scavenger
      training_cost: 150
      prerequisites:
        min_level: 1
    - job_id: drifter
      training_cost: 250
      prerequisites:
        min_level: 2
```

**banker:**
```yaml
banker:
  zone_id: <zone_id>
  base_rate: 0.92
  rate_variance: 0.05
```

**guard:**
```yaml
guard:
  wanted_threshold: 2
  bribeable: false
```

**hireling:**
```yaml
hireling:
  daily_cost: 75
  combat_role: melee
  max_follow_zones: 3
```

**fixer:**
```yaml
fixer:
  npc_variance: 1.15
  max_wanted_level: 4
  base_costs:
    1: 150
    2: 350
    3: 700
    4: 1500
```

### Task 2.1 — `content/npcs/non_combat/aloha.yaml`

Zone: The Aloha Neutral Zone. Safe room: `aloha_the_bazaar`. Optional: guard, fixer.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/aloha.yaml`:

```yaml
- id: aloha_merchant
  name: "Swap Meet Sally"
  npc_type: merchant
  type: human
  description: "A fast-talking trader with a folding table covered in goods from six different factions. She remembers every price."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: aloha_healer
  name: "Doc Neutral"
  npc_type: healer
  type: human
  description: "A compact medic who patches up anyone regardless of faction. Neutrality is the brand."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: aloha_job_trainer
  name: "The Coordinator"
  npc_type: job_trainer
  type: human
  description: "Manages work placement across the neutral zone. If there's a job worth doing, The Coordinator knows about it."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: aloha_banker
  name: "Escrow Eddie"
  npc_type: banker
  type: human
  description: "A soft-spoken man who holds everyone's money with equal disinterest. The zone runs on his credit."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: aloha
    base_rate: 0.92
    rate_variance: 0.05

- id: aloha_guard
  name: "Border Watcher"
  npc_type: guard
  type: human
  description: "A sentry stationed at the bazaar perimeter. Watches for weapons and raised voices."
  level: 3
  max_hp: 32
  ac: 14
  awareness: 5
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: aloha_fixer
  name: "The Adjuster"
  npc_type: fixer
  type: human
  description: "Speaks in euphemisms. Every problem has a price; every price has a solution."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 5
  disposition: neutral
  personality: cowardly
  fixer:
    npc_variance: 1.15
    max_wanted_level: 4
    base_costs:
      1: 150
      2: 350
      3: 700
      4: 1500
```

### Task 2.2 — `content/npcs/non_combat/lake_oswego.yaml`

Zone: Lake Oswego. Safe room: `lo_the_commons`. Optional: hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/lake_oswego.yaml`:

```yaml
- id: lake_oswego_merchant
  name: "The Sommelier"
  npc_type: merchant
  type: human
  description: "Sells curated goods from behind a repurposed wine bar. The markup is polite but firm."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.4
    buy_margin: 0.4
    budget: 1200
    inventory:
      - item_id: stim_pack
        base_price: 60
        init_stock: 4
        max_stock: 8
    replenish_rate:
      min_hours: 8
      max_hours: 16
      stock_refill: 1
      budget_refill: 250

- id: lake_oswego_healer
  name: "Dr. Ashford"
  npc_type: healer
  type: human
  description: "A former general practitioner who still insists on proper technique. More expensive than most."
  level: 4
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 6
    daily_capacity: 120

- id: lake_oswego_job_trainer
  name: "The Career Counselor"
  npc_type: job_trainer
  type: human
  description: "Believes in matching skills to roles. The counseling is unsolicited but surprisingly accurate."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: lake_oswego_banker
  name: "Private Banker"
  npc_type: banker
  type: human
  description: "Discreet, professional, and entirely uninterested in where the money came from."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: lake_oswego
    base_rate: 0.90
    rate_variance: 0.04

- id: lake_oswego_hireling
  name: "The Attendant"
  npc_type: hireling
  type: human
  description: "Trim, quiet, and capable. Available for hire. References provided on request."
  level: 3
  max_hp: 28
  ac: 12
  awareness: 4
  disposition: neutral
  personality: opportunistic
  hireling:
    daily_cost: 100
    combat_role: melee
    max_follow_zones: 3
```

### Task 2.3 — `content/npcs/non_combat/battleground.yaml`

Zone: Battleground. Safe room: `battle_infirmary`. Optional: guard.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/battleground.yaml`:

```yaml
- id: battleground_merchant
  name: "Commissar Goods"
  npc_type: merchant
  type: human
  description: "Runs the collective's supply counter. Every transaction is recorded in triplicate."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.2
    buy_margin: 0.5
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 6
        max_stock: 12
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 2
      budget_refill: 200

- id: battleground_healer
  name: "Field Medic Yuri"
  npc_type: healer
  type: human
  description: "Former army medic. Patches wounds with the efficiency of someone who's done it under fire."
  level: 4
  max_hp: 24
  ac: 11
  awareness: 5
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 3
    daily_capacity: 150

- id: battleground_job_trainer
  name: "Political Officer"
  npc_type: job_trainer
  type: human
  description: "Assigns duties and manages ideological alignment. Will teach the correct skills to the correct comrades."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 120
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 200
        prerequisites:
          min_level: 2

- id: battleground_banker
  name: "The Treasurer"
  npc_type: banker
  type: human
  description: "Manages collective funds. All deposits are voluntary. Withdrawals require paperwork."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: battleground
    base_rate: 0.91
    rate_variance: 0.04

- id: battleground_guard
  name: "People's Guard"
  npc_type: guard
  type: human
  description: "Guards the infirmary perimeter. Earnest and alert. Does not accept bribes on principle."
  level: 3
  max_hp: 34
  ac: 14
  awareness: 6
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false
```

### Task 2.4 — `content/npcs/non_combat/felony_flats.yaml`

Zone: Felony Flats. Safe room: `flats_jade_district`. Optional: fixer.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/felony_flats.yaml`:

```yaml
- id: felony_flats_merchant
  name: "Mama Jade"
  npc_type: merchant
  type: human
  description: "Runs the district's main supply stall from a folding table stacked with salvage and sundries."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.5
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 10
      stock_refill: 1
      budget_refill: 180

- id: felony_flats_healer
  name: "Herbalist Chen"
  npc_type: healer
  type: human
  description: "Relies on local herbs and clean hands. Slower than a stim pack but cheaper and she doesn't ask questions."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 3
    daily_capacity: 80

- id: felony_flats_job_trainer
  name: "Uncle Bao"
  npc_type: job_trainer
  type: human
  description: "Knows everyone who's hiring in the flats and what it takes to work for them."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 100
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 200
        prerequisites:
          min_level: 2

- id: felony_flats_banker
  name: "The Moneylender"
  npc_type: banker
  type: human
  description: "Loans at fair rates, banks for a fee. No names required."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: felony_flats
    base_rate: 0.88
    rate_variance: 0.06

- id: felony_flats_fixer
  name: "The Jade Fixer"
  npc_type: fixer
  type: human
  description: "Operates from a back booth in the district. Known by name, not by face."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 5
  disposition: neutral
  personality: cowardly
  fixer:
    npc_variance: 1.2
    max_wanted_level: 4
    base_costs:
      1: 120
      2: 300
      3: 650
      4: 1400
```

### Task 2.5 — `content/npcs/non_combat/beaverton.yaml`

Zone: Beaverton. New safe room: `beav_free_market`. Optional: guard, hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/beaverton.yaml`:

```yaml
- id: beaverton_merchant
  name: "Free Trader Bo"
  npc_type: merchant
  type: human
  description: "Works a stall in the free market. Buys anything, sells anything, takes no sides."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: beaverton_healer
  name: "Medtech Remy"
  npc_type: healer
  type: human
  description: "A compact medic with a portable kit and a businesslike manner. Gets you patched and back on the road."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: beaverton_job_trainer
  name: "Skills Broker"
  npc_type: job_trainer
  type: human
  description: "Matches skilled workers with employers. Keeps a running list of what the market needs."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: beaverton_banker
  name: "The Vault"
  npc_type: banker
  type: human
  description: "Runs an actual reinforced vault beneath the market. Rates are standard. Security is not."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: beaverton
    base_rate: 0.92
    rate_variance: 0.05

- id: beaverton_guard
  name: "Market Watch"
  npc_type: guard
  type: human
  description: "Patrols the market perimeter. Discourages weapons and loud disputes."
  level: 3
  max_hp: 32
  ac: 14
  awareness: 5
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: beaverton_hireling
  name: "Trail Hand"
  npc_type: hireling
  type: human
  description: "A capable mercenary available for day rates. Brings their own gear."
  level: 3
  max_hp: 28
  ac: 13
  awareness: 4
  disposition: neutral
  personality: opportunistic
  hireling:
    daily_cost: 75
    combat_role: melee
    max_follow_zones: 3
```

### Task 2.6 — `content/npcs/non_combat/downtown.yaml`

Zone: Downtown Portland. New safe room: `downtown_underground`. Optional: guard, fixer.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/downtown.yaml`:

```yaml
- id: downtown_merchant
  name: "Street Vendor"
  npc_type: merchant
  type: human
  description: "A wiry figure behind a folding table stacked with whatever fell off the last truck."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 180

- id: downtown_healer
  name: "Back-Alley Doc"
  npc_type: healer
  type: human
  description: "Moves like someone who's stitched a lot of wounds in bad light."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: downtown_job_trainer
  name: "The Fixer's Desk"
  npc_type: job_trainer
  type: human
  description: "A contact who knows who's hiring and what they need."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: downtown_banker
  name: "Cash Mutual"
  npc_type: banker
  type: human
  description: "No questions. No receipts. That's the deal."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: downtown
    base_rate: 0.90
    rate_variance: 0.05

- id: downtown_guard
  name: "Underground Muscle"
  npc_type: guard
  type: human
  description: "Big. Quiet. Watching the door."
  level: 3
  max_hp: 36
  ac: 14
  awareness: 5
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: downtown_fixer
  name: "The Middleman"
  npc_type: fixer
  type: human
  description: "Connects problems with solutions for a modest fee."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 5
  disposition: neutral
  personality: cowardly
  fixer:
    npc_variance: 1.15
    max_wanted_level: 4
    base_costs:
      1: 150
      2: 350
      3: 700
      4: 1500
```

### Task 2.7 — `content/npcs/non_combat/hillsboro.yaml`

Zone: Hillsboro. New safe room: `hills_the_keep`. Optional: guard, hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/hillsboro.yaml`:

```yaml
- id: hillsboro_merchant
  name: "Kingdom Merchant"
  npc_type: merchant
  type: human
  description: "Trades under the enclave's charter. Prices are set by the keep; complaints go through channels."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.35
    buy_margin: 0.4
    budget: 1100
    inventory:
      - item_id: stim_pack
        base_price: 55
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 8
      max_hours: 14
      stock_refill: 1
      budget_refill: 220

- id: hillsboro_healer
  name: "Court Surgeon"
  npc_type: healer
  type: human
  description: "Trained in the old methods, extended by necessity. Treats everyone who can pay the keep's rate."
  level: 4
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 5
    daily_capacity: 120

- id: hillsboro_job_trainer
  name: "The Chamberlain"
  npc_type: job_trainer
  type: human
  description: "Oversees assignments and training within the enclave. Formal but fair."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: hillsboro_banker
  name: "Royal Treasury"
  npc_type: banker
  type: human
  description: "The keep's treasury agent. Stores wealth under the enclave's seal. Rates reflect stability."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: hillsboro
    base_rate: 0.93
    rate_variance: 0.03

- id: hillsboro_guard
  name: "Keep Knight"
  npc_type: guard
  type: human
  description: "Armored and serious. Enforces the keep's rules on who enters and what they bring."
  level: 4
  max_hp: 40
  ac: 16
  awareness: 6
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: hillsboro_hireling
  name: "Sworn Sword"
  npc_type: hireling
  type: human
  description: "An enclave fighter available for contract work outside the walls."
  level: 4
  max_hp: 32
  ac: 14
  awareness: 4
  disposition: neutral
  personality: brave
  hireling:
    daily_cost: 100
    combat_role: melee
    max_follow_zones: 3
```

### Task 2.8 — `content/npcs/non_combat/ne_portland.yaml`

Zone: NE Portland. New safe room: `ne_corner_store`. No optional NPCs.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/ne_portland.yaml`:

```yaml
- id: ne_portland_merchant
  name: "Neighborhood Mike"
  npc_type: merchant
  type: human
  description: "Runs the corner store from behind a plywood counter. Knows what the neighborhood needs."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.45
    budget: 800
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 10
      stock_refill: 1
      budget_refill: 160

- id: ne_portland_healer
  name: "Nurse Practitioner"
  npc_type: healer
  type: human
  description: "Runs a walk-in clinic from the back of the store. Not a doctor, but close enough."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 3
    daily_capacity: 90

- id: ne_portland_job_trainer
  name: "Trade School Rep"
  npc_type: job_trainer
  type: human
  description: "Advocates for community job programs. Has a list of openings and the training to get you ready."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 120
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 200
        prerequisites:
          min_level: 2

- id: ne_portland_banker
  name: "Credit Union"
  npc_type: banker
  type: human
  description: "A neighborhood banking cooperative run out of a repurposed ATM vestibule."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: ne_portland
    base_rate: 0.91
    rate_variance: 0.04
```

### Task 2.9 — `content/npcs/non_combat/pdx_international.yaml`

Zone: PDX International. New safe room: `pdx_terminal_b`. Optional: guard.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/pdx_international.yaml`:

```yaml
- id: pdx_international_merchant
  name: "Duty-Free Dani"
  npc_type: merchant
  type: human
  description: "Runs the only legitimate goods counter in the terminal. The duty-free signage is ironic."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 8
      max_hours: 16
      stock_refill: 1
      budget_refill: 200

- id: pdx_international_healer
  name: "Airport Medic"
  npc_type: healer
  type: human
  description: "Stationed at Terminal B. Keeps a defibrillator and a first-aid kit. Busier than expected."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: pdx_international_job_trainer
  name: "Gate Agent"
  npc_type: job_trainer
  type: human
  description: "Former airport staff who now manages job routing in and out of the terminal zone."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: pdx_international_banker
  name: "Currency Exchange"
  npc_type: banker
  type: human
  description: "Still operates behind the old currency exchange counter. The rates have changed considerably."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: pdx_international
    base_rate: 0.89
    rate_variance: 0.05

- id: pdx_international_guard
  name: "TSA Remnant"
  npc_type: guard
  type: human
  description: "Still wearing the old badge out of habit. Still checking for contraband out of instinct."
  level: 3
  max_hp: 32
  ac: 13
  awareness: 5
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false
```

### Task 2.10 — `content/npcs/non_combat/ross_island.yaml`

Zone: Ross Island. New safe room: `ross_dock_shack`. Optional: hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/ross_island.yaml`:

```yaml
- id: ross_island_merchant
  name: "Island Trader"
  npc_type: merchant
  type: human
  description: "Sells salvage and supplies from the dock shack. Everything smells faintly of river."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.5
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 4
        max_stock: 8
    replenish_rate:
      min_hours: 8
      max_hours: 16
      stock_refill: 1
      budget_refill: 180

- id: ross_island_healer
  name: "Boat Medic"
  npc_type: healer
  type: human
  description: "Trained to patch wounds on a moving deck. Perfectly adequate on solid ground."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 80

- id: ross_island_job_trainer
  name: "The Captain"
  npc_type: job_trainer
  type: human
  description: "Retired river captain who now teaches anyone who's willing to put in the work."
  level: 4
  max_hp: 24
  ac: 11
  awareness: 5
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: ross_island_banker
  name: "The Chest"
  npc_type: banker
  type: human
  description: "Keeps the island's communal funds in a literal waterproofed chest. Reliable if basic."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: ross_island
    base_rate: 0.90
    rate_variance: 0.05

- id: ross_island_hireling
  name: "Deck Hand"
  npc_type: hireling
  type: human
  description: "Strong, experienced, and available. Works for day rates plus a meal."
  level: 3
  max_hp: 28
  ac: 12
  awareness: 4
  disposition: neutral
  personality: opportunistic
  hireling:
    daily_cost: 60
    combat_role: melee
    max_follow_zones: 2
```

### Task 2.11 — `content/npcs/non_combat/rustbucket_ridge.yaml`

Zone: Rustbucket Ridge. New safe room: `rust_scrap_office`. Optional: hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/rustbucket_ridge.yaml`:

```yaml
- id: rustbucket_ridge_merchant
  name: "Parts Dealer"
  npc_type: merchant
  type: human
  description: "Sells salvaged parts and supplies out of the scrap office. Everything is priced by weight."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: rustbucket_ridge_healer
  name: "Welder's Medic"
  npc_type: healer
  type: human
  description: "Handles burns, lacerations, and crush injuries. The ridge keeps them busy."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: rustbucket_ridge_job_trainer
  name: "Shop Foreman"
  npc_type: job_trainer
  type: human
  description: "Runs the work floor and takes on trainees. Blunt, experienced, and effective."
  level: 4
  max_hp: 24
  ac: 11
  awareness: 5
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: rustbucket_ridge_banker
  name: "The Lockbox"
  npc_type: banker
  type: human
  description: "Manages the ridge's communal savings in a series of locked strongboxes. Combination changes weekly."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: rustbucket_ridge
    base_rate: 0.91
    rate_variance: 0.05

- id: rustbucket_ridge_hireling
  name: "Grease Monkey"
  npc_type: hireling
  type: human
  description: "A capable hand who knows how to fight and fix things in equal measure."
  level: 3
  max_hp: 26
  ac: 12
  awareness: 4
  disposition: neutral
  personality: opportunistic
  hireling:
    daily_cost: 65
    combat_role: melee
    max_follow_zones: 3
```

### Task 2.12 — `content/npcs/non_combat/sauvie_island.yaml`

Zone: Sauvie Island. New safe room: `sauvie_farm_stand`. No optional NPCs.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/sauvie_island.yaml`:

```yaml
- id: sauvie_island_merchant
  name: "Farmer's Market"
  npc_type: merchant
  type: human
  description: "A composed farmer selling produce and preserved goods. Everything is local."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: consumables
    sell_margin: 1.2
    buy_margin: 0.5
    budget: 700
    inventory:
      - item_id: stim_pack
        base_price: 40
        init_stock: 4
        max_stock: 8
    replenish_rate:
      min_hours: 8
      max_hours: 16
      stock_refill: 1
      budget_refill: 140

- id: sauvie_island_healer
  name: "Herb Woman"
  npc_type: healer
  type: human
  description: "Uses locally grown herbs and careful hands. The oldest healer on the island by far."
  level: 3
  max_hp: 18
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 2
    daily_capacity: 60

- id: sauvie_island_job_trainer
  name: "The Old Hand"
  npc_type: job_trainer
  type: human
  description: "Has farmed, fought, and survived on Sauvie Island for decades. Teaches what's worked."
  level: 4
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 100
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 175
        prerequisites:
          min_level: 2

- id: sauvie_island_banker
  name: "The Tin Can"
  npc_type: banker
  type: human
  description: "Manages the island cooperative's shared funds. The tin can nickname is not entirely metaphorical."
  level: 2
  max_hp: 18
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: sauvie_island
    base_rate: 0.89
    rate_variance: 0.04
```

### Task 2.13 — `content/npcs/non_combat/se_industrial.yaml`

Zone: SE Industrial. New safe room: `sei_break_room`. Optional: guard, hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/se_industrial.yaml`:

```yaml
- id: se_industrial_merchant
  name: "Shift Trader"
  npc_type: merchant
  type: human
  description: "Sells supplies between shifts from a wheeled cart. Knows what every worker needs."
  level: 2
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.45
    budget: 900
    inventory:
      - item_id: stim_pack
        base_price: 45
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 10
      stock_refill: 1
      budget_refill: 180

- id: se_industrial_healer
  name: "Plant Medic"
  npc_type: healer
  type: human
  description: "Handles industrial injuries with the speed and competence of long practice."
  level: 3
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 3
    daily_capacity: 120

- id: se_industrial_job_trainer
  name: "Union Rep"
  npc_type: job_trainer
  type: human
  description: "Manages worker training and job placement for the industrial district. No work without dues."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 120
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 200
        prerequisites:
          min_level: 2

- id: se_industrial_banker
  name: "Paymaster"
  npc_type: banker
  type: human
  description: "Handles wage distribution and deposits for the district. Shift workers trust the paymaster."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: se_industrial
    base_rate: 0.91
    rate_variance: 0.04

- id: se_industrial_guard
  name: "Shop Steward"
  npc_type: guard
  type: human
  description: "Enforces break-room rules and zone safety protocols. Takes the job seriously."
  level: 3
  max_hp: 32
  ac: 13
  awareness: 5
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: se_industrial_hireling
  name: "Temp Worker"
  npc_type: hireling
  type: human
  description: "Available on short notice. Reliable in a fight, reliable on a loading dock."
  level: 2
  max_hp: 26
  ac: 12
  awareness: 3
  disposition: neutral
  personality: opportunistic
  hireling:
    daily_cost: 55
    combat_role: melee
    max_follow_zones: 2
```

### Task 2.14 — `content/npcs/non_combat/the_couve.yaml`

Zone: The Couve. New safe room: `couve_the_crossing`. Optional: guard, fixer.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/the_couve.yaml`:

```yaml
- id: the_couve_merchant
  name: "Border Trader"
  npc_type: merchant
  type: human
  description: "Runs a checkpoint stall at The Crossing. Stocks whatever comes across the bridge."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: the_couve_healer
  name: "Couve Medic"
  npc_type: healer
  type: human
  description: "Faction-aligned but treats anyone at the checkpoint. The Couve values functional allies."
  level: 3
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: the_couve_job_trainer
  name: "The Recruiter"
  npc_type: job_trainer
  type: human
  description: "Handles faction recruitment and skill verification at the crossing. Practical and efficient."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: the_couve_banker
  name: "Border Bank"
  npc_type: banker
  type: human
  description: "Manages currency exchange and deposits at the Washington end. Rates favor the faction."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: the_couve
    base_rate: 0.91
    rate_variance: 0.04

- id: the_couve_guard
  name: "Crossing Guard"
  npc_type: guard
  type: human
  description: "Controls access at The Crossing. Professional, not aggressive. Trouble is declined at the door."
  level: 3
  max_hp: 36
  ac: 15
  awareness: 6
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: the_couve_fixer
  name: "The Smuggler"
  npc_type: fixer
  type: human
  description: "Technically a checkpoint official. Practically, the most connected person at the bridge."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 6
  disposition: neutral
  personality: cowardly
  fixer:
    npc_variance: 1.2
    max_wanted_level: 4
    base_costs:
      1: 200
      2: 450
      3: 900
      4: 1800
```

### Task 2.15 — `content/npcs/non_combat/troutdale.yaml`

Zone: Troutdale. New safe room: `trout_truck_stop`. No optional NPCs.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/troutdale.yaml`:

```yaml
- id: troutdale_merchant
  name: "Road Trader"
  npc_type: merchant
  type: human
  description: "Sells goods to everyone passing through. The truck stop is the last chance before the gorge."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.3
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 50
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: troutdale_healer
  name: "Rest Stop Medic"
  npc_type: healer
  type: human
  description: "Patches up travelers before and after the gorge run. Volume business."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 90

- id: troutdale_job_trainer
  name: "The Dispatcher"
  npc_type: job_trainer
  type: human
  description: "Manages cargo runs and labor contracts out of the truck stop. Has work for anyone passing through."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 250
        prerequisites:
          min_level: 2

- id: troutdale_banker
  name: "Troutdale Trust"
  npc_type: banker
  type: human
  description: "A regional trust operating from the truck stop. Understands the road economy."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: troutdale
    base_rate: 0.91
    rate_variance: 0.04
```

### Task 2.16 — `content/npcs/non_combat/vantucky.yaml`

Zone: Vantucky. New safe room: `vantucky_the_compound`. Optional: guard, hireling.

- [ ] Create `/home/cjohannsen/src/mud/content/npcs/non_combat/vantucky.yaml`:

```yaml
- id: vantucky_merchant
  name: "Compound Trader"
  npc_type: merchant
  type: human
  description: "Supplies the militia compound from a locked storeroom. Standard-issue goods at standard-issue prices."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  merchant:
    merchant_type: general
    sell_margin: 1.25
    buy_margin: 0.45
    budget: 1000
    inventory:
      - item_id: stim_pack
        base_price: 48
        init_stock: 5
        max_stock: 10
    replenish_rate:
      min_hours: 6
      max_hours: 12
      stock_refill: 1
      budget_refill: 200

- id: vantucky_healer
  name: "Compound Doc"
  npc_type: healer
  type: human
  description: "The militia's field surgeon. Treats compound members and approved visitors."
  level: 4
  max_hp: 22
  ac: 10
  awareness: 4
  disposition: neutral
  personality: neutral
  healer:
    price_per_hp: 4
    daily_capacity: 100

- id: vantucky_job_trainer
  name: "The Sergeant"
  npc_type: job_trainer
  type: human
  description: "Handles combat training and duty assignments within the compound."
  level: 5
  max_hp: 40
  ac: 14
  awareness: 6
  disposition: neutral
  personality: neutral
  job_trainer:
    offered_jobs:
      - job_id: scavenger
        training_cost: 150
        prerequisites:
          min_level: 1
      - job_id: drifter
        training_cost: 300
        prerequisites:
          min_level: 2

- id: vantucky_banker
  name: "The Ammo Box"
  npc_type: banker
  type: human
  description: "Manages the militia's financial reserves. Named for where the ledger is kept."
  level: 3
  max_hp: 20
  ac: 10
  awareness: 3
  disposition: neutral
  personality: neutral
  banker:
    zone_id: vantucky
    base_rate: 0.92
    rate_variance: 0.04

- id: vantucky_guard
  name: "Compound Guard"
  npc_type: guard
  type: human
  description: "Armed militia member stationed at the compound entrance. Loyalty is the primary credential."
  level: 4
  max_hp: 38
  ac: 15
  awareness: 6
  disposition: neutral
  personality: brave
  guard:
    wanted_threshold: 2
    bribeable: false

- id: vantucky_hireling
  name: "Conscript"
  npc_type: hireling
  type: human
  description: "Available for hire outside the compound. Trained, motivated, and cheap."
  level: 3
  max_hp: 30
  ac: 13
  awareness: 4
  disposition: neutral
  personality: brave
  hireling:
    daily_cost: 65
    combat_role: melee
    max_follow_zones: 4
```

---

## Phase 3: Zone YAML Updates

### Task 3.1 — Update Zone YAML Files with New Safe Rooms

For each of the 12 zones requiring a new safe room, perform two edits:

1. Add the new safe room to the zone's `rooms:` list with correct `map_x`/`map_y`, `danger_level: safe`, description, exits, and spawns.
2. Add the forward exit to the anchor room.

The implementing agent MUST read each zone YAML, extract anchor room coordinates, compute the adjacent coordinate in the specified direction (north = y-2, south = y+2, east = x+2, west = x-2), and verify no existing room occupies those coordinates before writing.

**Direction offset convention** (derived from existing room coordinates in `downtown.yaml`): Each step is 2 units. North = y-2, South = y+2, East = x+2, West = x-2.

The spawns block in each new safe room MUST reference the NPC template IDs from Phase 2, with `count: 1` and `respawn_after: 0s` for each template in the zone's `non_combat` file.

**Zones and anchor coordinates to resolve:**

| Zone | New Room ID | Anchor Room ID | Direction | Reverse |
|------|------------|----------------|-----------|---------|
| `beaverton` | `beav_free_market` | `beav_canyon_road_east` | north | south |
| `downtown` | `downtown_underground` | `morrison_bridge` | north | south |
| `hillsboro` | `hills_the_keep` | `hills_tv_highway_east` | south | north |
| `ne_portland` | `ne_corner_store` | `ne_alberta_street` | north | south |
| `pdx_international` | `pdx_terminal_b` | `pdx_airport_way_west` | south | north |
| `ross_island` | `ross_dock_shack` | `ross_bridge_east` | east | west |
| `rustbucket_ridge` | `rust_scrap_office` | `last_stand_lodge` | east | west |
| `sauvie_island` | `sauvie_farm_stand` | `sauvie_bridge_south` | south | north |
| `se_industrial` | `sei_break_room` | `sei_holgate_blvd` | east | west |
| `the_couve` | `couve_the_crossing` | `couve_interstate_bridge_south` | west | east |
| `troutdale` | `trout_truck_stop` | `trout_i84_west` | north | south |
| `vantucky` | `vantucky_the_compound` | `vantucky_fourth_plain_west` | north | south |

- [ ] Update `content/zones/beaverton.yaml`: add `beav_free_market` room; add `north` exit to `beav_canyon_road_east`.
- [ ] Update `content/zones/downtown.yaml`: add `downtown_underground` room; add `north` exit to `morrison_bridge`.
- [ ] Update `content/zones/hillsboro.yaml`: add `hills_the_keep` room; add `south` exit to `hills_tv_highway_east`.
- [ ] Update `content/zones/ne_portland.yaml`: add `ne_corner_store` room; add `north` exit to `ne_alberta_street`.
- [ ] Update `content/zones/pdx_international.yaml`: add `pdx_terminal_b` room; add `south` exit to `pdx_airport_way_west`.
- [ ] Update `content/zones/ross_island.yaml`: add `ross_dock_shack` room; add `east` exit to `ross_bridge_east`.
- [ ] Update `content/zones/rustbucket_ridge.yaml`: add `rust_scrap_office` room; add `east` exit to `last_stand_lodge`.
- [ ] Update `content/zones/sauvie_island.yaml`: add `sauvie_farm_stand` room; add `south` exit to `sauvie_bridge_south`.
- [ ] Update `content/zones/se_industrial.yaml`: add `sei_break_room` room; add `east` exit to `sei_holgate_blvd`.
- [ ] Update `content/zones/the_couve.yaml`: add `couve_the_crossing` room; add `west` exit to `couve_interstate_bridge_south`.
- [ ] Update `content/zones/troutdale.yaml`: add `trout_truck_stop` room; add `north` exit to `trout_i84_west`.
- [ ] Update `content/zones/vantucky.yaml`: add `vantucky_the_compound` room; add `north` exit to `vantucky_fourth_plain_west`.

### Task 3.2 — Update Existing Safe Zone Files with Spawn Entries

For the 4 zones that already have a safe room (no new room needed), add `spawns:` entries to the designated safe room referencing the NPC templates from Phase 2.

- [ ] Update `content/zones/aloha.yaml`: add spawns to room `aloha_the_bazaar` for templates: `aloha_merchant`, `aloha_healer`, `aloha_job_trainer`, `aloha_banker`, `aloha_guard`, `aloha_fixer`.
- [ ] Update `content/zones/lake_oswego.yaml`: add spawns to room `lo_the_commons` for templates: `lake_oswego_merchant`, `lake_oswego_healer`, `lake_oswego_job_trainer`, `lake_oswego_banker`, `lake_oswego_hireling`.
- [ ] Update `content/zones/battleground.yaml`: add spawns to room `battle_infirmary` for templates: `battleground_merchant`, `battleground_healer`, `battleground_job_trainer`, `battleground_banker`, `battleground_guard`.
- [ ] Update `content/zones/felony_flats.yaml`: add spawns to room `flats_jade_district` for templates: `felony_flats_merchant`, `felony_flats_healer`, `felony_flats_job_trainer`, `felony_flats_banker`, `felony_flats_fixer`.

Each spawn entry format:
```yaml
    - template: <template_id>
      count: 1
      respawn_after: 0s
```

---

## Phase 4: Update NPC Template Loader to Load `non_combat/` Subdirectory

- [ ] Read `/home/cjohannsen/src/mud/internal/game/world/loader.go` to find where NPC templates are loaded.
- [ ] Verify the loader already scans subdirectories under `content/npcs/`, or determine if `content/npcs/non_combat/` must be explicitly registered.
- [ ] If the loader uses `LoadTemplates(dir)` which only reads flat `*.yaml` files (confirmed by `template.go` line 286: `os.ReadDir(dir)` iterating non-directory entries), the loader MUST be updated to also recurse into `content/npcs/non_combat/`.

**If a Go code change is required:**

- [ ] The implementing agent MUST open the loader, identify the NPC template directory constant or config, and add the subdirectory path following the existing pattern. This is the only permitted Go code change in this feature.
- [ ] The implementing agent MUST run existing NPC tests to confirm no regression: `cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... 2>&1 | tail -20`.

---

## Phase 5: Run Tests

- [ ] Run the full non-combat NPC coverage test suite:
  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run 'TestAllZones|TestNonCombat|TestOptional|TestNew|TestProperty_AllNonCombat' -v 2>&1 | tail -50
  ```
  All tests MUST pass.

- [ ] Run the full test suite:
  ```
  cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -30
  ```
  All tests MUST pass with 100% success (SWENG-6).

---

## Phase 6: Verify Feature Index

- [ ] Open `docs/features/index.yaml` and confirm the `non-combat-npcs-all-zones` entry status is updated to reflect implementation complete (change `status` from `blocked` or `backlog` to `complete` after all tests pass).

---

## Requirements Coverage Matrix

| Requirement | Phase | Task |
|-------------|-------|------|
| REQ-NCNAZ-0 (banker prereq) | Prerequisites | Verify `vera_coldcoin.yaml` exists |
| REQ-NCNAZ-1 (safe room per zone) | 3 | Tasks 3.1, 3.2 |
| REQ-NCNAZ-2 (bidirectional exits) | 3 | Task 3.1 |
| REQ-NCNAZ-3 (room descriptions) | 3 | Task 3.1 |
| REQ-NCNAZ-4 (four core types per zone) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-5 (optional types only in authorized zones) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-6 (no quest_giver/crafter) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-7 (template ID pattern) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-8 (spawn entry respawn_after: 0s) | 3 | Tasks 3.1, 3.2 |
| REQ-NCNAZ-9 (disposition: neutral) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-10 (unique lore-appropriate names) | 2 | Tasks 2.1–2.16 |
| REQ-NCNAZ-11 (file path convention) | 1, 2 | Tasks 1, 2.1–2.16 |
| REQ-NCNAZ-12 (non-overlapping coordinates) | 3 | Task 3.1 |
| REQ-NCNAZ-13 (anchor exit added) | 3 | Task 3.1 |

---

## Notes for Implementing Agent

- PLAN-NOTE-1: The `npc_type` field in YAML corresponds to the `NPCType string \`yaml:"npc_type"\`` field in `Template`. The spec uses `npc_role` as terminology but the YAML field is `npc_type`.
- PLAN-NOTE-2: The `respawn_after` field in zone spawn entries (room YAML) MUST be set to `0s` per REQ-NCNAZ-8. NPC templates use `respawn_delay` (not `respawn_after`). Non-combat NPC templates MUST leave `respawn_delay` empty — an empty string means "does not independently respawn" (the spawn system handles respawning via the zone entry). Do NOT add `respawn_delay` or `respawn_after` to NPC template YAML blocks.
- PLAN-NOTE-3: The template `Validate()` function requires `level >= 1`, `max_hp >= 1`, `ac >= 10`. All templates in this plan satisfy these constraints.
- PLAN-NOTE-4: Fixer templates MUST use `personality: cowardly` (enforced by `Validate()`). All fixer templates in this plan use `personality: cowardly`.
- PLAN-NOTE-5: The `TestNewSafeRoomsConnectedBidirectionally` test references exported `Room` and `Exit` fields. The implementing agent MUST verify the `world` package exposes these via its public `Zone`/`Room`/`Exit` types, and adjust test field access if needed after reading the full `loader.go`.
- PLAN-NOTE-6: Phase 4 (loader subdirectory) MUST be completed before Phase 5 tests will pass. If the loader already recurses into subdirectories, Phase 4 is a no-op.
