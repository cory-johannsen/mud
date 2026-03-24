# The Aloha Neutral Zone

Zone ID: `aloha` | Danger Level: safe | World Position: (-4, 2)

```mermaid
graph LR
    aloha_tv_highway_east["TV Highway — East Gate\n[DL: safe]"]
    aloha_sentry_post_alpha["Sentry Post Alpha\n[DL: safe]"]
    aloha_185th_crossing["185th Avenue Crossing\n[DL: safe]"]
    aloha_the_bazaar["The Bazaar\n[DL: safe] [S]"]
    aloha_brokers_row["Broker's Row\n[DL: safe]"]
    aloha_sentry_post_bravo["Sentry Post Bravo\n[DL: safe]"]
    aloha_tv_highway_west["TV Highway — West Gate\n[DL: safe]"]
    aloha_caravansary["The Caravansary\n[DL: safe]"]
    aloha_park["Aloha Park\n[DL: safe]"]
    aloha_refugee_camp["Refugee Camp\n[DL: safe]"]
    aloha_warehouse_district["Warehouse District\n[DL: safe]"]
    aloha_trading_post["Trading Post\n[DL: safe]"]
    aloha_black_market["Black Market\n[DL: safe]"]
    aloha_checkpoint_lane["Checkpoint Lane\n[DL: safe]"]
    aloha_message_board_plaza["Message Board Plaza\n[DL: safe]"]
    aloha_water_station["Water Station\n[DL: safe]"]
    aloha_inn["The Rusty Lantern Inn\n[DL: safe]"]
    aloha_stables["The Stables\n[DL: safe]"]
    aloha_back_alley["Back Alley\n[DL: safe]"]
    aloha_scrap_yard["Scrap Yard\n[DL: safe]"]
    aloha_old_church["The Old Church\n[DL: safe]"]

    aloha_tv_highway_east -->|east| beav_tv_highway_west
    aloha_tv_highway_east -->|west| aloha_sentry_post_alpha
    aloha_sentry_post_alpha -->|east| aloha_tv_highway_east
    aloha_sentry_post_alpha -->|west| aloha_185th_crossing
    aloha_sentry_post_alpha -->|south| aloha_checkpoint_lane
    aloha_185th_crossing -->|east| aloha_sentry_post_alpha
    aloha_185th_crossing -->|west| aloha_brokers_row
    aloha_185th_crossing -->|north| aloha_message_board_plaza
    aloha_185th_crossing -->|south| aloha_the_bazaar
    aloha_the_bazaar -->|north| aloha_185th_crossing
    aloha_the_bazaar -->|south| aloha_park
    aloha_brokers_row -->|east| aloha_185th_crossing
    aloha_brokers_row -->|west| aloha_caravansary
    aloha_brokers_row -->|south| aloha_black_market
    aloha_sentry_post_bravo -->|east| aloha_caravansary
    aloha_sentry_post_bravo -->|west| aloha_tv_highway_west
    aloha_tv_highway_west -->|west| hills_tv_highway_east
    aloha_tv_highway_west -->|east| aloha_sentry_post_bravo
    aloha_caravansary -->|east| aloha_brokers_row
    aloha_caravansary -->|west| aloha_sentry_post_bravo
    aloha_caravansary -->|south| aloha_stables
    aloha_park -->|north| aloha_the_bazaar
    aloha_park -->|west| aloha_warehouse_district
    aloha_refugee_camp -->|north| aloha_trading_post
    aloha_warehouse_district -->|east| aloha_park
    aloha_warehouse_district -->|north| aloha_black_market
    aloha_warehouse_district -->|south| aloha_back_alley
    aloha_trading_post -->|south| aloha_refugee_camp
    aloha_trading_post -->|north| aloha_checkpoint_lane
    aloha_black_market -->|south| aloha_warehouse_district
    aloha_black_market -->|north| aloha_brokers_row
    aloha_checkpoint_lane -->|north| aloha_sentry_post_alpha
    aloha_checkpoint_lane -->|south| aloha_trading_post
    aloha_message_board_plaza -->|south| aloha_185th_crossing
    aloha_message_board_plaza -->|east| aloha_inn
    aloha_message_board_plaza -->|west| aloha_water_station
    aloha_inn -->|west| aloha_message_board_plaza
    aloha_stables -->|north| aloha_caravansary
    aloha_scrap_yard -->|north| aloha_warehouse_district
    aloha_old_church -->|north| aloha_refugee_camp
```

## Legend

- `[S]` — Safe room (no hostile spawns, services available)
- DL values: `safe` `low` `med` `high` `xtr`
- `direction*` — Locked exit

## Room Table

| ID | Name | Danger Level | map_x | map_y |
|----|------|-------------|-------|-------|
| aloha_tv_highway_east | TV Highway — East Gate | safe | 0 | 0 |
| aloha_sentry_post_alpha | Sentry Post Alpha | safe | -2 | 0 |
| aloha_185th_crossing | 185th Avenue Crossing | safe | -4 | 0 |
| aloha_the_bazaar | The Bazaar | safe | -4 | 2 |
| aloha_brokers_row | Broker's Row | safe | -6 | 0 |
| aloha_sentry_post_bravo | Sentry Post Bravo | safe | -10 | 0 |
| aloha_tv_highway_west | TV Highway — West Gate | safe | -12 | 0 |
| aloha_caravansary | The Caravansary | safe | -8 | 0 |
| aloha_park | Aloha Park | safe | -4 | 4 |
| aloha_refugee_camp | Refugee Camp | safe | -2 | 6 |
| aloha_warehouse_district | Warehouse District | safe | -6 | 4 |
| aloha_trading_post | Trading Post | safe | -2 | 4 |
| aloha_black_market | Black Market | safe | -6 | 2 |
| aloha_checkpoint_lane | Checkpoint Lane | safe | -2 | 2 |
| aloha_message_board_plaza | Message Board Plaza | safe | -4 | -2 |
| aloha_water_station | Water Station | safe | -6 | -2 |
| aloha_inn | The Rusty Lantern Inn | safe | -2 | -2 |
| aloha_stables | The Stables | safe | -8 | 2 |
| aloha_back_alley | Back Alley | safe | -6 | 6 |
| aloha_scrap_yard | Scrap Yard | safe | 202 | 0 |
| aloha_old_church | The Old Church | safe | 202 | 2 |
