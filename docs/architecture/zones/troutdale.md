# Troutdale

Zone ID: `troutdale` | Danger Level: sketchy | World Position: (6, 0)

```mermaid
graph LR
    trout_i84_west["I-84 Westbound On-Ramp\n[DL: med]"]
    trout_i84_west_junction["I-84 West Junction\n[DL: med]"]
    trout_outlet_mall["Outlet Mall Ruins\n[DL: med]"]
    trout_outlet_east_wing["Outlet Mall East Wing\n[DL: med]"]
    trout_stark_street["Stark Street\n[DL: med]"]
    trout_sandy_river["Sandy River Ford\n[DL: med]"]
    trout_lewis_clark_park["Lewis and Clark State Park\n[DL: med]"]
    trout_crown_point["Crown Point Vista\n[DL: med]"]
    trout_multnomah_falls["Multnomah Falls\n[DL: med]"]
    trout_wind_tunnel["Wind Tunnel\n[DL: med]"]
    trout_edgefield["Edgefield Compound\n[DL: med]"]
    trout_wood_village["Wood Village Ruins\n[DL: med]"]
    trout_parking_lot["Outlet Parking Lot\n[DL: med]"]
    trout_ranger_station["Gorge Ranger Station\n[DL: med]"]
    trout_dam_overlook["Bonneville Dam Overlook\n[DL: med]"]
    trout_gorge_trail["Gorge Trail\n[DL: med]"]
    trout_wind_shelter["Wind Shelter\n[DL: med]"]
    trout_frontage_road["Frontage Road\n[DL: med]"]
    trout_river_trail["Sandy River Trail\n[DL: med]"]
    trout_halsey_street["Halsey Street\n[DL: med]"]
    trout_edgefield_gardens["Edgefield Gardens\n[DL: med]"]
    trout_loading_docks["Loading Docks\n[DL: med]"]
    trout_falls_overlook["Falls Overlook\n[DL: med]"]
    trout_truck_stop["Truck Stop\n[DL: safe] [S]"]

    trout_i84_west -->|west| vantucky_i84_east
    trout_i84_west -->|east| trout_stark_street
    trout_i84_west -->|south| trout_frontage_road
    trout_i84_west -->|north| trout_truck_stop
    trout_i84_west_junction -->|west| pdx_i84_east
    trout_outlet_mall -->|north| trout_stark_street
    trout_outlet_mall -->|east| trout_outlet_east_wing
    trout_outlet_mall -->|south| trout_parking_lot
    trout_outlet_east_wing -->|west| trout_outlet_mall
    trout_outlet_east_wing -->|south| trout_loading_docks
    trout_stark_street -->|west| trout_i84_west
    trout_stark_street -->|east| trout_edgefield
    trout_stark_street -->|south| trout_outlet_mall
    trout_stark_street -->|north| trout_sandy_river
    trout_sandy_river -->|south| trout_stark_street
    trout_lewis_clark_park -->|west| trout_river_trail
    trout_lewis_clark_park -->|north| trout_gorge_trail
    trout_crown_point -->|south| trout_gorge_trail
    trout_crown_point -->|east| trout_wind_tunnel
    trout_multnomah_falls -->|west| trout_wind_tunnel
    trout_multnomah_falls -->|north| trout_falls_overlook
    trout_wind_tunnel -->|west| trout_crown_point
    trout_wind_tunnel -->|east| trout_multnomah_falls
    trout_wind_tunnel -->|south| trout_wind_shelter
    trout_edgefield -->|west| trout_stark_street
    trout_edgefield -->|north| trout_edgefield_gardens
    trout_edgefield -->|east| trout_halsey_street
    trout_wood_village -->|east| trout_frontage_road
    trout_wood_village -->|south| trout_parking_lot
    trout_parking_lot -->|north| trout_outlet_mall
    trout_ranger_station -->|south| trout_gorge_trail
    trout_ranger_station -->|east| trout_dam_overlook
    trout_dam_overlook -->|west| trout_ranger_station
    trout_gorge_trail -->|north| trout_ranger_station
    trout_gorge_trail -->|south| trout_lewis_clark_park
    trout_gorge_trail -->|east| trout_crown_point
    trout_wind_shelter -->|north| trout_wind_tunnel
    trout_frontage_road -->|north| trout_i84_west
    trout_frontage_road -->|west| trout_i84_west_junction
    trout_river_trail -->|east| trout_lewis_clark_park
    trout_halsey_street -->|west| trout_edgefield
    trout_edgefield_gardens -->|south| trout_edgefield
    trout_loading_docks -->|north| trout_outlet_east_wing
    trout_loading_docks -->|west| trout_parking_lot
    trout_falls_overlook -->|south| trout_multnomah_falls
    trout_truck_stop -->|south| trout_i84_west
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| trout_i84_west | I-84 Westbound On-Ramp | med | 0 | 0 |
| trout_i84_west_junction | I-84 West Junction | med | -2 | 2 |
| trout_outlet_mall | Outlet Mall Ruins | med | 2 | 2 |
| trout_outlet_east_wing | Outlet Mall East Wing | med | 4 | 2 |
| trout_stark_street | Stark Street | med | 2 | 0 |
| trout_sandy_river | Sandy River Ford | med | 2 | -2 |
| trout_lewis_clark_park | Lewis and Clark State Park | med | 202 | 10 |
| trout_crown_point | Crown Point Vista | med | 202 | 12 |
| trout_multnomah_falls | Multnomah Falls | med | 202 | 14 |
| trout_wind_tunnel | Wind Tunnel | med | 202 | 16 |
| trout_edgefield | Edgefield Compound | med | 4 | 0 |
| trout_wood_village | Wood Village Ruins | med | 202 | 20 |
| trout_parking_lot | Outlet Parking Lot | med | 2 | 4 |
| trout_ranger_station | Gorge Ranger Station | med | 202 | 24 |
| trout_dam_overlook | Bonneville Dam Overlook | med | 202 | 26 |
| trout_gorge_trail | Gorge Trail | med | 202 | 28 |
| trout_wind_shelter | Wind Shelter | med | 202 | 30 |
| trout_frontage_road | Frontage Road | med | 0 | 2 |
| trout_river_trail | Sandy River Trail | med | 202 | 34 |
| trout_halsey_street | Halsey Street | med | 6 | 0 |
| trout_edgefield_gardens | Edgefield Gardens | med | 4 | -2 |
| trout_loading_docks | Loading Docks | med | 4 | 4 |
| trout_falls_overlook | Falls Overlook | med | 202 | 42 |
| trout_truck_stop | Truck Stop | safe | 0 | -2 |
