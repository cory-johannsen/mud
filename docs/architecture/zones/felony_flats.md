# Felony Flats — Zone Map

## ASCII Map

```
          [ 1]          
            |           
          [ 2]-[SS]     
            |           
     [ 4]-[ 5]-[ 6]     
            |           
[ 7] [ 8]-[ 9]-[10]-[11]
  |         |    |    | 
[12] [13]-[14]-[15]-[16]
            |         . 
          [17]-[18]     
            .    |      
               [19]     
```

## Room Table

| # | Room ID | Name | Danger Level | Coordinates |
|---|---------|------|--------------|-------------|
| 1 | `flats_rail_yard` | Abandoned Rail Yard | inherit | (0, -6) |
| 2 | `flats_powell_overpass` | Powell Overpass | inherit | (0, -4) |
| 3 | `flats_jade_district` | Jade District | safe | (2, -4) |
| 4 | `flats_hawthorne_approach` | Hawthorne Approach | inherit | (-2, -2) |
| 5 | `flats_powell_blvd` | Powell Boulevard | inherit | (0, -2) |
| 6 | `flats_gas_station` | Abandoned Gas Station | inherit | (2, -2) |
| 7 | `flats_motel_back_lot` | Motel Back Lot | inherit | (-4, 0) |
| 8 | `flats_motel_row` | Motel Row | inherit | (-2, 0) |
| 9 | `flats_82nd_ave` | 82nd Avenue | inherit | (0, 0) |
| 10 | `flats_strip_mall_north` | North Strip Mall | inherit | (2, 0) |
| 11 | `flats_parking_lot_east` | East Parking Lot | inherit | (4, 0) |
| 12 | `flats_motel_courtyard` | Motel Courtyard | inherit | (-4, 2) |
| 13 | `flats_apartments_west` | Western Apartments | inherit | (-2, 2) |
| 14 | `flats_foster_road` | Foster Road | inherit | (0, 2) |
| 15 | `flats_strip_mall_south` | South Strip Mall | inherit | (2, 2) |
| 16 | `flats_alley_east` | East Alley | inherit | (4, 2) |
| 17 | `flats_foster_south` | South Foster Road | inherit | (0, 4) |
| 18 | `flats_lents_park` | Lents Park | inherit | (2, 4) |
| 19 | `flats_johnson_creek` | Johnson Creek | inherit | (2, 6) |

## Orphaned Rooms

These rooms have off-grid coordinates and do not appear in the map above.

| Room ID | Name | Coordinates |
|---------|------|-------------|
| `flats_apartments_south` | Southern Apartments | (202, 0) |
| `flats_pawn_shop` | Ruined Pawn Shop | (202, 2) |
| `flats_convenience_store` | Looted Convenience Store | (202, 4) |
