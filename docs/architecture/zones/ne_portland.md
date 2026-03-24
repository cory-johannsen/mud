# Northeast Portland

Zone ID: `ne_portland` | Danger Level: sketchy | World Position: (2, 0)

```mermaid
graph LR
    ne_alberta_street["Alberta Street\n[DL: med]"]
    ne_mississippi_ave["Mississippi Avenue\n[DL: med]"]
    ne_mlk_boulevard["MLK Boulevard\n[DL: med]"]
    ne_irvington_ruins["Irvington Ruins\n[DL: med]"]
    ne_hollywood_district["Hollywood District\n[DL: med]"]
    ne_sullivans_gulch["Sullivan's Gulch\n[DL: med]"]
    ne_alameda_ridge["Alameda Ridge\n[DL: med]"]
    ne_lloyd_district["Lloyd District\n[DL: med]"]
    ne_broadway_bridge["Broadway Bridge\n[DL: med]"]
    ne_i205_onramp["I-205 On-Ramp\n[DL: med]"]
    ne_cully_road["Cully Road\n[DL: med]"]
    ne_interstate_ave["Interstate Avenue\n[DL: med]"]
    ne_killingsworth["Killingsworth Street\n[DL: med]"]
    ne_fremont_street["Fremont Street\n[DL: med]"]
    ne_sandy_boulevard["Sandy Boulevard\n[DL: med]"]
    ne_grant_park["Grant Park\n[DL: med]"]
    ne_rose_quarter["Rose Quarter\n[DL: med]"]
    ne_brewery_compound["Brewery Compound\n[DL: med]"]
    ne_bike_shop_ruins["Bike Shop Ruins\n[DL: med]"]
    ne_williams_corridor["Williams Corridor\n[DL: med]"]
    ne_prescott_street["Prescott Street\n[DL: med]"]
    ne_columbia_blvd["Columbia Boulevard\n[DL: med]"]
    ne_i5_bridge_approach["I-5 Bridge Approach\n[DL: med]"]
    ne_corner_store["Corner Store\n[DL: safe] [S]"]

    ne_alberta_street -->|south| ne_fremont_street
    ne_alberta_street -->|east| ne_prescott_street
    ne_alberta_street -->|west| ne_williams_corridor
    ne_alberta_street -->|north| ne_corner_store
    ne_mississippi_ave -->|south| ne_williams_corridor
    ne_mississippi_ave -->|north| ne_killingsworth
    ne_mlk_boulevard -->|south| ne_lloyd_district
    ne_mlk_boulevard -->|north| ne_killingsworth
    ne_mlk_boulevard -->|east| ne_hollywood_district
    ne_irvington_ruins -->|east| ne_hollywood_district
    ne_irvington_ruins -->|north| ne_fremont_street
    ne_hollywood_district -->|west| ne_irvington_ruins
    ne_hollywood_district -->|south| ne_sandy_boulevard
    ne_hollywood_district -->|east| ne_i205_onramp
    ne_alameda_ridge -->|west| ne_grant_park
    ne_lloyd_district -->|north| ne_mlk_boulevard
    ne_broadway_bridge -->|south| burnside_crossing
    ne_i205_onramp -->|west| ne_hollywood_district
    ne_i205_onramp -->|east| pdx_airport_way_west
    ne_cully_road -->|south| ne_prescott_street
    ne_cully_road -->|east| grinders_row
    ne_cully_road -->|west| ne_bike_shop_ruins
    ne_interstate_ave -->|south| ne_rose_quarter
    ne_interstate_ave -->|east| ne_williams_corridor
    ne_killingsworth -->|south| ne_mississippi_ave
    ne_killingsworth -->|north| ne_columbia_blvd
    ne_fremont_street -->|north| ne_alberta_street
    ne_fremont_street -->|south| ne_irvington_ruins
    ne_fremont_street -->|east| ne_grant_park
    ne_sandy_boulevard -->|north| ne_hollywood_district
    ne_grant_park -->|west| ne_fremont_street
    ne_grant_park -->|east| ne_alameda_ridge
    ne_rose_quarter -->|north| ne_interstate_ave
    ne_rose_quarter -->|south| ne_sullivans_gulch
    ne_brewery_compound -->|south| ne_mississippi_ave
    ne_brewery_compound -->|east| ne_williams_corridor
    ne_bike_shop_ruins -->|east| ne_cully_road
    ne_williams_corridor -->|east| ne_alberta_street
    ne_williams_corridor -->|north| ne_mississippi_ave
    ne_williams_corridor -->|south| ne_broadway_bridge
    ne_williams_corridor -->|west| ne_interstate_ave
    ne_williams_corridor -->|northeast| ne_bike_shop_ruins
    ne_prescott_street -->|north| ne_cully_road
    ne_prescott_street -->|west| ne_alberta_street
    ne_columbia_blvd -->|south| ne_killingsworth
    ne_columbia_blvd -->|north| ne_i5_bridge_approach
    ne_i5_bridge_approach -->|south| ne_columbia_blvd
    ne_i5_bridge_approach -->|north| couve_interstate_bridge_south
    ne_corner_store -->|south| ne_alberta_street
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| ne_alberta_street | Alberta Street | med | 0 | 0 |
| ne_mississippi_ave | Mississippi Avenue | med | -2 | -2 |
| ne_mlk_boulevard | MLK Boulevard | med | 202 | 0 |
| ne_irvington_ruins | Irvington Ruins | med | 0 | 4 |
| ne_hollywood_district | Hollywood District | med | 2 | 4 |
| ne_sullivans_gulch | Sullivan's Gulch | med | -4 | 4 |
| ne_alameda_ridge | Alameda Ridge | med | 4 | 2 |
| ne_lloyd_district | Lloyd District | med | 202 | 2 |
| ne_broadway_bridge | Broadway Bridge | med | -2 | 2 |
| ne_i205_onramp | I-205 On-Ramp | med | 4 | 4 |
| ne_cully_road | Cully Road | med | 2 | -2 |
| ne_interstate_ave | Interstate Avenue | med | -4 | 0 |
| ne_killingsworth | Killingsworth Street | med | -2 | -4 |
| ne_fremont_street | Fremont Street | med | 0 | 2 |
| ne_sandy_boulevard | Sandy Boulevard | med | 2 | 6 |
| ne_grant_park | Grant Park | med | 2 | 2 |
| ne_rose_quarter | Rose Quarter | med | -4 | 2 |
| ne_brewery_compound | Brewery Compound | med | 202 | 4 |
| ne_bike_shop_ruins | Bike Shop Ruins | med | 0 | -2 |
| ne_williams_corridor | Williams Corridor | med | -2 | 0 |
| ne_prescott_street | Prescott Street | med | 2 | 0 |
| ne_columbia_blvd | Columbia Boulevard | med | -2 | -6 |
| ne_i5_bridge_approach | I-5 Bridge Approach | med | -2 | -8 |
| ne_corner_store | Corner Store | safe | 0 | -4 |
