# Vantucky — Zone Map

## Zone Info

| Field | Value |
|-------|-------|
| Zone ID | `vantucky` |
| Danger Level | sketchy |
| Map Color | Yellow |
| World Position | (2, -4) |
| Start Room | `vantucky_the_compound` |

## ASCII Map

```
                    [ 7]

[ 1]-[ 2] [ 3]-[ 4] [ 5]-[ 6]
  |         |         |
[ 8]-[SS] [10] [11]-[12]
       |    |    |    |
     [13]-[14]-[15]-[16]-[17]-[18]
       |    |    |         |    |
     [19]-[20]-[21]-[22] [23]-[24]
       |                   |
     [25]-[26]-[27]>      [28]
       |
     [29]
       |
     [30]
```

**Legend:** `[SS]` = safe start room · `>` = zone border east (Troutdale) · `[7]` is connected via E/W exit to `[4]` (coords are diagonal; no clean ASCII line drawn)

## Room Table

| # | Room ID | Name | Danger Level | Coordinates |
|---|---------|------|--------------|-------------|
| 1 | `vantucky_neutral_back` | Neutral Ground Back Room | safe | (-2, -4) |
| 2 | `vantucky_neutral_vault` | Neutral Ground Vault | safe | (0, -4) |
| 3 | `vantucky_river_trail` | Columbia River Trail | inherit | (2, -4) |
| 4 | `vantucky_fishers_landing` | Fisher's Landing | inherit | (4, -4) |
| 5 | `vantucky_farm_compound` | Farm Compound | inherit | (6, -4) |
| 6 | `vantucky_water_tower` | Water Tower | inherit | (8, -4) |
| 7 | `vantucky_river_cliffs` | River Cliffs | inherit | (6, -6) |
| 8 | `vantucky_neutral_pawn` | Neutral Ground Pawn Shop | safe | (-2, -2) |
| 9 | `vantucky_the_compound` | The Compound | safe | (0, -2) |
| 10 | `vantucky_checkpoint_north` | Northern Checkpoint | inherit | (2, -2) |
| 11 | `vantucky_compound_alpha` | Compound Alpha | inherit | (4, -2) |
| 12 | `vantucky_compound_bravo` | Compound Bravo | inherit | (6, -2) |
| 13 | `vantucky_fourth_plain_west` | Fourth Plain — Vantucky Border | safe | (0, 0) |
| 14 | `vantucky_andresen_road` | Andresen Road | inherit | (2, 0) |
| 15 | `vantucky_orchards` | The Orchards | inherit | (4, 0) |
| 16 | `vantucky_164th_ave` | 164th Avenue — Militia Territory | inherit | (6, 0) |
| 17 | `vantucky_shooting_range` | Shooting Range | inherit | (8, 0) |
| 18 | `vantucky_abandoned_mall` | Abandoned Mall | inherit | (9, 0) |
| 19 | `vantucky_gas_station_ruins` | Gas Station Ruins | inherit | (0, 2) |
| 20 | `vantucky_gun_market` | Gun Market | inherit | (2, 2) |
| 21 | `vantucky_burnt_bridge_creek` | Burnt Bridge Creek | inherit | (4, 2) |
| 22 | `vantucky_hidden_bunker` | Hidden Bunker | inherit | (6, 2) |
| 23 | `vantucky_ammo_depot` | Ammo Depot | inherit | (8, 2) |
| 24 | `vantucky_east_side` | East Side Strip | inherit | (9, 2) |
| 25 | `vantucky_i84_onramp` | I-84 On-Ramp | inherit | (0, 4) |
| 26 | `vantucky_i84_overpass` | I-84 Overpass | inherit | (2, 4) |
| 27 | `vantucky_i84_east` | I-84 East — Troutdale Border | inherit | (4, 4) |
| 28 | `vantucky_rail_spur` | Rail Spur Yard | inherit | (8, 4) |
| 29 | `vantucky_trailer_park` | Trailer Park | inherit | (0, 6) |
| 30 | `vantucky_overgrown_freeway` | Overgrown Freeway | inherit | (0, 8) |

## Points of Interest

| Room | Name | POIs |
|------|------|------|
| `vantucky_the_compound` [SS] | The Compound | Merchant `$` · Healer `+` · Trainer `T` · Banker · Hireling · Motel Keeper |
| `vantucky_neutral_vault` [2] | Neutral Ground Vault | Chip Doc `N` |
| `vantucky_fourth_plain_west` [13] | Fourth Plain — Vantucky Border | Zone Map `M` |

**POI symbol key:** `$` merchant · `+` healer · `T` trainer · `G` guard · `N` notable NPC · `M` zone map · `C` cover · `E` other equipment
