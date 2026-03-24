# Battleground Socialist Collective

Zone ID: `battleground` | Danger Level: all_out_war | World Position: (4, -6)

```mermaid
graph LR
    battle_i5_south["I-5 Southbound On-Ramp\n[DL: xtr]"]
    battle_the_gate["The Gate\n[DL: xtr]"]
    battle_main_street["Main Street\n[DL: xtr]"]
    battle_assembly_hall["Assembly Hall\n[DL: xtr]"]
    battle_the_commons["The Commons\n[DL: xtr]"]
    battle_market_square["Market Square\n[DL: xtr]"]
    battle_propaganda_hall["Propaganda Hall\n[DL: xtr]"]
    battle_the_commissariat["The Commissariat\n[DL: xtr]"]
    battle_reeducation_center["Reeducation Center\n[DL: xtr]"]
    battle_worker_barracks["Worker Barracks\n[DL: xtr]"]
    battle_armory["The Armory\n[DL: xtr]"]
    battle_infirmary["The Infirmary\n[DL: safe] [S]"]
    battle_farm_east["East Fields\n[DL: xtr]"]
    battle_farm_west["West Fields\n[DL: xtr]"]
    battle_the_granary["The Granary\n[DL: xtr]"]
    battle_north_fields["North Fields\n[DL: xtr]"]
    battle_the_watchtower["The Watchtower\n[DL: xtr]"]
    battle_memorial_grove["Memorial Grove\n[DL: xtr]"]
    battle_greenhouse["The Greenhouse\n[DL: xtr]"]
    battle_training_yard["Training Yard\n[DL: xtr]"]
    battle_supply_depot["Supply Depot\n[DL: xtr]"]
    battle_cistern["The Cistern\n[DL: xtr]"]

    battle_i5_south -->|south| couve_i5_north
    battle_i5_south -->|north| battle_the_gate
    battle_the_gate -->|south| battle_i5_south
    battle_the_gate -->|north| battle_main_street
    battle_main_street -->|south| battle_the_gate
    battle_main_street -->|north| battle_market_square
    battle_main_street -->|east| battle_assembly_hall
    battle_main_street -->|west| battle_the_commons
    battle_assembly_hall -->|west| battle_main_street
    battle_the_commons -->|east| battle_main_street
    battle_the_commons -->|south| battle_infirmary
    battle_market_square -->|south| battle_main_street
    battle_market_square -->|east| battle_propaganda_hall
    battle_market_square -->|north| battle_farm_east
    battle_market_square -->|west| battle_armory
    battle_propaganda_hall -->|west| battle_market_square
    battle_propaganda_hall -->|north| battle_the_commissariat
    battle_the_commissariat -->|north| battle_reeducation_center
    battle_reeducation_center -->|south| battle_the_commissariat
    battle_worker_barracks -->|east| battle_market_square
    battle_worker_barracks -->|north| battle_greenhouse
    battle_armory -->|east| battle_market_square
    battle_infirmary -->|north| battle_the_commons
    battle_farm_east -->|south| battle_market_square
    battle_farm_east -->|west| battle_farm_west
    battle_farm_east -->|north| battle_north_fields
    battle_farm_west -->|east| battle_farm_east
    battle_farm_west -->|north| battle_the_granary
    battle_the_granary -->|south| battle_farm_west
    battle_the_granary -->|east| battle_north_fields
    battle_north_fields -->|south| battle_farm_east
    battle_north_fields -->|west| battle_the_granary
    battle_north_fields -->|north| battle_the_watchtower
    battle_the_watchtower -->|south| battle_north_fields
    battle_memorial_grove -->|west| battle_assembly_hall
    battle_memorial_grove -->|north| battle_the_commissariat
    battle_greenhouse -->|south| battle_worker_barracks
    battle_greenhouse -->|east| battle_farm_west
    battle_training_yard -->|east| battle_armory
    battle_training_yard -->|north| battle_farm_west
    battle_supply_depot -->|south| battle_the_commissariat
    battle_supply_depot -->|west| battle_north_fields
    battle_cistern -->|east| battle_the_commons
    battle_cistern -->|north| battle_worker_barracks
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| battle_i5_south | I-5 Southbound On-Ramp | xtr | 0 | 0 |
| battle_the_gate | The Gate | xtr | 0 | -2 |
| battle_main_street | Main Street | xtr | 0 | -4 |
| battle_assembly_hall | Assembly Hall | xtr | 2 | -4 |
| battle_the_commons | The Commons | xtr | -2 | -4 |
| battle_market_square | Market Square | xtr | 0 | -6 |
| battle_propaganda_hall | Propaganda Hall | xtr | 2 | -6 |
| battle_the_commissariat | The Commissariat | xtr | 2 | -8 |
| battle_reeducation_center | Reeducation Center | xtr | 2 | -10 |
| battle_worker_barracks | Worker Barracks | xtr | 202 | 16 |
| battle_armory | The Armory | xtr | -2 | -6 |
| battle_infirmary | The Infirmary | safe | -2 | -2 |
| battle_farm_east | East Fields | xtr | 0 | -8 |
| battle_farm_west | West Fields | xtr | -2 | -8 |
| battle_the_granary | The Granary | xtr | -2 | -10 |
| battle_north_fields | North Fields | xtr | 0 | -10 |
| battle_the_watchtower | The Watchtower | xtr | 0 | -12 |
| battle_memorial_grove | Memorial Grove | xtr | 202 | 32 |
| battle_greenhouse | The Greenhouse | xtr | 202 | 34 |
| battle_training_yard | Training Yard | xtr | 202 | 36 |
| battle_supply_depot | Supply Depot | xtr | 202 | 38 |
| battle_cistern | The Cistern | xtr | 202 | 40 |
