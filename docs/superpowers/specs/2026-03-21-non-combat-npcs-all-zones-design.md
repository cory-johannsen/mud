# Non-Combat NPCs — All Zones Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `non-combat-npcs-all-zones` (priority 310)
**Dependencies:** `non-combat-npcs`

---

## Overview

Places one lore-appropriate instance of each required non-combat NPC type in every zone. Each zone receives a safe room (or uses an existing one) as the NPC hub. All NPC templates are defined in `content/npcs/non_combat/` YAML files. This is a content/data feature — all NPC behavior is implemented by `non-combat-npcs`.

---

## 1. Safe Rooms

Every zone MUST have at least one room with `danger_level: safe` to host non-combat NPC spawns.

### 1.1 Existing Safe Rooms (no changes needed)

Zones where the entire zone is safe use one designated anchor room as the NPC hub. No new rooms are added.

| Zone | Safe Room ID | Title |
|------|-------------|-------|
| `aloha` | `aloha_the_bazaar` | The Bazaar |
| `lake_oswego` | `lo_the_commons` | The Commons |
| `battleground` | `battle_infirmary` | The Infirmary |
| `felony_flats` | `flats_jade_district` | Jade District |

### 1.2 New Safe Rooms Required

For the 12 zones without a safe room, a new hub room is added to the zone YAML. The new room has `danger_level: safe` and is connected bidirectionally to the specified anchor room via the listed exit directions.

| Zone | New Room ID | New Room Title | Anchor Room | Exit Direction (anchor→new) |
|------|------------|----------------|-------------|----------------------------|
| `beaverton` | `beav_free_market` | The Free Market | `beav_canyon_road_east` | north |
| `downtown` | `downtown_underground` | The Underground | `morrison_bridge` | north |
| `hillsboro` | `hills_the_keep` | The Keep | `hills_tv_highway_east` | south |
| `ne_portland` | `ne_corner_store` | The Corner Store | `ne_alberta_street` | north |
| `pdx_international` | `pdx_terminal_b` | Terminal B | `pdx_airport_way_west` | south |
| `ross_island` | `ross_dock_shack` | The Dock Shack | `ross_bridge_east` | east |
| `rustbucket_ridge` | `rust_scrap_office` | The Scrap Office | `last_stand_lodge` | east |
| `sauvie_island` | `sauvie_farm_stand` | The Farm Stand | `sauvie_bridge_south` | south |
| `se_industrial` | `sei_break_room` | The Break Room | `sei_holgate_blvd` | east |
| `the_couve` | `couve_the_crossing` | The Crossing | `couve_interstate_bridge_south` | west |
| `troutdale` | `trout_truck_stop` | The Truck Stop | `trout_i84_west` | north |
| `vantucky` | `vantucky_the_compound` | The Compound | `vantucky_fourth_plain_west` | north |

New room descriptions:

- **beav_free_market** — "An open-air block of vendor stalls under corrugated aluminum roofing. The smell of hot food and machine oil. People come here to trade, not fight."
- **downtown_underground** — "A repurposed parking garage two levels below street level. Strip lighting, folding tables, and the low hum of people who need things and people who have them."
- **hills_the_keep** — "A fortified community hall at the edge of the Hillsboro enclave. Stone walls and firelight. A place of order, or something close to it."
- **ne_corner_store** — "A converted convenience store with the shelving pushed to the walls. Locals come here to restock, get patched up, and hear what's going on in the neighborhood."
- **pdx_terminal_b** — "A section of the airport terminal cordoned off from the main concourse. Chairs bolted to the floor, vending machines that still work, and people who've learned to wait."
- **ross_dock_shack** — "A weathered shack at the island's main landing. Nets hang on the walls, a woodstove burns in the corner, and someone is always willing to do business."
- **rust_scrap_office** — "A repurposed foreman's office at the edge of the ridge. Metal desk, fluorescent light, and a corkboard full of job postings nobody's taken down."
- **sauvie_farm_stand** — "A roadside stand that evolved into a community hub. Folding tables with produce, herbs, and handmade goods. Calm enough that people leave their weapons at the door."
- **sei_break_room** — "A cinder-block room with folding chairs and a microwave that runs off a generator. Shift workers and traders share the same coffee and the same fatigue."
- **couve_the_crossing** — "A checkpoint building at the Washington end of the bridge. The Couve faction controls it, but they're practical: trade is welcome, trouble is not."
- **trout_truck_stop** — "A diesel-soaked rest stop with a diner counter, a parts wall, and a back room where deals get made. Everyone passes through Troutdale eventually."
- **vantucky_the_compound** — "The Vantucky militia's main compound. Spare and functional. They'll trade, train, and bank here — loyalty is assumed, not enforced."

- REQ-NCNAZ-1: Every zone MUST have at least one room with `danger_level: safe` before non-combat NPC spawns are placed.
- REQ-NCNAZ-2: New safe rooms MUST be connected bidirectionally to the anchor room listed in Section 1.2 using the specified exit direction pair.
- REQ-NCNAZ-3: New safe rooms MUST have a `description` field matching the lore descriptions in Section 1.2.

---

## 2. NPC Type Coverage

### 2.1 Core Types (required in every zone)

- REQ-NCNAZ-0: The `non-combat-npcs` feature MUST implement the `banker` npc_role before this feature can be implemented.

| Type | npc_role |
|------|---------|
| Merchant | `merchant` |
| Healer | `healer` |
| Job Trainer | `job_trainer` |
| Banker | `banker` |

### 2.2 Optional Types (per zone as listed)

| Type | npc_role | Zones |
|------|---------|-------|
| Guard | `guard` | aloha, battleground, beaverton, downtown, hillsboro, pdx_international, se_industrial, the_couve |
| Hireling | `hireling` | beaverton, hillsboro, lake_oswego, ross_island, rustbucket_ridge, se_industrial, vantucky |
| Fixer | `fixer` | aloha, downtown, felony_flats, the_couve |
| Quest Giver | `quest_giver` | — (deferred to `quests` feature) |
| Crafter | `crafter` | — (deferred to `crafting` feature) |

- REQ-NCNAZ-4: All four core NPC types MUST be present in every zone's safe room.
- REQ-NCNAZ-5: Optional NPC types MUST be present only in zones listed in Section 2.2.
- REQ-NCNAZ-6: `quest_giver` and `crafter` NPC templates MUST NOT be placed in any zone until `quests` and `crafting` features are implemented respectively.

---

## 3. NPC Names Per Zone

All non-combat NPC templates use `respawn_after: 0s` (permanent spawn, never despawn).

| Zone | Safe Room | Merchant | Healer | Job Trainer | Banker | Optional |
|------|-----------|----------|--------|-------------|--------|----------|
| aloha | The Bazaar | Swap Meet Sally | Doc Neutral | The Coordinator | Escrow Eddie | Guard: Border Watcher; Fixer: The Adjuster |
| lake_oswego | The Commons | The Sommelier | Dr. Ashford | The Career Counselor | Private Banker | Hireling: The Attendant |
| battleground | The Infirmary | Commissar Goods | Field Medic Yuri | Political Officer | The Treasurer | Guard: People's Guard |
| felony_flats | Jade District | Mama Jade | Herbalist Chen | Uncle Bao | The Moneylender | Fixer: The Jade Fixer |
| beaverton | The Free Market | Free Trader Bo | Medtech Remy | Skills Broker | The Vault | Guard: Market Watch; Hireling: Trail Hand |
| downtown | The Underground | Street Vendor | Back-Alley Doc | The Fixer's Desk | Cash Mutual | Guard: Underground Muscle; Fixer: The Middleman |
| hillsboro | The Keep | Kingdom Merchant | Court Surgeon | The Chamberlain | Royal Treasury | Guard: Keep Knight; Hireling: Sworn Sword |
| ne_portland | The Corner Store | Neighborhood Mike | Nurse Practitioner | Trade School Rep | Credit Union | — |
| pdx_international | Terminal B | Duty-Free Dani | Airport Medic | Gate Agent | Currency Exchange | Guard: TSA Remnant |
| ross_island | The Dock Shack | Island Trader | Boat Medic | The Captain | The Chest | Hireling: Deck Hand |
| rustbucket_ridge | The Scrap Office | Parts Dealer | Welder's Medic | Shop Foreman | The Lockbox | Hireling: Grease Monkey |
| sauvie_island | The Farm Stand | Farmer's Market | Herb Woman | The Old Hand | The Tin Can | — |
| se_industrial | The Break Room | Shift Trader | Plant Medic | Union Rep | Paymaster | Guard: Shop Steward; Hireling: Temp Worker |
| the_couve | The Crossing | Border Trader | Couve Medic | The Recruiter | Border Bank | Guard: Crossing Guard; Fixer: The Smuggler |
| troutdale | The Truck Stop | Road Trader | Rest Stop Medic | The Dispatcher | Troutdale Trust | — |
| vantucky | The Compound | Compound Trader | Compound Doc | The Sergeant | The Ammo Box | Guard: Compound Guard; Hireling: Conscript |

---

## 4. Content Data Structure

### 4.1 NPC Template Files

One file per zone in `content/npcs/non_combat/<zone_id>.yaml`. Each file contains a YAML list of NPC template definitions.

Example (`content/npcs/non_combat/downtown.yaml`):

```yaml
- id: downtown_merchant
  name: "Street Vendor"
  npc_role: merchant
  type: human
  description: "A wiry figure behind a folding table stacked with whatever fell off the last truck."
  disposition: neutral
  respawn_after: 0s

- id: downtown_healer
  name: "Back-Alley Doc"
  npc_role: healer
  type: human
  description: "Moves like someone who's stitched a lot of wounds in bad light."
  disposition: neutral
  respawn_after: 0s

- id: downtown_job_trainer
  name: "The Fixer's Desk"
  npc_role: job_trainer
  type: human
  description: "A contact who knows who's hiring and what they need."
  disposition: neutral
  respawn_after: 0s

- id: downtown_banker
  name: "Cash Mutual"
  npc_role: banker
  type: human
  description: "No questions. No receipts. That's the deal."
  disposition: neutral
  respawn_after: 0s

- id: downtown_guard
  name: "Underground Muscle"
  npc_role: guard
  type: human
  description: "Big. Quiet. Watching the door."
  disposition: neutral
  respawn_after: 0s

- id: downtown_fixer
  name: "The Middleman"
  npc_role: fixer
  type: human
  description: "Connects problems with solutions for a modest fee."
  disposition: neutral
  respawn_after: 0s
```

### 4.2 Zone YAML Safe Room Entry

New safe rooms are appended to the zone's room list. Existing safe rooms (Section 1.1) gain spawn entries only — no new room is added.

Example (new safe room in `content/zones/downtown.yaml`):

```yaml
- id: downtown_underground
  title: "The Underground"
  description: "A repurposed parking garage two levels below street level. Strip lighting, folding tables, and the low hum of people who need things and people who have them."
  danger_level: safe
  map_x: 0   # implementing agent MUST resolve from anchor room coordinates + direction offset
  map_y: -4  # implementing agent MUST verify no existing room occupies these coordinates
  exits:
    - direction: south
      target: morrison_bridge
  spawns:
    - template: downtown_merchant
      count: 1
      respawn_after: 0s
    - template: downtown_healer
      count: 1
      respawn_after: 0s
    - template: downtown_job_trainer
      count: 1
      respawn_after: 0s
    - template: downtown_banker
      count: 1
      respawn_after: 0s
    - template: downtown_guard
      count: 1
      respawn_after: 0s
    - template: downtown_fixer
      count: 1
      respawn_after: 0s
```

The anchor room (`morrison_bridge`) gains a corresponding north exit to `downtown_underground`.

### 4.3 Template ID Convention

All non-combat NPC template IDs follow the pattern `<zone_id>_<npc_role>` (e.g., `downtown_merchant`, `ross_island_hireling`).

- REQ-NCNAZ-7: Template IDs MUST follow the `<zone_id>_<npc_role>` pattern.
- REQ-NCNAZ-8: All non-combat NPC templates MUST set `respawn_after: 0s`.
- REQ-NCNAZ-9: All non-combat NPC templates MUST set `disposition: neutral`.
- REQ-NCNAZ-10: Each template MUST have a unique lore-appropriate `name` and `description` matching the zone's theme.
- REQ-NCNAZ-11: Non-combat NPC template files MUST be located at `content/npcs/non_combat/<zone_id>.yaml`.
- REQ-NCNAZ-12: New safe room `map_x`/`map_y` coordinates MUST NOT overlap with any existing room in the zone. Coordinates MUST be adjacent to the anchor room in the direction specified in Section 1.2.
- REQ-NCNAZ-13: The anchor room's exit list MUST gain an exit in the direction specified in Section 1.2 to the new safe room. The new safe room MUST have the reverse direction exit back to the anchor room.

---

## 5. Requirements Summary

- REQ-NCNAZ-0: The `banker` npc_role MUST be implemented in `non-combat-npcs` before this feature can be implemented.
- REQ-NCNAZ-1: Every zone MUST have at least one room with `danger_level: safe` before non-combat NPC spawns are placed.
- REQ-NCNAZ-2: New safe rooms MUST be connected bidirectionally to the anchor room listed in Section 1.2 using the specified exit direction pair.
- REQ-NCNAZ-3: New safe rooms MUST have a `description` field matching the lore descriptions in Section 1.2.
- REQ-NCNAZ-4: All four core NPC types (merchant, healer, job_trainer, banker) MUST be present in every zone's safe room.
- REQ-NCNAZ-5: Optional NPC types MUST be present only in zones listed in Section 2.2.
- REQ-NCNAZ-6: `quest_giver` and `crafter` MUST NOT be placed until their dependent features are implemented.
- REQ-NCNAZ-7: Template IDs MUST follow the `<zone_id>_<npc_role>` pattern.
- REQ-NCNAZ-8: All non-combat NPC templates MUST set `respawn_after: 0s`.
- REQ-NCNAZ-9: All non-combat NPC templates MUST set `disposition: neutral`.
- REQ-NCNAZ-10: Each template MUST have a unique lore-appropriate name and description.
- REQ-NCNAZ-11: Template files MUST be located at `content/npcs/non_combat/<zone_id>.yaml`.
- REQ-NCNAZ-12: New safe room coordinates MUST NOT overlap with existing rooms; MUST be adjacent to the anchor room in the direction from Section 1.2.
- REQ-NCNAZ-13: The anchor room MUST gain an exit in the direction listed in Section 1.2 to the new safe room. The new safe room MUST have the reverse direction exit back to the anchor room.
