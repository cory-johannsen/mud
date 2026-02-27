# Stage 12 — Zone Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expand the game world from 2 zones to 16 zones across the Portland metro area with zone-specific NPCs, items, loot tables, and cross-zone connections.

**Architecture:** Pure content addition — no Go code changes. Each zone is a YAML file in `content/zones/`, each NPC template a YAML file in `content/npcs/`, each item a YAML file in `content/items/`. The existing world loader, NPC system, and loot system support all of this. Cross-zone exits work because the world manager uses a flat room index — an exit's `target` can reference any room ID in any zone.

**Tech Stack:** YAML content files, validated by existing Go loader (`go build ./...` and `go test ./...`).

**Validation command (run after every task):**
```bash
mise exec -- go build ./... 2>&1 && mise exec -- go test ./internal/game/world/... ./internal/game/npc/... ./internal/game/inventory/... -race -count=1 -timeout=60s 2>&1
```

**YAML templates to follow:**

Zone YAML structure:
```yaml
zone:
  id: zone_id
  name: "Zone Name"
  description: "Zone description."
  start_room: first_room_id
  rooms:
    - id: room_id
      title: "Room Title"
      description: |
        Multi-line room description with dystopian Portland flavor.
        At least 2-3 lines of atmospheric text.
      exits:
        - direction: north
          target: other_room_id
      spawns:                    # optional — rooms with NPCs
        - template: npc_template_id
          count: 2
          respawn_after: "3m"
      properties:                # optional
        lighting: dim
        atmosphere: rain
```

NPC template YAML structure:
```yaml
id: npc_id
name: NPC Display Name
description: One-sentence NPC description.
level: 1
max_hp: 18
ac: 14
perception: 5
ai_domain: ganger_combat
respawn_delay: "5m"
abilities:
  strength: 14
  dexterity: 12
  constitution: 14
  intelligence: 8
  wisdom: 10
  charisma: 8
loot:
  currency:
    min: 10
    max: 50
  items:
    - item: item_id
      chance: 0.5
      min_qty: 1
      max_qty: 3
```

Item YAML structure:
```yaml
id: item_id
name: Item Name
description: Short item description.
kind: junk       # weapon | explosive | consumable | junk
weight: 1.0
stackable: true
max_stack: 10
value: 5
```

**AI domains:** Only two exist (`ganger_combat`, `scavenger_patrol`). All new NPC templates MUST use one of these two. Creating new AI domains is out of scope for this stage.

**NPC stat scaling by level:**

| Level | HP Range | AC Range | Ability High | Ability Low | Currency Range |
|-------|----------|----------|-------------|-------------|----------------|
| 1 | 14-20 | 13-14 | 14 | 8 | 5-50 |
| 2 | 22-30 | 14-15 | 15 | 9 | 15-75 |
| 3 | 30-42 | 15-16 | 16 | 10 | 25-100 |
| 4 | 38-50 | 16-17 | 17 | 10 | 40-150 |
| 5 | 45-60 | 17-18 | 18 | 11 | 60-200 |

**Room ID convention:** `{zone_prefix}_{descriptive_name}` — e.g. `ne_alberta_street`, `flats_82nd_motel`. Room IDs MUST be globally unique across all zones.

---

## Task 1: Fix Rustbucket Ridge Connectivity

**Files:**
- Modify: `content/zones/rustbucket_ridge.yaml`

**What to do:**

Connect the 30 dead-end rooms into a navigable zone. The zone has 6 area clusters plus some standalone rooms. Create a connected topology:

1. **Main spine:** `grinders_row` ↔ `rotgut_alley` (exists). Add exits to connect area clusters off the spine.

2. **Area cluster connections (internal):** Each area cluster's rooms should connect to each other and to a hub room that connects back to the spine.

3. **Standalone rooms** (`wayne_dawgs_trailer`, `last_stand_lodge`, `wreckers_rest`, `salvage_hut`) should connect to the nearest area or spine room.

**Cluster layout:**

- `grinders_row` (spine south):
  - west → `wayne_dawgs_trailer` (dead end, back east → `grinders_row`)
  - east → `last_stand_lodge` (dead end, back west → `grinders_row`)
  - south → `the_rusty_oasis` (filth_court hub)

- Filth Court cluster (hub: `the_rusty_oasis`):
  - `the_rusty_oasis` → north: `grinders_row`, east: `roach_haven`, south: `junkers_dream`, west: `the_green_hell`
  - `roach_haven` → west: `the_rusty_oasis`, south: `the_bottle_shack`
  - `junkers_dream` → north: `the_rusty_oasis`
  - `the_green_hell` → east: `the_rusty_oasis`
  - `the_bottle_shack` → north: `roach_haven`, west: `rotgut_alley`

- `rotgut_alley` (spine north):
  - south → `grinders_row` (exists)
  - east → `the_bottle_shack`
  - west → `the_heap` (scrapheap_circle hub)
  - north → `the_graveyard` (boneyard_bend hub)
  - northeast → `ashen_hollow` (cinder_pit hub)

- Scrapheap Circle cluster (hub: `the_heap`):
  - `the_heap` → east: `rotgut_alley`, north: `the_tinkers_den`, south: `scrapshack_23`
  - `the_tinkers_den` → south: `the_heap`, east: `wreckers_rest`
  - `scrapshack_23` → north: `the_heap`, east: `salvage_hut`
  - `wreckers_rest` → west: `the_tinkers_den`
  - `salvage_hut` → west: `scrapshack_23`

- Rotgut Alley area cluster (hub: `the_still`):
  - `the_still` → north: `rotgut_alley`, east: `rotgut_shack`, south: `the_barrel_house`
  - `rotgut_shack` → west: `the_still`, south: `snakepit`
  - `the_barrel_house` → north: `the_still`, east: `the_keg_hole`
  - `snakepit` → north: `rotgut_shack`
  - `the_keg_hole` → west: `the_barrel_house`

  Wait — `rotgut_alley` already has exits north→nothing and south→grinders_row. The `the_still` etc. have area=rotgut_alley. Let me reorganize. `rotgut_alley` connects to the rotgut_alley area rooms:
  - `rotgut_alley`: add west: `the_still`, north: `the_graveyard`, east: `the_heap`, northeast: `ashen_hollow`
  - `the_still` → east: `rotgut_alley`, south: `rotgut_shack`, west: `the_barrel_house`
  - `rotgut_shack` → north: `the_still`, south: `snakepit`
  - `the_barrel_house` → east: `the_still`, south: `the_keg_hole`
  - `snakepit` → north: `rotgut_shack`
  - `the_keg_hole` → north: `the_barrel_house`

- Boneyard Bend cluster (hub: `the_graveyard`):
  - `the_graveyard` → south: `rotgut_alley`, east: `the_mausoleum`, north: `ghosts_rest`
  - `the_mausoleum` → west: `the_graveyard`, north: `the_forgotten_trailer`
  - `ghosts_rest` → south: `the_graveyard`, east: `bonepickers_roost`
  - `the_forgotten_trailer` → south: `the_mausoleum`
  - `bonepickers_roost` → west: `ghosts_rest`

- Cinder Pit cluster (hub: `ashen_hollow`):
  - `ashen_hollow` → southwest: `rotgut_alley`, east: `smokers_den`, north: `the_furnace`
  - `smokers_den` → west: `ashen_hollow`, north: `scorchside_camp`
  - `the_furnace` → south: `ashen_hollow`, east: `the_embers_edge`
  - `scorchside_camp` → south: `smokers_den`
  - `the_embers_edge` → west: `the_furnace`, north: `blade_house`

- Shiv Way cluster (hub: `blade_house`):
  - `blade_house` → south: `the_embers_edge`, east: `the_cutthroat`, north: `the_slashers_den`
  - `the_cutthroat` → west: `blade_house`, north: `blood_camp`
  - `the_slashers_den` → south: `blade_house`, east: `the_razor_nest`
  - `blood_camp` → south: `the_cutthroat`
  - `the_razor_nest` → west: `the_slashers_den`

**Also add NPC spawns to Rustbucket Ridge rooms** (using existing templates):
- `grinders_row`: ganger ×2, respawn 3m
- `rotgut_alley`: ganger ×1, scavenger ×1, respawn 3m
- `blade_house`: ganger ×2, respawn 2m
- `the_slashers_den`: lieutenant ×1, respawn 5m

**Step 1:** Rewrite `content/zones/rustbucket_ridge.yaml` with all exits and spawns as described above. Keep all existing room IDs, titles, descriptions, and properties unchanged. Only add exits and spawns.

**Step 2:** Run validation:
```bash
mise exec -- go build ./... 2>&1 && mise exec -- go test ./internal/game/world/... -race -count=1 -timeout=60s 2>&1
```

**Step 3:** Commit:
```bash
git add content/zones/rustbucket_ridge.yaml
git commit -m "fix(content): connect Rustbucket Ridge dead-end rooms and add NPC spawns"
```

---

## Task 2: Northeast Portland Zone

**Files:**
- Create: `content/zones/ne_portland.yaml`
- Create: `content/npcs/bike_courier.yaml`
- Create: `content/npcs/brew_warlord.yaml`
- Create: `content/npcs/alberta_drifter.yaml`
- Create: `content/items/bike_chain.yaml`
- Create: `content/items/craft_brew.yaml`
- Create: `content/items/fixie_wheel.yaml`

**Zone ID:** `ne_portland`
**Start room:** `ne_alberta_street`
**Theme:** Gentrification ruins, craft brewery fortresses, bike gang territory. Alberta Street arts district gone feral, Mississippi district brewery compounds, MLK Boulevard no-man's-land. Bike couriers are the messengers and scouts, brewery warlords control territory through intoxicants and fortified taprooms.

**Rooms (20+):** Create at least 20 rooms. Use room ID prefix `ne_`. Include areas like:
- Alberta Street arts district (galleries turned gang hideouts)
- Mississippi Avenue brewery row (fortified taprooms)
- MLK Boulevard (wide, dangerous, exposed corridor)
- Irvington ruins (old mansions, overgrown)
- Hollywood District (theater ruins, scavenger markets)
- Sullivan's Gulch (train tracks, hobo camps)
- Alameda Ridge (hilltop lookouts)
- Lloyd District (convention center ruins, parking garage fortresses)
- Broadway Bridge approach (cross-zone border room connecting to Downtown)
- I-205 on-ramp (cross-zone border room connecting to PDX International)
- Cully neighborhood (connects to Rustbucket Ridge)

**Cross-zone exits:**
- One border room connects west to Downtown's `burnside_crossing`
- One border room connects east to Rustbucket Ridge's `grinders_row`
- One border room connects north to The Couve (a room TBD — use `couve_interstate_bridge_south`)
- One border room connects east to PDX International (use `pdx_airport_way_west`)

**NPC Templates:**

`bike_courier.yaml` — Level 1, fast and evasive:
- HP: 14, AC: 14, Perception: 8
- High DEX (16), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 5-30 currency, bike_chain (40%), medkit (15%)

`brew_warlord.yaml` — Level 3, tough brewer boss:
- HP: 38, AC: 16, Perception: 7
- High STR (16), CON (16), low INT (8)
- ai_domain: ganger_combat
- Loot: 30-100 currency, craft_brew (60%), scrap_metal (30%), medkit (20%)

`alberta_drifter.yaml` — Level 2, mid-range scavenger:
- HP: 24, AC: 14, Perception: 9
- High WIS (15), DEX (14), low CHA (9)
- ai_domain: scavenger_patrol
- Loot: 10-60 currency, scrap_metal (50%), medkit (20%)

**Items:**

`bike_chain.yaml` — junk, weight 0.8, stackable, max_stack 5, value 8
`craft_brew.yaml` — consumable, weight 0.5, stackable, max_stack 3, value 15
`fixie_wheel.yaml` — junk, weight 2.0, not stackable, value 25

**Spawns:** Distribute NPCs across the zone. Bike couriers in street/corridor rooms, brew warlords in brewery rooms, drifters in residential/park areas. ~8-10 rooms with spawns.

**Step 1:** Create all NPC template YAML files listed above.
**Step 2:** Create all item YAML files listed above.
**Step 3:** Create `content/zones/ne_portland.yaml` with 20+ rooms, exits, spawns, and properties.
**Step 4:** Run validation.
**Step 5:** Commit:
```bash
git add content/zones/ne_portland.yaml content/npcs/bike_courier.yaml content/npcs/brew_warlord.yaml content/npcs/alberta_drifter.yaml content/items/bike_chain.yaml content/items/craft_brew.yaml content/items/fixie_wheel.yaml
git commit -m "feat(content): add Northeast Portland zone with NPCs and items"
```

---

## Task 3: Felony Flats Zone

**Files:**
- Create: `content/zones/felony_flats.yaml`
- Create: `content/npcs/motel_raider.yaml`
- Create: `content/npcs/strip_mall_scav.yaml`
- Create: `content/npcs/82nd_enforcer.yaml`
- Create: `content/items/motel_key.yaml`
- Create: `content/items/neon_tube.yaml`
- Create: `content/items/cheap_blade.yaml`

**Zone ID:** `felony_flats`
**Start room:** `flats_82nd_ave`
**Theme:** 82nd Avenue strip mall corridor, motel fortresses, survival hustle. The real Felony Flats reputation cranked to dystopian extremes. Strip malls converted to trading posts and gang turf, motels as fortified camps, parking lots as open-air markets.

**Rooms (20+):** Use room ID prefix `flats_`. Include areas like:
- 82nd Avenue corridor (main spine, dangerous)
- Motel row (fortified motel compounds)
- Strip mall district (converted shops)
- Foster Road (southern approach)
- Johnson Creek (overgrown wetlands, hidden camps)
- Powell Boulevard (wide exposed road, ambush territory)
- Lents Park (overgrown, scavenger camps)
- Jade District (food stalls, trading)
- Hawthorne approach (cross-zone border to Downtown — use target `courthouse_steps`)

**Cross-zone exits:**
- One border room connects northwest to Downtown's `courthouse_steps`
- One border room connects south to SE Industrial (use `sei_powell_terminus`)

**NPC Templates:**

`motel_raider.yaml` — Level 2, ambush fighter:
- HP: 26, AC: 15, Perception: 6
- High STR (15), DEX (14), low WIS (9)
- ai_domain: ganger_combat
- Loot: 15-60 currency, motel_key (30%), cheap_blade (15%), medkit (10%)

`strip_mall_scav.yaml` — Level 1, scavenger:
- HP: 16, AC: 13, Perception: 8
- High DEX (14), WIS (14), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 5-35 currency, scrap_metal (60%), neon_tube (25%)

`82nd_enforcer.yaml` — Level 3, heavy hitter:
- HP: 36, AC: 16, Perception: 6
- High STR (16), CON (16), low DEX (10)
- ai_domain: ganger_combat
- Loot: 25-100 currency, cheap_blade (40%), medkit (25%), scrap_metal (30%)

**Items:**

`motel_key.yaml` — junk, weight 0.1, stackable, max_stack 10, value 3
`neon_tube.yaml` — junk, weight 0.8, stackable, max_stack 5, value 10
`cheap_blade.yaml` — weapon, weight 1.0, not stackable, value 20

**Step 1-5:** Same pattern as Task 2. Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Felony Flats zone with NPCs and items"
```

---

## Task 4: Lake Oswego Nation Zone

**Files:**
- Create: `content/zones/lake_oswego.yaml`
- Create: `content/npcs/oswego_guard.yaml`
- Create: `content/npcs/country_club_sniper.yaml`
- Create: `content/npcs/hoa_enforcer.yaml`
- Create: `content/items/silver_cufflink.yaml`
- Create: `content/items/golf_club.yaml`
- Create: `content/items/hoa_citation.yaml`

**Zone ID:** `lake_oswego`
**Start room:** `lo_checkpoint_north`
**Theme:** Gated wealth compound. The Lake Oswego residents declared independence early and fortified their borders. Private militia patrols manicured-but-crumbling neighborhoods. Country clubs serve as military HQs. HOA rules are enforced at gunpoint. The rich survived — barely — and they're paranoid about everyone outside the walls.

**Rooms (20+):** Use room ID prefix `lo_`. Include areas like:
- Northern checkpoint (border gate, cross-zone entry from Downtown)
- Lake View Boulevard (patrolled avenue)
- Country Club compound (fortified golf course)
- Lakewood Bay (waterfront mansions)
- HOA Headquarters (administrative fortress)
- The Commons (guarded shopping district)
- Iron Mountain (hilltop sniper nests)
- Tryon Creek (wilderness buffer zone)
- Macadam approach (cross-zone border to Downtown — use target `director_park`)

**Cross-zone exits:**
- One border room connects north to Downtown's `director_park`

**NPC Templates:**

`oswego_guard.yaml` — Level 2, disciplined patrol:
- HP: 28, AC: 15, Perception: 8
- High CON (15), WIS (14), low CHA (9)
- ai_domain: scavenger_patrol
- Loot: 20-75 currency, silver_cufflink (20%), medkit (25%)

`country_club_sniper.yaml` — Level 4, high-perception ranged:
- HP: 40, AC: 17, Perception: 10
- High DEX (17), WIS (15), low STR (10)
- ai_domain: ganger_combat
- Loot: 40-150 currency, silver_cufflink (30%), medkit (20%)

`hoa_enforcer.yaml` — Level 3, bureaucratic brute:
- HP: 35, AC: 16, Perception: 6
- High STR (16), CON (16), low INT (9)
- ai_domain: ganger_combat
- Loot: 30-100 currency, hoa_citation (50%), golf_club (15%), medkit (15%)

**Items:**

`silver_cufflink.yaml` — junk, weight 0.1, stackable, max_stack 10, value 30
`golf_club.yaml` — weapon, weight 2.0, not stackable, value 35
`hoa_citation.yaml` — junk, weight 0.05, stackable, max_stack 20, value 1

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Lake Oswego Nation zone with NPCs and items"
```

---

## Task 5: Free State of Beaverton Zone

**Files:**
- Create: `content/zones/beaverton.yaml`
- Create: `content/npcs/beaverton_minuteman.yaml`
- Create: `content/npcs/mall_cop_elite.yaml`
- Create: `content/npcs/cedar_hills_patrol.yaml`
- Create: `content/items/mall_badge.yaml`
- Create: `content/items/suburban_machete.yaml`
- Create: `content/items/parking_pass.yaml`

**Zone ID:** `beaverton`
**Start room:** `beav_canyon_road_east`
**Theme:** Suburban fortress-state built around strip malls and big-box stores. The Beaverton Town Center is the capitol. Mall cops evolved into a legitimate military force. Cedar Hills Crossing is a fortified trading hub. The suburbanites turned their HOAs into a functioning government — complete with parking enforcement that now carries lethal authority.

**Rooms (20+):** Use room ID prefix `beav_`. Include areas like:
- Canyon Road approach (eastern border, connects to Downtown)
- Beaverton Town Center (capitol building / former mall)
- Cedar Hills Crossing (trading hub)
- Murray Boulevard corridor (patrol route)
- Tualatin Hills Nature Park (overgrown, dangerous)
- Sunset Transit Center (fortified transit hub)
- TV Highway east (connects to Aloha Neutral Zone)
- Progress Ridge (southern lookout)
- Walker Road (residential patrol zone)

**Cross-zone exits:**
- One border room connects east to Downtown's `transit_mall` (via US-26)
- One border room connects west to Aloha Neutral Zone (use `aloha_tv_highway_east`)

**NPC Templates:**

`beaverton_minuteman.yaml` — Level 2, citizen militia:
- HP: 24, AC: 15, Perception: 7
- High CON (15), STR (14), low CHA (9)
- ai_domain: ganger_combat
- Loot: 15-65 currency, suburban_machete (20%), scrap_metal (40%), medkit (15%)

`mall_cop_elite.yaml` — Level 3, armored enforcer:
- HP: 34, AC: 16, Perception: 8
- High STR (16), DEX (14), low WIS (10)
- ai_domain: ganger_combat
- Loot: 25-90 currency, mall_badge (40%), medkit (25%), parking_pass (30%)

`cedar_hills_patrol.yaml` — Level 1, light patrol:
- HP: 18, AC: 14, Perception: 9
- High DEX (14), WIS (14), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 10-40 currency, parking_pass (50%), scrap_metal (30%)

**Items:**

`mall_badge.yaml` — junk, weight 0.1, stackable, max_stack 5, value 12
`suburban_machete.yaml` — weapon, weight 1.5, not stackable, value 30
`parking_pass.yaml` — junk, weight 0.05, stackable, max_stack 20, value 2

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Free State of Beaverton zone with NPCs and items"
```

---

## Task 6: Aloha Neutral Zone

**Files:**
- Create: `content/zones/aloha.yaml`
- Create: `content/npcs/aloha_broker.yaml`
- Create: `content/npcs/neutral_zone_sentry.yaml`
- Create: `content/npcs/smuggler.yaml`
- Create: `content/items/trade_token.yaml`
- Create: `content/items/contraband_pack.yaml`
- Create: `content/items/sentry_whistle.yaml`

**Zone ID:** `aloha`
**Start room:** `aloha_tv_highway_east`
**Theme:** Demilitarized trading post between Beaverton and Hillsboro. Neutral ground enforced by a sentry corps that answers to no faction. Smugglers, brokers, and refugees pass through. The old strip malls serve as bazaars. Trust is currency here — violate the peace and the sentries put you down.

**Rooms (20+):** Use room ID prefix `aloha_`. Include areas like:
- TV Highway east (border with Beaverton)
- TV Highway west (border with Hillsboro)
- The Bazaar (open-air market in old parking lots)
- Broker's Row (shipping containers converted to offices)
- Sentry Post Alpha/Bravo (guard stations)
- The Caravansary (travelers' rest stop)
- 185th Street crossing (north-south corridor)
- Aloha Park (overgrown, neutral meeting ground)
- Refugee Camp (tent city)
- The Warehouse District (smuggler hideouts)

**Cross-zone exits:**
- One border room connects east to Beaverton (use `beav_tv_highway_west`)
- One border room connects west to Kingdom of Hillsboro (use `hills_tv_highway_east`)

**NPC Templates:**

`aloha_broker.yaml` — Level 2, charismatic trader:
- HP: 22, AC: 14, Perception: 9
- High CHA (15), INT (14), low STR (9)
- ai_domain: scavenger_patrol
- Loot: 20-80 currency, trade_token (60%), medkit (15%)

`neutral_zone_sentry.yaml` — Level 3, disciplined guard:
- HP: 32, AC: 16, Perception: 9
- High DEX (15), WIS (15), low CHA (10)
- ai_domain: ganger_combat
- Loot: 20-70 currency, sentry_whistle (30%), medkit (20%)

`smuggler.yaml` — Level 2, sneaky dealer:
- HP: 20, AC: 14, Perception: 7
- High DEX (15), CHA (14), low WIS (9)
- ai_domain: scavenger_patrol
- Loot: 25-90 currency, contraband_pack (35%), trade_token (25%), scrap_metal (40%)

**Items:**

`trade_token.yaml` — junk, weight 0.05, stackable, max_stack 20, value 8
`contraband_pack.yaml` — junk, weight 2.0, not stackable, value 40
`sentry_whistle.yaml` — junk, weight 0.1, stackable, max_stack 3, value 5

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Aloha Neutral Zone with NPCs and items"
```

---

## Task 7: Kingdom of Hillsboro Zone

**Files:**
- Create: `content/zones/hillsboro.yaml`
- Create: `content/npcs/hillsboro_knight.yaml`
- Create: `content/npcs/silicon_serf.yaml`
- Create: `content/npcs/intel_guard.yaml`
- Create: `content/items/circuit_board.yaml`
- Create: `content/items/server_rack_panel.yaml`
- Create: `content/items/knight_tabard.yaml`

**Zone ID:** `hillsboro`
**Start room:** `hills_tv_highway_east`
**Theme:** Tech campus feudalism. The Intel and other tech campuses became walled fiefdoms. Engineers who kept the servers running became lords. Workers who maintain infrastructure are serfs bound to their campus. Knights in improvised plate armor (server rack panels) patrol the borders. The Kingdom runs on electricity — they control the last working solar farms and generator banks in the metro area.

**Rooms (20+):** Use room ID prefix `hills_`. Include areas like:
- TV Highway east (border with Aloha)
- Campus Gate (fortified tech campus entrance)
- The Server Keep (data center fortress, seat of power)
- Solar Farm (power generation, heavily guarded)
- Serf Quarters (worker housing)
- Knight Barracks (military compound)
- The Foundry (electronics manufacturing)
- Orenco Station (old transit hub, now trade post)
- Rock Creek Trail (wilderness patrol route)
- Tanasbourne Ruins (old shopping district)
- Cornell Road (northern patrol route)

**Cross-zone exits:**
- One border room connects east to Aloha Neutral Zone (use `aloha_tv_highway_west`)

**NPC Templates:**

`hillsboro_knight.yaml` — Level 4, armored tech-knight:
- HP: 45, AC: 17, Perception: 7
- High STR (17), CON (16), low DEX (10)
- ai_domain: ganger_combat
- Loot: 40-150 currency, knight_tabard (20%), server_rack_panel (15%), medkit (20%)

`silicon_serf.yaml` — Level 1, low-threat worker:
- HP: 14, AC: 13, Perception: 6
- High INT (14), DEX (12), low STR (8)
- ai_domain: scavenger_patrol
- Loot: 3-15 currency, circuit_board (50%), scrap_metal (30%)

`intel_guard.yaml` — Level 3, campus security:
- HP: 34, AC: 16, Perception: 8
- High DEX (15), CON (15), low CHA (9)
- ai_domain: ganger_combat
- Loot: 25-100 currency, circuit_board (35%), medkit (20%)

**Items:**

`circuit_board.yaml` — junk, weight 0.3, stackable, max_stack 10, value 15
`server_rack_panel.yaml` — junk, weight 3.0, not stackable, value 45
`knight_tabard.yaml` — junk, weight 0.5, not stackable, value 35

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Kingdom of Hillsboro zone with NPCs and items"
```

---

## Task 8: The Couve Zone

**Files:**
- Create: `content/zones/the_couve.yaml`
- Create: `content/npcs/couve_militia.yaml`
- Create: `content/npcs/jantzen_beach_pirate.yaml`
- Create: `content/npcs/mill_plain_thug.yaml`
- Create: `content/items/canadian_bacon.yaml`
- Create: `content/items/pirate_flag.yaml`
- Create: `content/items/militia_patch.yaml`

**Zone ID:** `the_couve`
**Start room:** `couve_interstate_bridge_south`
**Theme:** Vancouver, WA — strip mall empire across the river. Chain restaurant warlords who carved out territories around their franchises. The Interstate Bridge is the lifeline connecting The Couve to Portland. Jantzen Beach was once a shopping center — now it's a pirate haven controlling river traffic. Mill Plain Boulevard is the main artery of Couve civilization.

**Rooms (20+):** Use room ID prefix `couve_`. Include areas like:
- Interstate Bridge south approach (border with NE Portland)
- Jantzen Beach (pirate haven on the river)
- Mill Plain Boulevard (main commercial corridor)
- Vancouver Mall (warlord stronghold)
- Officers Row (historic houses, now militia HQ)
- Fort Vancouver (actual fort, now military base)
- Esther Short Park (public square, trading)
- Columbia River waterfront (docks, smuggling)
- Fourth Plain corridor (dangerous)
- East border (connects to Vantucky)
- I-5 north (connects to Battleground)

**Cross-zone exits:**
- One border room connects south to NE Portland (use `ne_broadway_bridge`)
- One border room connects east to Vantucky (use `vantucky_fourth_plain_west`)
- One border room connects north to Battleground Socialist Collective (use `battle_i5_south`)

**NPC Templates:**

`couve_militia.yaml` — Level 2, organized fighter:
- HP: 26, AC: 15, Perception: 7
- High STR (15), CON (14), low INT (9)
- ai_domain: ganger_combat
- Loot: 15-65 currency, militia_patch (35%), scrap_metal (40%), medkit (15%)

`jantzen_beach_pirate.yaml` — Level 3, river raider:
- HP: 32, AC: 15, Perception: 8
- High DEX (16), CHA (14), low WIS (9)
- ai_domain: ganger_combat
- Loot: 30-100 currency, pirate_flag (15%), canadian_bacon (40%), medkit (20%)

`mill_plain_thug.yaml` — Level 1, street tough:
- HP: 18, AC: 13, Perception: 5
- High STR (14), CON (14), low INT (8)
- ai_domain: ganger_combat
- Loot: 5-40 currency, scrap_metal (50%), medkit (10%)

**Items:**

`canadian_bacon.yaml` — consumable, weight 0.3, stackable, max_stack 5, value 10
`pirate_flag.yaml` — junk, weight 0.5, not stackable, value 20
`militia_patch.yaml` — junk, weight 0.05, stackable, max_stack 10, value 5

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add The Couve zone with NPCs and items"
```

---

## Task 9: Vantucky Zone

**Files:**
- Create: `content/zones/vantucky.yaml`
- Create: `content/npcs/vantucky_militiaman.yaml`
- Create: `content/npcs/compound_guard.yaml`
- Create: `content/npcs/highway_bandit.yaml`
- Create: `content/items/ammo_casing.yaml`
- Create: `content/items/camo_netting.yaml`
- Create: `content/items/truck_part.yaml`

**Zone ID:** `vantucky`
**Start room:** `vantucky_fourth_plain_west`
**Theme:** East Vancouver — rural-urban fringe, gun culture, fortress compounds. The further east you go, the more fortified and isolated the compounds become. Highway bandits control I-84. Militia compounds dot the landscape. Pickup trucks are both transportation and weapons platforms. The people here chose isolation and firepower over cooperation.

**Rooms (20+):** Use room ID prefix `vantucky_`. Include areas like:
- Fourth Plain west (border with The Couve)
- Andresen Road (commercial strip)
- Orchards district (suburban compounds)
- 164th Avenue corridor (militia territory)
- I-84 overpass (bandit ambush point)
- Camas border (eastern edge)
- Fisher's Landing (river access)
- Compound Alpha/Bravo/Charlie (fortified homesteads)
- The Gun Market (open-air arms bazaar)
- Burnt Bridge Creek (wilderness, hidden camps)
- I-84 east (border connecting to Troutdale)

**Cross-zone exits:**
- One border room connects west to The Couve (use `couve_fourth_plain_east`)
- One border room connects east to Troutdale (use `trout_i84_west`)

**NPC Templates:**

`vantucky_militiaman.yaml` — Level 3, well-armed:
- HP: 34, AC: 16, Perception: 7
- High STR (16), CON (15), low CHA (9)
- ai_domain: ganger_combat
- Loot: 20-90 currency, ammo_casing (60%), camo_netting (15%), medkit (20%)

`compound_guard.yaml` — Level 4, fortress defender:
- HP: 44, AC: 17, Perception: 8
- High CON (17), STR (16), low DEX (10)
- ai_domain: ganger_combat
- Loot: 35-130 currency, ammo_casing (50%), truck_part (20%), medkit (25%)

`highway_bandit.yaml` — Level 2, ambush specialist:
- HP: 22, AC: 14, Perception: 9
- High DEX (15), WIS (14), low CON (10)
- ai_domain: scavenger_patrol
- Loot: 15-70 currency, scrap_metal (45%), ammo_casing (35%)

**Items:**

`ammo_casing.yaml` — junk, weight 0.1, stackable, max_stack 20, value 4
`camo_netting.yaml` — junk, weight 1.5, not stackable, value 25
`truck_part.yaml` — junk, weight 3.0, not stackable, value 40

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Vantucky zone with NPCs and items"
```

---

## Task 10: Battleground Socialist Collective Zone

**Files:**
- Create: `content/zones/battleground.yaml`
- Create: `content/npcs/collective_guardian.yaml`
- Create: `content/npcs/commissar.yaml`
- Create: `content/npcs/field_worker.yaml`
- Create: `content/items/commune_ration.yaml`
- Create: `content/items/red_armband.yaml`
- Create: `content/items/harvest_sickle.yaml`

**Zone ID:** `battleground`
**Start room:** `battle_i5_south`
**Theme:** A commune society north of Vancouver. Collective farms, shared resources, ideological enforcers. The Commissars maintain order through rhetoric and force. Field workers tend the crops that feed the collective. Guardians patrol the perimeter. It's a functioning socialist micro-state — whether it's utopia or dystopia depends on who you ask and whether you've tried to leave.

**Rooms (20+):** Use room ID prefix `battle_`. Include areas like:
- I-5 south approach (border with The Couve)
- The Gate (heavily guarded entrance)
- Main Street collective (town center)
- The Assembly Hall (political center, speeches)
- Collective Farm east/west (agricultural fields)
- The Granary (food storage, heavily guarded)
- Worker Barracks (communal housing)
- The Commissariat (administrative offices)
- Re-education Center (ominous building)
- The Commons (shared dining, socializing)
- North Fields (open farmland, edge of territory)
- The Watchtower (northern lookout)
- Memorial Grove (tribute to founders)

**Cross-zone exits:**
- One border room connects south to The Couve (use `couve_i5_north`)

**NPC Templates:**

`collective_guardian.yaml` — Level 3, ideologically motivated:
- HP: 34, AC: 16, Perception: 8
- High STR (16), WIS (14), low CHA (10)
- ai_domain: ganger_combat
- Loot: 15-60 currency, red_armband (30%), medkit (25%), scrap_metal (20%)

`commissar.yaml` — Level 4, political officer:
- HP: 42, AC: 16, Perception: 9
- High CHA (17), INT (15), low STR (10)
- ai_domain: ganger_combat
- Loot: 30-120 currency, red_armband (50%), commune_ration (40%), medkit (20%)

`field_worker.yaml` — Level 1, reluctant combatant:
- HP: 16, AC: 13, Perception: 6
- High CON (14), STR (12), low INT (9)
- ai_domain: scavenger_patrol
- Loot: 3-20 currency, commune_ration (50%), harvest_sickle (10%)

**Items:**

`commune_ration.yaml` — consumable, weight 0.3, stackable, max_stack 5, value 8
`red_armband.yaml` — junk, weight 0.05, stackable, max_stack 5, value 6
`harvest_sickle.yaml` — weapon, weight 1.5, not stackable, value 15

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Battleground Socialist Collective zone with NPCs and items"
```

---

## Task 11: Southeast Industrial Zone

**Files:**
- Create: `content/zones/se_industrial.yaml`
- Create: `content/npcs/dock_worker.yaml`
- Create: `content/npcs/warehouse_guard.yaml`
- Create: `content/npcs/industrial_scav.yaml`
- Create: `content/items/shipping_manifest.yaml`
- Create: `content/items/steel_pipe.yaml`
- Create: `content/items/dock_hook.yaml`

**Zone ID:** `se_industrial`
**Start room:** `sei_powell_terminus`
**Theme:** Warehouse district along the Willamette. River docks, industrial salvage yards, massive abandoned warehouses. Dock workers control the waterfront and river trade. Warehouse guards protect valuable salvage caches. Industrial scavengers pick through the ruins for anything of value. The constant drip of rain on corrugated metal is the soundtrack here.

**Rooms (20+):** Use room ID prefix `sei_`. Include areas like:
- Powell terminus (border with Felony Flats)
- Dock Row (waterfront loading docks)
- Warehouse District (massive storage buildings)
- The Crane Yard (construction equipment graveyard)
- McLoughlin Boulevard (main road south)
- Milwaukie Junction (rail yard)
- The Dry Dock (ship repair, now fortress)
- Industrial Way (factory corridor)
- Holgate Boulevard (east-west connector)
- River Road south (leads toward Ross Island bridge)
- Ross Island Bridge approach (border with Ross Island)

**Cross-zone exits:**
- One border room connects north to Felony Flats (use `flats_foster_south`)
- One border room connects south to Ross Island (use `ross_bridge_east`)

**NPC Templates:**

`dock_worker.yaml` — Level 2, strong laborer:
- HP: 26, AC: 14, Perception: 6
- High STR (15), CON (15), low INT (9)
- ai_domain: ganger_combat
- Loot: 10-50 currency, dock_hook (15%), scrap_metal (50%), medkit (10%)

`warehouse_guard.yaml` — Level 3, armed protector:
- HP: 35, AC: 16, Perception: 8
- High CON (16), STR (15), low CHA (9)
- ai_domain: ganger_combat
- Loot: 20-80 currency, steel_pipe (25%), shipping_manifest (30%), medkit (20%)

`industrial_scav.yaml` — Level 1, nimble scavenger:
- HP: 15, AC: 14, Perception: 9
- High DEX (14), WIS (14), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 5-30 currency, scrap_metal (65%), shipping_manifest (20%)

**Items:**

`shipping_manifest.yaml` — junk, weight 0.1, stackable, max_stack 10, value 7
`steel_pipe.yaml` — weapon, weight 2.0, not stackable, value 15
`dock_hook.yaml` — weapon, weight 1.5, not stackable, value 20

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Southeast Industrial zone with NPCs and items"
```

---

## Task 12: Sauvie Island Zone

**Files:**
- Create: `content/zones/sauvie_island.yaml`
- Create: `content/npcs/island_farmer.yaml`
- Create: `content/npcs/river_pirate.yaml`
- Create: `content/npcs/harvest_guard.yaml`
- Create: `content/items/fresh_produce.yaml`
- Create: `content/items/fishing_net.yaml`
- Create: `content/items/river_map.yaml`

**Zone ID:** `sauvie_island`
**Start room:** `sauvie_bridge_south`
**Theme:** Agrarian holdout on the island in the Columbia River. Farm compounds grow real food — a luxury in the post-collapse world. River pirates raid from the waterways. The bridge is the only land connection and is heavily guarded. The island's interior is fertile farmland surrounded by dangerous riverbanks.

**Rooms (20+):** Use room ID prefix `sauvie_`. Include areas like:
- Bridge south approach (border with Downtown/NW Portland)
- The Tollgate (bridge checkpoint)
- Sauvie Road (main island road)
- Pumpkin Patch (old u-pick farms, now productive agriculture)
- Collins Beach (river pirate cove)
- Oak Island (interior wetlands)
- Dairy Farm (livestock compound)
- The Granary (food storage)
- Sturgeon Lake shore (fishing camps)
- The Orchard (fruit trees, guarded)
- North tip (river overlook)
- Gilbert River (waterway, boat access)

**Cross-zone exits:**
- One border room connects south to Downtown (use `waterfront_trail`)

**NPC Templates:**

`island_farmer.yaml` — Level 1, tough agrarian:
- HP: 18, AC: 13, Perception: 7
- High CON (14), WIS (14), low CHA (9)
- ai_domain: scavenger_patrol
- Loot: 5-25 currency, fresh_produce (60%), scrap_metal (20%)

`river_pirate.yaml` — Level 3, dangerous raider:
- HP: 32, AC: 15, Perception: 8
- High DEX (16), STR (14), low WIS (9)
- ai_domain: ganger_combat
- Loot: 25-100 currency, fishing_net (20%), river_map (25%), medkit (20%)

`harvest_guard.yaml` — Level 2, farm defender:
- HP: 26, AC: 15, Perception: 7
- High STR (15), CON (14), low DEX (10)
- ai_domain: ganger_combat
- Loot: 10-50 currency, fresh_produce (35%), medkit (15%)

**Items:**

`fresh_produce.yaml` — consumable, weight 0.5, stackable, max_stack 5, value 12
`fishing_net.yaml` — junk, weight 1.5, not stackable, value 18
`river_map.yaml` — junk, weight 0.1, stackable, max_stack 3, value 20

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Sauvie Island zone with NPCs and items"
```

---

## Task 13: Ross Island Zone

**Files:**
- Create: `content/zones/ross_island.yaml`
- Create: `content/npcs/bridge_troll.yaml`
- Create: `content/npcs/gravel_pit_boss.yaml`
- Create: `content/npcs/island_hermit.yaml`
- Create: `content/items/bridge_toll_token.yaml`
- Create: `content/items/gravel_chunk.yaml`
- Create: `content/items/hermit_charm.yaml`

**Zone ID:** `ross_island`
**Start room:** `ross_bridge_east`
**Theme:** A small, isolated island fortress in the Willamette. The old gravel pit operation left behind massive excavated areas now used as camps and fortifications. Bridge trolls literally control the Ross Island Bridge, demanding tolls. A gravel pit boss runs a mining/salvage operation. Hermits hide in the overgrown interior, suspicious of everyone.

**Rooms (20+):** Use room ID prefix `ross_`. Include areas like:
- Bridge east approach (border with SE Industrial)
- Bridge west approach (dead end or loop)
- The Toll Gate (bridge troll checkpoint)
- Gravel Pit (massive excavation, now settlement)
- The Quarry Floor (bottom of pit, mining)
- Lagoon (flooded pit, fishing)
- The Hermitage (hidden forest camp)
- Hardtack Island (smaller adjacent island)
- South Shore (river access)
- North Shore (view of downtown)
- The Overgrowth (dense vegetation interior)
- Rubble Ridge (elevated gravel pile lookout)

**Cross-zone exits:**
- One border room connects east to SE Industrial (use `sei_ross_bridge_approach`)

**NPC Templates:**

`bridge_troll.yaml` — Level 3, toll enforcer:
- HP: 36, AC: 16, Perception: 7
- High STR (16), CON (16), low INT (8)
- ai_domain: ganger_combat
- Loot: 30-100 currency, bridge_toll_token (50%), scrap_metal (30%), medkit (15%)

`gravel_pit_boss.yaml` — Level 4, pit overlord:
- HP: 48, AC: 17, Perception: 8
- High STR (17), CON (16), low DEX (10)
- ai_domain: ganger_combat
- Loot: 40-150 currency, gravel_chunk (40%), scrap_metal (35%), medkit (20%)

`island_hermit.yaml` — Level 2, reclusive survivor:
- HP: 20, AC: 14, Perception: 10
- High WIS (15), DEX (14), low CHA (8)
- ai_domain: scavenger_patrol
- Loot: 5-30 currency, hermit_charm (30%), fresh_produce (25%)

**Items:**

`bridge_toll_token.yaml` — junk, weight 0.1, stackable, max_stack 20, value 5
`gravel_chunk.yaml` — junk, weight 2.0, stackable, max_stack 5, value 3
`hermit_charm.yaml` — junk, weight 0.1, not stackable, value 15

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Ross Island zone with NPCs and items"
```

---

## Task 14: Troutdale Zone

**Files:**
- Create: `content/zones/troutdale.yaml`
- Create: `content/npcs/gorge_runner.yaml`
- Create: `content/npcs/outlet_scavenger.yaml`
- Create: `content/npcs/wind_walker.yaml`
- Create: `content/items/gorge_moss.yaml`
- Create: `content/items/outlet_tag.yaml`
- Create: `content/items/wind_goggles.yaml`

**Zone ID:** `troutdale`
**Start room:** `trout_i84_west`
**Theme:** Gateway to the Columbia River Gorge. The outlet malls are ruins picked clean by scavengers. The wind through the gorge is relentless and dangerous. Gorge runners are fast messengers and scouts who navigate the treacherous terrain. Wind walkers are mystical survivalists who've adapted to the extreme conditions. The gateway position makes Troutdale a crossroads between Portland metro and the eastern wilds.

**Rooms (20+):** Use room ID prefix `trout_`. Include areas like:
- I-84 west approach (border with Vantucky)
- I-84 east approach (border with PDX International)
- Outlet Mall ruins (massive shopping complex, scavenger territory)
- Stark Street corridor (main town road)
- Sandy River crossing (dangerous river ford)
- Lewis and Clark State Park (overgrown campground)
- Crown Point overlook (gorge vista, wind-blasted)
- Multnomah Falls approach (pilgrimage site)
- The Wind Tunnel (narrow gorge passage)
- Edgefield (old hotel/brewery compound, now settlement)
- Wood Village remnants (suburban ruins)

**Cross-zone exits:**
- One border room connects west to Vantucky (use `vantucky_i84_east`)
- One border room connects west to PDX International (use `pdx_i84_east`)

**NPC Templates:**

`gorge_runner.yaml` — Level 2, fast scout:
- HP: 22, AC: 15, Perception: 10
- High DEX (15), WIS (15), low STR (9)
- ai_domain: scavenger_patrol
- Loot: 10-50 currency, gorge_moss (40%), medkit (20%)

`outlet_scavenger.yaml` — Level 1, junk collector:
- HP: 16, AC: 13, Perception: 7
- High DEX (14), INT (12), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 5-30 currency, outlet_tag (50%), scrap_metal (40%)

`wind_walker.yaml` — Level 4, gorge mystic:
- HP: 40, AC: 16, Perception: 11
- High WIS (17), DEX (16), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 30-100 currency, wind_goggles (15%), gorge_moss (50%), medkit (25%)

**Items:**

`gorge_moss.yaml` — consumable, weight 0.2, stackable, max_stack 10, value 8
`outlet_tag.yaml` — junk, weight 0.05, stackable, max_stack 20, value 2
`wind_goggles.yaml` — junk, weight 0.3, not stackable, value 35

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add Troutdale zone with NPCs and items"
```

---

## Task 15: PDX International Zone

**Files:**
- Create: `content/zones/pdx_international.yaml`
- Create: `content/npcs/cargo_cultist.yaml`
- Create: `content/npcs/terminal_squatter.yaml`
- Create: `content/npcs/tarmac_raider.yaml`
- Create: `content/items/boarding_pass.yaml`
- Create: `content/items/airline_meal.yaml`
- Create: `content/items/jet_fuel_vial.yaml`

**Zone ID:** `pdx_international`
**Start room:** `pdx_airport_way_west`
**Theme:** The abandoned Portland International Airport. A cargo cult has formed around the planes, believing the aircraft will fly again someday. Terminal squatters occupy the vast indoor spaces. Tarmac raiders strip the planes for parts. The famous PDX carpet is still there, somehow. The airport's size makes it a world unto itself — terminals, concourses, hangars, and the vast tarmac stretching into the distance.

**Rooms (20+):** Use room ID prefix `pdx_`. Include areas like:
- Airport Way west (border with NE Portland)
- I-84 east approach (border with Troutdale)
- Terminal entrance (main building approach)
- Ticketing Hall (massive open space)
- The Carpet (iconic patterned floor, now a shrine)
- Concourse A/B/C/D/E (long terminal wings)
- The Food Court (scavenger market)
- Baggage Claim (settlement area)
- The Tarmac (vast open asphalt)
- Hangar Row (aircraft maintenance, raider territory)
- Control Tower (highest point, lookout)
- Cargo Terminal (cult headquarters)
- Parking Garage (multi-level fortress)
- MAX Station (abandoned rail terminal)

**Cross-zone exits:**
- One border room connects west to NE Portland (use `ne_i205_onramp`)
- One border room connects east to Troutdale (use `trout_i84_west_junction`)

Note: Troutdale has two western approaches — one from Vantucky via I-84, one from PDX. Use `trout_i84_west` for Vantucky and `trout_i84_west_junction` for PDX (a different room in Troutdale).

**NPC Templates:**

`cargo_cultist.yaml` — Level 3, zealous believer:
- HP: 30, AC: 15, Perception: 7
- High WIS (16), CHA (14), low DEX (10)
- ai_domain: ganger_combat
- Loot: 15-70 currency, boarding_pass (50%), airline_meal (30%), medkit (15%)

`terminal_squatter.yaml` — Level 1, survivalist:
- HP: 16, AC: 13, Perception: 7
- High CON (14), WIS (12), low STR (10)
- ai_domain: scavenger_patrol
- Loot: 3-25 currency, scrap_metal (50%), airline_meal (25%)

`tarmac_raider.yaml` — Level 4, heavily armed:
- HP: 44, AC: 17, Perception: 8
- High STR (17), DEX (15), low WIS (10)
- ai_domain: ganger_combat
- Loot: 35-140 currency, jet_fuel_vial (25%), scrap_metal (40%), medkit (20%)

**Items:**

`boarding_pass.yaml` — junk, weight 0.05, stackable, max_stack 20, value 3
`airline_meal.yaml` — consumable, weight 0.4, stackable, max_stack 5, value 6
`jet_fuel_vial.yaml` — junk, weight 0.5, stackable, max_stack 5, value 30

**Step 1-5:** Create NPCs, items, zone YAML, validate, commit.
```bash
git commit -m "feat(content): add PDX International zone with NPCs and items"
```

---

## Task 16: Cross-Zone Exit Wiring + Downtown Border Rooms

**Files:**
- Modify: `content/zones/downtown.yaml` (add border exits to new zones)
- Verify: All cross-zone exit targets exist in their respective zone YAML files

**What to do:**

Add exits to existing Downtown rooms that connect to the new zones:

1. In `burnside_crossing`, add exit: `north` → `ne_broadway_bridge` (NE Portland)
2. In `courthouse_steps`, add exit: `southeast` → `flats_hawthorne_approach` (Felony Flats)
3. In `director_park`, add exit: `south` → `lo_checkpoint_north` (Lake Oswego)
4. In `transit_mall`, add exit: `west` → `beav_canyon_road_east` (Beaverton)
5. In `waterfront_trail`, add exit: `northwest` → `sauvie_bridge_south` (Sauvie Island)

**Verification checklist — every cross-zone exit pair must exist:**

| Zone A Room ID | Direction | Zone B Room ID | Direction |
|----------------|-----------|----------------|-----------|
| `burnside_crossing` | north | `ne_broadway_bridge` | south |
| `courthouse_steps` | southeast | `flats_hawthorne_approach` | northwest |
| `director_park` | south | `lo_checkpoint_north` | north |
| `transit_mall` | west | `beav_canyon_road_east` | east |
| `waterfront_trail` | northwest | `sauvie_bridge_south` | southeast |
| `ne_broadway_bridge` | north | `couve_interstate_bridge_south` | south |
| `ne_i205_onramp` | east | `pdx_airport_way_west` | west |
| `ne_cully_road` | east | `grinders_row` | west |
| `couve_fourth_plain_east` | east | `vantucky_fourth_plain_west` | west |
| `couve_i5_north` | north | `battle_i5_south` | south |
| `vantucky_i84_east` | east | `trout_i84_west` | west |
| `pdx_i84_east` | east | `trout_i84_west_junction` | west |
| `flats_foster_south` | south | `sei_powell_terminus` | north |
| `sei_ross_bridge_approach` | south | `ross_bridge_east` | north |
| `beav_tv_highway_west` | west | `aloha_tv_highway_east` | east |
| `aloha_tv_highway_west` | west | `hills_tv_highway_east` | east |

For each pair: confirm room ID exists in the respective zone YAML and the exit direction + target are correctly specified in BOTH directions.

Also update Rustbucket Ridge: add exit from `grinders_row` west → (will be set by NE Portland zone, room `ne_cully_road` targeting `grinders_row`). Actually `grinders_row` needs a `west` exit → `ne_cully_road`.

**Step 1:** Modify `content/zones/downtown.yaml` — add the 5 new exits listed above.
**Step 2:** Modify `content/zones/rustbucket_ridge.yaml` — add exit from `grinders_row` west → `ne_cully_road`.
**Step 3:** Run full validation:
```bash
mise exec -- go build ./... 2>&1 && mise exec -- go test ./internal/game/world/... ./internal/game/npc/... ./internal/game/inventory/... -race -count=1 -timeout=60s 2>&1
```
**Step 4:** Run the full test suite:
```bash
mise exec -- go test -race -count=1 -timeout=300s $(mise exec -- go list ./... | grep -v 'storage/postgres') 2>&1 | tail -30
```
**Step 5:** Commit:
```bash
git add content/zones/downtown.yaml content/zones/rustbucket_ridge.yaml
git commit -m "feat(content): wire cross-zone exits between all 16 zones"
```

---

## Task 17: Final Verification + Tag

**Step 1:** Full test suite:
```bash
mise exec -- go test -race -count=1 -timeout=300s $(mise exec -- go list ./... | grep -v 'storage/postgres') 2>&1 | tail -30
```

**Step 2:** Build both binaries:
```bash
mise exec -- go build -o /dev/null ./cmd/gameserver 2>&1
mise exec -- go build -o /dev/null ./cmd/frontend 2>&1
```

**Step 3:** Verify zone/room counts:
```bash
mise exec -- go test ./internal/game/world/... -run TestLoadZonesFromDir -v 2>&1
```
Expected: 16 zones loaded successfully.

**Step 4:** Verify all items load:
```bash
mise exec -- go test ./internal/game/inventory/... -run TestLoadItems -v 2>&1
```

**Step 5:** Verify all NPC templates load:
```bash
mise exec -- go test ./internal/game/npc/... -run TestLoadTemplates -v 2>&1
```

**Step 6:** Tag:
```bash
git tag stage12-complete
git log --oneline -20
```

---

## Critical Notes

- **Room IDs must be globally unique.** Use zone prefix convention: `ne_`, `flats_`, `lo_`, `beav_`, `aloha_`, `hills_`, `couve_`, `vantucky_`, `battle_`, `sei_`, `sauvie_`, `ross_`, `trout_`, `pdx_`.
- **All exits are one-directional in YAML.** For bidirectional passage, define the exit in BOTH rooms.
- **Cross-zone exits work automatically** — the world manager's flat room index resolves any room ID regardless of zone.
- **The `lieutenant` NPC template exists** but has no `ai_domain`, `respawn_delay`, or `loot` — do NOT reference it in spawns until those fields are added (only use it in Rustbucket Ridge where it was added in Task 1).
- **Only two AI domains exist:** `ganger_combat` (aggressive, attacks enemies) and `scavenger_patrol` (passive/patrol behavior). All new NPCs must use one of these.
