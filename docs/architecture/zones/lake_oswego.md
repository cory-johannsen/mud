# Lake Oswego Nation

Zone ID: `lake_oswego` | Danger Level: safe | World Position: (0, 4)

```mermaid
graph LR
    lo_checkpoint_north["Northern Checkpoint\n[DL: safe]"]
    lo_lake_view_blvd["Lake View Boulevard\n[DL: safe]"]
    lo_country_club["Lake Oswego Country Club\n[DL: safe]"]
    lo_lakewood_bay["Lakewood Bay Estates\n[DL: safe]"]
    lo_hoa_headquarters["HOA Headquarters\n[DL: safe]"]
    lo_the_commons["The Commons\n[DL: safe] [S]"]
    lo_iron_mountain["Iron Mountain Overlook\n[DL: safe]"]
    lo_tryon_creek["Tryon Creek Buffer Zone\n[DL: safe]"]
    lo_first_addition["First Addition District\n[DL: safe]"]
    lo_millennium_park["Millennium Park\n[DL: safe]"]
    lo_guard_barracks["Guard Barracks\n[DL: safe]"]
    lo_marina["Oswego Marina\n[DL: safe]"]
    lo_tennis_grounds["Tennis Training Grounds\n[DL: safe]"]
    lo_wine_cellar_bunker["Wine Cellar Bunker\n[DL: safe]"]
    lo_gated_estates["Gated Estates\n[DL: safe]"]
    lo_patrol_route_east["Eastern Patrol Route\n[DL: safe]"]
    lo_checkpoint_south["Southern Checkpoint\n[DL: safe]"]
    lo_boat_house["Lakeside Boat House\n[DL: safe]"]
    lo_supply_depot["Supply Depot\n[DL: safe]"]
    lo_driving_range["Driving Range Firebase\n[DL: safe]"]
    lo_citation_court["Citation Court\n[DL: safe]"]
    lo_memorial_garden["Memorial Garden\n[DL: safe]"]

    lo_checkpoint_north -->|north| director_park
    lo_checkpoint_north -->|south| lo_lake_view_blvd
    lo_checkpoint_north -->|east| lo_guard_barracks
    lo_lake_view_blvd -->|north| lo_checkpoint_north
    lo_lake_view_blvd -->|south| lo_the_commons
    lo_lake_view_blvd -->|east| lo_first_addition
    lo_lake_view_blvd -->|west| lo_lakewood_bay
    lo_country_club -->|south| lo_driving_range
    lo_country_club -->|west| lo_the_commons
    lo_lakewood_bay -->|east| lo_lake_view_blvd
    lo_lakewood_bay -->|north| lo_boat_house
    lo_hoa_headquarters -->|north| lo_the_commons
    lo_hoa_headquarters -->|south| lo_gated_estates
    lo_the_commons -->|north| lo_lake_view_blvd
    lo_the_commons -->|east| lo_country_club
    lo_the_commons -->|south| lo_hoa_headquarters
    lo_the_commons -->|west| lo_millennium_park
    lo_iron_mountain -->|west| lo_tennis_grounds
    lo_tryon_creek -->|west| lo_gated_estates
    lo_first_addition -->|west| lo_lake_view_blvd
    lo_first_addition -->|east| lo_patrol_route_east
    lo_millennium_park -->|east| lo_the_commons
    lo_guard_barracks -->|west| lo_checkpoint_north
    lo_marina -->|east| lo_millennium_park
    lo_tennis_grounds -->|east| lo_iron_mountain
    lo_wine_cellar_bunker -->|west| lo_supply_depot
    lo_gated_estates -->|east| lo_tryon_creek
    lo_patrol_route_east -->|west| lo_first_addition
    lo_patrol_route_east -->|south| lo_checkpoint_south
    lo_boat_house -->|south| lo_lakewood_bay
    lo_supply_depot -->|east| lo_wine_cellar_bunker
    lo_supply_depot -->|north| lo_hoa_headquarters
    lo_driving_range -->|north| lo_country_club
    lo_memorial_garden -->|north| lo_the_commons
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| lo_checkpoint_north | Northern Checkpoint | safe | 0 | 0 |
| lo_lake_view_blvd | Lake View Boulevard | safe | 0 | 2 |
| lo_country_club | Lake Oswego Country Club | safe | 2 | 4 |
| lo_lakewood_bay | Lakewood Bay Estates | safe | -2 | 2 |
| lo_hoa_headquarters | HOA Headquarters | safe | 0 | 6 |
| lo_the_commons | The Commons | safe | 0 | 4 |
| lo_iron_mountain | Iron Mountain Overlook | safe | 202 | 0 |
| lo_tryon_creek | Tryon Creek Buffer Zone | safe | 2 | 8 |
| lo_first_addition | First Addition District | safe | 2 | 2 |
| lo_millennium_park | Millennium Park | safe | -2 | 4 |
| lo_guard_barracks | Guard Barracks | safe | 2 | 0 |
| lo_marina | Oswego Marina | safe | 202 | 2 |
| lo_tennis_grounds | Tennis Training Grounds | safe | 202 | 4 |
| lo_wine_cellar_bunker | Wine Cellar Bunker | safe | 202 | 6 |
| lo_gated_estates | Gated Estates | safe | 0 | 8 |
| lo_patrol_route_east | Eastern Patrol Route | safe | 4 | 2 |
| lo_checkpoint_south | Southern Checkpoint | safe | 4 | 4 |
| lo_boat_house | Lakeside Boat House | safe | -2 | 0 |
| lo_supply_depot | Supply Depot | safe | 202 | 8 |
| lo_driving_range | Driving Range Firebase | safe | 2 | 6 |
| lo_citation_court | Citation Court | safe | 202 | 10 |
| lo_memorial_garden | Memorial Garden | safe | 202 | 12 |
