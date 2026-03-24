# Downtown Portland

Zone ID: `downtown` | Danger Level: sketchy | World Position: (0, 0)

```mermaid
graph LR
    pioneer_square["Pioneer Courthouse Square\n[DL: med]"]
    morrison_bridge["Morrison Bridge Approach\n[DL: med]"]
    broadway_ruins["Broadway Ruins\n[DL: med]"]
    market_district["Market District\n[DL: med]"]
    waterfront_trail["Waterfront Trail\n[DL: med]"]
    transit_mall["Transit Mall\n[DL: med]"]
    director_park["Director Park Wasteland\n[DL: med]"]
    burnside_crossing["Burnside Crossing\n[DL: med]"]
    underground_max["Underground MAX Station\n[DL: med]"]
    courthouse_steps["Courthouse Steps\n[DL: med]"]
    cliff_base["Base of the Cliff\n[DL: med]"]
    cliff_top["Top of the Cliff\n[DL: med]"]
    ravine_flooded["Flooded Ravine\n[DL: med]"]
    downtown_underground["The Underground\n[DL: safe] [S]"]

    pioneer_square -->|north| morrison_bridge
    pioneer_square -->|east| broadway_ruins
    pioneer_square -->|south| market_district
    pioneer_square -->|west| transit_mall
    morrison_bridge -->|south| pioneer_square
    morrison_bridge -->|east| burnside_crossing
    morrison_bridge -->|west| waterfront_trail
    morrison_bridge -->|north| downtown_underground
    broadway_ruins -->|west| pioneer_square
    broadway_ruins -->|north| burnside_crossing
    broadway_ruins -->|south| courthouse_steps
    market_district -->|north| pioneer_square
    market_district -->|east| courthouse_steps
    market_district -->|west| director_park
    waterfront_trail -->|east| morrison_bridge
    waterfront_trail -->|northwest| sauvie_bridge_south
    transit_mall -->|east| pioneer_square
    transit_mall -->|south| director_park
    transit_mall -->|down| underground_max
    transit_mall -->|west| beav_canyon_road_east
    director_park -->|north| transit_mall
    director_park -->|east| market_district
    director_park -->|south| lo_checkpoint_north
    burnside_crossing -->|south| broadway_ruins
    burnside_crossing -->|west| morrison_bridge
    burnside_crossing -->|north| ne_broadway_bridge
    underground_max -->|up| transit_mall
    courthouse_steps -->|west| market_district
    courthouse_steps -->|north| broadway_ruins
    courthouse_steps -->|southeast| flats_hawthorne_approach
    courthouse_steps -->|east| cliff_base
    cliff_base -->|west| courthouse_steps
    cliff_base -->|up| cliff_top
    cliff_top -->|down| cliff_base
    cliff_top -->|east| ravine_flooded
    ravine_flooded -->|west| cliff_top
    downtown_underground -->|south| morrison_bridge
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| pioneer_square | Pioneer Courthouse Square | med | 0 | 0 |
| morrison_bridge | Morrison Bridge Approach | med | 0 | -2 |
| broadway_ruins | Broadway Ruins | med | 2 | 0 |
| market_district | Market District | med | 0 | 2 |
| waterfront_trail | Waterfront Trail | med | -2 | -2 |
| transit_mall | Transit Mall | med | -2 | 0 |
| director_park | Director Park Wasteland | med | -2 | 2 |
| burnside_crossing | Burnside Crossing | med | 2 | -2 |
| underground_max | Underground MAX Station | med | 202 | 0 |
| courthouse_steps | Courthouse Steps | med | 2 | 2 |
| cliff_base | Base of the Cliff | med | 4 | 2 |
| cliff_top | Top of the Cliff | med | 4 | 0 |
| ravine_flooded | Flooded Ravine | med | 4 | -2 |
| downtown_underground | The Underground | safe | 0 | -4 |
