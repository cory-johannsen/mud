# Vantucky

Zone ID: `vantucky` | Danger Level: dangerous | World Position: (2, -4)

```mermaid
graph LR
    vantucky_fourth_plain_west["Fourth Plain — Vantucky Border\n[DL: high]"]
    vantucky_andresen_road["Andresen Road\n[DL: high]"]
    vantucky_orchards["The Orchards\n[DL: high]"]
    vantucky_164th_ave["164th Avenue — Militia Territory\n[DL: high]"]
    vantucky_i84_overpass["I-84 Overpass\n[DL: high]"]
    vantucky_fishers_landing["Fisher's Landing\n[DL: high]"]
    vantucky_compound_alpha["Compound Alpha\n[DL: high]"]
    vantucky_compound_bravo["Compound Bravo\n[DL: high]"]
    vantucky_gun_market["Gun Market\n[DL: high]"]
    vantucky_burnt_bridge_creek["Burnt Bridge Creek\n[DL: high]"]
    vantucky_i84_east["I-84 East — Troutdale Border\n[DL: high]"]
    vantucky_gas_station_ruins["Gas Station Ruins\n[DL: high]"]
    vantucky_shooting_range["Shooting Range\n[DL: high]"]
    vantucky_hidden_bunker["Hidden Bunker\n[DL: high]"]
    vantucky_farm_compound["Farm Compound\n[DL: high]"]
    vantucky_checkpoint_north["Northern Checkpoint\n[DL: high]"]
    vantucky_river_trail["Columbia River Trail\n[DL: high]"]
    vantucky_i84_onramp["I-84 On-Ramp\n[DL: high]"]
    vantucky_water_tower["Water Tower\n[DL: high]"]
    vantucky_ammo_depot["Ammo Depot\n[DL: high]"]
    vantucky_trailer_park["Trailer Park\n[DL: high]"]
    vantucky_the_compound["The Compound\n[DL: safe] [S]"]

    vantucky_fourth_plain_west -->|west| couve_fourth_plain_east
    vantucky_fourth_plain_west -->|east| vantucky_andresen_road
    vantucky_fourth_plain_west -->|south| vantucky_gas_station_ruins
    vantucky_fourth_plain_west -->|north| vantucky_the_compound
    vantucky_andresen_road -->|west| vantucky_fourth_plain_west
    vantucky_andresen_road -->|north| vantucky_checkpoint_north
    vantucky_andresen_road -->|east| vantucky_orchards
    vantucky_andresen_road -->|south| vantucky_gun_market
    vantucky_orchards -->|west| vantucky_andresen_road
    vantucky_orchards -->|north| vantucky_compound_alpha
    vantucky_orchards -->|east| vantucky_164th_ave
    vantucky_orchards -->|south| vantucky_burnt_bridge_creek
    vantucky_164th_ave -->|west| vantucky_orchards
    vantucky_164th_ave -->|north| vantucky_compound_bravo
    vantucky_164th_ave -->|east| vantucky_shooting_range
    vantucky_i84_overpass -->|east| vantucky_i84_east
    vantucky_i84_overpass -->|west| vantucky_i84_onramp
    vantucky_fishers_landing -->|west| vantucky_river_trail
    vantucky_compound_alpha -->|south| vantucky_orchards
    vantucky_compound_alpha -->|east| vantucky_compound_bravo
    vantucky_compound_bravo -->|south| vantucky_164th_ave
    vantucky_compound_bravo -->|west| vantucky_compound_alpha
    vantucky_compound_bravo -->|north| vantucky_farm_compound
    vantucky_gun_market -->|north| vantucky_andresen_road
    vantucky_gun_market -->|east| vantucky_burnt_bridge_creek
    vantucky_burnt_bridge_creek -->|north| vantucky_orchards
    vantucky_burnt_bridge_creek -->|west| vantucky_gun_market
    vantucky_burnt_bridge_creek -->|east| vantucky_hidden_bunker
    vantucky_i84_east -->|west| vantucky_i84_overpass
    vantucky_i84_east -->|east| trout_i84_west
    vantucky_gas_station_ruins -->|north| vantucky_fourth_plain_west
    vantucky_gas_station_ruins -->|east| vantucky_gun_market
    vantucky_gas_station_ruins -->|south| vantucky_i84_onramp
    vantucky_shooting_range -->|west| vantucky_164th_ave
    vantucky_shooting_range -->|south| vantucky_ammo_depot
    vantucky_hidden_bunker -->|west| vantucky_burnt_bridge_creek
    vantucky_farm_compound -->|south| vantucky_compound_bravo
    vantucky_farm_compound -->|east| vantucky_water_tower
    vantucky_checkpoint_north -->|south| vantucky_andresen_road
    vantucky_checkpoint_north -->|north| vantucky_river_trail
    vantucky_river_trail -->|south| vantucky_checkpoint_north
    vantucky_river_trail -->|east| vantucky_fishers_landing
    vantucky_i84_onramp -->|north| vantucky_gas_station_ruins
    vantucky_i84_onramp -->|east| vantucky_i84_overpass
    vantucky_water_tower -->|west| vantucky_farm_compound
    vantucky_ammo_depot -->|north| vantucky_shooting_range
    vantucky_trailer_park -->|north| vantucky_gas_station_ruins
    vantucky_trailer_park -->|east| vantucky_burnt_bridge_creek
    vantucky_the_compound -->|south| vantucky_fourth_plain_west
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| vantucky_fourth_plain_west | Fourth Plain — Vantucky Border | high | 0 | 0 |
| vantucky_andresen_road | Andresen Road | high | 2 | 0 |
| vantucky_orchards | The Orchards | high | 4 | 0 |
| vantucky_164th_ave | 164th Avenue — Militia Territory | high | 6 | 0 |
| vantucky_i84_overpass | I-84 Overpass | high | 2 | 4 |
| vantucky_fishers_landing | Fisher's Landing | high | 4 | -4 |
| vantucky_compound_alpha | Compound Alpha | high | 4 | -2 |
| vantucky_compound_bravo | Compound Bravo | high | 6 | -2 |
| vantucky_gun_market | Gun Market | high | 2 | 2 |
| vantucky_burnt_bridge_creek | Burnt Bridge Creek | high | 4 | 2 |
| vantucky_i84_east | I-84 East — Troutdale Border | high | 4 | 4 |
| vantucky_gas_station_ruins | Gas Station Ruins | high | 0 | 2 |
| vantucky_shooting_range | Shooting Range | high | 8 | 0 |
| vantucky_hidden_bunker | Hidden Bunker | high | 6 | 2 |
| vantucky_farm_compound | Farm Compound | high | 6 | -4 |
| vantucky_checkpoint_north | Northern Checkpoint | high | 2 | -2 |
| vantucky_river_trail | Columbia River Trail | high | 2 | -4 |
| vantucky_i84_onramp | I-84 On-Ramp | high | 0 | 4 |
| vantucky_water_tower | Water Tower | high | 8 | -4 |
| vantucky_ammo_depot | Ammo Depot | high | 8 | 2 |
| vantucky_trailer_park | Trailer Park | high | 202 | 0 |
| vantucky_the_compound | The Compound | safe | 0 | -2 |
