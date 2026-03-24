# Felony Flats

Zone ID: `felony_flats` | Danger Level: dangerous | World Position: (4, 2)

```mermaid
graph LR
    flats_82nd_ave["82nd Avenue\n[DL: high]"]
    flats_motel_row["Motel Row\n[DL: high]"]
    flats_strip_mall_north["North Strip Mall\n[DL: high]"]
    flats_strip_mall_south["South Strip Mall\n[DL: high]"]
    flats_foster_road["Foster Road\n[DL: high]"]
    flats_johnson_creek["Johnson Creek\n[DL: high]"]
    flats_powell_blvd["Powell Boulevard\n[DL: high]"]
    flats_lents_park["Lents Park\n[DL: high]"]
    flats_jade_district["Jade District\n[DL: safe] [S]"]
    flats_hawthorne_approach["Hawthorne Approach\n[DL: high]"]
    flats_foster_south["South Foster Road\n[DL: high]"]
    flats_motel_courtyard["Motel Courtyard\n[DL: high]"]
    flats_motel_back_lot["Motel Back Lot\n[DL: high]"]
    flats_parking_lot_east["East Parking Lot\n[DL: high]"]
    flats_alley_east["East Alley\n[DL: high]"]
    flats_apartments_west["Western Apartments\n[DL: high]"]
    flats_apartments_south["Southern Apartments\n[DL: high]"]
    flats_gas_station["Abandoned Gas Station\n[DL: high]"]
    flats_powell_overpass["Powell Overpass\n[DL: high]"]
    flats_pawn_shop["Ruined Pawn Shop\n[DL: high]"]
    flats_convenience_store["Looted Convenience Store\n[DL: high]"]
    flats_rail_yard["Abandoned Rail Yard\n[DL: high]"]

    flats_82nd_ave -->|north| flats_powell_blvd
    flats_82nd_ave -->|south| flats_foster_road
    flats_82nd_ave -->|east| flats_strip_mall_north
    flats_82nd_ave -->|west| flats_motel_row
    flats_82nd_ave -->|northeast| flats_gas_station
    flats_motel_row -->|east| flats_82nd_ave
    flats_motel_row -->|west| flats_motel_back_lot
    flats_strip_mall_north -->|west| flats_82nd_ave
    flats_strip_mall_north -->|south| flats_strip_mall_south
    flats_strip_mall_north -->|east| flats_parking_lot_east
    flats_strip_mall_south -->|north| flats_strip_mall_north
    flats_strip_mall_south -->|west| flats_foster_road
    flats_strip_mall_south -->|east| flats_alley_east
    flats_foster_road -->|north| flats_82nd_ave
    flats_foster_road -->|east| flats_strip_mall_south
    flats_foster_road -->|south| flats_foster_south
    flats_foster_road -->|west| flats_apartments_west
    flats_johnson_creek -->|north| flats_lents_park
    flats_powell_blvd -->|south| flats_82nd_ave
    flats_powell_blvd -->|west| flats_hawthorne_approach
    flats_powell_blvd -->|east| flats_gas_station
    flats_powell_blvd -->|north| flats_powell_overpass
    flats_lents_park -->|south| flats_johnson_creek
    flats_lents_park -->|west| flats_foster_south
    flats_hawthorne_approach -->|east| flats_powell_blvd
    flats_hawthorne_approach -->|northwest| courthouse_steps
    flats_foster_south -->|north| flats_foster_road
    flats_foster_south -->|east| flats_lents_park
    flats_foster_south -->|south| sei_powell_terminus
    flats_foster_south -->|southeast| flats_johnson_creek
    flats_motel_back_lot -->|south| flats_motel_courtyard
    flats_parking_lot_east -->|west| flats_strip_mall_north
    flats_parking_lot_east -->|south| flats_alley_east
    flats_alley_east -->|north| flats_parking_lot_east
    flats_alley_east -->|west| flats_strip_mall_south
    flats_alley_east -->|south| flats_lents_park
    flats_apartments_west -->|east| flats_foster_road
    flats_apartments_south -->|west| flats_pawn_shop
    flats_apartments_south -->|north| flats_convenience_store
    flats_gas_station -->|west| flats_powell_blvd
    flats_gas_station -->|southwest| flats_82nd_ave
    flats_powell_overpass -->|south| flats_powell_blvd
    flats_powell_overpass -->|east| flats_jade_district
    flats_powell_overpass -->|north| flats_rail_yard
    flats_pawn_shop -->|east| flats_apartments_south
    flats_pawn_shop -->|north| flats_convenience_store
    flats_convenience_store -->|south| flats_apartments_south
    flats_convenience_store -->|west| flats_pawn_shop
    flats_rail_yard -->|south| flats_powell_overpass
    flats_jade_district -->|west| flats_powell_overpass
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit
- Note: `flats_alley_east` south exit to `flats_lents_park` is hidden

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| flats_82nd_ave | 82nd Avenue | high | 0 | 0 |
| flats_motel_row | Motel Row | high | -2 | 0 |
| flats_strip_mall_north | North Strip Mall | high | 2 | 0 |
| flats_strip_mall_south | South Strip Mall | high | 2 | 2 |
| flats_foster_road | Foster Road | high | 0 | 2 |
| flats_johnson_creek | Johnson Creek | high | 2 | 6 |
| flats_powell_blvd | Powell Boulevard | high | 0 | -2 |
| flats_lents_park | Lents Park | high | 2 | 4 |
| flats_jade_district | Jade District | safe | 2 | -4 |
| flats_hawthorne_approach | Hawthorne Approach | high | -2 | -2 |
| flats_foster_south | South Foster Road | high | 0 | 4 |
| flats_motel_courtyard | Motel Courtyard | high | -4 | 2 |
| flats_motel_back_lot | Motel Back Lot | high | -4 | 0 |
| flats_parking_lot_east | East Parking Lot | high | 4 | 0 |
| flats_alley_east | East Alley | high | 4 | 2 |
| flats_apartments_west | Western Apartments | high | -2 | 2 |
| flats_apartments_south | Southern Apartments | high | 202 | 0 |
| flats_gas_station | Abandoned Gas Station | high | 2 | -2 |
| flats_powell_overpass | Powell Overpass | high | 0 | -4 |
| flats_pawn_shop | Ruined Pawn Shop | high | 202 | 2 |
| flats_convenience_store | Looted Convenience Store | high | 202 | 4 |
| flats_rail_yard | Abandoned Rail Yard | high | 0 | -6 |
