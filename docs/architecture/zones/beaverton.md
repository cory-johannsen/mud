# The Free State of Beaverton

Zone ID: `beaverton` | Danger Level: sketchy | World Position: (-2, 0)

```mermaid
graph LR
    beav_canyon_road_east["Canyon Road East\n[DL: med]"]
    beav_canyon_road_west["Canyon Road West\n[DL: med]"]
    beav_town_center["Beaverton Town Center\n[DL: med]"]
    beav_food_court_command["Food Court Command Center\n[DL: med]"]
    beav_cedar_hills["Cedar Hills Crossing\n[DL: med]"]
    beav_murray_blvd["Murray Boulevard Patrol\n[DL: med]"]
    beav_nature_park["Tualatin Hills Nature Park\n[DL: med]"]
    beav_sunset_transit["Sunset Transit Center\n[DL: med]"]
    beav_tv_highway_west["TV Highway West\n[DL: med]"]
    beav_progress_ridge["Progress Ridge Lookout\n[DL: med]"]
    beav_walker_road["Walker Road Residential\n[DL: med]"]
    beav_hall_boulevard["Hall Boulevard\n[DL: med]"]
    beav_parking_garage_a["Parking Structure Alpha\n[DL: med]"]
    beav_parking_garage_b["Parking Structure Bravo\n[DL: med]"]
    beav_cul_de_sac_west["Westside Cul-de-Sac\n[DL: med]"]
    beav_hoa_court["HOA Court of Justice\n[DL: med]"]
    beav_bike_path_north["Fanno Creek Trail\n[DL: med]"]
    beav_neighborhood_watch_north["Neighborhood Watch North\n[DL: med]"]
    beav_neighborhood_watch_east["Neighborhood Watch East\n[DL: med]"]
    beav_beaverton_hillsdale["Beaverton-Hillsdale Hwy\n[DL: med]"]
    beav_scholls_ferry["Scholls Ferry Crossing\n[DL: med]"]
    beav_cul_de_sac_south["Southside Cul-de-Sac\n[DL: med]"]
    beav_free_market["Free Market\n[DL: safe] [S]"]

    beav_canyon_road_east -->|east| transit_mall
    beav_canyon_road_east -->|west| beav_canyon_road_west
    beav_canyon_road_east -->|south| beav_beaverton_hillsdale
    beav_canyon_road_east -->|north| beav_free_market
    beav_canyon_road_west -->|east| beav_canyon_road_east
    beav_canyon_road_west -->|west| beav_town_center
    beav_canyon_road_west -->|north| beav_sunset_transit
    beav_town_center -->|east| beav_canyon_road_west
    beav_town_center -->|north| beav_hall_boulevard
    beav_town_center -->|west| beav_food_court_command
    beav_food_court_command -->|east| beav_town_center
    beav_food_court_command -->|south| beav_parking_garage_a
    beav_cedar_hills -->|south| beav_hall_boulevard
    beav_cedar_hills -->|west| beav_tv_highway_west
    beav_cedar_hills -->|north| beav_bike_path_north
    beav_murray_blvd -->|south| beav_progress_ridge
    beav_murray_blvd -->|east| beav_beaverton_hillsdale
    beav_murray_blvd -->|west| beav_cul_de_sac_west
    beav_nature_park -->|south| beav_bike_path_north
    beav_sunset_transit -->|south| beav_canyon_road_west
    beav_sunset_transit -->|north| beav_neighborhood_watch_north
    beav_tv_highway_west -->|west| aloha_tv_highway_east
    beav_tv_highway_west -->|east| beav_cedar_hills
    beav_progress_ridge -->|north| beav_murray_blvd
    beav_progress_ridge -->|east| beav_scholls_ferry
    beav_hall_boulevard -->|south| beav_town_center
    beav_hall_boulevard -->|north| beav_cedar_hills
    beav_hall_boulevard -->|west| beav_walker_road
    beav_parking_garage_a -->|north| beav_food_court_command
    beav_parking_garage_b -->|south| beav_murray_blvd
    beav_cul_de_sac_west -->|east| beav_murray_blvd
    beav_hoa_court -->|east| beav_hall_boulevard
    beav_bike_path_north -->|south| beav_cedar_hills
    beav_bike_path_north -->|north| beav_nature_park
    beav_neighborhood_watch_north -->|south| beav_sunset_transit
    beav_neighborhood_watch_east -->|south| beav_beaverton_hillsdale
    beav_beaverton_hillsdale -->|north| beav_canyon_road_east
    beav_beaverton_hillsdale -->|west| beav_murray_blvd
    beav_scholls_ferry -->|west| beav_progress_ridge
    beav_cul_de_sac_south -->|west| beav_progress_ridge
    beav_free_market -->|south| beav_canyon_road_east
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| beav_canyon_road_east | Canyon Road East | med | 0 | 0 |
| beav_canyon_road_west | Canyon Road West | med | -2 | 0 |
| beav_town_center | Beaverton Town Center — Capitol Building | med | -4 | 0 |
| beav_food_court_command | Food Court Command Center | med | -6 | 0 |
| beav_cedar_hills | Cedar Hills Crossing | med | -4 | -4 |
| beav_murray_blvd | Murray Boulevard Patrol Corridor | med | -2 | 2 |
| beav_nature_park | Tualatin Hills Nature Park | med | -4 | -8 |
| beav_sunset_transit | Sunset Transit Center | med | -2 | -2 |
| beav_tv_highway_west | TV Highway West | med | -6 | -4 |
| beav_progress_ridge | Progress Ridge Lookout | med | -2 | 4 |
| beav_walker_road | Walker Road Residential Patrol | med | -6 | -2 |
| beav_hall_boulevard | Hall Boulevard | med | -4 | -2 |
| beav_parking_garage_a | Parking Structure Alpha | med | -6 | 2 |
| beav_parking_garage_b | Parking Structure Bravo | med | 202 | 24 |
| beav_cul_de_sac_west | Westside Cul-de-Sac Compound | med | -4 | 2 |
| beav_hoa_court | HOA Court of Justice | med | 202 | 28 |
| beav_bike_path_north | Fanno Creek Trail Patrol | med | -4 | -6 |
| beav_neighborhood_watch_north | Neighborhood Watch Post North | med | -2 | -4 |
| beav_neighborhood_watch_east | Neighborhood Watch Post East | med | 202 | 34 |
| beav_beaverton_hillsdale | Beaverton-Hillsdale Highway | med | 0 | 2 |
| beav_scholls_ferry | Scholls Ferry Crossing | med | 0 | 4 |
| beav_cul_de_sac_south | Southside Cul-de-Sac Compound | med | 202 | 40 |
| beav_free_market | Free Market | safe | 0 | -2 |
