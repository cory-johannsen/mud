# Zone Content Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Ordering Constraint:** This plan MUST be implemented AFTER `world-map`. The zone YAML files must already have `world_x`/`world_y` fields added by world-map before this plan adds rooms — to avoid merge conflicts on the same files.

**Goal:** Expand each zone with additional rooms, per-room danger level variation, diversified combat NPC types, improved NPC distribution, and a chip_doc NPC in each zone.

**Architecture:** Primarily YAML content work with minor Go changes if new danger-level fields or room properties are needed. Each zone gets new room entries, updated danger levels on existing rooms, new NPC types, and a chip_doc NPC in a safe room.

**Tech Stack:** YAML content, existing zone/room/NPC loaders, Go data model extensions as needed

---

## Reference: Design Spec

Full spec: `docs/superpowers/specs/2026-03-21-zone-content-expansion-design.md`
Feature entry: `docs/features/zone-content-expansion.md`

Key requirements:
- REQ-ZCE-1: Every zone MUST have at least 30 rooms after expansion.
- REQ-ZCE-3: Every zone MUST contain exactly one safe cluster.
- REQ-ZCE-4: Safe cluster = 1 anchor Safe room + 2–4 adjacent Safe rooms (3–5 total).
- REQ-ZCE-5: All safe cluster rooms MUST have `danger_level: safe`.
- REQ-ZCE-6: chip_doc NPC distributed across safe cluster; no room exceeds 2 non-combat NPCs.
- REQ-ZCE-7: Zone edge rooms MUST have danger level one step toward `safe` from zone default.
- REQ-ZCE-8: Zone core rooms MUST have the zone's default danger level.
- REQ-ZCE-9: `safe`-default zones treat all non-cluster rooms as `safe`.
- REQ-ZCE-10: At least `floor(room_count / 10) + 2` distinct combat NPC types per zone.
- REQ-ZCE-12: Safe rooms MUST have 0 combat NPC spawns.
- REQ-ZCE-13: Sketchy rooms: 1–2 combat NPC spawns.
- REQ-ZCE-14: Dangerous rooms: 2–3 combat NPC spawns.
- REQ-ZCE-15: All Out War rooms: 3–4 combat NPC spawns.

## Reference: Zone Inventory

| Zone File | Default Danger | Current Rooms | Rooms Needed | Notes |
|-----------|---------------|---------------|--------------|-------|
| `aloha.yaml` | safe | 21 | +9 | All non-cluster rooms safe per REQ-ZCE-9 |
| `battleground.yaml` | all_out_war | 22 | +8 | Edge: dangerous, core: all_out_war; has existing safe room |
| `beaverton.yaml` | sketchy | 22 | +8 | Edge: safe, core: sketchy |
| `downtown.yaml` | sketchy | 13 | +17 | Largest expansion needed |
| `felony_flats.yaml` | dangerous | 22 | +8 | Edge: sketchy, core: dangerous; has existing safe room |
| `hillsboro.yaml` | sketchy | 21 | +9 | Edge: safe, core: sketchy |
| `lake_oswego.yaml` | safe | 22 | +8 | All non-cluster rooms safe per REQ-ZCE-9 |
| `ne_portland.yaml` | sketchy | 23 | +7 | Edge: safe, core: sketchy |
| `pdx_international.yaml` | sketchy | 23 | +7 | Edge: safe, core: sketchy |
| `ross_island.yaml` | dangerous | 21 | +9 | Edge: sketchy, core: dangerous |
| `rustbucket_ridge.yaml` | dangerous | 34 | 0 new rooms | Already ≥30; designation + NPC work only |
| `sauvie_island.yaml` | sketchy | 21 | +9 | Edge: safe, core: sketchy |
| `se_industrial.yaml` | dangerous | 21 | +9 | Edge: sketchy, core: dangerous |
| `the_couve.yaml` | sketchy | 20 | +10 | Edge: safe, core: sketchy |
| `troutdale.yaml` | sketchy | 23 | +7 | Edge: safe, core: sketchy |
| `vantucky.yaml` | dangerous | 21 | +9 | Edge: sketchy, core: dangerous |

## Reference: chip_doc NPC Pattern

The `chip_doc` NPC type is defined by the `curse-removal` feature (plan: `docs/superpowers/plans/2026-03-22-curse-removal.md`). Each zone needs one `chip_doc` NPC YAML file placed in the safe cluster. The YAML structure follows the healer/merchant pattern but with `npc_type: chip_doc` and a `chip_doc:` config block containing `removal_cost` and `check_dc` fields.

Example structure (from curse-removal plan):
```yaml
id: chip_doc_<zone_id>
name: <lore-appropriate name>
npc_type: chip_doc
description: >
  <lore-appropriate description>
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
chip_doc:
  removal_cost: 200
  check_dc: 14
```

Each zone's chip_doc NPC YAML file goes in `content/npcs/chip_doc_<zone_id>.yaml`.

## Reference: Room YAML Fields Used in This Plan

```yaml
- id: <room_id>
  title: <display name>
  danger_level: safe|sketchy|dangerous|all_out_war   # overrides zone default
  core: true                                          # marks as zone core room
  description: |
    ...
  exits:
  - direction: <direction>
    target: <room_id>
  map_x: <int>
  map_y: <int>
  spawns:
  - template: <npc_template_id>
    count: <int>
    respawn_after: <duration>
  properties:
    lighting: <value>
    atmosphere: <value>
```

---

## Task 1: rustbucket_ridge — Designation and NPC Diversification Only

**Scope:** `content/zones/rustbucket_ridge.yaml` — 34 rooms, no new rooms needed.

**Work:**
- Designate 5–8 rooms as core rooms (`core: true`) at the zone's `dangerous` default level.
- Designate a 3–5 room safe cluster (anchor + 2–4 adjacent rooms with `danger_level: safe`).
- Assign `danger_level: sketchy` to all non-core, non-cluster rooms.
- Verify combat NPC diversity: floor(34/10)+2 = 5 distinct NPC types required; add any missing types.
- Verify spawn density on all rooms matches REQ-ZCE-12 through REQ-ZCE-14.
- Add chip_doc NPC spawn in one safe cluster room.
- Create `content/npcs/chip_doc_rustbucket_ridge.yaml`.

**chip_doc NPC for this zone:** `chip_doc_rustbucket_ridge` — a grizzled back-alley implant tech who operates out of a reinforced corner of the ridge's neutral market.

**Steps:**
- [ ] **Step 1: Read current file**
  Prerequisite: verify `world_x` and `world_y` fields are present in each zone YAML before proceeding.
  Read `content/zones/rustbucket_ridge.yaml` in full.
- [ ] **Step 2: Identify and mark core rooms**
  Edit `content/zones/rustbucket_ridge.yaml` to add `core: true` to the 5–8 most dangerous/central rooms (gang strongholds, faction headquarters).
- [ ] **Step 3: Designate safe cluster**
  Edit the file: set `danger_level: safe` on 3–5 contiguous rooms forming the neutral market/safe haven. Verify they are adjacent (exit-connected).
- [ ] **Step 4: Assign edge room danger levels**
  Edit all non-core, non-cluster rooms to add `danger_level: sketchy`.
- [ ] **Step 5: Audit NPC diversity**
  Count distinct `template:` values across all spawns. If fewer than 5 distinct types, add 1–2 new combat NPC types (lore-appropriate: `scavenger`, `ganger`, `lieutenant`, `commissar`, etc.) to rooms lacking diversity.
- [ ] **Step 6: Audit spawn density**
  Safe rooms: remove any spawns. Sketchy rooms: 1–2 spawns. Dangerous rooms: 2–3 spawns. Fix any violations.
- [ ] **Step 7: Create chip_doc NPC file**
  Create `content/npcs/chip_doc_rustbucket_ridge.yaml`.
- [ ] **Step 8: Add chip_doc spawn to safe cluster**
  Edit `content/zones/rustbucket_ridge.yaml`: add a spawn entry for `chip_doc_rustbucket_ridge` in one safe cluster room. Verify that room has ≤1 other non-combat NPC.
- [ ] **Step 9: Commit**
  `git add content/zones/rustbucket_ridge.yaml content/npcs/chip_doc_rustbucket_ridge.yaml`
  `git commit -m "feat(zone-content-expansion): expand rustbucket_ridge — safe cluster, danger levels, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 2: downtown — Major Room Expansion (+17 rooms)

**Scope:** `content/zones/downtown.yaml` — 13 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (17 minimum):**
- `dt_old_post_office` — Old Post Office ruin (edge, safe)
- `dt_park_blocks_north` — North Park Blocks (edge, safe)
- `dt_park_blocks_south` — South Park Blocks (edge, safe)
- `dt_pearl_district_edge` — Pearl District Edge (edge, safe)
- `dt_burnside_overpass` — Burnside Overpass (edge, safe)
- `dt_union_station_ruins` — Union Station Ruins (edge, safe)
- `dt_chinatown_gate` — Chinatown Gate (core, sketchy)
- `dt_old_town_square` — Old Town Square (core, sketchy)
- `dt_powell_books_ruin` — Powell's Books Ruin (core, sketchy)
- `dt_city_hall` — City Hall Stronghold (core, sketchy)
- `dt_main_library` — Main Library Fortress (core, sketchy)
- `dt_salmon_street_springs` — Salmon Street Springs (safe cluster anchor, safe)
- `dt_esplanade_north` — East Bank Esplanade North (safe cluster, safe)
- `dt_esplanade_south` — East Bank Esplanade South (safe cluster, safe)
- `dt_hawthorne_bridge_east` — Hawthorne Bridge East Approach (safe cluster, safe)
- `dt_steel_bridge_west` — Steel Bridge West Approach (sketchy)
- `dt_naito_parkway` — Naito Parkway (sketchy)
- `dt_sw_hills_road` — SW Hills Road (safe cluster, safe)

That yields 13 + 18 = 31 rooms (≥30). Safe cluster: 5 rooms (salmon_street_springs anchor + esplanade_north, esplanade_south, hawthorne_bridge_east, sw_hills_road). Core rooms: city_hall, main_library, old_town_square, chinatown_gate, powell_books_ruin. Edge rooms: old_post_office, park_blocks_north, park_blocks_south, pearl_district_edge, burnside_overpass, union_station_ruins (plus most existing rooms).

NPC diversity required: floor(31/10)+2 = 5 types. Use: `ganger`, `scavenger`, `lieutenant`, `commissar`, `82nd_enforcer` (or zone-appropriate variants).

**chip_doc NPC for this zone:** `chip_doc_downtown` — a former hospital pharmacist turned implant specialist who operates a clinic in the esplanade safe zone.

**Steps:**
- [ ] **Step 1: Read current file**
  Read `content/zones/downtown.yaml` in full.
- [ ] **Step 2: Plan map coordinates**
  Review existing map_x/map_y values to assign non-overlapping coordinates to all 18 new rooms. Keep the safe cluster rooms in a coherent geographic cluster. Existing rooms span roughly x: -2 to +4, y: -2 to +2.
- [ ] **Step 3: Add 18 new rooms**
  Edit `content/zones/downtown.yaml`: append all 18 new room entries with correct exits connecting them to existing rooms and to each other. Each new room MUST have `map_x`, `map_y`, `title`, `description`, `exits`, and `properties`.
- [ ] **Step 4: Designate core rooms**
  Add `core: true` to `dt_city_hall`, `dt_main_library`, `dt_old_town_square`, `dt_chinatown_gate`, `dt_powell_books_ruin`. Ensure they have `danger_level: sketchy` (zone default — omitting `danger_level` on these rooms is correct since they inherit zone default).
- [ ] **Step 5: Designate safe cluster**
  Add `danger_level: safe` to `dt_salmon_street_springs`, `dt_esplanade_north`, `dt_esplanade_south`, `dt_hawthorne_bridge_east`, `dt_sw_hills_road`. Verify exit connectivity.
- [ ] **Step 6: Assign edge room danger levels**
  Add `danger_level: safe` to all non-core, non-cluster new rooms.
- [ ] **Step 7: Update existing room danger levels**
  Rooms `pioneer_square`, `transit_mall`, `director_park` are central — keep as zone default (no override). `waterfront_trail`, `burnside_crossing`, `broadway_ruins`, `market_district`, `morrison_bridge`, `courthouse_steps` are edge rooms — add `danger_level: safe`. `underground_max`, `cliff_base`, `cliff_top`, `ravine_flooded` are edge/isolated — add `danger_level: safe`.
- [ ] **Step 8: Audit NPC diversity and spawn density**
  Ensure ≥5 distinct NPC types in combat-spawning rooms. Sketchy rooms: 1–2 spawns. Safe rooms: 0 spawns.
- [ ] **Step 9: Create chip_doc NPC file**
  Create `content/npcs/chip_doc_downtown.yaml`.
- [ ] **Step 10: Add chip_doc spawn**
  Add chip_doc spawn to `dt_salmon_street_springs` (anchor safe room, 0 other non-combat NPCs at this point).
- [ ] **Step 11: Commit**
  `git add content/zones/downtown.yaml content/npcs/chip_doc_downtown.yaml`
  `git commit -m "feat(zone-content-expansion): expand downtown — 18 new rooms, safe cluster, danger levels, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 3: aloha — Room Expansion (+9 rooms, safe-default zone)

**Scope:** `content/zones/aloha.yaml` — 21 rooms → ≥30 rooms. Zone default: `safe`. Per REQ-ZCE-9, all non-cluster rooms are `safe`. Safe cluster still required per REQ-ZCE-3.

**New rooms to add (9):**
- `aloha_farm_collective` — Community Farm Collective (safe)
- `aloha_solar_array` — Solar Array Field (safe)
- `aloha_trade_road_east` — Trade Road East (safe)
- `aloha_water_tower` — Old Water Tower (safe)
- `aloha_school_shelter` — Converted School Shelter (safe cluster anchor, safe)
- `aloha_school_yard` — School Yard (safe cluster, safe)
- `aloha_clinic_wing` — Clinic Wing (safe cluster, safe)
- `aloha_market_hall` — Community Market Hall (safe)
- `aloha_orchard_road` — Orchard Road (safe)

NPC diversity: floor(30/10)+2 = 5 types. Since zone is `safe`, combat NPCs must be 0 in all rooms (REQ-ZCE-12). No combat spawns; non-combat NPCs only.

**chip_doc NPC for this zone:** `chip_doc_aloha` — a quiet technician in the clinic wing who patches implants between growing seasons.

**Steps:**
- [ ] **Step 1: Read current file**
  Read `content/zones/aloha.yaml` in full.
- [ ] **Step 2: Add 9 new rooms**
  Edit `content/zones/aloha.yaml`: append 9 new room entries. No combat spawns on any room.
- [ ] **Step 3: Designate safe cluster**
  Add `danger_level: safe` to `aloha_school_shelter`, `aloha_school_yard`, `aloha_clinic_wing`. These 3 rooms form the anchor cluster (augment to 4–5 if nearby rooms are appropriate). Verify connectivity.
- [ ] **Step 4: Create chip_doc NPC file**
  Create `content/npcs/chip_doc_aloha.yaml`.
- [ ] **Step 5: Add chip_doc spawn**
  Add chip_doc spawn to `aloha_clinic_wing`.
- [ ] **Step 6: Commit**
  `git add content/zones/aloha.yaml content/npcs/chip_doc_aloha.yaml`
  `git commit -m "feat(zone-content-expansion): expand aloha — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,9,12)"`

---

## Task 4: battleground — Room Expansion (+8 rooms, all_out_war zone)

**Scope:** `content/zones/battleground.yaml` — 22 rooms → ≥30 rooms. Zone default: `all_out_war`. Edge rooms: `dangerous`. Core rooms: `all_out_war`. Zone has one existing `danger_level: safe` room — expand safe cluster around it.

**New rooms to add (8):**
- `battle_commissar_hq` — Commissar Headquarters (core, all_out_war)
- `battle_armory` — Collective Armory (core, all_out_war)
- `battle_reeducation_block` — Reeducation Block (core, all_out_war)
- `battle_outer_perimeter_east` — Outer Perimeter East (edge, dangerous)
- `battle_outer_perimeter_west` — Outer Perimeter West (edge, dangerous)
- `battle_infirmary` — Collective Infirmary (safe cluster anchor, safe)
- `battle_infirmary_ward` — Infirmary Ward (safe cluster, safe)
- `battle_neutral_commons` — Neutral Commons (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types. All_out_war rooms: 3–4 spawns. Dangerous rooms: 2–3 spawns.

**chip_doc NPC for this zone:** `chip_doc_battleground` — a collective medtech who handles implant maintenance in the infirmary; officially neutral.

**Steps:**
- [ ] **Step 1: Read current file**
  Read `content/zones/battleground.yaml` in full.
- [ ] **Step 2: Add 8 new rooms**
  Edit `content/zones/battleground.yaml`: append 8 new room entries with appropriate exits.
- [ ] **Step 3: Designate core rooms**
  Add `core: true` to `battle_commissar_hq`, `battle_armory`, `battle_reeducation_block` and any existing rooms that qualify as zone core.
- [ ] **Step 4: Designate safe cluster**
  The existing `danger_level: safe` room becomes the safe cluster anchor. Add `danger_level: safe` to `battle_infirmary`, `battle_infirmary_ward`, `battle_neutral_commons`. Verify 3–5 total safe rooms and exit connectivity.
- [ ] **Step 5: Assign edge room danger levels**
  Add `danger_level: dangerous` to all non-core, non-cluster rooms.
- [ ] **Step 6: Audit NPC diversity and spawn density**
  Ensure ≥5 distinct NPC types. All_out_war rooms: 3–4 spawns. Dangerous: 2–3. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file**
  Create `content/npcs/chip_doc_battleground.yaml`.
- [ ] **Step 8: Add chip_doc spawn**
  Add chip_doc spawn to `battle_infirmary`.
- [ ] **Step 9: Commit**
  `git add content/zones/battleground.yaml content/npcs/chip_doc_battleground.yaml`
  `git commit -m "feat(zone-content-expansion): expand battleground — 8 new rooms, safe cluster, danger levels, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,14,15)"`

---

## Task 5: beaverton — Room Expansion (+8 rooms)

**Scope:** `content/zones/beaverton.yaml` — 22 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (8):**
- `beav_mall_core` — Mall Enforcement Core (core, sketchy)
- `beav_security_hq` — Security HQ (core, sketchy)
- `beav_fine_dining_ruins` — Fine Dining Ruins (edge, safe)
- `beav_suburban_ruins_east` — Suburban Ruins East (edge, safe)
- `beav_suburban_ruins_west` — Suburban Ruins West (edge, safe)
- `beav_neutral_cafe` — Neutral Ground Café (safe cluster anchor, safe)
- `beav_neutral_workshop` — Repair Workshop (safe cluster, safe)
- `beav_neutral_market` — Neutral Market Stalls (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types. Sketchy rooms: 1–2 spawns. Safe: 0.

**chip_doc NPC for this zone:** `chip_doc_beaverton` — a twitchy tech refugee who operates an implant repair kiosk in the neutral café basement.

**Steps:**
- [ ] **Step 1: Read current file**
  Read `content/zones/beaverton.yaml` in full.
- [ ] **Step 2: Add 8 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `beav_mall_core`, `beav_security_hq`, plus 1–2 existing central rooms.
- [ ] **Step 4: Designate safe cluster** — `danger_level: safe` on `beav_neutral_cafe`, `beav_neutral_workshop`, `beav_neutral_market` (3-room cluster); add a 4th if a nearby existing room qualifies.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe` on all non-core, non-cluster rooms.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Sketchy: 1–2 spawns. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_beaverton.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `beav_neutral_cafe`.
- [ ] **Step 9: Commit**
  `git add content/zones/beaverton.yaml content/npcs/chip_doc_beaverton.yaml`
  `git commit -m "feat(zone-content-expansion): expand beaverton — 8 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 6: felony_flats — Room Expansion (+8 rooms)

**Scope:** `content/zones/felony_flats.yaml` — 22 rooms → ≥30 rooms. Zone default: `dangerous`. Edge: `sketchy`. Core: `dangerous`. Has existing `danger_level: safe` room (`flats_jade_district`) — expand safe cluster from it.

**New rooms to add (8):**
- `flats_crack_house_row` — Crack House Row (core, dangerous)
- `flats_enforcer_den` — Enforcer Den (core, dangerous)
- `flats_alley_north` — North Alley (edge, sketchy)
- `flats_dumpster_yard` — Dumpster Yard (edge, sketchy)
- `flats_mechanic_lot` — Mechanic Lot (edge, sketchy)
- `flats_safe_house` — Safe House (safe cluster, safe)
- `flats_back_room` — Back Room (safe cluster, safe)
- `flats_jade_alley` — Jade District Alley (safe cluster, safe)

Safe cluster: existing `flats_jade_district` + `flats_safe_house`, `flats_back_room`, `flats_jade_alley` = 4 rooms.

NPC diversity: floor(30/10)+2 = 5 types. Dangerous rooms: 2–3 spawns. Sketchy: 1–2. Safe: 0.

**chip_doc NPC for this zone:** `chip_doc_felony_flats` — a former street surgeon who patches up anyone in the Jade District for the right price.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/felony_flats.yaml` in full.
- [ ] **Step 2: Add 8 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `flats_crack_house_row`, `flats_enforcer_den`, plus 2–3 existing high-danger rooms.
- [ ] **Step 4: Expand safe cluster** — Add `danger_level: safe` to `flats_safe_house`, `flats_back_room`, `flats_jade_alley`. Verify they connect to existing `flats_jade_district`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: sketchy` on all non-core, non-cluster rooms.
- [ ] **Step 6: Audit NPC diversity and spawn density**.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_felony_flats.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `flats_jade_district`.
- [ ] **Step 9: Commit**
  `git add content/zones/felony_flats.yaml content/npcs/chip_doc_felony_flats.yaml`
  `git commit -m "feat(zone-content-expansion): expand felony_flats — 8 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 7: hillsboro — Room Expansion (+9 rooms)

**Scope:** `content/zones/hillsboro.yaml` — 21 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (9):**
- `hills_fortress_core` — Knight Fortress Core (core, sketchy)
- `hills_armory_hall` — Armory Hall (core, sketchy)
- `hills_outer_fields_north` — Outer Fields North (edge, safe)
- `hills_outer_fields_south` — Outer Fields South (edge, safe)
- `hills_east_road` — East Road (edge, safe)
- `hills_waystation` — Waystation (safe cluster anchor, safe)
- `hills_waystation_yard` — Waystation Yard (safe cluster, safe)
- `hills_waystation_shelter` — Waystation Shelter (safe cluster, safe)
- `hills_trading_post` — Trading Post (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_hillsboro` — a wandering implant mechanic who set up shop in the waystation.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/hillsboro.yaml` in full.
- [ ] **Step 2: Add 9 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `hills_fortress_core`, `hills_armory_hall`, plus 1–2 existing central rooms.
- [ ] **Step 4: Designate safe cluster** — 4 rooms: `hills_waystation`, `hills_waystation_yard`, `hills_waystation_shelter`, `hills_trading_post`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe`.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_hillsboro.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `hills_waystation_shelter`.
- [ ] **Step 9: Commit**
  `git add content/zones/hillsboro.yaml content/npcs/chip_doc_hillsboro.yaml`
  `git commit -m "feat(zone-content-expansion): expand hillsboro — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 8: lake_oswego — Room Expansion (+8 rooms, safe-default zone)

**Scope:** `content/zones/lake_oswego.yaml` — 22 rooms → ≥30 rooms. Zone default: `safe`. Per REQ-ZCE-9, all non-cluster rooms are `safe`. No combat spawns.

**New rooms to add (8):**
- `lo_lakeside_promenade` — Lakeside Promenade (safe)
- `lo_country_club_east` — Country Club East Wing (safe)
- `lo_country_club_west` — Country Club West Wing (safe)
- `lo_south_shore` — South Shore (safe)
- `lo_garden_district` — Garden District (safe)
- `lo_civic_center` — Civic Center (safe cluster anchor, safe)
- `lo_civic_garden` — Civic Garden (safe cluster, safe)
- `lo_civic_hall` — Civic Hall (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types (non-combat only).

**chip_doc NPC for this zone:** `chip_doc_lake_oswego` — a discreet implant specialist operating from a tastefully-appointed clinic in the civic hall.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/lake_oswego.yaml` in full.
- [ ] **Step 2: Add 8 new rooms** — Edit file, append rooms. No combat spawns.
- [ ] **Step 3: Designate safe cluster** — `lo_civic_center`, `lo_civic_garden`, `lo_civic_hall` as 3-room cluster.
- [ ] **Step 4: Create chip_doc NPC file** — Create `content/npcs/chip_doc_lake_oswego.yaml`.
- [ ] **Step 5: Add chip_doc spawn** — Add to `lo_civic_hall`.
- [ ] **Step 6: Commit**
  `git add content/zones/lake_oswego.yaml content/npcs/chip_doc_lake_oswego.yaml`
  `git commit -m "feat(zone-content-expansion): expand lake_oswego — 8 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,9,12)"`

---

## Task 9: ne_portland — Room Expansion (+7 rooms)

**Scope:** `content/zones/ne_portland.yaml` — 23 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (7):**
- `ne_brewery_vault` — Brewery Vault (core, sketchy)
- `ne_courier_dispatch` — Courier Dispatch Hub (core, sketchy)
- `ne_overpass_camp` — Overpass Camp (edge, safe)
- `ne_riverside_path` — Riverside Path (edge, safe)
- `ne_neutral_garage` — Neutral Garage (safe cluster anchor, safe)
- `ne_neutral_lot` — Neutral Lot (safe cluster, safe)
- `ne_neutral_bunker` — Neutral Bunker (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_ne_portland` — a bike-courier wash-out who retrained as an implant tech and operates from the neutral garage.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/ne_portland.yaml` in full.
- [ ] **Step 2: Add 7 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `ne_brewery_vault`, `ne_courier_dispatch`, plus 1–2 existing rooms (`ne_brewery_compound`, `ne_mississippi_ave`).
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `ne_neutral_garage`, `ne_neutral_lot`, `ne_neutral_bunker`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe` on non-core, non-cluster rooms.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_ne_portland.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `ne_neutral_garage`.
- [ ] **Step 9: Commit**
  `git add content/zones/ne_portland.yaml content/npcs/chip_doc_ne_portland.yaml`
  `git commit -m "feat(zone-content-expansion): expand ne_portland — 7 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 10: pdx_international — Room Expansion (+7 rooms)

**Scope:** `content/zones/pdx_international.yaml` — 23 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (7):**
- `pdx_terminal_core` — Terminal Core (core, sketchy)
- `pdx_control_tower` — Control Tower (core, sketchy)
- `pdx_cargo_bay_b` — Cargo Bay B (edge, safe)
- `pdx_runway_east` — Runway East (edge, safe)
- `pdx_baggage_claim_north` — Baggage Claim North (edge, safe)
- `pdx_maintenance_bay` — Maintenance Bay (safe cluster anchor, safe)
- `pdx_ground_crew_lounge` — Ground Crew Lounge (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_pdx_international` — a former aircraft technician repurposed as an implant specialist in the maintenance bay.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/pdx_international.yaml` in full.
- [ ] **Step 2: Add 7 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `pdx_terminal_core`, `pdx_control_tower`, plus existing terminal hub rooms.
- [ ] **Step 4: Designate safe cluster** — `pdx_maintenance_bay`, `pdx_ground_crew_lounge` + 1 existing suitable room = 3 rooms minimum.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe`.
- [ ] **Step 6: Audit NPC diversity and spawn density**.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_pdx_international.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `pdx_maintenance_bay`.
- [ ] **Step 9: Commit**
  `git add content/zones/pdx_international.yaml content/npcs/chip_doc_pdx_international.yaml`
  `git commit -m "feat(zone-content-expansion): expand pdx_international — 7 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 11: ross_island — Room Expansion (+9 rooms)

**Scope:** `content/zones/ross_island.yaml` — 21 rooms → ≥30 rooms. Zone default: `dangerous`. Edge: `sketchy`. Core: `dangerous`.

**New rooms to add (9):**
- `ross_gravel_pit_core` — Gravel Pit Core (core, dangerous)
- `ross_hermit_stronghold` — Hermit Stronghold (core, dangerous)
- `ross_toll_inner` — Inner Toll Zone (core, dangerous)
- `ross_north_shore_trail` — North Shore Trail (edge, sketchy)
- `ross_west_grove` — West Grove (edge, sketchy)
- `ross_south_shore_clearing` — South Shore Clearing (edge, sketchy)
- `ross_fishers_cove` — Fisher's Cove (safe cluster anchor, safe)
- `ross_cove_shelter` — Cove Shelter (safe cluster, safe)
- `ross_cove_dock` — Cove Dock (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types. Dangerous: 2–3 spawns. Sketchy: 1–2. Safe: 0.

**chip_doc NPC for this zone:** `chip_doc_ross_island` — a hermit medic who trades implant work for food at the cove shelter.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/ross_island.yaml` in full.
- [ ] **Step 2: Add 9 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on new core rooms plus qualifying existing rooms.
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `ross_fishers_cove`, `ross_cove_shelter`, `ross_cove_dock`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: sketchy`.
- [ ] **Step 6: Audit NPC diversity and spawn density**.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_ross_island.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `ross_cove_shelter`.
- [ ] **Step 9: Commit**
  `git add content/zones/ross_island.yaml content/npcs/chip_doc_ross_island.yaml`
  `git commit -m "feat(zone-content-expansion): expand ross_island — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 12: sauvie_island — Room Expansion (+9 rooms)

**Scope:** `content/zones/sauvie_island.yaml` — 21 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (9):**
- `sauvie_harvest_compound` — Harvest Compound (core, sketchy)
- `sauvie_guard_post_inner` — Inner Guard Post (core, sketchy)
- `sauvie_north_fields` — North Fields (edge, safe)
- `sauvie_south_bog` — South Bog (edge, safe)
- `sauvie_river_landing` — River Landing (edge, safe)
- `sauvie_east_orchards` — East Orchards (edge, safe)
- `sauvie_community_barn` — Community Barn (safe cluster anchor, safe)
- `sauvie_barn_loft` — Barn Loft (safe cluster, safe)
- `sauvie_barn_yard` — Barn Yard (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_sauvie_island` — a farmhand-turned-implant-tech who operates from the barn's tool room.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/sauvie_island.yaml` in full.
- [ ] **Step 2: Add 9 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on harvest_compound, guard_post_inner, plus existing central rooms.
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `sauvie_community_barn`, `sauvie_barn_loft`, `sauvie_barn_yard`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe`.
- [ ] **Step 6: Audit NPC diversity and spawn density**.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_sauvie_island.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `sauvie_community_barn`.
- [ ] **Step 9: Commit**
  `git add content/zones/sauvie_island.yaml content/npcs/chip_doc_sauvie_island.yaml`
  `git commit -m "feat(zone-content-expansion): expand sauvie_island — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 13: se_industrial — Room Expansion (+9 rooms)

**Scope:** `content/zones/se_industrial.yaml` — 21 rooms → ≥30 rooms. Zone default: `dangerous`. Edge: `sketchy`. Core: `dangerous`.

**New rooms to add (9):**
- `sei_forge_core` — Forge Core (core, dangerous)
- `sei_scrap_kingpin_den` — Scrap Kingpin Den (core, dangerous)
- `sei_smelter_floor` — Smelter Floor (core, dangerous)
- `sei_outer_rail_east` — Outer Rail East (edge, sketchy)
- `sei_access_road_north` — Access Road North (edge, sketchy)
- `sei_riverside_alley` — Riverside Alley (edge, sketchy)
- `sei_worker_break_room` — Worker Break Room (safe cluster anchor, safe)
- `sei_union_office` — Old Union Office (safe cluster, safe)
- `sei_locker_room` — Locker Room (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_se_industrial` — a dock worker's union medic who patches implants in the locker room between shifts.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/se_industrial.yaml` in full.
- [ ] **Step 2: Add 9 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on new core rooms plus existing `sei_dry_dock`, `sei_container_maze`.
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `sei_worker_break_room`, `sei_union_office`, `sei_locker_room`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: sketchy` on non-core, non-cluster rooms.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Dangerous: 2–3. Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_se_industrial.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `sei_locker_room`.
- [ ] **Step 9: Commit**
  `git add content/zones/se_industrial.yaml content/npcs/chip_doc_se_industrial.yaml`
  `git commit -m "feat(zone-content-expansion): expand se_industrial — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 14: the_couve — Room Expansion (+10 rooms)

**Scope:** `content/zones/the_couve.yaml` — 20 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (10):**
- `couve_militia_hq` — Militia Headquarters (core, sketchy)
- `couve_arsenal` — Militia Arsenal (core, sketchy)
- `couve_outer_highway_east` — Outer Highway East (edge, safe)
- `couve_outer_highway_west` — Outer Highway West (edge, safe)
- `couve_suburban_sprawl` — Suburban Sprawl (edge, safe)
- `couve_river_bank` — Columbia River Bank (edge, safe)
- `couve_neutral_diner` — Neutral Ground Diner (safe cluster anchor, safe)
- `couve_diner_kitchen` — Diner Kitchen (safe cluster, safe)
- `couve_diner_back_room` — Diner Back Room (safe cluster, safe)
- `couve_abandoned_church` — Abandoned Church (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_the_couve` — a militia deserter who trades implant work for meals at the neutral diner.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/the_couve.yaml` in full.
- [ ] **Step 2: Add 10 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `couve_militia_hq`, `couve_arsenal`, plus 1–2 existing rooms.
- [ ] **Step 4: Designate safe cluster** — 4 rooms: `couve_neutral_diner`, `couve_diner_kitchen`, `couve_diner_back_room`, `couve_abandoned_church`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe`.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_the_couve.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `couve_diner_back_room`.
- [ ] **Step 9: Commit**
  `git add content/zones/the_couve.yaml content/npcs/chip_doc_the_couve.yaml`
  `git commit -m "feat(zone-content-expansion): expand the_couve — 10 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 15: troutdale — Room Expansion (+7 rooms)

**Scope:** `content/zones/troutdale.yaml` — 23 rooms → ≥30 rooms. Zone default: `sketchy`. Edge: `safe`. Core: `sketchy`.

**New rooms to add (7):**
- `trout_gorge_runner_camp` — Gorge Runner Camp (core, sketchy)
- `trout_highway_junction` — Highway Junction (core, sketchy)
- `trout_river_road_south` — River Road South (edge, safe)
- `trout_airport_ruins_edge` — Airport Ruins Edge (edge, safe)
- `trout_neutral_camp` — Neutral Traveler Camp (safe cluster anchor, safe)
- `trout_camp_supply_tent` — Camp Supply Tent (safe cluster, safe)
- `trout_camp_fire_circle` — Camp Fire Circle (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_troutdale` — a traveling medic who camps at the neutral traveler camp between runs through the gorge.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/troutdale.yaml` in full.
- [ ] **Step 2: Add 7 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `trout_gorge_runner_camp`, `trout_highway_junction`, plus 1–2 existing rooms.
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `trout_neutral_camp`, `trout_camp_supply_tent`, `trout_camp_fire_circle`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: safe`.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_troutdale.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `trout_camp_supply_tent`.
- [ ] **Step 9: Commit**
  `git add content/zones/troutdale.yaml content/npcs/chip_doc_troutdale.yaml`
  `git commit -m "feat(zone-content-expansion): expand troutdale — 7 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12,13)"`

---

## Task 16: vantucky — Room Expansion (+9 rooms)

**Scope:** `content/zones/vantucky.yaml` — 21 rooms → ≥30 rooms. Zone default: `dangerous`. Edge: `sketchy`. Core: `dangerous`.

**New rooms to add (9):**
- `vantucky_militia_core` — Militia Core (core, dangerous)
- `vantucky_barracks` — Militia Barracks (core, dangerous)
- `vantucky_outer_burbs_east` — Outer Burbs East (edge, sketchy)
- `vantucky_outer_burbs_west` — Outer Burbs West (edge, sketchy)
- `vantucky_creek_trail` — Creek Trail (edge, sketchy)
- `vantucky_gas_stop` — Gas Stop (edge, sketchy)
- `vantucky_neutral_lot` — Neutral Lot (safe cluster anchor, safe)
- `vantucky_neutral_shed` — Neutral Shed (safe cluster, safe)
- `vantucky_neutral_corner` — Neutral Corner (safe cluster, safe)

NPC diversity: floor(30/10)+2 = 5 types.

**chip_doc NPC for this zone:** `chip_doc_vantucky` — a former army medic running an implant clinic out of the neutral shed.

**Steps:**
- [ ] **Step 1: Read current file** — Read `content/zones/vantucky.yaml` in full.
- [ ] **Step 2: Add 9 new rooms** — Edit file, append rooms.
- [ ] **Step 3: Designate core rooms** — `core: true` on `vantucky_militia_core`, `vantucky_barracks`, plus existing high-danger rooms.
- [ ] **Step 4: Designate safe cluster** — 3 rooms: `vantucky_neutral_lot`, `vantucky_neutral_shed`, `vantucky_neutral_corner`.
- [ ] **Step 5: Assign edge room danger levels** — `danger_level: sketchy`.
- [ ] **Step 6: Audit NPC diversity and spawn density** — Dangerous: 2–3. Sketchy: 1–2. Safe: 0.
- [ ] **Step 7: Create chip_doc NPC file** — Create `content/npcs/chip_doc_vantucky.yaml`.
- [ ] **Step 8: Add chip_doc spawn** — Add to `vantucky_neutral_shed`.
- [ ] **Step 9: Commit**
  `git add content/zones/vantucky.yaml content/npcs/chip_doc_vantucky.yaml`
  `git commit -m "feat(zone-content-expansion): expand vantucky — 9 new rooms, safe cluster, chip_doc (REQ-ZCE-1,3,4,5,6,7,8,10,12-14)"`

---

## Task 17: Verify All 16 Zones — Final Checklist

After all zone tasks complete, perform a final audit across all 16 zone files.

**Steps:**
- [ ] **Step 1: Room count check**
  For each zone file, count entries matching `^  - id:`. Each MUST be ≥ 30.
- [ ] **Step 2: Safe cluster check**
  For each zone, confirm exactly 3–5 rooms have `danger_level: safe`.
- [ ] **Step 3: NPC density check**
  For each safe room, confirm 0 combat spawns. For each sketchy room, confirm 1–2 combat spawns. For each dangerous room, confirm 2–3 combat spawns. For each all_out_war room, confirm 3–4 combat spawns.
- [ ] **Step 4: chip_doc file check**
  Confirm all 16 `content/npcs/chip_doc_<zone_id>.yaml` files exist.
- [ ] **Step 5: chip_doc placement check**
  For each zone, confirm the chip_doc NPC spawn is in a room with `danger_level: safe` and that the room has ≤ 1 other non-combat NPC spawn.
- [ ] **Step 6: NPC diversity check**
  For each zone, count distinct NPC template IDs in spawn entries. Confirm count ≥ floor(room_count/10)+2.
- [ ] **Step 7: Commit verification summary**
  `git add -A` (scope: only YAML files in `content/zones/` and `content/npcs/`)
  `git commit -m "feat(zone-content-expansion): final verification pass — all 16 zones complete (REQ-ZCE-1 through REQ-ZCE-16)"`

---

## Implementation Notes

### No Go Code Changes Required

Per the design spec: "No changes to Go source code, proto definitions, or database schema are required." The `Room.DangerLevel` field (type `string`, yaml tag `danger_level,omitempty`) and `Room.core` concept are already present or handled through content conventions.

### The `core` Field

The spec references `core: true` in room YAML as a content-author designation. The existing `Room` struct in `internal/game/world/model.go` does not currently define a `Core bool` field. The content spec uses `core: true` as a convention for documentation and authoring intent — it does not drive runtime behavior in this feature. If the loader rejects unknown YAML keys, `core: true` must be added as a `Core bool \`yaml:"core,omitempty"\`` field to the `Room` struct with a TDD cycle.

**TDD gate for Core field (if needed):**

- [ ] **Conditional Step: Add `Core` field to Room struct**
  If loader validation rejects unknown YAML keys:
  - File to modify: `internal/game/world/model.go`
  - Add `Core bool \`yaml:"core,omitempty"\`` to the `Room` struct after `DangerLevel`.
  - File to modify/create: `internal/game/world/model_test.go` (or existing test file)
  - Add property-based test using `pgregory.net/rapid`:
    ```go
    // TestRoomCoreFieldRoundTrip verifies Core survives YAML marshal/unmarshal.
    func TestRoomCoreFieldRoundTrip(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) {
            core := rapid.Bool().Draw(t, "core")
            r := Room{ID: "test", Core: core}
            data, err := yaml.Marshal(r)
            if err != nil {
                t.Fatalf("marshal: %v", err)
            }
            var r2 Room
            if err := yaml.Unmarshal(data, &r2); err != nil {
                t.Fatalf("unmarshal: %v", err)
            }
            if r2.Core != core {
                t.Fatalf("Core mismatch: got %v, want %v", r2.Core, core)
            }
        })
    }
    ```
  - Run: `cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/world/... -run TestRoomCoreFieldRoundTrip`
  - Commit: `git add internal/game/world/model.go internal/game/world/model_test.go`
    `git commit -m "feat(zone-content-expansion): add Room.Core bool field for content authoring (REQ-ZCE-8)"`

### chip_doc NPC Type Dependency

The `chip_doc` NPC type is implemented by the `curse-removal` feature plan. The zone YAML spawn entries referencing `chip_doc_<zone>` templates will cause a load error if the `curse-removal` feature has not yet been implemented. Zone YAML authoring can proceed independently; integration testing requires both features to be implemented.

### Loader Validation

The `content/zones/` YAML files are loaded by `internal/game/world/loader.go`. Rooms with `danger_level` set are validated against the allowed values (`safe`, `sketchy`, `dangerous`, `all_out_war`) by the existing `EffectiveDangerLevel` function in `internal/game/danger/level.go`. No loader changes are needed for danger level support.

### Map Coordinates

Each new room MUST have unique `map_x`/`map_y` coordinates within its zone. Rooms placed at x ≥ 200 (e.g., `map_x: 202`) are treated as "off-map" by the renderer and do not appear on the automap. New rooms in this expansion MUST use coordinates in the main grid (not ≥ 200) so they appear on the player's automap.

### Go Module

Module path: `github.com/cory-johannsen/mud`

---

## File Summary

**Zone YAML files modified (16):**
- `content/zones/aloha.yaml`
- `content/zones/battleground.yaml`
- `content/zones/beaverton.yaml`
- `content/zones/downtown.yaml`
- `content/zones/felony_flats.yaml`
- `content/zones/hillsboro.yaml`
- `content/zones/lake_oswego.yaml`
- `content/zones/ne_portland.yaml`
- `content/zones/pdx_international.yaml`
- `content/zones/ross_island.yaml`
- `content/zones/rustbucket_ridge.yaml`
- `content/zones/sauvie_island.yaml`
- `content/zones/se_industrial.yaml`
- `content/zones/the_couve.yaml`
- `content/zones/troutdale.yaml`
- `content/zones/vantucky.yaml`

**chip_doc NPC YAML files created (16):**
- `content/npcs/chip_doc_aloha.yaml`
- `content/npcs/chip_doc_battleground.yaml`
- `content/npcs/chip_doc_beaverton.yaml`
- `content/npcs/chip_doc_downtown.yaml`
- `content/npcs/chip_doc_felony_flats.yaml`
- `content/npcs/chip_doc_hillsboro.yaml`
- `content/npcs/chip_doc_lake_oswego.yaml`
- `content/npcs/chip_doc_ne_portland.yaml`
- `content/npcs/chip_doc_pdx_international.yaml`
- `content/npcs/chip_doc_ross_island.yaml`
- `content/npcs/chip_doc_rustbucket_ridge.yaml`
- `content/npcs/chip_doc_sauvie_island.yaml`
- `content/npcs/chip_doc_se_industrial.yaml`
- `content/npcs/chip_doc_the_couve.yaml`
- `content/npcs/chip_doc_troutdale.yaml`
- `content/npcs/chip_doc_vantucky.yaml`

**Go source files (conditional — only if loader rejects unknown YAML keys):**
- `internal/game/world/model.go` — add `Core bool` field
- `internal/game/world/model_test.go` — property-based roundtrip test
