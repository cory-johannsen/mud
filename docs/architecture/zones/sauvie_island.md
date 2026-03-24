# Sauvie Island

Zone ID: `sauvie_island` | Danger Level: sketchy | World Position: (-2, -2)

```mermaid
graph LR
    sauvie_bridge_south["Sauvie Island Bridge — South End\n[DL: med]"]
    sauvie_tollgate["Bridge Tollgate\n[DL: med]"]
    sauvie_road["Sauvie Island Road\n[DL: med]"]
    sauvie_pumpkin_patch["Pumpkin Patch Fields\n[DL: med]"]
    sauvie_berry_fields["Berry Fields\n[DL: med]"]
    sauvie_farmhouse["Island Farmhouse\n[DL: med]"]
    sauvie_barn["Old Red Barn\n[DL: med]"]
    sauvie_chicken_coop["Chicken Coops\n[DL: med]"]
    sauvie_dairy_farm["Dairy Farm Compound\n[DL: med]"]
    sauvie_granary["Island Granary\n[DL: med]"]
    sauvie_irrigation_canal["Irrigation Canal\n[DL: med]"]
    sauvie_collins_beach["Collins Beach — Pirate Cove\n[DL: med]"]
    sauvie_river_dock["River Dock\n[DL: med]"]
    sauvie_gilbert_river["Gilbert River Waterway\n[DL: med]"]
    sauvie_oak_island["Oak Island Wetlands\n[DL: med]"]
    sauvie_the_orchard["The Orchard\n[DL: med]"]
    sauvie_north_fields["Northern Fields\n[DL: med]"]
    sauvie_sturgeon_lake["Sturgeon Lake\n[DL: med]"]
    sauvie_fishing_camp["Fishing Camp\n[DL: med]"]
    sauvie_north_tip["North Tip Overlook\n[DL: med]"]
    sauvie_trading_post["Island Trading Post\n[DL: med]"]
    sauvie_farm_stand["Farm Stand\n[DL: safe] [S]"]

    sauvie_bridge_south -->|north| sauvie_tollgate
    sauvie_bridge_south -->|southeast| waterfront_trail
    sauvie_bridge_south -->|south| sauvie_farm_stand
    sauvie_tollgate -->|south| sauvie_bridge_south
    sauvie_tollgate -->|north| sauvie_road
    sauvie_road -->|south| sauvie_tollgate
    sauvie_road -->|north| sauvie_trading_post
    sauvie_road -->|east| sauvie_pumpkin_patch
    sauvie_road -->|west| sauvie_irrigation_canal
    sauvie_pumpkin_patch -->|west| sauvie_road
    sauvie_berry_fields -->|east| sauvie_the_orchard
    sauvie_berry_fields -->|north| sauvie_chicken_coop
    sauvie_chicken_coop -->|south| sauvie_berry_fields
    sauvie_chicken_coop -->|north| sauvie_dairy_farm
    sauvie_dairy_farm -->|north| sauvie_oak_island
    sauvie_irrigation_canal -->|east| sauvie_road
    sauvie_irrigation_canal -->|north| sauvie_gilbert_river
    sauvie_irrigation_canal -->|west| sauvie_collins_beach
    sauvie_collins_beach -->|east| sauvie_irrigation_canal
    sauvie_collins_beach -->|north| sauvie_river_dock
    sauvie_river_dock -->|south| sauvie_collins_beach
    sauvie_river_dock -->|east| sauvie_gilbert_river
    sauvie_gilbert_river -->|south| sauvie_irrigation_canal
    sauvie_gilbert_river -->|west| sauvie_river_dock
    sauvie_gilbert_river -->|north| sauvie_sturgeon_lake
    sauvie_oak_island -->|south| sauvie_dairy_farm
    sauvie_the_orchard -->|west| sauvie_berry_fields
    sauvie_the_orchard -->|north| sauvie_north_fields
    sauvie_north_fields -->|north| sauvie_north_tip
    sauvie_sturgeon_lake -->|north| sauvie_fishing_camp
    sauvie_fishing_camp -->|south| sauvie_sturgeon_lake
    sauvie_north_tip -->|south| sauvie_north_fields
    sauvie_trading_post -->|south| sauvie_road
    sauvie_trading_post -->|north| sauvie_berry_fields
    sauvie_trading_post -->|east| sauvie_farmhouse
    sauvie_farm_stand -->|north| sauvie_bridge_south
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| sauvie_bridge_south | Sauvie Island Bridge — South End | med | 0 | 0 |
| sauvie_tollgate | Bridge Tollgate | med | 0 | -2 |
| sauvie_road | Sauvie Island Road | med | 0 | -4 |
| sauvie_pumpkin_patch | Pumpkin Patch Fields | med | 2 | -4 |
| sauvie_berry_fields | Berry Fields | med | 0 | -8 |
| sauvie_farmhouse | Island Farmhouse | med | 2 | -6 |
| sauvie_barn | Old Red Barn | med | 202 | 0 |
| sauvie_chicken_coop | Chicken Coops | med | 0 | -10 |
| sauvie_dairy_farm | Dairy Farm Compound | med | 0 | -12 |
| sauvie_granary | Island Granary | med | 202 | 2 |
| sauvie_irrigation_canal | Irrigation Canal | med | -2 | -4 |
| sauvie_collins_beach | Collins Beach — Pirate Cove | med | -4 | -4 |
| sauvie_river_dock | River Dock | med | -4 | -6 |
| sauvie_gilbert_river | Gilbert River Waterway | med | -2 | -6 |
| sauvie_oak_island | Oak Island Wetlands | med | 0 | -14 |
| sauvie_the_orchard | The Orchard | med | 2 | -8 |
| sauvie_north_fields | Northern Fields | med | 2 | -10 |
| sauvie_sturgeon_lake | Sturgeon Lake | med | -2 | -8 |
| sauvie_fishing_camp | Fishing Camp | med | -2 | -10 |
| sauvie_north_tip | North Tip Overlook | med | 2 | -12 |
| sauvie_trading_post | Island Trading Post | med | 0 | -6 |
| sauvie_farm_stand | Farm Stand | safe | 0 | 2 |
