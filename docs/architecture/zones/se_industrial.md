# Southeast Industrial

Zone ID: `se_industrial` | Danger Level: dangerous | World Position: (2, 2)

```mermaid
graph LR
    sei_powell_terminus["Powell Boulevard Terminus\n[DL: high]"]
    sei_dock_row["Dock Row\n[DL: high]"]
    sei_warehouse_district["Warehouse District\n[DL: high]"]
    sei_crane_yard["Crane Yard\n[DL: high]"]
    sei_mcloughlin_blvd["McLoughlin Boulevard\n[DL: high]"]
    sei_milwaukie_junction["Milwaukie Junction\n[DL: high]"]
    sei_dry_dock["Dry Dock\n[DL: high]"]
    sei_industrial_way["Industrial Way\n[DL: high]"]
    sei_holgate_blvd["Holgate Boulevard\n[DL: high]"]
    sei_ross_bridge_approach["Ross Island Bridge Approach\n[DL: high]"]
    sei_loading_bay["Loading Bay\n[DL: high]"]
    sei_chemical_plant["Chemical Plant Ruins\n[DL: high]"]
    sei_pipe_yard["Pipe Yard\n[DL: high]"]
    sei_river_access["River Access\n[DL: high]"]
    sei_rail_siding["Rail Siding\n[DL: high]"]
    sei_machine_shop["Machine Shop\n[DL: high]"]
    sei_signal_tower["Signal Tower\n[DL: high]"]
    sei_warehouse_interior["Warehouse Interior\n[DL: high]"]
    sei_container_maze["Container Maze\n[DL: high]"]
    sei_transformer_station["Transformer Station\n[DL: high]"]
    sei_fuel_depot["Fuel Depot\n[DL: high]"]
    sei_break_room["Break Room\n[DL: safe] [S]"]

    sei_powell_terminus -->|north| flats_foster_south
    sei_powell_terminus -->|south| sei_industrial_way
    sei_powell_terminus -->|east| sei_holgate_blvd
    sei_powell_terminus -->|west| sei_mcloughlin_blvd
    sei_dock_row -->|west| sei_warehouse_district
    sei_dock_row -->|north| sei_river_access
    sei_dock_row -->|east| sei_loading_bay
    sei_warehouse_district -->|east| sei_dock_row
    sei_warehouse_district -->|north| sei_holgate_blvd
    sei_crane_yard -->|east| sei_industrial_way
    sei_crane_yard -->|south| sei_pipe_yard
    sei_crane_yard -->|north| sei_mcloughlin_blvd
    sei_mcloughlin_blvd -->|south| sei_crane_yard
    sei_mcloughlin_blvd -->|west| sei_chemical_plant
    sei_milwaukie_junction -->|south| sei_rail_siding
    sei_milwaukie_junction -->|east| sei_holgate_blvd
    sei_dry_dock -->|west| sei_ross_bridge_approach
    sei_industrial_way -->|north| sei_powell_terminus
    sei_industrial_way -->|south| sei_ross_bridge_approach
    sei_industrial_way -->|east| sei_warehouse_district
    sei_industrial_way -->|west| sei_crane_yard
    sei_holgate_blvd -->|west| sei_powell_terminus
    sei_holgate_blvd -->|north| sei_machine_shop
    sei_holgate_blvd -->|east| sei_break_room
    sei_ross_bridge_approach -->|north| sei_industrial_way
    sei_ross_bridge_approach -->|east| sei_dry_dock
    sei_ross_bridge_approach -->|south| ross_bridge_east
    sei_loading_bay -->|west| sei_dock_row
    sei_loading_bay -->|south| sei_warehouse_interior
    sei_chemical_plant -->|east| sei_mcloughlin_blvd
    sei_pipe_yard -->|north| sei_crane_yard
    sei_pipe_yard -->|south| sei_transformer_station
    sei_river_access -->|south| sei_dock_row
    sei_rail_siding -->|north| sei_milwaukie_junction
    sei_machine_shop -->|south| sei_holgate_blvd
    sei_machine_shop -->|east| sei_signal_tower
    sei_signal_tower -->|west| sei_machine_shop
    sei_warehouse_interior -->|north| sei_loading_bay
    sei_warehouse_interior -->|south| sei_container_maze
    sei_container_maze -->|north| sei_warehouse_interior
    sei_transformer_station -->|north| sei_pipe_yard
    sei_fuel_depot -->|east| sei_mcloughlin_blvd
    sei_fuel_depot -->|south| sei_chemical_plant
    sei_break_room -->|west| sei_holgate_blvd
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| sei_powell_terminus | Powell Boulevard Terminus | high | 0 | 0 |
| sei_dock_row | Dock Row | high | 4 | 2 |
| sei_warehouse_district | Warehouse District | high | 2 | 2 |
| sei_crane_yard | Crane Yard | high | -2 | 2 |
| sei_mcloughlin_blvd | McLoughlin Boulevard | high | -2 | 0 |
| sei_milwaukie_junction | Milwaukie Junction | high | 202 | 0 |
| sei_dry_dock | Dry Dock | high | 2 | 4 |
| sei_industrial_way | Industrial Way | high | 0 | 2 |
| sei_holgate_blvd | Holgate Boulevard | high | 2 | 0 |
| sei_ross_bridge_approach | Ross Island Bridge Approach | high | 0 | 4 |
| sei_loading_bay | Loading Bay | high | 6 | 2 |
| sei_chemical_plant | Chemical Plant Ruins | high | -4 | 0 |
| sei_pipe_yard | Pipe Yard | high | -2 | 4 |
| sei_river_access | River Access | high | 4 | 0 |
| sei_rail_siding | Rail Siding | high | 200 | 6 |
| sei_machine_shop | Machine Shop | high | 2 | -2 |
| sei_signal_tower | Signal Tower | high | 4 | -2 |
| sei_warehouse_interior | Warehouse Interior | high | 6 | 4 |
| sei_container_maze | Container Maze | high | 6 | 6 |
| sei_transformer_station | Transformer Station | high | -2 | 6 |
| sei_fuel_depot | Fuel Depot | high | 202 | 2 |
| sei_break_room | Break Room | safe | 6 | 0 |
