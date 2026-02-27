# Stage 12 — Zone Expansion Design

## Goal

Expand the game world from 2 zones to 16 zones across the Portland metro area. Add 14 new zones with 20+ rooms each, zone-specific NPC templates with loot tables, zone-specific items, cross-zone connections, and fix Rustbucket Ridge's broken room connectivity.

## Scope

- SCOPE-1: 14 new zone YAML files MUST be created, each with 20+ rooms.
- SCOPE-2: Each new zone MUST have 2-4 unique NPC template YAML files.
- SCOPE-3: Each zone MUST have zone-specific item YAML files for NPC loot.
- SCOPE-4: All zones MUST be connected via cross-zone exits per the connectivity map.
- SCOPE-5: Rustbucket Ridge MUST have its 30 dead-end rooms connected with exits.
- SCOPE-6: All new NPC templates MUST have loot tables.
- SCOPE-7: No Go code changes are required — this is pure content.

## Zone Network

```
                ┌───────────────────┐
                │    Battleground   │
                │Socialist Collective│
                └────────┬──────────┘
                    I-5  │
        ┌────────────────┼────────────────┐
        │                │                │
 ┌──────┴──────┐  ┌──────┴──────┐  ┌─────┴───────┐
 │  The Couve  │──│  Vantucky   │  │  Troutdale  │
 │ (W Vanc.)   │  │ (E Vanc.)   │  │  (E Gorge)  │
 └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
   I-5  │           I-5  │          I-84  │
        │                │                │
 ┌──────┴──────┐         │         ┌──────┴──────┐
 │Sauvie Island│         │         │     PDX     │
 │             │         │         │International│
 └──────┬──────┘         │         └──────┬──────┘
        │                │                │
        │         ┌──────┴──────┐  ┌──────┴──────┐
        │         │ NE Portland │──│ Rustbucket  │
        │         │             │  │   Ridge     │
        │         └──────┬──────┘  └─────────────┘
        │                │
 ┌──────┴────────────────┴──────────┐
 │         Downtown Portland        │
 └──┬────────┬────────┬─────────┬───┘
    │        │        │         │
    │  ┌─────┴─────┐  │  ┌─────┴──────────┐
    │  │  Felony   │  │  │  Free State of  │
    │  │  Flats    │  │  │   Beaverton     │
    │  └─────┬─────┘  │  └──────┬──────────┘
    │  ┌─────┴─────┐  │  ┌──────┴──────────┐
    │  │ SE Indust.│  │  │  Aloha Neutral  │
    │  │           │  │  │     Zone        │
    │  └─────┬─────┘  │  └──────┴──────────┘
    │  ┌─────┴─────┐  │  ┌──────┴──────────┐
    │  │Ross Island│  │  │   Kingdom of    │
    │  │           │  │  │   Hillsboro     │
    │  └───────────┘  │  └─────────────────┘
    │          ┌──────┴──────┐
    └──────────│Lake Oswego  │
               │  Nation     │
               └─────────────┘
```

## Cross-Zone Connections

| From | To | Route |
|------|----|-------|
| Downtown | NE Portland | Broadway Bridge / I-84 on-ramp |
| Downtown | Felony Flats | Hawthorne/SE approach |
| Downtown | Lake Oswego Nation | Macadam Ave / 99W south |
| Downtown | Free State of Beaverton | US-26 / Canyon Road |
| Downtown | Sauvie Island | US-30 northwest |
| NE Portland | Rustbucket Ridge | Adjacent neighborhoods |
| NE Portland | PDX International | I-205 / Airport Way |
| NE Portland | The Couve | I-5 / Interstate Bridge |
| The Couve | Vantucky | Adjacent (E/W) |
| The Couve | Battleground Socialist Collective | I-5 north |
| Vantucky | Troutdale | I-84 east |
| PDX International | Troutdale | I-84 east |
| Felony Flats | SE Industrial | South along river |
| SE Industrial | Ross Island | Bridge crossing |
| Free State of Beaverton | Aloha Neutral Zone | TV Highway west |
| Aloha Neutral Zone | Kingdom of Hillsboro | TV Highway west |

All connections are bidirectional (exit defined in both zones' border rooms).

## Zone Themes & NPC Templates

| Zone | Theme | NPCs |
|------|-------|------|
| NE Portland | Gentrification ruins, craft brewery fortresses, bike gang territory | Bike Courier, Brew Warlord, Alberta Street Drifter |
| Felony Flats | 82nd Ave strip malls, motels-turned-forts, survival hustle | Motel Raider, Strip Mall Scav, 82nd Enforcer |
| Lake Oswego Nation | Gated wealth compound, paranoid elites, private militia | Oswego Guard, Country Club Sniper, HOA Enforcer |
| Free State of Beaverton | Suburban fortress-state, strip mall bureaucracy, mall militias | Beaverton Minuteman, Mall Cop Elite, Cedar Hills Patrol |
| Aloha Neutral Zone | Demilitarized trading post, neutral ground, smugglers | Aloha Broker, Neutral Zone Sentry, Smuggler |
| Kingdom of Hillsboro | Tech campus feudalism, server farm keeps, engineer-lords | Hillsboro Knight, Silicon Serf, Intel Guard |
| The Couve | Vancouver strip mall empire, chain restaurant warlords | Couve Militia, Jantzen Beach Pirate, Mill Plain Thug |
| Vantucky | Rural-urban fringe, gun culture, fortress compounds | Vantucky Militiaman, Compound Guard, Highway Bandit |
| Battleground Socialist Collective | Commune society, collective farms, ideological enforcers | Collective Guardian, Commissar, Field Worker |
| SE Industrial | Warehouses, river docks, industrial salvage | Dock Worker, Warehouse Guard, Industrial Scav |
| Sauvie Island | Agrarian holdout, farm compounds, river pirates | Island Farmer, River Pirate, Harvest Guard |
| Ross Island | Isolated fortress, gravel pit camps, bridge toll gangs | Bridge Troll, Gravel Pit Boss, Island Hermit |
| Troutdale | Gorge gateway, outlet mall ruins, wind-blasted outpost | Gorge Runner, Outlet Scavenger, Wind Walker |
| PDX International | Abandoned airport, cargo cult, runway camps | Cargo Cultist, Terminal Squatter, Tarmac Raider |

## NPC Design Constraints

- NPC-1: Each NPC template MUST have full ability scores (str/dex/con/int/wis/cha).
- NPC-2: Each NPC template MUST have a loot table with currency and at least one item drop.
- NPC-3: NPC levels MUST vary within each zone (mixed difficulty).
- NPC-4: Each NPC template MUST have a respawn_delay.
- NPC-5: NPC stats MUST scale with level (higher level = more HP, higher AC, better abilities).

## Room Design Constraints

- ROOM-1: Each room MUST have a title and multi-line description with Portland dystopian flavor.
- ROOM-2: Each room MUST have at least one exit (no dead-end rooms).
- ROOM-3: Rooms with NPC spawns MUST define spawn configs (template, count, respawn_after).
- ROOM-4: Each zone MUST have a start_room defined.
- ROOM-5: Room IDs MUST be globally unique across all zones.
- ROOM-6: All exits MUST target valid room IDs (within same zone or cross-zone).

## Rustbucket Ridge Fix

The existing Rustbucket Ridge zone has 32 rooms but only 2 exits (grinders_row ↔ rotgut_alley). The remaining 30 rooms are unreachable dead ends. This fix will:

- Connect rooms within each area cluster (filth_court, scrapheap_circle, rotgut_alley, boneyard_bend, cinder_pit, shiv_way).
- Connect area clusters to the main spine (grinders_row ↔ rotgut_alley).
- Add NPC spawns to appropriate rooms.

## Deliverables

- 14 new zone YAML files in `content/zones/`
- ~40 new NPC template YAML files in `content/npcs/`
- ~40 new item YAML files in `content/items/`
- Modified `content/zones/rustbucket_ridge.yaml` (exits + spawns)
- Modified `content/zones/downtown.yaml` (cross-zone border exits)
