# Southeast Industrial — Zone Map

## ASCII Map

```
               [ 1]-[ 2]     
                 |           
[ 3]-[ 4] [ 5]-[ 6]-[ 7] [SS]
       |    |         |      
     [ 9]-[10]-[11]-[12]-[13]
       |    |              | 
     [14] [15]-[16]      [17]
       |    .              | 
     [18]                [19]
```

## Room Table

| # | Room ID | Name | Danger Level | Coordinates |
|---|---------|------|--------------|-------------|
| 1 | `sei_machine_shop` | Machine Shop | inherit | (2, -2) |
| 2 | `sei_signal_tower` | Signal Tower | inherit | (4, -2) |
| 3 | `sei_chemical_plant` | Chemical Plant Ruins | inherit | (-4, 0) |
| 4 | `sei_mcloughlin_blvd` | McLoughlin Boulevard | inherit | (-2, 0) |
| 5 | `sei_powell_terminus` | Powell Boulevard Terminus | inherit | (0, 0) |
| 6 | `sei_holgate_blvd` | Holgate Boulevard | inherit | (2, 0) |
| 7 | `sei_river_access` | River Access | inherit | (4, 0) |
| 8 | `sei_break_room` | Break Room | safe | (6, 0) |
| 9 | `sei_crane_yard` | Crane Yard | inherit | (-2, 2) |
| 10 | `sei_industrial_way` | Industrial Way | inherit | (0, 2) |
| 11 | `sei_warehouse_district` | Warehouse District | inherit | (2, 2) |
| 12 | `sei_dock_row` | Dock Row | inherit | (4, 2) |
| 13 | `sei_loading_bay` | Loading Bay | inherit | (6, 2) |
| 14 | `sei_pipe_yard` | Pipe Yard | inherit | (-2, 4) |
| 15 | `sei_ross_bridge_approach` | Ross Island Bridge Approach | inherit | (0, 4) |
| 16 | `sei_dry_dock` | Dry Dock | inherit | (2, 4) |
| 17 | `sei_warehouse_interior` | Warehouse Interior | inherit | (6, 4) |
| 18 | `sei_transformer_station` | Transformer Station | inherit | (-2, 6) |
| 19 | `sei_container_maze` | Container Maze | inherit | (6, 6) |

## Orphaned Rooms

These rooms have off-grid coordinates and do not appear in the map above.

| Room ID | Name | Coordinates |
|---------|------|-------------|
| `sei_milwaukie_junction` | Milwaukie Junction | (202, 0) |
| `sei_rail_siding` | Rail Siding | (200, 6) |
| `sei_fuel_depot` | Fuel Depot | (202, 2) |
