# Ross Island

Zone ID: `ross_island` | Danger Level: dangerous | World Position: (0, 2)

```mermaid
graph LR
    ross_bridge_east["Ross Island Bridge — East Landing\n[DL: high]"]
    ross_toll_gate["Toll Gate\n[DL: high]"]
    ross_bridge_road["Bridge Road\n[DL: high]"]
    ross_east_shore["East Shore\n[DL: high]"]
    ross_shoreline_path["Shoreline Path\n[DL: high]"]
    ross_south_shore["South Shore\n[DL: high]"]
    ross_tidal_pool["Tidal Pool\n[DL: high]"]
    ross_driftwood_camp["Driftwood Camp\n[DL: high]"]
    ross_the_overgrowth["The Overgrowth\n[DL: high]"]
    ross_the_hermitage["The Hermitage\n[DL: high]"]
    ross_cave_entrance["Cave Entrance\n[DL: high]"]
    ross_cave_interior["Cave Interior\n[DL: high]"]
    ross_rubble_ridge["Rubble Ridge\n[DL: high]"]
    ross_pit_edge["Pit Edge\n[DL: high]"]
    ross_gravel_pit["Gravel Pit\n[DL: high]"]
    ross_quarry_floor["Quarry Floor\n[DL: high]"]
    ross_lagoon["Lagoon\n[DL: high]"]
    ross_crane_platform["Crane Platform\n[DL: high]"]
    ross_north_shore["North Shore\n[DL: high]"]
    ross_west_shore["West Shore\n[DL: high]"]
    ross_hardtack_island["Hardtack Island\n[DL: high]"]
    ross_dock_shack["Dock Shack\n[DL: safe] [S]"]

    ross_bridge_east -->|north| sei_ross_bridge_approach
    ross_bridge_east -->|west| ross_toll_gate
    ross_bridge_east -->|south| ross_east_shore
    ross_bridge_east -->|east| ross_dock_shack
    ross_toll_gate -->|east| ross_bridge_east
    ross_toll_gate -->|west| ross_bridge_road
    ross_toll_gate -->|south| ross_shoreline_path
    ross_bridge_road -->|east| ross_toll_gate
    ross_bridge_road -->|west| ross_pit_edge
    ross_bridge_road -->|south| ross_the_overgrowth
    ross_east_shore -->|north| ross_bridge_east
    ross_east_shore -->|south| ross_south_shore
    ross_east_shore -->|west| ross_shoreline_path
    ross_shoreline_path -->|north| ross_toll_gate
    ross_shoreline_path -->|east| ross_east_shore
    ross_shoreline_path -->|south| ross_tidal_pool
    ross_tidal_pool -->|north| ross_shoreline_path
    ross_tidal_pool -->|west| ross_driftwood_camp
    ross_driftwood_camp -->|east| ross_tidal_pool
    ross_the_overgrowth -->|north| ross_bridge_road
    ross_the_hermitage -->|north| ross_cave_entrance
    ross_cave_entrance -->|south| ross_the_hermitage
    ross_cave_entrance -->|down| ross_cave_interior
    ross_cave_entrance -->|east| ross_pit_edge
    ross_cave_interior -->|up| ross_cave_entrance
    ross_rubble_ridge -->|north| ross_pit_edge
    ross_rubble_ridge -->|down| ross_quarry_floor
    ross_pit_edge -->|east| ross_bridge_road
    ross_pit_edge -->|south| ross_rubble_ridge
    ross_pit_edge -->|down| ross_gravel_pit
    ross_pit_edge -->|west| ross_cave_entrance
    ross_pit_edge -->|north| ross_north_shore
    ross_gravel_pit -->|up| ross_pit_edge
    ross_gravel_pit -->|south| ross_quarry_floor
    ross_gravel_pit -->|east| ross_crane_platform
    ross_gravel_pit -->|west| ross_lagoon
    ross_quarry_floor -->|north| ross_gravel_pit
    ross_quarry_floor -->|up| ross_rubble_ridge
    ross_quarry_floor -->|west| ross_lagoon
    ross_lagoon -->|east| ross_gravel_pit
    ross_lagoon -->|south| ross_quarry_floor
    ross_crane_platform -->|west| ross_gravel_pit
    ross_crane_platform -->|north| ross_pit_edge
    ross_north_shore -->|south| ross_pit_edge
    ross_north_shore -->|west| ross_west_shore
    ross_west_shore -->|east| ross_north_shore
    ross_west_shore -->|west| ross_hardtack_island
    ross_hardtack_island -->|east| ross_west_shore
    ross_dock_shack -->|west| ross_bridge_east
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| ross_bridge_east | Ross Island Bridge — East Landing | high | 0 | 0 |
| ross_toll_gate | Toll Gate | high | -2 | 0 |
| ross_bridge_road | Bridge Road | high | -4 | 0 |
| ross_east_shore | East Shore | high | 0 | 2 |
| ross_shoreline_path | Shoreline Path | high | -2 | 2 |
| ross_south_shore | South Shore | high | 0 | 4 |
| ross_tidal_pool | Tidal Pool | high | -2 | 4 |
| ross_driftwood_camp | Driftwood Camp | high | -4 | 4 |
| ross_the_overgrowth | The Overgrowth | high | -4 | 2 |
| ross_the_hermitage | The Hermitage | high | -8 | 2 |
| ross_cave_entrance | Cave Entrance | high | -8 | 0 |
| ross_cave_interior | Cave Interior | high | 202 | 0 |
| ross_rubble_ridge | Rubble Ridge | high | -6 | 2 |
| ross_pit_edge | Pit Edge | high | -6 | 0 |
| ross_gravel_pit | Gravel Pit | high | 202 | 2 |
| ross_quarry_floor | Quarry Floor | high | 202 | 4 |
| ross_lagoon | Lagoon | high | 202 | 6 |
| ross_crane_platform | Crane Platform | high | 202 | 8 |
| ross_north_shore | North Shore | high | -6 | -2 |
| ross_west_shore | West Shore | high | -8 | -2 |
| ross_hardtack_island | Hardtack Island | high | -10 | -2 |
| ross_dock_shack | Dock Shack | safe | 2 | 0 |
